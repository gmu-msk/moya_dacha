// Команда server — единственный бинарник проекта.
// Миграции он не накатывает: это отдельный шаг деплоя (ADR-0005).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

func main() {
	if err := run(); err != nil {
		slog.Error("сервис остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("переменная окружения DATABASE_URL не задана")
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// AUTH_FIXED_CODE задан только на демо-стенде и в тестах: он делает код
	// подтверждения предсказуемым, чтобы фичу можно было показать без SMS
	// (specs/001-auth.md). В проде переменная пуста, и код случайный.
	cfg := api.Config{FixedCode: os.Getenv("AUTH_FIXED_CODE")}
	if cfg.FixedCode != "" {
		slog.Warn("включён фиксированный код подтверждения: войти может кто угодно, в проде так быть не должно")
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.New(pool, cfg).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		slog.Info("сервис запущен", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		slog.Info("получен сигнал остановки, завершаемся")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
