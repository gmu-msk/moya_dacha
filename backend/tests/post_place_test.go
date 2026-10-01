package tests

// Тесты геометки поста и расстояния до него (specs/027-post-place.md).
// Написаны по спецификации и контракту, не глядя в реализацию (ADR-0002).
//
// Пункты сервис узнаёт из подсказок DaData, поэтому здесь, как в 025 и
// 026, DaData подменён фейком fakeDaData из places_test.go; подсказки
// по названию и пункты рядом теперь несут data.geo_lat и data.geo_lon
// (ФТ-12). Координаты наружу не отдаются (ФТ-10), поэтому то, какие
// координаты сервер запомнил, тест видит только по distance_km: смотрящий
// стоит в пункте с известными координатами, пост — в проверяемом пункте.
//
// Расстояния подобраны на одном меридиане: там дуга большого круга —
// ровно R·Δφ, и градус широты при R = 6371 км равен 111,19492664 км.

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// ppKmPerDegree — километров в градусе дуги большого круга при R = 6371 км.
const ppKmPerDegree = 6371 * math.Pi / 180

// Точка, от которой на север откладываются расстояния в проверках.
const (
	ppBaseLat = 45.0
	ppBaseLon = 37.5
)

// ppArea — area пунктов, которые этот файл кладёт в подсказки.
const ppArea = "Дмитровский р-н, Московская обл"

// ppNoField — значение координаты, при котором поля в data нет вовсе.
type ppNoField struct{}

// ppPlaceNo — счётчик пунктов, чтобы у каждого были свои id и название.
var ppPlaceNo int

// --- Хелперы --------------------------------------------------------------

// ppPhone — номер n-го участника теста; не пересекается с другими файлами.
func ppPhone(n int) string {
	return fmt.Sprintf("+7 (900) 727-00-%02d", n)
}

// newPPUser регистрирует n-го участника теста.
func newPPUser(t *testing.T, baseURL string, n int) dachnik {
	t.Helper()

	token, id := signIn(t, baseURL, ppPhone(n))

	return dachnik{token: token, id: id}
}

// ppCoord — координата строкой, как её отдаёт DaData.
func ppCoord(v float64) string {
	return strconv.FormatFloat(v, 'f', 8, 64)
}

// ppNorth — точка в km километрах к северу от базовой, строками.
func ppNorth(km float64) (lat, lon string) {
	return ppCoord(ppBaseLat + km/ppKmPerDegree), ppCoord(ppBaseLon)
}

// ppSuggestion — подсказка по названию для пункта с этим id и именем;
// lat и lon кладутся в data как есть (ppNoField{} — поля нет).
func ppSuggestion(id, name string, lat, lon any) map[string]any {
	s := suggestion("Московская обл, Дмитровский р-н, "+name, id, "65")
	data := s["data"].(map[string]any)
	if _, skip := lat.(ppNoField); !skip {
		data["geo_lat"] = lat
	}
	if _, skip := lon.(ppNoField); !skip {
		data["geo_lon"] = lon
	}

	return s
}

// ppNewPlace — новый пункт, сервису ещё не известный.
func ppNewPlace() placePayload {
	ppPlaceNo++

	return placePayload{
		ID:   fmt.Sprintf("27270000-0000-4000-8000-%012d", ppPlaceNo),
		Name: fmt.Sprintf("снт Точка-%d", ppPlaceNo),
		Area: ppArea,
	}
}

// ppSuggestPlace отдаёт пункт p подсказкой по названию с этими координатами
// и требует, чтобы сервис вернул его как пункт — с координатами или без
// (ФТ-11, последний абзац).
func ppSuggestPlace(t *testing.T, baseURL string, fake *fakeDaData, token string, p placePayload, lat, lon any) {
	t.Helper()

	fake.answer(ppSuggestion(p.ID, p.Name, lat, lon))
	requirePlaces(t, suggest(t, baseURL, token, "Точка"), []placePayload{p},
		fmt.Sprintf("подсказка %s с координатами %v, %v", p.Name, lat, lon))
}

// ppLearn — новый пункт, отданный подсказкой по названию с координатами.
func ppLearn(t *testing.T, baseURL string, fake *fakeDaData, token string, lat, lon any) placePayload {
	t.Helper()

	p := ppNewPlace()
	ppSuggestPlace(t, baseURL, fake, token, p, lat, lon)

	return p
}

// ppLearnNorth — новый пункт в km километрах к северу от базовой точки.
func ppLearnNorth(t *testing.T, baseURL string, fake *fakeDaData, token string, km float64) placePayload {
	t.Helper()

	lat, lon := ppNorth(km)

	return ppLearn(t, baseURL, fake, token, lat, lon)
}

// ppHouse — дом пункта p в ответе пунктов рядом с этими координатами.
func ppHouse(p placePayload, lat, lon any) map[string]any {
	return geoSuggestion(map[string]any{
		"settlement_fias_id":   p.ID,
		"settlement_with_type": p.Name,
		"area_with_type":       "Дмитровский р-н",
		"region_with_type":     "Московская обл",
		"geo_lat":              lat,
		"geo_lon":              lon,
	})
}

// ppPublish публикует пост с одной фотографией; extra — поля тела сверх
// media_ids (place_id, caption, visibility).
func ppPublish(t *testing.T, baseURL, token string, extra map[string]any) *http.Response {
	t.Helper()

	photo := photoOf(t, baseURL, token, 60, 40)
	body := map[string]any{"media_ids": []string{photo.ID}}
	for k, v := range extra {
		body[k] = v
	}

	return createPost(t, baseURL, token, body)
}

