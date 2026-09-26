package tests

// Тесты заходов и времени в приложении (specs/020-app-sessions.md).
//
// Начало и конец сессии сервис отмечает по своим часам, поэтому тест не
// может ждать минутами, чтобы проверить длительности, обрезку до 3 часов
// и границы 7 суток: он сдвигает started_at и ended_at прямо в таблице
// app_sessions, а экраны смотрит в app_session_screens — колонки взяты
// из раздела «Модель данных».

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// clockSlack — допуск между часами теста и часами базы: они на одной
// машине, но запрос и запись занимают время.
const clockSlack = 5 * time.Second

// --- Хелперы --------------------------------------------------------------

// appSessionPhone — номер n-го участника тестов сессий. Номера не
// пересекаются с номерами других тестов пакета.
func appSessionPhone(n int) string {
	return fmt.Sprintf("+7 (900) 820-00-%02d", n)
}

// startAppSessionRaw — POST /app-sessions, сырой ответ.
func startAppSessionRaw(t *testing.T, baseURL, token string) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, baseURL+"/app-sessions", token, nil)
}

// startAppSession начинает сессию и требует 201 с UUID в ответе.
func startAppSession(t *testing.T, baseURL, token string) string {
	t.Helper()

	resp := startAppSessionRaw(t, baseURL, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("на POST /app-sessions ожидался статус 201, получен %d", resp.StatusCode)
	}

	var body struct {
		ID string `json:"id"`
	}
	decode(t, resp, &body)

	if !uuidPattern.MatchString(body.ID) {
		t.Fatalf("id сессии %q — не UUID", body.ID)
	}

	return body.ID
}

// endAppSession — POST /app-sessions/{id}/end. body == nil — без тела.
func endAppSession(t *testing.T, baseURL, token, id string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, baseURL+"/app-sessions/"+id+"/end", token, body)
}

// endAppSessionOK отмечает конец с экранами и требует 204.
func endAppSessionOK(t *testing.T, baseURL, token, id string, screens map[string]int) {
	t.Helper()

	resp := endAppSession(t, baseURL, token, id, map[string]any{"screens": screens})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("на конец сессии с экранами %v ожидался статус 204, получен %d", screens, resp.StatusCode)
	}
}

// requireErrorStatus требует статус и машиночитаемый код ошибки.
func requireErrorStatus(t *testing.T, resp *http.Response, status int, code, what string) {
	t.Helper()

	if resp.StatusCode != status {
		t.Fatalf("%s: ожидался статус %d, получен %d", what, status, resp.StatusCode)
	}
	if got := errorCode(t, resp); got != code {
		t.Fatalf("%s: ожидался код ошибки %q, получен %q", what, code, got)
	}
}

// appSessionRow читает сессию из базы. found == false — строки нет.
func appSessionRow(t *testing.T, id string) (userID string, startedAt time.Time, endedAt *time.Time, found bool) {
	t.Helper()

	pool := connect(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `SELECT user_id::text, started_at, ended_at FROM app_sessions WHERE id = $1`, id)
	if err != nil {
		t.Fatalf("не удалось прочитать сессию из базы: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			t.Fatalf("не удалось прочитать сессию из базы: %v", err)
		}
		return "", time.Time{}, nil, false
	}
	if err := rows.Scan(&userID, &startedAt, &endedAt); err != nil {
		t.Fatalf("не удалось разобрать сессию из базы: %v", err)
	}

	return userID, startedAt, endedAt, true
}

// appSessionEndedAt — время конца сессии, nil — конца не было.
func appSessionEndedAt(t *testing.T, id string) *time.Time {
	t.Helper()

	_, _, endedAt, found := appSessionRow(t, id)
	if !found {
		t.Fatalf("сессии %s нет в базе", id)
	}

	return endedAt
}

