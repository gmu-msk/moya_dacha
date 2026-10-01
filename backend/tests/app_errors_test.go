package tests

// Тесты отчётов об ошибках приложения (specs/021-app-errors.md).
//
// Приём отчёта и данные дашборда проверяются по HTTP, как их видят
// приложение и владелец. Время отчёта сервис ставит по своим часам, а тест
// не может ждать часами и сутками, поэтому сдвигает app_errors.at и
// first_at/last_at группы прямо в базе (раздел «Модель данных»). Сообщения
// о новых ошибках шлёт бот: тест зовёт (*telegram.Bot).CheckAppErrors
// (ФТ-14а) против фейкового Telegram из telegram_test.go.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
	"github.com/gmu-msk/moya_dacha/backend/internal/telegram"
)

// Тексты из спецификации (ФТ-8, ФТ-8а).
const (
	aeHeader = "Новая ошибка в приложении:"

	// aeNick — ник вошедшей тестировщицы из aeTester; aeWho — её строка «Кто».
	aeNick = "Err_Tester"
	aeWho  = "Кто: @" + aeNick
)

// aeAnonymousCount — сообщение о новых ошибках без входа (ФТ-8а).
func aeAnonymousCount(n int) string {
	return fmt.Sprintf("Новые ошибки без входа: %d. Подробности — на дашборде.", n)
}

// aeLimit — предел отчётов за час (ФТ-4).
const aeLimit = 100

// --- Хелперы --------------------------------------------------------------

// appErrorPhone — номер n-го участника тестов ошибок приложения. Номера не
// пересекаются с номерами других тестов пакета.
func appErrorPhone(n int) string {
	return fmt.Sprintf("+7 (900) 821-00-%02d", n)
}

// reportAppErrorRaw — POST /app-errors, сырой ответ. token == "" — без входа.
func reportAppErrorRaw(t *testing.T, baseURL, token string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, baseURL+"/app-errors", token, body)
}

// reportAppError отправляет отчёт и требует 204.
func reportAppError(t *testing.T, baseURL, token string, body map[string]any) {
	t.Helper()

	resp := reportAppErrorRaw(t, baseURL, token, body)
	if resp.StatusCode != http.StatusNoContent {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("на отчёт %v ожидался статус 204, получен %d %s", body, resp.StatusCode, code)
	}
}

// countAppErrors — сколько отчётов в базе.
func countAppErrors(t *testing.T) int {
	t.Helper()
	return countSQL(t, `SELECT count(*) FROM app_errors`)
}

// countAppErrorGroups — сколько групп в базе.
func countAppErrorGroups(t *testing.T) int {
	t.Helper()
	return countSQL(t, `SELECT count(*) FROM app_error_groups`)
}

// shiftAppErrors ставит время отчётам, у которых column = value, и
// пересчитывает first_at/last_at групп по их отчётам: так отчёт выглядит
// пришедшим тогда (раздел «Модель данных»). column — имя колонки из
// спецификации, не ввод пользователя.
func shiftAppErrors(t *testing.T, column, value string, at time.Time) {
	t.Helper()

	n := execSQL(t, `UPDATE app_errors SET at = $1 WHERE `+column+` = $2`, at.UTC().Truncate(time.Microsecond), value)
	if n == 0 {
		t.Fatalf("не нашлось отчётов с %s = %q, чтобы сдвинуть время", column, value)
	}
	syncAppErrorGroups(t)
}

// syncAppErrorGroups выставляет группам first_at и last_at по их отчётам.
func syncAppErrorGroups(t *testing.T) {
	t.Helper()

	execSQL(t, `
		UPDATE app_error_groups g
		SET first_at = s.first_at, last_at = s.last_at
		FROM (SELECT group_id, min(at) AS first_at, max(at) AS last_at
		      FROM app_errors GROUP BY group_id) s
		WHERE s.group_id = g.id`)
}

// appErrorUserID — user_id отчёта с этим os; "" — отчёт без входа.
func appErrorUserID(t *testing.T, os string) string {
	t.Helper()

	pool := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var id *string
	if err := pool.QueryRow(ctx, `SELECT user_id::text FROM app_errors WHERE os = $1`, os).Scan(&id); err != nil {
		t.Fatalf("не удалось прочитать отчёт с os = %q: %v", os, err)
	}
	if id == nil {
		return ""
	}
	return *id
}

// notifiedGroups — сколько групп помечены как отправленные владельцу.
func notifiedGroups(t *testing.T) int {
	t.Helper()
	return countSQL(t, `SELECT count(*) FROM app_error_groups WHERE notified_at IS NOT NULL`)
}

// letters — имя из латинских букв по номеру: цифры при склейке не
// учитываются (ФТ-5), поэтому разные ошибки различаются буквами.
func letters(i int) string {
	return screenName(i)
}

// frames — стек из n строк вида «#k Класс.метод (package:moya_dacha/…)».
// Строки различаются буквами, не только цифрами.
func frames(n int) string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("#%d Frame%s.run (package:moya_dacha/screens/%s.dart:%d:5)", i, letters(i), letters(i), 10+i)
	}
	return strings.Join(out, "\n")
}

// --- Данные дашборда ------------------------------------------------------

type dashAppErrorGroup struct {
	Error     string `json:"error"`
	Where     string `json:"where"`
	Stack     string `json:"stack"`
	Reports   int    `json:"reports"`
	Users     int    `json:"users"`
	Anonymous int    `json:"anonymous"`
	FirstAt   string `json:"first_at"`
	LastAt    string `json:"last_at"`
	Version   string `json:"version"`
	Build     int64  `json:"build"`
	Screen    string `json:"screen"`
	OS        string `json:"os"`
}

type dashAppErrors struct {
	LastDay  int                 `json:"last_day"`
	LastWeek int                 `json:"last_week"`
	NewWeek  int                 `json:"new_week"`
	Groups   []dashAppErrorGroup `json:"groups"`
}

