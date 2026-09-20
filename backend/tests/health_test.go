package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Проверяет, что каркас собран целиком: контракт -> сгенерированный роутинг ->
// реализация -> настоящая Postgres, и всё это отвечает по HTTP.
func TestHealthReturnsOkWhenDatabaseIsReachable(t *testing.T) {
	baseURL := startAPI(t)

	resp := get(t, baseURL+"/health")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("ответ не разобрался как JSON: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("ожидался status=ok, получен %q", body.Status)
	}
}
