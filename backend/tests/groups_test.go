package tests

// Тесты групп по интересам и по месту (specs/029-groups.md). Написаны по
// спецификации и контракту, не глядя в реализацию (ADR-0002).
//
// Место группы по месту — пункт из подсказок, поэтому тесты расстояния
// поднимают сервис с фейковым DaData (places_test.go) и узнают пункты
// с координатами хелперами 027 (post_place_test.go): все расстояния
// отложены на север от одной точки по одному меридиану.
//
// Пуши (требование 28) смотрятся через фейковый FCM и настоящую отправку
// из push_test.go.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Значения из контракта.
const (
	grInterest = "interest"
	grPlace    = "place"

	grOpen    = "open"
	grRequest = "request"
	grInvite  = "invite"

	grOwner     = "owner"
	grMember    = "member"
	grRequested = "requested"
	grInvited   = "invited"
	grNone      = "none"

	grKindRequest = "request"
	grKindInvite  = "invite"
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
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Kind        string        `json:"kind"`
	JoinPolicy  string        `json:"join_policy"`
	Owner       authorPayload `json:"owner"`
	Members     int           `json:"members"`
	Membership  string        `json:"membership"`
	CreatedAt   string        `json:"created_at"`
	Place       *placePayload `json:"place"`
	RadiusKm    *int          `json:"radius_km"`
	DistanceKm  *int          `json:"distance_km"`
	Near        bool          `json:"near"`
	Requests    *int          `json:"requests"`
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

// grParseGroup разбирает один объект Group и проверяет обязательные поля.
func grParseGroup(t *testing.T, raw json.RawMessage, where string) grGroup {
	t.Helper()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("%s: группа не разобралась как JSON-объект: %v (%s)", where, err, raw)
	}
	grRequireFields(t, fields, where,
		"id", "name", "description", "kind", "join_policy", "owner",
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

// --- Хелперы: группы -------------------------------------------------------

func grGroupsURL(baseURL string) string {
	return baseURL + "/groups"
}

func grGroupURL(baseURL, groupID string) string {
	return baseURL + "/groups/" + url.PathEscape(groupID)
}

// grDraft — тело создания группы без места.
func grDraft(name, kind, policy string) map[string]any {
	return map[string]any{"name": name, "kind": kind, "join_policy": policy}
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

// grInterestGroup создаёт группу по интересам.
func grInterestGroup(t *testing.T, baseURL string, who dachnik, name, policy string) grGroup {
	t.Helper()
	return grCreate(t, baseURL, who, grDraft(name, grInterest, policy))
}

// grPlaceGroup создаёт группу по месту; radius == 0 — без радиуса.
func grPlaceGroup(t *testing.T, baseURL string, who dachnik, name, policy, placeID string, radius int) grGroup {
	t.Helper()

	body := grDraft(name, grPlace, policy)
	body["place_id"] = placeID
	if radius != 0 {
		body["radius_km"] = radius
	}

	return grCreate(t, baseURL, who, body)
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

// grLeave выходит и требует 204.
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

func grAddReq(t *testing.T, baseURL, token, groupID, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, grMemberURL(baseURL, groupID, userID), token, nil)
}

// grAddOK — хозяин принимает заявку или приглашает; 200 и GroupMember
// с этим state.
func grAddOK(t *testing.T, baseURL string, owner dachnik, groupID, userID, wantState string) grMemberRow {
	t.Helper()

	where := "PUT /groups/{id}/members/{userId}"
	resp := grAddReq(t, baseURL, owner.token, groupID, userID)
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

// grAdd — то же, когда строка из ответа не нужна.
func grAdd(t *testing.T, baseURL string, owner dachnik, groupID, userID, wantState string) {
	t.Helper()
	grAddOK(t, baseURL, owner, groupID, userID, wantState)
}

func grRemoveReq(t *testing.T, baseURL, token, groupID, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, grMemberURL(baseURL, groupID, userID), token, nil)
}

// grRemove — хозяин отклоняет, отзывает или убирает; 204.
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
		out = append(out, r)
	}

	return out
}

// grRequestKeys — «вид:группа:человек» по строкам списка.
func grRequestKeys(items []grRequestItem) []string {
	out := make([]string, 0, len(items))
	for _, r := range items {
		out = append(out, r.Kind+":"+r.Group.ID+":"+r.User.ID)
	}
	return out
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

// grRequireCode — ошибка с этим статусом и кодом, без остановки теста.
func grRequireCode(t *testing.T, resp *http.Response, status int, code, where string) {
	t.Helper()
	requireCodeE(t, resp, status, code, where)
}

// grRequireHidden требует, чтобы группы для смотрящего не было: 404
// group_not_found на каждой ручке группы и нет её в списках (ФТ-13, ФТ-14).
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
	nf(grMembersReq(t, baseURL, viewer.token, groupID, grRequested), "GET members?state=requested")
	nf(grAddReq(t, baseURL, viewer.token, groupID, otherID), "PUT members/{userId}")
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

// grRequireOwnerRequests требует у хозяина requests = want.
func grRequireOwnerRequests(t *testing.T, baseURL string, owner dachnik, groupID string, want int, where string) {
	t.Helper()

	g := grGet(t, baseURL, owner, groupID)
	if g.Requests == nil {
		t.Errorf("%s: хозяину не пришло поле requests", where)
		return
	}
	if *g.Requests != want {
		t.Errorf("%s: requests = %d, ожидалось %d", where, *g.Requests, want)
	}
}

// grRawFields — сырое тело ответа как словарь полей (для проверки отсутствия полей).
func grRawFields(t *testing.T, resp *http.Response, where string) map[string]json.RawMessage {
	t.Helper()

	grStatus(t, resp, http.StatusOK, where)

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON(t, resp), &fields); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON-объект: %v", where, err)
	}

	return fields
}

// grAbsent — поля нет или оно null.
func grAbsent(fields map[string]json.RawMessage, name string) bool {
	raw, ok := fields[name]
	return !ok || string(raw) == "null"
}

// ============================================================================
// Доступ
// ============================================================================

// Без сессии ручки групп отвечают 401.
func TestGroupsRequireAuth(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	g := grInterestGroup(t, baseURL, owner, "Розы и клематисы", grOpen)

	for _, token := range []string{"", "не-токен-сессии"} {
		requireUnauthorized(t, grListReq(t, baseURL, token, nil), "GET /groups")
		requireUnauthorized(t, grCreateReq(t, baseURL, token, grDraft("Группа", grInterest, grOpen)), "POST /groups")
		requireUnauthorized(t, grGetReq(t, baseURL, token, g.ID), "GET /groups/{id}")
		requireUnauthorized(t, grDeleteReq(t, baseURL, token, g.ID), "DELETE /groups/{id}")
		requireUnauthorized(t, grJoinReq(t, baseURL, token, g.ID), "PUT membership")
		requireUnauthorized(t, grLeaveReq(t, baseURL, token, g.ID), "DELETE membership")
		requireUnauthorized(t, grMembersReq(t, baseURL, token, g.ID, ""), "GET members")
		requireUnauthorized(t, grAddReq(t, baseURL, token, g.ID, owner.id), "PUT members/{userId}")
		requireUnauthorized(t, grRemoveReq(t, baseURL, token, g.ID, owner.id), "DELETE members/{userId}")
		requireUnauthorized(t, grRequestsReq(t, baseURL, token), "GET /me/group-requests")
	}
}

// ============================================================================
// Создание (ФТ-1–6, ФТ-9)
// ============================================================================