// dashboardAppErrors читает /dashboard/data и возвращает поле app_errors
// и его сырой вид (чтобы отличить [] от null).
func dashboardAppErrors(t *testing.T, root string) (dashAppErrors, map[string]json.RawMessage) {
	t.Helper()

	var envelope struct {
		AppErrors json.RawMessage `json:"app_errors"`
	}
	if err := json.Unmarshal(dashboardRaw(t, root), &envelope); err != nil {
		t.Fatalf("данные дашборда не разобрались как JSON: %v", err)
	}
	if len(envelope.AppErrors) == 0 || string(envelope.AppErrors) == "null" {
		t.Fatalf("в данных дашборда нет поля app_errors (ФТ-11)")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(envelope.AppErrors, &raw); err != nil {
		t.Fatalf("app_errors — не объект: %v", err)
	}

	var out dashAppErrors
	if err := json.Unmarshal(envelope.AppErrors, &out); err != nil {
		t.Fatalf("app_errors не разобрался: %v", err)
	}

	return out, raw
}

// requireNear требует, чтобы время из ответа было не дальше секунды от want.
func requireNear(t *testing.T, value string, want time.Time, what string) {
	t.Helper()

	got := parseDashTime(t, value, what)
	if d := got.Sub(want); d > time.Second || d < -time.Second {
		t.Errorf("%s: ожидалось %s, получено %s", what, want.UTC().Format(time.RFC3339), got.UTC().Format(time.RFC3339))
	}
}

// --- Приём отчёта ---------------------------------------------------------

// Отчёт с входом и без входа принимается ответом 204; время ставит сервер;
// с входом отчёт записан от имени человека, без входа — без человека
// (ФТ-1–3). Отчёт виден на дашборде (ФТ-11, ФТ-13).
func TestAppErrorReportAccepted(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})
	anna := newAppErrorUser(t, baseURL, 1)

	before := time.Now()
	reportAppError(t, baseURL, anna.token, map[string]any{
		"error":   "Null check operator used on a null value",
		"stack":   "#0 PostScreen.build (package:moya_dacha/screens/post_screen.dart:120:5)",
		"version": "1.0.0",
		"build":   386900,
		"screen":  "post",
		"os":      "android 14",
	})
	reportAppError(t, baseURL, "", map[string]any{
		"error": "Ошибка на экране входа",
		"os":    "без входа",
	})
	after := time.Now()

	if got := countAppErrors(t); got != 2 {
		t.Fatalf("после двух отчётов в базе ожидалось 2 записи, найдено %d", got)
	}
	if got := appErrorUserID(t, "android 14"); got != anna.id {
		t.Errorf("отчёт с токеном должен быть записан от имени человека %s, записан от %q (ФТ-3)", anna.id, got)
	}
	if got := appErrorUserID(t, "без входа"); got != "" {
		t.Errorf("отчёт без токена должен быть без человека, записан от %q (ФТ-3)", got)
	}

	outside := countSQL(t, `SELECT count(*) FROM app_errors WHERE at < $1 OR at > $2`,
		before.Add(-clockSlack), after.Add(clockSlack))
	if outside != 0 {
		t.Errorf("время отчёта ставит сервер по своим часам (ФТ-2): %d отчётов со временем вне запроса", outside)
	}

	data, _ := dashboardAppErrors(t, root)
	if len(data.Groups) != 2 {
		t.Fatalf("на дашборде ожидалось 2 группы, получено %d: %+v", len(data.Groups), data.Groups)
	}
	var withUser, anonymous *dashAppErrorGroup
	for i := range data.Groups {
		switch data.Groups[i].Error {
		case "Null check operator used on a null value":
			withUser = &data.Groups[i]
		case "Ошибка на экране входа":
			anonymous = &data.Groups[i]
		}
	}
	if withUser == nil || anonymous == nil {
		t.Fatalf("на дашборде нет отправленных ошибок: %+v", data.Groups)
	}
	if withUser.Users != 1 || withUser.Anonymous != 0 || withUser.Reports != 1 {
		t.Errorf("отчёт с входом: ожидалось reports=1 users=1 anonymous=0, получено %+v", *withUser)
	}
	if anonymous.Users != 0 || anonymous.Anonymous != 1 || anonymous.Reports != 1 {
		t.Errorf("отчёт без входа: ожидалось reports=1 users=0 anonymous=1, получено %+v", *anonymous)
	}
	if data.LastDay != 2 || data.LastWeek != 2 || data.NewWeek != 2 {
		t.Errorf("ожидалось last_day=2 last_week=2 new_week=2, получено %d/%d/%d", data.LastDay, data.LastWeek, data.NewWeek)
	}
}

// Недействительный токен — как без входа: 204, отчёт без человека, не 401
// (ФТ-3). Токен после выхода — тоже недействителен.
func TestAppErrorReportWithInvalidTokenIsAnonymous(t *testing.T) {
	baseURL := startAPI(t)
	anna := newAppErrorUser(t, baseURL, 1)
	boris := newAppErrorUser(t, baseURL, 2)

	if resp := signOut(t, baseURL, boris.token); resp.StatusCode >= 300 {
		t.Fatalf("выход не прошёл: статус %d", resp.StatusCode)
	}

	for _, c := range []struct{ token, os string }{
		{"not-a-real-token", "выдуманный токен"},
		{boris.token, "токен после выхода"},
	} {
		resp := reportAppErrorRaw(t, baseURL, c.token, map[string]any{"error": "Bad state", "os": c.os})
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("%s: ожидался статус 204 (ручка не отвечает 401), получен %d", c.os, resp.StatusCode)
		}
		if got := appErrorUserID(t, c.os); got != "" {
			t.Errorf("%s: отчёт должен быть записан как без входа, записан от %q", c.os, got)
		}
	}

	// Токен Анны по-прежнему работает — это не сломалось.
	reportAppError(t, baseURL, anna.token, map[string]any{"error": "Bad state", "os": "анна"})
	if got := appErrorUserID(t, "анна"); got != anna.id {
		t.Errorf("отчёт с действительным токеном должен быть от %s, записан от %q", anna.id, got)
	}
}

// Поля по правилам, включая границы, принимаются (ФТ-1). Символы — это
// символы, а не байты: кириллица на границе проходит.
func TestAppErrorReportAcceptsLimits(t *testing.T) {
	baseURL := startAPI(t)

	cases := map[string]map[string]any{
		"только error":         {"error": "x"},
		"error 2000 символов":  {"error": strings.Repeat("ё", 2000)},
		"stack 20000 символов": {"error": "длинный стек", "stack": strings.Repeat("ж", 20000)},
		"пустой stack":         {"error": "пустой стек", "stack": ""},
		"version 32 символа":   {"error": "версия", "version": strings.Repeat("в", 32)},
		"build 0":              {"error": "сборка ноль", "build": 0},
		"build большой":        {"error": "сборка большая", "build": int64(9_000_000_000)},
		"screen одна буква":    {"error": "экран а", "screen": "a"},
		"screen 32 символа":    {"error": "экран длинный", "screen": strings.Repeat("a_", 16)},
		"os 100 символов":      {"error": "система", "os": strings.Repeat("о", 100)},
		"все поля":             {"error": "всё", "stack": "#0 main", "version": "1.0.0", "build": 1, "screen": "post_screen", "os": "android 14"},
	}
	for name, body := range cases {
		resp := reportAppErrorRaw(t, baseURL, "", body)
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("%s: ожидался статус 204, получен %d", name, resp.StatusCode)
		}
	}
	if got := countAppErrors(t); got != len(cases) {
		t.Errorf("ожидалось %d записанных отчётов, найдено %d", len(cases), got)
	}
}

