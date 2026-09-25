package tests

// Тесты дашборда владельца (specs/016-dashboard.md).
//
// Дашборд — не часть контракта приложения: адреса /dashboard и
// /dashboard/data висят рядом с /api, а форма ответа описана в самой
// спецификации, раздел «API / контракт данных». Измерения машины и записи
// об ошибках сервис копит сам, раз в минуту и по мере ошибок; тест не может
// ждать минутами, поэтому готовит их прямо в таблицах server_samples
// и server_errors — колонки взяты из раздела «Модель данных».

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
	"github.com/gmu-msk/moya_dacha/backend/internal/media"
)

// dashboardPassword — пароль дашборда в тестах (ФТ-1).
const dashboardPassword = "correct-horse-battery-staple-42"

// moscow — часовой пояс, по которому считаются дни активности (ФТ-10).
// Перехода на летнее время в Москве нет с 2014 года, поэтому фиксированное
// смещение совпадает с Europe/Moscow и не требует базы часовых поясов.
var moscow = time.FixedZone("MSK", 3*60*60)

// --- Форма ответа ---------------------------------------------------------

type dashAlert struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type dashTotals struct {
	Users    int `json:"users"`
	Posts    int `json:"posts"`
	Likes    int `json:"likes"`
	Comments int `json:"comments"`
	Reports  int `json:"reports"`
}

type dashDay struct {
	Date        string `json:"date"`
	NewUsers    int    `json:"new_users"`
	ActiveUsers int    `json:"active_users"`
	Posts       int    `json:"posts"`
	Likes       int    `json:"likes"`
	Comments    int    `json:"comments"`
}

type dashNow struct {
	MeasuredAt       string  `json:"measured_at"`
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryUsedBytes  float64 `json:"memory_used_bytes"`
	MemoryTotalBytes float64 `json:"memory_total_bytes"`
	SwapUsedBytes    float64 `json:"swap_used_bytes"`
	DiskUsedBytes    float64 `json:"disk_used_bytes"`
	DiskTotalBytes   float64 `json:"disk_total_bytes"`
	DBSizeBytes      float64 `json:"db_size_bytes"`
	UptimeSeconds    float64 `json:"uptime_seconds"`
}

type dashStep struct {
	At              string  `json:"at"`
	CPUPercent      float64 `json:"cpu_percent"`
	MemoryUsedBytes float64 `json:"memory_used_bytes"`
	DiskUsedBytes   float64 `json:"disk_used_bytes"`
	Requests        int     `json:"requests"`
	Errors          int     `json:"errors"`
}

type dashError struct {
	At      string `json:"at"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type dashboardPayload struct {
	GeneratedAt string      `json:"generated_at"`
	Alerts      []dashAlert `json:"alerts"`
	Totals      dashTotals  `json:"totals"`
	Activity    []dashDay   `json:"activity"`
	Server      struct {
		Now     *dashNow   `json:"now"`
		History []dashStep `json:"history"`
	} `json:"server"`
	Errors struct {
		LastHour int         `json:"last_hour"`
		LastDay  int         `json:"last_day"`
		Recent   []dashError `json:"recent"`
	} `json:"errors"`
}

// --- Хелперы --------------------------------------------------------------

// startDashboardAPI поднимает сервис с включённым дашбордом и возвращает
// адрес /api и корень сервиса, от которого считается /dashboard.
func startDashboardAPI(t *testing.T, cfg api.Config) (baseURL, root string) {
	t.Helper()

	cfg.DashboardPassword = dashboardPassword
	baseURL = startAPIWith(t, cfg)

	return baseURL, strings.TrimSuffix(baseURL, "/api")
}

// dashboardRequest открывает адрес дашборда. user и password — HTTP Basic
// (не ставится, если оба пусты), bearer — токен приложения (не ставится,
// если пуст).
func dashboardRequest(t *testing.T, address, user, password, bearer string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		t.Fatalf("не удалось собрать запрос %s: %v", address, err)
	}
	if user != "" || password != "" {
		req.SetBasicAuth(user, password)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("запрос %s не прошёл: %v", address, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// dashboardRaw читает /dashboard/data с верным паролем и требует 200 с JSON.
func dashboardRaw(t *testing.T, root string) []byte {
	t.Helper()

	resp := dashboardRequest(t, root+"/dashboard/data", "владелец", dashboardPassword, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на /dashboard/data с верным паролем ожидался статус 200, получен %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("данные дашборда должны быть application/json, получено %q", ct)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("не удалось прочитать данные дашборда: %v", err)
	}

	return raw
}

// dashboardData читает и разбирает /dashboard/data.
func dashboardData(t *testing.T, root string) dashboardPayload {
	t.Helper()

	var body dashboardPayload
	if err := json.Unmarshal(dashboardRaw(t, root), &body); err != nil {
		t.Fatalf("данные дашборда не разобрались как JSON: %v", err)
	}

	return body
}

// waitDashboard опрашивает /dashboard/data, пока ok не вернёт true, но не
// дольше двух секунд: ошибка может записываться в базу уже после ответа
// клиенту. Возвращает последние прочитанные данные и итог ожидания.
func waitDashboard(t *testing.T, root string, ok func(dashboardPayload) bool) (dashboardPayload, bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		data := dashboardData(t, root)
		if ok(data) {
			return data, true
		}
		if time.Now().After(deadline) {
			return data, false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// moscowDate — день по Москве в формате ГГГГ-ММ-ДД.
func moscowDate(at time.Time) string {
	return at.In(moscow).Format("2006-01-02")
}

// startOfMoscowDay — полночь по Москве того дня, в который попадает at.
func startOfMoscowDay(at time.Time) time.Time {
	local := at.In(moscow)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, moscow)
}

// parseDashTime разбирает время из ответа дашборда.
func parseDashTime(t *testing.T, value, what string) time.Time {
	t.Helper()

	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatalf("%s: время %q не в формате RFC 3339: %v", what, value, err)
	}

	return at
}

// requireThirtyDays требует ровно 30 дней подряд от старого к новому,
// последний — сегодня по Москве (ФТ-10). before и after — моменты до и
// после запроса: около полуночи «сегодня» могло смениться.
func requireThirtyDays(t *testing.T, days []dashDay, before, after time.Time) {
	t.Helper()

	if len(days) != 30 {
		t.Fatalf("в activity ожидалось ровно 30 дней, получено %d", len(days))
	}

	last := days[len(days)-1].Date
	if last != moscowDate(before) && last != moscowDate(after) {
		t.Fatalf("последний день activity %q, а сегодня по Москве %q", last, moscowDate(after))
	}

	lastDay, err := time.ParseInLocation("2006-01-02", last, moscow)
	if err != nil {
		t.Fatalf("дата %q не в формате ГГГГ-ММ-ДД: %v", last, err)
	}
	for i, day := range days {
		want := lastDay.AddDate(0, 0, i-(len(days)-1)).Format("2006-01-02")
		if day.Date != want {
			t.Fatalf("день %d в activity: ожидалась дата %q, получена %q — дни должны идти подряд от старого к новому", i, want, day.Date)
		}
	}
}

// dashboardPhone — номер n-го участника теста дашборда. Номера не
// пересекаются с номерами других тестов пакета.
func dashboardPhone(n int) string {
	return fmt.Sprintf("+7 (900) 816-00-%02d", n)
}

// insertSample кладёт измерение машины в server_samples.
func insertSample(t *testing.T, s sampleRow) {
	t.Helper()

	execSQL(t, `
		INSERT INTO server_samples (measured_at, cpu_percent, memory_used_bytes, memory_total_bytes,
			swap_used_bytes, disk_used_bytes, disk_total_bytes, requests)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		s.at.UTC().Truncate(time.Microsecond), s.cpu, s.memUsed, s.memTotal, s.swap, s.diskUsed, s.diskTotal, s.requests)
}