// ppFields разбирает ответ-объект как словарь полей, требуя статус.
func ppFields(t *testing.T, resp *http.Response, status int, where string) map[string]json.RawMessage {
	t.Helper()

	if resp.StatusCode != status {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус %d, получен %d %s", where, status, resp.StatusCode, code)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON(t, resp), &fields); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON-объект: %v", where, err)
	}

	return fields
}

// ppID — id поста из словаря полей.
func ppID(t *testing.T, fields map[string]json.RawMessage, where string) string {
	t.Helper()

	var id string
	if err := json.Unmarshal(fields["id"], &id); err != nil || id == "" {
		t.Fatalf("%s: у поста нет id: %s", where, fields["id"])
	}

	return id
}

// ppPost публикует пост с этим местом (пустое — поля place_id нет),
// требует 201 и возвращает ответ.
func ppPost(t *testing.T, baseURL, token, placeID string) map[string]json.RawMessage {
	t.Helper()

	extra := map[string]any{"caption": postCaption}
	if placeID != "" {
		extra["place_id"] = placeID
	}

	return ppFields(t, ppPublish(t, baseURL, token, extra), http.StatusCreated, "POST /posts")
}

// ppPostID — то же, но нужен только id поста.
func ppPostID(t *testing.T, baseURL, token, placeID string) string {
	t.Helper()
	return ppID(t, ppPost(t, baseURL, token, placeID), "POST /posts")
}

// ppRequirePlaceShape требует, чтобы у place поста не было ничего, кроме
// id, name и area: координаты наружу не отдаются (ФТ-10).
func ppRequirePlaceShape(t *testing.T, fields map[string]json.RawMessage, where string) {
	t.Helper()

	raw, ok := fields["place"]
	if !ok || string(raw) == "null" {
		return
	}

	var shape map[string]json.RawMessage
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatalf("%s: place не объект: %s", where, raw)
	}
	for key := range shape {
		if key != "id" && key != "name" && key != "area" {
			t.Errorf("%s: в place лишнее поле %q: %s", where, key, raw)
		}
	}
}

// ppPostPlace требует у поста ровно это место.
func ppPostPlace(t *testing.T, fields map[string]json.RawMessage, want placePayload, where string) {
	t.Helper()

	requirePlace(t, fields, want, where)
	ppRequirePlaceShape(t, fields, where)
}

// ppDistance — distance_km поста; nil — поля нет или null (ФТ-7).
func ppDistance(t *testing.T, fields map[string]json.RawMessage, where string) *int {
	t.Helper()

	raw, ok := fields["distance_km"]
	if !ok || string(raw) == "null" {
		return nil
	}

	var km int
	if err := json.Unmarshal(raw, &km); err != nil {
		t.Fatalf("%s: distance_km не целое число: %s", where, raw)
	}

	return &km
}

// ppRequireDistance требует distance_km, равный want.
func ppRequireDistance(t *testing.T, fields map[string]json.RawMessage, want int, where string) {
	t.Helper()

	got := ppDistance(t, fields, where)
	if got == nil {
		t.Errorf("%s: ожидалось distance_km = %d, а поля нет", where, want)
		return
	}
	if *got != want {
		t.Errorf("%s: distance_km = %d, ожидалось %d", where, *got, want)
	}
}

// ppRequireNoDistance требует, чтобы distance_km не было (или был null).
func ppRequireNoDistance(t *testing.T, fields map[string]json.RawMessage, where string) {
	t.Helper()

	if got := ppDistance(t, fields, where); got != nil {
		t.Errorf("%s: distance_km быть не должно, а он %d", where, *got)
	}
}

// ppView — пост так, как его видит смотрящий в одном из мест.
type ppView struct {
	where  string
	fields map[string]json.RawMessage
}

// ppItemsOf разбирает страницу ленты или постов пользователя и находит
// в ней пост postID.
func ppItemsOf(t *testing.T, resp *http.Response, postID, where string) map[string]json.RawMessage {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: ожидался статус 200, получен %d", where, resp.StatusCode)
	}

	var page struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rawJSON(t, resp), &page); err != nil {
		t.Fatalf("%s: страница не разобралась: %v", where, err)
	}
	for _, item := range page.Items {
		var id string
		_ = json.Unmarshal(item["id"], &id)
		if id == postID {
			return item
		}
	}
	t.Fatalf("%s: поста %s на странице нет", where, postID)

	return nil
}

// ppViews — пост глазами смотрящего: по адресу, в ленте «Все» и в постах
// автора (ФТ-4: место есть во всех ответах, что отдают пост).
func ppViews(t *testing.T, baseURL, token, authorID, postID, who string) []ppView {
	t.Helper()

	return []ppView{
		{who + ": GET /posts/{id}", ppFields(t, fetchPost(t, baseURL, token, postID), http.StatusOK, who+": GET /posts/{id}")},
		{who + ": лента", ppItemsOf(t, fetchFeed(t, baseURL, token, scopeParams("all", 50, "")), postID, who+": лента")},
		{who + ": посты автора", ppItemsOf(t, fetchUserPosts(t, baseURL, token, authorID, feedParams(50, "")), postID, who+": посты автора")},
	}
}

