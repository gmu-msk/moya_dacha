package tests

// Тесты населённого пункта по геолокации (specs/026-places-nearby.md).
//
// Пункты рядом сервис берёт у DaData (поиск адреса по координатам) сам,
// поэтому тест, как и в 025, подменяет DaData фейком fakeDaData из
// places_test.go: тот отдаёт заданные подсказки (или ошибку, мусор,
// задержку) и запоминает каждый пришедший запрос. Здесь подсказки —
// дома и участки рядом с точкой, пункт сервис берёт из их полей data
// (ФТ-7, ФТ-8). Адрес и ключ — те же поля api.Config (ФТ-10).

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// Точка из примера контракта: дача в Дмитровском районе.
const (
	nearbyLat = "56.3412"
	nearbyLon = "37.5203"
)

// Идентификаторы ФИАС пунктов, которые фейк отдаёт в проверках рядом.
const (
	nearSntID      = "7a1b2c3d-0000-4a00-8000-000000000001" // снт Андрейково
	nearDerID      = "7a1b2c3d-0000-4a00-8000-000000000002" // д Андрейково
	nearDmitrovID  = "7a1b2c3d-0000-4a00-8000-000000000003" // г Дмитров
	nearMoscowID   = "0c5b2444-70a0-4932-980c-b4dc0d3f02b5" // г Москва
	nearVnukovoID  = "7a1b2c3d-0000-4a00-8000-000000000005" // п Внуково в Москве
	nearRomashkaID = "7a1b2c3d-0000-4a00-8000-000000000006" // снт Ромашка в г Дмитров
)

// Пункты так, как их должен отдать сервис.
var (
	nearSnt = placePayload{ID: nearSntID, Name: "снт Андрейково", Area: "Дмитровский р-н, Московская обл"}
	nearDer = placePayload{ID: nearDerID, Name: "д Андрейково", Area: "Дмитровский р-н, Московская обл"}
)

// --- Подсказки фейка ------------------------------------------------------

// houseNo — счётчик номеров домов, чтобы у подсказок были разные value
// и fias_id, как у настоящих соседних домов.
var houseNo int

// geoSuggestion — подсказка DaData о доме рядом с точкой. fields — поля
// data, которые важны проверке (settlement_fias_id, city_with_type и т. д.);
// значение nil уходит как JSON null — так DaData отдаёт пустые поля.
// Остальное — как у настоящего дома: свой fias_id и fias_level «8»,
// чтобы фильтр по уровню из 025 сюда не просочился.
func geoSuggestion(fields map[string]any) map[string]any {
	houseNo++

	data := map[string]any{
		"fias_id":              fmt.Sprintf("dddddddd-0000-4000-8000-%012d", houseNo),
		"fias_level":           "8",
		"house":                fmt.Sprint(houseNo),
		"postal_code":          "141800",
		"geo_lat":              "56.3409",
		"geo_lon":              "37.5211",
		"settlement_fias_id":   nil,
		"settlement_with_type": nil,
		"city_fias_id":         nil,
		"city_with_type":       nil,
		"area_with_type":       nil,
		"region_with_type":     nil,
	}
	for k, v := range fields {
		data[k] = v
	}

	return map[string]any{
		"value":              fmt.Sprintf("какой-то адрес, д %d", houseNo),
		"unrestricted_value": fmt.Sprintf("141800, какой-то адрес, д %d", houseNo),
		"data":               data,
	}
}

// sntHouse — дом в снт Андрейково Дмитровского района.
func sntHouse() map[string]any {
	return geoSuggestion(map[string]any{
		"settlement_fias_id":   nearSntID,
		"settlement_with_type": "снт Андрейково",
		"area_with_type":       "Дмитровский р-н",
		"region_with_type":     "Московская обл",
	})
}

// derHouse — дом в д Андрейково того же района.
func derHouse() map[string]any {
	return geoSuggestion(map[string]any{
		"settlement_fias_id":   nearDerID,
		"settlement_with_type": "д Андрейково",
		"area_with_type":       "Дмитровский р-н",
		"region_with_type":     "Московская обл",
	})
}

// --- Хелперы --------------------------------------------------------------

// nearbyPhone — номер n-го участника теста.
func nearbyPhone(n int) string {
	return fmt.Sprintf("+7 (900) 726-00-%02d", n)
}