// sampleRow — строка server_samples.
type sampleRow struct {
	at        time.Time
	cpu       float64
	memUsed   int64
	memTotal  int64
	swap      int64
	diskUsed  int64
	diskTotal int64
	requests  int
}

// calmSample — измерение спокойной машины: ни одна тревога от него
// не поднимается.
func calmSample(at time.Time) sampleRow {
	return sampleRow{
		at:        at,
		cpu:       5,
		memUsed:   800_000_000,
		memTotal:  2_000_000_000,
		swap:      0,
		diskUsed:  5_000_000_000,
		diskTotal: 20_000_000_000,
		requests:  1,
	}
}

// insertServerError кладёт запись об ошибке сервера в server_errors.
func insertServerError(t *testing.T, at time.Time, method, path string, status int, message string) {
	t.Helper()

	execSQL(t, `INSERT INTO server_errors (at, method, path, status, message) VALUES ($1, $2, $3, $4, $5)`,
		at.UTC().Truncate(time.Microsecond), method, path, status, message)
}

// hasAlert — есть ли среди тревог тревога этого вида.
func hasAlert(alerts []dashAlert, kind string) bool {
	for _, a := range alerts {
		if a.Kind == kind {
			return true
		}
	}
	return false
}

// findRecentError ищет в errors.recent запись с этими методом, путём
// и кодом.
func findRecentError(data dashboardPayload, method, path string, status int) *dashError {
	for i, e := range data.Errors.Recent {
		if e.Method == method && e.Path == path && e.Status == status {
			return &data.Errors.Recent[i]
		}
	}
	return nil
}

// failingStorage — хранилище, которое не может сохранить файл: так
// через публичный API получается настоящий ответ 500.
type failingStorage struct {
	media.Storage
}

func (failingStorage) Put(context.Context, string, []byte) error {
	return errors.New("хранилище недоступно: диск отвалился")
}

// panickingStorage — хранилище, которое паникует при сохранении файла:
// так проверяется, что паника в обработчике не роняет сервис (ФТ-14).
type panickingStorage struct {
	media.Storage
}

func (panickingStorage) Put(context.Context, string, []byte) error {
	panic("хранилище сломалось посреди записи")
}

// --- Доступ ---------------------------------------------------------------