// Поле не по правилам — 400 invalid_report, и ничего не записано (ФТ-1).
func TestAppErrorReportRejectsInvalidFields(t *testing.T) {
	baseURL := startAPI(t)
	anna := newAppErrorUser(t, baseURL, 1)

	cases := map[string]map[string]any{
		"нет error":              {"stack": "#0 main"},
		"пустой error":           {"error": ""},
		"error 2001 символ":      {"error": strings.Repeat("ё", 2001)},
		"stack 20001 символ":     {"error": "стек", "stack": strings.Repeat("ж", 20001)},
		"version 33 символа":     {"error": "версия", "version": strings.Repeat("в", 33)},
		"build отрицательный":    {"error": "сборка", "build": -1},
		"screen с большой буквы": {"error": "экран", "screen": "Post"},
		"screen с дефисом":       {"error": "экран", "screen": "post-screen"},
		"screen с цифрой":        {"error": "экран", "screen": "post2"},
		"screen кириллицей":      {"error": "экран", "screen": "пост"},
		"screen 33 символа":      {"error": "экран", "screen": strings.Repeat("a", 33)},
		"os 101 символ":          {"error": "система", "os": strings.Repeat("о", 101)},
	}
	for name, body := range cases {
		for _, token := range []string{"", anna.token} {
			who := "без входа"
			if token != "" {
				who = "с входом"
			}
			resp := reportAppErrorRaw(t, baseURL, token, body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%s (%s): ожидался статус 400, получен %d", name, who, resp.StatusCode)
				continue
			}
			if code := errorCode(t, resp); code != "invalid_report" {
				t.Errorf("%s (%s): ожидался код invalid_report, получен %q", name, who, code)
			}
		}
	}
	if got := countAppErrors(t); got != 0 {
		t.Errorf("отчёты не по правилам не записываются, а в базе %d", got)
	}
	if got := countAppErrorGroups(t); got != 0 {
		t.Errorf("отчёты не по правилам не создают групп, а в базе %d", got)
	}
}

// --- Предел в час ---------------------------------------------------------

// От одного человека — не больше 100 отчётов за час, дальше 429
// too_many_reports и ничего не записано; предел одного не мешает другому
// и отчётам без входа (ФТ-4).
func TestAppErrorReportLimitPerUser(t *testing.T) {
	baseURL := startAPI(t)
	anna := newAppErrorUser(t, baseURL, 1)
	boris := newAppErrorUser(t, baseURL, 2)

	for i := 0; i < aeLimit; i++ {
		resp := reportAppErrorRaw(t, baseURL, anna.token, map[string]any{"error": "Bad state"})
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("отчёт %d из %d: ожидался статус 204, получен %d", i+1, aeLimit, resp.StatusCode)
		}
	}

	requireErrorStatus(t, reportAppErrorRaw(t, baseURL, anna.token, map[string]any{"error": "Bad state"}),
		http.StatusTooManyRequests, "too_many_reports", "101-й отчёт за час от одного человека")
	if got := countAppErrors(t); got != aeLimit {
		t.Fatalf("отчёт сверх предела не записывается: ожидалось %d, найдено %d", aeLimit, got)
	}

	reportAppError(t, baseURL, boris.token, map[string]any{"error": "Bad state", "os": "борис"})
	reportAppError(t, baseURL, "", map[string]any{"error": "Bad state", "os": "без входа"})
}

// Отчёты старше часа в предел не считаются (ФТ-4).
func TestAppErrorReportLimitIsForLastHour(t *testing.T) {
	baseURL := startAPI(t)
	anna := newAppErrorUser(t, baseURL, 1)

	for i := 0; i < aeLimit; i++ {
		reportAppError(t, baseURL, anna.token, map[string]any{"error": "Bad state", "os": "старый"})
	}
	shiftAppErrors(t, "os", "старый", time.Now().Add(-61*time.Minute))

	reportAppError(t, baseURL, anna.token, map[string]any{"error": "Bad state", "os": "новый"})
}

// Отчёты без входа — не больше 100 за час на всех вместе; с
// недействительным токеном — тоже без входа (ФТ-3, ФТ-4). Вошедшим это
// не мешает.
func TestAppErrorReportLimitForAnonymous(t *testing.T) {
	baseURL := startAPI(t)
	anna := newAppErrorUser(t, baseURL, 1)

	for i := 0; i < aeLimit; i++ {
		token := ""
		if i%2 == 1 {
			token = fmt.Sprintf("bad-token-%d", i)
		}
		resp := reportAppErrorRaw(t, baseURL, token, map[string]any{"error": "Bad state"})
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("отчёт без входа %d из %d: ожидался статус 204, получен %d", i+1, aeLimit, resp.StatusCode)
		}
	}

	requireErrorStatus(t, reportAppErrorRaw(t, baseURL, "", map[string]any{"error": "Bad state"}),
		http.StatusTooManyRequests, "too_many_reports", "101-й отчёт без входа за час")
	requireErrorStatus(t, reportAppErrorRaw(t, baseURL, "another-bad-token", map[string]any{"error": "Bad state"}),
		http.StatusTooManyRequests, "too_many_reports", "101-й отчёт с недействительным токеном за час")
	if got := countAppErrors(t); got != aeLimit {
		t.Fatalf("отчёты сверх предела не записываются: ожидалось %d, найдено %d", aeLimit, got)
	}

	reportAppError(t, baseURL, anna.token, map[string]any{"error": "Bad state", "os": "анна"})

	execSQL(t, `UPDATE app_errors SET at = $1 WHERE user_id IS NULL`, time.Now().Add(-61*time.Minute).UTC())
	syncAppErrorGroups(t)
	reportAppError(t, baseURL, "", map[string]any{"error": "Bad state", "os": "через час"})
}

// --- Склейка --------------------------------------------------------------

