// Пакет tests содержит интеграционные тесты уровня HTTP.
//
// Он намеренно лежит отдельно от кода и обращается к сервису только так же,
// как обращается мобильное приложение: по HTTP, через публичный контракт.
// Прямые вызовы функций реализации здесь невозможны — см. ADR-0002.
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// authCode — код подтверждения, который сервис в тестах выдаёт всегда
// (specs/001-auth.md, AUTH_FIXED_CODE).
const authCode = "4242"

// startAPI поднимает сервис с настройками по умолчанию на чистой базе
// и возвращает базовый адрес вида http://127.0.0.1:PORT/api.
func startAPI(t *testing.T) string {
	t.Helper()
	return startAPIWith(t, api.Config{FixedCode: authCode})
}

// startAPIWith — то же, но с заданными настройками сервиса: тесты, которым
// нужны короткие сроки жизни кода, задают их здесь.
func startAPIWith(t *testing.T, cfg api.Config) string {
	t.Helper()

	if cfg.FixedCode == "" {
		cfg.FixedCode = authCode
	}

	pool := connect(t)
	truncateAll(t, pool)

	srv := httptest.NewServer(api.New(pool, cfg).Handler())
	t.Cleanup(srv.Close)

	return srv.URL + "/api"
}

func connect(t *testing.T) *pgxpool.Pool {
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
	t.Cleanup(pool.Close)

	return pool
}

// truncateAll чистит продуктовые таблицы, чтобы тесты не зависели друг
// от друга. Таблицу goose с версиями миграций не трогает: схему накатывает
// отдельный шаг (`make test`), а не тесты.
func truncateAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const query = `
		DO $$
		DECLARE
			tables text;
		BEGIN
			SELECT string_agg(format('%I.%I', schemaname, tablename), ', ')
			INTO tables
			FROM pg_tables
			WHERE schemaname = 'public' AND tablename <> 'goose_db_version';

			IF tables IS NOT NULL THEN
				EXECUTE 'TRUNCATE TABLE ' || tables || ' RESTART IDENTITY CASCADE';
			END IF;
		END $$;`

	if _, err := pool.Exec(ctx, query); err != nil {
		t.Fatalf("не удалось очистить базу перед тестом: %v", err)
	}
}

// do отправляет запрос к сервису. body == nil — запрос без тела;
// token == "" — запрос без заголовка Authorization.
func do(t *testing.T, method, url, token string, body any) *http.Response {
	t.Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("не удалось собрать тело запроса: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("не удалось собрать запрос %s %s: %v", method, url, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("запрос %s %s не прошёл: %v", method, url, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, url, "", nil)
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, url, "", body)
}

// decode разбирает тело ответа в target.
func decode(t *testing.T, resp *http.Response, target any) {
	t.Helper()

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("ответ %s не разобрался как JSON: %v", resp.Request.URL, err)
	}
}

// errorCode возвращает машиночитаемый код ошибки из тела ответа.
func errorCode(t *testing.T, resp *http.Response) string {
	t.Helper()

	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decode(t, resp, &body)

	if body.Message == "" {
		t.Errorf("в ошибке %q пустое сообщение для пользователя", body.Code)
	}

	return body.Code
}