// Без пароля в настройках дашборда нет: и страница, и данные отвечают 404,
// как несуществующий адрес (ФТ-1).
func TestDashboardIsAbsentWithoutPassword(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{})
	root := strings.TrimSuffix(baseURL, "/api")

	for _, path := range []string{"/dashboard", "/dashboard/data"} {
		t.Run(path+" без авторизации", func(t *testing.T) {
			resp := dashboardRequest(t, root+path, "", "", "")
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("без DASHBOARD_PASSWORD ожидался статус 404, получен %d", resp.StatusCode)
			}
		})
		t.Run(path+" с каким-то паролем", func(t *testing.T) {
			resp := dashboardRequest(t, root+path, "владелец", "", "")
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("без DASHBOARD_PASSWORD ожидался статус 404 при любом пароле, получен %d", resp.StatusCode)
			}
			resp = dashboardRequest(t, root+path, "владелец", dashboardPassword, "")
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("без DASHBOARD_PASSWORD ожидался статус 404 при любом пароле, получен %d", resp.StatusCode)
			}
		})
	}
}

// Без заголовка, с неверным паролем и с токеном приложения — 401 с вызовом
// Basic, чтобы браузер спросил имя и пароль (ФТ-2, ФТ-3).
func TestDashboardRejectsWithoutRightPassword(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})
	token, _ := signIn(t, baseURL, dashboardPhone(1))

	cases := []struct {
		name           string
		user, password string
		bearer         string
	}{
		{name: "без заголовка"},
		{name: "неверный пароль", user: "владелец", password: "неверный-пароль"},
		{name: "пустой пароль", user: "владелец", password: ""},
		{name: "пароль с лишним символом", user: "владелец", password: dashboardPassword + "x"},
		{name: "токен приложения", bearer: token},
		{name: "пароль дашборда вместо токена", bearer: dashboardPassword},
	}

	for _, path := range []string{"/dashboard", "/dashboard/data"} {
		for _, c := range cases {
			t.Run(path+" "+c.name, func(t *testing.T) {
				resp := dashboardRequest(t, root+path, c.user, c.password, c.bearer)

				if resp.StatusCode != http.StatusUnauthorized {
					t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
				}
				if got := resp.Header.Get("WWW-Authenticate"); got != `Basic realm="moya-dacha"` {
					t.Fatalf(`ожидался заголовок WWW-Authenticate: Basic realm="moya-dacha", получен %q`, got)
				}
			})
		}
	}
}

// С верным паролем пускает при любом имени пользователя, а ответы не
// кэшируются (ФТ-2, ФТ-4).
func TestDashboardAcceptsRightPasswordWithAnyName(t *testing.T) {
	_, root := startDashboardAPI(t, api.Config{})

	for _, path := range []string{"/dashboard", "/dashboard/data"} {
		for _, user := range []string{"владелец", "admin", "x"} {
			t.Run(path+" имя "+user, func(t *testing.T) {
				resp := dashboardRequest(t, root+path, user, dashboardPassword, "")
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("с верным паролем ожидался статус 200, получен %d", resp.StatusCode)
				}
				if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
					t.Fatalf("ответ дашборда должен быть Cache-Control: no-store, получено %q", cc)
				}
			})
		}
	}
}

// --- Страница -------------------------------------------------------------