// appSessionScreens — экраны сессии из app_session_screens.
func appSessionScreens(t *testing.T, id string) map[string]int {
	t.Helper()

	pool := connect(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `SELECT screen, opens FROM app_session_screens WHERE session_id = $1`, id)
	if err != nil {
		t.Fatalf("не удалось прочитать экраны сессии из базы: %v", err)
	}
	defer rows.Close()

	screens := map[string]int{}
	for rows.Next() {
		var (
			screen string
			opens  int
		)
		if err := rows.Scan(&screen, &opens); err != nil {
			t.Fatalf("не удалось разобрать экран сессии: %v", err)
		}
		screens[screen] = opens
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("не удалось прочитать экраны сессии из базы: %v", err)
	}

	return screens
}

// requireScreens сравнивает экраны сессии в базе с ожидаемыми.
func requireScreens(t *testing.T, id string, want map[string]int, what string) {
	t.Helper()

	got := appSessionScreens(t, id)
	if len(got) != len(want) {
		t.Fatalf("%s: ожидались экраны %v, в базе %v", what, want, got)
	}
	for screen, opens := range want {
		if got[screen] != opens {
			t.Fatalf("%s: ожидались экраны %v, в базе %v", what, want, got)
		}
	}
}

// setSessionTimes ставит сессии начало и конец (end == nil — конца нет).
func setSessionTimes(t *testing.T, id string, start time.Time, end *time.Time) {
	t.Helper()

	var endArg any
	if end != nil {
		endArg = end.UTC().Truncate(time.Microsecond)
	}
	if n := execSQL(t, `UPDATE app_sessions SET started_at = $1, ended_at = $2 WHERE id = $3`,
		start.UTC().Truncate(time.Microsecond), endArg, id); n != 1 {
		t.Fatalf("время сессии %s не сдвинулось: изменено строк %d", id, n)
	}
}

// setSessionDuration оставляет начало, которое поставил сервис, а конец
// ставит через duration после него.
func setSessionDuration(t *testing.T, id string, duration time.Duration) {
	t.Helper()

	if n := execSQL(t, `UPDATE app_sessions SET ended_at = started_at + make_interval(secs => $1) WHERE id = $2`,
		duration.Seconds(), id); n != 1 {
		t.Fatalf("длительность сессии %s не поставилась: изменено строк %d", id, n)
	}
}

// --- Форма usage в данных дашборда ------------------------------------------

type usageDay struct {
	Date     string `json:"date"`
	Users    int    `json:"users"`
	Sessions int    `json:"sessions"`
	Minutes  int    `json:"minutes"`
}

type usageScreen struct {
	Screen string `json:"screen"`
	Opens  int    `json:"opens"`
	Users  int    `json:"users"`
}

type usagePayload struct {
	WeekUsers int           `json:"week_users"`
	Days      []usageDay    `json:"days"`
	Screens   []usageScreen `json:"screens"`
}