// Группа по интересам: создатель — хозяин и первый участник, 201 Group;
// название и описание обрезаются по краям; места, радиуса и расстояния
// нет; хозяину — число заявок; та же группа по адресу и в «Моих».
func TestGroupCreateInterest(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrNamed(t, baseURL, 1, "valya_dacha")

	body := grDraft("  Любители рыбалки \t", grInterest, grOpen)
	body["description"] = "  Ловим на пруду за СНТ \n"
	resp := grCreateReq(t, baseURL, owner.token, body)
	grStatus(t, resp, http.StatusCreated, "создание")
	raw := rawJSON(t, resp)
	g := grParseGroup(t, raw, "создание")

	if g.Name != "Любители рыбалки" {
		t.Errorf("name = %q, ожидалось обрезанное «Любители рыбалки»", g.Name)
	}
	if g.Description != "Ловим на пруду за СНТ" {
		t.Errorf("description = %q, ожидалось обрезанное", g.Description)
	}
	if g.Kind != grInterest || g.JoinPolicy != grOpen {
		t.Errorf("kind = %q, join_policy = %q", g.Kind, g.JoinPolicy)
	}
	if g.Owner.ID != owner.id || g.Owner.Nickname != "valya_dacha" {
		t.Errorf("owner = %+v, ожидался хозяин %s valya_dacha", g.Owner, owner.id)
	}
	if g.Members != 1 {
		t.Errorf("members = %d, ожидался 1 — сам хозяин", g.Members)
	}
	if g.Membership != grOwner {
		t.Errorf("membership = %q, ожидался owner", g.Membership)
	}
	if g.Near {
		t.Error("near = true у группы по интересам")
	}
	if g.Requests == nil || *g.Requests != 0 {
		t.Errorf("requests у хозяина = %v, ожидался 0", g.Requests)
	}

	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	for _, name := range []string{"place", "radius_km", "distance_km"} {
		if !grAbsent(fields, name) {
			t.Errorf("у группы по интересам есть поле %s: %s", name, fields[name])
		}
	}

	got := grGet(t, baseURL, owner, g.ID)
	if got.Name != g.Name || got.Description != g.Description || got.CreatedAt != g.CreatedAt ||
		got.Membership != grOwner || got.Members != 1 {
		t.Errorf("GET отдаёт другую группу: %+v, создана %+v", got, g)
	}

	mine := grMine(t, baseURL, owner)
	grRequireList(t, grIDs(mine), []string{g.ID}, "«Мои» хозяина")

	// Без описания — пустая строка; названия не уникальны.
	twin := grInterestGroup(t, baseURL, owner, "Любители рыбалки", grRequest)
	if twin.Description != "" {
		t.Errorf("без описания description = %q, ожидалась пустая строка", twin.Description)
	}
	if twin.ID == g.ID {
		t.Error("вторая группа с тем же названием получила тот же id")
	}
}

// Группа по месту: место — пункт из подсказок, радиус необязателен, 1–100.
func TestGroupCreatePlace(t *testing.T) {
	baseURL, fake := startPlaces(t)
	owner := newGrUser(t, baseURL, 1)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, owner.token, "Андрейково")

	g := grPlaceGroup(t, baseURL, owner, "СНТ Андрейково", grRequest, sntAndreykovoID, 5)
	if g.Kind != grPlace || g.JoinPolicy != grRequest {
		t.Errorf("kind = %q, join_policy = %q", g.Kind, g.JoinPolicy)
	}
	if g.Place == nil || *g.Place != sntAndreykovo {
		t.Errorf("place = %+v, ожидался %+v", g.Place, sntAndreykovo)
	}
	if g.RadiusKm == nil || *g.RadiusKm != 5 {
		t.Errorf("radius_km = %v, ожидалось 5", g.RadiusKm)
	}
	if got := grGet(t, baseURL, owner, g.ID); got.Place == nil || *got.Place != sntAndreykovo ||
		got.RadiusKm == nil || *got.RadiusKm != 5 {
		t.Errorf("GET: place = %+v, radius_km = %v", got.Place, got.RadiusKm)
	}

	// Без радиуса — только сам пункт: поля нет.
	point := grPlaceGroup(t, baseURL, owner, "д Андрейково", grOpen, derAndreykovoID, 0)
	fields := grRawFields(t, grGetReq(t, baseURL, owner.token, point.ID), "GET группы без радиуса")
	if !grAbsent(fields, "radius_km") {
		t.Errorf("у группы без радиуса есть radius_km: %s", fields["radius_km"])
	}
	if point.Place == nil || *point.Place != derAndreykovo {
		t.Errorf("place = %+v, ожидался %+v", point.Place, derAndreykovo)
	}

	// Границы радиуса.
	for _, r := range []int{1, 100} {
		g := grPlaceGroup(t, baseURL, owner, fmt.Sprintf("Радиус %d", r), grOpen, sntAndreykovoID, r)
		if g.RadiusKm == nil || *g.RadiusKm != r {
			t.Errorf("radius_km = %v, ожидалось %d", g.RadiusKm, r)
		}
	}
}

// Ошибки создания: коды из таблицы, и группа не создаётся (ФТ-1, ФТ-4, ФТ-6).
func TestGroupCreateValidation(t *testing.T) {
	baseURL, fake := startPlaces(t)
	owner := newGrUser(t, baseURL, 1)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, owner.token, "Андрейково")

	with := func(base map[string]any, extra map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range base {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	interest := grDraft("Розы", grInterest, grOpen)
	place := with(grDraft("СНТ", grPlace, grOpen), map[string]any{"place_id": sntAndreykovoID})

	cases := []struct {
		name   string
		body   any
		status int
		code   string
	}{
		{"пустое название", with(interest, map[string]any{"name": ""}), 400, "invalid_group"},
		{"название из пробелов", with(interest, map[string]any{"name": "   \t "}), 400, "invalid_group"},
		{"название длиннее 60", with(interest, map[string]any{"name": strings.Repeat("я", 61)}), 400, "invalid_group"},
		{"описание длиннее 500", with(interest, map[string]any{"description": strings.Repeat("я", 501)}), 400, "invalid_group"},
		{"место у группы по интересам", with(interest, map[string]any{"place_id": sntAndreykovoID}), 400, "invalid_group"},
		{"радиус у группы по интересам", with(interest, map[string]any{"radius_km": 5}), 400, "invalid_group"},
		{"радиус 0", with(place, map[string]any{"radius_km": 0}), 400, "invalid_group"},
		{"радиус 101", with(place, map[string]any{"radius_km": 101}), 400, "invalid_group"},
		{"радиус отрицательный", with(place, map[string]any{"radius_km": -5}), 400, "invalid_group"},
		{"неизвестный тип", with(interest, map[string]any{"kind": "club"}), 400, "invalid_request"},
		{"неизвестное правило", with(interest, map[string]any{"join_policy": "anyone"}), 400, "invalid_request"},
		{"группа по месту без места", grDraft("СНТ", grPlace, grOpen), 400, "place_required"},
		{"группа по месту с радиусом, но без места", with(grDraft("СНТ", grPlace, grOpen), map[string]any{"radius_km": 5}), 400, "place_required"},
		{"место, которого сервер не отдавал", with(place, map[string]any{"place_id": "27290000-0000-4000-8000-000000000001"}), 400, "unknown_place"},
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

	// Границы длины — принимаются; длина считается в знаках, не байтах.
	ok := grCreate(t, baseURL, owner, with(interest, map[string]any{
		"name":        strings.Repeat("я", 60),
		"description": strings.Repeat("я", 500),
	}))
	if ok.Name != strings.Repeat("я", 60) || ok.Description != strings.Repeat("я", 500) {
		t.Error("название в 60 и описание в 500 знаков сохранились не так")
	}
	// Пробелы по краям в длину не входят.
	grCreate(t, baseURL, owner, with(interest, map[string]any{"name": "  " + strings.Repeat("ё", 60) + "  "}))
}

// Группы, которой нет, — 404 group_not_found на её ручках (ФТ-15).
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
	nf(grAddReq(t, baseURL, who.token, grNoGroup, other.id), "PUT members/{userId}")
	nf(grRemoveReq(t, baseURL, who.token, grNoGroup, other.id), "DELETE members/{userId}")
}

// ============================================================================
// Расстояние и «рядом» (ФТ-11, ФТ-12)
// ============================================================================

// distance_km округляется как у поста; near — тот же пункт или
// неокруглённое расстояние не больше радиуса. Без пункта у смотрящего,
// без координат и у группы по интересам — поля нет, near = false.
func TestGroupDistanceAndNear(t *testing.T) {
	baseURL, fake := startPlaces(t)
	owner := newGrUser(t, baseURL, 1)

	home := ppLearnNorth(t, baseURL, fake, owner.token, 0)
	noCoords := ppLearn(t, baseURL, fake, owner.token, ppNoField{}, ppNoField{})

	radius5 := grPlaceGroup(t, baseURL, owner, "Радиус пять", grOpen, home.ID, 5)
	point := grPlaceGroup(t, baseURL, owner, "Только пункт", grOpen, home.ID, 0)
	blind := grPlaceGroup(t, baseURL, owner, "Без координат", grOpen, noCoords.ID, 50)
	hobby := grInterestGroup(t, baseURL, owner, "По интересам", grOpen)

	viewerAt := func(n int, p *placePayload) dachnik {
		v := newGrUser(t, baseURL, n)
		if p != nil {
			setPlaceOK(t, baseURL, v.token, *p)
		}
		return v
	}
	at := func(n int, km float64) dachnik {
		v := newGrUser(t, baseURL, n)
		p := ppLearnNorth(t, baseURL, fake, v.token, km)
		setPlaceOK(t, baseURL, v.token, p)
		return v
	}

	same := viewerAt(2, &home)
	km3 := at(3, 3)
	km49 := at(4, 4.9)
	km52 := at(5, 5.2)
	km24 := at(6, 24)
	nowhere := viewerAt(7, nil)
	sameBlind := viewerAt(8, &noCoords)

	cases := []struct {
		who      string
		viewer   dachnik
		group    grGroup
		distance *int
		near     bool
	}{
		{"тот же пункт, радиус 5", same, radius5, ppInt(0), true},
		{"тот же пункт, без радиуса", same, point, ppInt(0), true},
		{"3 км, радиус 5", km3, radius5, ppInt(3), true},
		{"3 км, без радиуса", km3, point, ppInt(3), false},
		{"4,9 км, радиус 5", km49, radius5, ppInt(5), true},
		{"5,2 км, радиус 5: округлённое 5, но дальше радиуса", km52, radius5, ppInt(5), false},
		{"24 км, радиус 5", km24, radius5, ppInt(25), false},
		{"у смотрящего нет пункта", nowhere, radius5, nil, false},
		{"у места группы нет координат", same, blind, nil, false},
		{"тот же пункт без координат", sameBlind, blind, ppInt(0), true},
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

			// В «Найти» — то же самое.
			found := grFind(grAvailable(t, baseURL, c.viewer), c.group.ID)
			if found == nil {
				t.Fatalf("%s: группы нет в «Найти»", c.who)
			}
			if found.Near != c.near {
				t.Errorf("%s: в «Найти» near = %v, ожидалось %v", c.who, found.Near, c.near)
			}
			switch {
			case c.distance == nil && found.DistanceKm != nil:
				t.Errorf("%s: в «Найти» distance_km = %d, а его быть не должно", c.who, *found.DistanceKm)
			case c.distance != nil && (found.DistanceKm == nil || *found.DistanceKm != *c.distance):
				t.Errorf("%s: в «Найти» distance_km = %v, ожидалось %d", c.who, found.DistanceKm, *c.distance)
			}
		})
	}
}