// /dashboard — одна HTML-страница (ФТ-6).
func TestDashboardPageIsHTML(t *testing.T) {
	_, root := startDashboardAPI(t, api.Config{})

	resp := dashboardRequest(t, root+"/dashboard", "владелец", dashboardPassword, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("страница дашборда должна быть text/html, получено %q", ct)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("не удалось прочитать страницу: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(raw)), "<html") {
		t.Fatal("в теле страницы нет разметки HTML")
	}
}

// --- Данные: форма и пустая база ------------------------------------------

// На пустой базе все поля на месте: итоги нули, 30 дней нулей, последний —
// сегодня по Москве, измерений ещё нет, тревог нет, ошибок нет (ФТ-8 —
// ФТ-12, ФТ-16, ФТ-17, ФТ-19, «Ограничения и edge cases», «Пустая база»).
func TestDashboardDataOnEmptyBase(t *testing.T) {
	_, root := startDashboardAPI(t, api.Config{})

	before := time.Now()
	raw := dashboardRaw(t, root)
	after := time.Now()

	t.Run("все поля на месте", func(t *testing.T) {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatalf("данные не разобрались как объект: %v", err)
		}
		for _, field := range []string{"generated_at", "alerts", "totals", "activity", "server", "errors"} {
			if _, ok := fields[field]; !ok {
				t.Errorf("в данных нет поля %q", field)
			}
		}

		requireKeys := func(what string, value json.RawMessage, keys ...string) {
			t.Helper()
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(value, &obj); err != nil {
				t.Fatalf("%s — не объект: %v", what, err)
			}
			for _, key := range keys {
				if _, ok := obj[key]; !ok {
					t.Errorf("в %s нет поля %q", what, key)
				}
			}
		}
		requireKeys("totals", fields["totals"], "users", "posts", "likes", "comments", "reports")
		requireKeys("server", fields["server"], "now", "history")
		requireKeys("errors", fields["errors"], "last_hour", "last_day", "recent")

		var days []json.RawMessage
		if err := json.Unmarshal(fields["activity"], &days); err != nil || len(days) == 0 {
			t.Fatalf("activity — не непустой список: %v", err)
		}
		requireKeys("дне activity", days[0], "date", "new_users", "active_users", "posts", "likes", "comments")

		requireList := func(what string, value json.RawMessage) {
			t.Helper()
			var list []json.RawMessage
			if err := json.Unmarshal(value, &list); err != nil || list == nil {
				t.Errorf("%s должен быть списком, а не %s", what, string(value))
			}
		}
		requireList("alerts", fields["alerts"])

		var server map[string]json.RawMessage
		_ = json.Unmarshal(fields["server"], &server)
		requireList("server.history", server["history"])
		if string(server["now"]) != "null" {
			t.Errorf("пока измерений не было, server.now должен быть null, получено %s", string(server["now"]))
		}

		var errs map[string]json.RawMessage
		_ = json.Unmarshal(fields["errors"], &errs)
		requireList("errors.recent", errs["recent"])
	})

	var data dashboardPayload
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("данные не разобрались как JSON: %v", err)
	}

	t.Run("generated_at — время сборки ответа", func(t *testing.T) {
		at := parseDashTime(t, data.GeneratedAt, "generated_at")
		if at.Before(before.Add(-time.Minute)) || at.After(after.Add(time.Minute)) {
			t.Fatalf("generated_at %s не похож на время запроса %s", at, after)
		}
	})

	t.Run("итоги нули", func(t *testing.T) {
		if data.Totals != (dashTotals{}) {
			t.Fatalf("на пустой базе итоги должны быть нулями, получено %+v", data.Totals)
		}
	})

	t.Run("30 дней нулей, последний сегодня", func(t *testing.T) {
		requireThirtyDays(t, data.Activity, before, after)
		for _, day := range data.Activity {
			if day.NewUsers != 0 || day.ActiveUsers != 0 || day.Posts != 0 || day.Likes != 0 || day.Comments != 0 {
				t.Fatalf("на пустой базе день %s должен быть нулевым, получено %+v", day.Date, day)
			}
		}
	})

	t.Run("измерений и ошибок нет", func(t *testing.T) {
		if data.Server.Now != nil {
			t.Errorf("пока измерений не было, server.now должен быть null, получено %+v", *data.Server.Now)
		}
		if len(data.Server.History) != 0 {
			t.Errorf("пока измерений не было, history должна быть пустой, получено %d шагов", len(data.Server.History))
		}
		if data.Errors.LastHour != 0 || data.Errors.LastDay != 0 || len(data.Errors.Recent) != 0 {
			t.Errorf("на пустой базе ошибок нет, получено %+v", data.Errors)
		}
	})

	t.Run("тревог нет", func(t *testing.T) {
		if data.Alerts == nil || len(data.Alerts) != 0 {
			t.Fatalf("на пустой базе alerts должен быть пустым списком, получено %+v", data.Alerts)
		}
	})
}

// --- Данные: итоги и активность по дням -----------------------------------

// Итоги сходятся с тем, что сделали люди через приложение, а сегодняшний
// день считает новых людей, посты, лайки, комментарии и разных активных
// людей: кто сделал несколько дел, считается один раз, а жалоба активностью
// не считается (ФТ-9, ФТ-10).
func TestDashboardCountsWhatPeopleDidToday(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	before := time.Now()

	anna := newDashUser(t, baseURL, 1)
	boris := newDashUser(t, baseURL, 2)
	vera := newDashUser(t, baseURL, 3) // только жалуется

	annaFirst := publishPost(t, baseURL, anna.token, "Первые огурцы")
	annaSecond := publishPost(t, baseURL, anna.token, "Кабачки пошли")
	borisPost := publishPost(t, baseURL, boris.token, "Теплица готова")

	// Пост, который опубликовали и удалили: удалённое удаляется честно
	// и нигде не считается (ФТ-10, ADR-0007).
	gone := publishPost(t, baseURL, boris.token, "Передумал")
	requireDeleted(t, deletePost(t, baseURL, boris.token, gone.ID))

	likedOK(t, likePost(t, baseURL, anna.token, borisPost.ID), http.StatusOK)
	likedOK(t, likePost(t, baseURL, boris.token, annaFirst.ID), http.StatusOK)
	likedOK(t, likePost(t, baseURL, boris.token, annaSecond.ID), http.StatusOK)

	commentOf(t, baseURL, anna.token, annaFirst.ID, "Спасибо!")
	commentOf(t, baseURL, boris.token, annaFirst.ID, "Отличные огурцы")

	if resp := reportPostReason(t, baseURL, vera.token, borisPost.ID, "спам"); resp.StatusCode != http.StatusAccepted {
		t.Fatalf("жалоба не принята: статус %d", resp.StatusCode)
	}

	data := dashboardData(t, root)
	after := time.Now()

	t.Run("итоги", func(t *testing.T) {
		want := dashTotals{Users: 3, Posts: 3, Likes: 3, Comments: 2, Reports: 1}
		if data.Totals != want {
			t.Fatalf("итоги: ожидалось %+v, получено %+v", want, data.Totals)
		}
	})

	t.Run("сегодня", func(t *testing.T) {
		requireThirtyDays(t, data.Activity, before, after)
		today := data.Activity[len(data.Activity)-1]
		want := dashDay{Date: today.Date, NewUsers: 3, ActiveUsers: 2, Posts: 3, Likes: 3, Comments: 2}
		if today != want {
			t.Fatalf("сегодняшний день: ожидалось %+v, получено %+v (активны двое — Анна и Борис; Вера только пожаловалась)", want, today)
		}
	})

	t.Run("прошлые дни нулевые", func(t *testing.T) {
		for _, day := range data.Activity[:len(data.Activity)-1] {
			if day != (dashDay{Date: day.Date}) {
				t.Fatalf("всё сделано сегодня, а день %s не нулевой: %+v", day.Date, day)
			}
		}
	})
}