// newNearbyUser регистрирует n-го участника теста.
func newNearbyUser(t *testing.T, baseURL string, n int) dachnik {
	t.Helper()

	token, id := signIn(t, baseURL, nearbyPhone(n))

	return dachnik{token: token, id: id}
}

// fetchNearbyRaw запрашивает пункты рядом с запросом query как есть
// (без «?»): так можно не передать параметр или передать мусор.
func fetchNearbyRaw(t *testing.T, baseURL, token, query string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, baseURL+"/places/nearby?"+query, token, nil)
}

// fetchNearby запрашивает пункты рядом с точкой.
func fetchNearby(t *testing.T, baseURL, token, lat, lon string) *http.Response {
	t.Helper()
	return fetchNearbyRaw(t, baseURL, token, url.Values{"lat": {lat}, "lon": {lon}}.Encode())
}

// nearby — самый частый случай: пункты рядом, которые должны прийти.
func nearby(t *testing.T, baseURL, token string) []placePayload {
	t.Helper()
	return placesOK(t, fetchNearby(t, baseURL, token, nearbyLat, nearbyLon), "GET /places/nearby")
}

// ============================================================================
// GET /api/places/nearby — пункты рядом
// ============================================================================

// Пункты рядом — только для вошедшего; справочник без входа не
// спрашивается (ФТ-1).
func TestPlacesNearbyRequireAuth(t *testing.T) {
	baseURL, fake := startPlaces(t)
	fake.answer(sntHouse())

	requireUnauthorized(t, fetchNearby(t, baseURL, "", nearbyLat, nearbyLon), "GET /places/nearby без токена")
	requireUnauthorized(t, fetchNearby(t, baseURL, "не-токен", nearbyLat, nearbyLon), "GET /places/nearby с чужим токеном")
	requireNoRequests(t, fake, 0, "GET /places/nearby без входа")
}

// Пользовательский сценарий: рядом с дачей дома снт и деревни — 200,
// PlaceList с двумя пунктами в порядке справочника (ФТ-1, ФТ-7, ФТ-8).
func TestPlacesNearbyReturnsPlaces(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)
	fake.answer(sntHouse(), derHouse())

	got := nearby(t, baseURL, user.token)
	requirePlaces(t, got, []placePayload{nearSnt, nearDer}, "пункты рядом с дачей")
}

// В радиусе нет ни одного адреса — 200 и пустой массив, не null; нет
// поля suggestions — тоже пусто (ФТ-7, «Ограничения»).
func TestPlacesNearbyEmpty(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)

	t.Run("пустой suggestions", func(t *testing.T) {
		fake.answer()
		requirePlaces(t, nearby(t, baseURL, user.token), nil, "пустой ответ справочника")
	})

	t.Run("нет поля suggestions", func(t *testing.T) {
		fake.garbage(`{}`)
		requirePlaces(t, nearby(t, baseURL, user.token), nil, "ответ справочника без suggestions")
	})
}

// Нет lat или lon, или значение не число — 400 invalid_request, справочник
// не спрашивается (ФТ-2).
func TestPlacesNearbyRejectsMalformedQuery(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)
	fake.answer(sntHouse())

	cases := []struct {
		name  string
		query string
	}{
		{"без параметров", ""},
		{"нет lon", "lat=" + nearbyLat},
		{"нет lat", "lon=" + nearbyLon},
		{"lat не число", url.Values{"lat": {"север"}, "lon": {nearbyLon}}.Encode()},
		{"lon не число", url.Values{"lat": {nearbyLat}, "lon": {"восток"}}.Encode()},
		{"lat с запятой", url.Values{"lat": {"56,3412"}, "lon": {nearbyLon}}.Encode()},
		{"lon с единицами", url.Values{"lat": {nearbyLat}, "lon": {"37.5203°"}}.Encode()},
		{"lat пустой", "lat=&lon=" + nearbyLon},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := fake.calls()
			requireCodeE(t, fetchNearbyRaw(t, baseURL, user.token, c.query), http.StatusBadRequest, "invalid_request", "GET /places/nearby?"+c.query)
			requireNoRequests(t, fake, before, "кривые координаты")
		})
	}
}