// dashboardUsage читает /dashboard/data и возвращает поле usage и его
// сырой вид (чтобы отличить [] от null).
func dashboardUsage(t *testing.T, root string) (usagePayload, map[string]json.RawMessage) {
	t.Helper()

	var envelope struct {
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(dashboardRaw(t, root), &envelope); err != nil {
		t.Fatalf("данные дашборда не разобрались как JSON: %v", err)
	}
	if len(envelope.Usage) == 0 || string(envelope.Usage) == "null" {
		t.Fatalf("в данных дашборда нет поля usage")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Usage, &raw); err != nil {
		t.Fatalf("usage — не объект: %v", err)
	}

	var usage usagePayload
	if err := json.Unmarshal(envelope.Usage, &usage); err != nil {
		t.Fatalf("usage не разобрался: %v", err)
	}

	return usage, raw
}

// requireUsageThirtyDays требует ровно 30 дней подряд от старого к новому,
// последний — сегодня по Москве (ФТ-12).
func requireUsageThirtyDays(t *testing.T, days []usageDay, before, after time.Time) {
	t.Helper()

	if len(days) != 30 {
		t.Fatalf("в usage.days ожидалось ровно 30 дней, получено %d", len(days))
	}

	last := days[len(days)-1].Date
	if last != moscowDate(before) && last != moscowDate(after) {
		t.Fatalf("последний день usage.days %q, а сегодня по Москве %q", last, moscowDate(after))
	}

	lastDay, err := time.ParseInLocation("2006-01-02", last, moscow)
	if err != nil {
		t.Fatalf("дата %q не в формате ГГГГ-ММ-ДД: %v", last, err)
	}
	for i, day := range days {
		want := lastDay.AddDate(0, 0, i-(len(days)-1)).Format("2006-01-02")
		if day.Date != want {
			t.Fatalf("день %d в usage.days: ожидалась дата %q, получена %q — дни должны идти подряд от старого к новому", i, want, day.Date)
		}
	}
}

// --- Начало сессии ----------------------------------------------------------

// Начало заводит сессию вошедшего человека со временем «сейчас» по часам
// сервера и без конца; каждое начало — новая сессия (ФТ-3, ФТ-4).
func TestAppSessionStart(t *testing.T) {
	baseURL := startAPI(t)
	token, userID := signIn(t, baseURL, appSessionPhone(1))

	before := time.Now()
	first := startAppSession(t, baseURL, token)
	second := startAppSession(t, baseURL, token)
	after := time.Now()

	if first == second {
		t.Fatalf("два начала вернули одну сессию %s", first)
	}

	for _, id := range []string{first, second} {
		owner, startedAt, endedAt, found := appSessionRow(t, id)
		if !found {
			t.Fatalf("сессии %s нет в app_sessions", id)
		}
		if owner != userID {
			t.Errorf("сессия %s записана на %s, а начинал её %s", id, owner, userID)
		}
		if startedAt.Before(before.Add(-clockSlack)) || startedAt.After(after.Add(clockSlack)) {
			t.Errorf("начало сессии %s = %s, а запрос шёл между %s и %s", id, startedAt, before, after)
		}
		if endedAt != nil {
			t.Errorf("у только что начатой сессии %s уже есть конец %s", id, endedAt)
		}
		requireScreens(t, id, map[string]int{}, "только что начатая сессия")
	}
}

// Без токена или с чужим/битым токеном сессию не начать и не закончить
// (ФТ-6).
func TestAppSessionRequiresToken(t *testing.T) {
	baseURL := startAPI(t)
	token, _ := signIn(t, baseURL, appSessionPhone(1))
	id := startAppSession(t, baseURL, token)

	t.Run("начало без токена", func(t *testing.T) {
		resp := startAppSessionRaw(t, baseURL, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
		}
		if n := countSQL(t, `SELECT count(*) FROM app_sessions`); n != 1 {
			t.Fatalf("без токена сессия завелась: в базе %d сессий, ожидалась 1", n)
		}
	})

	t.Run("начало с недействительным токеном", func(t *testing.T) {
		resp := startAppSessionRaw(t, baseURL, "не-токен")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
		}
	})

	t.Run("конец без токена", func(t *testing.T) {
		resp := endAppSession(t, baseURL, "", id, map[string]any{"screens": map[string]int{"feed": 1}})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
		}
		if end := appSessionEndedAt(t, id); end != nil {
			t.Fatalf("конец без токена записался: %s", end)
		}
		requireScreens(t, id, map[string]int{}, "конец без токена")
	})

	t.Run("конец с недействительным токеном", func(t *testing.T) {
		resp := endAppSession(t, baseURL, "не-токен", id, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
		}
		if end := appSessionEndedAt(t, id); end != nil {
			t.Fatalf("конец с недействительным токеном записался: %s", end)
		}
	})
}

// --- Конец сессии -----------------------------------------------------------

// Конец ставит время «сейчас» и запоминает экраны (ФТ-5, ФТ-8).
func TestAppSessionEnd(t *testing.T) {
	baseURL := startAPI(t)
	token, _ := signIn(t, baseURL, appSessionPhone(1))
	id := startAppSession(t, baseURL, token)

	before := time.Now()
	endAppSessionOK(t, baseURL, token, id, map[string]int{"feed": 3, "post": 5})
	after := time.Now()

	end := appSessionEndedAt(t, id)
	if end == nil {
		t.Fatalf("после отметки конца у сессии нет времени конца")
	}
	if end.Before(before.Add(-clockSlack)) || end.After(after.Add(clockSlack)) {
		t.Fatalf("конец сессии %s, а запрос шёл между %s и %s", end, before, after)
	}

	requireScreens(t, id, map[string]int{"feed": 3, "post": 5}, "после конца")
}

