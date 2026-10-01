package tests

// Тесты населённого пункта в профиле (specs/025-places.md).
//
// Подсказки сервис берёт у DaData сам, поэтому тест подменяет DaData
// фейком на httptest: тот отдаёт заданные подсказки (или ошибку, мусор,
// задержку) и запоминает каждый пришедший запрос. Адрес и ключ задаются
// полями api.Config (ФТ-14). Выбор пункта и его видимость проверяются
// по HTTP, как это делает приложение; таблица places из «Модели данных»
// читается напрямую только там, где требование говорит о ней самой (ФТ-13).

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// placesKey — ключ DaData, который сервис получает в тестах.
const placesKey = "test-key"

// placesPrefix — путь в адресе фейка, как у настоящего адреса по умолчанию
// (ФТ-6): сервис обязан дописать /suggest/address к нему, а не заменить его.
const placesPrefix = "/suggestions/api/4_1/rs"

// Идентификаторы ФИАС, которые фейк отдаёт в большинстве проверок.
const (
	sntAndreykovoID = "5f4a3b2c-1d0e-4f9a-8b7c-6d5e4f3a2b1c"
	derAndreykovoID = "0c1d2e3f-4a5b-4c6d-8e7f-9a0b1c2d3e4f"
	dmitrovID       = "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d"
)

// --- Представления из контракта -------------------------------------------

// placePayload — schema Place.
type placePayload struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Area string `json:"area"`
}

// --- Фейковый DaData ------------------------------------------------------

// dadataRequest — запрос, который пришёл в фейк.
type dadataRequest struct {
	Method        string
	Path          string
	Authorization string
	ContentType   string
	Accept        string
	Body          []byte
}

// fakeDaData — фейковый сервис подсказок DaData.
type fakeDaData struct {
	srv *httptest.Server

	mu          sync.Mutex
	requests    []dadataRequest
	suggestions []map[string]any // что отдать в suggestions
	status      int              // 0 — 200
	rawBody     *string          // задан — отдать это тело как есть
	delay       time.Duration    // задержка перед ответом
	stop        chan struct{}
}

func newFakeDaData(t *testing.T) *fakeDaData {
	t.Helper()

	f := &fakeDaData{stop: make(chan struct{})}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(func() {
		// Сначала отпускаем задержанные ответы, потом гасим сервер:
		// Close ждёт, пока обработчики закончат.
		close(f.stop)
		f.srv.Close()
	})

	return f
}

// URL — адрес, который сервис получает в PlacesURL.
func (f *fakeDaData) URL() string { return f.srv.URL + placesPrefix }

func (f *fakeDaData) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	f.mu.Lock()
	f.requests = append(f.requests, dadataRequest{
		Method:        r.Method,
		Path:          r.URL.Path,
		Authorization: r.Header.Get("Authorization"),
		ContentType:   r.Header.Get("Content-Type"),
		Accept:        r.Header.Get("Accept"),
		Body:          body,
	})
	suggestions := f.suggestions
	status := f.status
	rawBody := f.rawBody
	delay := f.delay
	f.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		case <-f.stop:
			return
		}
	}

	if status == 0 {
		status = http.StatusOK
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if rawBody != nil {
		_, _ = io.WriteString(w, *rawBody)
		return
	}
	if suggestions == nil {
		suggestions = []map[string]any{}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"suggestions": suggestions})
}

// answer задаёт подсказки, которые фейк будет отдавать со статусом 200.
func (f *fakeDaData) answer(suggestions ...map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.suggestions = suggestions
	f.status = 0
	f.rawBody = nil
	f.delay = 0
}

// fail заставляет фейк отвечать этим статусом.
func (f *fakeDaData) fail(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.status = status
	f.rawBody = nil
}

// garbage заставляет фейк отвечать 200 с этим телом как есть.
func (f *fakeDaData) garbage(body string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.status = 0
	f.rawBody = &body
}

// slow заставляет фейк отвечать с задержкой.
func (f *fakeDaData) slow(delay time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.delay = delay
}

// calls — сколько запросов пришло в фейк.
func (f *fakeDaData) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.requests)
}

// last — последний пришедший запрос.
func (f *fakeDaData) last(t *testing.T) dadataRequest {
	t.Helper()

	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.requests) == 0 {
		t.Fatal("в DaData не пришло ни одного запроса")
	}

	return f.requests[len(f.requests)-1]
}

// suggestion — подсказка DaData: value и data с fias_id и fias_level.
// level — строка или число (ФТ-7); nil — поля fias_level нет вовсе.
// Остальные поля настоящего ответа сервису не нужны, но одно лишнее
// кладём, чтобы разбор их терпел.
func suggestion(value, fiasID string, level any) map[string]any {
	data := map[string]any{
		"fias_id":     fiasID,
		"postal_code": nil,
		"region":      "Московская",
	}
	if level != nil {
		data["fias_level"] = level
	}

	return map[string]any{
		"value":              value,
		"unrestricted_value": "141800, " + value,
		"data":               data,
	}
}