// dashUser — вошедший участник теста дашборда.
type dashUser struct {
	token string
	id    string
}

func newDashUser(t *testing.T, baseURL string, n int) dashUser {
	t.Helper()

	token, id := signIn(t, baseURL, dashboardPhone(n))

	return dashUser{token: token, id: id}
}

// События раскладываются по дням по Москве: вчерашнее — во вчерашний день,
// позавчерашнее — в свой, а то, что старше 30 дней, в activity не попадает,
// но в итогах есть (ФТ-9, ФТ-10). Время событий через API не задать,
// поэтому тест сдвигает created_at прямо в базе.
func TestDashboardSplitsActivityByMoscowDays(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	anna := newDashUser(t, baseURL, 1)
	boris := newDashUser(t, baseURL, 2)
	vera := newDashUser(t, baseURL, 3)

	todayPost := publishPost(t, baseURL, anna.token, "Сегодняшний")
	yesterdayPost := publishPost(t, baseURL, boris.token, "Вчерашний")
	ancientPost := publishPost(t, baseURL, vera.token, "Прошлогодний")

	commentOf(t, baseURL, anna.token, yesterdayPost.ID, "Хорошо!")
	likedOK(t, likePost(t, baseURL, anna.token, todayPost.ID), http.StatusOK)

	now := time.Now()
	todayStart := startOfMoscowDay(now)
	// Вчера в 22:00 по Москве: это вчера, как бы близко к полуночи
	// ни шёл тест.
	yesterday := todayStart.Add(-2 * time.Hour)
	// Позавчера в полдень по Москве.
	dayBefore := todayStart.AddDate(0, 0, -2).Add(12 * time.Hour)
	// 40 дней назад — за пределами 30 дней.
	ancient := now.AddDate(0, 0, -40)

	if n := execSQL(t, `UPDATE posts SET created_at = $1 WHERE id = $2`, yesterday.UTC(), yesterdayPost.ID); n != 1 {
		t.Fatalf("время поста не сдвинулось: изменено строк %d", n)
	}
	if n := execSQL(t, `UPDATE posts SET created_at = $1 WHERE id = $2`, ancient.UTC(), ancientPost.ID); n != 1 {
		t.Fatalf("время поста не сдвинулось: изменено строк %d", n)
	}
	// Борис завёлся позавчера, Вера — 40 дней назад.
	if n := execSQL(t, `UPDATE users SET created_at = $1 WHERE id = $2`, dayBefore.UTC(), boris.id); n != 1 {
		t.Fatalf("время регистрации не сдвинулось: изменено строк %d", n)
	}
	if n := execSQL(t, `UPDATE users SET created_at = $1 WHERE id = $2`, ancient.UTC(), vera.id); n != 1 {
		t.Fatalf("время регистрации не сдвинулось: изменено строк %d", n)
	}

	data := dashboardData(t, root)
	after := time.Now()

	// Полночь по Москве между подготовкой и запросом сдвинула бы все дни.
	if moscowDate(now) != moscowDate(after) {
		t.Skip("тест пересёк полночь по Москве, дни сдвинулись — повторите")
	}

	requireThirtyDays(t, data.Activity, now, after)

	t.Run("итоги считают всё, даже старое", func(t *testing.T) {
		want := dashTotals{Users: 3, Posts: 3, Likes: 1, Comments: 1, Reports: 0}
		if data.Totals != want {
			t.Fatalf("итоги: ожидалось %+v, получено %+v", want, data.Totals)
		}
	})

	byDate := map[string]dashDay{}
	for _, day := range data.Activity {
		byDate[day.Date] = day
	}

	t.Run("сегодня", func(t *testing.T) {
		got := byDate[moscowDate(now)]
		// Сегодня завелась Анна; она же опубликовала пост, прокомментировала
		// и лайкнула — один активный человек.
		want := dashDay{Date: moscowDate(now), NewUsers: 1, ActiveUsers: 1, Posts: 1, Likes: 1, Comments: 1}
		if got != want {
			t.Fatalf("сегодня: ожидалось %+v, получено %+v", want, got)
		}
	})

	t.Run("вчера", func(t *testing.T) {
		got := byDate[moscowDate(yesterday)]
		want := dashDay{Date: moscowDate(yesterday), ActiveUsers: 1, Posts: 1}
		if got != want {
			t.Fatalf("вчера по Москве (%s): ожидалось %+v, получено %+v", moscowDate(yesterday), want, got)
		}
		if data.Activity[len(data.Activity)-2].Date != moscowDate(yesterday) {
			t.Fatalf("предпоследний день activity должен быть вчерашним %s, получен %s",
				moscowDate(yesterday), data.Activity[len(data.Activity)-2].Date)
		}
	})

	t.Run("позавчера", func(t *testing.T) {
		got := byDate[moscowDate(dayBefore)]
		want := dashDay{Date: moscowDate(dayBefore), NewUsers: 1}
		if got != want {
			t.Fatalf("позавчера по Москве (%s): ожидалось %+v, получено %+v", moscowDate(dayBefore), want, got)
		}
	})

	t.Run("старше 30 дней в activity нет", func(t *testing.T) {
		var sum dashDay
		for _, day := range data.Activity {
			sum.NewUsers += day.NewUsers
			sum.Posts += day.Posts
		}
		if sum.NewUsers != 2 || sum.Posts != 2 {
			t.Fatalf("за 30 дней ожидалось 2 новых человека и 2 поста (события 40-дневной давности не в счёт), получено %d и %d",
				sum.NewUsers, sum.Posts)
		}
	})
}