// ppSeesPlace требует, чтобы смотрящий везде видел у поста это место
// (nil — места нет) и это расстояние (nil — расстояния нет).
func ppSeesPlace(t *testing.T, baseURL string, viewer dachnik, authorID, postID string, place *placePayload, km *int, who string) {
	t.Helper()

	for _, v := range ppViews(t, baseURL, viewer.token, authorID, postID, who) {
		if place == nil {
			requireNoPlace(t, v.fields, v.where)
		} else {
			ppPostPlace(t, v.fields, *place, v.where)
		}
		if km == nil {
			ppRequireNoDistance(t, v.fields, v.where)
		} else {
			ppRequireDistance(t, v.fields, *km, v.where)
		}
	}
}

// ppDistanceAt — distance_km поста по адресу глазами смотрящего.
func ppDistanceAt(t *testing.T, baseURL, token, postID, where string) *int {
	t.Helper()

	fields := ppFields(t, fetchPost(t, baseURL, token, postID), http.StatusOK, where+": GET /posts/{id}")

	return ppDistance(t, fields, where)
}

// ppRequireDistanceAt — distance_km поста по адресу равен want.
func ppRequireDistanceAt(t *testing.T, baseURL, token, postID string, want int, where string) {
	t.Helper()

	got := ppDistanceAt(t, baseURL, token, postID, where)
	if got == nil {
		t.Errorf("%s: ожидалось distance_km = %d, а поля нет", where, want)
		return
	}
	if *got != want {
		t.Errorf("%s: distance_km = %d, ожидалось %d", where, *got, want)
	}
}

// ppRequireNoDistanceAt — distance_km поста по адресу нет.
func ppRequireNoDistanceAt(t *testing.T, baseURL, token, postID, where string) {
	t.Helper()

	if got := ppDistanceAt(t, baseURL, token, postID, where); got != nil {
		t.Errorf("%s: distance_km быть не должно, а он %d", where, *got)
	}
}

func ppInt(v int) *int { return &v }

// ppStage — сервис с фейковым DaData, автор и смотрящий; у смотрящего
// в профиле пункт в базовой точке с известными координатами.
type ppStage struct {
	baseURL string
	fake    *fakeDaData
	author  dachnik
	viewer  dachnik
	home    placePayload
}

func newPPStage(t *testing.T) ppStage {
	t.Helper()

	baseURL, fake := startPlaces(t)
	s := ppStage{
		baseURL: baseURL,
		fake:    fake,
		author:  newPPUser(t, baseURL, 1),
		viewer:  newPPUser(t, baseURL, 2),
	}
	s.home = ppLearn(t, baseURL, fake, s.viewer.token, ppCoord(ppBaseLat), ppCoord(ppBaseLon))
	setPlaceOK(t, baseURL, s.viewer.token, s.home)

	return s
}

// ============================================================================
// Место поста
// ============================================================================

// Пост с известным пунктом: место в ответе на создание, по адресу, в ленте
// и в постах автора — у автора и у других (ФТ-1, ФТ-4).
func TestPostPlaceIsShownEverywhere(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)
	other := newPPUser(t, baseURL, 2)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, author.token, "Андрейково")

	// Публикация справочник не спрашивает: пункт уже в places.
	fake.fail(http.StatusInternalServerError)
	before := fake.calls()

	created := ppPost(t, baseURL, author.token, sntAndreykovoID)
	ppPostPlace(t, created, sntAndreykovo, "ответ на публикацию")
	postID := ppID(t, created, "ответ на публикацию")
	requireNoRequests(t, fake, before, "POST /posts с place_id")

	ppSeesPlace(t, baseURL, author, author.id, postID, &sntAndreykovo, nil, "автор")
	ppSeesPlace(t, baseURL, other, author.id, postID, &sntAndreykovo, nil, "сосед")
}

// Ответы на правки поста тоже несут место (ФТ-4): правка подписи и смена
// видимости.
func TestPostPlaceInEditResponses(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, author.token, "Андрейково")
	postID := ppPostID(t, baseURL, author.token, sntAndreykovoID)

	edited := ppFields(t, editCaptionText(t, baseURL, author.token, postID, "Новая подпись"), http.StatusOK, "правка подписи")
	ppPostPlace(t, edited, sntAndreykovo, "ответ на правку подписи")

	changed := ppFields(t, setVisibility(t, baseURL, author.token, postID, map[string]any{"visibility": visibilityFriends}),
		http.StatusOK, "смена видимости")
	ppPostPlace(t, changed, sntAndreykovo, "ответ на смену видимости")
}