// ============================================================================
// Видимость (ФТ-13, ФТ-14)
// ============================================================================

// Группу по приглашению видят хозяин, участники и приглашённые; для
// остальных её нет ни на одной ручке, в том числе на «Вступить».
func TestGroupInviteOnlyIsHiddenFromStrangers(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	guest := newGrUser(t, baseURL, 2)
	stranger := newGrUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, owner, "Закрытый клуб", grInvite)

	grRequireHidden(t, baseURL, stranger, g.ID, owner.id, "посторонний")
	grRequireHidden(t, baseURL, guest, g.ID, owner.id, "ещё не приглашённый")

	// Вступить без приглашения не вышло: в составе только хозяин.
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, "")), []string{owner.id}, "состав после попыток")
	if n := len(grMembers(t, baseURL, owner, g.ID, grRequested)); n != 0 {
		t.Errorf("после попыток вступить появилось заявок: %d", n)
	}

	grAdd(t, baseURL, owner, g.ID, guest.id, grInvited)

	if got := grGet(t, baseURL, guest, g.ID); got.Membership != grInvited {
		t.Errorf("приглашённый: membership = %q, ожидалось invited", got.Membership)
	}
	if found := grFind(grAvailable(t, baseURL, guest), g.ID); found == nil || found.Membership != grInvited {
		t.Errorf("приглашённому группа в «Найти»: %+v, ожидалась с membership invited", found)
	}
	grStatus(t, grMembersReq(t, baseURL, guest.token, g.ID, ""), http.StatusOK, "состав глазами приглашённого")
	grRequireHidden(t, baseURL, stranger, g.ID, owner.id, "посторонний после приглашения другого")

	grJoin(t, baseURL, guest, g.ID, grMember)
	if found := grFind(grMine(t, baseURL, guest), g.ID); found == nil {
		t.Error("вступившему по приглашению группы нет в «Моих»")
	}
	grRequireMembers(t, baseURL, guest, g.ID, 2, "после вступления по приглашению")

	// Вышел — снова не видит.
	grLeave(t, baseURL, guest, g.ID)
	grRequireHidden(t, baseURL, guest, g.ID, owner.id, "вышедший участник")
}

// Отклонённое приглашение — группы по приглашению снова нет.
func TestGroupInviteOnlyHiddenAfterDecline(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	guest := newGrUser(t, baseURL, 2)

	g := grInterestGroup(t, baseURL, owner, "Закрытый клуб", grInvite)
	grAdd(t, baseURL, owner, g.ID, guest.id, grInvited)
	grLeave(t, baseURL, guest, g.ID)

	grRequireHidden(t, baseURL, guest, g.ID, owner.id, "отклонивший приглашение")
}

// Хозяин заблокировал смотрящего — группы для него нет; заблокированный
// участник остаётся в составе, но группу не видит («Ограничения»), и
// хозяин может его убрать.
func TestGroupHiddenFromBlockedByOwner(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	stranger := newGrUser(t, baseURL, 2)
	member := newGrUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, owner, "СНТ Ромашка", grOpen)
	grJoin(t, baseURL, member, g.ID, grMember)

	blockOK(t, baseURL, owner, stranger.id)
	blockOK(t, baseURL, owner, member.id)

	grRequireHidden(t, baseURL, stranger, g.ID, owner.id, "заблокированный посторонний")
	grRequireHidden(t, baseURL, member, g.ID, owner.id, "заблокированный участник")

	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, "")), []string{owner.id, member.id},
		"заблокированный участник остаётся в составе")
	grRequireMembers(t, baseURL, owner, g.ID, 2, "число участников с заблокированным")

	grRemove(t, baseURL, owner, g.ID, member.id)
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, "")), []string{owner.id}, "после «Убрать»")
}

// ============================================================================
// Списки (ФТ-16, ФТ-17)
// ============================================================================