// andreykovo — две подсказки из пользовательского сценария.
func andreykovo() []map[string]any {
	return []map[string]any{
		suggestion("Московская обл, Дмитровский р-н, снт Андрейково", sntAndreykovoID, "65"),
		suggestion("Вологодская обл, Вологодский р-н, д Андрейково", derAndreykovoID, "6"),
	}
}

// --- Хелперы --------------------------------------------------------------

// placePhone — номер n-го участника теста.
func placePhone(n int) string {
	return fmt.Sprintf("+7 (900) 725-00-%02d", n)
}

// newPlaceUser регистрирует n-го участника теста.
func newPlaceUser(t *testing.T, baseURL string, n int) dachnik {
	t.Helper()

	token, id := signIn(t, baseURL, placePhone(n))

	return dachnik{token: token, id: id}
}

// startPlaces поднимает сервис с фейковым DaData и ключом.
func startPlaces(t *testing.T) (string, *fakeDaData) {
	t.Helper()

	fake := newFakeDaData(t)
	baseURL := startAPIWith(t, api.Config{PlacesURL: fake.URL(), PlacesKey: placesKey})

	return baseURL, fake
}

// placesAddress — адрес подсказок с этим q (q уходит экранированным).
func placesAddress(baseURL, q string) string {
	return baseURL + "/places?" + url.Values{"q": {q}}.Encode()
}

// fetchPlaces запрашивает подсказки.
func fetchPlaces(t *testing.T, baseURL, token, q string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, placesAddress(baseURL, q), token, nil)
}

// placesOK требует 200 и возвращает подсказки. Поле places обязано быть
// массивом (не null), у каждого пункта обязаны быть id, name и area.
func placesOK(t *testing.T, resp *http.Response, where string) []placePayload {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус 200, получен %d %s", where, resp.StatusCode, code)
	}

	raw := rawJSON(t, resp)

	var shape struct {
		Places []map[string]json.RawMessage `json:"places"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatalf("%s: ответ не разобрался как PlaceList: %v (%s)", where, err, raw)
	}
	if shape.Places == nil {
		t.Fatalf("%s: поле places должно быть массивом, получено %s", where, raw)
	}
	for i, p := range shape.Places {
		for _, field := range []string{"id", "name", "area"} {
			value, ok := p[field]
			if !ok {
				t.Errorf("%s: у пункта %d нет поля %s", where, i, field)
				continue
			}
			var s string
			if err := json.Unmarshal(value, &s); err != nil {
				t.Errorf("%s: у пункта %d поле %s не строка: %s", where, i, field, value)
			}
		}
	}

	var body struct {
		Places []placePayload `json:"places"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("%s: ответ не разобрался как PlaceList: %v", where, err)
	}

	return body.Places
}

// suggest — самый частый случай: подсказки, которые должны прийти.
func suggest(t *testing.T, baseURL, token, q string) []placePayload {
	t.Helper()
	return placesOK(t, fetchPlaces(t, baseURL, token, q), "GET /places?q="+q)
}

// requirePlaces сверяет подсказки целиком и по порядку.
func requirePlaces(t *testing.T, got, want []placePayload, where string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s: ожидалось %d пунктов, получено %d: %+v", where, len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s: пункт %d = %+v, ожидался %+v", where, i, got[i], want[i])
		}
	}
}

// setPlaceReq выбирает или убирает пункт; body — как есть.
func setPlaceReq(t *testing.T, baseURL, token string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/me/place", token, body)
}

// fieldsOf разбирает ответ-объект как словарь полей и требует статус 200.
func fieldsOf(t *testing.T, resp *http.Response, where string) map[string]json.RawMessage {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус 200, получен %d %s", where, resp.StatusCode, code)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON(t, resp), &fields); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON-объект: %v", where, err)
	}

	return fields
}

// placeOf достаёт поле place из ответа-профиля. nil — пункта нет: поля
// нет или оно null (ФТ-11, клиент понимает оба).
func placeOf(t *testing.T, fields map[string]json.RawMessage, where string) *placePayload {
	t.Helper()

	raw, ok := fields["place"]
	if !ok || string(raw) == "null" {
		return nil
	}

	var p placePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("%s: поле place не разобралось как Place: %v (%s)", where, err, raw)
	}

	var shape map[string]json.RawMessage
	_ = json.Unmarshal(raw, &shape)
	for _, field := range []string{"id", "name", "area"} {
		if _, ok := shape[field]; !ok {
			t.Errorf("%s: в place нет поля %s: %s", where, field, raw)
		}
	}

	return &p
}