// Неизвестный place_id — 400 unknown_place, пост не создаётся, и та же
// фотография потом публикуется (ФТ-1).
func TestPostPlaceUnknownIsRejected(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)

	// Улица из подсказки фильтром 025 отброшена — пунктом она не стала.
	fake.answer(
		suggestion("Московская обл, Дмитровский р-н, снт Андрейково", sntAndreykovoID, "65"),
		suggestion("Московская обл, Дмитровский р-н, д Ивановка, ул Садовая", "27279999-0000-4000-8000-000000000007", "7"),
	)
	suggest(t, baseURL, author.token, "Андрейково")

	cases := []struct {
		name    string
		placeID string
	}{
		{"UUID, которого справочник не отдавал", unknownID},
		{"отброшенная подсказка-улица", "27279999-0000-4000-8000-000000000007"},
		{"не UUID", "не-пункт"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			photo := photoOf(t, baseURL, author.token, 60, 40)
			resp := createPost(t, baseURL, author.token, map[string]any{
				"media_ids": []string{photo.ID},
				"caption":   postCaption,
				"place_id":  c.placeID,
			})
			requireCodeE(t, resp, http.StatusBadRequest, "unknown_place", "POST /posts с place_id "+c.placeID)

			page := userPostsPage(t, baseURL, author.token, author.id, feedParams(50, ""))
			if len(page.Items) != 0 {
				t.Fatalf("после отказа unknown_place у автора %d постов, ожидалось 0", len(page.Items))
			}

			// Пост не создан — фотография свободна, из неё выходит пост.
			fields := ppFields(t, createPost(t, baseURL, author.token, map[string]any{"media_ids": []string{photo.ID}}),
				http.StatusCreated, "публикация той же фотографии без места")
			requireNoPlace(t, fields, "пост без места")
			if resp := deletePost(t, baseURL, author.token, ppID(t, fields, "пост без места")); resp.StatusCode >= 300 {
				t.Fatalf("не удалось удалить пост перед следующим случаем: %d", resp.StatusCode)
			}
		})
	}
}

// Нет поля, null, пустая строка или одни пробелы — пост без места
// (ФТ-1).
func TestPostPlaceEmptyMeansNoPlace(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)
	other := newPPUser(t, baseURL, 2)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, author.token, "Андрейково")
	// И у смотрящего есть пункт — расстояния всё равно быть не должно.
	setPlaceOK(t, baseURL, other.token, sntAndreykovo)

	cases := []struct {
		name  string
		extra map[string]any
	}{
		{"нет поля", map[string]any{}},
		{"null", map[string]any{"place_id": nil}},
		{"пустая строка", map[string]any{"place_id": ""}},
		{"пробелы", map[string]any{"place_id": "   "}},
		{"табуляции и переводы строки", map[string]any{"place_id": "\t\n \n"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			created := ppFields(t, ppPublish(t, baseURL, author.token, c.extra), http.StatusCreated, "POST /posts: "+c.name)
			requireNoPlace(t, created, "ответ на публикацию: "+c.name)
			ppRequireNoDistance(t, created, "ответ на публикацию: "+c.name)

			postID := ppID(t, created, c.name)
			ppSeesPlace(t, baseURL, author, author.id, postID, nil, nil, "автор, "+c.name)
			ppSeesPlace(t, baseURL, other, author.id, postID, nil, nil, "сосед с пунктом, "+c.name)
		})
	}
}

// Сервер место из профиля сам не подставляет (ФТ-2).
func TestPostPlaceNotTakenFromProfile(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, author.token, "Андрейково")
	setPlaceOK(t, baseURL, author.token, sntAndreykovo)

	created := ppPost(t, baseURL, author.token, "")
	requireNoPlace(t, created, "пост без place_id у автора с пунктом в профиле")
	ppSeesPlace(t, baseURL, author, author.id, ppID(t, created, "пост"), nil, nil, "автор")
}

// Место поста — своё: оно может отличаться от профиля, профиль от него не
// меняется, и смена пункта в профиле не трогает опубликованный пост
// (ФТ-3, сценарий, шаг 6).
func TestPostPlaceIsIndependentOfProfile(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)
	other := newPPUser(t, baseURL, 2)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, author.token, "Андрейково")
	setPlaceOK(t, baseURL, author.token, sntAndreykovo)

	postID := ppPostID(t, baseURL, author.token, derAndreykovoID)
	requirePlace(t, meFields(t, baseURL, author.token), sntAndreykovo, "профиль автора после поста из другого пункта")

	setPlaceOK(t, baseURL, author.token, derAndreykovo)
	ppSeesPlace(t, baseURL, other, author.id, postID, &derAndreykovo, nil, "после смены пункта в профиле")

	if resp := setPlaceReq(t, baseURL, author.token, map[string]any{"place_id": nil}); resp.StatusCode != http.StatusOK {
		t.Fatalf("не удалось убрать пункт из профиля: %d", resp.StatusCode)
	}
	ppSeesPlace(t, baseURL, other, author.id, postID, &derAndreykovo, nil, "после того, как пункт из профиля убран")
}

// Переименование пункта в справочнике меняет название и у постов (ФТ-6).
func TestPostPlaceRenamedWithPlace(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)
	other := newPPUser(t, baseURL, 2)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, author.token, "Андрейково")
	postID := ppPostID(t, baseURL, author.token, sntAndreykovoID)

	renamed := placePayload{ID: sntAndreykovoID, Name: "тер СНТ Андрейково-1", Area: "г.о. Дмитровский, Московская обл"}
	fake.answer(suggestion("Московская обл, г.о. Дмитровский, тер СНТ Андрейково-1", sntAndreykovoID, "65"))
	suggest(t, baseURL, other.token, "Андрейково")

	ppSeesPlace(t, baseURL, other, author.id, postID, &renamed, nil, "после переименования")
}