// mine — где хозяин или участник (по умолчанию); available — остальные
// видимые, с заявкой и приглашением тоже; неизвестные scope и kind — 400.
func TestGroupListScopes(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	own := grInterestGroup(t, baseURL, me, "Своя", grOpen)
	joined := grInterestGroup(t, baseURL, owner, "Вступил", grOpen)
	requested := grInterestGroup(t, baseURL, owner, "Попросился", grRequest)
	invited := grInterestGroup(t, baseURL, owner, "Позвали", grRequest)
	other := grInterestGroup(t, baseURL, owner, "Чужая", grOpen)

	grJoin(t, baseURL, me, joined.ID, grMember)
	grJoin(t, baseURL, me, requested.ID, grRequested)
	grAdd(t, baseURL, owner, invited.ID, me.id, grInvited)

	wantMine := map[string]string{own.ID: grOwner, joined.ID: grMember}
	wantAvailable := map[string]string{requested.ID: grRequested, invited.ID: grInvited, other.ID: grNone}

	check := func(items []grGroup, want map[string]string, where string) {
		t.Helper()
		if len(items) != len(want) {
			t.Errorf("%s: групп %d %q, ожидалось %d", where, len(items), grNames(items), len(want))
		}
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
	}

	check(grList(t, baseURL, me, nil), wantMine, "без scope")
	check(grMine(t, baseURL, me), wantMine, "scope=mine")
	check(grAvailable(t, baseURL, me), wantAvailable, "scope=available")

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
	baseURL, fake := startPlaces(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	fake.answer(andreykovo()...)
	suggest(t, baseURL, owner.token, "Андрейково")

	fishing := grInterestGroup(t, baseURL, owner, "Любители рыбалки", grOpen)
	roses := grCreate(t, baseURL, owner, map[string]any{
		"name": "Розы и клематисы", "kind": grInterest, "join_policy": grOpen,
		"description": "Обрезка, подкормка, укрытие на зиму",
	})
	snt := grCreate(t, baseURL, owner, map[string]any{
		"name": "СНТ Ромашка", "kind": grPlace, "join_policy": grOpen, "place_id": sntAndreykovoID,
		"description": "Вода, дороги, ВЗНОСЫ",
	})
	pond := grCreate(t, baseURL, owner, map[string]any{
		"name": "Пруд", "kind": grPlace, "join_policy": grOpen, "place_id": derAndreykovoID,
		"description": "Рыбный пруд у деревни",
	})

	ids := func(items []grGroup) []string { return sortedCopy(grIDs(items)) }
	cases := []struct {
		name   string
		params url.Values
		want   []string
	}{
		{"kind=interest", grParams("available", grInterest, ""), []string{fishing.ID, roses.ID}},
		{"kind=place", grParams("available", grPlace, ""), []string{snt.ID, pond.ID}},
		{"q в названии", grParams("available", "", "рыб"), []string{fishing.ID, pond.ID}},
		{"q в другом регистре", grParams("available", "", "РЫБ"), []string{fishing.ID, pond.ID}},
		{"q в описании", grParams("available", "", "взносы"), []string{snt.ID}},
		{"q с пробелами по краям", grParams("available", "", "  клематис "), []string{roses.ID}},
		{"q и kind", grParams("available", grInterest, "рыб"), []string{fishing.ID}},
		{"q из пробелов — как без него", grParams("available", "", "   "), []string{fishing.ID, roses.ID, snt.ID, pond.ID}},
		{"ничего не нашлось", grParams("available", "", "тюльпаны"), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			grRequireList(t, ids(grList(t, baseURL, me, c.params)), sortedCopy(c.want), c.name)
		})
	}

	// В «Моих» хозяина — те же фильтры.
	t.Run("mine", func(t *testing.T) {
		grRequireList(t, ids(grList(t, baseURL, owner, grParams("mine", grPlace, "рыб"))), []string{pond.ID}, "mine kind=place q=рыб")
		grRequireList(t, ids(grList(t, baseURL, owner, grParams("", grInterest, ""))), sortedCopy([]string{fishing.ID, roses.ID}), "kind=interest без scope")
	})
}

// «Мои» — по названию без учёта регистра.
func TestGroupListMineOrder(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	grInterestGroup(t, baseURL, me, "Бархатцы", grOpen)
	for _, name := range []string{"Яблони", "агрономы", "Bees", "apples"} {
		g := grInterestGroup(t, baseURL, owner, name, grOpen)
		grJoin(t, baseURL, me, g.ID, grMember)
	}

	grRequireList(t, grNames(grMine(t, baseURL, me)),
		[]string{"apples", "Bees", "агрономы", "Бархатцы", "Яблони"}, "порядок «Моих»")
}

// «Найти»: приглашения, потом рядом, потом больше участников, при
// равенстве — по названию.
func TestGroupListAvailableOrder(t *testing.T) {
	baseURL, fake := startPlaces(t)
	owner := newGrUser(t, baseURL, 1)
	helper1 := newGrUser(t, baseURL, 2)
	helper2 := newGrUser(t, baseURL, 3)
	me := newGrUser(t, baseURL, 4)

	home := ppLearnNorth(t, baseURL, fake, me.token, 0)
	setPlaceOK(t, baseURL, me.token, home)
	far := ppLearnNorth(t, baseURL, fake, owner.token, 24)

	withMembers := func(g grGroup, people ...dachnik) {
		for _, p := range people {
			grJoin(t, baseURL, p, g.ID, grMember)
		}
	}

	grInterestGroup(t, baseURL, owner, "Альфа", grOpen)
	beta := grInterestGroup(t, baseURL, owner, "Бета", grOpen)
	withMembers(beta, helper1, helper2)
	delta := grInterestGroup(t, baseURL, owner, "Дельта", grOpen)
	withMembers(delta, helper1, helper2)
	grPlaceGroup(t, baseURL, owner, "Вега", grOpen, home.ID, 0)
	lake := grPlaceGroup(t, baseURL, owner, "Озеро", grOpen, far.ID, 5)
	withMembers(lake, helper1)
	gamma := grInterestGroup(t, baseURL, owner, "Гамма", grRequest)
	grAdd(t, baseURL, owner, gamma.ID, me.id, grInvited)
	zhuk := grInterestGroup(t, baseURL, owner, "Жук", grRequest)
	grJoin(t, baseURL, me, zhuk.ID, grRequested)
	grInterestGroup(t, baseURL, me, "Своя", grOpen)

	grRequireList(t, grNames(grAvailable(t, baseURL, me)),
		[]string{"Гамма", "Вега", "Бета", "Дельта", "Озеро", "Альфа", "Жук"}, "порядок «Найти»")
}

// В списке не больше 100 групп.
func TestGroupListLimit(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	for i := 0; i < 101; i++ {
		grInterestGroup(t, baseURL, owner, fmt.Sprintf("Группа %03d", i), grOpen)
	}

	if n := len(grMine(t, baseURL, owner)); n != 100 {
		t.Errorf("«Мои»: групп %d, ожидалось 100", n)
	}
	if n := len(grAvailable(t, baseURL, me)); n != 100 {
		t.Errorf("«Найти»: групп %d, ожидалось 100", n)
	}
}

// ============================================================================
// Вступить и выйти (ФТ-18, ФТ-19, ФТ-24)
// ============================================================================

// Открытая — сразу участник; повтор ничего не меняет; хозяину — owner.
func TestGroupJoinOpen(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	g := grInterestGroup(t, baseURL, owner, "Любители рыбалки", grOpen)

	joined := grJoin(t, baseURL, me, g.ID, grMember)
	if joined.Members != 2 {
		t.Errorf("после вступления members = %d, ожидалось 2", joined.Members)
	}
	if joined.Requests != nil {
		t.Errorf("участнику пришло requests = %d — оно только хозяину", *joined.Requests)
	}

	again := grJoin(t, baseURL, me, g.ID, grMember)
	if again.Members != 2 {
		t.Errorf("повторное вступление: members = %d, ожидалось 2", again.Members)
	}

	if own := grJoin(t, baseURL, owner, g.ID, grOwner); own.Members != 2 {
		t.Errorf("«Вступить» хозяина: members = %d, ожидалось 2", own.Members)
	}
	grRequireMembers(t, baseURL, owner, g.ID, 2, "итог")
}

// По заявке — requested, хозяин видит заявку; повтор ничего не меняет.
func TestGroupJoinRequest(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	g := grInterestGroup(t, baseURL, owner, "СНТ Ромашка", grRequest)

	asked := grJoin(t, baseURL, me, g.ID, grRequested)
	if asked.Members != 1 {
		t.Errorf("заявка: members = %d, ожидалось 1 — заявитель ещё не участник", asked.Members)
	}
	grJoin(t, baseURL, me, g.ID, grRequested)

	grRequireOwnerRequests(t, baseURL, owner, g.ID, 1, "одна заявка")
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, grRequested)), []string{me.id}, "заявки")
	if grFind(grMine(t, baseURL, me), g.ID) != nil {
		t.Error("с заявкой группа уже в «Моих»")
	}

	grAdd(t, baseURL, owner, g.ID, me.id, grMember)
	grJoin(t, baseURL, me, g.ID, grMember)
	grRequireMembers(t, baseURL, me, g.ID, 2, "после принятия")
	grRequireOwnerRequests(t, baseURL, owner, g.ID, 0, "после принятия")
}