// requirePlace требует, чтобы в профиле был ровно этот пункт.
func requirePlace(t *testing.T, fields map[string]json.RawMessage, want placePayload, where string) {
	t.Helper()

	got := placeOf(t, fields, where)
	if got == nil {
		t.Errorf("%s: ожидался пункт %+v, а пункта нет", where, want)
		return
	}
	if *got != want {
		t.Errorf("%s: пункт %+v, ожидался %+v", where, *got, want)
	}
}

// requireNoPlace требует, чтобы пункта в профиле не было.
func requireNoPlace(t *testing.T, fields map[string]json.RawMessage, where string) {
	t.Helper()

	if got := placeOf(t, fields, where); got != nil {
		t.Errorf("%s: пункта быть не должно, а он есть: %+v", where, *got)
	}
}

// meFields — GET /me как словарь полей.
func meFields(t *testing.T, baseURL, token string) map[string]json.RawMessage {
	t.Helper()
	return fieldsOf(t, getProfile(t, baseURL, token), "GET /me")
}

// userFields — GET /users/{id} как словарь полей.
func userFields(t *testing.T, baseURL, token, userID string) map[string]json.RawMessage {
	t.Helper()
	return fieldsOf(t, fetchUser(t, baseURL, token, userID), "GET /users/{id}")
}

// setPlaceOK выбирает пункт и требует 200 с этим пунктом в CurrentUser.
func setPlaceOK(t *testing.T, baseURL, token string, want placePayload) {
	t.Helper()

	fields := fieldsOf(t, setPlaceReq(t, baseURL, token, map[string]any{"place_id": want.ID}), "PUT /me/place")
	requirePlace(t, fields, want, "ответ PUT /me/place")
}

// requireNoRequests требует, чтобы в DaData не пришло новых запросов.
func requireNoRequests(t *testing.T, fake *fakeDaData, before int, where string) {
	t.Helper()

	if got := fake.calls(); got != before {
		t.Errorf("%s: справочник спрашивать не должны были, а запросов пришло %d", where, got-before)
	}
}

// Пункты из andreykovo() так, как их должен отдать сервис.
var (
	sntAndreykovo = placePayload{ID: sntAndreykovoID, Name: "снт Андрейково", Area: "Дмитровский р-н, Московская обл"}
	derAndreykovo = placePayload{ID: derAndreykovoID, Name: "д Андрейково", Area: "Вологодский р-н, Вологодская обл"}
)

// ============================================================================
// GET /api/places — подсказки
// ============================================================================

// Подсказки — только для вошедшего; справочник без входа не спрашивается
// (ФТ-1).
func TestPlacesRequireAuth(t *testing.T) {
	baseURL, fake := startPlaces(t)
	fake.answer(andreykovo()...)

	requireUnauthorized(t, fetchPlaces(t, baseURL, "", "Андрейково"), "GET /places без токена")
	requireUnauthorized(t, fetchPlaces(t, baseURL, "не-токен", "Андрейково"), "GET /places с чужим токеном")
	requireNoRequests(t, fake, 0, "GET /places без входа")
}

// Подсказки из пользовательского сценария: 200, пункты в порядке
// справочника, name и area по ФТ-3 и ФТ-8.
func TestPlacesReturnsSuggestionsInOrder(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer(andreykovo()...)

	got := suggest(t, baseURL, user.token, "Андрейково")
	requirePlaces(t, got, []placePayload{sntAndreykovo, derAndreykovo}, "подсказки «Андрейково»")
}

// Справочник ничего не нашёл — 200 и пустой массив, не null (ФТ-1).
func TestPlacesEmptyAnswer(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer()

	got := suggest(t, baseURL, user.token, "Тьмутаракань")
	requirePlaces(t, got, nil, "пустые подсказки")
}

// q обрезается по краям; короче 2 или длиннее 100 символов — 400
// invalid_query, и справочник не спрашивается (ФТ-2).
func TestPlacesRejectsBadQuery(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer(andreykovo()...)

	cases := []struct {
		name string
		q    string
	}{
		{"пусто", ""},
		{"одни пробелы", "     "},
		{"один символ", "А"},
		{"один символ в пробелах", "   А   "},
		{"один символ в табуляциях и переводах строки", "\t\nА\n\t"},
		{"сто один символ", strings.Repeat("я", 101)},
		{"сто один символ в пробелах", "  " + strings.Repeat("я", 101) + "  "},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := fake.calls()
			requireCodeE(t, fetchPlaces(t, baseURL, user.token, c.q), http.StatusBadRequest, "invalid_query", fmt.Sprintf("GET /places?q=%q", c.q))
			requireNoRequests(t, fake, before, "невалидный q")
		})
	}
}