// --- Данные: учёт ошибок сервера ------------------------------------------

// Ответ 500 записывается как ошибка сервера: метод, путь, код; она видна
// в errors.recent и в счётчиках за час и сутки (ФТ-14 — ФТ-16).
func TestDashboardRecordsServerError(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{
		Media: failingStorage{Storage: media.NewDisk(t.TempDir(), "/media")},
	})

	token, _ := signIn(t, baseURL, dashboardPhone(1))

	resp := putAvatar(t, baseURL, token, "avatar.png", imageBytes(t, "png", 200, 200))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("когда хранилище не сохраняет файл, ожидался статус 500, получен %d", resp.StatusCode)
	}

	data, ok := waitDashboard(t, root, func(d dashboardPayload) bool {
		return findRecentError(d, http.MethodPut, "/api/me/avatar", 500) != nil
	})
	if !ok {
		t.Fatalf("ошибка 500 на PUT /api/me/avatar не появилась в errors.recent: %+v", data.Errors.Recent)
	}

	rec := findRecentError(data, http.MethodPut, "/api/me/avatar", 500)
	at := parseDashTime(t, rec.At, "время ошибки")
	if time.Since(at) > time.Minute || time.Until(at) > time.Minute {
		t.Errorf("время ошибки %s не похоже на только что", at)
	}
	if data.Errors.LastHour < 1 {
		t.Errorf("errors.last_hour должен учесть ошибку, получено %d", data.Errors.LastHour)
	}
	if data.Errors.LastDay < 1 {
		t.Errorf("errors.last_day должен учесть ошибку, получено %d", data.Errors.LastDay)
	}
}

// Паника в обработчике — тоже ошибка сервера: клиент получает 500, сервис
// продолжает отвечать, ошибка записана (ФТ-14, ФТ-15).
func TestDashboardSurvivesAndRecordsPanic(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{
		Media: panickingStorage{Storage: media.NewDisk(t.TempDir(), "/media")},
	})

	token, _ := signIn(t, baseURL, dashboardPhone(1))

	t.Run("клиент получает 500", func(t *testing.T) {
		resp := putAvatar(t, baseURL, token, "avatar.png", imageBytes(t, "png", 200, 200))
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("при панике в обработчике ожидался статус 500, получен %d", resp.StatusCode)
		}
	})

	t.Run("сервис продолжает работать", func(t *testing.T) {
		resp := get(t, baseURL+"/health")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("после паники /api/health должен отвечать 200, получен %d", resp.StatusCode)
		}
		if resp := getProfile(t, baseURL, token); resp.StatusCode != http.StatusOK {
			t.Fatalf("после паники /api/me должен отвечать 200, получен %d", resp.StatusCode)
		}
	})

	t.Run("паника записана как ошибка", func(t *testing.T) {
		data, ok := waitDashboard(t, root, func(d dashboardPayload) bool {
			return findRecentError(d, http.MethodPut, "/api/me/avatar", 500) != nil
		})
		if !ok {
			t.Fatalf("паника на PUT /api/me/avatar не появилась в errors.recent: %+v", data.Errors.Recent)
		}
		if data.Errors.LastHour < 1 {
			t.Errorf("errors.last_hour должен учесть панику, получено %d", data.Errors.LastHour)
		}
	})
}