// Повторный конец сдвигает время конца на «сейчас» и заменяет экраны
// присланными, а не прибавляет к прошлым (ФТ-5, ФТ-8).
func TestAppSessionEndAgain(t *testing.T) {
	baseURL := startAPI(t)
	token, _ := signIn(t, baseURL, appSessionPhone(1))
	id := startAppSession(t, baseURL, token)

	endAppSessionOK(t, baseURL, token, id, map[string]int{"feed": 3, "post": 5})

	// Первый конец отодвигается в прошлое, чтобы сдвиг был заметен.
	now := time.Now()
	earlier := now.Add(-9 * time.Minute)
	setSessionTimes(t, id, now.Add(-10*time.Minute), &earlier)

	before := time.Now()
	endAppSessionOK(t, baseURL, token, id, map[string]int{"feed": 4, "user": 1})
	after := time.Now()

	end := appSessionEndedAt(t, id)
	if end == nil {
		t.Fatalf("после повторного конца у сессии нет времени конца")
	}
	if end.Before(before.Add(-clockSlack)) || end.After(after.Add(clockSlack)) {
		t.Fatalf("повторный конец должен сдвинуть время на «сейчас» (%s…%s), в базе %s", before, after, end)
	}

	requireScreens(t, id, map[string]int{"feed": 4, "user": 1}, "после повторного конца")

	if n := countSQL(t, `SELECT count(*) FROM app_sessions`); n != 1 {
		t.Fatalf("повторный конец должен продолжать ту же сессию, а в базе %d сессий", n)
	}
}

// Экраны можно не прислать или прислать пустыми — 204, сессия без
// экранов (ФТ-9).
func TestAppSessionEndWithoutScreens(t *testing.T) {
	baseURL := startAPI(t)
	token, _ := signIn(t, baseURL, appSessionPhone(1))

	cases := []struct {
		name string
		body any
	}{
		{name: "без тела", body: nil},
		{name: "пустой объект", body: map[string]any{}},
		{name: "пустые screens", body: map[string]any{"screens": map[string]int{}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := startAppSession(t, baseURL, token)

			resp := endAppSession(t, baseURL, token, id, c.body)
			if resp.StatusCode != http.StatusNoContent {
				t.Fatalf("ожидался статус 204, получен %d", resp.StatusCode)
			}
			if end := appSessionEndedAt(t, id); end == nil {
				t.Fatalf("конец без экранов не записался")
			}
			requireScreens(t, id, map[string]int{}, c.name)
		})
	}
}

// Чужая, несуществующая сессия и id не UUID — 404 not_found, и ничего не
// записывается (ФТ-6).
func TestAppSessionEndNotFound(t *testing.T) {
	baseURL := startAPI(t)
	anna, _ := signIn(t, baseURL, appSessionPhone(1))
	boris, _ := signIn(t, baseURL, appSessionPhone(2))

	annaSession := startAppSession(t, baseURL, anna)

	t.Run("чужая сессия", func(t *testing.T) {
		resp := endAppSession(t, baseURL, boris, annaSession, map[string]any{"screens": map[string]int{"feed": 1}})
		requireErrorStatus(t, resp, http.StatusNotFound, "not_found", "конец чужой сессии")

		if end := appSessionEndedAt(t, annaSession); end != nil {
			t.Fatalf("чужой человек отметил конец сессии: %s", end)
		}
		requireScreens(t, annaSession, map[string]int{}, "после попытки чужого конца")
	})

	t.Run("чужая сессия без тела", func(t *testing.T) {
		resp := endAppSession(t, baseURL, boris, annaSession, nil)
		requireErrorStatus(t, resp, http.StatusNotFound, "not_found", "конец чужой сессии без тела")

		if end := appSessionEndedAt(t, annaSession); end != nil {
			t.Fatalf("чужой человек отметил конец сессии: %s", end)
		}
	})

	t.Run("несуществующая сессия", func(t *testing.T) {
		resp := endAppSession(t, baseURL, anna, "6f1c2b3a-4d5e-4f60-8a7b-9c0d1e2f3a4b", nil)
		requireErrorStatus(t, resp, http.StatusNotFound, "not_found", "конец несуществующей сессии")
	})

	for _, bad := range []string{"not-a-uuid", "123", "ZZ" + annaSession[2:]} {
		t.Run("id не UUID "+bad, func(t *testing.T) {
			resp := endAppSession(t, baseURL, anna, bad, nil)
			requireErrorStatus(t, resp, http.StatusNotFound, "not_found", "конец сессии с id "+bad)
		})
	}

	// Своя сессия после всех попыток по-прежнему заканчивается.
	endAppSessionOK(t, baseURL, anna, annaSession, map[string]int{"feed": 2})
	requireScreens(t, annaSession, map[string]int{"feed": 2}, "своя сессия")
}