// Одна ошибка — совпадают текст и первые 5 непустых строк стека; цифры
// не учитываются, пробелы по краям и подряд — как один (ФТ-5).
func TestAppErrorGrouping(t *testing.T) {
	base := "#0 PostScreen.build (package:moya_dacha/screens/post_screen.dart:120:5)\n" +
		"#1 StatelessElement.build (package:flutter/src/widgets/framework.dart:5.2:15)\n" +
		"#2 ComponentElement.performRebuild (package:flutter/src/widgets/framework.dart:5367:16)\n" +
		"#3 Element.rebuild (package:flutter/src/widgets/framework.dart:5053:7)\n" +
		"#4 BuildOwner.buildScope (package:flutter/src/widgets/framework.dart:2848:19)\n" +
		"#5 WidgetsBinding.drawFrame (package:flutter/src/widgets/binding.dart:1068:21)\n" +
		"#6 RendererBinding._handlePersistentFrameCallback (package:flutter/src/rendering/binding.dart:358:5)"

	// Те же строки с другими цифрами, лишними пробелами и пустыми строками.
	noisy := "\n   #0    PostScreen.build (package:moya_dacha/screens/post_screen.dart:131:9)   \n" +
		"\n" +
		"#1 StatelessElement.build  (package:flutter/src/widgets/framework.dart:77.3:1)\n" +
		"  #2 ComponentElement.performRebuild (package:flutter/src/widgets/framework.dart:1:1)\n" +
		"#3   Element.rebuild (package:flutter/src/widgets/framework.dart:5053:7)\n" +
		"#4 BuildOwner.buildScope (package:flutter/src/widgets/framework.dart:2848:19)\n" +
		"#5 WidgetsBinding.drawFrame (package:flutter/src/widgets/binding.dart:1068:21)\n" +
		"#6 RendererBinding._handlePersistentFrameCallback (package:flutter/src/rendering/binding.dart:358:5)"

	replaceLine := func(stack string, n int, line string) string {
		ls := strings.Split(stack, "\n")
		ls[n] = line
		return strings.Join(ls, "\n")
	}

	cases := []struct {
		name   string
		first  map[string]any
		second map[string]any
		groups int
	}{
		{
			name:   "тот же текст и стек",
			first:  map[string]any{"error": "Bad state: no element", "stack": base},
			second: map[string]any{"error": "Bad state: no element", "stack": base},
			groups: 1,
		},
		{
			name:   "другие цифры в тексте, цифры и пробелы в стеке, пустые строки",
			first:  map[string]any{"error": "RangeError (index): Invalid value: Not in inclusive range 0..2: 5", "stack": base},
			second: map[string]any{"error": "RangeError (index): Invalid value: Not in inclusive range 0..17: 42", "stack": noisy},
			groups: 1,
		},
		{
			name:   "другой текст",
			first:  map[string]any{"error": "Bad state: no element", "stack": base},
			second: map[string]any{"error": "Bad state: too many elements", "stack": base},
			groups: 2,
		},
		{
			name:   "другая третья строка стека",
			first:  map[string]any{"error": "Bad state: no element", "stack": base},
			second: map[string]any{"error": "Bad state: no element", "stack": replaceLine(base, 2, "#2 FeedScreen.build (package:moya_dacha/screens/feed_screen.dart:120:5)")},
			groups: 2,
		},
		{
			name:   "другая пятая строка стека",
			first:  map[string]any{"error": "Bad state: no element", "stack": base},
			second: map[string]any{"error": "Bad state: no element", "stack": replaceLine(base, 4, "#4 Other.frame (package:flutter/src/other.dart:1:1)")},
			groups: 2,
		},
		{
			name:   "другая шестая строка стека",
			first:  map[string]any{"error": "Bad state: no element", "stack": base},
			second: map[string]any{"error": "Bad state: no element", "stack": replaceLine(base, 5, "#5 Other.frame (package:flutter/src/other.dart:1:1)")},
			groups: 1,
		},
		{
			name:   "шестая строка стека есть только у одного",
			first:  map[string]any{"error": "Bad state: no element", "stack": strings.Join(strings.Split(base, "\n")[:5], "\n")},
			second: map[string]any{"error": "Bad state: no element", "stack": base},
			groups: 1,
		},
		{
			name:   "без стека и со стеком",
			first:  map[string]any{"error": "Bad state: no element"},
			second: map[string]any{"error": "Bad state: no element", "stack": base},
			groups: 2,
		},
		{
			name:   "без стека оба",
			first:  map[string]any{"error": "Bad state: no element"},
			second: map[string]any{"error": "Bad state: no element", "stack": ""},
			groups: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			baseURL := startAPI(t)

			reportAppError(t, baseURL, "", c.first)
			reportAppError(t, baseURL, "", c.second)

			if got := countAppErrors(t); got != 2 {
				t.Fatalf("ожидалось 2 отчёта в базе, найдено %d", got)
			}
			if got := countAppErrorGroups(t); got != c.groups {
				t.Fatalf("ожидалось групп: %d, найдено %d", c.groups, got)
			}
		})
	}
}

// --- Дашборд --------------------------------------------------------------

// На пустой базе app_errors есть: нули и groups — [] (ФТ-11–13).
func TestDashboardAppErrorsOnEmptyBase(t *testing.T) {
	_, root := startDashboardAPI(t, api.Config{})

	data, raw := dashboardAppErrors(t, root)
	for _, key := range []string{"last_day", "last_week", "new_week", "groups"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("в app_errors нет поля %q", key)
		}
	}
	if string(raw["groups"]) != "[]" {
		t.Errorf("групп нет — groups должен быть [], получено %s", raw["groups"])
	}
	if data.LastDay != 0 || data.LastWeek != 0 || data.NewWeek != 0 {
		t.Errorf("на пустой базе ожидались нули, получено %d/%d/%d", data.LastDay, data.LastWeek, data.NewWeek)
	}
}

// Счётчики за сутки и неделю, новые группы за неделю, порядок групп по
// последнему отчёту, поля группы по последнему отчёту, люди и отчёты без
// входа; отчёты старше 30 дней не видны (ФТ-6, ФТ-12, ФТ-13).
func TestDashboardAppErrorsCountsAndGroups(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})
	anna := newAppErrorUser(t, baseURL, 1)
	boris := newAppErrorUser(t, baseURL, 2)
	now := time.Now()

	stackOf := func(line int) string {
		return "#0 Future._propagateToListeners (dart:async/future_impl.dart:" + fmt.Sprint(line) + ":5)\n" +
			"   #1 PostScreen.build (package:moya_dacha/screens/post_screen.dart:" + fmt.Sprint(line) + ":5)   \n" +
			"#2 main (package:moya_dacha/main.dart:1:1)"
	}

	// Группа «один»: первый отчёт 10 суток назад, дальше 3 суток, 2 часа
	// (без входа) и час назад — последний.
	reportAppError(t, baseURL, anna.token, map[string]any{"error": "Ошибка один: индекс 1", "stack": stackOf(11),
		"version": "0.9.0", "build": 100, "screen": "feed", "os": "m1a"})
	reportAppError(t, baseURL, boris.token, map[string]any{"error": "Ошибка один: индекс 2", "stack": stackOf(22),
		"version": "0.9.5", "build": 150, "screen": "feed", "os": "m1b"})
	reportAppError(t, baseURL, "", map[string]any{"error": "Ошибка один: индекс 4", "stack": stackOf(44),
		"version": "1.0.0", "build": 200, "screen": "sign_in", "os": "m1d"})
	reportAppError(t, baseURL, anna.token, map[string]any{"error": "Ошибка один: индекс 3", "stack": stackOf(33),
		"version": "1.0.0", "build": 386900, "screen": "post", "os": "m1c"})

	// Группа «два»: один отчёт двое суток назад, без необязательных полей.
	reportAppError(t, baseURL, boris.token, map[string]any{"error": "Ошибка два"})

	// Группа «три»: один отчёт без входа пять минут назад.
	reportAppError(t, baseURL, "", map[string]any{"error": "Ошибка три", "os": "m3"})

	// Группа «старая»: единственный отчёт 40 суток назад — её не видно.
	reportAppError(t, baseURL, anna.token, map[string]any{"error": "Ошибка старая", "os": "m4"})

	at := map[string]time.Time{
		"m1a": now.Add(-10 * 24 * time.Hour),
		"m1b": now.Add(-3 * 24 * time.Hour),
		"m1d": now.Add(-2 * time.Hour),
		"m1c": now.Add(-time.Hour),
		"m3":  now.Add(-5 * time.Minute),
		"m4":  now.Add(-40 * 24 * time.Hour),
	}
	for os, when := range at {
		shiftAppErrors(t, "os", os, when)
	}
	twoDays := now.Add(-2 * 24 * time.Hour)
	shiftAppErrors(t, "error", "Ошибка два", twoDays)

	data, _ := dashboardAppErrors(t, root)

	// За сутки: m1d, m1c, m3. За неделю: ещё m1b и «два». Новых за неделю —
	// «два» и «три»: у «один» первый отчёт 10 суток назад.
	if data.LastDay != 3 {
		t.Errorf("last_day: ожидалось 3, получено %d", data.LastDay)
	}
	if data.LastWeek != 5 {
		t.Errorf("last_week: ожидалось 5, получено %d", data.LastWeek)
	}
	if data.NewWeek != 2 {
		t.Errorf("new_week: ожидалось 2, получено %d", data.NewWeek)
	}

	var order []string
	for _, g := range data.Groups {
		order = append(order, g.Error)
	}
	want := []string{"Ошибка три", "Ошибка один: индекс 3", "Ошибка два"}
	if strings.Join(order, " | ") != strings.Join(want, " | ") {
		t.Fatalf("группы по последнему отчёту от новых к старым, без групп старше 30 дней: ожидалось %q, получено %q", want, order)
	}

	one := data.Groups[1]
	if one.Where != "#1 PostScreen.build (package:moya_dacha/screens/post_screen.dart:33:5)" {
		t.Errorf("where — первая строка стека с package:moya_dacha/ последнего отчёта без пробелов по краям, получено %q", one.Where)
	}
	if one.Stack != stackOf(33) {
		t.Errorf("stack — стек последнего отчёта: ожидалось %q, получено %q", stackOf(33), one.Stack)
	}
	if one.Reports != 4 || one.Users != 2 || one.Anonymous != 1 {
		t.Errorf("группа «один»: ожидалось reports=4 users=2 anonymous=1, получено reports=%d users=%d anonymous=%d",
			one.Reports, one.Users, one.Anonymous)
	}
	requireNear(t, one.FirstAt, at["m1a"], "first_at группы «один»")
	requireNear(t, one.LastAt, at["m1c"], "last_at группы «один»")
	if one.Version != "1.0.0" || one.Build != 386900 || one.Screen != "post" || one.OS != "m1c" {
		t.Errorf("version/build/screen/os — последнего отчёта: ожидалось 1.0.0/386900/post/m1c, получено %s/%d/%s/%s",
			one.Version, one.Build, one.Screen, one.OS)
	}

	two := data.Groups[2]
	if two.Where != "" || two.Stack != "" || two.Version != "" || two.Build != 0 || two.Screen != "" || two.OS != "" {
		t.Errorf("у отчёта без необязательных полей — пустые строки и 0, получено %+v", two)
	}
	if two.Reports != 1 || two.Users != 1 || two.Anonymous != 0 {
		t.Errorf("группа «два»: ожидалось reports=1 users=1 anonymous=0, получено %+v", two)
	}
	requireNear(t, two.FirstAt, twoDays, "first_at группы «два»")
	requireNear(t, two.LastAt, twoDays, "last_at группы «два»")

	three := data.Groups[0]
	if three.Reports != 1 || three.Users != 0 || three.Anonymous != 1 {
		t.Errorf("группа «три»: ожидалось reports=1 users=0 anonymous=1, получено %+v", three)
	}
}