// Приглашённый становится участником сразу в группе любого правила.
func TestGroupJoinByInvite(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	me := newGrUser(t, baseURL, 2)

	for _, policy := range []string{grOpen, grRequest, grInvite} {
		t.Run(policy, func(t *testing.T) {
			g := grInterestGroup(t, baseURL, owner, "Группа "+policy, policy)
			grAdd(t, baseURL, owner, g.ID, me.id, grInvited)

			if got := grGet(t, baseURL, me, g.ID); got.Membership != grInvited {
				t.Errorf("до ответа membership = %q, ожидалось invited", got.Membership)
			}
			joined := grJoin(t, baseURL, me, g.ID, grMember)
			if joined.Members != 2 {
				t.Errorf("members = %d, ожидалось 2", joined.Members)
			}
			if n := len(grMembers(t, baseURL, owner, g.ID, grInvited)); n != 0 {
				t.Errorf("приглашение не ушло из «Приглашены»: %d", n)
			}
			if n := len(grMembers(t, baseURL, owner, g.ID, grRequested)); n != 0 {
				t.Errorf("вместо вступления появилась заявка: %d", n)
			}
		})
	}
}

// Выйти, отозвать заявку, отклонить приглашение — 204; без строки — тоже;
// хозяину — 409 owner_cannot_leave.
func TestGroupLeave(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	asker := newGrUser(t, baseURL, 3)
	guest := newGrUser(t, baseURL, 4)
	nobody := newGrUser(t, baseURL, 5)

	open := grInterestGroup(t, baseURL, owner, "Открытая", grOpen)
	byRequest := grInterestGroup(t, baseURL, owner, "По заявке", grRequest)

	grJoin(t, baseURL, member, open.ID, grMember)
	grLeave(t, baseURL, member, open.ID)
	if got := grGet(t, baseURL, member, open.ID); got.Membership != grNone || got.Members != 1 {
		t.Errorf("после выхода membership = %q, members = %d", got.Membership, got.Members)
	}
	if grFind(grMine(t, baseURL, member), open.ID) != nil {
		t.Error("после выхода группа осталась в «Моих»")
	}
	grLeave(t, baseURL, member, open.ID)
	grLeave(t, baseURL, nobody, open.ID)

	grJoin(t, baseURL, asker, byRequest.ID, grRequested)
	grLeave(t, baseURL, asker, byRequest.ID)
	if got := grGet(t, baseURL, asker, byRequest.ID); got.Membership != grNone {
		t.Errorf("после отзыва заявки membership = %q", got.Membership)
	}
	grRequireOwnerRequests(t, baseURL, owner, byRequest.ID, 0, "после отзыва заявки")

	grAdd(t, baseURL, owner, byRequest.ID, guest.id, grInvited)
	grLeave(t, baseURL, guest, byRequest.ID)
	if got := grGet(t, baseURL, guest, byRequest.ID); got.Membership != grNone {
		t.Errorf("после отказа от приглашения membership = %q", got.Membership)
	}
	if n := len(grMembers(t, baseURL, owner, byRequest.ID, grInvited)); n != 0 {
		t.Errorf("после отказа приглашений осталось: %d", n)
	}

	grRequireCode(t, grLeaveReq(t, baseURL, owner.token, open.ID), http.StatusConflict, "owner_cannot_leave", "хозяин выходит")
	grRequireMembers(t, baseURL, owner, open.ID, 1, "хозяин остался")
}

// ============================================================================
// Состав, заявки, приглашения (ФТ-20–24)
// ============================================================================

// Состав: хозяин первым, дальше новые выше; принятая заявка — вступление
// в момент принятия; role и state у каждой строки; видят все, кому видна
// группа.
func TestGroupMembersOrder(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrNamed(t, baseURL, 1, "nikolay_dacha")
	a := newGrUser(t, baseURL, 2)
	b := newGrUser(t, baseURL, 3)
	c := newGrUser(t, baseURL, 4)
	stranger := newGrUser(t, baseURL, 5)

	g := grInterestGroup(t, baseURL, owner, "СНТ Ромашка", grRequest)

	grJoin(t, baseURL, a, g.ID, grRequested)
	grJoin(t, baseURL, b, g.ID, grRequested)
	grJoin(t, baseURL, c, g.ID, grRequested)
	grAdd(t, baseURL, owner, g.ID, b.id, grMember)
	grAdd(t, baseURL, owner, g.ID, a.id, grMember)

	for _, viewer := range []dachnik{owner, a, stranger} {
		items := grMembers(t, baseURL, viewer, g.ID, "")
		grRequireList(t, grMemberIDs(items), []string{owner.id, a.id, b.id}, "состав")
		if len(items) != 3 {
			continue
		}
		if items[0].Role != grOwner || items[0].State != grMember || items[0].User.Nickname != "nikolay_dacha" {
			t.Errorf("строка хозяина: %+v", items[0])
		}
		for _, m := range items[1:] {
			if m.Role != grMember || m.State != grMember {
				t.Errorf("строка участника: role = %q, state = %q", m.Role, m.State)
			}
		}
		later, _ := time.Parse(time.RFC3339, items[1].CreatedAt)
		earlier, _ := time.Parse(time.RFC3339, items[2].CreatedAt)
		if later.Before(earlier) {
			t.Errorf("created_at — время принятия, принятый позже выше: %s и %s", items[1].CreatedAt, items[2].CreatedAt)
		}
	}

	grRequireMembers(t, baseURL, stranger, g.ID, 3, "число участников с хозяином")

	// Участники открытой группы, вступившие по очереди: новые выше.
	open := grInterestGroup(t, baseURL, owner, "Открытая", grOpen)
	for _, p := range []dachnik{a, b, c} {
		grJoin(t, baseURL, p, open.ID, grMember)
	}
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, stranger, open.ID, "")),
		[]string{owner.id, c.id, b.id, a.id}, "состав открытой группы")
}

// Люди, заблокировавшие смотрящего, в составе ему не показываются.
func TestGroupMembersHideThoseWhoBlockedViewer(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	shy := newGrUser(t, baseURL, 2)
	viewer := newGrUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, owner, "Розы", grOpen)
	grJoin(t, baseURL, shy, g.ID, grMember)
	grJoin(t, baseURL, viewer, g.ID, grMember)

	blockOK(t, baseURL, shy, viewer.id)

	grRequireList(t, grMemberIDs(grMembers(t, baseURL, viewer, g.ID, "")), []string{owner.id, viewer.id},
		"глазами заблокированного")
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, "")), []string{owner.id, viewer.id, shy.id},
		"глазами хозяина")
}

// Заявки и приглашения видит только хозяин, новые сверху; неизвестный
// state — 400.
func TestGroupPendingListsOwnerOnly(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	r1 := newGrUser(t, baseURL, 3)
	r2 := newGrUser(t, baseURL, 4)
	i1 := newGrUser(t, baseURL, 5)
	i2 := newGrUser(t, baseURL, 6)

	g := grInterestGroup(t, baseURL, owner, "СНТ Ромашка", grRequest)
	grAdd(t, baseURL, owner, g.ID, member.id, grInvited)
	grJoin(t, baseURL, member, g.ID, grMember)

	grJoin(t, baseURL, r1, g.ID, grRequested)
	grJoin(t, baseURL, r2, g.ID, grRequested)
	grAdd(t, baseURL, owner, g.ID, i1.id, grInvited)
	grAdd(t, baseURL, owner, g.ID, i2.id, grInvited)

	requests := grMembers(t, baseURL, owner, g.ID, grRequested)
	grRequireList(t, grMemberIDs(requests), []string{r2.id, r1.id}, "заявки")
	for _, m := range requests {
		if m.State != grRequested || m.Role != grMember {
			t.Errorf("заявка: state = %q, role = %q", m.State, m.Role)
		}
	}
	invites := grMembers(t, baseURL, owner, g.ID, grInvited)
	grRequireList(t, grMemberIDs(invites), []string{i2.id, i1.id}, "приглашения")
	for _, m := range invites {
		if m.State != grInvited {
			t.Errorf("приглашение: state = %q", m.State)
		}
	}

	// В составе — только участники.
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, g.ID, "")), []string{owner.id, member.id}, "состав")
	grRequireOwnerRequests(t, baseURL, owner, g.ID, 2, "requests")

	for _, who := range []dachnik{member, r1, i1} {
		for _, state := range []string{grRequested, grInvited} {
			grRequireCode(t, grMembersReq(t, baseURL, who.token, g.ID, state), http.StatusForbidden, "not_group_owner",
				"state="+state+" не хозяину")
		}
	}
	grRequireCode(t, grMembersReq(t, baseURL, owner.token, g.ID, "banned"), http.StatusBadRequest, "invalid_request",
		"неизвестный state")
}

