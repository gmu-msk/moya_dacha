package tests

// Тесты по specs/030-group-posts.md, требования 1–15: пост в группах,
// поле groups у поста, посты групп в «Подписках», лента группы, кто удаляет
// пост в группе и что с ним бывает после выхода из группы и её удаления.
// Написаны по спецификации и контракту, без взгляда на реализацию
// (ADR-0002).
//
// Группы создаются и наполняются хелперами groups_test.go (gr*), видимость
// постов — хелперами 013 (post_visibility_test.go), блокировки — 022
// (edit_block_delete_test.go), удаление из дашборда — 023
// (moderation_test.go). Свои хелперы этого файла — с префиксом gp.
//
// Требования 16–18 (приложение) проверяются не здесь.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// gpNotUUID — идентификатор группы, который не похож на UUID.
const gpNotUUID = "не-группа"

// --- Представления из контракта -------------------------------------------

// gpBrief — schema GroupBrief.
type gpBrief struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// gpPost — пост с полями, важными этой фиче (schema Post). groups —
// указатель: сервис с группами в постах отдаёт поле всегда, его отсутствие
// или null — ошибка.
type gpPost struct {
	ID         string `json:"id"`
	Caption    string `json:"caption"`
	Visibility string `json:"visibility"`
	Author     struct {
		ID string `json:"id"`
	} `json:"author"`
	Groups *[]gpBrief `json:"groups"`
}