// Место видно всем, кому виден пост, и только им (ФТ-5, 013).
func TestPostPlaceVisibleToWhoeverSeesPost(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)
	friend := newPPUser(t, baseURL, 2)
	stranger := newPPUser(t, baseURL, 3)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, author.token, "Андрейково")
	makeFriends(t, baseURL, author, friend)

	created := ppFields(t, ppPublish(t, baseURL, author.token, map[string]any{
		"place_id":   sntAndreykovoID,
		"visibility": visibilityFriends,
	}), http.StatusCreated, "пост для друзей")
	ppPostPlace(t, created, sntAndreykovo, "ответ на публикацию поста для друзей")
	postID := ppID(t, created, "пост для друзей")

	ppSeesPlace(t, baseURL, friend, author.id, postID, &sntAndreykovo, nil, "друг")
	requireNotFound(t, fetchPost(t, baseURL, stranger.token, postID), "GET /posts/{id}", "посторонний")

	mine := ppFields(t, ppPublish(t, baseURL, author.token, map[string]any{
		"place_id":   derAndreykovoID,
		"visibility": visibilityMe,
	}), http.StatusCreated, "пост для себя")
	mineID := ppID(t, mine, "пост для себя")
	ppPostPlace(t, ppFields(t, fetchPost(t, baseURL, author.token, mineID), http.StatusOK, "автор: пост для себя"),
		derAndreykovo, "автор: пост для себя")
	requireNotFound(t, fetchPost(t, baseURL, friend.token, mineID), "GET /posts/{id}", "друг: пост для себя")
}

// ============================================================================
// Расстояние
// ============================================================================

// Пользовательский сценарий: Галина в соседнем пункте видит ~25 км, сосед
// по СНТ — 0, Андрей без пункта — без расстояния, автор — без расстояния
// у своего поста; везде, где отдаётся пост (ФТ-7, ФТ-8, ФТ-9).
func TestPostDistanceScenario(t *testing.T) {
	baseURL, fake := startPlaces(t)
	nikolay := newPPUser(t, baseURL, 1)
	galina := newPPUser(t, baseURL, 2)
	neighbour := newPPUser(t, baseURL, 3)
	andrey := newPPUser(t, baseURL, 4)

	home := ppLearnNorth(t, baseURL, fake, nikolay.token, 0)
	rassvet := ppLearnNorth(t, baseURL, fake, galina.token, 24)

	setPlaceOK(t, baseURL, nikolay.token, home)
	setPlaceOK(t, baseURL, galina.token, rassvet)
	setPlaceOK(t, baseURL, neighbour.token, home)

	created := ppPost(t, baseURL, nikolay.token, home.ID)
	ppRequireNoDistance(t, created, "ответ на публикацию своего поста")
	postID := ppID(t, created, "пост Николая")

	ppSeesPlace(t, baseURL, galina, nikolay.id, postID, &home, ppInt(25), "Галина")
	ppSeesPlace(t, baseURL, neighbour, nikolay.id, postID, &home, ppInt(0), "сосед по СНТ")
	ppSeesPlace(t, baseURL, andrey, nikolay.id, postID, &home, nil, "Андрей без пункта")
	ppSeesPlace(t, baseURL, nikolay, nikolay.id, postID, &home, nil, "Николай, свой пост")

	// Расстояние — от текущего пункта смотрящего: Андрей выбрал пункт —
	// появилось, Галина переехала к Николаю — стало 0.
	setPlaceOK(t, baseURL, andrey.token, rassvet)
	ppRequireDistanceAt(t, baseURL, andrey.token, postID, 25, "Андрей выбрал пункт")
	setPlaceOK(t, baseURL, galina.token, home)
	ppRequireDistanceAt(t, baseURL, galina.token, postID, 0, "Галина в том же пункте")
	if resp := setPlaceReq(t, baseURL, galina.token, map[string]any{"place_id": nil}); resp.StatusCode != http.StatusOK {
		t.Fatalf("не удалось убрать пункт Галины: %d", resp.StatusCode)
	}
	ppRequireNoDistanceAt(t, baseURL, galina.token, postID, "Галина убрала пункт")
}

// У поста без места расстояния нет, даже если у смотрящего пункт есть
// (ФТ-7).
func TestPostDistanceAbsentWithoutPostPlace(t *testing.T) {
	s := newPPStage(t)

	postID := ppPostID(t, s.baseURL, s.author.token, "")
	ppSeesPlace(t, s.baseURL, s.viewer, s.author.id, postID, nil, nil, "смотрящий с пунктом")
}

// Один пункт — 0, и координаты для этого не нужны (ФТ-8).
func TestPostDistanceZeroForSamePlaceWithoutCoordinates(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)
	viewer := newPPUser(t, baseURL, 2)

	snt := ppLearn(t, baseURL, fake, author.token, ppNoField{}, ppNoField{})
	setPlaceOK(t, baseURL, viewer.token, snt)
	setPlaceOK(t, baseURL, author.token, snt)

	postID := ppPostID(t, baseURL, author.token, snt.ID)
	ppSeesPlace(t, baseURL, viewer, author.id, postID, &snt, ppInt(0), "сосед по СНТ без координат")
	ppSeesPlace(t, baseURL, author, author.id, postID, &snt, nil, "автор в том же пункте")
}