// PUT members/{userId}: заявка → участник, ничего → приглашение; повтор
// ничего не меняет; ответ — GroupMember.
func TestGroupAddMember(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	asker := newGrNamed(t, baseURL, 2, "galina_dacha")
	guest := newGrUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, owner, "СНТ Ромашка", grRequest)
	grJoin(t, baseURL, asker, g.ID, grRequested)

	accepted := grAddOK(t, baseURL, owner, g.ID, asker.id, grMember)
	if accepted.User.ID != asker.id || accepted.User.Nickname != "galina_dacha" || accepted.Role != grMember {
		t.Errorf("принятая заявка: %+v", accepted)
	}
	if got := grGet(t, baseURL, asker, g.ID); got.Membership != grMember {
		t.Errorf("после принятия membership = %q", got.Membership)
	}
	again := grAddOK(t, baseURL, owner, g.ID, asker.id, grMember)
	if again.CreatedAt != accepted.CreatedAt {
		t.Errorf("повтор для участника изменил created_at: %s → %s", accepted.CreatedAt, again.CreatedAt)
	}

	invited := grAddOK(t, baseURL, owner, g.ID, guest.id, grInvited)
	if invited.User.ID != guest.id || invited.Role != grMember {
		t.Errorf("приглашение: %+v", invited)
	}
	if got := grGet(t, baseURL, guest, g.ID); got.Membership != grInvited {
		t.Errorf("приглашённый: membership = %q", got.Membership)
	}
	againInvite := grAddOK(t, baseURL, owner, g.ID, guest.id, grInvited)
	if againInvite.CreatedAt != invited.CreatedAt {
		t.Errorf("повтор для приглашённого изменил created_at: %s → %s", invited.CreatedAt, againInvite.CreatedAt)
	}
	grRequireMembers(t, baseURL, owner, g.ID, 2, "приглашённый — ещё не участник")
}

// Ошибки PUT members/{userId} (ФТ-22).
func TestGroupAddMemberErrors(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	target := newGrUser(t, baseURL, 3)
	blocker := newGrUser(t, baseURL, 4)
	blocked := newGrUser(t, baseURL, 5)

	g := grInterestGroup(t, baseURL, owner, "СНТ Ромашка", grOpen)
	grJoin(t, baseURL, member, g.ID, grMember)

	grRequireCode(t, grAddReq(t, baseURL, member.token, g.ID, target.id), http.StatusForbidden, "not_group_owner", "участник приглашает")
	grRequireCode(t, grAddReq(t, baseURL, target.token, g.ID, member.id), http.StatusForbidden, "not_group_owner", "посторонний")
	grRequireCode(t, grAddReq(t, baseURL, owner.token, g.ID, owner.id), http.StatusBadRequest, "cannot_invite_self", "хозяин себя")
	grRequireCode(t, grAddReq(t, baseURL, owner.token, g.ID, grNoUser), http.StatusNotFound, "user_not_found", "человека нет")

	blockOK(t, baseURL, blocker, owner.id)
	grRequireCode(t, grAddReq(t, baseURL, owner.token, g.ID, blocker.id), http.StatusNotFound, "user_not_found",
		"человек заблокировал хозяина")

	blockOK(t, baseURL, owner, blocked.id)
	grRequireCode(t, grAddReq(t, baseURL, owner.token, g.ID, blocked.id), http.StatusConflict, "user_blocked",
		"хозяин заблокировал человека")

	if n := len(grMembers(t, baseURL, owner, g.ID, grInvited)); n != 0 {
		t.Errorf("после отказов появились приглашения: %d", n)
	}
	if got := grGet(t, baseURL, target, g.ID); got.Membership != grNone {
		t.Errorf("после отказа «участник приглашает» membership = %q", got.Membership)
	}
}

// DELETE members/{userId}: отклонить заявку, отозвать приглашение, убрать
// участника; без строки — 204; не хозяин — 403; себя — 409. Убранный и
// отклонённый могут вступить и попроситься снова (ФТ-23, ФТ-24).
func TestGroupRemoveMember(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	asker := newGrUser(t, baseURL, 3)
	guest := newGrUser(t, baseURL, 4)
	nobody := newGrUser(t, baseURL, 5)

	open := grInterestGroup(t, baseURL, owner, "Открытая", grOpen)
	byRequest := grInterestGroup(t, baseURL, owner, "По заявке", grRequest)

	grJoin(t, baseURL, member, open.ID, grMember)
	grJoin(t, baseURL, member, byRequest.ID, grRequested)
	grAdd(t, baseURL, owner, byRequest.ID, member.id, grMember)
	grJoin(t, baseURL, asker, byRequest.ID, grRequested)
	grAdd(t, baseURL, owner, byRequest.ID, guest.id, grInvited)

	grRequireCode(t, grRemoveReq(t, baseURL, member.token, byRequest.ID, asker.id), http.StatusForbidden, "not_group_owner", "участник убирает")
	grRequireCode(t, grRemoveReq(t, baseURL, nobody.token, open.ID, member.id), http.StatusForbidden, "not_group_owner", "посторонний убирает")
	grRequireCode(t, grRemoveReq(t, baseURL, owner.token, open.ID, owner.id), http.StatusConflict, "owner_cannot_leave", "хозяин убирает себя")

	// Отклонить заявку.
	grRemove(t, baseURL, owner, byRequest.ID, asker.id)
	if got := grGet(t, baseURL, asker, byRequest.ID); got.Membership != grNone {
		t.Errorf("отклонённый заявитель: membership = %q", got.Membership)
	}
	grRequireOwnerRequests(t, baseURL, owner, byRequest.ID, 0, "после отклонения")

	// Отозвать приглашение.
	grRemove(t, baseURL, owner, byRequest.ID, guest.id)
	if got := grGet(t, baseURL, guest, byRequest.ID); got.Membership != grNone {
		t.Errorf("приглашение отозвано: membership = %q", got.Membership)
	}

	// Убрать участника.
	grRemove(t, baseURL, owner, open.ID, member.id)
	if got := grGet(t, baseURL, member, open.ID); got.Membership != grNone || got.Members != 1 {
		t.Errorf("убранный участник: membership = %q, members = %d", got.Membership, got.Members)
	}
	grRemove(t, baseURL, owner, byRequest.ID, member.id)
	grRequireMembers(t, baseURL, owner, byRequest.ID, 1, "после «Убрать» в группе по заявке")

	// Без строки — не ошибка.
	grRemove(t, baseURL, owner, open.ID, nobody.id)
	grRemove(t, baseURL, owner, open.ID, member.id)

	// Снова.
	grJoin(t, baseURL, member, open.ID, grMember)
	grJoin(t, baseURL, asker, byRequest.ID, grRequested)
	grJoin(t, baseURL, member, byRequest.ID, grRequested)
	grRequireOwnerRequests(t, baseURL, owner, byRequest.ID, 2, "попросились снова")
}

// ============================================================================
// «Группы» в «Уведомлениях» (ФТ-25–27)
// ============================================================================