// Отчёты старше 30 дней не входят в reports, а группа с такими и свежими
// отчётами видна (ФТ-13).
func TestDashboardAppErrorsCountsReportsForThirtyDays(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	reportAppError(t, baseURL, "", map[string]any{"error": "Bad state", "os": "старый"})
	reportAppError(t, baseURL, "", map[string]any{"error": "Bad state", "os": "свежий"})
	shiftAppErrors(t, "os", "старый", time.Now().Add(-31*24*time.Hour))

	data, _ := dashboardAppErrors(t, root)
	if len(data.Groups) != 1 {
		t.Fatalf("ожидалась одна группа, получено %+v", data.Groups)
	}
	if data.Groups[0].Reports != 1 || data.Groups[0].Anonymous != 1 {
		t.Errorf("отчёт старше 30 дней не считается: ожидалось reports=1 anonymous=1, получено %+v", data.Groups[0])
	}
}

// where — первая строка с package:moya_dacha/, иначе первая непустая;
// stack — первые 30 строк (ФТ-8, ФТ-13).
func TestDashboardAppErrorsWhereAndStack(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	long := frames(40)
	reportAppError(t, baseURL, "", map[string]any{"error": "Длинный стек", "stack": long})

	foreign := "\n\n   #0 Future._complete (dart:async/future_impl.dart:12:3)  \n#1 Zone.run (dart:async/zone.dart:1:1)"
	reportAppError(t, baseURL, "", map[string]any{"error": "Чужой стек", "stack": foreign})
	shiftAppErrors(t, "error", "Длинный стек", time.Now().Add(-time.Minute))

	data, _ := dashboardAppErrors(t, root)
	if len(data.Groups) != 2 {
		t.Fatalf("ожидалось две группы, получено %+v", data.Groups)
	}

	other, lng := data.Groups[0], data.Groups[1]
	if other.Error != "Чужой стек" || lng.Error != "Длинный стек" {
		t.Fatalf("ожидался порядок «Чужой стек», «Длинный стек», получено %q, %q", other.Error, lng.Error)
	}
	if other.Where != "#0 Future._complete (dart:async/future_impl.dart:12:3)" {
		t.Errorf("нет строки с package:moya_dacha/ — where первая непустая строка без пробелов по краям, получено %q", other.Where)
	}

	wantStack := strings.Join(strings.Split(long, "\n")[:30], "\n")
	if strings.TrimRight(lng.Stack, "\n") != wantStack {
		t.Errorf("stack — первые 30 строк: ожидалось %d строк, получено %d:\n%s",
			30, len(strings.Split(strings.TrimRight(lng.Stack, "\n"), "\n")), lng.Stack)
	}
	if lng.Where != strings.Split(long, "\n")[0] {
		t.Errorf("where — первая строка с package:moya_dacha/, получено %q", lng.Where)
	}
}

// Групп на дашборде — не больше 20, самые свежие (ФТ-13).
func TestDashboardAppErrorsShowsTwentyGroups(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})
	now := time.Now()

	for i := 0; i < 22; i++ {
		text := "Ошибка " + letters(i)
		reportAppError(t, baseURL, "", map[string]any{"error": text})
		shiftAppErrors(t, "error", text, now.Add(-time.Duration(30-i)*time.Minute))
	}

	data, _ := dashboardAppErrors(t, root)
	if len(data.Groups) != 20 {
		t.Fatalf("ожидалось 20 групп, получено %d", len(data.Groups))
	}
	for i, g := range data.Groups {
		want := "Ошибка " + letters(21-i)
		if g.Error != want {
			t.Errorf("группа %d: ожидалась %q, получена %q", i, want, g.Error)
		}
	}
	if data.NewWeek != 22 || data.LastDay != 22 {
		t.Errorf("счётчики считают все группы и отчёты, а не только показанные: ожидалось new_week=22 last_day=22, получено %d/%d",
			data.NewWeek, data.LastDay)
	}
}

// --- Бот ------------------------------------------------------------------

// appErrorBot поднимает сервис и бота на одной чистой базе. Личка
// владельца не привязана.
func appErrorBot(t *testing.T) (baseURL string, bot *telegram.Bot, fake *fakeTelegram) {
	t.Helper()

	baseURL = startAPI(t) // чистит базу
	fake = newFakeTelegram(t)
	bot = telegram.New(connect(t), tgConfig(t, fake))

	return baseURL, bot, fake
}

// checkAppErrors — одна проверка бота; ошибки быть не должно.
func checkAppErrors(t *testing.T, bot *telegram.Bot, step string) {
	t.Helper()

	if err := bot.CheckAppErrors(context.Background()); err != nil {
		t.Fatalf("%s: CheckAppErrors: %v", step, err)
	}
}

// requireLines требует, чтобы сообщение было sendMessage ровно из этих строк.
func requireLines(t *testing.T, s tgSent, want ...string) {
	t.Helper()

	if s.method != "sendMessage" {
		t.Fatalf("ожидалось sendMessage, получено %s", describeSent([]tgSent{s}))
	}
	got := lines(s.text)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("сообщение о новой ошибке:\nожидалось:\n%s\nполучено:\n%s", strings.Join(want, "\n"), s.text)
	}
}