// --- Проверка экранов ---------------------------------------------------------

// Имя экрана — 1–32 символа a-z и _, число — 1–10000, экранов не больше
// 50. Иначе 400 invalid_screens, и отметка не записывается целиком: ни
// время конца, ни экраны, в том числе правильные из той же отметки (ФТ-9).
func TestAppSessionEndValidatesScreens(t *testing.T) {
	baseURL := startAPI(t)
	token, _ := signIn(t, baseURL, appSessionPhone(1))

	fiftyOne := map[string]int{}
	for i := 0; i < 51; i++ {
		fiftyOne[screenName(i)] = 1
	}

	cases := []struct {
		name    string
		screens map[string]int
	}{
		{name: "заглавные буквы", screens: map[string]int{"feed": 1, "Feed": 1}},
		{name: "цифра в имени", screens: map[string]int{"feed": 1, "feed2": 1}},
		{name: "дефис в имени", screens: map[string]int{"feed": 1, "new-post": 1}},
		{name: "пробел в имени", screens: map[string]int{"feed": 1, "new post": 1}},
		{name: "кириллица", screens: map[string]int{"feed": 1, "лента": 1}},
		{name: "пустое имя", screens: map[string]int{"feed": 1, "": 1}},
		{name: "имя 33 символа", screens: map[string]int{"feed": 1, strings.Repeat("a", 33): 1}},
		{name: "число 0", screens: map[string]int{"feed": 1, "post": 0}},
		{name: "отрицательное число", screens: map[string]int{"feed": 1, "post": -1}},
		{name: "число 10001", screens: map[string]int{"feed": 1, "post": 10001}},
		{name: "51 экран", screens: fiftyOne},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("первая отметка", func(t *testing.T) {
				id := startAppSession(t, baseURL, token)

				resp := endAppSession(t, baseURL, token, id, map[string]any{"screens": c.screens})
				requireErrorStatus(t, resp, http.StatusBadRequest, "invalid_screens", c.name)

				if end := appSessionEndedAt(t, id); end != nil {
					t.Fatalf("отметка с ошибкой в экранах записала конец %s", end)
				}
				requireScreens(t, id, map[string]int{}, "после отметки с ошибкой")
			})

			t.Run("повторная отметка", func(t *testing.T) {
				id := startAppSession(t, baseURL, token)
				endAppSessionOK(t, baseURL, token, id, map[string]int{"feed": 2, "about": 1})

				// Прошлый конец — в прошлом, чтобы было видно, не сдвинулся ли он.
				now := time.Now()
				earlier := now.Add(-time.Minute).UTC().Truncate(time.Microsecond)
				setSessionTimes(t, id, now.Add(-2*time.Minute), &earlier)

				resp := endAppSession(t, baseURL, token, id, map[string]any{"screens": c.screens})
				requireErrorStatus(t, resp, http.StatusBadRequest, "invalid_screens", c.name)

				end := appSessionEndedAt(t, id)
				if end == nil || !end.Equal(earlier) {
					t.Fatalf("отметка с ошибкой сдвинула конец: было %s, стало %v", earlier, end)
				}
				requireScreens(t, id, map[string]int{"feed": 2, "about": 1}, "после повторной отметки с ошибкой")
			})
		})
	}
}