// Границы q: 2 и 100 символов (кириллица считается символами, а не
// байтами) проходят, в том числе с пробелами по краям (ФТ-2).
func TestPlacesAcceptsQueryAtBounds(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer(andreykovo()...)

	cases := []struct {
		name string
		q    string
	}{
		{"два символа", "Ан"},
		{"два символа в пробелах", "   Ан   "},
		{"сто символов", strings.Repeat("я", 100)},
		{"сто символов в пробелах", "  " + strings.Repeat("я", 100) + "  "},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := fake.calls()
			suggest(t, baseURL, user.token, c.q)
			if fake.calls() != before+1 {
				t.Errorf("q=%q: ожидался ровно один запрос в справочник, пришло %d", c.q, fake.calls()-before)
			}
		})
	}
}

// Ключа нет — 503 places_unavailable, справочник не спрашивается
// (ФТ-4, ФТ-14).
func TestPlacesUnavailableWithoutKey(t *testing.T) {
	fake := newFakeDaData(t)
	fake.answer(andreykovo()...)
	baseURL := startAPIWith(t, api.Config{PlacesURL: fake.URL()})
	user := newPlaceUser(t, baseURL, 1)

	requireCodeE(t, fetchPlaces(t, baseURL, user.token, "Андрейково"), http.StatusServiceUnavailable, "places_unavailable", "GET /places без ключа")
	requireNoRequests(t, fake, 0, "GET /places без ключа")
}

// Справочник ответил не 200 — 503 places_unavailable (ФТ-4).
func TestPlacesUnavailableWhenDaDataFails(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)

	for _, status := range []int{
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusBadRequest,
		http.StatusCreated,
	} {
		t.Run(fmt.Sprintf("статус %d", status), func(t *testing.T) {
			// Тело — нормальные подсказки: статус важнее тела.
			fake.answer(andreykovo()...)
			fake.fail(status)

			requireCodeE(t, fetchPlaces(t, baseURL, user.token, "Андрейково"), http.StatusServiceUnavailable, "places_unavailable", fmt.Sprintf("DaData ответил %d", status))
		})
	}
}