// Сообщение о новой ошибке в точном формате: строки из первого отчёта
// группы, «Где» — строка с package:moya_dacha/, ник вошедшего (ФТ-8).
func TestAppErrorBotMessageFormat(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)
	anna := newAppErrorUser(t, baseURL, 1)
	boris := newAppErrorUser(t, baseURL, 2)
	chooseNickname(t, baseURL, anna.token, "Anya_Test")
	chooseNickname(t, baseURL, boris.token, "Boris_Test")

	stack := "#0 Object.noSuchMethod (dart:core-patch/object_patch.dart:38:5)\n" +
		"   #1 PostScreen.build (package:moya_dacha/screens/post_screen.dart:120:5)  \n" +
		"#2 main (package:moya_dacha/main.dart:1:1)"
	reportAppError(t, baseURL, anna.token, map[string]any{
		"error": "Null check operator used on a null value", "stack": stack,
		"version": "1.0.0", "build": 386900, "screen": "post", "os": "android 14",
	})
	// Второй отчёт той же ошибки до проверки — сообщение всё равно по первому.
	reportAppError(t, baseURL, boris.token, map[string]any{
		"error": "Null check operator used on a null value", "stack": strings.ReplaceAll(stack, "120", "99"),
		"version": "1.0.1", "build": 386999, "screen": "feed", "os": "android 13",
	})

	checkAppErrors(t, bot, "первая проверка")
	got := fake.sentTo(tgOwnerID)
	if len(got) != 1 {
		t.Fatalf("ожидалось одно сообщение владельцу, получено: %s", describeSent(fake.allSent()))
	}
	requireLines(t, got[0],
		aeHeader,
		"Null check operator used on a null value",
		"Где: #1 PostScreen.build (package:moya_dacha/screens/post_screen.dart:120:5)",
		"Сборка 1.0.0 (386900), экран post",
		"Кто: @Anya_Test",
	)
	if other := len(fake.allSent()) - len(got); other != 0 {
		t.Errorf("сообщения о новых ошибках уходят только в личку владельца, а ушло ещё %d", other)
	}
}

// Без стека, без версии и экрана: «Где» нет, «Сборка ?» без номера (ФТ-8).
func TestAppErrorBotMessageWithoutOptionalFields(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)
	token := aeTester(t, baseURL)

	reportAppError(t, baseURL, token, map[string]any{"error": "Ошибка без подробностей"})
	checkAppErrors(t, bot, "проверка")

	got := fake.sentTo(tgOwnerID)
	if len(got) != 1 {
		t.Fatalf("ожидалось одно сообщение, получено: %s", describeSent(got))
	}
	requireLines(t, got[0], aeHeader, "Ошибка без подробностей", "Сборка ?", aeWho)
}

// Нет версии, но есть экран — «Сборка ?, экран …»; нет экрана, но есть
// версия — без «, экран …»; стек без package:moya_dacha/ — «Где» по первой
// непустой строке (ФТ-8).
func TestAppErrorBotMessageBuildLineAndForeignStack(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)
	token := aeTester(t, baseURL)
	now := time.Now()

	reportAppError(t, baseURL, token, map[string]any{
		"error": "Ошибка а", "screen": "sign_in", "build": 5,
		"stack": "\n  #0 Zone.run (dart:async/zone.dart:1:1)  \n#1 Timer._run (dart:async/timer.dart:2:2)",
	})
	reportAppError(t, baseURL, token, map[string]any{"error": "Ошибка б", "version": "1.2.3", "build": 7})
	shiftAppErrors(t, "error", "Ошибка а", now.Add(-2*time.Minute))
	shiftAppErrors(t, "error", "Ошибка б", now.Add(-time.Minute))

	checkAppErrors(t, bot, "проверка")
	got := fake.sentTo(tgOwnerID)
	if len(got) != 2 {
		t.Fatalf("ожидалось два сообщения, получено: %s", describeSent(got))
	}
	requireLines(t, got[0], aeHeader, "Ошибка а", "Где: #0 Zone.run (dart:async/zone.dart:1:1)", "Сборка ?, экран sign_in", aeWho)
	requireLines(t, got[1], aeHeader, "Ошибка б", "Сборка 1.2.3 (7)", aeWho)
}

// Текст ошибки в сообщении — не длиннее 500 символов, без многоточия (ФТ-8).
func TestAppErrorBotMessageCutsLongError(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)

	long := strings.Repeat("я", 300) + strings.Repeat("z", 300)
	reportAppError(t, baseURL, aeTester(t, baseURL), map[string]any{"error": long})
	checkAppErrors(t, bot, "проверка")

	got := fake.sentTo(tgOwnerID)
	if len(got) != 1 {
		t.Fatalf("ожидалось одно сообщение, получено: %s", describeSent(got))
	}
	want := string([]rune(long)[:500])
	requireLines(t, got[0], aeHeader, want, "Сборка ?", aeWho)
}

// О группе пишется один раз: повторная проверка, та же ошибка ещё раз и
// перезапуск бота повторов не дают; группа помечена в базе (ФТ-9).
func TestAppErrorBotNotifiesOnce(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)
	token := aeTester(t, baseURL)

	reportAppError(t, baseURL, token, map[string]any{"error": "Bad state: 1"})
	checkAppErrors(t, bot, "первая проверка")
	if got := fake.sentTo(tgOwnerID); len(got) != 1 {
		t.Fatalf("о новой ошибке ожидалось одно сообщение, получено: %s", describeSent(got))
	}
	if got := notifiedGroups(t); got != 1 {
		t.Fatalf("после отправки группа должна быть помечена, помечено %d", got)
	}

	checkAppErrors(t, bot, "вторая проверка")
	reportAppError(t, baseURL, token, map[string]any{"error": "Bad state: 2"})
	checkAppErrors(t, bot, "та же ошибка ещё раз")

	restarted := telegram.New(connect(t), tgConfig(t, fake))
	checkAppErrors(t, restarted, "после перезапуска")

	if got := fake.allSent(); len(got) != 1 {
		t.Fatalf("о группе пишется один раз, а ушло: %s", describeSent(got))
	}
	if got := countAppErrorGroups(t); got != 1 {
		t.Fatalf("та же ошибка должна попасть в ту же группу, групп %d", got)
	}

	// Новая ошибка после этого — снова сообщение.
	reportAppError(t, baseURL, token, map[string]any{"error": "Другая ошибка"})
	checkAppErrors(t, restarted, "новая ошибка")
	got := fake.sentTo(tgOwnerID)
	if len(got) != 2 {
		t.Fatalf("о новой ошибке ожидалось сообщение, всего получено: %s", describeSent(got))
	}
	requireLines(t, got[1], aeHeader, "Другая ошибка", "Сборка ?", aeWho)
}