// Границы проверки экранов принимаются: имя из 1 и 32 символов, числа
// 1 и 10000, ровно 50 экранов, имя не из таблицы ФТ-10 (ФТ-9, ФТ-10).
func TestAppSessionEndAcceptsScreenLimits(t *testing.T) {
	baseURL := startAPI(t)
	token, _ := signIn(t, baseURL, appSessionPhone(1))

	fifty := map[string]int{}
	for i := 0; i < 50; i++ {
		fifty[screenName(i)] = 10000
	}

	cases := []struct {
		name    string
		screens map[string]int
	}{
		{name: "имя из одного символа", screens: map[string]int{"a": 1}},
		{name: "имя из 32 символов", screens: map[string]int{strings.Repeat("z", 32): 1}},
		{name: "подчёркивания", screens: map[string]int{"new_post": 1, "_": 2, "user_posts_": 3}},
		{name: "числа 1 и 10000", screens: map[string]int{"feed": 1, "post": 10000}},
		{name: "ровно 50 экранов", screens: fifty},
		{name: "имя не из таблицы", screens: map[string]int{"garden_map": 7}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := startAppSession(t, baseURL, token)
			endAppSessionOK(t, baseURL, token, id, c.screens)
			requireScreens(t, id, c.screens, c.name)
		})
	}
}

// screenName — i-е правильное имя экрана из букв a-z: a, b, …, z, ba, bb, …
func screenName(i int) string {
	name := ""
	for {
		name = string(rune('a'+i%26)) + name
		i /= 26
		if i == 0 {
			return name
		}
	}
}

// --- Дашборд ----------------------------------------------------------------

// На пустой базе usage есть: никого за неделю, 30 нулевых дней, экранов
// нет — [] (ФТ-11, ФТ-12, ФТ-13, ФТ-15).
func TestDashboardUsageOnEmptyBase(t *testing.T) {
	_, root := startDashboardAPI(t, api.Config{})

	before := time.Now()
	usage, raw := dashboardUsage(t, root)
	after := time.Now()

	for _, field := range []string{"week_users", "days", "screens"} {
		if _, ok := raw[field]; !ok {
			t.Fatalf("в usage нет поля %q", field)
		}
	}

	if usage.WeekUsers != 0 {
		t.Errorf("week_users на пустой базе: ожидался 0, получено %d", usage.WeekUsers)
	}

	requireUsageThirtyDays(t, usage.Days, before, after)
	for _, day := range usage.Days {
		if day != (usageDay{Date: day.Date}) {
			t.Fatalf("на пустой базе день %s не нулевой: %+v", day.Date, day)
		}
	}

	var dayFields []map[string]json.RawMessage
	if err := json.Unmarshal(raw["days"], &dayFields); err != nil || len(dayFields) == 0 {
		t.Fatalf("usage.days — не непустой список: %v", err)
	}
	for _, field := range []string{"date", "users", "sessions", "minutes"} {
		if _, ok := dayFields[0][field]; !ok {
			t.Fatalf("в дне usage.days нет поля %q", field)
		}
	}

	if got := strings.TrimSpace(string(raw["screens"])); got != "[]" {
		t.Fatalf("usage.screens на пустой базе: ожидался [], получено %s", got)
	}
}