// Ответ справочника не разобрался — 503 places_unavailable (ФТ-4).
func TestPlacesUnavailableWhenAnswerIsGarbage(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)

	cases := []struct {
		name string
		body string
	}{
		{"не JSON", "<html>Сервис на обслуживании</html>"},
		{"пустое тело", ""},
		{"оборванный JSON", `{"suggestions": [{"value": "г Дмитров", "data": {`},
		{"массив вместо объекта", `[1, 2, 3]`},
		{"suggestions не массив", `{"suggestions": "много"}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake.garbage(c.body)
			requireCodeE(t, fetchPlaces(t, baseURL, user.token, "Андрейково"), http.StatusServiceUnavailable, "places_unavailable", "ответ DaData: "+c.name)
		})
	}
}

// Справочник не ответил за 5 секунд — 503 places_unavailable, и сервис
// не ждёт сильно дольше (ФТ-4). Тест идёт около пяти секунд.
func TestPlacesUnavailableOnTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("ждёт таймаут справочника — пять секунд")
	}

	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer(andreykovo()...)
	fake.slow(15 * time.Second)

	started := time.Now()
	resp := fetchPlaces(t, baseURL, user.token, "Андрейково")
	elapsed := time.Since(started)

	requireCodeE(t, resp, http.StatusServiceUnavailable, "places_unavailable", "DaData молчит")
	if elapsed > 10*time.Second {
		t.Errorf("сервис ждал справочник %s, а должен сдаваться через 5 секунд", elapsed)
	}
}

// ============================================================================
// Как сервер спрашивает DaData
// ============================================================================

// Запрос в справочник: POST <адрес>/suggest/address, ключ в Authorization,
// JSON туда и обратно, тело — обрезанный q и count 20 (ФТ-2, ФТ-6).
func TestPlacesRequestToDaData(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer(andreykovo()...)

	suggest(t, baseURL, user.token, "  снт Андрейково  ")

	if fake.calls() != 1 {
		t.Fatalf("на одну подсказку ожидался один запрос в справочник, пришло %d", fake.calls())
	}
	req := fake.last(t)

	if req.Method != http.MethodPost {
		t.Errorf("метод запроса в справочник %s, ожидался POST", req.Method)
	}
	if want := placesPrefix + "/suggest/address"; req.Path != want {
		t.Errorf("путь запроса в справочник %q, ожидался %q", req.Path, want)
	}
	if want := "Token " + placesKey; req.Authorization != want {
		t.Errorf("Authorization = %q, ожидался %q", req.Authorization, want)
	}
	if media, _, err := mime.ParseMediaType(req.ContentType); err != nil || media != "application/json" {
		t.Errorf("Content-Type = %q, ожидался application/json", req.ContentType)
	}
	if !strings.Contains(req.Accept, "application/json") {
		t.Errorf("Accept = %q, ожидался application/json", req.Accept)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &body); err != nil {
		t.Fatalf("тело запроса в справочник не JSON: %v (%s)", err, req.Body)
	}

	var query string
	if err := json.Unmarshal(body["query"], &query); err != nil {
		t.Errorf("query в теле запроса не строка: %s", body["query"])
	}
	if query != "снт Андрейково" {
		t.Errorf("query = %q, ожидался обрезанный текст «снт Андрейково»", query)
	}

	var count float64
	if err := json.Unmarshal(body["count"], &count); err != nil {
		t.Errorf("count в теле запроса не число: %s", body["count"])
	}
	if count != 20 {
		t.Errorf("count = %v, ожидалось 20", count)
	}
}

// ============================================================================
// Разбор ответа DaData
// ============================================================================

// Остаются только города (4), населённые пункты (6), планировочные
// структуры (65) и дополнительные территории (90) — строкой или числом;
// подсказки с пустым fias_id или без fias_level пропускаются (ФТ-7).
func TestPlacesFilterByFiasLevel(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)

	fake.answer(
		suggestion("Московская обл", "id-region", "1"),
		suggestion("Московская обл, г Дмитров", "id-city", "4"),
		suggestion("Московская обл, Дмитровский р-н", "id-district", "3"),
		suggestion("Московская обл, Дмитровский р-н, д Ивановка", "id-village", "6"),
		suggestion("Московская обл, Дмитровский р-н, д Ивановка, ул Садовая", "id-street", "7"),
		suggestion("Московская обл, Дмитровский р-н, снт Ромашка", "id-snt-gar", "65"),
		suggestion("Московская обл, г Дмитров, ул Садовая, д 1", "id-house", "8"),
		suggestion("Московская обл, Дмитровский р-н, тер СНТ Берёзка", "id-snt-fias", "90"),
		suggestion("Московская обл, Дмитровский р-н, снт Пустое", "", "65"),
		suggestion("Московская обл, Дмитровский р-н, д Безуровня", "id-no-level", nil),
		suggestion("Московская обл, Дмитровский р-н, д Числовая", "id-num-6", 6),
		suggestion("Московская обл, Дмитровский р-н, снт Числовое", "id-num-65", 65),
		suggestion("Московская обл, Дмитровский р-н, д Числовая, ул Лесная", "id-num-7", 7),
		suggestion("Тверская обл, г Тверь", "id-num-4", 4),
		suggestion("Московская обл, Дмитровский р-н, тер Числовая", "id-num-90", 90),
		suggestion("Московская обл, Дмитровский р-н, д Пустая строка уровня", "id-empty-level", ""),
	)

	got := suggest(t, baseURL, user.token, "Дмитров")
	requirePlaces(t, got, []placePayload{
		{ID: "id-city", Name: "г Дмитров", Area: "Московская обл"},
		{ID: "id-village", Name: "д Ивановка", Area: "Дмитровский р-н, Московская обл"},
		{ID: "id-snt-gar", Name: "снт Ромашка", Area: "Дмитровский р-н, Московская обл"},
		{ID: "id-snt-fias", Name: "тер СНТ Берёзка", Area: "Дмитровский р-н, Московская обл"},
		{ID: "id-num-6", Name: "д Числовая", Area: "Дмитровский р-н, Московская обл"},
		{ID: "id-num-65", Name: "снт Числовое", Area: "Дмитровский р-н, Московская обл"},
		{ID: "id-num-4", Name: "г Тверь", Area: "Тверская обл"},
		{ID: "id-num-90", Name: "тер Числовая", Area: "Дмитровский р-н, Московская обл"},
	}, "фильтр по fias_level")
}

// value делится по «, »: name — последняя часть, area — остальные
// в обратном порядке; части обрезаются, пустые отбрасываются; выше пункта
// ничего нет — area пустая строка, а не отсутствует (ФТ-3, ФТ-8).
func TestPlacesSplitValue(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)

	cases := []struct {
		name  string
		value string
		level any
		want  placePayload
	}{
		{"пример из спецификации", "Московская обл, Дмитровский р-н, снт Андрейково", "65",
			placePayload{Name: "снт Андрейково", Area: "Дмитровский р-н, Московская обл"}},
		{"четыре части", "Вологодская обл, Вологодский р-н, Майское с/п, д Андрейково", "6",
			placePayload{Name: "д Андрейково", Area: "Майское с/п, Вологодский р-н, Вологодская обл"}},
		{"две части", "Московская обл, г Дмитров", "4",
			placePayload{Name: "г Дмитров", Area: "Московская обл"}},
		{"город федерального значения", "г Москва", "4",
			placePayload{Name: "г Москва", Area: ""}},
		{"пробелы по краям частей и пустая часть", " Московская обл ,  , г Дмитров ", "4",
			placePayload{Name: "г Дмитров", Area: "Московская обл"}},
		{"разделитель в конце", "Московская обл, г Дмитров, ", "4",
			placePayload{Name: "г Дмитров", Area: "Московская обл"}},
		{"запятая без пробела не делит", "Московская обл, д Иваново,Петрово", "6",
			placePayload{Name: "д Иваново,Петрово", Area: "Московская обл"}},
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.want.ID = fmt.Sprintf("id-split-%d", i)
			fake.answer(suggestion(c.value, c.want.ID, c.level))

			got := suggest(t, baseURL, user.token, "Дмитров")
			requirePlaces(t, got, []placePayload{c.want}, fmt.Sprintf("value %q", c.value))
		})
	}
}

// Повтор fias_id в одном ответе пропускается: остаётся первый (ФТ-8).
func TestPlacesSkipDuplicateIDs(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)

	fake.answer(
		suggestion("Московская обл, Дмитровский р-н, снт Андрейково", sntAndreykovoID, "65"),
		suggestion("Московская обл, Дмитровский р-н, тер СНТ Андрейково", sntAndreykovoID, "90"),
		suggestion("Вологодская обл, Вологодский р-н, д Андрейково", derAndreykovoID, "6"),
		suggestion("Московская обл, Дмитровский р-н, снт Андрейково-2", sntAndreykovoID, 65),
	)

	got := suggest(t, baseURL, user.token, "Андрейково")
	requirePlaces(t, got, []placePayload{sntAndreykovo, derAndreykovo}, "дубли fias_id")
}

// В ответ идут первые 10 подсказок, оставшихся после фильтра и дублей:
// отброшенные места в десятке не занимают (ФТ-1, ФТ-8).
func TestPlacesAtMostTen(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)

	var answer []map[string]any
	var want []placePayload
	for i := 1; i <= 15; i++ {
		id := fmt.Sprintf("id-snt-%02d", i)
		name := fmt.Sprintf("снт Ромашка-%d", i)
		answer = append(answer, suggestion("Московская обл, Дмитровский р-н, "+name, id, "65"))
		if i <= 10 {
			want = append(want, placePayload{ID: id, Name: name, Area: "Дмитровский р-н, Московская обл"})
		}
		if i%3 == 0 {
			// Улица и повтор — между нормальными подсказками.
			answer = append(answer, suggestion("Московская обл, г Дмитров, ул Садовая", fmt.Sprintf("id-street-%d", i), "7"))
			answer = append(answer, suggestion("Московская обл, Дмитровский р-н, "+name, id, "65"))
		}
	}
	fake.answer(answer...)

	got := suggest(t, baseURL, user.token, "Ромашка")
	requirePlaces(t, got, want, "двадцать с лишним подсказок")
}

// ============================================================================
// PUT /api/me/place — выбор в профиле
// ============================================================================

// Выбор без входа — 401 (контракт).
func TestSetPlaceRequiresAuth(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer(andreykovo()...)
	suggest(t, baseURL, user.token, "Андрейково")

	requireUnauthorized(t, setPlaceReq(t, baseURL, "", map[string]any{"place_id": sntAndreykovoID}), "PUT /me/place без токена")
	requireNoPlace(t, meFields(t, baseURL, user.token), "GET /me после попытки без токена")
}

// Пункт из подсказки ставится: 200, CurrentUser с place, то же в GET /me
// (ФТ-5, ФТ-9, ФТ-11). Новый пользователь — без пункта.
func TestSetPlaceFromSuggestion(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)

	requireNoPlace(t, meFields(t, baseURL, user.token), "GET /me нового пользователя")

	fake.answer(andreykovo()...)
	suggest(t, baseURL, user.token, "Андрейково")

	resp := setPlaceReq(t, baseURL, user.token, map[string]any{"place_id": sntAndreykovoID})
	fields := fieldsOf(t, resp, "PUT /me/place")
	requirePlace(t, fields, sntAndreykovo, "ответ PUT /me/place")

	var id string
	_ = json.Unmarshal(fields["id"], &id)
	if id != user.id {
		t.Errorf("PUT /me/place вернул пользователя %q, ожидался %q (CurrentUser)", id, user.id)
	}
	if _, ok := fields["phone"]; !ok {
		t.Error("PUT /me/place: в ответе нет phone — это не CurrentUser")
	}

	requirePlace(t, meFields(t, baseURL, user.token), sntAndreykovo, "GET /me")
}

// У человека один пункт: новый заменяет прежний (ФТ-10).
func TestSetPlaceReplacesPrevious(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer(andreykovo()...)
	suggest(t, baseURL, user.token, "Андрейково")

	setPlaceOK(t, baseURL, user.token, sntAndreykovo)
	setPlaceOK(t, baseURL, user.token, derAndreykovo)
	requirePlace(t, meFields(t, baseURL, user.token), derAndreykovo, "GET /me после замены")

	// Тот же пункт ещё раз — не ошибка.
	setPlaceOK(t, baseURL, user.token, derAndreykovo)
	requirePlace(t, meFields(t, baseURL, user.token), derAndreykovo, "GET /me после повтора")
}

// {"place_id": null} и пустая строка убирают пункт (ФТ-9, ФТ-11).
func TestSetPlaceClears(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer(andreykovo()...)
	suggest(t, baseURL, user.token, "Андрейково")

	cases := []struct {
		name string
		body any
	}{
		{"null", map[string]any{"place_id": nil}},
		{"пустая строка", map[string]any{"place_id": ""}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setPlaceOK(t, baseURL, user.token, sntAndreykovo)

			fields := fieldsOf(t, setPlaceReq(t, baseURL, user.token, c.body), "PUT /me/place "+c.name)
			requireNoPlace(t, fields, "ответ PUT /me/place "+c.name)
			requireNoPlace(t, meFields(t, baseURL, user.token), "GET /me после "+c.name)

			// Убрать, когда и так пусто, — тоже 200.
			fields = fieldsOf(t, setPlaceReq(t, baseURL, user.token, c.body), "повторный PUT /me/place "+c.name)
			requireNoPlace(t, fields, "ответ повторного PUT /me/place "+c.name)
		})
	}
}

// Пункт, которого сервер не отдавал в подсказках, — 400 unknown_place,
// профиль не меняется, справочник не спрашивается (ФТ-5, ФТ-9). Сюда же —
// пункт, который справочник прислал, но сервис отфильтровал (ФТ-7).
func TestSetPlaceRejectsUnknown(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)

	fake.answer(append(andreykovo(),
		suggestion("Московская обл, г Дмитров, ул Садовая", "id-street", "7"),
		suggestion("Московская обл, Дмитровский р-н, снт Без уровня", "id-no-level", nil),
	)...)
	suggest(t, baseURL, user.token, "Андрейково")
	setPlaceOK(t, baseURL, user.token, sntAndreykovo)

	for _, id := range []string{
		"ffffffff-ffff-4fff-8fff-ffffffffffff",
		"id-street",
		"id-no-level",
		"не-идентификатор",
		strings.ToUpper(sntAndreykovoID) + "x",
	} {
		t.Run(id, func(t *testing.T) {
			before := fake.calls()
			requireCodeE(t, setPlaceReq(t, baseURL, user.token, map[string]any{"place_id": id}), http.StatusBadRequest, "unknown_place", "PUT /me/place "+id)
			requireNoRequests(t, fake, before, "PUT /me/place с неизвестным пунктом")
			requirePlace(t, meFields(t, baseURL, user.token), sntAndreykovo, "GET /me после отказа")
		})
	}
}

// Выбор не ходит в справочник: ранее отданный пункт ставится, даже когда
// DaData уже отвечает ошибкой (ФТ-9).
func TestSetPlaceWorksWhenDaDataIsDown(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer(andreykovo()...)
	suggest(t, baseURL, user.token, "Андрейково")

	fake.fail(http.StatusInternalServerError)
	requireCodeE(t, fetchPlaces(t, baseURL, user.token, "Андрейково"), http.StatusServiceUnavailable, "places_unavailable", "DaData лежит")

	before := fake.calls()
	setPlaceOK(t, baseURL, user.token, sntAndreykovo)
	requireNoRequests(t, fake, before, "PUT /me/place")
	requirePlace(t, meFields(t, baseURL, user.token), sntAndreykovo, "GET /me")
}

// ============================================================================
// Где виден пункт
// ============================================================================

// place есть во всех ответах с CurrentUser: PUT /me, PUT /me/privacy
// (ФТ-11).
func TestPlaceInEveryCurrentUserAnswer(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newPlaceUser(t, baseURL, 1)
	fake.answer(andreykovo()...)
	suggest(t, baseURL, user.token, "Андрейково")
	setPlaceOK(t, baseURL, user.token, sntAndreykovo)

	fields := fieldsOf(t, updateProfile(t, baseURL, user.token, map[string]any{"name": "Николай", "about": "Три сотки"}), "PUT /me")
	requirePlace(t, fields, sntAndreykovo, "ответ PUT /me")

	fields = fieldsOf(t, setPrivacy(t, baseURL, user.token, map[string]any{"closed": true}), "PUT /me/privacy")
	requirePlace(t, fields, sntAndreykovo, "ответ PUT /me/privacy")

	// Правка имени пункт не трогает.
	requirePlace(t, meFields(t, baseURL, user.token), sntAndreykovo, "GET /me после правки профиля")
}

// Сосед видит пункт в GET /users/{id}; пункта нет — нет и поля (или null)
// (ФТ-11, ФТ-12, сценарий п. 4–5: у обоих одна запись).
func TestPlaceVisibleToNeighbour(t *testing.T) {
	baseURL, fake := startPlaces(t)
	owner := newPlaceUser(t, baseURL, 1)
	neighbour := newPlaceUser(t, baseURL, 2)

	requireNoPlace(t, userFields(t, baseURL, neighbour.token, owner.id), "профиль без пункта глазами соседа")

	fake.answer(andreykovo()...)
	suggest(t, baseURL, owner.token, "Андрейково")
	setPlaceOK(t, baseURL, owner.token, sntAndreykovo)

	requirePlace(t, userFields(t, baseURL, neighbour.token, owner.id), sntAndreykovo, "профиль глазами соседа")
	requirePlace(t, userFields(t, baseURL, owner.token, owner.id), sntAndreykovo, "свой профиль по /users/{id}")

	// Сосед выбирает тот же пункт из своих подсказок — та же запись.
	suggest(t, baseURL, neighbour.token, "снт Андрейково")
	setPlaceOK(t, baseURL, neighbour.token, sntAndreykovo)
	requirePlace(t, userFields(t, baseURL, owner.token, neighbour.id), sntAndreykovo, "профиль соседа глазами первого")
}

// В закрытом профиле пункт виден не подписчику — как имя и «о себе»
// (ФТ-12).
func TestPlaceVisibleInClosedProfile(t *testing.T) {
	baseURL, fake := startPlaces(t)
	owner := newPlaceUser(t, baseURL, 1)
	stranger := newPlaceUser(t, baseURL, 2)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, owner.token, "Андрейково")
	setPlaceOK(t, baseURL, owner.token, derAndreykovo)
	setClosed(t, baseURL, owner.token, true)

	requirePlace(t, userFields(t, baseURL, stranger.token, owner.id), derAndreykovo, "закрытый профиль глазами не подписчика")
}

// Справочник переименовал пункт — следующая подсказка обновляет name
// и area у всех, кто его выбрал (ФТ-5, «Ограничения и edge cases»).
func TestPlaceRenamedOnNextSuggestion(t *testing.T) {
	baseURL, fake := startPlaces(t)
	first := newPlaceUser(t, baseURL, 1)
	second := newPlaceUser(t, baseURL, 2)
	viewer := newPlaceUser(t, baseURL, 3)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, first.token, "Андрейково")
	setPlaceOK(t, baseURL, first.token, sntAndreykovo)
	setPlaceOK(t, baseURL, second.token, sntAndreykovo)

	renamed := placePayload{ID: sntAndreykovoID, Name: "тер СНТ Андрейково-1", Area: "г.о. Дмитровский, Московская обл"}
	fake.answer(suggestion("Московская обл, г.о. Дмитровский, тер СНТ Андрейково-1", sntAndreykovoID, "65"))
	requirePlaces(t, suggest(t, baseURL, viewer.token, "Андрейково"), []placePayload{renamed}, "подсказка после переименования")

	requirePlace(t, meFields(t, baseURL, first.token), renamed, "GET /me первого")
	requirePlace(t, meFields(t, baseURL, second.token), renamed, "GET /me второго")
	requirePlace(t, userFields(t, baseURL, viewer.token, first.id), renamed, "профиль первого глазами соседа")

	// Пункт, которого в этой подсказке не было, остаётся прежним.
	setPlaceOK(t, baseURL, viewer.token, derAndreykovo)
	requirePlace(t, meFields(t, baseURL, viewer.token), derAndreykovo, "GET /me с непереименованным пунктом")
}

// Удаление аккаунта стирает привязку, запись в places остаётся, и другой
// человек может выбрать тот же пункт без новой подсказки (ФТ-13).
func TestPlaceSurvivesAccountDeletion(t *testing.T) {
	baseURL, fake := startPlaces(t)
	leaving := newPlaceUser(t, baseURL, 1)
	staying := newPlaceUser(t, baseURL, 2)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, leaving.token, "Андрейково")
	setPlaceOK(t, baseURL, leaving.token, sntAndreykovo)

	deleteMeOK(t, baseURL, leaving.token)

	if got := countSQL(t, `SELECT count(*) FROM users WHERE place_id = $1`, sntAndreykovoID); got != 0 {
		t.Errorf("после удаления аккаунта к пункту привязано %d пользователей, ожидалось 0", got)
	}
	if got := countSQL(t, `SELECT count(*) FROM places WHERE id = $1`, sntAndreykovoID); got != 1 {
		t.Errorf("после удаления аккаунта записей пункта в places %d, ожидалась 1", got)
	}

	before := fake.calls()
	setPlaceOK(t, baseURL, staying.token, sntAndreykovo)
	requireNoRequests(t, fake, before, "PUT /me/place после удаления чужого аккаунта")
	requirePlace(t, meFields(t, baseURL, staying.token), sntAndreykovo, "GET /me оставшегося")
}