// Счётчики за час и сутки считают только свои окна; errors.recent — не
// больше 20 последних записей, от новых к старым, со всеми полями (ФТ-16).
func TestDashboardErrorCountsAndRecentOrder(t *testing.T) {
	_, root := startDashboardAPI(t, api.Config{})

	now := time.Now()

	// За последний час — 2, за сутки — ещё 1, и одна позавчерашняя.
	insertServerError(t, now.Add(-10*time.Minute), "POST", "/api/posts", 500, "нет связи с базой")
	insertServerError(t, now.Add(-40*time.Minute), "GET", "/api/feed", 502, "")
	insertServerError(t, now.Add(-3*time.Hour), "PUT", "/api/me/avatar", 500, "диск")
	insertServerError(t, now.Add(-49*time.Hour), "GET", "/api/me", 500, "старое")

	t.Run("счётчики по окнам", func(t *testing.T) {
		data := dashboardData(t, root)
		if data.Errors.LastHour != 2 {
			t.Errorf("errors.last_hour: ожидалось 2, получено %d", data.Errors.LastHour)
		}
		if data.Errors.LastDay != 3 {
			t.Errorf("errors.last_day: ожидалось 3, получено %d", data.Errors.LastDay)
		}

		if len(data.Errors.Recent) == 0 {
			t.Fatal("errors.recent пуст, хотя ошибки есть")
		}
		first := data.Errors.Recent[0]
		if first.Method != "POST" || first.Path != "/api/posts" || first.Status != 500 || first.Message != "нет связи с базой" {
			t.Errorf("самая свежая запись должна быть POST /api/posts 500 «нет связи с базой», получено %+v", first)
		}
	})

	// Ещё 25 ошибок в прошлом часе и раньше, каждая на минуту старше
	// предыдущей, — всего записей больше 20.
	for i := 0; i < 25; i++ {
		insertServerError(t, now.Add(-time.Duration(60+i)*time.Minute), "GET", fmt.Sprintf("/api/posts/%d", i), 500, fmt.Sprintf("ошибка %d", i))
	}

	t.Run("последние 20 от новых к старым", func(t *testing.T) {
		data := dashboardData(t, root)
		if len(data.Errors.Recent) != 20 {
			t.Fatalf("errors.recent: ожидалось 20 записей, получено %d", len(data.Errors.Recent))
		}

		var prev time.Time
		for i, e := range data.Errors.Recent {
			at := parseDashTime(t, e.At, fmt.Sprintf("errors.recent[%d].at", i))
			if i > 0 && at.After(prev) {
				t.Fatalf("errors.recent не от новых к старым: запись %d (%s) новее записи %d (%s)", i, at, i-1, prev)
			}
			prev = at
		}

		// Двадцать самых свежих: две за последний час, затем 18 из 25
		// (60–77 минут назад); запись трёхчасовой давности уже не входит.
		if data.Errors.Recent[0].Path != "/api/posts" || data.Errors.Recent[1].Path != "/api/feed" {
			t.Errorf("первыми должны идти две свежие ошибки, получено %s и %s",
				data.Errors.Recent[0].Path, data.Errors.Recent[1].Path)
		}
		last := data.Errors.Recent[19]
		if last.Path != "/api/posts/17" {
			t.Errorf("двадцатой должна быть ошибка /api/posts/17, получено %s", last.Path)
		}
		if last.Status != 500 || last.Method != "GET" || last.Message != "ошибка 17" {
			t.Errorf("поля записи ошибки не сохранились: %+v", last)
		}
	})
}

// --- Данные: сервер -------------------------------------------------------

// server.now — последнее измерение; history — шаги по 5 минут за сутки,
// только те, где были измерения, от старого к новому: средние процессор и
// память, наибольший диск, сумма запросов и ошибок (ФТ-11, ФТ-12).
func TestDashboardServerNowAndHistory(t *testing.T) {
	_, root := startDashboardAPI(t, api.Config{})

	now := time.Now()
	current := now.Truncate(5 * time.Minute)

	// Старше суток — в history не попадает.
	insertSample(t, calmSample(now.Add(-25*time.Hour)))

	// Шаг два часа назад: два измерения и одна ошибка.
	older := current.Add(-2 * time.Hour)
	insertSample(t, sampleRow{at: older.Add(1 * time.Minute), cpu: 10, memUsed: 100_000_000, memTotal: 2_000_000_000,
		diskUsed: 3_000_000_000, diskTotal: 20_000_000_000, requests: 3})
	insertSample(t, sampleRow{at: older.Add(3 * time.Minute), cpu: 30, memUsed: 300_000_000, memTotal: 2_000_000_000,
		diskUsed: 1_000_000_000, diskTotal: 20_000_000_000, requests: 4})
	insertServerError(t, older.Add(2*time.Minute), "POST", "/api/posts", 500, "сбой")

	// Ошибка в шаге без измерений шага не создаёт.
	insertServerError(t, older.Add(30*time.Minute), "GET", "/api/feed", 500, "сбой")

	// Шаг час назад: одно измерение, оно же последнее.
	newer := current.Add(-1 * time.Hour)
	latestAt := newer.Add(2 * time.Minute)
	insertSample(t, sampleRow{at: latestAt, cpu: 50, memUsed: 500_000_000, memTotal: 2_000_000_000, swap: 7_000_000,
		diskUsed: 2_000_000_000, diskTotal: 20_000_000_000, requests: 1})

	data := dashboardData(t, root)

	t.Run("now — последнее измерение", func(t *testing.T) {
		n := data.Server.Now
		if n == nil {
			t.Fatal("измерения есть, а server.now равен null")
		}
		if at := parseDashTime(t, n.MeasuredAt, "server.now.measured_at"); !at.Equal(latestAt.UTC().Truncate(time.Microsecond)) {
			t.Errorf("server.now.measured_at: ожидалось %s, получено %s", latestAt.UTC(), at)
		}
		if math.Abs(n.CPUPercent-50) > 0.01 || n.MemoryUsedBytes != 500_000_000 || n.MemoryTotalBytes != 2_000_000_000 ||
			n.SwapUsedBytes != 7_000_000 || n.DiskUsedBytes != 2_000_000_000 || n.DiskTotalBytes != 20_000_000_000 {
			t.Errorf("server.now не совпадает с последним измерением: %+v", *n)
		}
		if n.DBSizeBytes <= 0 {
			t.Errorf("размер базы должен быть больше нуля, получено %v", n.DBSizeBytes)
		}
		if n.UptimeSeconds < 0 {
			t.Errorf("uptime_seconds не может быть отрицательным, получено %v", n.UptimeSeconds)
		}
	})

	t.Run("history — только шаги с измерениями за сутки", func(t *testing.T) {
		h := data.Server.History
		if len(h) != 2 {
			t.Fatalf("в history ожидалось 2 шага (измерение старше суток и шаг без измерений не в счёт), получено %d: %+v", len(h), h)
		}

		first, second := h[0], h[1]
		if at := parseDashTime(t, first.At, "history[0].at"); !at.Equal(older) {
			t.Errorf("первый шаг должен начинаться в %s, получено %s", older.UTC(), at)
		}
		if at := parseDashTime(t, second.At, "history[1].at"); !at.Equal(newer) {
			t.Errorf("второй шаг должен начинаться в %s, получено %s", newer.UTC(), at)
		}

		if math.Abs(first.CPUPercent-20) > 0.01 {
			t.Errorf("процессор в шаге — среднее: ожидалось 20, получено %v", first.CPUPercent)
		}
		if math.Abs(first.MemoryUsedBytes-200_000_000) > 1 {
			t.Errorf("память в шаге — среднее: ожидалось 200000000, получено %v", first.MemoryUsedBytes)
		}
		if first.DiskUsedBytes != 3_000_000_000 {
			t.Errorf("диск в шаге — наибольшее: ожидалось 3000000000, получено %v", first.DiskUsedBytes)
		}
		if first.Requests != 7 {
			t.Errorf("запросы в шаге — сумма: ожидалось 7, получено %d", first.Requests)
		}
		if first.Errors != 1 {
			t.Errorf("ошибки в шаге — сумма: ожидалась 1, получено %d", first.Errors)
		}

		if math.Abs(second.CPUPercent-50) > 0.01 || second.MemoryUsedBytes != 500_000_000 ||
			second.DiskUsedBytes != 2_000_000_000 || second.Requests != 1 || second.Errors != 0 {
			t.Errorf("второй шаг не совпадает со своим измерением: %+v", second)
		}
	})
}

