// Package api связывает сгенерированный из specs/openapi.yaml контракт
// с реализацией. Генерируемый код лежит в backend/api/gen и не правится руками.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
	"github.com/gmu-msk/moya_dacha/backend/internal/auth"
	"github.com/gmu-msk/moya_dacha/backend/internal/feedback"
	"github.com/gmu-msk/moya_dacha/backend/internal/media"
	"github.com/gmu-msk/moya_dacha/backend/internal/monitor"
)

// basePath повторяет servers[].url из контракта.
const basePath = "/api"

// DefaultMediaBaseURL — префикс, по которому сервис раздаёт файлы
// пользователей. Ссылки на них относительные, клиент достраивает их
// до адреса сервиса (specs/002-profile.md, требование 10).
const DefaultMediaBaseURL = "/media"

// Config — то, что сервису задают снаружи: всё, что отличает стенд
// и тесты от прода.
type Config struct {
	// FixedCode — код подтверждения, который сервис выдаёт всегда,
	// вместо случайного. Нужен стенду и тестам, в проде пуст
	// (переменная окружения AUTH_FIXED_CODE, см. specs/001-auth.md).
	FixedCode string

	// Invites включает вход по коду приглашения вместо кода из SMS
	// (переменная окружения AUTH_INVITES, specs/015-invites.md). В этом
	// режиме коды подтверждения не выдаются, и FixedCode не действует.
	Invites bool

	// CodeSender доставляет код подтверждения на номер. Пока
	// SMS-провайдер не выбран, это запись в лог.
	CodeSender auth.Sender

	// CodeTTL — сколько живёт выданный код.
	// Ноль означает значение из спеки (5 минут).
	CodeTTL time.Duration

	// ResendAfter — как часто можно просить код на один номер.
	// Ноль означает значение из спеки (60 секунд).
	ResendAfter time.Duration

	// UnpublishedMediaTTL — сколько живёт загруженная, но не
	// опубликованная фотография. Ноль означает значение из спеки (час).
	UnpublishedMediaTTL time.Duration

	// Media — хранилище файлов пользователей (аватары и фотографии постов).
	// Ноль означает диск во временной папке: так сервис не падает
	// при запуске без настройки, но в проде и на стенде папка задаётся
	// переменной окружения MEDIA_DIR (specs/002-profile.md).
	Media media.Storage

	// DashboardPassword — пароль дашборда владельца (переменная окружения
	// DASHBOARD_PASSWORD, specs/016-dashboard.md). Пустой — дашборда нет.
	DashboardPassword string

	// DiskPath — где дашборд меряет диск: там, где лежат файлы
	// пользователей. Пустой — корень файловой системы.
	DiskPath string

	// Feedback — отзывы разработчику и задачи GitHub (specs/019-feedback.md,
	// требование 33). nil — свой, без GitHub, с хранилищем Media.
	Feedback *feedback.Service
}

// Server реализует gen.StrictServerInterface.
type Server struct {
	db      *pgxpool.Pool
	cfg     Config
	monitor *monitor.Monitor
}

func New(db *pgxpool.Pool, cfg Config) *Server {
	if cfg.CodeSender == nil {
		cfg.CodeSender = auth.LogSender{}
	}
	if cfg.CodeTTL == 0 {
		cfg.CodeTTL = auth.DefaultCodeTTL
	}
	if cfg.ResendAfter == 0 {
		cfg.ResendAfter = auth.DefaultResendAfter
	}
	if cfg.UnpublishedMediaTTL == 0 {
		cfg.UnpublishedMediaTTL = DefaultUnpublishedMediaTTL
	}
	if cfg.Media == nil {
		dir := filepath.Join(os.TempDir(), "moya-dacha-media")
		slog.Warn("хранилище файлов не задано, беру временную папку", "dir", dir)
		cfg.Media = media.NewDisk(dir, DefaultMediaBaseURL)
	}
	if cfg.DiskPath == "" {
		cfg.DiskPath = "/"
	}
	if cfg.Feedback == nil {
		cfg.Feedback = feedback.New(db, feedback.Config{Media: cfg.Media})
	}
	return &Server{db: db, cfg: cfg, monitor: monitor.New(db, cfg.DiskPath)}
}

// RunMonitor меряет машину для дашборда раз в минуту, пока не отменён
// ctx (specs/016-dashboard.md, требование 13). Зовёт его main; тесты
// кладут измерения в базу сами.
func (s *Server) RunMonitor(ctx context.Context) {
	s.monitor.Run(ctx)
}

// Handler возвращает готовый http.Handler со всеми маршрутами контракта.
// Это единственная точка входа в приложение — и для main, и для тестов.
func (s *Server) Handler() http.Handler {
	strict := gen.NewStrictHandlerWithOptions(
		s,
		[]gen.StrictMiddlewareFunc{s.withSession},
		gen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  badRequest,
			ResponseErrorHandlerFunc: internalError,
		},
	)

	api := gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseURL:          basePath,
		ErrorHandlerFunc: badRequest,
	})

	mux := http.NewServeMux()
	mux.Handle("/", api)

	// Файлы пользователей раздаёт сам сервис, пока они лежат у него
	// на диске. Объектное хранилище вернёт пустой префикс — тогда
	// раздавать нечего, ссылки ведут мимо сервиса.
	if prefix, files := s.cfg.Media.FileHandler(); prefix != "" && files != nil {
		mux.Handle(prefix, files)
	}

	s.mountDashboard(mux)

	// Учёт запросов и ошибок для дашборда — снаружи всего остального,
	// чтобы видеть и панику в любом обработчике.
	return s.observe(mux)
}

// badRequest отвечает на запрос, который не разобрался, — телом из
// контракта, а не строкой: клиент разбирает ошибки одинаково,
// независимо от того, где именно запрос был отвергнут.
func badRequest(w http.ResponseWriter, r *http.Request, err error) {
	slog.Debug("запрос не разобрался", "path", r.URL.Path, "err", err)
	writeError(w, http.StatusBadRequest, gen.Error{
		Code:    "invalid_request",
		Message: "Запрос не разобрался",
	})
}

// internalError — то, чем заканчивается любая непредвиденная ошибка
// сервиса. Подробности уходят в лог, наружу не выносятся.
func internalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("ошибка обработки запроса", "path", r.URL.Path, "err", err)
	noteError(r, err)
	writeError(w, http.StatusInternalServerError, gen.Error{
		Code:    "internal_error",
		Message: "Что-то сломалось на нашей стороне. Попробуйте позже",
	})
}

func writeError(w http.ResponseWriter, status int, body gen.Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (s *Server) GetHealth(ctx context.Context, _ gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	if err := s.db.Ping(ctx); err != nil {
		return gen.GetHealth503JSONResponse{
			Code:    "database_unavailable",
			Message: "База данных недоступна",
		}, nil
	}
	return gen.GetHealth200JSONResponse{Status: gen.Ok}, nil
}
