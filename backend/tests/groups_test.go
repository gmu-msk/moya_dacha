package tests

// Тесты групп: геогруппы и группы по интересам (specs/029-groups.md, вторая
// версия). Написаны по спецификации и контракту, не глядя в реализацию
// (ADR-0002).
//
// Геогруппа появляется, когда человек выбирает пункт в профиле, поэтому
// почти все тесты поднимают сервис с фейковым DaData (places_test.go):
// пункт сначала отдаётся подсказкой, потом выбирается через PUT /me/place.
// У каждого пункта этого файла свой id и своё название. Расстояния —
// хелперами 027 (post_place_test.go): точки отложены на север от одной.
//
// Пуши (требование 29) смотрятся через фейковый FCM из push_test.go,
// дашборд (требования 30–31) — с паролем, как в moderation_test.go.
//
// Требование 9 (переход старых профилей в геогруппы миграцией) через HTTP
// не проверяется.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// Значения из контракта.
const (
	grInterest = "interest"
	grPlace    = "place"

	grOpen = "open"

	grOwner     = "owner"
	grMember    = "member"
	grRequested = "requested"
	grInvited   = "invited"
	grNone      = "none"

	grKindInvite = "invite"
)

// Идентификаторы, которых в базе нет.
const (
	grNoGroup = "00000000-0000-4000-8000-000000000929"
	grNoUser  = "00000000-0000-4000-8000-000000000029"
)

// --- Представления из контракта -------------------------------------------

// grGroup — schema Group. Необязательные поля — указатели: их отсутствие
// проверяется отдельно.
type grGroup struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Kind        string         `json:"kind"`
	JoinPolicy  string         `json:"join_policy"`
	Owner       *authorPayload `json:"owner"`
	Members     int            `json:"members"`
	Membership  string         `json:"membership"`
	CreatedAt   string         `json:"created_at"`
	Place       *placePayload  `json:"place"`
	DistanceKm  *int           `json:"distance_km"`
	Near        bool           `json:"near"`
}

// grMemberRow — schema GroupMember.
type grMemberRow struct {
	User      authorPayload `json:"user"`
	Role      string        `json:"role"`
	State     string        `json:"state"`
	CreatedAt string        `json:"created_at"`
}

