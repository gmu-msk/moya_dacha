package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
	"github.com/gmu-msk/moya_dacha/backend/internal/monitor"
)

// Дашборд владельца (specs/016-dashboard.md). Он не часть контракта
// приложения: клиент его не зовёт, поэтому в openapi.yaml его нет, и
// маршруты вешаются здесь руками (ADR-0019).

const dashboardPath = "/dashboard"

//go:embed dashboard.html
var dashboardPage []byte

func (s *Server) mountDashboard(mux *http.ServeMux) {
	// Без пароля дашборда нет: адрес отвечает как несуществующий
	// (требование 1).
	if s.cfg.DashboardPassword == "" {
		mux.HandleFunc(dashboardPath, http.NotFound)
		mux.HandleFunc(dashboardPath+"/", http.NotFound)
		return
	}
	mux.Handle("GET "+dashboardPath, s.dashboardAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(dashboardPage)
	})))
	mux.Handle("GET "+dashboardPath+"/data", s.dashboardAuth(http.HandlerFunc(s.dashboardData)))
	s.mountModeration(mux)
}

func (s *Server) dashboardData(w http.ResponseWriter, r *http.Request) {
	snap, err := s.monitor.Collect(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	for i, f := range snap.Feedback.Recent {
		if f.Screenshot != nil {
			url := s.cfg.Media.URL(*f.Screenshot)
			snap.Feedback.Recent[i].ScreenshotURL = &url
		}
	}
	for _, list := range [][]monitor.ModerationItem{snap.Moderation.Reported, snap.Moderation.Recent} {
		for i, it := range list {
			if it.Photo != nil {
				url := s.cfg.Media.URL(*it.Photo)
				list[i].PhotoURL = &url
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snap)
}

// dashboardAuth пускает по HTTP Basic: имя любое, пароль — пароль
// дашборда (требования 2–4). Сравниваются хэши, чтобы время ответа
// не выдавало ни длину пароля, ни совпавшее начало.
func (s *Server) dashboardAuth(next http.Handler) http.Handler {
	want := sha256.Sum256([]byte(s.cfg.DashboardPassword))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		_, password, ok := r.BasicAuth()
		got := sha256.Sum256([]byte(password))
		if !ok || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="moya-dacha"`)
			http.Error(w, "Нужен пароль дашборда", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// errorNoteKey — ключ контекста, под которым лежит текст ошибки
// обработчика: internalError кладёт его туда, observe записывает.
type errorNoteKey struct{}

func noteError(r *http.Request, err error) {
	if note, ok := r.Context().Value(errorNoteKey{}).(*string); ok && err != nil {
		*note = err.Error()
	}
}

// statusRecorder запоминает код ответа.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// observe считает запросы и записывает ошибки сервера для дашборда
// (требования 13–15). Паника в обработчике становится ответом 500 и
// записью об ошибке, а сервис продолжает работать.
func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Сам дашборд не считается и ошибок в учёт не пишет: иначе он
		// видел бы в графиках собственные обновления раз в минуту.
		if path == dashboardPath || strings.HasPrefix(path, dashboardPath+"/") {
			next.ServeHTTP(w, r)
			return
		}

		note := new(string)
		r = r.WithContext(context.WithValue(r.Context(), errorNoteKey{}, note))
		rec := &statusRecorder{ResponseWriter: w}

		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p)
				}
				slog.Error("паника в обработчике", "path", path, "panic", p, "stack", string(debug.Stack()))
				*note = fmt.Sprint("паника: ", p)
				if rec.status == 0 {
					writeError(rec, http.StatusInternalServerError, gen.Error{
						Code:    "internal_error",
						Message: "Что-то сломалось на нашей стороне. Попробуйте позже",
					})
				}
				rec.status = http.StatusInternalServerError
			}
			if path != basePath+"/health" {
				s.monitor.CountRequest()
			}
			if rec.status >= 500 {
				s.monitor.RecordError(r.Method, path, rec.status, *note)
			}
		}()

		next.ServeHTTP(rec, r)
	})
}