// Хозяину — заявки в его группы, приглашённому — приглашения; новые сверху;
// у строки группа и человек.
func TestGroupRequestsList(t *testing.T) {
	baseURL := startAPI(t)
	nikolay := newGrNamed(t, baseURL, 1, "nikolay_dacha")
	galina := newGrNamed(t, baseURL, 2, "galina_dacha")
	valya := newGrNamed(t, baseURL, 3, "valya_dacha")
	petya := newGrUser(t, baseURL, 4)

	snt := grInterestGroup(t, baseURL, nikolay, "СНТ Ромашка", grRequest)
	roses := grInterestGroup(t, baseURL, nikolay, "Розы", grRequest)
	open := grInterestGroup(t, baseURL, nikolay, "Открытая", grOpen)

	if items := grRequests(t, baseURL, nikolay); len(items) != 0 {
		t.Fatalf("до заявок список не пуст: %q", grRequestKeys(items))
	}
	grRequireGroupsUnread(t, baseURL, nikolay, 0, "до заявок")

	grJoin(t, baseURL, galina, snt.ID, grRequested)
	grJoin(t, baseURL, petya, roses.ID, grRequested)
	grJoin(t, baseURL, petya, open.ID, grMember) // вступление в открытую — не заявка
	grAdd(t, baseURL, nikolay, snt.ID, valya.id, grInvited)
	grAdd(t, baseURL, nikolay, roses.ID, valya.id, grInvited)

	owner := grRequests(t, baseURL, nikolay)
	grRequireList(t, grRequestKeys(owner), []string{
		grKindRequest + ":" + roses.ID + ":" + petya.id,
		grKindRequest + ":" + snt.ID + ":" + galina.id,
	}, "заявки хозяину")
	if len(owner) == 2 {
		r := owner[1]
		if r.Group.Name != "СНТ Ромашка" || r.User.Nickname != "galina_dacha" {
			t.Errorf("строка заявки: группа %+v, человек %+v", r.Group, r.User)
		}
		if _, err := time.Parse(time.RFC3339, r.CreatedAt); err != nil {
			t.Errorf("created_at %q — не дата-время", r.CreatedAt)
		}
	}
	grRequireGroupsUnread(t, baseURL, nikolay, 2, "две заявки")

	guest := grRequests(t, baseURL, valya)
	grRequireList(t, grRequestKeys(guest), []string{
		grKindInvite + ":" + roses.ID + ":" + nikolay.id,
		grKindInvite + ":" + snt.ID + ":" + nikolay.id,
	}, "приглашения Валентине")
	if len(guest) == 2 && (guest[1].Group.Name != "СНТ Ромашка" || guest[1].User.Nickname != "nikolay_dacha") {
		t.Errorf("строка приглашения: группа %+v, человек %+v", guest[1].Group, guest[1].User)
	}
	grRequireGroupsUnread(t, baseURL, valya, 2, "два приглашения")

	// Заявителю и приглашающему их же заявки и приглашения не показываются.
	if items := grRequests(t, baseURL, galina); len(items) != 0 {
		t.Errorf("заявителю показано: %q", grRequestKeys(items))
	}
	grRequireGroupsUnread(t, baseURL, galina, 0, "у заявителя")

	// Принята, отклонена, отозвана, отказ — строки пропадают (ФТ-26).
	grAdd(t, baseURL, nikolay, snt.ID, galina.id, grMember)
	grRemove(t, baseURL, nikolay, roses.ID, petya.id)
	grRequireList(t, grRequestKeys(grRequests(t, baseURL, nikolay)), nil, "заявки после ответа")
	grRequireGroupsUnread(t, baseURL, nikolay, 0, "после ответа")

	grLeave(t, baseURL, valya, snt.ID)
	grRemove(t, baseURL, nikolay, roses.ID, valya.id)
	grRequireList(t, grRequestKeys(grRequests(t, baseURL, valya)), nil, "приглашения после ответа")
	grRequireGroupsUnread(t, baseURL, valya, 0, "после ответа")

	// Отозванная заявка и принятое приглашение — тоже.
	grJoin(t, baseURL, petya, roses.ID, grRequested)
	grAdd(t, baseURL, nikolay, snt.ID, valya.id, grInvited)
	grRequireGroupsUnread(t, baseURL, nikolay, 1, "заявка снова")
	grRequireGroupsUnread(t, baseURL, valya, 1, "приглашение снова")
	grLeave(t, baseURL, petya, roses.ID)
	grJoin(t, baseURL, valya, snt.ID, grMember)
	grRequireList(t, grRequestKeys(grRequests(t, baseURL, nikolay)), nil, "после отзыва заявки")
	grRequireList(t, grRequestKeys(grRequests(t, baseURL, valya)), nil, "после принятия приглашения")
	grRequireGroupsUnread(t, baseURL, nikolay, 0, "после отзыва заявки")
	grRequireGroupsUnread(t, baseURL, valya, 0, "после принятия приглашения")
}

// Заявки на подписку и заявки в группы считаются раздельно (ФТ-27).
func TestGroupUnreadSeparateFromFollowRequests(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	fan := newGrUser(t, baseURL, 2)

	setClosed(t, baseURL, owner.token, true)
	followOK(t, baseURL, fan, owner.id)

	g := grInterestGroup(t, baseURL, owner, "СНТ Ромашка", grRequest)
	grJoin(t, baseURL, fan, g.ID, grRequested)

	resp := fetchUnread(t, baseURL, owner.token)
	grStatus(t, resp, http.StatusOK, "GET /me/notifications/unread")
	var body grUnread
	decode(t, resp, &body)
	if body.Requests == nil || *body.Requests != 1 {
		t.Errorf("requests = %v, ожидалась 1 заявка на подписку", body.Requests)
	}
	if body.Groups == nil || *body.Groups != 1 {
		t.Errorf("groups = %v, ожидалась 1 заявка в группу", body.Groups)
	}

	// Открытие раздела «Уведомления» заявки в группы не гасит.
	markSeenOK(t, baseURL, owner)
	grRequireGroupsUnread(t, baseURL, owner, 1, "после открытия раздела")
}

// ============================================================================
// Удаление группы и аккаунта (ФТ-7, ФТ-26, «Ограничения»)
// ============================================================================

// Удалить может только хозяин; группа уходит со всем составом, заявками
// и приглашениями.
func TestGroupDelete(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	asker := newGrUser(t, baseURL, 3)
	guest := newGrUser(t, baseURL, 4)
	stranger := newGrUser(t, baseURL, 5)

	g := grInterestGroup(t, baseURL, owner, "СНТ Ромашка", grRequest)
	grAdd(t, baseURL, owner, g.ID, member.id, grInvited)
	grJoin(t, baseURL, member, g.ID, grMember)
	grJoin(t, baseURL, asker, g.ID, grRequested)
	grAdd(t, baseURL, owner, g.ID, guest.id, grInvited)

	grRequireCode(t, grDeleteReq(t, baseURL, member.token, g.ID), http.StatusForbidden, "not_group_owner", "участник удаляет")
	grRequireCode(t, grDeleteReq(t, baseURL, stranger.token, g.ID), http.StatusForbidden, "not_group_owner", "посторонний удаляет")
	grRequireMembers(t, baseURL, owner, g.ID, 2, "группа цела после отказов")

	requireEmpty204(t, grDeleteReq(t, baseURL, owner.token, g.ID), "хозяин удаляет")

	for _, who := range []dachnik{owner, member, asker, guest, stranger} {
		grRequireCode(t, grGetReq(t, baseURL, who.token, g.ID), http.StatusNotFound, "group_not_found", "GET удалённой")
		if grFind(grMine(t, baseURL, who), g.ID) != nil || grFind(grAvailable(t, baseURL, who), g.ID) != nil {
			t.Errorf("удалённая группа осталась в списках")
		}
	}
	grRequireCode(t, grMembersReq(t, baseURL, owner.token, g.ID, ""), http.StatusNotFound, "group_not_found", "состав удалённой")
	grRequireCode(t, grJoinReq(t, baseURL, asker.token, g.ID), http.StatusNotFound, "group_not_found", "вступить в удалённую")
	grRequireCode(t, grDeleteReq(t, baseURL, owner.token, g.ID), http.StatusNotFound, "group_not_found", "удалить ещё раз")

	grRequireList(t, grRequestKeys(grRequests(t, baseURL, owner)), nil, "заявки хозяину после удаления")
	grRequireList(t, grRequestKeys(grRequests(t, baseURL, guest)), nil, "приглашение после удаления")
	grRequireGroupsUnread(t, baseURL, owner, 0, "хозяин после удаления")
	grRequireGroupsUnread(t, baseURL, guest, 0, "приглашённый после удаления")
}