// grRequestItem — schema GroupRequest.
type grRequestItem struct {
	Kind  string `json:"kind"`
	Group struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"group"`
	User      authorPayload `json:"user"`
	CreatedAt string        `json:"created_at"`
}

// grUnread — schema UnreadNotifications с полем этой фичи.
type grUnread struct {
	Unread   *int `json:"unread"`
	Requests *int `json:"requests"`
	Groups   *int `json:"groups"`
}

// grDashGroup — геогруппа в moderation.groups дашборда (требование 30).
type grDashGroup struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Area    string `json:"area"`
	Members []struct {
		ID       string `json:"id"`
		Nickname string `json:"nickname"`
		Name     string `json:"name"`
	} `json:"members"`
}

// --- Хелперы: люди ---------------------------------------------------------

// grPhone — номер n-го участника теста; не пересекается с другими файлами.
func grPhone(n int) string {
	return fmt.Sprintf("+7 (900) 729-00-%02d", n)
}

// newGrUser регистрирует n-го участника теста.
func newGrUser(t *testing.T, baseURL string, n int) dachnik {
	t.Helper()

	token, id := signIn(t, baseURL, grPhone(n))

	return dachnik{token: token, id: id}
}

// newGrNamed регистрирует участника и даёт ему никнейм.
func newGrNamed(t *testing.T, baseURL string, n int, nick string) dachnik {
	t.Helper()

	d := newGrUser(t, baseURL, n)
	chooseNickname(t, baseURL, d.token, nick)

	return d
}

// newGrPerson — участник с никнеймом и именем (для дашборда).
func newGrPerson(t *testing.T, baseURL string, n int, nick, name string) dachnik {
	t.Helper()

	d := newGrNamed(t, baseURL, n, nick)
	if resp := do(t, http.MethodPut, baseURL+"/me", d.token, map[string]any{"name": name}); resp.StatusCode != http.StatusOK {
		t.Fatalf("имя %q не сохранилось: статус %d", name, resp.StatusCode)
	}

	return d
}

// --- Хелперы: пункты и геогруппы -------------------------------------------

// grPlaceNo — счётчик пунктов этого файла: у каждого свой id.
var grPlaceNo int

// grLearn отдаёт новый пункт с этим названием подсказкой (lat, lon — как
// в ppSuggestion, ppNoField{} — координаты нет) и требует, чтобы сервис
// вернул его как пункт. Название — без запятых: оно последняя часть value.
func grLearn(t *testing.T, baseURL string, fake *fakeDaData, token, name string, lat, lon any) placePayload {
	t.Helper()

	grPlaceNo++
	p := placePayload{
		ID:   fmt.Sprintf("27290000-0000-4000-8000-%012d", grPlaceNo),
		Name: name,
		Area: ppArea,
	}

	fake.answer(ppSuggestion(p.ID, p.Name, lat, lon))
	requirePlaces(t, suggest(t, baseURL, token, "снт"), []placePayload{p}, "подсказка "+name)

	return p
}

// grLearnPlain — пункт без координат.
func grLearnPlain(t *testing.T, baseURL string, fake *fakeDaData, token, name string) placePayload {
	t.Helper()
	return grLearn(t, baseURL, fake, token, name, ppNoField{}, ppNoField{})
}

// grLearnNorth — пункт в km километрах к северу от базовой точки 027.
func grLearnNorth(t *testing.T, baseURL string, fake *fakeDaData, token, name string, km float64) placePayload {
	t.Helper()

	lat, lon := ppNorth(km)

	return grLearn(t, baseURL, fake, token, name, lat, lon)
}

// grSettle — человек выбирает пункт в профиле.
func grSettle(t *testing.T, baseURL string, who dachnik, p placePayload) {
	t.Helper()
	setPlaceOK(t, baseURL, who.token, p)
}

// grUnsettle — человек снимает пункт (place_id: null).
func grUnsettle(t *testing.T, baseURL string, who dachnik) {
	t.Helper()

	fields := fieldsOf(t, setPlaceReq(t, baseURL, who.token, map[string]any{"place_id": nil}), "PUT /me/place null")
	requireNoPlace(t, fields, "ответ PUT /me/place null")
}

// grFindPlace — геогруппа пункта в списке или nil.
func grFindPlace(items []grGroup, placeID string) *grGroup {
	for i := range items {
		if items[i].Kind == grPlace && items[i].Place != nil && items[i].Place.ID == placeID {
			return &items[i]
		}
	}
	return nil
}

// grPlaceGroups — все геогруппы пункта, видимые смотрящему в обеих вкладках.
func grPlaceGroups(t *testing.T, baseURL string, viewer dachnik, p placePayload) []grGroup {
	t.Helper()

	var out []grGroup
	for _, scope := range []string{"mine", "available"} {
		for _, g := range grList(t, baseURL, viewer, grParams(scope, grPlace, p.Name)) {
			if g.Place != nil && g.Place.ID == p.ID {
				out = append(out, g)
			}
		}
	}

	return out
}

// grGeoOf — геогруппа пункта глазами смотрящего; ровно одна (требование 2).
func grGeoOf(t *testing.T, baseURL string, viewer dachnik, p placePayload) grGroup {
	t.Helper()

	found := grPlaceGroups(t, baseURL, viewer, p)
	if len(found) != 1 {
		t.Fatalf("геогрупп пункта %q в «Моих» и «Найти» %d, ожидалась ровно одна: %q", p.Name, len(found), grNames(found))
	}

	return found[0]
}

// grRequireGeoShape — геогруппа пункта p по требованиям 2 и 12.
func grRequireGeoShape(t *testing.T, g grGroup, p placePayload, where string) {
	t.Helper()

	if g.Kind != grPlace {
		t.Errorf("%s: kind = %q, ожидалось place", where, g.Kind)
	}
	if g.Name != p.Name {
		t.Errorf("%s: name = %q, ожидалось название пункта %q", where, g.Name, p.Name)
	}
	if g.Description != p.Area {
		t.Errorf("%s: description = %q, ожидалось area пункта %q", where, g.Description, p.Area)
	}
	if g.JoinPolicy != grOpen {
		t.Errorf("%s: join_policy = %q, ожидалось open", where, g.JoinPolicy)
	}
	if g.Owner != nil {
		t.Errorf("%s: у геогруппы есть хозяин %+v", where, *g.Owner)
	}
	if g.Place == nil || *g.Place != p {
		t.Errorf("%s: place = %+v, ожидался %+v", where, g.Place, p)
	}
}

// startGrPlaces — сервис с фейковым DaData (как startPlaces).
func startGrPlaces(t *testing.T) (string, *fakeDaData) {
	t.Helper()
	return startPlaces(t)
}

// grPlacesConfig — настройки сервиса с этим фейковым DaData.
func grPlacesConfig(fake *fakeDaData) api.Config {
	return api.Config{PlacesURL: fake.URL(), PlacesKey: placesKey}
}

// --- Хелперы: разбор ответов ----------------------------------------------

// grRequireFields требует, чтобы в объекте были эти поля.
func grRequireFields(t *testing.T, fields map[string]json.RawMessage, where string, names ...string) {
	t.Helper()

	for _, name := range names {
		if _, ok := fields[name]; !ok {
			t.Errorf("%s: в ответе нет обязательного поля %s", where, name)
		}
	}
}

// grAbsent — поля нет или оно null.
func grAbsent(fields map[string]json.RawMessage, name string) bool {
	raw, ok := fields[name]
	return !ok || string(raw) == "null"
}

// grParseGroup разбирает один объект Group и проверяет обязательные поля:
// у группы по интересам — хозяин, у геогруппы — место и нет хозяина.
func grParseGroup(t *testing.T, raw json.RawMessage, where string) grGroup {
	t.Helper()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("%s: группа не разобралась как JSON-объект: %v (%s)", where, err, raw)
	}
	grRequireFields(t, fields, where,
		"id", "name", "description", "kind", "join_policy",
		"members", "membership", "near", "created_at")

	var g grGroup
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatalf("%s: группа не разобралась как Group: %v (%s)", where, err, raw)
	}
	if g.ID == "" {
		t.Fatalf("%s: у группы пустой id: %s", where, raw)
	}
	if _, err := time.Parse(time.RFC3339, g.CreatedAt); err != nil {
		t.Errorf("%s: created_at %q — не дата-время: %v", where, g.CreatedAt, err)
	}
	if g.JoinPolicy != grOpen {
		t.Errorf("%s: join_policy = %q, ожидалось open всегда (требование 12)", where, g.JoinPolicy)
	}

	switch g.Kind {
	case grInterest:
		if grAbsent(fields, "owner") {
			t.Errorf("%s: у группы по интересам нет owner", where)
		}
		if !grAbsent(fields, "place") {
			t.Errorf("%s: у группы по интересам есть place: %s", where, fields["place"])
		}
		if !grAbsent(fields, "distance_km") {
			t.Errorf("%s: у группы по интересам есть distance_km: %s", where, fields["distance_km"])
		}
		if g.Near {
			t.Errorf("%s: near = true у группы по интересам", where)
		}
	case grPlace:
		if !grAbsent(fields, "owner") {
			t.Errorf("%s: у геогруппы есть owner: %s", where, fields["owner"])
		}
		if grAbsent(fields, "place") {
			t.Errorf("%s: у геогруппы нет place", where)
		}
		if g.Membership == grOwner {
			t.Errorf("%s: membership owner у геогруппы", where)
		}
	default:
		t.Errorf("%s: неизвестный kind %q", where, g.Kind)
	}

	return g
}

// grStatus требует статус и, если он не тот, показывает код ошибки.
func grStatus(t *testing.T, resp *http.Response, status int, where string) {
	t.Helper()

	if resp.StatusCode != status {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус %d, получен %d %s", where, status, resp.StatusCode, code)
	}
}

// grGroupOK требует статус и возвращает группу из ответа.
func grGroupOK(t *testing.T, resp *http.Response, status int, where string) grGroup {
	t.Helper()

	grStatus(t, resp, status, where)

	return grParseGroup(t, rawJSON(t, resp), where)
}

// grItems требует 200 и возвращает элементы {"items": [...]} как есть.
func grItems(t *testing.T, resp *http.Response, where string) []json.RawMessage {
	t.Helper()

	grStatus(t, resp, http.StatusOK, where)

	var body struct {
		Items *[]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rawJSON(t, resp), &body); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON: %v", where, err)
	}
	if body.Items == nil {
		t.Fatalf("%s: в ответе нет items (пустой список — [], а не отсутствие поля)", where)
	}

	return *body.Items
}

// grRawFields — тело ответа 200 как словарь полей.
func grRawFields(t *testing.T, resp *http.Response, where string) map[string]json.RawMessage {
	t.Helper()

	grStatus(t, resp, http.StatusOK, where)

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON(t, resp), &fields); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON-объект: %v", where, err)
	}

	return fields
}

// grRequireCode — ошибка с этим статусом и кодом, без остановки теста.
func grRequireCode(t *testing.T, resp *http.Response, status int, code, where string) {
	t.Helper()
	requireCodeE(t, resp, status, code, where)
}

// grRequireList требует ровно эти строки в этом порядке.
func grRequireList(t *testing.T, got, want []string, where string) {
	t.Helper()

	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: получено %q, ожидалось %q", where, got, want)
	}
}

// grOnly оставляет из списка только строки из known, сохраняя порядок:
// так тест не зависит от чужих групп в базе.
func grOnly(got []string, known ...string) []string {
	keep := map[string]bool{}
	for _, k := range known {
		keep[k] = true
	}

	out := []string{}
	for _, g := range got {
		if keep[g] {
			out = append(out, g)
		}
	}
	return out
}

// --- Хелперы: группы -------------------------------------------------------

func grGroupsURL(baseURL string) string {
	return baseURL + "/groups"
}

func grGroupURL(baseURL, groupID string) string {
	return baseURL + "/groups/" + url.PathEscape(groupID)
}

// grDraft — тело создания группы по интересам по новой версии.
func grDraft(name, description string) map[string]any {
	return map[string]any{"name": name, "description": description}
}

// grCreateReq создаёт группу; body — как есть.
func grCreateReq(t *testing.T, baseURL, token string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, grGroupsURL(baseURL), token, body)
}

// grCreate создаёт группу и требует 201.
func grCreate(t *testing.T, baseURL string, who dachnik, body map[string]any) grGroup {
	t.Helper()
	return grGroupOK(t, grCreateReq(t, baseURL, who.token, body), http.StatusCreated,
		fmt.Sprintf("создание группы %v", body["name"]))
}

// grInterestGroup создаёт группу по интересам без описания.
func grInterestGroup(t *testing.T, baseURL string, who dachnik, name string) grGroup {
	t.Helper()
	return grCreate(t, baseURL, who, map[string]any{"name": name})
}

func grGetReq(t *testing.T, baseURL, token, groupID string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, grGroupURL(baseURL, groupID), token, nil)
}

// grGet — группа глазами смотрящего, 200.
func grGet(t *testing.T, baseURL string, who dachnik, groupID string) grGroup {
	t.Helper()
	return grGroupOK(t, grGetReq(t, baseURL, who.token, groupID), http.StatusOK, "GET /groups/{id}")
}

func grDeleteReq(t *testing.T, baseURL, token, groupID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, grGroupURL(baseURL, groupID), token, nil)
}

// grParams — параметры списка; пустые не попадают в запрос.
func grParams(scope, kind, q string) url.Values {
	params := url.Values{}
	if scope != "" {
		params.Set("scope", scope)
	}
	if kind != "" {
		params.Set("kind", kind)
	}
	if q != "" {
		params.Set("q", q)
	}
	return params
}

func grListReq(t *testing.T, baseURL, token string, params url.Values) *http.Response {
	t.Helper()

	address := grGroupsURL(baseURL)
	if len(params) > 0 {
		address += "?" + params.Encode()
	}

	return do(t, http.MethodGet, address, token, nil)
}

// grList — список групп, 200.
func grList(t *testing.T, baseURL string, who dachnik, params url.Values) []grGroup {
	t.Helper()

	where := "GET /groups?" + params.Encode()
	raw := grItems(t, grListReq(t, baseURL, who.token, params), where)
	out := make([]grGroup, 0, len(raw))
	for _, item := range raw {
		out = append(out, grParseGroup(t, item, where))
	}

	return out
}

// grMine и grAvailable — две вкладки.
func grMine(t *testing.T, baseURL string, who dachnik) []grGroup {
	t.Helper()
	return grList(t, baseURL, who, grParams("mine", "", ""))
}

func grAvailable(t *testing.T, baseURL string, who dachnik) []grGroup {
	t.Helper()
	return grList(t, baseURL, who, grParams("available", "", ""))
}

func grNames(items []grGroup) []string {
	out := make([]string, 0, len(items))
	for _, g := range items {
		out = append(out, g.Name)
	}
	return out
}

func grIDs(items []grGroup) []string {
	out := make([]string, 0, len(items))
	for _, g := range items {
		out = append(out, g.ID)
	}
	return out
}

// grFind — группа с этим id в списке или nil.
func grFind(items []grGroup, id string) *grGroup {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

// grRequireIn требует группу в «Моих» (mine == true) или в «Найти» с этим
// membership и её отсутствие на другой вкладке.
func grRequireIn(t *testing.T, baseURL string, who dachnik, groupID string, mine bool, membership, where string) {
	t.Helper()

	inMine := grFind(grMine(t, baseURL, who), groupID)
	inAvailable := grFind(grAvailable(t, baseURL, who), groupID)

	here, there, tab, other := inMine, inAvailable, "«Моих»", "«Найти»"
	if !mine {
		here, there, tab, other = inAvailable, inMine, "«Найти»", "«Моих»"
	}
	if here == nil {
		t.Errorf("%s: группы нет в %s", where, tab)
	} else if here.Membership != membership {
		t.Errorf("%s: в %s membership = %q, ожидалось %q", where, tab, here.Membership, membership)
	}
	if there != nil {
		t.Errorf("%s: группа есть и в %s", where, other)
	}
}

func grMembershipURL(baseURL, groupID string) string {
	return grGroupURL(baseURL, groupID) + "/membership"
}

func grJoinReq(t *testing.T, baseURL, token, groupID string) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, grMembershipURL(baseURL, groupID), token, nil)
}

// grJoin вступает и требует 200 с этим membership.
func grJoin(t *testing.T, baseURL string, who dachnik, groupID, want string) grGroup {
	t.Helper()

	g := grGroupOK(t, grJoinReq(t, baseURL, who.token, groupID), http.StatusOK, "вступление")
	if g.Membership != want {
		t.Fatalf("вступление: membership = %q, ожидалось %q", g.Membership, want)
	}

	return g
}

func grLeaveReq(t *testing.T, baseURL, token, groupID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, grMembershipURL(baseURL, groupID), token, nil)
}

// grLeave выходит (или отклоняет приглашение) и требует 204.
func grLeave(t *testing.T, baseURL string, who dachnik, groupID string) {
	t.Helper()
	requireEmpty204(t, grLeaveReq(t, baseURL, who.token, groupID), "DELETE /groups/{id}/membership")
}

func grMembersReq(t *testing.T, baseURL, token, groupID, state string) *http.Response {
	t.Helper()

	address := grGroupURL(baseURL, groupID) + "/members"
	if state != "" {
		address += "?" + url.Values{"state": {state}}.Encode()
	}

	return do(t, http.MethodGet, address, token, nil)
}

// grParseMember разбирает один GroupMember.
func grParseMember(t *testing.T, raw json.RawMessage, where string) grMemberRow {
	t.Helper()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("%s: строка состава не разобралась: %v (%s)", where, err, raw)
	}
	grRequireFields(t, fields, where, "user", "role", "state", "created_at")

	var m grMemberRow
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s: строка состава не разобралась как GroupMember: %v (%s)", where, err, raw)
	}
	if _, err := time.Parse(time.RFC3339, m.CreatedAt); err != nil {
		t.Errorf("%s: created_at %q — не дата-время: %v", where, m.CreatedAt, err)
	}

	return m
}

// grMembers — состав (state == "") или ждущие строки, 200.
func grMembers(t *testing.T, baseURL string, who dachnik, groupID, state string) []grMemberRow {
	t.Helper()

	where := "GET /groups/{id}/members?state=" + state
	raw := grItems(t, grMembersReq(t, baseURL, who.token, groupID, state), where)
	out := make([]grMemberRow, 0, len(raw))
	for _, item := range raw {
		out = append(out, grParseMember(t, item, where))
	}

	return out
}

func grMemberIDs(items []grMemberRow) []string {
	out := make([]string, 0, len(items))
	for _, m := range items {
		out = append(out, m.User.ID)
	}
	return out
}

func grMemberURL(baseURL, groupID, userID string) string {
	return grGroupURL(baseURL, groupID) + "/members/" + url.PathEscape(userID)
}

func grInviteReq(t *testing.T, baseURL, token, groupID, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, grMemberURL(baseURL, groupID, userID), token, nil)
}

// grInviteOK — участник приглашает; 200 и GroupMember с этим state.
func grInviteOK(t *testing.T, baseURL string, who dachnik, groupID, userID, wantState string) grMemberRow {
	t.Helper()

	where := "PUT /groups/{id}/members/{userId}"
	resp := grInviteReq(t, baseURL, who.token, groupID, userID)
	grStatus(t, resp, http.StatusOK, where)

	m := grParseMember(t, rawJSON(t, resp), where)
	if m.User.ID != userID {
		t.Errorf("%s: в ответе человек %s, ожидался %s", where, m.User.ID, userID)
	}
	if m.State != wantState {
		t.Fatalf("%s: state = %q, ожидалось %q", where, m.State, wantState)
	}

	return m
}

// grInvite — новое приглашение, строка из ответа не нужна.
func grInvite(t *testing.T, baseURL string, who dachnik, groupID, userID string) {
	t.Helper()
	grInviteOK(t, baseURL, who, groupID, userID, grInvited)
}

func grRemoveReq(t *testing.T, baseURL, token, groupID, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, grMemberURL(baseURL, groupID, userID), token, nil)
}

// grRemove — хозяин отзывает приглашение или убирает участника; 204.
func grRemove(t *testing.T, baseURL string, owner dachnik, groupID, userID string) {
	t.Helper()
	requireEmpty204(t, grRemoveReq(t, baseURL, owner.token, groupID, userID), "DELETE /groups/{id}/members/{userId}")
}

func grRequestsReq(t *testing.T, baseURL, token string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, baseURL+"/me/group-requests", token, nil)
}

// grRequests — «Группы» в «Уведомлениях», 200.
func grRequests(t *testing.T, baseURL string, who dachnik) []grRequestItem {
	t.Helper()

	where := "GET /me/group-requests"
	raw := grItems(t, grRequestsReq(t, baseURL, who.token), where)
	out := make([]grRequestItem, 0, len(raw))
	for _, item := range raw {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(item, &fields); err != nil {
			t.Fatalf("%s: строка не разобралась: %v (%s)", where, err, item)
		}
		grRequireFields(t, fields, where, "kind", "group", "user", "created_at")

		var r grRequestItem
		if err := json.Unmarshal(item, &r); err != nil {
			t.Fatalf("%s: строка не разобралась как GroupRequest: %v (%s)", where, err, item)
		}
		if _, err := time.Parse(time.RFC3339, r.CreatedAt); err != nil {
			t.Errorf("%s: created_at %q — не дата-время", where, r.CreatedAt)
		}
		out = append(out, r)
	}

	return out
}

// grRequestKeys — «вид:группа:кто пригласил» по строкам списка.
func grRequestKeys(items []grRequestItem) []string {
	out := make([]string, 0, len(items))
	for _, r := range items {
		out = append(out, r.Kind+":"+r.Group.ID+":"+r.User.ID)
	}
	return out
}

// grInviteKey — ключ строки приглашения в группу от этого человека.
func grInviteKey(groupID, inviterID string) string {
	return grKindInvite + ":" + groupID + ":" + inviterID
}

// grRequireGroupsUnread требует groups в счётчике непрочитанного.
func grRequireGroupsUnread(t *testing.T, baseURL string, who dachnik, want int, where string) {
	t.Helper()

	resp := fetchUnread(t, baseURL, who.token)
	grStatus(t, resp, http.StatusOK, where+": GET /me/notifications/unread")

	var body grUnread
	decode(t, resp, &body)

	if body.Unread == nil || body.Requests == nil {
		t.Errorf("%s: в счётчике нет unread или requests: %+v", where, body)
	}
	if body.Groups == nil {
		t.Fatalf("%s: в счётчике нет поля groups", where)
	}
	if *body.Groups != want {
		t.Errorf("%s: groups = %d, ожидалось %d", where, *body.Groups, want)
	}
}

// grRequireHidden требует, чтобы группы для смотрящего не было: 404
// group_not_found на каждой ручке группы и нет её в списках (требование 16).
// otherID — кто-то другой, кем пробуются ручки состава.
func grRequireHidden(t *testing.T, baseURL string, viewer dachnik, groupID, otherID, where string) {
	t.Helper()

	nf := func(resp *http.Response, what string) {
		t.Helper()
		grRequireCode(t, resp, http.StatusNotFound, "group_not_found", where+": "+what)
	}

	nf(grGetReq(t, baseURL, viewer.token, groupID), "GET /groups/{id}")
	nf(grJoinReq(t, baseURL, viewer.token, groupID), "PUT membership")
	nf(grLeaveReq(t, baseURL, viewer.token, groupID), "DELETE membership")
	nf(grMembersReq(t, baseURL, viewer.token, groupID, ""), "GET members")
	nf(grMembersReq(t, baseURL, viewer.token, groupID, grInvited), "GET members?state=invited")
	nf(grInviteReq(t, baseURL, viewer.token, groupID, otherID), "PUT members/{userId}")
	nf(grRemoveReq(t, baseURL, viewer.token, groupID, otherID), "DELETE members/{userId}")
	nf(grDeleteReq(t, baseURL, viewer.token, groupID), "DELETE /groups/{id}")

	if grFind(grMine(t, baseURL, viewer), groupID) != nil {
		t.Errorf("%s: группа есть в «Моих»", where)
	}
	if grFind(grAvailable(t, baseURL, viewer), groupID) != nil {
		t.Errorf("%s: группа есть в «Найти»", where)
	}
}

// grRequireMembers требует число участников глазами этого смотрящего.
func grRequireMembers(t *testing.T, baseURL string, who dachnik, groupID string, want int, where string) {
	t.Helper()

	if g := grGet(t, baseURL, who, groupID); g.Members != want {
		t.Errorf("%s: members = %d, ожидалось %d", where, g.Members, want)
	}
}

// grRequireMembership требует отношение смотрящего к группе.
func grRequireMembership(t *testing.T, baseURL string, who dachnik, groupID, want, where string) {
	t.Helper()

	if g := grGet(t, baseURL, who, groupID); g.Membership != want {
		t.Errorf("%s: membership = %q, ожидалось %q", where, g.Membership, want)
	}
}

// --- Хелперы: дашборд ------------------------------------------------------

// grDashGroups — moderation.groups из /dashboard/data (требование 30).
func grDashGroups(t *testing.T, root string) []grDashGroup {
	t.Helper()

	moderation := modRawModeration(t, root)
	raw, ok := moderation["groups"]
	if !ok {
		t.Fatal("в moderation нет поля groups (требование 30)")
	}

	var groups []grDashGroup
	if err := json.Unmarshal(raw, &groups); err != nil {
		t.Fatalf("moderation.groups не разобрался как список геогрупп: %v (%s)", err, raw)
	}

	return groups
}

// grDashFind — геогруппа дашборда с этим id или nil.
func grDashFind(groups []grDashGroup, id string) *grDashGroup {
	for i := range groups {
		if groups[i].ID == id {
			return &groups[i]
		}
	}
	return nil
}

// grDashMemberIDs — id участников геогруппы дашборда, отсортированные.
func grDashMemberIDs(g grDashGroup) []string {
	out := make([]string, 0, len(g.Members))
	for _, m := range g.Members {
		out = append(out, m.ID)
	}
	return sortedCopy(out)
}

// grDashRemovePath — путь «Убрать» из геогруппы в дашборде.
func grDashRemovePath(groupID, userID string) string {
	return "/dashboard/groups/" + url.PathEscape(groupID) + "/members/" + url.PathEscape(userID)
}

// ============================================================================
// Доступ
// ============================================================================

// Без сессии ручки групп отвечают 401.
func TestGroupsRequireAuth(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	g := grInterestGroup(t, baseURL, owner, "Розы и клематисы")

	for _, token := range []string{"", "не-токен-сессии"} {
		requireUnauthorized(t, grListReq(t, baseURL, token, nil), "GET /groups")
		requireUnauthorized(t, grCreateReq(t, baseURL, token, grDraft("Группа", "")), "POST /groups")
		requireUnauthorized(t, grGetReq(t, baseURL, token, g.ID), "GET /groups/{id}")
		requireUnauthorized(t, grDeleteReq(t, baseURL, token, g.ID), "DELETE /groups/{id}")
		requireUnauthorized(t, grJoinReq(t, baseURL, token, g.ID), "PUT membership")
		requireUnauthorized(t, grLeaveReq(t, baseURL, token, g.ID), "DELETE membership")
		requireUnauthorized(t, grMembersReq(t, baseURL, token, g.ID, ""), "GET members")
		requireUnauthorized(t, grInviteReq(t, baseURL, token, g.ID, owner.id), "PUT members/{userId}")
		requireUnauthorized(t, grRemoveReq(t, baseURL, token, g.ID, owner.id), "DELETE members/{userId}")
		requireUnauthorized(t, grRequestsReq(t, baseURL, token), "GET /me/group-requests")
	}
}

// ============================================================================
// Геогруппа создаётся сама (требования 2, 5–8, 12)
// ============================================================================

// Выбор пункта создаёт геогруппу: название — пункт из справочника,
// описание — area, хозяина нет, человек — участник. Второй с тем же
// пунктом попадает в ту же группу: на пункт одна геогруппа.
func TestGeoGroupCreatedOnPlaceChoice(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	galina := newGrNamed(t, baseURL, 1, "galina_dacha")
	nikolay := newGrNamed(t, baseURL, 2, "nikolay_dacha")
	stranger := newGrUser(t, baseURL, 3)

	romashka := grLearnNorth(t, baseURL, fake, galina.token, "снт Ромашка-гр", 0)

	if found := grPlaceGroups(t, baseURL, stranger, romashka); len(found) != 0 {
		t.Fatalf("геогруппа появилась до выбора пункта: %q", grNames(found))
	}

	grSettle(t, baseURL, galina, romashka)

	mine := grList(t, baseURL, galina, grParams("mine", grPlace, ""))
	geo := grFindPlace(mine, romashka.ID)
	if geo == nil {
		t.Fatalf("после выбора пункта геогруппы нет в «Моих»: %q", grNames(mine))
	}
	grRequireGeoShape(t, *geo, romashka, "«Мои» Галины")
	if geo.Membership != grMember {
		t.Errorf("membership = %q, ожидалось member", geo.Membership)
	}
	if geo.Members != 1 {
		t.Errorf("members = %d, ожидался 1", geo.Members)
	}
	if !geo.Near {
		t.Error("near = false, а это пункт смотрящего")
	}
	if geo.DistanceKm == nil || *geo.DistanceKm != 0 {
		t.Errorf("distance_km = %v, ожидался 0 — тот же пункт", geo.DistanceKm)
	}

	byID := grGet(t, baseURL, galina, geo.ID)
	grRequireGeoShape(t, byID, romashka, "GET геогруппы")
	if byID.CreatedAt != geo.CreatedAt || byID.Membership != grMember {
		t.Errorf("GET отдаёт другое: %+v, в списке %+v", byID, *geo)
	}

	// Посторонний видит её в «Найти».
	if g := grGeoOf(t, baseURL, stranger, romashka); g.ID != geo.ID || g.Membership != grNone {
		t.Errorf("посторонний: группа %s membership %q, ожидалась %s none", g.ID, g.Membership, geo.ID)
	}

	// Николай выбирает тот же пункт — та же группа, теперь их двое.
	grSettle(t, baseURL, nikolay, romashka)
	same := grGeoOf(t, baseURL, nikolay, romashka)
	if same.ID != geo.ID {
		t.Fatalf("второй выбравший попал в другую группу %s, ожидалась %s", same.ID, geo.ID)
	}
	if same.Membership != grMember || same.Members != 2 {
		t.Errorf("Николай: membership = %q, members = %d; ожидалось member, 2", same.Membership, same.Members)
	}
	grRequireMembers(t, baseURL, galina, geo.ID, 2, "глазами Галины")
	if g := grGeoOf(t, baseURL, stranger, romashka); g.ID != geo.ID {
		t.Errorf("у пункта появилась вторая группа %s", g.ID)
	}

	// Состав: оба участники, хозяина нет, новые выше.
	items := grMembers(t, baseURL, stranger, geo.ID, "")
	grRequireList(t, grMemberIDs(items), []string{nikolay.id, galina.id}, "состав геогруппы")
	for _, m := range items {
		if m.Role != grMember || m.State != grMember {
			t.Errorf("строка состава геогруппы: role = %q, state = %q", m.Role, m.State)
		}
	}
}

// Смена пункта не выводит из прежней геогруппы; снятый пункт тоже
// (требование 6).
func TestGeoGroupKeptOnPlaceChange(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	nikolay := newGrUser(t, baseURL, 1)

	romashka := grLearnPlain(t, baseURL, fake, nikolay.token, "снт Ромашка-смена")
	rassvet := grLearnPlain(t, baseURL, fake, nikolay.token, "снт Рассвет-смена")

	grSettle(t, baseURL, nikolay, romashka)
	old := grGeoOf(t, baseURL, nikolay, romashka)

	grSettle(t, baseURL, nikolay, rassvet)
	fresh := grGeoOf(t, baseURL, nikolay, rassvet)
	if fresh.ID == old.ID {
		t.Fatal("у двух пунктов одна геогруппа")
	}
	grRequireGeoShape(t, fresh, rassvet, "новая геогруппа")

	grRequireIn(t, baseURL, nikolay, old.ID, true, grMember, "прежняя геогруппа после смены пункта")
	grRequireIn(t, baseURL, nikolay, fresh.ID, true, grMember, "новая геогруппа")
	grRequireMembers(t, baseURL, nikolay, old.ID, 1, "прежняя геогруппа")

	grUnsettle(t, baseURL, nikolay)
	grRequireIn(t, baseURL, nikolay, old.ID, true, grMember, "прежняя после снятия пункта")
	grRequireIn(t, baseURL, nikolay, fresh.ID, true, grMember, "последняя после снятия пункта")
	grRequireMembers(t, baseURL, nikolay, fresh.ID, 1, "последняя после снятия пункта")
}

// Выбор того же пункта ничего не меняет: вышедший не возвращается
// (требование 7). Выбор пункта заново после другого — это выбор пункта
// (требование 5), и человек снова участник.
func TestGeoGroupSamePlaceDoesNotRejoin(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	me := newGrUser(t, baseURL, 1)

	romashka := grLearnPlain(t, baseURL, fake, me.token, "снт Ромашка-повтор")
	other := grLearnPlain(t, baseURL, fake, me.token, "снт Другое-повтор")

	grSettle(t, baseURL, me, romashka)
	geo := grGeoOf(t, baseURL, me, romashka)
	grLeave(t, baseURL, me, geo.ID)
	grRequireIn(t, baseURL, me, geo.ID, false, grNone, "после выхода")

	grSettle(t, baseURL, me, romashka)
	grRequireIn(t, baseURL, me, geo.ID, false, grNone, "после повторного выбора того же пункта")
	grRequireMembers(t, baseURL, me, geo.ID, 0, "после повторного выбора того же пункта")

	grSettle(t, baseURL, me, other)
	grSettle(t, baseURL, me, romashka)
	grRequireIn(t, baseURL, me, geo.ID, true, grMember, "после выбора пункта заново")
	grRequireMembers(t, baseURL, me, geo.ID, 1, "после выбора пункта заново")
}

// Геогруппа без участников остаётся: её видно в «Найти», в неё попадает
// следующий выбравший пункт (требование 8); удаление аккаунта её не
// удаляет («Ограничения»).
func TestGeoGroupWithoutMembersStays(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	first := newGrUser(t, baseURL, 1)
	second := newGrUser(t, baseURL, 2)
	viewer := newGrUser(t, baseURL, 3)

	romashka := grLearnPlain(t, baseURL, fake, first.token, "снт Пустое-гр")
	grSettle(t, baseURL, first, romashka)
	geo := grGeoOf(t, baseURL, first, romashka)
	grLeave(t, baseURL, first, geo.ID)

	g := grGet(t, baseURL, viewer, geo.ID)
	if g.Members != 0 || g.Membership != grNone {
		t.Errorf("пустая геогруппа: members = %d, membership = %q", g.Members, g.Membership)
	}
	if found := grFind(grList(t, baseURL, viewer, grParams("available", grPlace, romashka.Name)), geo.ID); found == nil {
		t.Error("пустой геогруппы нет в «Найти»")
	}

	grSettle(t, baseURL, second, romashka)
	if got := grGeoOf(t, baseURL, second, romashka); got.ID != geo.ID || got.Membership != grMember || got.Members != 1 {
		t.Errorf("следующий выбравший: группа %s membership %q members %d; ожидалась %s member 1",
			got.ID, got.Membership, got.Members, geo.ID)
	}

	deleteMeOK(t, baseURL, second.token)
	g = grGet(t, baseURL, viewer, geo.ID)
	if g.Members != 0 {
		t.Errorf("после удаления аккаунта участника members = %d, ожидалось 0", g.Members)
	}
	grRequireGeoShape(t, g, romashka, "геогруппа после удаления аккаунта")
}

// ============================================================================
// Создать (требования 3, 10)
// ============================================================================

// Группа по интересам: создатель — хозяин и первый участник, 201 Group;
// название и описание обрезаются; kind можно не слать.
func TestGroupCreateInterest(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrNamed(t, baseURL, 1, "kolya_dacha")

	resp := grCreateReq(t, baseURL, owner.token, grDraft("  Розы и клематисы \t", "  Обрезка и укрытие \n"))
	g := grGroupOK(t, resp, http.StatusCreated, "создание")

	if g.Name != "Розы и клематисы" {
		t.Errorf("name = %q, ожидалось обрезанное «Розы и клематисы»", g.Name)
	}
	if g.Description != "Обрезка и укрытие" {
		t.Errorf("description = %q, ожидалось обрезанное", g.Description)
	}
	if g.Kind != grInterest {
		t.Errorf("kind = %q, ожидалось interest", g.Kind)
	}
	if g.Owner == nil || g.Owner.ID != owner.id || g.Owner.Nickname != "kolya_dacha" {
		t.Errorf("owner = %+v, ожидался %s kolya_dacha", g.Owner, owner.id)
	}
	if g.Members != 1 || g.Membership != grOwner {
		t.Errorf("members = %d, membership = %q; ожидалось 1, owner", g.Members, g.Membership)
	}

	got := grGet(t, baseURL, owner, g.ID)
	if got.Name != g.Name || got.Description != g.Description || got.CreatedAt != g.CreatedAt ||
		got.Membership != grOwner || got.Members != 1 {
		t.Errorf("GET отдаёт другую группу: %+v, создана %+v", got, g)
	}
	grRequireIn(t, baseURL, owner, g.ID, true, grOwner, "своя группа")

	// Без описания — пустая строка; названия не уникальны.
	twin := grInterestGroup(t, baseURL, owner, "Розы и клематисы")
	if twin.Description != "" {
		t.Errorf("без описания description = %q, ожидалась пустая строка", twin.Description)
	}
	if twin.ID == g.ID {
		t.Error("вторая группа с тем же названием получила тот же id")
	}

	// Старая сборка шлёт kind interest — то же самое.
	withKind := grCreate(t, baseURL, owner, map[string]any{"name": "Рыбалка", "kind": grInterest})
	if withKind.Kind != grInterest || withKind.Membership != grOwner {
		t.Errorf("с kind interest: kind = %q, membership = %q", withKind.Kind, withKind.Membership)
	}
}

// Поля первой версии: join_policy — любой, пропускается; place_id и
// radius_km пустые — пропускаются. Группа всё равно открытая, по интересам.
func TestGroupCreateIgnoresOldFields(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	guest := newGrUser(t, baseURL, 2)

	cases := []struct {
		name  string
		extra map[string]any
	}{
		{"join_policy invite", map[string]any{"kind": grInterest, "join_policy": "invite"}},
		{"join_policy request", map[string]any{"join_policy": "request"}},
		{"join_policy неизвестный", map[string]any{"join_policy": "anyone"}},
		{"join_policy пустой", map[string]any{"join_policy": ""}},
		{"join_policy null", map[string]any{"join_policy": nil}},
		{"place_id и radius_km null", map[string]any{"kind": grInterest, "join_policy": grOpen, "place_id": nil, "radius_km": nil}},
		{"place_id пустой", map[string]any{"place_id": ""}},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := grDraft(fmt.Sprintf("Группа %d", i), "")
			for k, v := range c.extra {
				body[k] = v
			}
			g := grCreate(t, baseURL, owner, body)
			if g.Kind != grInterest || g.JoinPolicy != grOpen {
				t.Errorf("kind = %q, join_policy = %q; ожидалось interest, open", g.Kind, g.JoinPolicy)
			}

			// Открытая на деле: вступают сразу.
			grJoin(t, baseURL, guest, g.ID, grMember)
		})
	}
}

// Ошибки создания: коды из таблицы, и группа не создаётся.
func TestGroupCreateValidation(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	owner := newGrUser(t, baseURL, 1)

	known := grLearnPlain(t, baseURL, fake, owner.token, "снт Известное-гр")

	with := func(extra map[string]any) map[string]any {
		out := grDraft("Розы", "")
		for k, v := range extra {
			out[k] = v
		}
		return out
	}

	cases := []struct {
		name   string
		body   any
		status int
		code   string
	}{
		{"пустое название", with(map[string]any{"name": ""}), 400, "invalid_group"},
		{"название из пробелов", with(map[string]any{"name": "   \t "}), 400, "invalid_group"},
		{"название длиннее 60", with(map[string]any{"name": strings.Repeat("я", 61)}), 400, "invalid_group"},
		{"описание длиннее 500", with(map[string]any{"description": strings.Repeat("я", 501)}), 400, "invalid_group"},
		{"kind place", with(map[string]any{"kind": grPlace}), 400, "invalid_group"},
		{"kind place с местом", with(map[string]any{"kind": grPlace, "place_id": known.ID}), 400, "invalid_group"},
		{"место известного пункта", with(map[string]any{"place_id": known.ID}), 400, "invalid_group"},
		{"место неизвестного пункта", with(map[string]any{"place_id": "27290000-0000-4000-8000-999999999999"}), 400, "invalid_group"},
		{"радиус", with(map[string]any{"radius_km": 5}), 400, "invalid_group"},
		{"неизвестный kind", with(map[string]any{"kind": "club"}), 400, "invalid_request"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			grRequireCode(t, grCreateReq(t, baseURL, owner.token, c.body), c.status, c.code, c.name)
		})
	}

	t.Run("не JSON", func(t *testing.T) {
		grRequireCode(t, ebdRaw(t, http.MethodPost, grGroupsURL(baseURL), owner.token, "{не json"),
			http.StatusBadRequest, "invalid_request", "не JSON")
	})

	if mine := grMine(t, baseURL, owner); len(mine) != 0 {
		t.Fatalf("после отказов группы появились: %q", grNames(mine))
	}
	if found := grPlaceGroups(t, baseURL, owner, known); len(found) != 0 {
		t.Errorf("попытки создать группу с местом создали геогруппу: %q", grNames(found))
	}

	// Границы длины принимаются; длина — в знаках, не байтах.
	ok := grCreate(t, baseURL, owner, grDraft(strings.Repeat("я", 60), strings.Repeat("я", 500)))
	if ok.Name != strings.Repeat("я", 60) || ok.Description != strings.Repeat("я", 500) {
		t.Error("название в 60 и описание в 500 знаков сохранились не так")
	}
	// Пробелы по краям в длину не входят.
	grCreate(t, baseURL, owner, grDraft("  "+strings.Repeat("ё", 60)+"  ", " "+strings.Repeat("ё", 500)+" "))
}

// Группы, которой нет, — 404 group_not_found на её ручках (требование 16).
func TestGroupNotFound(t *testing.T) {
	baseURL := startAPI(t)
	who := newGrUser(t, baseURL, 1)
	other := newGrUser(t, baseURL, 2)

	nf := func(resp *http.Response, what string) {
		t.Helper()
		grRequireCode(t, resp, http.StatusNotFound, "group_not_found", what)
	}
	nf(grGetReq(t, baseURL, who.token, grNoGroup), "GET")
	nf(grDeleteReq(t, baseURL, who.token, grNoGroup), "DELETE")
	nf(grJoinReq(t, baseURL, who.token, grNoGroup), "PUT membership")
	nf(grLeaveReq(t, baseURL, who.token, grNoGroup), "DELETE membership")
	nf(grMembersReq(t, baseURL, who.token, grNoGroup, ""), "GET members")
	nf(grMembersReq(t, baseURL, who.token, grNoGroup, grInvited), "GET members?state=invited")
	nf(grInviteReq(t, baseURL, who.token, grNoGroup, other.id), "PUT members/{userId}")
	nf(grRemoveReq(t, baseURL, who.token, grNoGroup, other.id), "DELETE members/{userId}")
}

// ============================================================================
// Удалить (требования 11, 27)
// ============================================================================

// Группу по интересам удаляет только хозяин; уходит со всем составом
// и приглашениями.
func TestGroupDelete(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	guest := newGrUser(t, baseURL, 3)
	stranger := newGrUser(t, baseURL, 4)

	g := grInterestGroup(t, baseURL, owner, "Розы-удаление")
	grJoin(t, baseURL, member, g.ID, grMember)
	grInvite(t, baseURL, member, g.ID, guest.id)

	grRequireCode(t, grDeleteReq(t, baseURL, member.token, g.ID), http.StatusForbidden, "not_group_owner", "участник удаляет")
	grRequireCode(t, grDeleteReq(t, baseURL, stranger.token, g.ID), http.StatusForbidden, "not_group_owner", "посторонний удаляет")
	grRequireCode(t, grDeleteReq(t, baseURL, guest.token, g.ID), http.StatusForbidden, "not_group_owner", "приглашённый удаляет")
	grRequireMembers(t, baseURL, owner, g.ID, 2, "группа цела после отказов")
	grRequireGroupsUnread(t, baseURL, guest, 1, "приглашение до удаления")

	requireEmpty204(t, grDeleteReq(t, baseURL, owner.token, g.ID), "хозяин удаляет")

	for _, who := range []dachnik{owner, member, guest, stranger} {
		grRequireCode(t, grGetReq(t, baseURL, who.token, g.ID), http.StatusNotFound, "group_not_found", "GET удалённой")
		if grFind(grMine(t, baseURL, who), g.ID) != nil || grFind(grAvailable(t, baseURL, who), g.ID) != nil {
			t.Errorf("удалённая группа осталась в списках")
		}
	}
	grRequireCode(t, grMembersReq(t, baseURL, owner.token, g.ID, ""), http.StatusNotFound, "group_not_found", "состав удалённой")
	grRequireCode(t, grJoinReq(t, baseURL, guest.token, g.ID), http.StatusNotFound, "group_not_found", "вступить в удалённую")
	grRequireCode(t, grDeleteReq(t, baseURL, owner.token, g.ID), http.StatusNotFound, "group_not_found", "удалить ещё раз")

	grRequireList(t, grRequestKeys(grRequests(t, baseURL, guest)), nil, "приглашение после удаления группы")
	grRequireGroupsUnread(t, baseURL, guest, 0, "приглашённый после удаления группы")
}

// Геогруппу не удалить никому — 403 not_group_owner.
func TestGeoGroupCannotBeDeleted(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	member := newGrUser(t, baseURL, 1)
	stranger := newGrUser(t, baseURL, 2)

	p := grLearnPlain(t, baseURL, fake, member.token, "снт Вечное-гр")
	grSettle(t, baseURL, member, p)
	geo := grGeoOf(t, baseURL, member, p)

	grRequireCode(t, grDeleteReq(t, baseURL, member.token, geo.ID), http.StatusForbidden, "not_group_owner", "участник удаляет геогруппу")
	grRequireCode(t, grDeleteReq(t, baseURL, stranger.token, geo.ID), http.StatusForbidden, "not_group_owner", "посторонний удаляет геогруппу")

	grRequireMembers(t, baseURL, stranger, geo.ID, 1, "геогруппа цела")
}

// ============================================================================
// Расстояние и «рядом» (требования 14, 15)
// ============================================================================

// distance_km — от пункта смотрящего до места геогруппы, округлено как у
// поста; near — пункт смотрящего и есть место группы. Без пункта у
// смотрящего, без координат и у группы по интересам — поля нет, near false.
func TestGeoGroupDistanceAndNear(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	setter := newGrUser(t, baseURL, 1)

	home := grLearnNorth(t, baseURL, fake, setter.token, "снт Дом-расст", 0)
	blindPlace := grLearnPlain(t, baseURL, fake, setter.token, "снт Слепое-расст")
	grSettle(t, baseURL, setter, blindPlace)
	grSettle(t, baseURL, setter, home)
	homeGroup := grGeoOf(t, baseURL, setter, home)
	blindGroup := grGeoOf(t, baseURL, setter, blindPlace)
	hobby := grInterestGroup(t, baseURL, setter, "По интересам-расст")

	n := 1
	at := func(name string, km float64) dachnik {
		n++
		v := newGrUser(t, baseURL, n)
		p := grLearnNorth(t, baseURL, fake, v.token, name, km)
		grSettle(t, baseURL, v, p)
		return v
	}
	settledAt := func(p placePayload) dachnik {
		n++
		v := newGrUser(t, baseURL, n)
		grSettle(t, baseURL, v, p)
		return v
	}

	same := settledAt(home)
	km03 := at("снт Рядом-расст", 0.3)
	km3 := at("снт Три-расст", 3)
	km49 := at("снт Пять-расст", 4.9)
	km24 := at("снт Двадцать-расст", 24)
	n++
	nowhere := newGrUser(t, baseURL, n)
	sameBlind := settledAt(blindPlace)

	cases := []struct {
		who      string
		viewer   dachnik
		group    grGroup
		distance *int
		near     bool
	}{
		{"тот же пункт", same, homeGroup, ppInt(0), true},
		{"0,3 км — другой пункт", km03, homeGroup, ppInt(1), false},
		{"3 км", km3, homeGroup, ppInt(3), false},
		{"4,9 км", km49, homeGroup, ppInt(5), false},
		{"24 км", km24, homeGroup, ppInt(25), false},
		{"у смотрящего нет пункта", nowhere, homeGroup, nil, false},
		{"у места группы нет координат", km3, blindGroup, nil, false},
		{"тот же пункт без координат", sameBlind, blindGroup, ppInt(0), true},
		{"группа по интересам", same, hobby, nil, false},
		{"группа по интересам, смотрящий без пункта", nowhere, hobby, nil, false},
	}
	for _, c := range cases {
		t.Run(c.who, func(t *testing.T) {
			fields := grRawFields(t, grGetReq(t, baseURL, c.viewer.token, c.group.ID), c.who)
			if c.distance == nil {
				ppRequireNoDistance(t, fields, c.who+": GET")
			} else {
				ppRequireDistance(t, fields, *c.distance, c.who+": GET")
			}
			var near bool
			if err := json.Unmarshal(fields["near"], &near); err != nil {
				t.Fatalf("%s: near не bool: %s", c.who, fields["near"])
			}
			if near != c.near {
				t.Errorf("%s: GET near = %v, ожидалось %v", c.who, near, c.near)
			}

			// В списке — то же самое.
			found := grFind(grMine(t, baseURL, c.viewer), c.group.ID)
			if found == nil {
				found = grFind(grAvailable(t, baseURL, c.viewer), c.group.ID)
			}
			if found == nil {
				t.Fatalf("%s: группы нет ни в «Моих», ни в «Найти»", c.who)
			}
			if found.Near != c.near {
				t.Errorf("%s: в списке near = %v, ожидалось %v", c.who, found.Near, c.near)
			}
			switch {
			case c.distance == nil && found.DistanceKm != nil:
				t.Errorf("%s: в списке distance_km = %d, а его быть не должно", c.who, *found.DistanceKm)
			case c.distance != nil && (found.DistanceKm == nil || *found.DistanceKm != *c.distance):
				t.Errorf("%s: в списке distance_km = %v, ожидалось %d", c.who, found.DistanceKm, *c.distance)
			}
		})
	}
}

// ============================================================================
// Видимость (требование 16)
// ============================================================================

// Хозяин группы по интересам заблокировал смотрящего — группы для него
// нет; заблокированный участник остаётся в составе, хозяин может его
// убрать. Блокировка участником группу не прячет.
func TestGroupHiddenFromBlockedByOwner(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	stranger := newGrUser(t, baseURL, 2)
	member := newGrUser(t, baseURL, 3)
	shy := newGrUser(t, baseURL, 4)
	viewer := newGrUser(t, baseURL, 5)

	g := grInterestGroup(t, baseURL, owner, "Розы-блок")
	grJoin(t, baseURL, member, g.ID, grMember)
	grJoin(t, baseURL, shy, g.ID, grMember)

	blockOK(t, baseURL, owner, stranger.id)
	blockOK(t, baseURL, owner, member.id)

	grRequireHidden(t, baseURL, stranger, g.ID, owner.id, "заблокированный посторонний")
	grRequireHidden(t, baseURL, member, g.ID, owner.id, "заблокированный участник")

	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, "")), []string{owner.id, shy.id, member.id},
		"заблокированный участник остаётся в составе")
	grRequireMembers(t, baseURL, owner, g.ID, 3, "число участников с заблокированным")

	grRemove(t, baseURL, owner, g.ID, member.id)
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, "")), []string{owner.id, shy.id}, "после «Убрать»")

	// Заблокировал участник, не хозяин, — группа видна.
	blockOK(t, baseURL, shy, viewer.id)
	if got := grGet(t, baseURL, viewer, g.ID); got.Membership != grNone {
		t.Errorf("membership = %q у постороннего", got.Membership)
	}
	if grFind(grAvailable(t, baseURL, viewer), g.ID) == nil {
		t.Error("группы нет в «Найти» у того, кого заблокировал участник")
	}
}

// Геогруппу видят все, кто бы кого ни блокировал.
func TestGeoGroupVisibleDespiteBlocks(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	first := newGrUser(t, baseURL, 1)
	second := newGrUser(t, baseURL, 2)
	outsider := newGrUser(t, baseURL, 3)

	p := grLearnPlain(t, baseURL, fake, first.token, "снт Открытое-блок")
	grSettle(t, baseURL, first, p)
	grSettle(t, baseURL, second, p)
	geo := grGeoOf(t, baseURL, first, p)

	blockOK(t, baseURL, first, second.id)
	blockOK(t, baseURL, first, outsider.id)

	grRequireIn(t, baseURL, second, geo.ID, true, grMember, "заблокированный участником геогруппы")
	grRequireIn(t, baseURL, outsider, geo.ID, false, grNone, "посторонний, заблокированный участником")
	grStatus(t, grMembersReq(t, baseURL, outsider.token, geo.ID, ""), http.StatusOK, "состав глазами заблокированного")
}

// ============================================================================
// Вступить и выйти (требования 4, 17, 18)
// ============================================================================

// Вступить — сразу участник; повтор ничего не меняет; хозяину — owner;
// выйти — 204, без строки — тоже; хозяину — 409 owner_cannot_leave.
func TestGroupJoinAndLeave(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)
	nobody := newGrUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, owner, "Любители рыбалки-вступ")

	joined := grJoin(t, baseURL, me, g.ID, grMember)
	if joined.Members != 2 {
		t.Errorf("после вступления members = %d, ожидалось 2", joined.Members)
	}
	if again := grJoin(t, baseURL, me, g.ID, grMember); again.Members != 2 {
		t.Errorf("повторное вступление: members = %d, ожидалось 2", again.Members)
	}
	if own := grJoin(t, baseURL, owner, g.ID, grOwner); own.Members != 2 {
		t.Errorf("«Вступить» хозяина: members = %d, ожидалось 2", own.Members)
	}
	grRequireIn(t, baseURL, me, g.ID, true, grMember, "после вступления")

	grLeave(t, baseURL, me, g.ID)
	if got := grGet(t, baseURL, me, g.ID); got.Membership != grNone || got.Members != 1 {
		t.Errorf("после выхода membership = %q, members = %d", got.Membership, got.Members)
	}
	grRequireIn(t, baseURL, me, g.ID, false, grNone, "после выхода")
	grLeave(t, baseURL, me, g.ID)
	grLeave(t, baseURL, nobody, g.ID)

	grRequireCode(t, grLeaveReq(t, baseURL, owner.token, g.ID), http.StatusConflict, "owner_cannot_leave", "хозяин выходит")
	grRequireMembers(t, baseURL, owner, g.ID, 1, "хозяин остался")

	// Вышедший вступает снова.
	grJoin(t, baseURL, me, g.ID, grMember)
}

// В геогруппу вступает любой, выйти может любой участник.
func TestGeoGroupJoinAndLeave(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	local := newGrUser(t, baseURL, 1)
	andrey := newGrUser(t, baseURL, 2)

	p := grLearnPlain(t, baseURL, fake, local.token, "снт Вторая-дача")
	grSettle(t, baseURL, local, p)
	geo := grGeoOf(t, baseURL, local, p)

	joined := grJoin(t, baseURL, andrey, geo.ID, grMember)
	if joined.Members != 2 {
		t.Errorf("members = %d, ожидалось 2", joined.Members)
	}
	grRequireIn(t, baseURL, andrey, geo.ID, true, grMember, "вступивший в геогруппу без пункта")

	grLeave(t, baseURL, local, geo.ID)
	grRequireIn(t, baseURL, local, geo.ID, false, grNone, "вышедший из геогруппы своего пункта")
	grRequireMembers(t, baseURL, andrey, geo.ID, 1, "после выхода")

	grJoin(t, baseURL, local, geo.ID, grMember)
	grRequireMembers(t, baseURL, andrey, geo.ID, 2, "вернулся сам")
}

// ============================================================================
// Списки (требования 19, 20)
// ============================================================================

// mine — где хозяин или участник (по умолчанию); available — остальные
// видимые, с приглашением тоже; неизвестные scope и kind — 400.
func TestGroupListScopes(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	myPlace := grLearnPlain(t, baseURL, fake, me.token, "снт Моё-списки")
	theirPlace := grLearnPlain(t, baseURL, fake, owner.token, "снт Чужое-списки")
	grSettle(t, baseURL, me, myPlace)
	grSettle(t, baseURL, owner, theirPlace)
	myGeo := grGeoOf(t, baseURL, me, myPlace)
	theirGeo := grGeoOf(t, baseURL, me, theirPlace)

	own := grInterestGroup(t, baseURL, me, "Своя-списки")
	joined := grInterestGroup(t, baseURL, owner, "Вступил-списки")
	invited := grInterestGroup(t, baseURL, owner, "Позвали-списки")
	other := grInterestGroup(t, baseURL, owner, "Чужая-списки")

	grJoin(t, baseURL, me, joined.ID, grMember)
	grInvite(t, baseURL, owner, invited.ID, me.id)

	wantMine := map[string]string{own.ID: grOwner, joined.ID: grMember, myGeo.ID: grMember}
	wantAvailable := map[string]string{invited.ID: grInvited, other.ID: grNone, theirGeo.ID: grNone}

	check := func(items []grGroup, want, absent map[string]string, where string) {
		t.Helper()
		for id, membership := range want {
			g := grFind(items, id)
			if g == nil {
				t.Errorf("%s: нет группы %s", where, id)
				continue
			}
			if g.Membership != membership {
				t.Errorf("%s: у %q membership = %q, ожидалось %q", where, g.Name, g.Membership, membership)
			}
		}
		for id := range absent {
			if g := grFind(items, id); g != nil {
				t.Errorf("%s: лишняя группа %q", where, g.Name)
			}
		}
	}

	check(grList(t, baseURL, me, nil), wantMine, wantAvailable, "без scope")
	check(grMine(t, baseURL, me), wantMine, wantAvailable, "scope=mine")
	check(grAvailable(t, baseURL, me), wantAvailable, wantMine, "scope=available")

	for _, params := range []url.Values{
		grParams("all", "", ""),
		grParams("", "hobby", ""),
		grParams("available", "garden", ""),
	} {
		grRequireCode(t, grListReq(t, baseURL, me.token, params), http.StatusBadRequest, "invalid_request",
			"GET /groups?"+params.Encode())
	}
}

// kind — только этого типа; q — подстрока названия или описания без учёта
// регистра; q из пробелов — как без него. Фильтры работают на обеих
// вкладках.
func TestGroupListFilters(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	fishing := grInterestGroup(t, baseURL, owner, "Любители рыбалки-фильтр")
	roses := grCreate(t, baseURL, owner, grDraft("Розы и клематисы-фильтр", "Обрезка, подкормка, УКРЫТИЕ на зиму"))
	pondPlace := grLearnPlain(t, baseURL, fake, owner.token, "д Рыбное-фильтр")
	sntPlace := grLearnPlain(t, baseURL, fake, owner.token, "снт Ромашка-фильтр")
	grSettle(t, baseURL, owner, pondPlace)
	grSettle(t, baseURL, owner, sntPlace)
	pond := grGeoOf(t, baseURL, owner, pondPlace)
	snt := grGeoOf(t, baseURL, owner, sntPlace)

	known := []string{fishing.ID, roses.ID, pond.ID, snt.ID}
	ids := func(items []grGroup) []string { return sortedCopy(grOnly(grIDs(items), known...)) }

	cases := []struct {
		name   string
		params url.Values
		want   []string
	}{
		{"kind=interest", grParams("available", grInterest, ""), []string{fishing.ID, roses.ID}},
		{"kind=place", grParams("available", grPlace, ""), []string{pond.ID, snt.ID}},
		{"q в названии", grParams("available", "", "рыб"), []string{fishing.ID, pond.ID}},
		{"q в другом регистре", grParams("available", "", "РЫБ"), []string{fishing.ID, pond.ID}},
		{"q в описании", grParams("available", "", "укрытие"), []string{roses.ID}},
		{"q в описании геогруппы — area", grParams("available", grPlace, "дмитровский"), []string{pond.ID, snt.ID}},
		{"q с пробелами по краям", grParams("available", "", "  клематис "), []string{roses.ID}},
		{"q и kind", grParams("available", grInterest, "рыб"), []string{fishing.ID}},
		{"q из пробелов — как без него", grParams("available", "", "   "), known},
		{"ничего не нашлось", grParams("available", "", "тюльпаны-фильтр"), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			grRequireList(t, ids(grList(t, baseURL, me, c.params)), sortedCopy(c.want), c.name)
		})
	}

	t.Run("mine", func(t *testing.T) {
		grRequireList(t, ids(grList(t, baseURL, owner, grParams("mine", grPlace, "рыб"))), []string{pond.ID}, "mine kind=place q=рыб")
		grRequireList(t, ids(grList(t, baseURL, owner, grParams("", grInterest, ""))),
			sortedCopy([]string{fishing.ID, roses.ID}), "kind=interest без scope")
	})
}

// «Мои»: сначала геогруппы (near выше), потом остальные; внутри — по
// названию без учёта регистра.
func TestGroupListMineOrder(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	// Пункты выбираются по очереди; последний — текущий, его группа near.
	var known []string
	for _, name := range []string{"снт Берёзка-порядок", "снт Яблонька-порядок", "снт акация-порядок"} {
		p := grLearnPlain(t, baseURL, fake, me.token, name)
		grSettle(t, baseURL, me, p)
		known = append(known, grGeoOf(t, baseURL, me, p).ID)
	}
	current := grLearnPlain(t, baseURL, fake, me.token, "снт Ясное-порядок")
	grSettle(t, baseURL, me, current)
	known = append(known, grGeoOf(t, baseURL, me, current).ID)

	known = append(known, grInterestGroup(t, baseURL, me, "Бархатцы").ID)
	for _, name := range []string{"Яблони", "агрономы", "Bees", "apples"} {
		g := grInterestGroup(t, baseURL, owner, name)
		grJoin(t, baseURL, me, g.ID, grMember)
		known = append(known, g.ID)
	}

	mine := grMine(t, baseURL, me)
	byID := map[string]string{}
	for _, g := range mine {
		byID[g.ID] = g.Name
	}
	var names []string
	for _, id := range grOnly(grIDs(mine), known...) {
		names = append(names, byID[id])
	}

	grRequireList(t, names, []string{
		"снт Ясное-порядок",
		"снт акация-порядок", "снт Берёзка-порядок", "снт Яблонька-порядок",
		"apples", "Bees", "агрономы", "Бархатцы", "Яблони",
	}, "порядок «Моих»")
}

// «Найти»: приглашения, потом near, потом больше участников, при
// равенстве — по названию.
func TestGroupListAvailableOrder(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	owner := newGrUser(t, baseURL, 1)
	helper1 := newGrUser(t, baseURL, 2)
	helper2 := newGrUser(t, baseURL, 3)
	me := newGrUser(t, baseURL, 4)

	// Своя геогруппа, из которой вышел, — near, хоть в ней никого.
	home := grLearnPlain(t, baseURL, fake, me.token, "снт Вега-найти")
	grSettle(t, baseURL, me, home)
	vega := grGeoOf(t, baseURL, me, home)
	grLeave(t, baseURL, me, vega.ID)

	// Чужая геогруппа на двоих.
	far := grLearnPlain(t, baseURL, fake, owner.token, "снт Озеро-найти")
	grSettle(t, baseURL, helper1, far)
	grSettle(t, baseURL, helper2, far)
	lake := grGeoOf(t, baseURL, me, far)

	withMembers := func(g grGroup, people ...dachnik) {
		for _, p := range people {
			grJoin(t, baseURL, p, g.ID, grMember)
		}
	}

	alpha := grInterestGroup(t, baseURL, owner, "Альфа-найти")
	beta := grInterestGroup(t, baseURL, owner, "Бета-найти")
	withMembers(beta, helper1, helper2)
	delta := grInterestGroup(t, baseURL, owner, "Дельта-найти")
	withMembers(delta, helper1, helper2)
	gamma := grInterestGroup(t, baseURL, owner, "Гамма-найти")
	grInvite(t, baseURL, owner, gamma.ID, me.id)
	zhuk := grInterestGroup(t, baseURL, owner, "Жук-найти")
	own := grInterestGroup(t, baseURL, me, "Своя-найти")

	known := []string{vega.ID, lake.ID, alpha.ID, beta.ID, delta.ID, gamma.ID, zhuk.ID, own.ID}
	available := grAvailable(t, baseURL, me)
	byID := map[string]string{}
	for _, g := range available {
		byID[g.ID] = g.Name
	}
	var names []string
	for _, id := range grOnly(grIDs(available), known...) {
		names = append(names, byID[id])
	}

	grRequireList(t, names,
		[]string{"Гамма-найти", "снт Вега-найти", "Бета-найти", "Дельта-найти", "снт Озеро-найти", "Альфа-найти", "Жук-найти"},
		"порядок «Найти»")
}

// В списке не больше 100 групп.
func TestGroupListLimit(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	for i := 0; i < 101; i++ {
		grInterestGroup(t, baseURL, owner, fmt.Sprintf("Группа-лимит %03d", i))
	}

	if n := len(grMine(t, baseURL, owner)); n != 100 {
		t.Errorf("«Мои»: групп %d, ожидалось 100", n)
	}
	if n := len(grAvailable(t, baseURL, me)); n != 100 {
		t.Errorf("«Найти»: групп %d, ожидалось 100", n)
	}
}

// ============================================================================
// Состав и приглашения (требования 21–25)
// ============================================================================

// Состав группы по интересам: хозяин первым, дальше новые выше; role и
// state у каждой строки; приглашённые в состав не входят; видят все, кому
// видна группа.
func TestGroupMembersOrder(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrNamed(t, baseURL, 1, "nikolay_sostav")
	a := newGrUser(t, baseURL, 2)
	b := newGrUser(t, baseURL, 3)
	c := newGrUser(t, baseURL, 4)
	guest := newGrUser(t, baseURL, 5)
	stranger := newGrUser(t, baseURL, 6)

	g := grInterestGroup(t, baseURL, owner, "Розы-состав")
	for _, p := range []dachnik{a, b, c} {
		grJoin(t, baseURL, p, g.ID, grMember)
	}
	grInvite(t, baseURL, a, g.ID, guest.id)

	for _, viewer := range []dachnik{owner, a, stranger, guest} {
		for _, state := range []string{"", grMember} {
			items := grMembers(t, baseURL, viewer, g.ID, state)
			grRequireList(t, grMemberIDs(items), []string{owner.id, c.id, b.id, a.id}, "состав, state="+state)
			if len(items) != 4 {
				continue
			}
			if items[0].Role != grOwner || items[0].State != grMember || items[0].User.Nickname != "nikolay_sostav" {
				t.Errorf("строка хозяина: %+v", items[0])
			}
			for _, m := range items[1:] {
				if m.Role != grMember || m.State != grMember {
					t.Errorf("строка участника: role = %q, state = %q", m.Role, m.State)
				}
			}
		}
	}
	grRequireMembers(t, baseURL, stranger, g.ID, 4, "число участников с хозяином, без приглашённого")
}

// Состав геогруппы: хозяина нет, все — участники, новые выше.
func TestGeoGroupMembersOrder(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	a := newGrUser(t, baseURL, 1)
	b := newGrUser(t, baseURL, 2)
	c := newGrUser(t, baseURL, 3)
	stranger := newGrUser(t, baseURL, 4)

	p := grLearnPlain(t, baseURL, fake, a.token, "снт Состав-гео")
	grSettle(t, baseURL, a, p)
	geo := grGeoOf(t, baseURL, a, p)
	grSettle(t, baseURL, b, p)
	grJoin(t, baseURL, c, geo.ID, grMember)

	items := grMembers(t, baseURL, stranger, geo.ID, "")
	grRequireList(t, grMemberIDs(items), []string{c.id, b.id, a.id}, "состав геогруппы")
	for _, m := range items {
		if m.Role != grMember || m.State != grMember {
			t.Errorf("строка геогруппы: role = %q, state = %q", m.Role, m.State)
		}
	}
}

// Люди, заблокировавшие смотрящего, в составе ему не показываются.
func TestGroupMembersHideThoseWhoBlockedViewer(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	shy := newGrUser(t, baseURL, 2)
	viewer := newGrUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, owner, "Розы-скрытые")
	grJoin(t, baseURL, shy, g.ID, grMember)
	grJoin(t, baseURL, viewer, g.ID, grMember)

	blockOK(t, baseURL, shy, viewer.id)

	grRequireList(t, grMemberIDs(grMembers(t, baseURL, viewer, g.ID, "")), []string{owner.id, viewer.id},
		"глазами заблокированного")
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, "")), []string{owner.id, viewer.id, shy.id},
		"глазами хозяина")
}

// state=invited — ждущие приглашения, новые сверху, видят участники
// группы (в том числе геогруппы); остальным — 403 not_group_member.
// state=requested — пустой список; неизвестный state — 400.
func TestGroupInvitedList(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	i1 := newGrUser(t, baseURL, 3)
	i2 := newGrUser(t, baseURL, 4)
	stranger := newGrUser(t, baseURL, 5)

	g := grInterestGroup(t, baseURL, owner, "Розы-приглашены")
	grJoin(t, baseURL, member, g.ID, grMember)
	grInvite(t, baseURL, owner, g.ID, i1.id)
	grInvite(t, baseURL, member, g.ID, i2.id)

	for _, viewer := range []dachnik{owner, member} {
		invites := grMembers(t, baseURL, viewer, g.ID, grInvited)
		grRequireList(t, grMemberIDs(invites), []string{i2.id, i1.id}, "приглашения")
		for _, m := range invites {
			if m.State != grInvited || m.Role != grMember {
				t.Errorf("приглашение: state = %q, role = %q", m.State, m.Role)
			}
		}
		grRequireList(t, grMemberIDs(grMembers(t, baseURL, viewer, g.ID, grRequested)), nil, "state=requested")
	}

	for _, who := range []struct {
		name string
		d    dachnik
	}{{"посторонний", stranger}, {"приглашённый", i1}} {
		grRequireCode(t, grMembersReq(t, baseURL, who.d.token, g.ID, grInvited), http.StatusForbidden, "not_group_member",
			"state=invited: "+who.name)
	}
	grRequireCode(t, grMembersReq(t, baseURL, owner.token, g.ID, "banned"), http.StatusBadRequest, "invalid_request",
		"неизвестный state")

	// Геогруппа: приглашения видят её участники.
	a := newGrUser(t, baseURL, 6)
	b := newGrUser(t, baseURL, 7)
	guest := newGrUser(t, baseURL, 8)
	p := grLearnPlain(t, baseURL, fake, a.token, "снт Приглашены-гео")
	grSettle(t, baseURL, a, p)
	grSettle(t, baseURL, b, p)
	geo := grGeoOf(t, baseURL, a, p)
	grInvite(t, baseURL, a, geo.ID, guest.id)

	grRequireList(t, grMemberIDs(grMembers(t, baseURL, b, geo.ID, grInvited)), []string{guest.id}, "приглашения геогруппы")
	grRequireCode(t, grMembersReq(t, baseURL, guest.token, geo.ID, grInvited), http.StatusForbidden, "not_group_member",
		"приглашения геогруппы глазами приглашённого")
	grRequireCode(t, grMembersReq(t, baseURL, stranger.token, geo.ID, grInvited), http.StatusForbidden, "not_group_member",
		"приглашения геогруппы глазами постороннего")
}

// Приглашает любой участник: хозяин, участник группы по интересам,
// участник геогруппы. Повтор и приглашение участника ничего не меняют.
// Приглашённый вступает (требования 17, 23).
func TestGroupInvite(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	guest := newGrNamed(t, baseURL, 3, "andrey_dacha")
	other := newGrUser(t, baseURL, 4)

	g := grInterestGroup(t, baseURL, owner, "Рыбалка-пригл")
	grJoin(t, baseURL, member, g.ID, grMember)

	invited := grInviteOK(t, baseURL, member, g.ID, guest.id, grInvited)
	if invited.User.Nickname != "andrey_dacha" || invited.Role != grMember {
		t.Errorf("приглашение: %+v", invited)
	}
	again := grInviteOK(t, baseURL, owner, g.ID, guest.id, grInvited)
	if again.CreatedAt != invited.CreatedAt {
		t.Errorf("повтор приглашения изменил created_at: %s → %s", invited.CreatedAt, again.CreatedAt)
	}
	grInvite(t, baseURL, owner, g.ID, other.id)

	grRequireIn(t, baseURL, guest, g.ID, false, grInvited, "приглашённый")
	grRequireMembers(t, baseURL, owner, g.ID, 2, "приглашённые — ещё не участники")

	// Пригласить участника или хозяина — ничего не меняется.
	if m := grInviteOK(t, baseURL, member, g.ID, owner.id, grMember); m.Role != grOwner {
		t.Errorf("«пригласить» хозяина: role = %q, ожидалось owner", m.Role)
	}
	if m := grInviteOK(t, baseURL, owner, g.ID, member.id, grMember); m.Role != grMember {
		t.Errorf("«пригласить» участника: role = %q", m.Role)
	}

	joined := grJoin(t, baseURL, guest, g.ID, grMember)
	if joined.Members != 3 {
		t.Errorf("после принятия приглашения members = %d, ожидалось 3", joined.Members)
	}
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, grInvited)), []string{other.id}, "после принятия")

	// Геогруппа: участник зовёт человека из другого СНТ.
	local := newGrUser(t, baseURL, 5)
	andrey := newGrUser(t, baseURL, 6)
	p := grLearnPlain(t, baseURL, fake, local.token, "снт Ромашка-пригл")
	grSettle(t, baseURL, local, p)
	geo := grGeoOf(t, baseURL, local, p)

	grInvite(t, baseURL, local, geo.ID, andrey.id)
	grRequireIn(t, baseURL, andrey, geo.ID, false, grInvited, "приглашённый в геогруппу")
	grJoin(t, baseURL, andrey, geo.ID, grMember)
	grRequireMembers(t, baseURL, local, geo.ID, 2, "геогруппа после принятия")
}

// Ошибки приглашения (требование 23).
func TestGroupInviteErrors(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	target := newGrUser(t, baseURL, 3)
	blocker := newGrUser(t, baseURL, 4)
	blocked := newGrUser(t, baseURL, 5)
	guest := newGrUser(t, baseURL, 6)

	g := grInterestGroup(t, baseURL, owner, "Розы-ошибки")
	grJoin(t, baseURL, member, g.ID, grMember)
	grInvite(t, baseURL, owner, g.ID, guest.id)

	grRequireCode(t, grInviteReq(t, baseURL, target.token, g.ID, blocked.id), http.StatusForbidden, "not_group_member", "посторонний приглашает")
	grRequireCode(t, grInviteReq(t, baseURL, guest.token, g.ID, target.id), http.StatusForbidden, "not_group_member", "приглашённый приглашает")
	grRequireCode(t, grInviteReq(t, baseURL, member.token, g.ID, member.id), http.StatusBadRequest, "cannot_invite_self", "участник себя")
	grRequireCode(t, grInviteReq(t, baseURL, owner.token, g.ID, owner.id), http.StatusBadRequest, "cannot_invite_self", "хозяин себя")
	grRequireCode(t, grInviteReq(t, baseURL, member.token, g.ID, grNoUser), http.StatusNotFound, "user_not_found", "человека нет")

	blockOK(t, baseURL, blocker, member.id)
	grRequireCode(t, grInviteReq(t, baseURL, member.token, g.ID, blocker.id), http.StatusNotFound, "user_not_found",
		"человек заблокировал приглашающего")

	blockOK(t, baseURL, member, blocked.id)
	grRequireCode(t, grInviteReq(t, baseURL, member.token, g.ID, blocked.id), http.StatusConflict, "user_blocked",
		"приглашающий заблокировал человека")

	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, grInvited)), []string{guest.id}, "после отказов")
	grRequireMembership(t, baseURL, target, g.ID, grNone, "после отказов")

	// Геогруппа: не участник не приглашает.
	local := newGrUser(t, baseURL, 7)
	p := grLearnPlain(t, baseURL, fake, local.token, "снт Ошибки-гео")
	grSettle(t, baseURL, local, p)
	geo := grGeoOf(t, baseURL, local, p)
	grRequireCode(t, grInviteReq(t, baseURL, target.token, geo.ID, owner.id), http.StatusForbidden, "not_group_member",
		"посторонний приглашает в геогруппу")
}

// DELETE members/{userId}: хозяин группы по интересам отзывает приглашение
// (чьё угодно) и убирает участника; без строки — 204; не хозяин — 403;
// себя — 409. Убранный вступает снова (требования 24, 25).
func TestGroupRemoveMember(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	guest := newGrUser(t, baseURL, 3)
	nobody := newGrUser(t, baseURL, 4)

	g := grInterestGroup(t, baseURL, owner, "Розы-убрать")
	grJoin(t, baseURL, member, g.ID, grMember)
	grInvite(t, baseURL, member, g.ID, guest.id)

	grRequireCode(t, grRemoveReq(t, baseURL, member.token, g.ID, guest.id), http.StatusForbidden, "not_group_owner", "участник отзывает своё приглашение")
	grRequireCode(t, grRemoveReq(t, baseURL, nobody.token, g.ID, member.id), http.StatusForbidden, "not_group_owner", "посторонний убирает")
	grRequireCode(t, grRemoveReq(t, baseURL, owner.token, g.ID, owner.id), http.StatusConflict, "owner_cannot_leave", "хозяин убирает себя")
	grRequireMembership(t, baseURL, guest, g.ID, grInvited, "после отказов")

	grRemove(t, baseURL, owner, g.ID, guest.id)
	grRequireMembership(t, baseURL, guest, g.ID, grNone, "приглашение отозвано")

	grRemove(t, baseURL, owner, g.ID, member.id)
	if got := grGet(t, baseURL, member, g.ID); got.Membership != grNone || got.Members != 1 {
		t.Errorf("убранный участник: membership = %q, members = %d", got.Membership, got.Members)
	}

	// Без строки — не ошибка.
	grRemove(t, baseURL, owner, g.ID, nobody.id)
	grRemove(t, baseURL, owner, g.ID, member.id)

	// Убранный вступает снова.
	grJoin(t, baseURL, member, g.ID, grMember)
	grRequireMembers(t, baseURL, owner, g.ID, 2, "вернулся")
}

// В геогруппе убрать никого нельзя: любой — 403 not_group_owner, даже
// себя и даже своё приглашение (требование 24).
func TestGeoGroupRemoveMemberForbidden(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	a := newGrUser(t, baseURL, 1)
	b := newGrUser(t, baseURL, 2)
	guest := newGrUser(t, baseURL, 3)
	stranger := newGrUser(t, baseURL, 4)

	p := grLearnPlain(t, baseURL, fake, a.token, "снт Убрать-гео")
	grSettle(t, baseURL, a, p)
	grSettle(t, baseURL, b, p)
	geo := grGeoOf(t, baseURL, a, p)
	grInvite(t, baseURL, a, geo.ID, guest.id)

	forbidden := func(who dachnik, whom, what string) {
		t.Helper()
		grRequireCode(t, grRemoveReq(t, baseURL, who.token, geo.ID, whom), http.StatusForbidden, "not_group_owner", what)
	}
	forbidden(a, b.id, "участник убирает участника")
	forbidden(a, guest.id, "участник отзывает своё приглашение")
	forbidden(a, a.id, "участник убирает себя")
	forbidden(stranger, a.id, "посторонний убирает")

	grRequireMembers(t, baseURL, a, geo.ID, 2, "состав цел")
	grRequireMembership(t, baseURL, guest, geo.ID, grInvited, "приглашение цело")
}

// ============================================================================
// Приглашения в «Уведомлениях» (требования 26–28)
// ============================================================================

// Смотрящему — только его ждущие приглашения, новые сверху; user — кто
// пригласил (не обязательно хозяин); ответил, отклонил, отозвали — строка
// пропадает; groups в счётчике — сколько их.
func TestGroupRequestsList(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	nikolay := newGrNamed(t, baseURL, 1, "nikolay_uved")
	galina := newGrNamed(t, baseURL, 2, "galina_uved")
	valya := newGrNamed(t, baseURL, 3, "valya_uved")
	petya := newGrNamed(t, baseURL, 4, "petya_uved")

	roses := grInterestGroup(t, baseURL, nikolay, "Розы-уведомления")
	fishing := grInterestGroup(t, baseURL, nikolay, "Рыбалка-уведомления")
	grJoin(t, baseURL, galina, fishing.ID, grMember)
	p := grLearnPlain(t, baseURL, fake, petya.token, "снт Ромашка-уведомления")
	grSettle(t, baseURL, petya, p)
	geo := grGeoOf(t, baseURL, petya, p)

	if items := grRequests(t, baseURL, nikolay); len(items) != 0 {
		t.Fatalf("вступление в открытую группу дало хозяину строку: %q", grRequestKeys(items))
	}
	grRequireGroupsUnread(t, baseURL, nikolay, 0, "хозяин после вступления участника")
	grRequireGroupsUnread(t, baseURL, valya, 0, "до приглашений")

	grInvite(t, baseURL, nikolay, roses.ID, valya.id)
	grInvite(t, baseURL, petya, geo.ID, valya.id)
	grInvite(t, baseURL, galina, fishing.ID, valya.id)

	items := grRequests(t, baseURL, valya)
	grRequireList(t, grRequestKeys(items), []string{
		grInviteKey(fishing.ID, galina.id),
		grInviteKey(geo.ID, petya.id),
		grInviteKey(roses.ID, nikolay.id),
	}, "приглашения Вале")
	if len(items) == 3 {
		if items[0].Group.Name != "Рыбалка-уведомления" || items[0].User.Nickname != "galina_uved" {
			t.Errorf("строка приглашения от участника: группа %+v, человек %+v", items[0].Group, items[0].User)
		}
		if items[1].Group.Name != p.Name || items[1].User.Nickname != "petya_uved" {
			t.Errorf("строка приглашения в геогруппу: группа %+v, человек %+v", items[1].Group, items[1].User)
		}
	}
	grRequireGroupsUnread(t, baseURL, valya, 3, "три приглашения")

	// Пригласившим их же приглашения не показываются.
	for _, who := range []dachnik{nikolay, galina, petya} {
		if items := grRequests(t, baseURL, who); len(items) != 0 {
			t.Errorf("пригласившему показано: %q", grRequestKeys(items))
		}
		grRequireGroupsUnread(t, baseURL, who, 0, "у пригласившего")
	}

	// Вступила, отклонила, отозвали — строки пропадают (требование 27).
	grJoin(t, baseURL, valya, fishing.ID, grMember)
	grRequireList(t, grRequestKeys(grRequests(t, baseURL, valya)), []string{
		grInviteKey(geo.ID, petya.id),
		grInviteKey(roses.ID, nikolay.id),
	}, "после принятия")
	grRequireGroupsUnread(t, baseURL, valya, 2, "после принятия")

	grLeave(t, baseURL, valya, geo.ID)
	grRemove(t, baseURL, nikolay, roses.ID, valya.id)
	grRequireList(t, grRequestKeys(grRequests(t, baseURL, valya)), nil, "после отказа и отзыва")
	grRequireGroupsUnread(t, baseURL, valya, 0, "после отказа и отзыва")
}

// Приглашения от тех, кто заблокировал смотрящего, не показываются.
func TestGroupRequestsHideBlockers(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	inviter := newGrUser(t, baseURL, 2)
	guest := newGrUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, owner, "Розы-блокировка")
	grJoin(t, baseURL, inviter, g.ID, grMember)
	grInvite(t, baseURL, inviter, g.ID, guest.id)
	other := grInterestGroup(t, baseURL, owner, "Клематисы-блокировка")
	grInvite(t, baseURL, owner, other.ID, guest.id)

	blockOK(t, baseURL, inviter, guest.id)

	grRequireList(t, grRequestKeys(grRequests(t, baseURL, guest)), []string{grInviteKey(other.ID, owner.id)},
		"приглашения после блокировки пригласившим")
}

// Приглашение пропадает, когда кто-то из двоих удалил аккаунт; хозяин
// удалил аккаунт — группа по интересам удаляется; геогруппа остаётся
// (требование 27, «Ограничения»).
func TestGroupAccountDeletion(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	inviter := newGrUser(t, baseURL, 3)
	guest := newGrUser(t, baseURL, 4)
	leaving := newGrUser(t, baseURL, 5)

	g := grInterestGroup(t, baseURL, owner, "Розы-аккаунт")
	grJoin(t, baseURL, member, g.ID, grMember)
	grJoin(t, baseURL, inviter, g.ID, grMember)
	grInvite(t, baseURL, inviter, g.ID, guest.id)
	grInvite(t, baseURL, owner, g.ID, leaving.id)
	grRequireGroupsUnread(t, baseURL, guest, 1, "до удаления")

	// Удалил аккаунт пригласивший — его приглашения пропадают.
	deleteMeOK(t, baseURL, inviter.token)
	grRequireList(t, grRequestKeys(grRequests(t, baseURL, guest)), nil, "приглашение от удалённого")
	grRequireGroupsUnread(t, baseURL, guest, 0, "приглашённый после удаления пригласившего")
	grRequireMembership(t, baseURL, guest, g.ID, grNone, "приглашённый после удаления пригласившего")
	grRequireMembers(t, baseURL, owner, g.ID, 2, "без удалённого участника")

	// Удалил аккаунт приглашённый — из «Приглашены» пропал.
	deleteMeOK(t, baseURL, leaving.token)
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, grInvited)), nil, "приглашённый удалил аккаунт")

	// Геогруппа хозяина остаётся после удаления его аккаунта.
	p := grLearnPlain(t, baseURL, fake, owner.token, "снт Аккаунт-гео")
	grSettle(t, baseURL, owner, p)
	grSettle(t, baseURL, member, p)
	geo := grGeoOf(t, baseURL, member, p)

	deleteMeOK(t, baseURL, owner.token)
	grRequireCode(t, grGetReq(t, baseURL, member.token, g.ID), http.StatusNotFound, "group_not_found", "группа удалённого хозяина")
	if grFind(grMine(t, baseURL, member), g.ID) != nil {
		t.Error("группа удалённого хозяина осталась в «Моих»")
	}
	grRequireIn(t, baseURL, member, geo.ID, true, grMember, "геогруппа после удаления аккаунта соседа")
	grRequireMembers(t, baseURL, member, geo.ID, 1, "геогруппа после удаления аккаунта соседа")
}

// Заявки на подписку и приглашения в группы считаются раздельно;
// открытие «Уведомлений» приглашения не гасит (требование 28).
func TestGroupUnreadSeparateFromFollowRequests(t *testing.T) {
	baseURL := startAPI(t)
	me := newGrUser(t, baseURL, 1)
	fan := newGrUser(t, baseURL, 2)

	setClosed(t, baseURL, me.token, true)
	followOK(t, baseURL, fan, me.id)

	g := grInterestGroup(t, baseURL, fan, "Клуб-счётчик")
	grInvite(t, baseURL, fan, g.ID, me.id)

	resp := fetchUnread(t, baseURL, me.token)
	grStatus(t, resp, http.StatusOK, "GET /me/notifications/unread")
	var body grUnread
	decode(t, resp, &body)
	if body.Requests == nil || *body.Requests != 1 {
		t.Errorf("requests = %v, ожидалась 1 заявка на подписку", body.Requests)
	}
	if body.Groups == nil || *body.Groups != 1 {
		t.Errorf("groups = %v, ожидалось 1 приглашение", body.Groups)
	}

	markSeenOK(t, baseURL, me)
	grRequireGroupsUnread(t, baseURL, me, 1, "после открытия раздела")
}

// ============================================================================
// Пуши (требование 29)
// ============================================================================

// Автовступление в геогруппу пушей не даёт; приглашение — пуш
// приглашённому: заголовок — ник пригласившего, текст «приглашает вас в
// группу «X»»; вступление в группу пушей не даёт.
func TestGroupPushes(t *testing.T) {
	fake := newFakeDaData(t)
	baseURL, fcm := startPushWith(t, grPlacesConfig(fake), 100*time.Millisecond)
	nikolay := newGrNamed(t, baseURL, 1, "nikolay_push")
	galina := newGrNamed(t, baseURL, 2, "galina_push")
	valya := newGrNamed(t, baseURL, 3, "valya_push")
	registerPhone(t, baseURL, nikolay.token, "nikolay-phone")
	registerPhone(t, baseURL, galina.token, "galina-phone")
	registerPhone(t, baseURL, valya.token, "valya-phone")

	p := grLearnPlain(t, baseURL, fake, galina.token, "снт Ромашка-пуш")
	grSettle(t, baseURL, galina, p)
	grSettle(t, baseURL, nikolay, p)
	geo := grGeoOf(t, baseURL, nikolay, p)

	quiet(100 * time.Millisecond)
	for _, phone := range []string{"nikolay-phone", "galina-phone"} {
		if got := fcm.sentTo(phone); len(got) != 0 {
			t.Errorf("автовступление дало пуши на %s: %s", phone, describePushes(got))
		}
	}

	// Николай — участник, не хозяин — зовёт Валю в геогруппу.
	grInvite(t, baseURL, nikolay, geo.ID, valya.id)
	got := fcm.waitSent("valya-phone", 1, "пуш о приглашении в геогруппу")
	if got[0].Title != "nikolay_push" || got[0].Body != "приглашает вас в группу «снт Ромашка-пуш»" {
		t.Errorf("пуш о приглашении: %q — %q", got[0].Title, got[0].Body)
	}

	// Участница группы по интересам зовёт Николая.
	fishing := grInterestGroup(t, baseURL, valya, "Любители рыбалки-пуш")
	grJoin(t, baseURL, galina, fishing.ID, grMember)
	grInvite(t, baseURL, galina, fishing.ID, nikolay.id)
	got = fcm.waitSent("nikolay-phone", 1, "пуш о приглашении в группу по интересам")
	if got[0].Title != "galina_push" || got[0].Body != "приглашает вас в группу «Любители рыбалки-пуш»" {
		t.Errorf("пуш о приглашении: %q — %q", got[0].Title, got[0].Body)
	}

	// Вступления пушей не дают.
	grJoin(t, baseURL, valya, geo.ID, grMember)
	grJoin(t, baseURL, nikolay, fishing.ID, grMember)
	quiet(100 * time.Millisecond)
	if n := len(fcm.sentTo("valya-phone")); n != 1 {
		t.Errorf("Вале пришло пушей %d, ожидался 1: %s", n, describePushes(fcm.sentTo("valya-phone")))
	}
	if n := len(fcm.sentTo("nikolay-phone")); n != 1 {
		t.Errorf("Николаю пришло пушей %d, ожидался 1: %s", n, describePushes(fcm.sentTo("nikolay-phone")))
	}
	if n := len(fcm.sentTo("galina-phone")); n != 0 {
		t.Errorf("Галине пришло пушей %d, ожидалось 0: %s", n, describePushes(fcm.sentTo("galina-phone")))
	}
}

// Приглашение отозвали, отклонили или приняли до отправки — пуша нет.
func TestGroupPushCancelledBeforeSend(t *testing.T) {
	baseURL, fcm := startPush(t, pushDelay)
	owner := newGrNamed(t, baseURL, 1, "nikolay_otmena")
	revoked := newGrNamed(t, baseURL, 2, "revoked_otmena")
	declined := newGrNamed(t, baseURL, 3, "declined_otmena")
	accepted := newGrNamed(t, baseURL, 4, "accepted_otmena")
	registerPhone(t, baseURL, revoked.token, "revoked-phone")
	registerPhone(t, baseURL, declined.token, "declined-phone")
	registerPhone(t, baseURL, accepted.token, "accepted-phone")

	g := grInterestGroup(t, baseURL, owner, "Розы-отмена")

	grInvite(t, baseURL, owner, g.ID, revoked.id)
	grRemove(t, baseURL, owner, g.ID, revoked.id)

	grInvite(t, baseURL, owner, g.ID, declined.id)
	grLeave(t, baseURL, declined, g.ID)

	grInvite(t, baseURL, owner, g.ID, accepted.id)
	grJoin(t, baseURL, accepted, g.ID, grMember)

	quiet(pushDelay)
	for _, phone := range []string{"revoked-phone", "declined-phone", "accepted-phone"} {
		if got := fcm.sentTo(phone); len(got) != 0 {
			t.Errorf("%s: пришли пуши об отменённом приглашении: %s", phone, describePushes(got))
		}
	}
}

// ============================================================================
// Дашборд (требования 30, 31)
// ============================================================================

// moderation.groups: геогруппы с участниками (id, ник, имя), больше
// участников — выше; групп по интересам там нет.
func TestDashboardGeoGroups(t *testing.T) {
	fake := newFakeDaData(t)
	baseURL, root := startDashboardAPI(t, grPlacesConfig(fake))

	a := newGrPerson(t, baseURL, 1, "galina_dash", "Галина")
	b := newGrPerson(t, baseURL, 2, "nikolay_dash", "Николай")
	c := newGrPerson(t, baseURL, 3, "andrey_dash", "Андрей")
	d := newGrPerson(t, baseURL, 4, "valya_dash", "Валентина")
	e := newGrPerson(t, baseURL, 5, "petya_dash", "Пётр")
	f := newGrPerson(t, baseURL, 6, "olga_dash", "Ольга")

	big := grLearnPlain(t, baseURL, fake, a.token, "снт Большое-дашборд")
	small := grLearnPlain(t, baseURL, fake, a.token, "снт Малое-дашборд")
	middle := grLearnPlain(t, baseURL, fake, a.token, "снт Среднее-дашборд")
	for _, who := range []dachnik{a, b, c} {
		grSettle(t, baseURL, who, big)
	}
	grSettle(t, baseURL, d, small)
	grSettle(t, baseURL, e, middle)
	grSettle(t, baseURL, f, middle)

	bigGroup := grGeoOf(t, baseURL, a, big)
	smallGroup := grGeoOf(t, baseURL, a, small)
	middleGroup := grGeoOf(t, baseURL, a, middle)
	hobby := grInterestGroup(t, baseURL, a, "Клуб-дашборд")
	grJoin(t, baseURL, b, hobby.ID, grMember)

	groups := grDashGroups(t, root)

	var ids []string
	for _, g := range groups {
		ids = append(ids, g.ID)
	}
	grRequireList(t, grOnly(ids, bigGroup.ID, middleGroup.ID, smallGroup.ID, hobby.ID),
		[]string{bigGroup.ID, middleGroup.ID, smallGroup.ID}, "геогруппы в дашборде")

	got := grDashFind(groups, bigGroup.ID)
	if got == nil {
		t.Fatal("в дашборде нет самой большой геогруппы")
	}
	if got.Name != big.Name || got.Area != big.Area {
		t.Errorf("геогруппа в дашборде: name = %q, area = %q; ожидалось %q, %q", got.Name, got.Area, big.Name, big.Area)
	}
	grRequireList(t, grDashMemberIDs(*got), sortedCopy([]string{a.id, b.id, c.id}), "участники в дашборде")
	for _, m := range got.Members {
		want := map[string][2]string{
			a.id: {"galina_dash", "Галина"},
			b.id: {"nikolay_dash", "Николай"},
			c.id: {"andrey_dash", "Андрей"},
		}[m.ID]
		if m.Nickname != want[0] || m.Name != want[1] {
			t.Errorf("участник %s в дашборде: @%s %q, ожидалось @%s %q", m.ID, m.Nickname, m.Name, want[0], want[1])
		}
	}
}

// DELETE /dashboard/groups/{groupId}/members/{userId}: убирает из
// геогруппы — 204; повтор и не геогруппа — 404; без пароля — 401.
// Убранный может вступить снова.
func TestDashboardRemoveFromGeoGroup(t *testing.T) {
	fake := newFakeDaData(t)
	baseURL, root := startDashboardAPI(t, grPlacesConfig(fake))

	a := newGrPerson(t, baseURL, 1, "galina_ubrat", "Галина")
	b := newGrPerson(t, baseURL, 2, "nikolay_ubrat", "Николай")
	owner := newGrUser(t, baseURL, 3)

	p := grLearnPlain(t, baseURL, fake, a.token, "снт Убрать-дашборд")
	grSettle(t, baseURL, a, p)
	grSettle(t, baseURL, b, p)
	geo := grGeoOf(t, baseURL, a, p)

	hobby := grInterestGroup(t, baseURL, owner, "Клуб-убрать")
	grJoin(t, baseURL, b, hobby.ID, grMember)

	path := grDashRemovePath(geo.ID, b.id)

	// Без верного пароля — 401, ничего не меняется.
	for _, c := range []struct {
		name           string
		user, password string
		bearer         string
	}{
		{name: "без заголовка"},
		{name: "неверный пароль", user: "владелец", password: "неверный-пароль"},
		{name: "токен приложения", bearer: a.token},
	} {
		if resp := modRequest(t, http.MethodDelete, root+path, c.user, c.password, c.bearer); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: ожидался статус 401, получен %d", c.name, resp.StatusCode)
		}
	}
	grRequireMembers(t, baseURL, a, geo.ID, 2, "после отвергнутых попыток")

	if resp := modDelete(t, root, path); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("убрать из геогруппы: ожидался статус 204, получен %d", resp.StatusCode)
	}
	grRequireIn(t, baseURL, b, geo.ID, false, grNone, "убранный владельцем сервиса")
	grRequireMembers(t, baseURL, a, geo.ID, 1, "после «Убрать»")
	if got := grDashFind(grDashGroups(t, root), geo.ID); got != nil {
		grRequireList(t, grDashMemberIDs(*got), []string{a.id}, "участники в дашборде после «Убрать»")
	}

	notFound := func(path, what string) {
		t.Helper()
		if resp := modDelete(t, root, path); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: ожидался статус 404, получен %d", what, resp.StatusCode)
		}
	}
	notFound(path, "повтор")
	notFound(grDashRemovePath(hobby.ID, b.id), "группа по интересам")
	notFound(grDashRemovePath(grNoGroup, b.id), "группы нет")
	notFound(grDashRemovePath(geo.ID, grNoUser), "человека нет")
	grRequireMembers(t, baseURL, owner, hobby.ID, 2, "группа по интересам не тронута")

	// Убранный вступает снова сам.
	grJoin(t, baseURL, b, geo.ID, grMember)
	grRequireMembers(t, baseURL, a, geo.ID, 2, "вернулся")
}