// Число вне пределов, NaN и бесконечность — 400 invalid_location,
// справочник не спрашивается (ФТ-2).
func TestPlacesNearbyRejectsOutOfRange(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)
	fake.answer(sntHouse())

	cases := []struct {
		name     string
		lat, lon string
	}{
		{"lat чуть больше 90", "90.0001", nearbyLon},
		{"lat чуть меньше −90", "-90.0001", nearbyLon},
		{"lon чуть больше 180", nearbyLat, "180.0001"},
		{"lon чуть меньше −180", nearbyLat, "-180.0001"},
		{"lat далеко за пределами", "1e10", nearbyLon},
		{"lon 360", nearbyLat, "360"},
		{"оба вне пределов", "91", "181"},
		{"lat NaN", "NaN", nearbyLon},
		{"lon NaN", nearbyLat, "NaN"},
		{"lat Inf", "Inf", nearbyLon},
		{"lat +Inf", "+Inf", nearbyLon},
		{"lon -Inf", nearbyLat, "-Inf"},
		{"lon Infinity", nearbyLat, "Infinity"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := fake.calls()
			requireCodeE(t, fetchNearby(t, baseURL, user.token, c.lat, c.lon), http.StatusBadRequest, "invalid_location", fmt.Sprintf("lat=%s lon=%s", c.lat, c.lon))
			requireNoRequests(t, fake, before, "координаты вне пределов")
		})
	}
}

// Границы включаются: ровно 90, −90, 180 и −180 — допустимые координаты,
// справочник спрашивается (ФТ-2).
func TestPlacesNearbyAcceptsBounds(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)
	fake.answer()

	cases := []struct {
		name     string
		lat, lon string
	}{
		{"северный полюс", "90", "0"},
		{"южный полюс", "-90", "0"},
		{"180-й меридиан", "0", "180"},
		{"−180-й меридиан", "0", "-180"},
		{"все углы сразу", "90.0", "-180.0"},
		{"ноль", "0", "0"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := fake.calls()
			placesOK(t, fetchNearby(t, baseURL, user.token, c.lat, c.lon), fmt.Sprintf("lat=%s lon=%s", c.lat, c.lon))
			if fake.calls() != before+1 {
				t.Errorf("lat=%s lon=%s: ожидался ровно один запрос в справочник, пришло %d", c.lat, c.lon, fake.calls()-before)
			}
		})
	}
}

// Ключа нет — 503 places_unavailable, справочник не спрашивается
// (ФТ-3, ФТ-10).
func TestPlacesNearbyUnavailableWithoutKey(t *testing.T) {
	fake := newFakeDaData(t)
	fake.answer(sntHouse())
	baseURL := startAPIWith(t, api.Config{PlacesURL: fake.URL()})
	user := newNearbyUser(t, baseURL, 1)

	requireCodeE(t, fetchNearby(t, baseURL, user.token, nearbyLat, nearbyLon), http.StatusServiceUnavailable, "places_unavailable", "GET /places/nearby без ключа")
	requireNoRequests(t, fake, 0, "GET /places/nearby без ключа")
}

// Справочник ответил не 200 — 503 places_unavailable (ФТ-3).
func TestPlacesNearbyUnavailableWhenDaDataFails(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)

	for _, status := range []int{
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusBadRequest,
		http.StatusNotFound,
		http.StatusCreated,
	} {
		t.Run(fmt.Sprintf("статус %d", status), func(t *testing.T) {
			// Тело — нормальные подсказки: статус важнее тела.
			fake.answer(sntHouse())
			fake.fail(status)

			requireCodeE(t, fetchNearby(t, baseURL, user.token, nearbyLat, nearbyLon), http.StatusServiceUnavailable, "places_unavailable", fmt.Sprintf("DaData ответил %d", status))
		})
	}
}