// Хозяин удалил аккаунт — его группы удаляются со всем составом.
func TestGroupOwnerAccountDeletion(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	guest := newGrUser(t, baseURL, 3)
	other := newGrUser(t, baseURL, 4)

	g := grInterestGroup(t, baseURL, owner, "СНТ Ромашка", grOpen)
	closed := grInterestGroup(t, baseURL, owner, "Клуб", grInvite)
	grJoin(t, baseURL, member, g.ID, grMember)
	grAdd(t, baseURL, owner, closed.ID, guest.id, grInvited)
	survivor := grInterestGroup(t, baseURL, other, "Розы", grOpen)
	grJoin(t, baseURL, member, survivor.ID, grMember)

	deleteMeOK(t, baseURL, owner.token)

	for _, id := range []string{g.ID, closed.ID} {
		grRequireCode(t, grGetReq(t, baseURL, member.token, id), http.StatusNotFound, "group_not_found", "группа удалённого хозяина")
	}
	grRequireList(t, grIDs(grMine(t, baseURL, member)), []string{survivor.ID}, "«Мои» участника")
	grRequireList(t, grIDs(grAvailable(t, baseURL, guest)), []string{survivor.ID}, "«Найти» приглашённого")
	grRequireList(t, grRequestKeys(grRequests(t, baseURL, guest)), nil, "приглашение от удалённого хозяина")
	grRequireGroupsUnread(t, baseURL, guest, 0, "приглашённый")
}

// Участник или заявитель удалил аккаунт — пропадает из состава, число
// участников уменьшается, заявка пропадает у хозяина; приглашённый удалил
// аккаунт — приглашение пропадает (ФТ-26).
func TestGroupMemberAccountDeletion(t *testing.T) {
	baseURL := startAPI(t)
	owner := newGrUser(t, baseURL, 1)
	member := newGrUser(t, baseURL, 2)
	asker := newGrUser(t, baseURL, 3)
	guest := newGrUser(t, baseURL, 4)

	open := grInterestGroup(t, baseURL, owner, "Открытая", grOpen)
	byRequest := grInterestGroup(t, baseURL, owner, "По заявке", grRequest)
	grJoin(t, baseURL, member, open.ID, grMember)
	grJoin(t, baseURL, asker, byRequest.ID, grRequested)
	grAdd(t, baseURL, owner, byRequest.ID, guest.id, grInvited)
	grRequireGroupsUnread(t, baseURL, owner, 1, "до удалений")

	deleteMeOK(t, baseURL, member.token)
	deleteMeOK(t, baseURL, asker.token)
	deleteMeOK(t, baseURL, guest.token)

	grRequireMembers(t, baseURL, owner, open.ID, 1, "после удаления участника")
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, open.ID, "")), []string{owner.id}, "состав")
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, byRequest.ID, grRequested)), nil, "заявки")
	grRequireList(t, grMemberIDs(grMembers(t, baseURL, owner, byRequest.ID, grInvited)), nil, "приглашения")
	grRequireOwnerRequests(t, baseURL, owner, byRequest.ID, 0, "requests")
	grRequireList(t, grRequestKeys(grRequests(t, baseURL, owner)), nil, "«Группы» хозяина")
	grRequireGroupsUnread(t, baseURL, owner, 0, "после удалений")
}

// ============================================================================
// Пуши (ФТ-28)
// ============================================================================

// Новая заявка — хозяину «просится в группу «X»», новое приглашение —
// приглашённому «приглашает вас в группу «X»»; заголовок — ник.
func TestGroupPushes(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	nikolay := newGrNamed(t, baseURL, 1, "nikolay_dacha")
	galina := newGrNamed(t, baseURL, 2, "galina_dacha")
	valya := newGrNamed(t, baseURL, 3, "valya_dacha")
	registerPhone(t, baseURL, nikolay.token, "nikolay-phone")
	registerPhone(t, baseURL, valya.token, "valya-phone")
	registerPhone(t, baseURL, galina.token, "galina-phone")

	g := grInterestGroup(t, baseURL, nikolay, "СНТ Ромашка", grRequest)

	grJoin(t, baseURL, galina, g.ID, grRequested)
	got := fake.waitSent("nikolay-phone", 1, "пуш о заявке")
	if got[0].Title != "galina_dacha" || got[0].Body != "просится в группу «СНТ Ромашка»" {
		t.Errorf("пуш о заявке: %q — %q", got[0].Title, got[0].Body)
	}

	grAdd(t, baseURL, nikolay, g.ID, valya.id, grInvited)
	got = fake.waitSent("valya-phone", 1, "пуш о приглашении")
	if got[0].Title != "nikolay_dacha" || got[0].Body != "приглашает вас в группу «СНТ Ромашка»" {
		t.Errorf("пуш о приглашении: %q — %q", got[0].Title, got[0].Body)
	}

	// Вступление в открытую группу и принятие заявки пушей не дают
	// (событие «вас приняли» — вне рамок).
	open := grInterestGroup(t, baseURL, valya, "Любители рыбалки", grOpen)
	grJoin(t, baseURL, galina, open.ID, grMember)
	grAdd(t, baseURL, nikolay, g.ID, galina.id, grMember)
	quiet(100 * time.Millisecond)
	if n := len(fake.sentTo("nikolay-phone")); n != 1 {
		t.Errorf("хозяину пришло пушей %d, ожидался 1: %s", n, describePushes(fake.sentTo("nikolay-phone")))
	}
	if n := len(fake.sentTo("valya-phone")); n != 1 {
		t.Errorf("Валентине пришло пушей %d, ожидался 1: %s", n, describePushes(fake.sentTo("valya-phone")))
	}
	if n := len(fake.sentTo("galina-phone")); n != 0 {
		t.Errorf("Галине пришло пушей %d, ожидалось 0: %s", n, describePushes(fake.sentTo("galina-phone")))
	}
}

// Заявку отозвали или отклонили, приглашение отозвали или отклонили до
// отправки — пуша нет.
func TestGroupPushCancelledBeforeSend(t *testing.T) {
	baseURL, fake := startPush(t, pushDelay)
	owner := newGrNamed(t, baseURL, 1, "nikolay_dacha")
	asker := newGrNamed(t, baseURL, 2, "galina_dacha")
	guest := newGrNamed(t, baseURL, 3, "valya_dacha")
	registerPhone(t, baseURL, owner.token, "owner-phone")
	registerPhone(t, baseURL, guest.token, "guest-phone")

	g := grInterestGroup(t, baseURL, owner, "СНТ Ромашка", grRequest)

	grJoin(t, baseURL, asker, g.ID, grRequested)
	grLeave(t, baseURL, asker, g.ID)
	grJoin(t, baseURL, asker, g.ID, grRequested)
	grRemove(t, baseURL, owner, g.ID, asker.id)

	grAdd(t, baseURL, owner, g.ID, guest.id, grInvited)
	grRemove(t, baseURL, owner, g.ID, guest.id)
	grAdd(t, baseURL, owner, g.ID, guest.id, grInvited)
	grLeave(t, baseURL, guest, g.ID)

	quiet(pushDelay)
	if got := fake.sentTo("owner-phone"); len(got) != 0 {
		t.Errorf("хозяину пришли пуши об отменённых заявках: %s", describePushes(got))
	}
	if got := fake.sentTo("guest-phone"); len(got) != 0 {
		t.Errorf("приглашённому пришли пуши об отменённых приглашениях: %s", describePushes(got))
	}
}