// Не больше 5 сообщений за проверку, от старых к новым; остальные — на
// следующей (ФТ-8).
func TestAppErrorBotSendsFivePerCheckOldestFirst(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)
	token := aeTester(t, baseURL)
	now := time.Now()

	// Отправляются в обратном порядке, а время — от старой «a» к новой «g».
	for i := 6; i >= 0; i-- {
		text := "Ошибка " + letters(i)
		reportAppError(t, baseURL, token, map[string]any{"error": text})
		shiftAppErrors(t, "error", text, now.Add(-time.Duration(70-i)*time.Minute))
	}

	errorLine := func(s tgSent) string {
		ls := lines(s.text)
		if len(ls) < 2 {
			return ""
		}
		return ls[1]
	}

	checkAppErrors(t, bot, "первая проверка")
	got := fake.sentTo(tgOwnerID)
	if len(got) != 5 {
		t.Fatalf("за проверку — не больше 5 сообщений, получено %d: %s", len(got), describeSent(got))
	}
	for i, s := range got {
		if want := "Ошибка " + letters(i); errorLine(s) != want {
			t.Errorf("сообщение %d: ожидалась %q (от старых к новым), получено %s", i, want, describeSent([]tgSent{s}))
		}
	}

	checkAppErrors(t, bot, "вторая проверка")
	got = fake.sentTo(tgOwnerID)
	if len(got) != 7 {
		t.Fatalf("остальные 2 уходят на следующей проверке, всего получено %d: %s", len(got), describeSent(got))
	}
	for i := 5; i < 7; i++ {
		if want := "Ошибка " + letters(i); errorLine(got[i]) != want {
			t.Errorf("сообщение %d: ожидалась %q, получено %s", i, want, describeSent(got[i:i+1]))
		}
	}

	checkAppErrors(t, bot, "третья проверка")
	if got := fake.sentTo(tgOwnerID); len(got) != 7 {
		t.Fatalf("новых групп нет — ничего не уходит, всего получено %d", len(got))
	}
}

// Группа, чей первый отчёт старше суток, не отправляется никогда, даже
// если отчёты по ней идут и сейчас (ФТ-8, ФТ-9).
func TestAppErrorBotSkipsGroupsOlderThanDay(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)
	token := aeTester(t, baseURL)

	reportAppError(t, baseURL, token, map[string]any{"error": "Старая ошибка", "os": "давно"})
	shiftAppErrors(t, "os", "давно", time.Now().Add(-25*time.Hour))
	reportAppError(t, baseURL, token, map[string]any{"error": "Старая ошибка", "os": "сейчас"})
	reportAppError(t, baseURL, token, map[string]any{"error": "Свежая ошибка"})
	if got := countAppErrorGroups(t); got != 2 {
		t.Fatalf("ожидалось две группы, найдено %d", got)
	}

	checkAppErrors(t, bot, "первая проверка")
	checkAppErrors(t, bot, "вторая проверка")

	got := fake.sentTo(tgOwnerID)
	if len(got) != 1 {
		t.Fatalf("ожидалось одно сообщение — о свежей ошибке, получено: %s", describeSent(got))
	}
	requireLines(t, got[0], aeHeader, "Свежая ошибка", "Сборка ?", aeWho)
}

// Личка не привязана — ничего не уходит и ничего не помечается; после
// привязки сообщение приходит на следующей проверке (ФТ-10). Привязанная
// группа тестировщиков сообщений не получает.
func TestAppErrorBotWaitsForOwnerChat(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "group", tgGroupID)

	reportAppError(t, baseURL, aeTester(t, baseURL), map[string]any{"error": "Ошибка без лички"})

	if err := bot.CheckAppErrors(context.Background()); err != nil {
		t.Fatalf("CheckAppErrors без привязанной лички не должен падать: %v", err)
	}
	if got := fake.allSent(); len(got) != 0 {
		t.Fatalf("личка не привязана — ничего не должно уходить, а ушло: %s", describeSent(got))
	}
	if got := notifiedGroups(t); got != 0 {
		t.Fatalf("личка не привязана — группа не помечается, помечено %d", got)
	}

	bindChat(t, "owner", tgOwnerID)
	checkAppErrors(t, bot, "после привязки")
	got := fake.sentTo(tgOwnerID)
	if len(got) != 1 {
		t.Fatalf("после привязки лички сообщение должно прийти, получено: %s", describeSent(fake.allSent()))
	}
	requireLines(t, got[0], aeHeader, "Ошибка без лички", "Сборка ?", aeWho)
	if g := fake.sentTo(tgGroupID); len(g) != 0 {
		t.Errorf("в группу сообщения о новых ошибках не уходят, а ушло: %s", describeSent(g))
	}
}

// Бот не настроен (нет токена) — ничего не уходит и ничего не помечается;
// настроенный бот потом отправляет (ФТ-10).
func TestAppErrorBotNotConfigured(t *testing.T) {
	baseURL, _, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)

	reportAppError(t, baseURL, "", map[string]any{"error": "Ошибка без бота"})

	cfg := tgConfig(t, fake)
	cfg.Token = ""
	unconfigured := telegram.New(connect(t), cfg)
	// Вернёт ли CheckAppErrors ошибку, спецификация не говорит: важно,
	// что ничего не ушло и ничего не помечено.
	_ = unconfigured.CheckAppErrors(context.Background())

	if got := fake.allSent(); len(got) != 0 {
		t.Fatalf("бот не настроен — ничего не должно уходить, а ушло: %s", describeSent(got))
	}
	if got := notifiedGroups(t); got != 0 {
		t.Fatalf("бот не настроен — группа не помечается, помечено %d", got)
	}

	configured := telegram.New(connect(t), tgConfig(t, fake))
	checkAppErrors(t, configured, "настроенный бот")
	if got := fake.sentTo(tgOwnerID); len(got) != 1 {
		t.Fatalf("настроенный бот должен отправить ошибку, получено: %s", describeSent(fake.allSent()))
	}
}

// requireNoAnonymousText — текст ошибок без входа владельцу в личку не
// уходит ни в каком виде (ФТ-8а).
func requireNoAnonymousText(t *testing.T, fake *fakeTelegram, texts ...string) {
	t.Helper()
	for _, s := range fake.allSent() {
		for _, text := range texts {
			if strings.Contains(s.text, text) {
				t.Fatalf("текст ошибки без входа %q ушёл в Telegram: %s", text, describeSent([]tgSent{s}))
			}
		}
	}
}

// countMessages — сколько сообщений ровно с этим текстом и сколько
// отдельных сообщений «Новая ошибка в приложении».
func countMessages(list []tgSent, text string) (exact, single int) {
	for _, s := range list {
		switch {
		case strings.TrimSpace(s.text) == text:
			exact++
		case strings.HasPrefix(strings.TrimSpace(s.text), aeHeader):
			single++
		}
	}
	return exact, single
}

