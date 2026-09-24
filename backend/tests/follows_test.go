package tests

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"testing"
)

// Тесты подписок и закрытых профилей (specs/012-follows.md). Написаны по
// спецификации и контракту, не глядя в реализацию (ADR-0002).

// Значения Relation.following из контракта.
const (
	followingNone      = "none"
	followingRequested = "requested"
	followingYes       = "yes"
)

// --- Представления из контракта -------------------------------------------

// relationPayload — отношение смотрящего к пользователю (schema Relation).
// followed_by — указатель: его отсутствие в ответе — ошибка, а не false.
type relationPayload struct {
	Following  string `json:"following"`
	FollowedBy *bool  `json:"followed_by"`
}

// followProfilePayload — профиль пользователя (schema UserProfile) с полями,
// которые добавляет эта фича. Числа — указатели: поле обязано прийти.
type followProfilePayload struct {
	ID        string           `json:"id"`
	Nickname  string           `json:"nickname"`
	Name      string           `json:"name"`
	About     string           `json:"about"`
	CreatedAt string           `json:"created_at"`
	Posts     *int             `json:"posts"`
	Followers *int             `json:"followers"`
	Following *int             `json:"following"`
	Closed    *bool            `json:"closed"`
	Relation  *relationPayload `json:"relation"`
}

// followUserPayload — человек в списке подписчиков или подписок
// (schema FollowUser).
type followUserPayload struct {
	ID       string           `json:"id"`
	Nickname string           `json:"nickname"`
	Name     string           `json:"name"`
	Relation *relationPayload `json:"relation"`
}

// followListPayload — страница подписчиков или подписок (schema FollowList).
type followListPayload struct {
	Items      []followUserPayload `json:"items"`
	NextCursor *string             `json:"next_cursor"`
}

