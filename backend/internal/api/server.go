// Package api связывает сгенерированный из specs/openapi.yaml контракт
// с реализацией. Генерируемый код лежит в backend/api/gen и не правится руками.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
	"github.com/gmu-msk/moya_dacha/backend/internal/auth"
)

// basePath повторяет servers[].url из контракта.
const basePath = "/api"

// Config — то, что сервису задают снаружи: всё, что отличает стенд
// и тесты от прода.
type Config struct {
	// FixedCode — код подтверждения, который сервис выдаёт всегда,
	// вместо случайного. Нужен стенду и тестам, в проде пуст
	// (переменная окружения AUTH_FIXED_CODE, см. specs/001-auth.md).
	FixedCode string

	// CodeSender доставляет код подтверждения на номер. Пока
	// SMS-провайдер не выбран, это запись в лог.
	CodeSender auth.Sender

	// CodeTTL — сколько живёт выданный код.
	// Ноль означает значение из спеки (5 минут).
	CodeTTL time.Duration

	// ResendAfter — как часто можно просить код на один номер.
	// Ноль означает значение из спеки (60 секунд).
	ResendAfter time.Duration
}

// Server реализует gen.StrictServerInterface.
type Server struct {
	db  *pgxpool.Pool
	cfg Config
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
	return &Server{db: db, cfg: cfg}
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

	return gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseURL:          basePath,
		ErrorHandlerFunc: badRequest,
	})
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