// Группы, чей первый отчёт без входа, отдельными сообщениями не уходят:
// за проверку — одно сообщение со счётчиком, без текста ошибок; лимит 5
// их не касается и места у групп с входом не занимает; все они
// помечаются, повторов нет. Группы старше суток в счёт не идут (ФТ-8а).
func TestAppErrorBotAnonymousGroupsAsOneCounter(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)
	token := aeTester(t, baseURL)
	now := time.Now()

	var anonymous []string
	for i := 0; i < 7; i++ {
		text := "Аноним " + letters(i)
		anonymous = append(anonymous, text)
		reportAppError(t, baseURL, "", map[string]any{"error": text})
		shiftAppErrors(t, "error", text, now.Add(-time.Duration(90-i)*time.Minute))
	}
	for i := 0; i < 6; i++ {
		text := "Вошедшая " + letters(i)
		reportAppError(t, baseURL, token, map[string]any{"error": text})
		shiftAppErrors(t, "error", text, now.Add(-time.Duration(60-i)*time.Minute))
	}
	// Без входа, но старше суток — не новая, в счёт не идёт (ФТ-9).
	reportAppError(t, baseURL, "", map[string]any{"error": "Давний аноним"})
	shiftAppErrors(t, "error", "Давний аноним", now.Add(-25*time.Hour))

	checkAppErrors(t, bot, "первая проверка")
	got := fake.sentTo(tgOwnerID)
	counters, singles := countMessages(got, aeAnonymousCount(7))
	if counters != 1 || singles != 5 || len(got) != 6 {
		t.Fatalf("первая проверка: ожидались 5 сообщений об ошибках с входом и одно %q, получено: %s",
			aeAnonymousCount(7), describeSent(got))
	}
	for _, s := range got {
		if strings.HasPrefix(strings.TrimSpace(s.text), aeHeader) && indexOfLine(lines(s.text), aeWho) < 0 {
			t.Fatalf("отдельным сообщением уходят только ошибки с входом: %s", describeSent([]tgSent{s}))
		}
	}
	requireNoAnonymousText(t, fake, append(anonymous, "Давний аноним")...)
	if n := notifiedGroups(t); n != 5+7 {
		t.Fatalf("после первой проверки помечены должны быть 5 групп с входом и все 7 без входа, помечено %d", n)
	}

	checkAppErrors(t, bot, "вторая проверка")
	got = fake.sentTo(tgOwnerID)
	if len(got) != 7 {
		t.Fatalf("вторая проверка: ожидалось одно сообщение — шестая ошибка с входом, всего получено: %s", describeSent(got))
	}
	requireLines(t, got[6], aeHeader, "Вошедшая "+letters(5), "Сборка ?", aeWho)

	checkAppErrors(t, bot, "третья проверка")
	if got := fake.sentTo(tgOwnerID); len(got) != 7 {
		t.Fatalf("новых групп нет — ничего не уходит, всего получено: %s", describeSent(got))
	}
	if n := notifiedGroups(t); n != 6+7 {
		t.Fatalf("давняя группа не помечается, остальные помечены: ожидалось 13, помечено %d", n)
	}
}

// Без входа или с входом — по первому отчёту группы: группа, начатая
// без входа, остаётся в счётчике, даже если потом её прислала вошедшая;
// начатая вошедшей уходит отдельным сообщением с её ником. Новых ошибок
// без входа нет — сообщения со счётчиком нет (ФТ-8, ФТ-8а).
func TestAppErrorBotAnonymousByFirstReport(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)
	token := aeTester(t, baseURL)
	now := time.Now()

	reportAppError(t, baseURL, "", map[string]any{"error": "Начата без входа", "os": "аноним первый"})
	shiftAppErrors(t, "os", "аноним первый", now.Add(-10*time.Minute))
	reportAppError(t, baseURL, token, map[string]any{"error": "Начата без входа", "os": "вошедшая потом"})

	reportAppError(t, baseURL, token, map[string]any{"error": "Начата вошедшей", "os": "вошедшая первой"})
	shiftAppErrors(t, "os", "вошедшая первой", now.Add(-10*time.Minute))
	reportAppError(t, baseURL, "", map[string]any{"error": "Начата вошедшей", "os": "аноним потом"})

	if n := countAppErrorGroups(t); n != 2 {
		t.Fatalf("ожидалось две группы, найдено %d", n)
	}

	checkAppErrors(t, bot, "проверка")
	got := fake.sentTo(tgOwnerID)
	counters, singles := countMessages(got, aeAnonymousCount(1))
	if counters != 1 || singles != 1 || len(got) != 2 {
		t.Fatalf("ожидались одно сообщение об ошибке вошедшей и одно %q, получено: %s", aeAnonymousCount(1), describeSent(got))
	}
	for _, s := range got {
		if strings.HasPrefix(strings.TrimSpace(s.text), aeHeader) {
			requireLines(t, s, aeHeader, "Начата вошедшей", "Сборка ?", aeWho)
		}
	}
	requireNoAnonymousText(t, fake, "Начата без входа")
	if n := notifiedGroups(t); n != 2 {
		t.Fatalf("обе группы должны быть помечены, помечено %d", n)
	}

	// Только ошибка с входом — счётчика нет.
	reportAppError(t, baseURL, token, map[string]any{"error": "Ещё одна вошедшей"})
	checkAppErrors(t, bot, "вторая проверка")
	got = fake.sentTo(tgOwnerID)
	if len(got) != 3 {
		t.Fatalf("новых ошибок без входа нет — ожидалось одно новое сообщение, всего получено: %s", describeSent(got))
	}
	requireLines(t, got[2], aeHeader, "Ещё одна вошедшей", "Сборка ?", aeWho)
}

// Telegram ответил ошибкой на счётчик — ни одна группа без входа не
// помечается, и счётчик со всеми ними уходит на следующей проверке
// (ФТ-8а, ФТ-10).
func TestAppErrorBotAnonymousCounterTelegramError(t *testing.T) {
	baseURL, bot, fake := appErrorBot(t)
	bindChat(t, "owner", tgOwnerID)

	reportAppError(t, baseURL, "", map[string]any{"error": "Первая без входа"})
	reportAppError(t, baseURL, "", map[string]any{"error": "Вторая без входа"})

	fake.setFailSend(true)
	// Вернёт ли проверка ошибку, спецификация не говорит: важно, что
	// ничего не помечено.
	_ = bot.CheckAppErrors(context.Background())
	if n := notifiedGroups(t); n != 0 {
		t.Fatalf("Telegram ответил ошибкой — группы без входа не помечаются, помечено %d", n)
	}

	fake.setFailSend(false)
	checkAppErrors(t, bot, "проверка после сбоя")
	got := fake.sentTo(tgOwnerID)
	if len(got) != 1 || strings.TrimSpace(got[0].text) != aeAnonymousCount(2) {
		t.Fatalf("после сбоя ожидалось одно сообщение %q, получено: %s", aeAnonymousCount(2), describeSent(got))
	}
	if n := notifiedGroups(t); n != 2 {
		t.Fatalf("после отправки обе группы без входа помечены, помечено %d", n)
	}
	requireNoAnonymousText(t, fake, "Первая без входа", "Вторая без входа")
}

// aeTester — вошедшая тестировщица с ником aeNick. Отдельным сообщением
// уходят только группы, чей первый отчёт с входом (ФТ-8а), поэтому
// проверки формата сообщения шлют отчёты от неё.
func aeTester(t *testing.T, baseURL string) string {
	t.Helper()

	u := newAppErrorUser(t, baseURL, 50)
	chooseNickname(t, baseURL, u.token, aeNick)
	return u.token
}

// appErrorUser — вошедший участник тестов ошибок приложения.
type appErrorUser struct {
	token string
	id    string
}

func newAppErrorUser(t *testing.T, baseURL string, n int) appErrorUser {
	t.Helper()

	token, id := signIn(t, baseURL, appErrorPhone(n))

	return appErrorUser{token: token, id: id}
}
