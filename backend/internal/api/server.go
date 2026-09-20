// Package api связывает сгенерированный из specs/openapi.yaml контракт
// с реализацией. Генерируемый код лежит в backend/api/gen и не правится руками.
package api

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// basePath повторяет servers[].url из контракта.
const basePath = "/api"

// Server реализует gen.StrictServerInterface.
type Server struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Server {
	return &Server{db: db}
}

// Handler возвращает готовый http.Handler со всеми маршрутами контракта.
// Это единственная точка входа в приложение — и для main, и для тестов.
func (s *Server) Handler() http.Handler {
	return gen.HandlerWithOptions(
		gen.NewStrictHandler(s, nil),
		gen.StdHTTPServerOptions{BaseURL: basePath},
	)
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