// Пункты разные и у одного из них нет координат — расстояния нет (ФТ-7).
func TestPostDistanceNeedsCoordinatesOfBoth(t *testing.T) {
	s := newPPStage(t)

	t.Run("у места поста нет координат", func(t *testing.T) {
		far := ppLearn(t, s.baseURL, s.fake, s.author.token, nil, nil)
		postID := ppPostID(t, s.baseURL, s.author.token, far.ID)
		ppSeesPlace(t, s.baseURL, s.viewer, s.author.id, postID, &far, nil, "смотрящий с координатами")
	})

	t.Run("у пункта смотрящего нет координат", func(t *testing.T) {
		near := ppLearnNorth(t, s.baseURL, s.fake, s.author.token, 30)
		postID := ppPostID(t, s.baseURL, s.author.token, near.ID)
		ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, 30, "смотрящий с координатами")

		bare := ppLearn(t, s.baseURL, s.fake, s.viewer.token, ppNoField{}, ppNoField{})
		setPlaceOK(t, s.baseURL, s.viewer.token, bare)
		ppSeesPlace(t, s.baseURL, s.viewer, s.author.id, postID, &near, nil, "смотрящий в пункте без координат")
	})
}

// Округление расстояния (ФТ-9): меньше 10 — до целого, но не меньше 1;
// от 10 до 100 — до кратного 5; от 100 — до кратного 10; половина вверх.
// Точки не ставятся ровно на границу округления: координаты строками
// дают погрешность в сотые доли километра.
func TestPostDistanceRounding(t *testing.T) {
	s := newPPStage(t)

	cases := []struct {
		km   float64
		want int
	}{
		{0, 1}, // разные пункты в одной точке — не 0
		{0.3, 1},
		{0.9, 1},
		{1.4, 1},
		{2.4, 2},
		{2.6, 3},
		{3.4, 3},
		{9.4, 9},
		{9.6, 10},
		{10.4, 10},
		{12.4, 10},
		{12.6, 15},
		{24, 25},
		{47.4, 45},
		{47.6, 50},
		{97.4, 95},
		{97.6, 100},
		{99.9, 100},
		{102, 100},
		{104.6, 100},
		{105.4, 110},
		{247, 250},
		{1234, 1230},
		{1235.6, 1240},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%v км", c.km), func(t *testing.T) {
			p := ppLearnNorth(t, s.baseURL, s.fake, s.author.token, c.km)
			postID := ppPostID(t, s.baseURL, s.author.token, p.ID)
			ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, c.want, fmt.Sprintf("%v км", c.km))
		})
	}
}

// Расстояние — по дуге большого круга, а не по меридиану: вдоль
// параллели, между далёкими городами и через 180-й меридиан (ФТ-9).
// Ожидаемые значения посчитаны по формуле гаверсинусов с R = 6371 км.
func TestPostDistanceGreatCircle(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)
	viewer := newPPUser(t, baseURL, 2)

	cases := []struct {
		name                           string
		fromLat, fromLon, toLat, toLon string
		want                           int
	}{
		// 23,40 км
		{"вдоль параллели", "56.3439", "37.5203", "56.3439", "37.9", 25},
		// 634,19 км
		{"Москва — Петербург", "55.7558", "37.6173", "59.9386", "30.3141", 630},
		// 144,32 км, а не тысячи
		{"через 180-й меридиан", "64.7337", "177.5089", "65.0", "-179.5", 140},
		// 111,19 км
		{"по экватору на градус", "0", "0", "0", "1", 110},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from := ppLearn(t, baseURL, fake, viewer.token, c.fromLat, c.fromLon)
			to := ppLearn(t, baseURL, fake, author.token, c.toLat, c.toLon)
			setPlaceOK(t, baseURL, viewer.token, from)

			postID := ppPostID(t, baseURL, author.token, to.ID)
			ppRequireDistanceAt(t, baseURL, viewer.token, postID, c.want, c.name)
		})
	}
}

// ============================================================================
// Координаты из подсказки по названию
// ============================================================================

// Координаты записываются, если пришли обе и в пределах; строки и числа
// разбираются одинаково, границы включены. Иначе пункт всё равно отдаётся
// подсказкой, но без координат (ФТ-11).
func TestPlaceCoordinatesFromSuggestion(t *testing.T) {
	s := newPPStage(t)

	lat50, lon50 := ppNorth(50)
	lat50f, _ := strconv.ParseFloat(lat50, 64)

	cases := []struct {
		name     string
		lat, lon any
		want     *int // nil — координат у пункта нет, расстояния нет
	}{
		{"строки", lat50, lon50, ppInt(50)},
		{"числа", lat50f, ppBaseLon, ppInt(50)},
		{"широта строкой, долгота числом", lat50, ppBaseLon, ppInt(50)},
		{"северный полюс", "90", "0", ppInt(5000)},
		{"южный полюс", "-90", "0", ppInt(15010)},
		{"долгота 180", "45", "180", ppInt(9350)},
		{"долгота −180", "45", "-180", ppInt(9350)},
		{"обе null", nil, nil, nil},
		{"обоих полей нет", ppNoField{}, ppNoField{}, nil},
		{"нет долготы", lat50, ppNoField{}, nil},
		{"долгота null", lat50, nil, nil},
		{"нет широты", ppNoField{}, lon50, nil},
		{"широта null", nil, lon50, nil},
		{"пустые строки", "", "", nil},
		{"широта не число", "север", lon50, nil},
		{"долгота не число", lat50, "восток", nil},
		{"широта больше 90", "90.5", lon50, nil},
		{"широта меньше −90", "-91", lon50, nil},
		{"долгота больше 180", lat50, "180.5", nil},
		{"долгота меньше −180", lat50, "-181", nil},
		{"широта числом вне пределов", 120.0, ppBaseLon, nil},
		{"долгота объектом", lat50, map[string]any{"value": lon50}, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := ppLearn(t, s.baseURL, s.fake, s.author.token, c.lat, c.lon)

			// Пункт без координат выбирается как обычно.
			setPlaceOK(t, s.baseURL, s.author.token, p)

			postID := ppPostID(t, s.baseURL, s.author.token, p.ID)
			if c.want == nil {
				ppRequireNoDistanceAt(t, s.baseURL, s.viewer.token, postID, c.name)
			} else {
				ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, *c.want, c.name)
			}
		})
	}
}