// Сессии раскладываются по дням по Москве по началу: люди считаются
// разными один раз, минуты — сумма длительностей, сессия без конца — 0,
// одна сессия — не больше 3 часов. Экраны и week_users — за 7 суток,
// экраны по убыванию opens, при равенстве — по имени (ФТ-12, ФТ-13, ФТ-15).
func TestDashboardUsageCountsSessions(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	before := time.Now()

	anna := newAppSessionUser(t, baseURL, 1)  // две сессии сегодня
	boris := newAppSessionUser(t, baseURL, 2) // две сегодня, одна на сутки
	vera := newAppSessionUser(t, baseURL, 3)  // вчера
	dima := newAppSessionUser(t, baseURL, 4)  // почти 7 суток назад — ещё в неделе
	gleb := newAppSessionUser(t, baseURL, 5)  // 8 суток и 40 дней назад — вне недели

	// Анна: 10 минут 30 секунд; экраны отмечены дважды — считается
	// последняя отметка, и новое имя экрана записывается как есть.
	annaLong := startAppSession(t, baseURL, anna.token)
	endAppSessionOK(t, baseURL, anna.token, annaLong, map[string]int{"feed": 10})
	endAppSessionOK(t, baseURL, anna.token, annaLong, map[string]int{"feed": 3, "post": 2, "garden_map": 1})
	setSessionDuration(t, annaLong, 10*time.Minute+30*time.Second)

	// Анна: приложение убито — конца нет, 0 минут, но заход считается.
	startAppSession(t, baseURL, anna.token)

	// Борис: телефон пролежал сутки с экраном — обрезается до 3 часов.
	borisDay := startAppSession(t, baseURL, boris.token)
	endAppSessionOK(t, baseURL, boris.token, borisDay, map[string]int{"feed": 1, "user": 4})
	setSessionDuration(t, borisDay, 24*time.Hour)

	// Борис: 2 минуты, без экранов.
	borisShort := startAppSession(t, baseURL, boris.token)
	if resp := endAppSession(t, baseURL, boris.token, borisShort, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("конец сессии без тела: ожидался статус 204, получен %d", resp.StatusCode)
	}
	setSessionDuration(t, borisShort, 2*time.Minute)

	now := time.Now()
	todayStart := startOfMoscowDay(now)

	// Вера: вчера в 22:00 по Москве, 20 минут.
	veraSession := startAppSession(t, baseURL, vera.token)
	endAppSessionOK(t, baseURL, vera.token, veraSession, map[string]int{"post": 3, "feed": 1})
	veraStart := todayStart.Add(-2 * time.Hour)
	veraEnd := veraStart.Add(20 * time.Minute)
	setSessionTimes(t, veraSession, veraStart, &veraEnd)

	// Дима: 6 суток 23 часа назад — ещё в последних 7 сутках, 3 минуты.
	dimaSession := startAppSession(t, baseURL, dima.token)
	endAppSessionOK(t, baseURL, dima.token, dimaSession, map[string]int{"about": 2})
	dimaStart := now.Add(-(6*24 + 23) * time.Hour)
	dimaEnd := dimaStart.Add(3*time.Minute + 59*time.Second)
	setSessionTimes(t, dimaSession, dimaStart, &dimaEnd)

	// Глеб: 8 суток назад — вне недели, но в 30 днях; 7 минут.
	glebOld := startAppSession(t, baseURL, gleb.token)
	endAppSessionOK(t, baseURL, gleb.token, glebOld, map[string]int{"feed": 100, "server": 1})
	glebOldStart := now.Add(-8 * 24 * time.Hour)
	glebOldEnd := glebOldStart.Add(7 * time.Minute)
	setSessionTimes(t, glebOld, glebOldStart, &glebOldEnd)

	// Глеб: 40 дней назад — нет ни в днях, ни в экранах.
	glebAncient := startAppSession(t, baseURL, gleb.token)
	endAppSessionOK(t, baseURL, gleb.token, glebAncient, map[string]int{"feed": 50})
	glebAncientStart := now.AddDate(0, 0, -40)
	glebAncientEnd := glebAncientStart.Add(30 * time.Minute)
	setSessionTimes(t, glebAncient, glebAncientStart, &glebAncientEnd)

	usage, _ := dashboardUsage(t, root)
	after := time.Now()

	if moscowDate(before) != moscowDate(after) {
		t.Skip("тест пересёк полночь по Москве, дни сдвинулись — повторите")
	}

	t.Run("30 дней", func(t *testing.T) {
		requireUsageThirtyDays(t, usage.Days, before, after)
	})

	t.Run("дни", func(t *testing.T) {
		requireUsageThirtyDays(t, usage.Days, before, after)

		want := map[string]usageDay{}
		add := func(date string, users, sessions, minutes int) {
			d := want[date]
			d.Date = date
			d.Users += users
			d.Sessions += sessions
			d.Minutes += minutes
			want[date] = d
		}
		// Сегодня: Анна и Борис, 4 захода, 10.5 + 0 + 180 + 2 = 192.5 → 192.
		add(moscowDate(after), 2, 4, 192)
		add(moscowDate(veraStart), 1, 1, 20)
		// 3 минуты 59 секунд → 3.
		add(moscowDate(dimaStart), 1, 1, 3)
		add(moscowDate(glebOldStart), 1, 1, 7)

		for _, day := range usage.Days {
			expected, ok := want[day.Date]
			if !ok {
				expected = usageDay{Date: day.Date}
			}
			if day != expected {
				t.Errorf("день %s: ожидалось %+v, получено %+v", day.Date, expected, day)
			}
		}
	})

	t.Run("сегодня", func(t *testing.T) {
		if len(usage.Days) == 0 {
			t.Fatalf("usage.days пуст")
		}
		today := usage.Days[len(usage.Days)-1]
		want := usageDay{Date: moscowDate(after), Users: 2, Sessions: 4, Minutes: 192}
		if today != want {
			t.Fatalf("сегодня: ожидалось %+v, получено %+v (2 человека, 4 захода; сессия без конца — 0 минут, сутки обрезаны до 3 часов)", want, today)
		}
	})

	t.Run("людей за 7 дней", func(t *testing.T) {
		if usage.WeekUsers != 4 {
			t.Fatalf("week_users: ожидалось 4 (Анна, Борис, Вера, Дима; Глеб — старше недели), получено %d", usage.WeekUsers)
		}
	})

	t.Run("экраны за неделю", func(t *testing.T) {
		want := []usageScreen{
			{Screen: "feed", Opens: 5, Users: 3},
			{Screen: "post", Opens: 5, Users: 2},
			{Screen: "user", Opens: 4, Users: 1},
			{Screen: "about", Opens: 2, Users: 1},
			{Screen: "garden_map", Opens: 1, Users: 1},
		}
		if len(usage.Screens) != len(want) {
			t.Fatalf("экраны: ожидалось %+v, получено %+v", want, usage.Screens)
		}
		for i := range want {
			if usage.Screens[i] != want[i] {
				t.Fatalf("экраны (по убыванию opens, при равенстве по имени): ожидалось %+v, получено %+v", want, usage.Screens)
			}
		}
	})
}