// Ответ справочника не разобрался — 503 places_unavailable (ФТ-3).
func TestPlacesNearbyUnavailableWhenAnswerIsGarbage(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)

	cases := []struct {
		name string
		body string
	}{
		{"не JSON", "<html>Сервис на обслуживании</html>"},
		{"пустое тело", ""},
		{"оборванный JSON", `{"suggestions": [{"value": "снт Андрейково", "data": {`},
		{"массив вместо объекта", `[1, 2, 3]`},
		{"suggestions не массив", `{"suggestions": "много"}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake.garbage(c.body)
			requireCodeE(t, fetchNearby(t, baseURL, user.token, nearbyLat, nearbyLon), http.StatusServiceUnavailable, "places_unavailable", "ответ DaData: "+c.name)
		})
	}
}

// Справочник не ответил за 5 секунд — 503 places_unavailable, и сервис
// не ждёт сильно дольше (ФТ-3). Тест идёт около пяти секунд.
func TestPlacesNearbyUnavailableOnTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("ждёт таймаут справочника — пять секунд")
	}

	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)
	fake.answer(sntHouse())
	fake.slow(15 * time.Second)

	started := time.Now()
	resp := fetchNearby(t, baseURL, user.token, nearbyLat, nearbyLon)
	elapsed := time.Since(started)

	requireCodeE(t, resp, http.StatusServiceUnavailable, "places_unavailable", "DaData молчит")
	if elapsed > 10*time.Second {
		t.Errorf("сервис ждал справочник %s, а должен сдаваться через 5 секунд", elapsed)
	}
}

// ============================================================================
// Как сервер спрашивает DaData
// ============================================================================

// Запрос в справочник: POST <адрес>/geolocate/address, заголовки как в 025,
// тело — lat и lon числами, radius_meters 1000, count 20 (ФТ-6, ФТ-10).
func TestPlacesNearbyRequestToDaData(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)
	fake.answer(sntHouse())

	placesOK(t, fetchNearby(t, baseURL, user.token, "56.341234", "-37.520345"), "GET /places/nearby")

	if fake.calls() != 1 {
		t.Fatalf("на один запрос пунктов рядом ожидался один запрос в справочник, пришло %d", fake.calls())
	}
	req := fake.last(t)

	if req.Method != http.MethodPost {
		t.Errorf("метод запроса в справочник %s, ожидался POST", req.Method)
	}
	if want := placesPrefix + "/geolocate/address"; req.Path != want {
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

	numbers := []struct {
		field string
		want  float64
	}{
		{"lat", 56.341234},
		{"lon", -37.520345},
		{"radius_meters", 1000},
		{"count", 20},
	}
	for _, n := range numbers {
		raw, ok := body[n.field]
		if !ok {
			t.Errorf("в теле запроса в справочник нет %s: %s", n.field, req.Body)
			continue
		}
		var got float64
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Errorf("%s в теле запроса не число: %s", n.field, raw)
			continue
		}
		if got != n.want {
			t.Errorf("%s = %v, ожидалось %v", n.field, got, n.want)
		}
	}
}

// ============================================================================
// Разбор ответа DaData
// ============================================================================

// Какой пункт берётся из подсказки: settlement_fias_id — населённый пункт,
// иначе city_fias_id — город, иначе подсказка пропускается; пропускается
// и подсказка с пустым name (ФТ-7).
func TestPlacesNearbyPicksPlaceFromData(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)

	fake.answer(
		// Ни населённого пункта, ни города — только регион и улица.
		geoSuggestion(map[string]any{
			"region_with_type": "Московская обл",
			"street_with_type": "ул Лесная",
		}),
		// Дом в снт: снт, а не город и не дом.
		sntHouse(),
		// Населённый пункт есть, но без названия — пропуск, а не город.
		geoSuggestion(map[string]any{
			"settlement_fias_id":   "7a1b2c3d-0000-4a00-8000-0000000000aa",
			"settlement_with_type": "",
			"city_fias_id":         nearDmitrovID,
			"city_with_type":       "г Дмитров",
			"area_with_type":       "Дмитровский р-н",
			"region_with_type":     "Московская обл",
		}),
		// Город без названия — пропуск.
		geoSuggestion(map[string]any{
			"city_fias_id":     "7a1b2c3d-0000-4a00-8000-0000000000bb",
			"city_with_type":   nil,
			"region_with_type": "Московская обл",
		}),
		// Пустой settlement_fias_id — как его нет: берётся город.
		geoSuggestion(map[string]any{
			"settlement_fias_id": "",
			"city_fias_id":       nearDmitrovID,
			"city_with_type":     "г Дмитров",
			"area_with_type":     "Дмитровский р-н",
			"region_with_type":   "Московская обл",
		}),
		// Деревня.
		derHouse(),
	)

	got := nearby(t, baseURL, user.token)
	requirePlaces(t, got, []placePayload{
		nearSnt,
		{ID: nearDmitrovID, Name: "г Дмитров", Area: "Дмитровский р-н, Московская обл"},
		nearDer,
	}, "пункты из полей data")
}

// area: city_with_type (только у населённого пункта), area_with_type,
// region_with_type через «, »; пустые и повторяющие name или уже взятую
// часть пропускаются (ФТ-8).
func TestPlacesNearbyArea(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)

	cases := []struct {
		name   string
		fields map[string]any
		want   placePayload
	}{
		{
			"снт в районе — пример из спецификации",
			map[string]any{
				"settlement_fias_id":   nearSntID,
				"settlement_with_type": "снт Андрейково",
				"area_with_type":       "Дмитровский р-н",
				"region_with_type":     "Московская обл",
			},
			nearSnt,
		},
		{
			"город Москва — area пустая",
			map[string]any{
				"city_fias_id":     nearMoscowID,
				"city_with_type":   "г Москва",
				"area_with_type":   nil,
				"region_with_type": "г Москва",
			},
			placePayload{ID: nearMoscowID, Name: "г Москва", Area: ""},
		},
		{
			"город Москва с пустыми строками вместо null",
			map[string]any{
				"city_fias_id":     nearMoscowID,
				"city_with_type":   "г Москва",
				"area_with_type":   "",
				"region_with_type": "г Москва",
			},
			placePayload{ID: nearMoscowID, Name: "г Москва", Area: ""},
		},
		{
			"город в районе — city_with_type не повторяется в area",
			map[string]any{
				"city_fias_id":     nearDmitrovID,
				"city_with_type":   "г Дмитров",
				"area_with_type":   "Дмитровский р-н",
				"region_with_type": "Московская обл",
			},
			placePayload{ID: nearDmitrovID, Name: "г Дмитров", Area: "Дмитровский р-н, Московская обл"},
		},
		{
			"населённый пункт внутри города — город первым",
			map[string]any{
				"settlement_fias_id":   nearRomashkaID,
				"settlement_with_type": "снт Ромашка",
				"city_fias_id":         nearDmitrovID,
				"city_with_type":       "г Дмитров",
				"area_with_type":       "Дмитровский р-н",
				"region_with_type":     "Московская обл",
			},
			placePayload{ID: nearRomashkaID, Name: "снт Ромашка", Area: "г Дмитров, Дмитровский р-н, Московская обл"},
		},
		{
			"населённый пункт в Москве — регион совпадает с городом",
			map[string]any{
				"settlement_fias_id":   nearVnukovoID,
				"settlement_with_type": "п Внуково",
				"city_fias_id":         nearMoscowID,
				"city_with_type":       "г Москва",
				"area_with_type":       nil,
				"region_with_type":     "г Москва",
			},
			placePayload{ID: nearVnukovoID, Name: "п Внуково", Area: "г Москва"},
		},
		{
			"часть, совпадающая с name, пропускается",
			map[string]any{
				"settlement_fias_id":   nearDerID,
				"settlement_with_type": "д Андрейково",
				"area_with_type":       "д Андрейково",
				"region_with_type":     "Московская обл",
			},
			placePayload{ID: nearDerID, Name: "д Андрейково", Area: "Московская обл"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake.answer(geoSuggestion(c.fields))
			requirePlaces(t, nearby(t, baseURL, user.token), []placePayload{c.want}, c.name)
		})
	}
}

// Повтор id пропускается (рядом много домов одного снт), порядок — как
// у справочника, первое появление (ФТ-9).
func TestPlacesNearbySkipsDuplicateIDs(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)
	fake.answer(sntHouse(), sntHouse(), derHouse(), sntHouse(), derHouse(), sntHouse())

	got := nearby(t, baseURL, user.token)
	requirePlaces(t, got, []placePayload{nearSnt, nearDer}, "дома одного снт и одной деревни")
}

// В ответ идут первые 10 пунктов — после пропуска повторов (ФТ-1, ФТ-9).
func TestPlacesNearbyAtMostTen(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)

	settlement := func(i int) map[string]any {
		return geoSuggestion(map[string]any{
			"settlement_fias_id":   fmt.Sprintf("eeeeeeee-0000-4000-8000-%012d", i),
			"settlement_with_type": fmt.Sprintf("снт Пункт-%d", i),
			"area_with_type":       "Дмитровский р-н",
			"region_with_type":     "Московская обл",
		})
	}

	// 20 подсказок: каждый пункт повторён дважды подряд в начале, так что
	// десятый различный пункт стоит за десятой подсказкой.
	var suggestions []map[string]any
	for i := 1; i <= 4; i++ {
		suggestions = append(suggestions, settlement(i), settlement(i))
	}
	for i := 5; i <= 16; i++ {
		suggestions = append(suggestions, settlement(i))
	}
	fake.answer(suggestions...)

	got := nearby(t, baseURL, user.token)

	var want []placePayload
	for i := 1; i <= 10; i++ {
		want = append(want, placePayload{
			ID:   fmt.Sprintf("eeeeeeee-0000-4000-8000-%012d", i),
			Name: fmt.Sprintf("снт Пункт-%d", i),
			Area: "Дмитровский р-н, Московская обл",
		})
	}
	requirePlaces(t, got, want, "первые 10 пунктов")
}

// ============================================================================
// Пункт рядом в профиле
// ============================================================================

// Отданный пункт запомнен: его можно поставить через PUT /me/place, и он
// виден в GET /me (ФТ-4).
func TestPlacesNearbyPlaceCanBeSet(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)
	fake.answer(sntHouse(), derHouse())

	nearby(t, baseURL, user.token)

	// Справочник после этого может лечь: выбор его не спрашивает.
	fake.fail(http.StatusInternalServerError)
	before := fake.calls()

	setPlaceOK(t, baseURL, user.token, nearSnt)
	requirePlace(t, meFields(t, baseURL, user.token), nearSnt, "GET /me после выбора пункта рядом")

	setPlaceOK(t, baseURL, user.token, nearDer)
	requirePlace(t, meFields(t, baseURL, user.token), nearDer, "GET /me после выбора второго пункта рядом")

	requireNoRequests(t, fake, before, "PUT /me/place с пунктом рядом")
}

// Пункт, уже известный по подсказке по названию, при выдаче рядом
// обновляет name и area — у всех, кто его выбрал (ФТ-4, 025 ФТ-5).
func TestPlacesNearbyUpdatesKnownPlace(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)

	fake.answer(suggestion("Московская обл, Дмитровский р-н, снт Андрейково-старое", nearSntID, "65"))
	suggest(t, baseURL, user.token, "Андрейково")
	setPlaceOK(t, baseURL, user.token, placePayload{ID: nearSntID, Name: "снт Андрейково-старое", Area: "Дмитровский р-н, Московская обл"})

	fake.answer(sntHouse())
	nearby(t, baseURL, user.token)

	requirePlace(t, meFields(t, baseURL, user.token), nearSnt, "GET /me после выдачи пункта рядом")
}

// Координаты не записываются в базу: после запросов пунктов рядом ни
// в одной колонке ни одной таблицы нет присланных чисел (ФТ-5). Лог
// сервиса из теста не виден, его эта проверка не покрывает.
func TestPlacesNearbyDoesNotStoreCoordinates(t *testing.T) {
	baseURL, fake := startPlaces(t)
	user := newNearbyUser(t, baseURL, 1)
	fake.answer(sntHouse(), derHouse())

	// Координаты с редкими дробными частями, чтобы не совпасть случайно.
	const lat, lon = "56.3417291", "37.5208463"
	placesOK(t, fetchNearby(t, baseURL, user.token, lat, lon), "GET /places/nearby")

	// И ошибочный запрос тоже не должен оставить следа.
	fake.fail(http.StatusInternalServerError)
	fetchNearby(t, baseURL, user.token, lat, lon)

	pool := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name <> 'goose_db_version'
		ORDER BY table_name, ordinal_position`)
	if err != nil {
		t.Fatalf("не удалось прочитать список колонок: %v", err)
	}
	type column struct{ table, name string }
	var columns []column
	for rows.Next() {
		var c column
		if err := rows.Scan(&c.table, &c.name); err != nil {
			t.Fatalf("не удалось прочитать колонку: %v", err)
		}
		columns = append(columns, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("не удалось прочитать список колонок: %v", err)
	}
	if len(columns) == 0 {
		t.Fatal("в схеме public нет ни одной колонки — проверять нечего")
	}

	// Дробные части без целой: так найдутся и строка «56.3417291», и число
	// в колонке double precision, и координаты внутри JSON.
	needles := []string{"3417291", "5208463"}

	for _, c := range columns {
		for _, needle := range needles {
			query := fmt.Sprintf(`SELECT count(*) FROM %q WHERE %q::text LIKE '%%' || $1 || '%%'`, c.table, c.name)
			var count int
			if err := pool.QueryRow(ctx, query, needle).Scan(&count); err != nil {
				t.Fatalf("не удалось проверить %s.%s: %v", c.table, c.name, err)
			}
			if count != 0 {
				t.Errorf("в %s.%s нашлось %d строк с координатой (%s)", c.table, c.name, count, needle)
			}
		}
	}
}