// Повторная подсказка с новыми координатами их перезаписывает; без
// координат или с негодными — оставляет прежние (ФТ-11).
func TestPlaceCoordinatesUpdatedBySuggestion(t *testing.T) {
	s := newPPStage(t)

	p := ppLearnNorth(t, s.baseURL, s.fake, s.author.token, 50)
	postID := ppPostID(t, s.baseURL, s.author.token, p.ID)
	ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, 50, "первая подсказка")

	lat200, lon200 := ppNorth(200)
	ppSuggestPlace(t, s.baseURL, s.fake, s.author.token, p, lat200, lon200)
	ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, 200, "подсказка с новыми координатами")

	keep := []struct {
		name     string
		lat, lon any
	}{
		{"обе null", nil, nil},
		{"полей нет", ppNoField{}, ppNoField{}},
		{"только широта", ppCoord(ppBaseLat), ppNoField{}},
		{"широта вне пределов", "95", ppCoord(ppBaseLon)},
		{"долгота не число", ppCoord(ppBaseLat), "восток"},
	}
	for _, c := range keep {
		ppSuggestPlace(t, s.baseURL, s.fake, s.author.token, p, c.lat, c.lon)
		ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, 200, "подсказка: "+c.name)
	}

	lat30, _ := ppNorth(30)
	lat30f, _ := strconv.ParseFloat(lat30, 64)
	ppSuggestPlace(t, s.baseURL, s.fake, s.author.token, p, lat30f, ppBaseLon)
	ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, 30, "подсказка с координатами числами")
}

// ============================================================================
// Координаты из пунктов рядом
// ============================================================================

// Пункт без координат получает их из первого своего дома в ответе пунктов
// рядом; дальше ни пункты рядом их не трогают, а подсказка по названию —
// перезаписывает (ФТ-11).
func TestPlaceCoordinatesFromNearbyOnlyWhenUnknown(t *testing.T) {
	s := newPPStage(t)

	p := ppLearn(t, s.baseURL, s.fake, s.author.token, nil, nil)
	postID := ppPostID(t, s.baseURL, s.author.token, p.ID)
	ppRequireNoDistanceAt(t, s.baseURL, s.viewer.token, postID, "пункт без координат")

	other := ppNewPlace()
	lat30, lon30 := ppNorth(30)
	lat80, lon80 := ppNorth(80)
	lat10, lon10 := ppNorth(10)
	s.fake.answer(ppHouse(other, lat10, lon10), ppHouse(p, lat30, lon30), ppHouse(p, lat80, lon80))
	requirePlaces(t, nearby(t, s.baseURL, s.author.token), []placePayload{other, p}, "пункты рядом")
	ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, 30, "координаты первого дома пункта")

	lat120, lon120 := ppNorth(120)
	s.fake.answer(ppHouse(p, lat120, lon120))
	requirePlaces(t, nearby(t, s.baseURL, s.author.token), []placePayload{p}, "пункты рядом ещё раз")
	ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, 30, "пункты рядом не перезаписывают координаты")

	lat70, lon70 := ppNorth(70)
	ppSuggestPlace(t, s.baseURL, s.fake, s.author.token, p, lat70, lon70)
	ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, 70, "подсказка по названию перезаписывает")
}

// Пункт с координатами из подсказки по названию пункты рядом не трогают
// (ФТ-11).
func TestPlaceCoordinatesFromNameKeptByNearby(t *testing.T) {
	s := newPPStage(t)

	p := ppLearnNorth(t, s.baseURL, s.fake, s.author.token, 40)
	postID := ppPostID(t, s.baseURL, s.author.token, p.ID)

	lat90, lon90 := ppNorth(90)
	s.fake.answer(ppHouse(p, lat90, lon90))
	requirePlaces(t, nearby(t, s.baseURL, s.author.token), []placePayload{p}, "пункты рядом")
	ppRequireDistanceAt(t, s.baseURL, s.viewer.token, postID, 40, "после пунктов рядом")
}