// gpFeed — schema Feed.
type gpFeed struct {
	Items      *[]gpPost `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}

// --- Хелперы: люди ---------------------------------------------------------

// gpPhone — номер n-го участника теста; не пересекается с другими файлами.
func gpPhone(n int) string {
	return fmt.Sprintf("+7 (900) 730-00-%02d", n)
}

// newGpUser регистрирует n-го участника теста.
func newGpUser(t *testing.T, baseURL string, n int) dachnik {
	t.Helper()

	token, id := signIn(t, baseURL, gpPhone(n))

	return dachnik{token: token, id: id}
}

// --- Хелперы: посты --------------------------------------------------------

// gpPublishWith публикует пост с одной уже загруженной фотографией; extra —
// поля тела сверх media_ids (group_ids, visibility, tags, …) как есть.
func gpPublishWith(t *testing.T, baseURL, token, mediaID string, extra map[string]any) *http.Response {
	t.Helper()

	body := map[string]any{"media_ids": []string{mediaID}, "caption": "Во вторник отключат воду"}
	for k, v := range extra {
		body[k] = v
	}

	return createPost(t, baseURL, token, body)
}

// gpPublishReq — то же с новой фотографией; фотография возвращается, чтобы
// проверить, что после отказа она осталась неопубликованной.
func gpPublishReq(t *testing.T, baseURL, token string, extra map[string]any) (*http.Response, mediaPayload) {
	t.Helper()

	photo := photoOf(t, baseURL, token, 60, 40)

	return gpPublishWith(t, baseURL, token, photo.ID, extra), photo
}

// gpParsePost разбирает один объект Post и требует поле groups.
func gpParsePost(t *testing.T, raw []byte, where string) gpPost {
	t.Helper()

	var post gpPost
	if err := json.Unmarshal(raw, &post); err != nil {
		t.Fatalf("%s: пост не разобрался как JSON: %v", where, err)
	}
	if post.Groups == nil {
		t.Errorf("%s: у поста %s нет поля groups (или оно null), ожидался массив, пусть пустой", where, post.ID)
	}

	return post
}

// gpPostOK требует статус и возвращает пост из ответа.
func gpPostOK(t *testing.T, resp *http.Response, status int, where string) gpPost {
	t.Helper()

	grStatus(t, resp, status, where)

	return gpParsePost(t, rawJSON(t, resp), where)
}

// gpPublish публикует пост в группах groupIDs (без них — поля group_ids
// нет) с видимостью visibility (пустая — поля нет) и требует 201.
func gpPublish(t *testing.T, baseURL string, who dachnik, visibility string, groupIDs ...string) gpPost {
	t.Helper()

	extra := map[string]any{}
	if len(groupIDs) > 0 {
		extra["group_ids"] = groupIDs
	}
	if visibility != "" {
		extra["visibility"] = visibility
	}

	resp, _ := gpPublishReq(t, baseURL, who.token, extra)

	return gpPostOK(t, resp, http.StatusCreated, fmt.Sprintf("публикация поста в группах %q", groupIDs))
}

// gpGet открывает пост по адресу и требует 200.
func gpGet(t *testing.T, baseURL string, who dachnik, postID string) gpPost {
	t.Helper()
	return gpPostOK(t, fetchPost(t, baseURL, who.token, postID), http.StatusOK, "GET /posts/"+postID)
}

// gpBriefOf — группа так, как она должна стоять в groups поста.
func gpBriefOf(g grGroup) gpBrief {
	return gpBrief{ID: g.ID, Name: g.Name}
}

// gpRequireGroups требует у поста ровно эти группы в этом порядке; пустой
// want — пустой массив.
func gpRequireGroups(t *testing.T, post gpPost, want []gpBrief, where string) {
	t.Helper()

	if post.Groups == nil {
		t.Errorf("%s: у поста %s нет поля groups, ожидалось %+v", where, post.ID, want)
		return
	}
	if len(*post.Groups) == 0 && len(want) == 0 {
		return
	}
	if !slices.Equal(*post.Groups, want) {
		t.Errorf("%s: groups = %+v, ожидалось %+v", where, *post.Groups, want)
	}
}

// --- Хелперы: ленты --------------------------------------------------------

// gpFeedOK требует 200 и разбирает страницу ленты, требуя items и groups
// у каждого поста.
func gpFeedOK(t *testing.T, resp *http.Response, where string) gpFeed {
	t.Helper()

	grStatus(t, resp, http.StatusOK, where)

	var raw struct {
		Items      *[]json.RawMessage `json:"items"`
		NextCursor *string            `json:"next_cursor"`
	}
	if err := json.Unmarshal(rawJSON(t, resp), &raw); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON: %v", where, err)
	}
	if raw.Items == nil {
		t.Fatalf("%s: в ответе нет items (пустая лента — [], а не отсутствие поля)", where)
	}

	items := make([]gpPost, 0, len(*raw.Items))
	for _, item := range *raw.Items {
		items = append(items, gpParsePost(t, item, where))
	}

	return gpFeed{Items: &items, NextCursor: raw.NextCursor}
}

// gpScopeFeed — первая страница вкладки ленты (limit 50).
func gpScopeFeed(t *testing.T, baseURL string, who dachnik, scope string) gpFeed {
	t.Helper()

	params := scopeParams(scope, 50, "")

	return gpFeedOK(t, fetchFeed(t, baseURL, who.token, params), "лента ?"+params.Encode())
}

// gpFollowing и gpAll — две вкладки ленты.
func gpFollowing(t *testing.T, baseURL string, who dachnik) gpFeed {
	t.Helper()
	return gpScopeFeed(t, baseURL, who, "following")
}

func gpAll(t *testing.T, baseURL string, who dachnik) gpFeed {
	t.Helper()
	return gpScopeFeed(t, baseURL, who, "all")
}

// gpUserPosts — первая страница постов пользователя (limit 50).
func gpUserPosts(t *testing.T, baseURL string, who dachnik, userID string) gpFeed {
	t.Helper()
	return gpFeedOK(t, fetchUserPosts(t, baseURL, who.token, userID, feedParams(50, "")), "посты пользователя")
}

// gpGroupPostsURL — адрес ленты группы с готовой строкой запроса.
func gpGroupPostsURL(baseURL, groupID, query string) string {
	address := grGroupURL(baseURL, groupID) + "/posts"
	if query != "" {
		address += "?" + query
	}
	return address
}

// gpGroupPostsReq — GET /groups/{id}/posts с готовой строкой запроса.
func gpGroupPostsReq(t *testing.T, baseURL, token, groupID, query string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, gpGroupPostsURL(baseURL, groupID, query), token, nil)
}

// gpGroupPage — страница ленты группы, 200.
func gpGroupPage(t *testing.T, baseURL string, who dachnik, groupID string, params url.Values) gpFeed {
	t.Helper()

	query := params.Encode()

	return gpFeedOK(t, gpGroupPostsReq(t, baseURL, who.token, groupID, query), "лента группы ?"+query)
}

// gpGroupFeed — первая страница ленты группы (limit 50).
func gpGroupFeed(t *testing.T, baseURL string, who dachnik, groupID string) gpFeed {
	t.Helper()
	return gpGroupPage(t, baseURL, who, groupID, feedParams(50, ""))
}

// gpWalkGroup проходит ленту группы страницами по limit.
func gpWalkGroup(t *testing.T, baseURL string, who dachnik, groupID string, limit int) []string {
	t.Helper()

	var ids []string
	cursor := ""

	for page := 1; ; page++ {
		if page > 50 {
			t.Fatalf("лента группы не кончается")
		}

		got := gpGroupPage(t, baseURL, who, groupID, feedParams(limit, cursor))
		if len(*got.Items) > limit {
			t.Errorf("лента группы, страница %d: постов %d при limit %d", page, len(*got.Items), limit)
		}
		if len(*got.Items) == 0 && page > 1 {
			t.Fatalf("лента группы, страница %d пуста, хотя курсор обещал продолжение", page)
		}
		ids = append(ids, gpIDs(got)...)

		if got.NextCursor == nil {
			break
		}
		if *got.NextCursor == "" {
			t.Fatalf("лента группы, страница %d: пустой курсор", page)
		}
		cursor = *got.NextCursor
	}

	return ids
}

// gpIDs — идентификаторы постов страницы по порядку.
func gpIDs(page gpFeed) []string {
	ids := []string{}
	if page.Items == nil {
		return ids
	}
	for _, item := range *page.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

// gpFind — пост с этим id на странице или nil.
func gpFind(page gpFeed, postID string) *gpPost {
	if page.Items == nil {
		return nil
	}
	for i := range *page.Items {
		if (*page.Items)[i].ID == postID {
			return &(*page.Items)[i]
		}
	}
	return nil
}

// gpCount — сколько раз пост стоит на странице.
func gpCount(page gpFeed, postID string) int {
	n := 0
	for _, id := range gpIDs(page) {
		if id == postID {
			n++
		}
	}
	return n
}

// gpRequireIDs требует ровно эти посты в этом порядке.
func gpRequireIDs(t *testing.T, got, want []string, where string) {
	t.Helper()

	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s: посты %q, ожидалось %q", where, got, want)
	}
}

// gpRequireOnce требует, чтобы пост стоял на странице ровно один раз,
// и возвращает его.
func gpRequireOnce(t *testing.T, page gpFeed, postID, where string) *gpPost {
	t.Helper()

	if n := gpCount(page, postID); n != 1 {
		t.Errorf("%s: пост %s стоит %d раз, ожидался ровно один", where, postID, n)
	}

	return gpFind(page, postID)
}

// gpRequireAbsent требует, чтобы поста на странице не было.
func gpRequireAbsent(t *testing.T, page gpFeed, postID, where string) {
	t.Helper()

	if gpFind(page, postID) != nil {
		t.Errorf("%s: пост %s есть, а его быть не должно", where, postID)
	}
}

// --- Пост в группах: создание (требования 1, 2, 4) -------------------------

// Нет поля, null или пустой массив — пост ни в какой группе: groups — пустой
// массив во всех ответах (требования 1, 4).
func TestGroupPostsWithoutGroups(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	reader := newGpUser(t, baseURL, 2)

	// Группа у автора есть, но пост в неё не выкладывается.
	g := grInterestGroup(t, baseURL, author, "Огородники-без")

	cases := []struct {
		name  string
		extra map[string]any
	}{
		{"поля нет", map[string]any{}},
		{"null", map[string]any{"group_ids": nil}},
		{"пустой массив", map[string]any{"group_ids": []string{}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, _ := gpPublishReq(t, baseURL, author.token, c.extra)
			post := gpPostOK(t, resp, http.StatusCreated, "создание: "+c.name)
			gpRequireGroups(t, post, nil, "ответ на создание")

			gpRequireGroups(t, gpGet(t, baseURL, author, post.ID), nil, "GET /posts/{id} автором")
			gpRequireGroups(t, gpGet(t, baseURL, reader, post.ID), nil, "GET /posts/{id} читателем")

			for _, view := range []struct {
				name string
				page gpFeed
			}{
				{"лента «Все»", gpAll(t, baseURL, reader)},
				{"«Подписки» автора", gpFollowing(t, baseURL, author)},
				{"посты в профиле", gpUserPosts(t, baseURL, reader, author.id)},
			} {
				if got := gpFind(view.page, post.ID); got == nil {
					t.Errorf("%s: поста нет", view.name)
				} else {
					gpRequireGroups(t, *got, nil, view.name)
				}
			}

			gpRequireAbsent(t, gpGroupFeed(t, baseURL, author, g.ID), post.ID, "лента группы, куда пост не выкладывали")
		})
	}
}

// Пост в нескольких группах: groups — по названию без учёта регистра,
// повторы в group_ids схлопываются; одно и то же во всех ответах и у всех,
// кому группы видны (требования 1, 2, 4). Выложить можно и как хозяин,
// и как участник.
func TestGroupPostsInSeveralGroupsSortedAndDeduplicated(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	host := newGpUser(t, baseURL, 2)
	reader := newGpUser(t, baseURL, 3)

	// Без учёта регистра: «Акации» < «бузина» < «Вишни»; побайтово
	// заглавные шли бы раньше строчных.
	cherries := grInterestGroup(t, baseURL, author, "Вишни")
	elder := grInterestGroup(t, baseURL, host, "бузина")
	acacia := grInterestGroup(t, baseURL, host, "Акации")
	grJoin(t, baseURL, author, elder.ID, grMember)
	grJoin(t, baseURL, author, acacia.ID, grMember)

	want := []gpBrief{gpBriefOf(acacia), gpBriefOf(elder), gpBriefOf(cherries)}

	resp, _ := gpPublishReq(t, baseURL, author.token, map[string]any{
		"group_ids": []string{cherries.ID, elder.ID, cherries.ID, acacia.ID, elder.ID},
	})
	post := gpPostOK(t, resp, http.StatusCreated, "пост в трёх группах с повторами")
	gpRequireGroups(t, post, want, "ответ на создание")

	gpRequireGroups(t, gpGet(t, baseURL, author, post.ID), want, "GET /posts/{id} автором")
	gpRequireGroups(t, gpGet(t, baseURL, reader, post.ID), want, "GET /posts/{id} посторонним")

	views := []struct {
		name string
		page gpFeed
	}{
		{"лента «Все» постороннего", gpAll(t, baseURL, reader)},
		{"«Подписки» автора", gpFollowing(t, baseURL, author)},
		{"«Подписки» хозяина групп", gpFollowing(t, baseURL, host)},
		{"посты в профиле", gpUserPosts(t, baseURL, reader, author.id)},
	}
	for _, g := range []grGroup{cherries, elder, acacia} {
		views = append(views, struct {
			name string
			page gpFeed
		}{"лента группы " + g.Name, gpGroupFeed(t, baseURL, reader, g.ID)})
	}

	for _, view := range views {
		got := gpRequireOnce(t, view.page, post.ID, view.name)
		if got != nil {
			gpRequireGroups(t, *got, want, view.name)
		}
	}
}

// Ответы на правки поста тоже несут groups, и правки группы не меняют
// (требования 4, 5).
func TestGroupPostsGroupsInEditResponsesAndUnchanged(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)

	g := grInterestGroup(t, baseURL, author, "Пруд")
	post := gpPublish(t, baseURL, author, "", g.ID)
	want := []gpBrief{gpBriefOf(g)}

	gpRequireGroups(t, gpPostOK(t, editCaptionText(t, baseURL, author.token, post.ID, "На пруду клюёт"),
		http.StatusOK, "правка подписи"), want, "ответ на правку подписи")
	gpRequireGroups(t, gpPostOK(t, ptSetTagsReq(t, baseURL, author.token, post.ID, map[string]any{"tags": []string{"рыбалка"}}),
		http.StatusOK, "правка тэгов"), want, "ответ на правку тэгов")
	gpRequireGroups(t, gpPostOK(t, setVisibility(t, baseURL, author.token, post.ID, map[string]any{"visibility": visibilityFriends}),
		http.StatusOK, "правка видимости"), want, "ответ на правку видимости")

	gpRequireGroups(t, gpGet(t, baseURL, author, post.ID), want, "после правок")
	gpRequireOnce(t, gpGroupFeed(t, baseURL, author, g.ID), post.ID, "лента группы после правок")
}

// Выложить можно только в группы, где автор — участник или хозяин. Иначе —
// 403 not_group_member, и пост не создаётся: фотография остаётся
// неопубликованной, у автора нет постов, в ленте группы пусто
// (требование 2).
func TestGroupPostsRejectsGroupsWhereAuthorIsNotMember(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	host := newGpUser(t, baseURL, 2)
	reader := newGpUser(t, baseURL, 3)

	own := grInterestGroup(t, baseURL, author, "Своя")
	foreign := grInterestGroup(t, baseURL, host, "Чужая")
	invited := grInterestGroup(t, baseURL, host, "Пригласили")
	grInvite(t, baseURL, host, invited.ID, author.id)
	grRequireMembership(t, baseURL, author, invited.ID, grInvited, "приглашение")

	// Группа, хозяин которой заблокировал автора: автор в ней участник,
	// но группа ему не видна (029, требование 16).
	hidden := grInterestGroup(t, baseURL, host, "Скрытая")
	grJoin(t, baseURL, author, hidden.ID, grMember)
	blockOK(t, baseURL, host, author.id)

	cases := []struct {
		name   string
		groups []string
	}{
		{"не участник", []string{foreign.ID}},
		{"только приглашён", []string{invited.ID}},
		{"группы нет", []string{grNoGroup}},
		{"не UUID", []string{gpNotUUID}},
		{"группа не видна автору", []string{hidden.ID}},
		{"одна своя, одна чужая", []string{own.ID, foreign.ID}},
		{"чужая первой, своя второй", []string{foreign.ID, own.ID}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, photo := gpPublishReq(t, baseURL, author.token, map[string]any{"group_ids": c.groups})
			grRequireCode(t, resp, http.StatusForbidden, "not_group_member", c.name)

			if n := len(*gpUserPosts(t, baseURL, author, author.id).Items); n != 0 {
				t.Errorf("%s: после отказа у автора постов %d, ожидалось 0", c.name, n)
			}

			// Фотография не ушла в пост: её можно выложить заново.
			retry := gpPostOK(t, gpPublishWith(t, baseURL, author.token, photo.ID, nil), http.StatusCreated,
				c.name+": та же фотография без групп")
			gpRequireGroups(t, retry, nil, c.name+": повтор без групп")
			requireDeleted(t, deletePost(t, baseURL, author.token, retry.ID))
		})
	}

	for _, g := range []grGroup{own, foreign, invited} {
		gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, reader, g.ID)), nil, "лента группы "+g.Name+" после отказов")
	}

	// Свою группу без чужих — можно.
	post := gpPublish(t, baseURL, author, "", own.ID)
	gpRequireGroups(t, post, []gpBrief{gpBriefOf(own)}, "своя группа")
}

// Сообщение not_group_member — из спецификации.
func TestGroupPostsNotGroupMemberMessage(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	host := newGpUser(t, baseURL, 2)
	foreign := grInterestGroup(t, baseURL, host, "Чужая-текст")

	resp, _ := gpPublishReq(t, baseURL, author.token, map[string]any{"group_ids": []string{foreign.ID}})
	grStatus(t, resp, http.StatusForbidden, "чужая группа")

	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decode(t, resp, &body)
	if body.Code != "not_group_member" {
		t.Errorf("код %q, ожидался not_group_member", body.Code)
	}
	if body.Message != "Выложить можно только в свою группу" {
		t.Errorf("сообщение %q, ожидалось «Выложить можно только в свою группу»", body.Message)
	}
}

// Группы проверяются после тэгов и до места (требование 2): плохой тэг
// и чужая группа — invalid_tag; чужая группа и неизвестное место —
// not_group_member.
func TestGroupPostsCheckOrderOnCreate(t *testing.T) {
	baseURL, _ := startPlaces(t)
	author := newGpUser(t, baseURL, 1)
	host := newGpUser(t, baseURL, 2)
	foreign := grInterestGroup(t, baseURL, host, "Чужая-порядок")

	cases := []struct {
		name   string
		extra  map[string]any
		status int
		code   string
	}{
		{
			"длинная подпись и чужая группа",
			map[string]any{"caption": strings.Repeat("я", 1001), "group_ids": []string{foreign.ID}},
			http.StatusBadRequest, "invalid_caption",
		},
		{
			"плохой тэг и чужая группа",
			map[string]any{"tags": []string{"зелёный лук"}, "group_ids": []string{foreign.ID}},
			http.StatusBadRequest, "invalid_tag",
		},
		{
			"одиннадцать тэгов и чужая группа",
			map[string]any{"tags": ptManyTags(11), "group_ids": []string{foreign.ID}},
			http.StatusBadRequest, "too_many_tags",
		},
		{
			"чужая группа и неизвестное место",
			map[string]any{"place_id": unknownID, "group_ids": []string{foreign.ID}},
			http.StatusForbidden, "not_group_member",
		},
		{
			"несуществующая группа и неизвестное место",
			map[string]any{"place_id": unknownID, "group_ids": []string{grNoGroup}},
			http.StatusForbidden, "not_group_member",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, _ := gpPublishReq(t, baseURL, author.token, c.extra)
			grRequireCode(t, resp, c.status, c.code, c.name)
		})
	}
}

// Сценарий спецификации: пост в геогруппе «снт Ромашка» видят в
// «Подписках» соседи, которые на автора не подписаны; в ленте группы он
// сверху; группа по интересам автора, куда пост не выкладывали, его не
// получает (требования 2, 4, 6, 9).
func TestGroupPostsGeoGroupScenario(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	nikolay := newGpUser(t, baseURL, 1)
	galina := newGpUser(t, baseURL, 2)
	sergey := newGpUser(t, baseURL, 3)
	fisher := newGpUser(t, baseURL, 4)

	p := grLearnPlain(t, baseURL, fake, nikolay.token, "снт Ромашка")
	grSettle(t, baseURL, nikolay, p)
	grSettle(t, baseURL, galina, p)
	grSettle(t, baseURL, sergey, p)
	geo := grGeoOf(t, baseURL, nikolay, p)

	fishing := grInterestGroup(t, baseURL, fisher, "Любители рыбалки")
	grJoin(t, baseURL, nikolay, fishing.ID, grMember)

	older := gpPublish(t, baseURL, galina, "", geo.ID)
	post := gpPublish(t, baseURL, nikolay, "", geo.ID)
	want := []gpBrief{{ID: geo.ID, Name: "снт Ромашка"}}
	gpRequireGroups(t, post, want, "ответ на создание")

	for _, who := range []struct {
		name string
		d    dachnik
	}{{"Галина", galina}, {"Сергей", sergey}} {
		got := gpRequireOnce(t, gpFollowing(t, baseURL, who.d), post.ID, "«Подписки»: "+who.name)
		if got != nil {
			gpRequireGroups(t, *got, want, "«Подписки»: "+who.name)
		}
	}
	gpRequireAbsent(t, gpFollowing(t, baseURL, fisher), post.ID, "«Подписки» хозяина группы, куда пост не выкладывали")

	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, galina, geo.ID)), []string{post.ID, older.ID}, "лента снт Ромашка")
	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, fisher, fishing.ID)), nil, "лента «Любителей рыбалки»")
}

// --- Видимость (требование 3, edge cases) ---------------------------------

// Группа видимость не расширяет: пост «Только мне» в группе видит только
// автор; пост «Друзьям» участник-не-друг не видит ни в ленте группы, ни в
// «Подписках», а друг-участник видит (требования 3, 6, 9).
func TestGroupPostsRespectVisibility(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	friend := newGpUser(t, baseURL, 2)
	neighbour := newGpUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, author, "Ромашка-видимость")
	grJoin(t, baseURL, friend, g.ID, grMember)
	grJoin(t, baseURL, neighbour, g.ID, grMember)
	makeFriends(t, baseURL, author, friend)

	mine := gpPublish(t, baseURL, author, visibilityMe, g.ID)
	friends := gpPublish(t, baseURL, author, visibilityFriends, g.ID)
	open := gpPublish(t, baseURL, author, visibilityAll, g.ID)

	// Автор видит всё своё, в том числе в ленте группы.
	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, author, g.ID)), []string{open.ID, friends.ID, mine.ID}, "лента группы автора")
	gpRequireGroups(t, gpGet(t, baseURL, author, mine.ID), []gpBrief{gpBriefOf(g)}, "свой пост «Только мне»")

	// Друг — «Друзьям» и «Всем».
	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, friend, g.ID)), []string{open.ID, friends.ID}, "лента группы друга")
	following := gpFollowing(t, baseURL, friend)
	gpRequireAbsent(t, following, mine.ID, "«Подписки» друга, пост «Только мне»")
	gpRequireOnce(t, following, friends.ID, "«Подписки» друга, пост «Друзьям»")

	// Сосед по группе, не друг, — только «Всем».
	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, neighbour, g.ID)), []string{open.ID}, "лента группы соседа")
	following = gpFollowing(t, baseURL, neighbour)
	gpRequireAbsent(t, following, mine.ID, "«Подписки» соседа, пост «Только мне»")
	gpRequireAbsent(t, following, friends.ID, "«Подписки» соседа, пост «Друзьям»")
	gpRequireOnce(t, following, open.ID, "«Подписки» соседа, пост «Всем»")
	requireNotFound(t, fetchPost(t, baseURL, neighbour.token, friends.ID), "GET /posts/{id}", "сосед и пост «Друзьям»")
	requireNotFound(t, fetchPost(t, baseURL, neighbour.token, mine.ID), "GET /posts/{id}", "сосед и пост «Только мне»")
}

// Блокировки (022) действуют и на посты групп: автор заблокировал
// участника — его посты пропадают у того и из «Подписок», и из ленты
// группы (требования 6, 9).
func TestGroupPostsRespectBlocks(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	member := newGpUser(t, baseURL, 2)
	host := newGpUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, host, "Ромашка-блок")
	grJoin(t, baseURL, author, g.ID, grMember)
	grJoin(t, baseURL, member, g.ID, grMember)

	post := gpPublish(t, baseURL, author, "", g.ID)
	gpRequireOnce(t, gpFollowing(t, baseURL, member), post.ID, "«Подписки» до блокировки")

	blockOK(t, baseURL, author, member.id)

	gpRequireAbsent(t, gpFollowing(t, baseURL, member), post.ID, "«Подписки» заблокированного")
	gpRequireAbsent(t, gpGroupFeed(t, baseURL, member, g.ID), post.ID, "лента группы у заблокированного")
	gpRequireOnce(t, gpGroupFeed(t, baseURL, host, g.ID), post.ID, "лента группы у хозяина")
}

// Группа, хозяин которой заблокировал смотрящего, ему не видна: в groups
// поста её нет, а сам пост виден, если виден по 013 и 022; её лента —
// 404 (требования 4, 10, edge cases).
func TestGroupPostsGroupHiddenByOwnerBlockIsNotShown(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	host := newGpUser(t, baseURL, 2)
	viewer := newGpUser(t, baseURL, 3)

	hidden := grInterestGroup(t, baseURL, host, "Ромашка-скрытая")
	open := grInterestGroup(t, baseURL, author, "Ромашка-открытая")
	grJoin(t, baseURL, author, hidden.ID, grMember)
	blockOK(t, baseURL, host, viewer.id)

	both := gpPublish(t, baseURL, author, "", hidden.ID, open.ID)
	only := gpPublish(t, baseURL, author, "", hidden.ID)

	// Автор видит обе группы.
	gpRequireGroups(t, both, []gpBrief{gpBriefOf(open), gpBriefOf(hidden)}, "ответ автору")

	// Смотрящий — только видимую.
	gpRequireGroups(t, gpGet(t, baseURL, viewer, both.ID), []gpBrief{gpBriefOf(open)}, "GET /posts/{id}, две группы")
	gpRequireGroups(t, gpGet(t, baseURL, viewer, only.ID), nil, "GET /posts/{id}, только скрытая группа")

	all := gpAll(t, baseURL, viewer)
	if got := gpRequireOnce(t, all, both.ID, "«Все», две группы"); got != nil {
		gpRequireGroups(t, *got, []gpBrief{gpBriefOf(open)}, "«Все», две группы")
	}
	if got := gpRequireOnce(t, all, only.ID, "«Все», только скрытая группа"); got != nil {
		gpRequireGroups(t, *got, nil, "«Все», только скрытая группа")
	}
	profile := gpUserPosts(t, baseURL, viewer, author.id)
	if got := gpFind(profile, both.ID); got == nil {
		t.Error("посты в профиле: поста нет")
	} else {
		gpRequireGroups(t, *got, []gpBrief{gpBriefOf(open)}, "посты в профиле")
	}
	if got := gpFind(gpGroupFeed(t, baseURL, viewer, open.ID), both.ID); got == nil {
		t.Error("лента видимой группы: поста нет")
	} else {
		gpRequireGroups(t, *got, []gpBrief{gpBriefOf(open)}, "лента видимой группы")
	}

	grRequireCode(t, gpGroupPostsReq(t, baseURL, viewer.token, hidden.ID, ""), http.StatusNotFound, "group_not_found",
		"лента скрытой группы")
}

// --- «Подписки» (требования 6–8) ------------------------------------------

// «Подписки» отдают посты групп, где смотрящий участник или хозяин, —
// без подписки на автора; приглашённому — нет, пока не вступит. Пост в двух
// группах смотрящего — один раз. Лента «Все» не меняется.
func TestGroupPostsInFollowingFeed(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	viewer := newGpUser(t, baseURL, 2)
	host := newGpUser(t, baseURL, 3)
	stranger := newGpUser(t, baseURL, 4)

	asMember := grInterestGroup(t, baseURL, host, "Где участник")
	asOwner := grInterestGroup(t, baseURL, viewer, "Где хозяин")
	asInvited := grInterestGroup(t, baseURL, host, "Куда пригласили")
	grJoin(t, baseURL, viewer, asMember.ID, grMember)
	grInvite(t, baseURL, host, asInvited.ID, viewer.id)
	for _, g := range []grGroup{asMember, asOwner, asInvited} {
		grJoin(t, baseURL, author, g.ID, grMember)
	}

	inMember := gpPublish(t, baseURL, author, "", asMember.ID)
	inOwner := gpPublish(t, baseURL, author, "", asOwner.ID)
	inBoth := gpPublish(t, baseURL, author, "", asMember.ID, asOwner.ID)
	inInvited := gpPublish(t, baseURL, author, "", asInvited.ID)
	noGroup := gpPublish(t, baseURL, author, "")

	gpRequireIDs(t, gpIDs(gpFollowing(t, baseURL, viewer)), []string{inBoth.ID, inOwner.ID, inMember.ID},
		"«Подписки»: посты своих групп, без дублей, новые сверху")

	// Страницами по одному — тоже без дублей и пропусков.
	gpRequireIDs(t, walkScopeFeed(t, baseURL, viewer.token, "following", 1), []string{inBoth.ID, inOwner.ID, inMember.ID},
		"«Подписки» страницами по одному")

	// «Все» — все видимые посты, как и раньше, у участника и у постороннего.
	wantAll := []string{noGroup.ID, inInvited.ID, inBoth.ID, inOwner.ID, inMember.ID}
	gpRequireIDs(t, gpIDs(gpAll(t, baseURL, viewer)), wantAll, "«Все» у участника")
	gpRequireIDs(t, gpIDs(gpAll(t, baseURL, stranger)), wantAll, "«Все» у постороннего")
	gpRequireIDs(t, gpIDs(gpFollowing(t, baseURL, stranger)), nil, "«Подписки» постороннего")

	// Принял приглашение — посты группы приходят.
	grJoin(t, baseURL, viewer, asInvited.ID, grMember)
	gpRequireIDs(t, gpIDs(gpFollowing(t, baseURL, viewer)), []string{inInvited.ID, inBoth.ID, inOwner.ID, inMember.ID},
		"«Подписки» после вступления")

	// Подписка на автора не задваивает пост группы.
	followOK(t, baseURL, viewer, author.id)
	following := gpFollowing(t, baseURL, viewer)
	for _, id := range []string{inInvited.ID, inBoth.ID, inOwner.ID, inMember.ID, noGroup.ID} {
		gpRequireOnce(t, following, id, "«Подписки» с подпиской на автора")
	}
}

// Вышел смотрящий из группы — её посты в его «Подписки» больше не приходят
// (требование 6: только группы, где он участник).
func TestGroupPostsLeaveStopsFollowing(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	viewer := newGpUser(t, baseURL, 2)

	g := grInterestGroup(t, baseURL, author, "Ромашка-выход-читателя")
	grJoin(t, baseURL, viewer, g.ID, grMember)
	post := gpPublish(t, baseURL, author, "", g.ID)
	gpRequireOnce(t, gpFollowing(t, baseURL, viewer), post.ID, "«Подписки» участника")

	grLeave(t, baseURL, viewer, g.ID)
	gpRequireAbsent(t, gpFollowing(t, baseURL, viewer), post.ID, "«Подписки» вышедшего")
	gpRequireOnce(t, gpGroupFeed(t, baseURL, viewer, g.ID), post.ID, "лента группы вышедшего")
}

// --- Лента группы (требования 9–11) ---------------------------------------

// Лента группы: только посты группы, новые сверху; видят и не участники
// (требования 9, 10).
func TestGroupPostsGroupFeedOrder(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	member := newGpUser(t, baseURL, 2)
	outsider := newGpUser(t, baseURL, 3)
	invited := newGpUser(t, baseURL, 4)

	g := grInterestGroup(t, baseURL, author, "Ромашка-лента")
	other := grInterestGroup(t, baseURL, author, "Другая-лента")
	grJoin(t, baseURL, member, g.ID, grMember)
	grInvite(t, baseURL, author, g.ID, invited.id)

	var want []string
	want = append(want, gpPublish(t, baseURL, author, "", g.ID).ID)
	gpPublish(t, baseURL, author, "")
	want = append(want, gpPublish(t, baseURL, member, "", g.ID).ID)
	gpPublish(t, baseURL, author, "", other.ID)
	want = append(want, gpPublish(t, baseURL, author, "", g.ID, other.ID).ID)
	gpPublish(t, baseURL, outsider, "")
	slices.Reverse(want)

	for _, who := range []struct {
		name string
		d    dachnik
	}{{"хозяин", author}, {"участник", member}, {"не участник", outsider}, {"приглашённый", invited}} {
		page := gpGroupFeed(t, baseURL, who.d, g.ID)
		gpRequireIDs(t, gpIDs(page), want, "лента группы: "+who.name)
		if page.NextCursor != nil {
			t.Errorf("лента группы (%s): постов меньше limit, а курсор есть: %q", who.name, *page.NextCursor)
		}
	}

	// Новая группа — пустая лента без курсора.
	empty := grInterestGroup(t, baseURL, outsider, "Пустая-лента")
	page := gpGroupFeed(t, baseURL, member, empty.ID)
	gpRequireIDs(t, gpIDs(page), nil, "лента новой группы")
	if page.NextCursor != nil {
		t.Errorf("лента новой группы: курсор %q", *page.NextCursor)
	}
}

// Курсор и limit — как у ленты: без limit — двадцать новых; страницы по
// limit проходят ленту целиком без дублей и пропусков; новый пост между
// страницами не сдвигает вторую (требование 9).
func TestGroupPostsGroupFeedPaging(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	reader := newGpUser(t, baseURL, 2)

	g := grInterestGroup(t, baseURL, author, "Ромашка-страницы")

	var published []string
	for i := 0; i < 23; i++ {
		published = append(published, gpPublish(t, baseURL, author, "", g.ID).ID)
		if i%5 == 0 {
			gpPublish(t, baseURL, author, "") // посты вне группы между ними
		}
	}
	newest := slices.Clone(published)
	slices.Reverse(newest)

	// Без limit — двадцать.
	def := gpGroupPage(t, baseURL, reader, g.ID, url.Values{})
	gpRequireIDs(t, gpIDs(def), newest[:20], "без limit")
	if def.NextCursor == nil {
		t.Error("без limit: постов больше двадцати, а курсора нет")
	}

	// По limit 5 — все 23 по порядку.
	gpRequireIDs(t, gpWalkGroup(t, baseURL, reader, g.ID, 5), newest, "страницами по 5")
	// limit 1 и 50.
	gpRequireIDs(t, gpWalkGroup(t, baseURL, reader, g.ID, 1), newest, "страницами по 1")
	gpRequireIDs(t, gpIDs(gpGroupPage(t, baseURL, reader, g.ID, feedParams(50, ""))), newest, "limit 50")

	// Новый пост между страницами не сдвигает вторую.
	page1 := gpGroupPage(t, baseURL, reader, g.ID, feedParams(10, ""))
	if page1.NextCursor == nil {
		t.Fatal("первая страница по 10: нет курсора")
	}
	fresh := gpPublish(t, baseURL, author, "", g.ID)
	page2 := gpGroupPage(t, baseURL, reader, g.ID, feedParams(10, *page1.NextCursor))
	gpRequireIDs(t, gpIDs(page2), newest[10:20], "вторая страница после нового поста")
	gpRequireAbsent(t, page2, fresh.ID, "вторая страница")

	// Пустой cursor= — первая страница.
	resp := gpGroupPostsReq(t, baseURL, reader.token, g.ID, "limit=3&cursor=")
	gpRequireIDs(t, gpIDs(gpFeedOK(t, resp, "пустой курсор")), []string{fresh.ID, newest[0], newest[1]}, "пустой курсор")
}

// Скрытые смотрящему посты не укорачивают страницы ленты группы: курсор
// проходит её целиком (требование 9).
func TestGroupPostsGroupFeedHiddenPostsDoNotBreakPaging(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	reader := newGpUser(t, baseURL, 2)

	g := grInterestGroup(t, baseURL, author, "Ромашка-скрытые")

	var visible []string
	for i := 0; i < 6; i++ {
		gpPublish(t, baseURL, author, visibilityMe, g.ID)
		visible = append(visible, gpPublish(t, baseURL, author, "", g.ID).ID)
	}
	slices.Reverse(visible)

	gpRequireIDs(t, gpWalkGroup(t, baseURL, reader, g.ID, 2), visible, "страницами по 2 без скрытых постов")
}

// Группы нет, она не видна или id не UUID — 404 group_not_found; неверный
// limit — 400 invalid_request; курсор не разбирается — 400 invalid_cursor;
// без токена — 401 (требования 10, 11).
func TestGroupPostsGroupFeedErrors(t *testing.T) {
	baseURL := startAPI(t)
	host := newGpUser(t, baseURL, 1)
	reader := newGpUser(t, baseURL, 2)
	blocked := newGpUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, host, "Ромашка-ошибки")
	for i := 0; i < 3; i++ {
		gpPublish(t, baseURL, host, "", g.ID)
	}
	blockOK(t, baseURL, host, blocked.id)

	notFound := []struct {
		name  string
		who   dachnik
		group string
	}{
		{"группы нет", reader, grNoGroup},
		{"не UUID", reader, gpNotUUID},
		{"группа скрыта блокировкой хозяина", blocked, g.ID},
	}
	for _, c := range notFound {
		t.Run(c.name, func(t *testing.T) {
			grRequireCode(t, gpGroupPostsReq(t, baseURL, c.who.token, c.group, ""), http.StatusNotFound, "group_not_found", c.name)
		})
	}

	// Удалённая группа — тоже 404.
	deleted := grInterestGroup(t, baseURL, host, "Ромашка-удалённая")
	requireEmpty204(t, grDeleteReq(t, baseURL, host.token, deleted.ID), "удаление группы")
	grRequireCode(t, gpGroupPostsReq(t, baseURL, reader.token, deleted.ID, ""), http.StatusNotFound, "group_not_found", "удалённая группа")

	for _, c := range []struct{ name, query string }{
		{"limit больше пятидесяти", "limit=51"},
		{"limit ноль", "limit=0"},
		{"limit отрицательный", "limit=-1"},
		{"limit не число", "limit=abc"},
	} {
		t.Run(c.name, func(t *testing.T) {
			grRequireCode(t, gpGroupPostsReq(t, baseURL, reader.token, g.ID, c.query), http.StatusBadRequest, "invalid_request", c.name)
		})
	}

	issued := gpGroupPage(t, baseURL, reader, g.ID, feedParams(1, ""))
	if issued.NextCursor == nil || len(*issued.NextCursor) < 4 {
		t.Fatalf("первая страница по одному: курсор %v подозрительный, тест собран неправильно", issued.NextCursor)
	}
	cursor := *issued.NextCursor
	for _, c := range []struct{ name, cursor string }{
		{"не курсор вовсе", "курсор"},
		{"мусор в base64", "0L3QtSDQutGD0YDRgdC-0YA="},
		{"обрезанный настоящий курсор", cursor[:len(cursor)-3]},
	} {
		t.Run(c.name, func(t *testing.T) {
			resp := gpGroupPostsReq(t, baseURL, reader.token, g.ID, "cursor="+url.QueryEscape(c.cursor))
			grRequireCode(t, resp, http.StatusBadRequest, "invalid_cursor", c.name)
		})
	}

	requireUnauthorized(t, gpGroupPostsReq(t, baseURL, "", g.ID, ""), "лента группы без токена")
	requireUnauthorized(t, gpGroupPostsReq(t, baseURL, "unknown-token", g.ID, ""), "лента группы с чужим токеном")
	requireUnauthorized(t, gpGroupPostsReq(t, baseURL, "", grNoGroup, ""), "лента несуществующей группы без токена")
}

// --- Кто что может (требования 12–15) -------------------------------------

// Хозяин группы и её участники чужой пост в группе не удаляют —
// 403 not_your_post; автор удаляет, и пост пропадает из ленты группы и из
// «Подписок» участников (требования 12, 15).
func TestGroupPostsOnlyAuthorDeletes(t *testing.T) {
	baseURL := startAPI(t)
	author := newGpUser(t, baseURL, 1)
	host := newGpUser(t, baseURL, 2)
	member := newGpUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, host, "Ромашка-удаление")
	grJoin(t, baseURL, author, g.ID, grMember)
	grJoin(t, baseURL, member, g.ID, grMember)

	post := gpPublish(t, baseURL, author, "", g.ID)
	other := gpPublish(t, baseURL, author, "", g.ID)

	grRequireCode(t, deletePost(t, baseURL, host.token, post.ID), http.StatusForbidden, "not_your_post", "хозяин группы")
	grRequireCode(t, deletePost(t, baseURL, member.token, post.ID), http.StatusForbidden, "not_your_post", "участник группы")
	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, member, g.ID)), []string{other.ID, post.ID}, "после отказов")

	requireDeleted(t, deletePost(t, baseURL, author.token, post.ID))

	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, member, g.ID)), []string{other.ID}, "лента группы после удаления")
	gpRequireAbsent(t, gpFollowing(t, baseURL, member), post.ID, "«Подписки» участника после удаления")
	gpRequireAbsent(t, gpFollowing(t, baseURL, host), post.ID, "«Подписки» хозяина после удаления")
	requirePostGone(t, baseURL, member.token, post.ID, "удалённый пост в группе")
}

// Владелец сервиса удаляет пост в группе из дашборда — пост пропадает из
// ленты группы (требования 12, 15).
func TestGroupPostsDashboardDelete(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})
	author := newGpUser(t, baseURL, 1)
	member := newGpUser(t, baseURL, 2)

	g := grInterestGroup(t, baseURL, author, "Ромашка-дашборд")
	grJoin(t, baseURL, member, g.ID, grMember)

	post := gpPublish(t, baseURL, author, "", g.ID)
	other := gpPublish(t, baseURL, author, "", g.ID)

	modDeleted(t, root, "/dashboard/posts/"+post.ID, "удаление поста в группе владельцем")

	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, member, g.ID)), []string{other.ID}, "лента группы после удаления из дашборда")
	gpRequireAbsent(t, gpFollowing(t, baseURL, member), post.ID, "«Подписки» участника")
	requirePostGone(t, baseURL, member.token, post.ID, "после удаления из дашборда")
}

// Автор вышел из группы или хозяин его убрал — его посты остаются в ленте
// группы, в «Подписках» её участников и в groups поста; нового поста в эту
// группу он уже не выложит (требования 2, 13).
func TestGroupPostsStayAfterAuthorLeavesOrIsRemoved(t *testing.T) {
	baseURL := startAPI(t)
	host := newGpUser(t, baseURL, 1)
	leaver := newGpUser(t, baseURL, 2)
	removed := newGpUser(t, baseURL, 3)
	reader := newGpUser(t, baseURL, 4)

	g := grInterestGroup(t, baseURL, host, "Ромашка-выход")
	for _, d := range []dachnik{leaver, removed, reader} {
		grJoin(t, baseURL, d, g.ID, grMember)
	}

	byLeaver := gpPublish(t, baseURL, leaver, "", g.ID)
	byRemoved := gpPublish(t, baseURL, removed, "", g.ID)

	grLeave(t, baseURL, leaver, g.ID)
	grRemove(t, baseURL, host, g.ID, removed.id)

	want := []gpBrief{gpBriefOf(g)}

	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, reader, g.ID)), []string{byRemoved.ID, byLeaver.ID}, "лента группы")
	following := gpFollowing(t, baseURL, reader)
	for _, c := range []struct {
		name string
		post gpPost
		who  dachnik
	}{{"вышел сам", byLeaver, leaver}, {"убрали", byRemoved, removed}} {
		if got := gpRequireOnce(t, following, c.post.ID, "«Подписки» участника: "+c.name); got != nil {
			gpRequireGroups(t, *got, want, "«Подписки» участника: "+c.name)
		}
		gpRequireGroups(t, gpGet(t, baseURL, reader, c.post.ID), want, "GET /posts/{id} читателем: "+c.name)
		gpRequireGroups(t, gpGet(t, baseURL, c.who, c.post.ID), want, "GET /posts/{id} автором: "+c.name)

		resp, _ := gpPublishReq(t, baseURL, c.who.token, map[string]any{"group_ids": []string{g.ID}})
		grRequireCode(t, resp, http.StatusForbidden, "not_group_member", "новый пост после выхода: "+c.name)
	}
}

// Группу удалили — посты остаются у автора, из groups она пропадает,
// в «Подписки» бывших участников по этой группе посты не приходят; пост ещё
// и в другой группе смотрящего — приходит по ней (требование 14).
func TestGroupPostsStayAfterGroupDeleted(t *testing.T) {
	baseURL := startAPI(t)
	host := newGpUser(t, baseURL, 1)
	author := newGpUser(t, baseURL, 2)
	member := newGpUser(t, baseURL, 3)

	doomed := grInterestGroup(t, baseURL, host, "Ромашка-удаляемая")
	kept := grInterestGroup(t, baseURL, author, "Ромашка-остаётся")
	grJoin(t, baseURL, author, doomed.ID, grMember)
	grJoin(t, baseURL, member, doomed.ID, grMember)
	grJoin(t, baseURL, member, kept.ID, grMember)

	onlyDoomed := gpPublish(t, baseURL, author, "", doomed.ID)
	both := gpPublish(t, baseURL, author, "", doomed.ID, kept.ID)

	before := gpFollowing(t, baseURL, member)
	gpRequireOnce(t, before, onlyDoomed.ID, "«Подписки» до удаления группы")
	gpRequireOnce(t, before, both.ID, "«Подписки» до удаления группы")

	requireEmpty204(t, grDeleteReq(t, baseURL, host.token, doomed.ID), "удаление группы")

	// Посты на месте, без удалённой группы.
	gpRequireGroups(t, gpGet(t, baseURL, author, onlyDoomed.ID), nil, "пост только в удалённой группе")
	gpRequireGroups(t, gpGet(t, baseURL, member, both.ID), []gpBrief{gpBriefOf(kept)}, "пост в двух группах")
	profile := gpUserPosts(t, baseURL, member, author.id)
	gpRequireIDs(t, gpIDs(profile), []string{both.ID, onlyDoomed.ID}, "посты в профиле автора")
	if got := gpFind(profile, onlyDoomed.ID); got != nil {
		gpRequireGroups(t, *got, nil, "посты в профиле")
	}
	if got := gpFind(gpAll(t, baseURL, member), onlyDoomed.ID); got == nil {
		t.Error("«Все»: пост из удалённой группы пропал")
	} else {
		gpRequireGroups(t, *got, nil, "«Все»")
	}

	after := gpFollowing(t, baseURL, member)
	gpRequireAbsent(t, after, onlyDoomed.ID, "«Подписки» бывшего участника")
	if got := gpRequireOnce(t, after, both.ID, "«Подписки»: пост ещё и в другой группе"); got != nil {
		gpRequireGroups(t, *got, []gpBrief{gpBriefOf(kept)}, "«Подписки»: пост ещё и в другой группе")
	}
	gpRequireAbsent(t, gpFollowing(t, baseURL, host), onlyDoomed.ID, "«Подписки» бывшего хозяина")

	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, member, kept.ID)), []string{both.ID}, "лента оставшейся группы")
	grRequireCode(t, gpGroupPostsReq(t, baseURL, member.token, doomed.ID, ""), http.StatusNotFound, "group_not_found",
		"лента удалённой группы")
}

// Автор удалил аккаунт — его посты пропадают и из ленты группы, и из
// «Подписок» участников (edge cases, 022).
func TestGroupPostsGoneWithAuthorAccount(t *testing.T) {
	baseURL := startAPI(t)
	host := newGpUser(t, baseURL, 1)
	author := newGpUser(t, baseURL, 2)

	g := grInterestGroup(t, baseURL, host, "Ромашка-аккаунт")
	grJoin(t, baseURL, author, g.ID, grMember)

	post := gpPublish(t, baseURL, author, "", g.ID)
	own := gpPublish(t, baseURL, host, "", g.ID)

	deleteMeOK(t, baseURL, author.token)

	gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, host, g.ID)), []string{own.ID}, "лента группы после удаления аккаунта автора")
	gpRequireAbsent(t, gpFollowing(t, baseURL, host), post.ID, "«Подписки» хозяина")
}