// authorListPayload — страница заявок (schema AuthorList).
type authorListPayload struct {
	Items      []authorPayload `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}

// dachnik — вошедший пользователь: токен и идентификатор.
type dachnik struct {
	token string
	id    string
}

// --- Хелперы --------------------------------------------------------------

// dachnikPhone — номер n-го участника теста. Номера не пересекаются
// с номерами других тестов пакета.
func dachnikPhone(n int) string {
	return fmt.Sprintf("+7 (900) 700-00-%02d", n)
}

// newDachnik регистрирует n-го участника теста.
func newDachnik(t *testing.T, baseURL string, n int) dachnik {
	t.Helper()

	token, id := signIn(t, baseURL, dachnikPhone(n))

	return dachnik{token: token, id: id}
}

// newDachniks регистрирует count участников с номерами first, first+1, ….
func newDachniks(t *testing.T, baseURL string, first, count int) []dachnik {
	t.Helper()

	people := make([]dachnik, 0, count)
	for i := 0; i < count; i++ {
		people = append(people, newDachnik(t, baseURL, first+i))
	}

	return people
}

// followAddress — адрес подписки на пользователя.
func followAddress(baseURL, userID string) string {
	return userAddress(baseURL, userID) + "/follow"
}

// follow подписывается на пользователя или подаёт заявку.
func follow(t *testing.T, baseURL, token, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, followAddress(baseURL, userID), token, nil)
}

// unfollow отписывается или отменяет заявку.
func unfollow(t *testing.T, baseURL, token, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, followAddress(baseURL, userID), token, nil)
}

// relationOK требует 200 и возвращает отношение из ответа.
func relationOK(t *testing.T, resp *http.Response, where string) relationPayload {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: ожидался статус 200, получен %d", where, resp.StatusCode)
	}

	var body relationPayload
	decode(t, resp, &body)

	return body
}

// followOK подписывается и требует успеха.
func followOK(t *testing.T, baseURL string, who dachnik, whom string) relationPayload {
	t.Helper()
	return relationOK(t, follow(t, baseURL, who.token, whom), "подписка")
}

// unfollowOK отписывается и требует успеха.
func unfollowOK(t *testing.T, baseURL string, who dachnik, whom string) relationPayload {
	t.Helper()
	return relationOK(t, unfollow(t, baseURL, who.token, whom), "отписка")
}

// requireRelation требует ровно такое отношение.
func requireRelation(t *testing.T, rel *relationPayload, following string, followedBy bool, where string) {
	t.Helper()

	if rel == nil {
		t.Fatalf("%s: отношения relation нет, а оно ожидалось", where)
	}
	if rel.Following != following {
		t.Errorf("%s: following = %q, ожидалось %q", where, rel.Following, following)
	}
	if rel.FollowedBy == nil {
		t.Errorf("%s: поля followed_by нет", where)
	} else if *rel.FollowedBy != followedBy {
		t.Errorf("%s: followed_by = %v, ожидалось %v", where, *rel.FollowedBy, followedBy)
	}
}

// setPrivacy закрывает или открывает свой профиль; body — как есть.
func setPrivacy(t *testing.T, baseURL, token string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/me/privacy", token, body)
}

// setClosed закрывает (closed == true) или открывает профиль и требует,
// чтобы ответ — CurrentUser — показал новое значение.
func setClosed(t *testing.T, baseURL, token string, closed bool) {
	t.Helper()

	resp := setPrivacy(t, baseURL, token, map[string]any{"closed": closed})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /me/privacy closed=%v: ожидался статус 200, получен %d", closed, resp.StatusCode)
	}

	var body map[string]any
	decode(t, resp, &body)
	if got, ok := body["closed"].(bool); !ok || got != closed {
		t.Fatalf("PUT /me/privacy closed=%v: в ответе closed = %v", closed, body["closed"])
	}
}

// meClosed — признак closed в GET /me. Поле обязано быть булевым.
func meClosed(t *testing.T, baseURL, token string) bool {
	t.Helper()

	resp := getProfile(t, baseURL, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /me: ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body map[string]any
	decode(t, resp, &body)

	closed, ok := body["closed"].(bool)
	if !ok {
		t.Fatalf("GET /me: поле closed должно быть true или false, получено %v", body["closed"])
	}

	return closed
}

// followProfile открывает профиль и требует, чтобы в нём были все поля фичи.
func followProfile(t *testing.T, baseURL, token, userID string) followProfilePayload {
	t.Helper()

	resp := fetchUser(t, baseURL, token, userID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("профиль %s: ожидался статус 200, получен %d", userID, resp.StatusCode)
	}

	var body followProfilePayload
	decode(t, resp, &body)

	if body.Posts == nil || body.Followers == nil || body.Following == nil || body.Closed == nil {
		t.Fatalf("профиль %s: нет одного из полей posts, followers, following, closed: %+v", userID, body)
	}

	return body
}

// requireCounts требует такие числа подписчиков и подписок в профиле.
func requireCounts(t *testing.T, baseURL, token, userID string, followers, following int, where string) {
	t.Helper()

	profile := followProfile(t, baseURL, token, userID)
	if *profile.Followers != followers {
		t.Errorf("%s: подписчиков %d, ожидалось %d", where, *profile.Followers, followers)
	}
	if *profile.Following != following {
		t.Errorf("%s: подписок %d, ожидалось %d", where, *profile.Following, following)
	}
}

// requireProfileRelation требует такое отношение смотрящего в профиле.
func requireProfileRelation(t *testing.T, baseURL string, viewer dachnik, userID, following string, followedBy bool, where string) {
	t.Helper()

	profile := followProfile(t, baseURL, viewer.token, userID)
	requireRelation(t, profile.Relation, following, followedBy, where)
}

// fetchFollowListRaw запрашивает список которой — "followers" или
// "following" — с готовой строкой запроса.
func fetchFollowListRaw(t *testing.T, baseURL, token, userID, which, query string) *http.Response {
	t.Helper()

	address := userAddress(baseURL, userID) + "/" + which
	if query != "" {
		address += "?" + query
	}

	return do(t, http.MethodGet, address, token, nil)
}

// fetchFollowList — то же с параметрами страницы.
func fetchFollowList(t *testing.T, baseURL, token, userID, which string, params url.Values) *http.Response {
	t.Helper()
	return fetchFollowListRaw(t, baseURL, token, userID, which, params.Encode())
}

// followListPage требует, чтобы страница списка отдалась, и возвращает её.
func followListPage(t *testing.T, baseURL, token, userID, which string, params url.Values) followListPayload {
	t.Helper()

	resp := fetchFollowList(t, baseURL, token, userID, which, params)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("список %s пользователя %s: ожидался статус 200, получен %d", which, userID, resp.StatusCode)
	}

	var page followListPayload
	decode(t, resp, &page)

	return page
}

// followUserIDs — идентификаторы людей страницы по порядку.
func followUserIDs(items []followUserPayload) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	return ids
}

// walkFollowList проходит список страницами по limit и возвращает людей
// подряд; требует, чтобы проход был конечен и страницы с курсором не пусты.
func walkFollowList(t *testing.T, baseURL, token, userID, which string, limit int) []string {
	t.Helper()

	var ids []string
	cursor := ""

	for page := 1; ; page++ {
		if page > 50 {
			t.Fatalf("список %s не кончается: за %d страниц отдано %d человек", which, page-1, len(ids))
		}

		got := followListPage(t, baseURL, token, userID, which, feedParams(limit, cursor))
		if len(got.Items) > limit {
			t.Fatalf("список %s, страница %d: %d человек при limit=%d", which, page, len(got.Items), limit)
		}
		if len(got.Items) == 0 && page > 1 {
			t.Fatalf("список %s, страница %d пуста, хотя курсор обещал продолжение", which, page)
		}
		ids = append(ids, followUserIDs(got.Items)...)

		if got.NextCursor == nil {
			break
		}
		if *got.NextCursor == "" {
			t.Fatalf("список %s, страница %d: курсор пустой", which, page)
		}
		cursor = *got.NextCursor
	}

	return ids
}

// fetchFollowRequests запрашивает заявки ко мне.
func fetchFollowRequests(t *testing.T, baseURL, token string, params url.Values) *http.Response {
	t.Helper()

	address := baseURL + "/me/follow-requests"
	if len(params) > 0 {
		address += "?" + params.Encode()
	}

	return do(t, http.MethodGet, address, token, nil)
}

// followRequestsPage требует, чтобы страница заявок отдалась.
func followRequestsPage(t *testing.T, baseURL, token string, params url.Values) authorListPayload {
	t.Helper()

	resp := fetchFollowRequests(t, baseURL, token, params)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("заявки: ожидался статус 200, получен %d", resp.StatusCode)
	}

	var page authorListPayload
	decode(t, resp, &page)

	return page
}

// requesterIDs — идентификаторы заявителей первой страницы (limit 50).
func requesterIDs(t *testing.T, baseURL, token string) []string {
	t.Helper()

	page := followRequestsPage(t, baseURL, token, feedParams(50, ""))
	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.ID)
	}

	return ids
}

// requestAddress — адрес заявки от пользователя ко мне.
func requestAddress(baseURL, userID string) string {
	return baseURL + "/me/follow-requests/" + url.PathEscape(userID)
}

// acceptRequest принимает заявку.
func acceptRequest(t *testing.T, baseURL, token, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, requestAddress(baseURL, userID), token, nil)
}

// declineRequest отклоняет заявку.
func declineRequest(t *testing.T, baseURL, token, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, requestAddress(baseURL, userID), token, nil)
}

// requireNoContent требует 204.
func requireNoContent(t *testing.T, resp *http.Response, where string) {
	t.Helper()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("%s: ожидался статус 204, получен %d", where, resp.StatusCode)
	}
}

// scopeParams — параметры ленты с вкладкой (пустой scope — параметра нет).
func scopeParams(scope string, limit int, cursor string) url.Values {
	params := feedParams(limit, cursor)
	if scope != "" {
		params.Set("scope", scope)
	}

	return params
}

// scopeFeedIDs — посты первой страницы вкладки (limit 50).
func scopeFeedIDs(t *testing.T, baseURL, token, scope string) []string {
	t.Helper()
	return postIDs(feedPage(t, baseURL, token, scopeParams(scope, 50, "")).Items)
}

// walkScopeFeed проходит вкладку ленты страницами по limit.
func walkScopeFeed(t *testing.T, baseURL, token, scope string, limit int) []string {
	t.Helper()

	var ids []string
	cursor := ""

	for page := 1; ; page++ {
		if page > 50 {
			t.Fatalf("вкладка %s не кончается", scope)
		}

		got := feedPage(t, baseURL, token, scopeParams(scope, limit, cursor))
		if len(got.Items) == 0 && page > 1 {
			t.Fatalf("вкладка %s, страница %d пуста, хотя курсор обещал продолжение", scope, page)
		}
		ids = append(ids, postIDs(got.Items)...)

		if got.NextCursor == nil {
			break
		}
		cursor = cursorOf(t, got, fmt.Sprintf("вкладка %s, страница %d", scope, page))
	}

	return ids
}

// contains — есть ли id в списке.
func contains(ids []string, id string) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}

	return false
}

// sortedCopy — отсортированная копия: для сравнения множеств.
func sortedCopy(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)

	return out
}

// --- PUT/DELETE /api/users/{userId}/follow --------------------------------

// Подписка на открытый профиль оформляется сразу: yes, у него +1 подписчик,
// у подписавшегося +1 подписка (ФТ-2, «Подписка на открытый профиль»).
func TestFollowOpenProfileAtOnce(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	rel := followOK(t, baseURL, a, b.id)
	requireRelation(t, &rel, followingYes, false, "ответ на подписку")

	requireCounts(t, baseURL, a.token, b.id, 1, 0, "профиль того, на кого подписались")
	requireCounts(t, baseURL, a.token, a.id, 0, 1, "профиль подписавшегося")
	requireProfileRelation(t, baseURL, a, b.id, followingYes, false, "A смотрит на B")
}

// Подписка односторонняя: B видит, что A на него подписан, но сам он на A
// не подписан (ФТ-1). Встречная подписка делает обе стороны yes/true (ФТ-5).
func TestFollowIsOneWayUntilMutual(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	followOK(t, baseURL, a, b.id)
	requireProfileRelation(t, baseURL, b, a.id, followingNone, true, "B смотрит на A после подписки A")

	rel := followOK(t, baseURL, b, a.id)
	requireRelation(t, &rel, followingYes, true, "ответ на встречную подписку")

	requireProfileRelation(t, baseURL, a, b.id, followingYes, true, "A смотрит на B")
	requireProfileRelation(t, baseURL, b, a.id, followingYes, true, "B смотрит на A")
	requireCounts(t, baseURL, a.token, a.id, 1, 1, "профиль A")
	requireCounts(t, baseURL, a.token, b.id, 1, 1, "профиль B")
}

// Повторная подписка ничего не меняет и не ошибка (ФТ-3, «Повторная
// подписка»).
func TestFollowAgainChangesNothing(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	followOK(t, baseURL, a, b.id)
	rel := followOK(t, baseURL, a, b.id)
	requireRelation(t, &rel, followingYes, false, "ответ на повторную подписку")

	requireCounts(t, baseURL, a.token, b.id, 1, 0, "профиль B")
	requireCounts(t, baseURL, a.token, a.id, 0, 1, "профиль A")
	if got := walkFollowList(t, baseURL, a.token, b.id, "followers", 50); !reflect.DeepEqual(got, []string{a.id}) {
		t.Errorf("подписчики B: %v, ожидался только A", got)
	}
}

// Повторная заявка на закрытый профиль тоже ничего не меняет: заявка одна
// (ФТ-3).
func TestFollowRequestAgainChangesNothing(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]
	setClosed(t, baseURL, b.token, true)

	followOK(t, baseURL, a, b.id)
	rel := followOK(t, baseURL, a, b.id)
	requireRelation(t, &rel, followingRequested, false, "ответ на повторную заявку")

	if got := requesterIDs(t, baseURL, b.token); !reflect.DeepEqual(got, []string{a.id}) {
		t.Errorf("заявки к B: %v, ожидалась одна от A", got)
	}
	requireCounts(t, baseURL, b.token, b.id, 0, 0, "профиль B")
}

// Подписаться на себя нельзя: 400 cannot_follow_self, числа не меняются
// (ФТ-1, «Подписка на себя»).
func TestFollowSelfIsRejected(t *testing.T) {
	baseURL := startAPI(t)
	a := newDachnik(t, baseURL, 1)

	requireError(t, follow(t, baseURL, a.token, a.id), http.StatusBadRequest, "cannot_follow_self")

	requireCounts(t, baseURL, a.token, a.id, 0, 0, "свой профиль после попытки")
}

// Отписка без подписки — 200 none, и повторная отписка тоже (ФТ-3,
// «Отписка без подписки»).
func TestUnfollowWithoutFollowIsNotAnError(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	for i := 1; i <= 2; i++ {
		rel := unfollowOK(t, baseURL, a, b.id)
		requireRelation(t, &rel, followingNone, false, fmt.Sprintf("отписка без подписки №%d", i))
	}
	requireCounts(t, baseURL, a.token, b.id, 0, 0, "профиль B")
}

// Отписка убирает подписку: none, числа обратно, в списках его нет (ФТ-3).
func TestUnfollowRemovesFollow(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	followOK(t, baseURL, b, a.id) // встречная подписка: followed_by у A остаётся
	followOK(t, baseURL, a, b.id)

	rel := unfollowOK(t, baseURL, a, b.id)
	requireRelation(t, &rel, followingNone, true, "ответ на отписку")

	rel = unfollowOK(t, baseURL, a, b.id)
	requireRelation(t, &rel, followingNone, true, "повторная отписка")

	requireCounts(t, baseURL, a.token, b.id, 0, 1, "профиль B")
	requireCounts(t, baseURL, a.token, a.id, 1, 0, "профиль A")
	requireProfileRelation(t, baseURL, a, b.id, followingNone, true, "A смотрит на B")
	if got := walkFollowList(t, baseURL, a.token, b.id, "followers", 50); len(got) != 0 {
		t.Errorf("у B после отписки подписчики %v, ожидалось пусто", got)
	}
}

// Пользователя нет или идентификатор не UUID — 404 user_not_found у
// подписки, отписки и обоих списков («Ошибки»).
func TestFollowUnknownUserIsNotFound(t *testing.T) {
	baseURL := startAPI(t)
	a := newDachnik(t, baseURL, 1)
	post := publishPost(t, baseURL, a.token, postCaption)

	cases := []struct {
		name   string
		userID string
	}{
		{"UUID, которого нет", unknownID},
		{"идентификатор поста вместо пользователя", post.ID},
		{"не UUID", "abc"},
		{"не UUID по-русски", notAnID},
	}

	for _, testCase := range cases {
		t.Run(testCase.name+": подписка", func(t *testing.T) {
			requireError(t, follow(t, baseURL, a.token, testCase.userID), http.StatusNotFound, "user_not_found")
		})
		t.Run(testCase.name+": отписка", func(t *testing.T) {
			requireError(t, unfollow(t, baseURL, a.token, testCase.userID), http.StatusNotFound, "user_not_found")
		})
		t.Run(testCase.name+": подписчики", func(t *testing.T) {
			resp := fetchFollowList(t, baseURL, a.token, testCase.userID, "followers", nil)
			requireError(t, resp, http.StatusNotFound, "user_not_found")
		})
		t.Run(testCase.name+": подписки", func(t *testing.T) {
			resp := fetchFollowList(t, baseURL, a.token, testCase.userID, "following", nil)
			requireError(t, resp, http.StatusNotFound, "user_not_found")
		})
	}
}

// --- Закрытый профиль и заявки --------------------------------------------

// Новый профиль открыт; closed есть и в /me, и в профиле; переключатель
// закрывает и открывает его (ФТ-6, «Изменения в существующих»).
func TestPrivacyTogglesClosedInMeAndProfile(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	if meClosed(t, baseURL, b.token) {
		t.Fatal("новый профиль должен быть открыт")
	}
	if *followProfile(t, baseURL, a.token, b.id).Closed {
		t.Fatal("новый профиль в глазах соседа должен быть открыт")
	}

	setClosed(t, baseURL, b.token, true)
	if !meClosed(t, baseURL, b.token) {
		t.Error("после закрытия GET /me должен показать closed: true")
	}
	if !*followProfile(t, baseURL, a.token, b.id).Closed {
		t.Error("после закрытия профиль должен быть closed: true для соседа")
	}
	if !*followProfile(t, baseURL, b.token, b.id).Closed {
		t.Error("после закрытия свой профиль должен быть closed: true")
	}

	setClosed(t, baseURL, b.token, false)
	if meClosed(t, baseURL, b.token) {
		t.Error("после открытия GET /me должен показать closed: false")
	}
	if *followProfile(t, baseURL, a.token, b.id).Closed {
		t.Error("после открытия профиль должен быть closed: false для соседа")
	}
}

// Без closed, с null или не булевым closed — 400 invalid_request, и признак
// не меняется («Ошибки», schema PrivacyUpdate).
func TestPrivacyRequiresBooleanClosed(t *testing.T) {
	baseURL := startAPI(t)
	a := newDachnik(t, baseURL, 1)
	setClosed(t, baseURL, a.token, true)

	cases := []struct {
		name string
		body any
	}{
		{"пустой объект", map[string]any{}},
		{"closed: null", map[string]any{"closed": nil}},
		{"closed строкой", map[string]any{"closed": "false"}},
		{"closed числом", map[string]any{"closed": 0}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			requireError(t, setPrivacy(t, baseURL, a.token, testCase.body), http.StatusBadRequest, "invalid_request")
			if !meClosed(t, baseURL, a.token) {
				t.Error("после отказа профиль должен остаться закрытым")
			}
		})
	}
}

// Подписка на закрытый профиль — заявка: requested, числа не меняются,
// у хозяина появилась заявка; в глазах хозяина заявитель на него не
// подписан (ФТ-2, ФТ-12, «Подписка на закрытый»).
func TestFollowClosedProfileSendsRequest(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]
	setClosed(t, baseURL, b.token, true)

	rel := followOK(t, baseURL, a, b.id)
	requireRelation(t, &rel, followingRequested, false, "ответ на заявку")

	requireCounts(t, baseURL, a.token, b.id, 0, 0, "профиль B")
	requireCounts(t, baseURL, a.token, a.id, 0, 0, "профиль A")
	requireProfileRelation(t, baseURL, a, b.id, followingRequested, false, "A смотрит на B")
	requireProfileRelation(t, baseURL, b, a.id, followingNone, false, "B смотрит на A: заявка — не подписка")

	if got := requesterIDs(t, baseURL, b.token); !reflect.DeepEqual(got, []string{a.id}) {
		t.Errorf("заявки к B: %v, ожидалась от A", got)
	}
	if got := walkFollowList(t, baseURL, b.token, b.id, "followers", 50); len(got) != 0 {
		t.Errorf("заявка попала в подписчики B: %v", got)
	}
	if got := walkFollowList(t, baseURL, a.token, a.id, "following", 50); len(got) != 0 {
		t.Errorf("заявка попала в подписки A: %v", got)
	}
}

// Отмена заявки: none, у хозяина заявки нет, принять её уже нельзя —
// 404 request_not_found («Отмена заявки», «Принятие заявки, которую уже
// отменили»).
func TestCancelledRequestIsGone(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]
	setClosed(t, baseURL, b.token, true)

	followOK(t, baseURL, a, b.id)
	rel := unfollowOK(t, baseURL, a, b.id)
	requireRelation(t, &rel, followingNone, false, "ответ на отмену заявки")

	if got := requesterIDs(t, baseURL, b.token); len(got) != 0 {
		t.Errorf("после отмены у B заявки %v, ожидалось пусто", got)
	}
	requireError(t, acceptRequest(t, baseURL, b.token, a.id), http.StatusNotFound, "request_not_found")
	requireError(t, declineRequest(t, baseURL, b.token, a.id), http.StatusNotFound, "request_not_found")

	requireCounts(t, baseURL, b.token, b.id, 0, 0, "профиль B")
	requireProfileRelation(t, baseURL, a, b.id, followingNone, false, "A смотрит на B")
}

// Принятие заявки: 204, заявитель — подписчик, заявки больше нет, посты
// хозяина ему открыты (ФТ-2, ФТ-9 сценария, ФТ-10).
func TestAcceptRequestMakesFollower(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]
	setClosed(t, baseURL, b.token, true)
	postID := publishPost(t, baseURL, b.token, postCaption).ID

	followOK(t, baseURL, a, b.id)
	requireNoContent(t, acceptRequest(t, baseURL, b.token, a.id), "принятие заявки")

	requireProfileRelation(t, baseURL, a, b.id, followingYes, false, "A смотрит на B")
	requireProfileRelation(t, baseURL, b, a.id, followingNone, true, "B смотрит на A")
	requireCounts(t, baseURL, a.token, b.id, 1, 0, "профиль B")
	requireCounts(t, baseURL, a.token, a.id, 0, 1, "профиль A")
	if got := requesterIDs(t, baseURL, b.token); len(got) != 0 {
		t.Errorf("после принятия у B заявки %v, ожидалось пусто", got)
	}

	requireFeedPosts(t, userPostsPage(t, baseURL, a.token, b.id, nil), []string{postID}, "посты B для A")
	if got := walkFollowList(t, baseURL, a.token, b.id, "followers", 50); !reflect.DeepEqual(got, []string{a.id}) {
		t.Errorf("подписчики B: %v, ожидался A", got)
	}

	// Повторно принять уже принятую нельзя: заявки больше нет.
	requireError(t, acceptRequest(t, baseURL, b.token, a.id), http.StatusNotFound, "request_not_found")
}

// Отклонение заявки: 204, заявка исчезает молча, заявитель видит none
// и может подать новую (ФТ-10, «Отклонённая заявка»).
func TestDeclinedRequestCanBeSentAgain(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]
	setClosed(t, baseURL, b.token, true)

	followOK(t, baseURL, a, b.id)
	requireNoContent(t, declineRequest(t, baseURL, b.token, a.id), "отклонение заявки")

	requireProfileRelation(t, baseURL, a, b.id, followingNone, false, "A смотрит на B после отказа")
	requireCounts(t, baseURL, a.token, b.id, 0, 0, "профиль B")
	if got := requesterIDs(t, baseURL, b.token); len(got) != 0 {
		t.Errorf("после отказа у B заявки %v, ожидалось пусто", got)
	}
	requireError(t, declineRequest(t, baseURL, b.token, a.id), http.StatusNotFound, "request_not_found")

	rel := followOK(t, baseURL, a, b.id)
	requireRelation(t, &rel, followingRequested, false, "новая заявка после отказа")
	if got := requesterIDs(t, baseURL, b.token); !reflect.DeepEqual(got, []string{a.id}) {
		t.Errorf("новая заявка не дошла до B: %v", got)
	}
}

// Заявки нет — 404 request_not_found: чужая заявка, подписка вместо заявки,
// неизвестный и не-UUID идентификатор («Ошибки»).
func TestAcceptOrDeclineWithoutRequestIsNotFound(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 4)
	a, b, c, d := people[0], people[1], people[2], people[3]
	setClosed(t, baseURL, b.token, true)
	followOK(t, baseURL, a, b.id) // заявка A → B
	followOK(t, baseURL, d, c.id) // подписка D → C, C открыт

	cases := []struct {
		name   string
		owner  dachnik
		userID string
	}{
		{"заявка A к B, а принимает C", c, a.id},
		{"D подписан на C, заявки нет", c, d.id},
		{"B, но от D заявки нет", b, d.id},
		{"UUID, которого нет", b, unknownID},
		{"не UUID", b, notAnID},
	}

	for _, testCase := range cases {
		t.Run(testCase.name+": принять", func(t *testing.T) {
			requireError(t, acceptRequest(t, baseURL, testCase.owner.token, testCase.userID), http.StatusNotFound, "request_not_found")
		})
		t.Run(testCase.name+": отклонить", func(t *testing.T) {
			requireError(t, declineRequest(t, baseURL, testCase.owner.token, testCase.userID), http.StatusNotFound, "request_not_found")
		})
	}

	// Ничего из этого не тронуло настоящие связи.
	requireProfileRelation(t, baseURL, a, b.id, followingRequested, false, "заявка A → B")
	requireProfileRelation(t, baseURL, d, c.id, followingYes, false, "подписка D → C")
}

// Закрытие профиля не трогает подписчиков: они остаются и видят посты,
// а новые подписываются по заявке (ФТ-8, «Хозяин закрывает профиль»).
func TestClosingProfileKeepsFollowers(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	a, b, c := people[0], people[1], people[2]
	postID := publishPost(t, baseURL, b.token, postCaption).ID

	followOK(t, baseURL, a, b.id)
	setClosed(t, baseURL, b.token, true)

	requireProfileRelation(t, baseURL, a, b.id, followingYes, false, "старый подписчик после закрытия")
	requireCounts(t, baseURL, a.token, b.id, 1, 0, "профиль B после закрытия")
	requireFeedPosts(t, userPostsPage(t, baseURL, a.token, b.id, nil), []string{postID}, "посты B для старого подписчика")
	if !contains(scopeFeedIDs(t, baseURL, a.token, "following"), postID) {
		t.Error("пост B пропал из «Подписок» старого подписчика после закрытия")
	}

	rel := followOK(t, baseURL, c, b.id)
	requireRelation(t, &rel, followingRequested, false, "новый после закрытия")
	requireCounts(t, baseURL, a.token, b.id, 1, 0, "профиль B после заявки C")
}

// Открытие профиля принимает все ждущие заявки: заявители — подписчики,
// заявок нет (ФТ-9, «Хозяин открывает профиль с тремя заявками»).
func TestOpeningProfileAcceptsAllRequests(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 4)
	b, requesters := people[0], people[1:]
	setClosed(t, baseURL, b.token, true)

	var want []string
	for _, r := range requesters {
		followOK(t, baseURL, r, b.id)
		want = append(want, r.id)
	}
	if got := requesterIDs(t, baseURL, b.token); !reflect.DeepEqual(sortedCopy(got), sortedCopy(want)) {
		t.Fatalf("до открытия у B заявки %v, ожидались %v", got, want)
	}

	setClosed(t, baseURL, b.token, false)

	if got := requesterIDs(t, baseURL, b.token); len(got) != 0 {
		t.Errorf("после открытия у B остались заявки %v", got)
	}
	requireCounts(t, baseURL, b.token, b.id, 3, 0, "профиль B после открытия")
	got := walkFollowList(t, baseURL, b.token, b.id, "followers", 50)
	if !reflect.DeepEqual(sortedCopy(got), sortedCopy(want)) {
		t.Errorf("подписчики B после открытия %v, ожидались %v", got, want)
	}
	for i, r := range requesters {
		requireProfileRelation(t, baseURL, r, b.id, followingYes, false, fmt.Sprintf("заявитель %d после открытия", i+1))
	}
}

// Закрытый профиль не-подписчику: шапка и три числа видны, число постов —
// сколько он увидит, подписавшись, а не ноль (ФТ-7, ФТ-13).
func TestClosedProfileHeaderAndCountsAreVisibleToStranger(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 4)
	stranger, b, follower, followee := people[0], people[1], people[2], people[3]

	introduce(t, baseURL, b.token, profileName)
	publishPosts(t, baseURL, b.token, 3)
	followOK(t, baseURL, follower, b.id)
	followOK(t, baseURL, b, followee.id)
	setClosed(t, baseURL, b.token, true)

	profile := followProfile(t, baseURL, stranger.token, b.id)
	if profile.Name != profileName {
		t.Errorf("имя закрытого профиля %q, ожидалось %q", profile.Name, profileName)
	}
	if profile.Nickname == "" || profile.CreatedAt == "" {
		t.Errorf("у закрытого профиля должны быть видны никнейм и дата регистрации: %+v", profile)
	}
	if *profile.Posts != 3 {
		t.Errorf("не-подписчик видит %d постов закрытого профиля, ожидалось 3 (не ноль)", *profile.Posts)
	}
	if *profile.Followers != 1 || *profile.Following != 1 {
		t.Errorf("числа закрытого профиля: подписчиков %d, подписок %d; ожидалось 1 и 1", *profile.Followers, *profile.Following)
	}
	if !*profile.Closed {
		t.Error("профиль должен быть closed: true")
	}
	requireRelation(t, profile.Relation, followingNone, false, "не-подписчик смотрит на закрытый профиль")
}

// Посты закрытого профиля — только хозяину и подписчикам; не-подписчику
// и заявителю — 403 profile_closed (ФТ-7, «Посты закрытого профиля без
// подписки»).
func TestClosedProfilePostsAreForFollowersOnly(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 4)
	b, follower, stranger, requester := people[0], people[1], people[2], people[3]
	postID := publishPost(t, baseURL, b.token, postCaption).ID

	followOK(t, baseURL, follower, b.id)
	setClosed(t, baseURL, b.token, true)
	followOK(t, baseURL, requester, b.id)

	requireFeedPosts(t, userPostsPage(t, baseURL, b.token, b.id, nil), []string{postID}, "свои посты хозяина")
	requireFeedPosts(t, userPostsPage(t, baseURL, follower.token, b.id, nil), []string{postID}, "посты для подписчика")

	requireError(t, fetchUserPosts(t, baseURL, stranger.token, b.id, nil), http.StatusForbidden, "profile_closed")
	requireError(t, fetchUserPosts(t, baseURL, requester.token, b.id, nil), http.StatusForbidden, "profile_closed")
}

// Списки закрытого профиля — только хозяину и подписчикам; остальным —
// 403 profile_closed, а числа в профиле видны («Списки закрытого профиля
// без подписки»).
func TestClosedProfileListsAreForFollowersOnly(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 5)
	b, follower, stranger, requester, followee := people[0], people[1], people[2], people[3], people[4]

	followOK(t, baseURL, follower, b.id)
	followOK(t, baseURL, b, followee.id)
	setClosed(t, baseURL, b.token, true)
	followOK(t, baseURL, requester, b.id)

	for _, which := range []string{"followers", "following"} {
		for _, viewer := range []struct {
			name string
			who  dachnik
		}{{"хозяин", b}, {"подписчик", follower}} {
			page := followListPage(t, baseURL, viewer.who.token, b.id, which, nil)
			if len(page.Items) != 1 {
				t.Errorf("%s: список %s закрытого профиля — %v, ожидался один человек", viewer.name, which, followUserIDs(page.Items))
			}
		}
		for _, viewer := range []struct {
			name string
			who  dachnik
		}{{"не-подписчик", stranger}, {"заявитель", requester}} {
			t.Run(viewer.name+": "+which, func(t *testing.T) {
				resp := fetchFollowList(t, baseURL, viewer.who.token, b.id, which, nil)
				requireError(t, resp, http.StatusForbidden, "profile_closed")
			})
		}
	}

	requireCounts(t, baseURL, stranger.token, b.id, 1, 1, "числа закрытого профиля для не-подписчика")
}

// --- Профиль: relation ----------------------------------------------------

// В своём профиле relation нет, а followers, following и closed есть;
// в чужом — relation с обоими полями (ФТ-11, «Изменения в существующих»).
func TestOwnProfileHasNoRelation(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	own := userProfileFields(t, baseURL, a.token, a.id)
	if _, ok := own["relation"]; ok {
		t.Errorf("в своём профиле relation быть не должно, получено %v", own["relation"])
	}
	for _, field := range []string{"followers", "following", "closed", "posts"} {
		if _, ok := own[field]; !ok {
			t.Errorf("в своём профиле нет поля %s", field)
		}
	}

	other := userProfileFields(t, baseURL, a.token, b.id)
	raw, ok := other["relation"].(map[string]any)
	if !ok {
		t.Fatalf("в чужом профиле relation должен быть объектом, получено %v", other["relation"])
	}
	if raw["following"] != followingNone {
		t.Errorf("relation.following = %v, ожидалось none", raw["following"])
	}
	if raw["followed_by"] != false {
		t.Errorf("relation.followed_by = %v, ожидалось false", raw["followed_by"])
	}
}

// --- Списки подписчиков и подписок ----------------------------------------

// Подписчики — новые связи сверху, у каждого отношение смотрящего,
// у самого смотрящего relation нет (ФТ-14).
func TestFollowersNewestFirstWithRelation(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 5)
	b, viewer, c, d, e := people[0], people[1], people[2], people[3], people[4]

	// Подписчики B по порядку: смотрящий, C, D, E.
	followOK(t, baseURL, viewer, b.id)
	followOK(t, baseURL, c, b.id)
	followOK(t, baseURL, d, b.id)
	followOK(t, baseURL, e, b.id)

	// Отношения смотрящего: на C подписан; D подписан на него; E закрыт,
	// и смотрящий подал ему заявку.
	followOK(t, baseURL, viewer, c.id)
	followOK(t, baseURL, d, viewer.id)
	setClosed(t, baseURL, e.token, true)
	followOK(t, baseURL, viewer, e.id)

	page := followListPage(t, baseURL, viewer.token, b.id, "followers", nil)
	want := []string{e.id, d.id, c.id, viewer.id}
	if got := followUserIDs(page.Items); !reflect.DeepEqual(got, want) {
		t.Fatalf("подписчики B: %v, ожидались новые сверху %v", got, want)
	}
	requireListEnd(t, page, "подписчики B")

	requireRelation(t, page.Items[0].Relation, followingRequested, false, "E в списке")
	requireRelation(t, page.Items[1].Relation, followingNone, true, "D в списке")
	requireRelation(t, page.Items[2].Relation, followingYes, false, "C в списке")
	if page.Items[3].Relation != nil {
		t.Errorf("у самого смотрящего в списке relation быть не должно, получено %+v", *page.Items[3].Relation)
	}
	for _, item := range page.Items {
		if item.Nickname == "" {
			t.Errorf("у %s в списке пустой никнейм", item.ID)
		}
	}
}

// Подписки — новые связи сверху, заявки не входят, relation смотрящего
// у каждого (ФТ-12, ФТ-14).
func TestFollowingNewestFirstWithoutRequests(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 5)
	a, c, d, closed, viewer := people[0], people[1], people[2], people[3], people[4]
	setClosed(t, baseURL, closed.token, true)

	followOK(t, baseURL, a, c.id)
	followOK(t, baseURL, a, closed.id) // заявка — не подписка
	followOK(t, baseURL, a, d.id)
	followOK(t, baseURL, a, viewer.id)
	followOK(t, baseURL, viewer, d.id)

	page := followListPage(t, baseURL, viewer.token, a.id, "following", nil)
	want := []string{viewer.id, d.id, c.id}
	if got := followUserIDs(page.Items); !reflect.DeepEqual(got, want) {
		t.Fatalf("подписки A: %v, ожидались новые сверху без заявок %v", got, want)
	}
	if page.Items[0].Relation != nil {
		t.Errorf("у самого смотрящего в списке relation быть не должно, получено %+v", *page.Items[0].Relation)
	}
	requireRelation(t, page.Items[1].Relation, followingYes, false, "D в списке")
	requireRelation(t, page.Items[2].Relation, followingNone, false, "C в списке")

	requireCounts(t, baseURL, viewer.token, a.id, 0, 3, "профиль A: заявка не в счёт")
}

// Пустые списки — пустая страница без курсора.
func TestFollowListsOfNewcomerAreEmpty(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	for _, which := range []string{"followers", "following"} {
		page := followListPage(t, baseURL, a.token, b.id, which, nil)
		if len(page.Items) != 0 {
			t.Errorf("список %s новичка: %v, ожидалось пусто", which, followUserIDs(page.Items))
		}
		requireListEnd(t, page, "список "+which+" новичка")
	}
}

// Списки отдаются страницами по курсору: проход по limit=2 даёт всех
// по одному разу в порядке первой страницы (ФТ-14, ФТ-18).
func TestFollowListsPaginateByCursor(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 6)
	b, others := people[0], people[1:]

	var followersWant, followingWant []string
	for _, o := range others {
		followOK(t, baseURL, o, b.id)
		followOK(t, baseURL, b, o.id)
		followersWant = append([]string{o.id}, followersWant...)
		followingWant = append([]string{o.id}, followingWant...)
	}

	cases := []struct {
		which string
		want  []string
	}{
		{"followers", followersWant},
		{"following", followingWant},
	}
	for _, testCase := range cases {
		t.Run(testCase.which, func(t *testing.T) {
			for _, limit := range []int{1, 2, 5} {
				got := walkFollowList(t, baseURL, others[0].token, b.id, testCase.which, limit)
				if !reflect.DeepEqual(got, testCase.want) {
					t.Errorf("limit=%d: проход дал %v, ожидалось %v", limit, got, testCase.want)
				}
			}

			page := followListPage(t, baseURL, others[0].token, b.id, testCase.which, feedParams(5, ""))
			requireListEnd(t, page, "ровно limit человек")

			page = followListPage(t, baseURL, others[0].token, b.id, testCase.which, nil)
			if len(page.Items) != len(testCase.want) {
				t.Errorf("без limit пришло %d человек, ожидалось %d", len(page.Items), len(testCase.want))
			}
		})
	}
}

// Неверный limit — 400 invalid_request, битый курсор — 400 invalid_cursor
// («Ошибки», контракт).
func TestFollowListsRejectBadLimitAndCursor(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]
	followOK(t, baseURL, a, b.id)

	for _, which := range []string{"followers", "following"} {
		for _, query := range []string{"limit=0", "limit=51", "limit=abc", "limit=-1"} {
			t.Run(which+" "+query, func(t *testing.T) {
				requireError(t, fetchFollowListRaw(t, baseURL, a.token, b.id, which, query), http.StatusBadRequest, "invalid_request")
			})
		}
		t.Run(which+" битый курсор", func(t *testing.T) {
			resp := fetchFollowListRaw(t, baseURL, a.token, b.id, which, "cursor="+url.QueryEscape("курсор"))
			requireError(t, resp, http.StatusBadRequest, "invalid_cursor")
		})
	}
}

// --- GET /api/me/follow-requests ------------------------------------------

// Заявки ко мне — только ждущие, новые сверху, страницами (ФТ-10,
// «API / контракт данных»).
func TestFollowRequestsNewestFirstOnlyPending(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 6)
	b, r1, r2, r3, r4, r5 := people[0], people[1], people[2], people[3], people[4], people[5]
	setClosed(t, baseURL, b.token, true)

	for _, r := range []dachnik{r1, r2, r3, r4, r5} {
		followOK(t, baseURL, r, b.id)
	}

	all := followRequestsPage(t, baseURL, b.token, nil)
	want := []string{r5.id, r4.id, r3.id, r2.id, r1.id}
	got := make([]string, 0, len(all.Items))
	for _, item := range all.Items {
		got = append(got, item.ID)
		if item.Nickname == "" {
			t.Errorf("у заявителя %s пустой никнейм", item.ID)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("заявки к B: %v, ожидались новые сверху %v", got, want)
	}

	requireNoContent(t, acceptRequest(t, baseURL, b.token, r2.id), "принятие r2")
	requireNoContent(t, declineRequest(t, baseURL, b.token, r4.id), "отклонение r4")
	unfollowOK(t, baseURL, r5, b.id)

	want = []string{r3.id, r1.id}
	if got := requesterIDs(t, baseURL, b.token); !reflect.DeepEqual(got, want) {
		t.Fatalf("ждущие заявки: %v, ожидались %v", got, want)
	}

	// Страницами по одной.
	first := followRequestsPage(t, baseURL, b.token, feedParams(1, ""))
	if len(first.Items) != 1 || first.Items[0].ID != r3.id {
		t.Fatalf("первая страница заявок по одной: %+v, ожидался r3", first.Items)
	}
	if first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatal("после первой заявки из двух ожидался курсор")
	}
	second := followRequestsPage(t, baseURL, b.token, feedParams(1, *first.NextCursor))
	if len(second.Items) != 1 || second.Items[0].ID != r1.id {
		t.Fatalf("вторая страница заявок: %+v, ожидался r1", second.Items)
	}
	if second.NextCursor != nil {
		t.Errorf("после последней заявки курсора быть не должно, получен %q", *second.NextCursor)
	}

	// У открытого профиля без заявок — пустая страница.
	if got := requesterIDs(t, baseURL, r1.token); len(got) != 0 {
		t.Errorf("у r1 заявок быть не должно: %v", got)
	}
}

// --- GET /api/feed?scope=… ------------------------------------------------

// «Все»: посты всех, кроме закрытых профилей без подписки; свои и посты
// закрытого, на которого подписан, — есть. Без scope — то же, что all
// (ФТ-15, ФТ-16, «Посты закрытого профиля без подписки»).
func TestFeedAllHidesClosedProfilesWithoutFollow(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 5)
	viewer, open, closedFollowed, closedStranger, closedRequested := people[0], people[1], people[2], people[3], people[4]

	followOK(t, baseURL, viewer, closedFollowed.id)
	for _, c := range []dachnik{closedFollowed, closedStranger, closedRequested} {
		setClosed(t, baseURL, c.token, true)
	}
	followOK(t, baseURL, viewer, closedRequested.id)

	own := publishPost(t, baseURL, viewer.token, "свой").ID
	openPost := publishPost(t, baseURL, open.token, "открытый").ID
	followedPost := publishPost(t, baseURL, closedFollowed.token, "закрытый, подписан").ID
	strangerPost := publishPost(t, baseURL, closedStranger.token, "закрытый, чужой").ID
	publishPost(t, baseURL, closedRequested.token, "закрытый, заявка")

	want := []string{followedPost, openPost, own}
	requireFeedPosts(t, feedPage(t, baseURL, viewer.token, scopeParams("all", 50, "")), want, "вкладка all")
	requireFeedPosts(t, feedPage(t, baseURL, viewer.token, scopeParams("", 50, "")), want, "лента без scope")
	if got := walkScopeFeed(t, baseURL, viewer.token, "all", 1); !reflect.DeepEqual(got, want) {
		t.Errorf("проход all по одному: %v, ожидалось %v", got, want)
	}

	// Хозяин закрытого профиля свои посты во «Всех» видит.
	if !contains(scopeFeedIDs(t, baseURL, closedStranger.token, "all"), strangerPost) {
		t.Error("хозяин закрытого профиля не видит свой пост во «Всех»")
	}
}

// «Подписки»: свои посты и посты тех, на кого подписан; заявка постов не
// даёт, чужие открытые не приходят (ФТ-17, «Свои посты»).
func TestFeedFollowingShowsOwnAndFollowed(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 5)
	viewer, followed, closedFollowed, requested, stranger := people[0], people[1], people[2], people[3], people[4]

	followOK(t, baseURL, viewer, followed.id)
	followOK(t, baseURL, viewer, closedFollowed.id)
	setClosed(t, baseURL, closedFollowed.token, true)
	setClosed(t, baseURL, requested.token, true)
	followOK(t, baseURL, viewer, requested.id)
	followOK(t, baseURL, stranger, viewer.id) // подписка на смотрящего не даёт его постов смотрящему

	own := publishPost(t, baseURL, viewer.token, "свой").ID
	followedPost := publishPost(t, baseURL, followed.token, "подписан").ID
	publishPost(t, baseURL, stranger.token, "чужой открытый")
	closedPost := publishPost(t, baseURL, closedFollowed.token, "закрытый, подписан").ID
	publishPost(t, baseURL, requested.token, "закрытый, заявка")
	own2 := publishPost(t, baseURL, viewer.token, "свой второй").ID

	want := []string{own2, closedPost, followedPost, own}
	requireFeedPosts(t, feedPage(t, baseURL, viewer.token, scopeParams("following", 50, "")), want, "вкладка following")
	if got := walkScopeFeed(t, baseURL, viewer.token, "following", 2); !reflect.DeepEqual(got, want) {
		t.Errorf("проход following по два: %v, ожидалось %v", got, want)
	}

	// После отписки посты уходят из «Подписок».
	unfollowOK(t, baseURL, viewer, followed.id)
	want = []string{own2, closedPost, own}
	requireFeedPosts(t, feedPage(t, baseURL, viewer.token, scopeParams("following", 50, "")), want, "following после отписки")

	// После принятия заявки посты по ней приходят.
	requireNoContent(t, acceptRequest(t, baseURL, requested.token, viewer.id), "принятие заявки смотрящего")
	got := scopeFeedIDs(t, baseURL, viewer.token, "following")
	if len(got) != 4 {
		t.Errorf("после принятия заявки во вкладке %d постов (%v), ожидалось 4", len(got), got)
	}
}

// Новичок: «Подписки» пусты — пустая страница без курсора, хотя другие
// выкладывают посты («Новичок»).
func TestFeedFollowingIsEmptyForNewcomer(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	newcomer, other := people[0], people[1]
	publishPosts(t, baseURL, other.token, 3)

	page := feedPage(t, baseURL, newcomer.token, scopeParams("following", 0, ""))
	if len(page.Items) != 0 {
		t.Errorf("«Подписки» новичка: %v, ожидалось пусто", postIDs(page.Items))
	}
	requireFeedEnd(t, page, "«Подписки» новичка")

	if got := scopeFeedIDs(t, baseURL, newcomer.token, "all"); len(got) != 3 {
		t.Errorf("во «Всех» у новичка %d постов, ожидалось 3", len(got))
	}
}

// Курсор одной вкладки в другой — не ошибка: то же место во времени (ФТ-18).
func TestFeedCursorOfOneTabWorksInTheOther(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	viewer, followed, stranger := people[0], people[1], people[2]
	followOK(t, baseURL, viewer, followed.id)

	oldFollowed := publishPost(t, baseURL, followed.token, "старый").ID
	publishPost(t, baseURL, stranger.token, "чужой")
	publishPost(t, baseURL, followed.token, "свежий")

	first := feedPage(t, baseURL, viewer.token, scopeParams("all", 2, ""))
	cursor := cursorOf(t, first, "первая страница «Всех»")

	page := feedPage(t, baseURL, viewer.token, scopeParams("following", 10, cursor))
	requireFeedPosts(t, page, []string{oldFollowed}, "«Подписки» с курсором «Всех»")
}

// Неизвестный scope — 400 invalid_request (контракт).
func TestFeedRejectsUnknownScope(t *testing.T) {
	baseURL := startAPI(t)
	a := newDachnik(t, baseURL, 1)

	for _, scope := range []string{"friends", "ALL", "Following", "все"} {
		t.Run(scope, func(t *testing.T) {
			resp := fetchFeedRaw(t, baseURL, a.token, "scope="+url.QueryEscape(scope))
			requireError(t, resp, http.StatusBadRequest, "invalid_request")
		})
	}
}

// --- Доступ -----------------------------------------------------------------

// Все новые ручки — только вошедшему: без токена 401 unauthorized.
func TestFollowsRequireValidToken(t *testing.T) {
	baseURL := startAPI(t)
	a := newDachnik(t, baseURL, 1)

	cases := []struct {
		name    string
		method  string
		address string
		body    any
	}{
		{"подписка", http.MethodPut, followAddress(baseURL, a.id), nil},
		{"отписка", http.MethodDelete, followAddress(baseURL, a.id), nil},
		{"подписчики", http.MethodGet, userAddress(baseURL, a.id) + "/followers", nil},
		{"подписки", http.MethodGet, userAddress(baseURL, a.id) + "/following", nil},
		{"закрыть профиль", http.MethodPut, baseURL + "/me/privacy", map[string]any{"closed": true}},
		{"заявки", http.MethodGet, baseURL + "/me/follow-requests", nil},
		{"принять заявку", http.MethodPut, requestAddress(baseURL, a.id), nil},
		{"отклонить заявку", http.MethodDelete, requestAddress(baseURL, a.id), nil},
		{"вкладка «Подписки»", http.MethodGet, baseURL + "/feed?scope=following", nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			resp := do(t, testCase.method, testCase.address, "", testCase.body)
			requireError(t, resp, http.StatusUnauthorized, "unauthorized")
		})
	}
}

// requireListEnd требует, чтобы за страницей списка ничего не было.
func requireListEnd(t *testing.T, page followListPayload, where string) {
	t.Helper()

	if page.NextCursor != nil {
		t.Errorf("%s: ожидался конец списка без курсора, получен курсор %q", where, *page.NextCursor)
	}
}