// Пункт, впервые отданный пунктами рядом, получает координаты своего
// первого дома; дом без координат или с неразборчивыми оставляет пункт
// без них, но пункт отдаётся (ФТ-11).
func TestPlaceCoordinatesFromNearbyForNewPlace(t *testing.T) {
	s := newPPStage(t)

	withCoords := ppNewPlace()
	withNull := ppNewPlace()
	withGarbage := ppNewPlace()

	lat60, lon60 := ppNorth(60)
	lat5, lon5 := ppNorth(5)
	s.fake.answer(
		ppHouse(withCoords, lat60, lon60),
		ppHouse(withNull, nil, nil),
		ppHouse(withGarbage, "где-то", lon5),
		ppHouse(withCoords, lat5, lon5),
	)
	requirePlaces(t, nearby(t, s.baseURL, s.author.token), []placePayload{withCoords, withNull, withGarbage}, "пункты рядом")

	coordsPost := ppPostID(t, s.baseURL, s.author.token, withCoords.ID)
	nullPost := ppPostID(t, s.baseURL, s.author.token, withNull.ID)
	garbagePost := ppPostID(t, s.baseURL, s.author.token, withGarbage.ID)

	ppRequireDistanceAt(t, s.baseURL, s.viewer.token, coordsPost, 60, "пункт с координатами первого дома")
	ppRequireNoDistanceAt(t, s.baseURL, s.viewer.token, nullPost, "дом без координат")
	ppRequireNoDistanceAt(t, s.baseURL, s.viewer.token, garbagePost, "дом с неразборчивыми координатами")

	// Координат у пункта нет — следующий ответ рядом их даёт.
	lat15, lon15 := ppNorth(15)
	s.fake.answer(ppHouse(withNull, lat15, lon15))
	nearby(t, s.baseURL, s.author.token)
	ppRequireDistanceAt(t, s.baseURL, s.viewer.token, nullPost, 15, "координаты появились позже")
}

// Точка человека из запроса пунктов рядом не записывается никогда:
// дома без координат оставляют пункт без координат, хотя lat и lon
// запроса известны (ФТ-11, 026 ФТ-5).
func TestPlaceCoordinatesNeverFromNearbyRequestPoint(t *testing.T) {
	s := newPPStage(t)

	p := ppNewPlace()
	s.fake.answer(ppHouse(p, nil, nil), ppHouse(p, nil, nil))

	lat, lon := ppNorth(20)
	placesOK(t, fetchNearby(t, s.baseURL, s.author.token, lat, lon), "GET /places/nearby")

	postID := ppPostID(t, s.baseURL, s.author.token, p.ID)
	ppRequireNoDistanceAt(t, s.baseURL, s.viewer.token, postID, "пункт, найденный только по точке человека")
}

// ============================================================================
// Координаты наружу не отдаются
// ============================================================================

// Ни одна ручка не отдаёт координаты пункта: ни подсказки, ни пункты рядом,
// ни профиль, ни пост — ни отдельными полями, ни внутри строк (ФТ-10).
func TestPlaceCoordinatesNeverReturned(t *testing.T) {
	baseURL, fake := startPlaces(t)
	author := newPPUser(t, baseURL, 1)
	viewer := newPPUser(t, baseURL, 2)

	// Редкие дробные части, чтобы не совпасть случайно.
	const lat, lon = "55.4817263", "37.6592814"
	needles := []string{"4817263", "6592814"}

	requireClean := func(raw []byte, where string) {
		t.Helper()
		body := string(raw)
		for _, needle := range needles {
			if strings.Contains(body, needle) {
				t.Errorf("%s: в ответе нашлась координата пункта (%s): %s", where, needle, body)
			}
		}
		for _, key := range []string{"geo_lat", "geo_lon", `"lat"`, `"lon"`, "latitude", "longitude"} {
			if strings.Contains(body, key) {
				t.Errorf("%s: в ответе поле координат %s: %s", where, key, body)
			}
		}
	}
	read := func(resp *http.Response, where string) []byte {
		t.Helper()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Fatalf("%s: ожидался статус 200 или 201, получен %d", where, resp.StatusCode)
		}
		return rawJSON(t, resp)
	}

	p := placePayload{ID: "27278888-0000-4000-8000-000000000001", Name: "снт Секретное", Area: ppArea}
	fake.answer(ppSuggestion(p.ID, p.Name, lat, lon))
	requireClean(read(fetchPlaces(t, baseURL, author.token, "Секретное"), "GET /places"), "GET /places")

	fake.answer(ppHouse(p, lat, lon))
	requireClean(read(fetchNearby(t, baseURL, author.token, nearbyLat, nearbyLon), "GET /places/nearby"), "GET /places/nearby")

	requireClean(read(setPlaceReq(t, baseURL, author.token, map[string]any{"place_id": p.ID}), "PUT /me/place"), "PUT /me/place")
	requireClean(read(getProfile(t, baseURL, author.token), "GET /me"), "GET /me")
	requireClean(read(fetchUser(t, baseURL, viewer.token, author.id), "GET /users/{id}"), "GET /users/{id}")

	setPlaceOK(t, baseURL, viewer.token, p)

	created := ppPublish(t, baseURL, author.token, map[string]any{"place_id": p.ID})
	createdRaw := read(created, "POST /posts")
	requireClean(createdRaw, "POST /posts")

	var post struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createdRaw, &post); err != nil || post.ID == "" {
		t.Fatalf("ответ на публикацию не разобрался: %s", createdRaw)
	}

	requireClean(read(fetchPost(t, baseURL, viewer.token, post.ID), "GET /posts/{id}"), "GET /posts/{id}")
	requireClean(read(fetchFeed(t, baseURL, viewer.token, scopeParams("all", 50, "")), "GET /feed"), "GET /feed")
	requireClean(read(fetchUserPosts(t, baseURL, viewer.token, author.id, feedParams(50, "")), "GET /users/{id}/posts"),
		"GET /users/{id}/posts")

	// И форма place у поста — только id, name, area.
	for _, v := range ppViews(t, baseURL, viewer.token, author.id, post.ID, "смотрящий") {
		ppPostPlace(t, v.fields, p, v.where)
		ppRequireDistance(t, v.fields, 0, v.where)
	}
}