// Экраны и week_users считаются только за последние 7 суток: если все
// сессии старше, экранов нет — [], а дни за 30 дней всё равно их видят
// (ФТ-13, ФТ-15).
func TestDashboardUsageScreensOnlyForWeek(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	anna := newAppSessionUser(t, baseURL, 1)

	id := startAppSession(t, baseURL, anna.token)
	endAppSessionOK(t, baseURL, anna.token, id, map[string]int{"feed": 3})
	start := time.Now().Add(-(7*24 + 1) * time.Hour)
	end := start.Add(5 * time.Minute)
	setSessionTimes(t, id, start, &end)

	usage, raw := dashboardUsage(t, root)

	if usage.WeekUsers != 0 {
		t.Errorf("week_users: сессия старше 7 суток, ожидался 0, получено %d", usage.WeekUsers)
	}
	if got := strings.TrimSpace(string(raw["screens"])); got != "[]" {
		t.Errorf("usage.screens: сессия старше 7 суток, ожидался [], получено %s", got)
	}

	found := false
	for _, day := range usage.Days {
		if day.Date == moscowDate(start) {
			found = true
			want := usageDay{Date: day.Date, Users: 1, Sessions: 1, Minutes: 5}
			if day != want {
				t.Errorf("день %s: ожидалось %+v, получено %+v", day.Date, want, day)
			}
		}
	}
	if !found {
		t.Errorf("дня %s нет в usage.days, а он в пределах 30 дней", moscowDate(start))
	}
}

// appSessionUser — вошедший участник тестов сессий на дашборде.
type appSessionUser struct {
	token string
	id    string
}

func newAppSessionUser(t *testing.T, baseURL string, n int) appSessionUser {
	t.Helper()

	token, id := signIn(t, baseURL, appSessionPhone(n))

	return appSessionUser{token: token, id: id}
}