// --- Тревоги --------------------------------------------------------------

// Тревога memory — последние 10 минут подряд занято больше 90% памяти;
// провал ниже порога внутри этих 10 минут тревогу снимает (ФТ-18, ФТ-19).
func TestDashboardMemoryAlert(t *testing.T) {
	cases := []struct {
		name  string
		used  func(minutesAgo int) int64
		alert bool
	}{
		{
			name:  "занято 95% уже 12 минут",
			used:  func(int) int64 { return 1_900_000_000 },
			alert: true,
		},
		{
			name:  "занято 50%",
			used:  func(int) int64 { return 1_000_000_000 },
			alert: false,
		},
		{
			name: "три минуты назад памяти хватало",
			used: func(minutesAgo int) int64 {
				if minutesAgo == 3 {
					return 1_000_000_000
				}
				return 1_900_000_000
			},
			alert: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, root := startDashboardAPI(t, api.Config{})

			now := time.Now()
			for m := 12; m >= 0; m-- {
				s := calmSample(now.Add(-time.Duration(m)*time.Minute - 5*time.Second))
				s.memUsed = c.used(m)
				insertSample(t, s)
			}

			data := dashboardData(t, root)
			if got := hasAlert(data.Alerts, "memory"); got != c.alert {
				t.Fatalf("тревога memory: ожидалось %v, получено %v (тревоги: %+v)", c.alert, got, data.Alerts)
			}
			for _, a := range data.Alerts {
				if a.Kind == "memory" && strings.TrimSpace(a.Message) == "" {
					t.Error("у тревоги memory пустое сообщение для человека")
				}
			}
		})
	}
}

// Тревога errors — за последний час три ошибки сервера или больше; две —
// ещё не тревога, и ошибки старше часа не в счёт (ФТ-18, ФТ-19).
func TestDashboardErrorsAlert(t *testing.T) {
	cases := []struct {
		name  string
		ago   []time.Duration
		alert bool
	}{
		{name: "три за час", ago: []time.Duration{5 * time.Minute, 20 * time.Minute, 50 * time.Minute}, alert: true},
		{name: "две за час", ago: []time.Duration{5 * time.Minute, 20 * time.Minute}, alert: false},
		{name: "две за час и одна раньше", ago: []time.Duration{5 * time.Minute, 20 * time.Minute, 2 * time.Hour}, alert: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, root := startDashboardAPI(t, api.Config{})

			now := time.Now()
			for i, ago := range c.ago {
				insertServerError(t, now.Add(-ago), "GET", fmt.Sprintf("/api/posts/%d", i), 500, "сбой")
			}

			data := dashboardData(t, root)
			if got := hasAlert(data.Alerts, "errors"); got != c.alert {
				t.Fatalf("тревога errors: ожидалось %v, получено %v (тревоги: %+v)", c.alert, got, data.Alerts)
			}
			for _, a := range data.Alerts {
				if a.Kind == "errors" && strings.TrimSpace(a.Message) == "" {
					t.Error("у тревоги errors пустое сообщение для человека")
				}
			}
		})
	}
}
