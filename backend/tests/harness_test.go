// Пакет tests содержит интеграционные тесты уровня HTTP.
//
// Он намеренно лежит отдельно от кода и обращается к сервису только так же,
// как обращается мобильное приложение: по HTTP, через публичный контракт.
// Прямые вызовы функций реализации здесь невозможны — см. ADR-0002.
package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// startAPI поднимает сервис на случайном порту с настоящей Postgres
// и возвращает базовый адрес вида http://127.0.0.1:PORT/api.
func startAPI(t *testing.T) string {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL не задан: интеграционные тесты требуют настоящую Postgres, запускайте через `make test`")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("не удалось подключиться к базе: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("база не отвечает: %v", err)
	}

	srv := httptest.NewServer(api.New(pool).Handler())
	t.Cleanup(func() {
		srv.Close()
		pool.Close()
	})

	return srv.URL + "/api"
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("запрос %s не прошёл: %v", url, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}
