package tests

// Тесты по specs/031-group-visibility.md, требования 1–13: видимость
// «только участники группы X» — поле visibility_group_id при публикации,
// visibility_group в ответе, кто видит такой пост и что с ним бывает после
// выхода из группы, её удаления и смены видимости. Написаны по спецификации
// и контракту, без взгляда на реализацию (ADR-0002).
//
// Группы создаются и наполняются хелперами groups_test.go (gr*), посты в
// группах и ленты — хелперами group_posts_test.go (gp*), «невидим нигде» —
// requireHiddenFrom из post_visibility_test.go (013), блокировки — 022.
// Свои хелперы этого файла — с префиксом gv.
//
// Требование 13 (в меню поста нет групп) — про приложение: в контракте
// PUT /posts/{id}/visibility группы не принимает, проверять тут нечего.
// Требования 14–18 (приложение) проверяются не здесь.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// --- Представления из контракта -------------------------------------------

// gvPost — пост с полями этой фичи (schema Post). VisibilityGroup — nil,
// если поля нет или оно null: контракт разрешает оба.
type gvPost struct {
	ID              string
	Visibility      *string
	VisibilityGroup *gpBrief
	Groups          *[]gpBrief
}

// --- Хелперы: люди ---------------------------------------------------------

// gvPhone — номер n-го участника теста; не пересекается с другими файлами.
func gvPhone(n int) string {
	return fmt.Sprintf("+7 (900) 731-00-%02d", n)
}

// newGvUser регистрирует n-го участника теста.
func newGvUser(t *testing.T, baseURL string, n int) dachnik {
	t.Helper()

	token, id := signIn(t, baseURL, gvPhone(n))

	return dachnik{token: token, id: id}
}

// --- Хелперы: разбор поста -------------------------------------------------

// gvParse разбирает один объект Post: id, visibility, groups и
// visibility_group (нет поля и null — одно и то же).
func gvParse(t *testing.T, raw []byte, where string) gvPost {
	t.Helper()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("%s: пост не разобрался как JSON-объект: %v (%s)", where, err, raw)
	}

	var body struct {
		ID         string     `json:"id"`
		Visibility *string    `json:"visibility"`
		Groups     *[]gpBrief `json:"groups"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("%s: пост не разобрался: %v (%s)", where, err, raw)
	}

	post := gvPost{ID: body.ID, Visibility: body.Visibility, Groups: body.Groups}
	if vg, ok := fields["visibility_group"]; ok && strings.TrimSpace(string(vg)) != "null" {
		var brief gpBrief
		if err := json.Unmarshal(vg, &brief); err != nil {
			t.Fatalf("%s: visibility_group не разобрался как GroupBrief: %v (%s)", where, err, vg)
		}
		post.VisibilityGroup = &brief
	}

	return post
}

// gvPostOK требует статус и возвращает пост из ответа.
func gvPostOK(t *testing.T, resp *http.Response, status int, where string) gvPost {
	t.Helper()

	grStatus(t, resp, status, where)

	return gvParse(t, rawJSON(t, resp), where)
}

// gvItems требует 200 и разбирает страницу ленты (schema Feed).
func gvItems(t *testing.T, resp *http.Response, where string) []gvPost {
	t.Helper()

	raw := grItems(t, resp, where)
	out := make([]gvPost, 0, len(raw))
	for _, item := range raw {
		out = append(out, gvParse(t, item, where))
	}

	return out
}

// gvFind — пост с этим id в списке или nil.
func gvFind(items []gvPost, postID string) *gvPost {
	for i := range items {
		if items[i].ID == postID {
			return &items[i]
		}
	}
	return nil
}

// --- Хелперы: проверки поста -----------------------------------------------

// gvRequireVisibility требует у поста ровно эту visibility.
func gvRequireVisibility(t *testing.T, post gvPost, want, where string) {
	t.Helper()

	if post.Visibility == nil {
		t.Errorf("%s: у поста %s нет поля visibility, ожидалось %q", where, post.ID, want)
		return
	}
	if *post.Visibility != want {
		t.Errorf("%s: visibility = %q, ожидалось %q", where, *post.Visibility, want)
	}
}

// gvRequireGroupVis — пост с видимостью группы g: visibility == friends,
// visibility_group == {id, name} группы (требования 10, 11).
func gvRequireGroupVis(t *testing.T, post gvPost, g grGroup, where string) {
	t.Helper()

	gvRequireVisibility(t, post, visibilityFriends, where)
	want := gpBriefOf(g)
	if post.VisibilityGroup == nil {
		t.Errorf("%s: у поста %s нет visibility_group, ожидалось %+v", where, post.ID, want)
	} else if *post.VisibilityGroup != want {
		t.Errorf("%s: visibility_group = %+v, ожидалось %+v", where, *post.VisibilityGroup, want)
	}
}

// gvRequirePlain — пост без видимости группы: visibility_group нет или null,
// visibility — want (требование 11).
func gvRequirePlain(t *testing.T, post gvPost, want, where string) {
	t.Helper()

	gvRequireVisibility(t, post, want, where)
	if post.VisibilityGroup != nil {
		t.Errorf("%s: у поста %s visibility_group = %+v, ожидалось нет или null", where, post.ID, *post.VisibilityGroup)
	}
}

// gvRequireGroups требует у поста ровно эти группы в этом порядке.
func gvRequireGroups(t *testing.T, post gvPost, want []gpBrief, where string) {
	t.Helper()
	gpRequireGroups(t, gpPost{ID: post.ID, Groups: post.Groups}, want, where)
}

// --- Хелперы: публикация ---------------------------------------------------

// gvPublishReq публикует пост с новой фотографией; extra — поля тела сверх
// media_ids как есть. Фотография возвращается для проверки «пост не
// создан».
func gvPublishReq(t *testing.T, baseURL string, who dachnik, extra map[string]any) (*http.Response, mediaPayload) {
	t.Helper()
	return gpPublishReq(t, baseURL, who.token, extra)
}

// gvPublish публикует пост с видимостью группы g так, как это делает
// приложение (visibility: me, требование 3), с полями extra поверх, и
// требует 201 и видимость группы в ответе.
func gvPublish(t *testing.T, baseURL string, who dachnik, g grGroup, extra map[string]any) gvPost {
	t.Helper()

	body := map[string]any{"visibility": visibilityMe, "visibility_group_id": g.ID}
	for k, v := range extra {
		body[k] = v
	}

	resp, _ := gvPublishReq(t, baseURL, who, body)
	where := "публикация с видимостью группы " + g.Name
	post := gvPostOK(t, resp, http.StatusCreated, where)
	gvRequireGroupVis(t, post, g, where)

	return post
}

// gvRequireNotCreated требует, чтобы у автора не появилось постов, а
// фотография осталась неопубликованной: её можно выложить заново.
func gvRequireNotCreated(t *testing.T, baseURL string, author dachnik, photo mediaPayload, where string) {
	t.Helper()

	if items := gvItems(t, fetchUserPosts(t, baseURL, author.token, author.id, feedParams(50, "")), where+": свои посты"); len(items) != 0 {
		t.Errorf("%s: после отказа у автора постов %d, ожидалось 0", where, len(items))
	}

	retry := gvPostOK(t, gpPublishWith(t, baseURL, author.token, photo.ID, nil), http.StatusCreated,
		where+": та же фотография без видимости группы")
	requireDeleted(t, deletePost(t, baseURL, author.token, retry.ID))
}

// --- Хелперы: кто видит ----------------------------------------------------

// gvGet открывает пост по адресу и требует 200.
func gvGet(t *testing.T, baseURL string, who dachnik, postID, where string) gvPost {
	t.Helper()
	return gvPostOK(t, fetchPost(t, baseURL, who.token, postID), http.StatusOK, where+": GET /posts/{id}")
}

// gvInScope требует пост во вкладке ленты и возвращает его.
func gvInScope(t *testing.T, baseURL string, who dachnik, scope, postID, where string) *gvPost {
	t.Helper()

	items := gvItems(t, fetchFeed(t, baseURL, who.token, scopeParams(scope, 50, "")), where+": лента "+scope)
	post := gvFind(items, postID)
	if post == nil {
		t.Errorf("%s: поста %s нет во вкладке %s", where, postID, scope)
	}

	return post
}

// gvInUserPosts требует пост в постах автора и возвращает его.
func gvInUserPosts(t *testing.T, baseURL string, who dachnik, authorID, postID, where string) *gvPost {
	t.Helper()

	items := gvItems(t, fetchUserPosts(t, baseURL, who.token, authorID, feedParams(50, "")), where+": посты автора")
	post := gvFind(items, postID)
	if post == nil {
		t.Errorf("%s: поста %s нет в постах автора", where, postID)
	}

	return post
}

// gvInGroupFeed требует пост в ленте группы и возвращает его.
func gvInGroupFeed(t *testing.T, baseURL string, who dachnik, groupID, postID, where string) *gvPost {
	t.Helper()

	items := gvItems(t, gpGroupPostsReq(t, baseURL, who.token, groupID, feedParams(50, "").Encode()),
		where+": лента группы")
	post := gvFind(items, postID)
	if post == nil {
		t.Errorf("%s: поста %s нет в ленте группы", where, postID)
	}

	return post
}

// gvRequireSees — смотрящий видит пост с видимостью группы g везде: по
// адресу, в обеих вкладках ленты, в постах автора, в ленте g; комментарии
// читаются (требование 6).
func gvRequireSees(t *testing.T, baseURL string, viewer dachnik, authorID, postID string, g grGroup, where string) {
	t.Helper()

	gvRequireGroupVis(t, gvGet(t, baseURL, viewer, postID, where), g, where+": GET /posts/{id}")
	for _, scope := range []string{"all", "following"} {
		if post := gvInScope(t, baseURL, viewer, scope, postID, where); post != nil {
			gvRequireGroupVis(t, *post, g, where+": лента "+scope)
		}
	}
	if post := gvInUserPosts(t, baseURL, viewer, authorID, postID, where); post != nil {
		gvRequireGroupVis(t, *post, g, where+": посты автора")
	}
	if post := gvInGroupFeed(t, baseURL, viewer, g.ID, postID, where); post != nil {
		gvRequireGroupVis(t, *post, g, where+": лента группы")
	}
	if resp := fetchComments(t, baseURL, viewer.token, postID); resp.StatusCode != http.StatusOK {
		t.Errorf("%s: комментарии видимого поста отвечают %d, ожидался 200", where, resp.StatusCode)
	}
}

// gvRequireHidden — смотрящий не видит пост нигде (требования 5, 6): все
// ручки поста — 404 post_not_found, в лентах и постах автора его нет, в
// ленте группы тоже (или сама группа смотрящему не видна — 404).
func gvRequireHidden(t *testing.T, baseURL string, viewer dachnik, authorID, postID, groupID, where string) {
	t.Helper()

	requireHiddenFrom(t, baseURL, viewer, authorID, postID, where)
	requireHiddenPostNotDeletable(t, baseURL, viewer, postID, where)

	resp := gpGroupPostsReq(t, baseURL, viewer.token, groupID, feedParams(50, "").Encode())
	if resp.StatusCode == http.StatusNotFound {
		if code := errorCode(t, resp); code != "group_not_found" {
			t.Errorf("%s: лента группы ответила 404 с кодом %q", where, code)
		}
		return
	}
	if gvFind(gvItems(t, resp, where+": лента группы"), postID) != nil {
		t.Errorf("%s: пост %s стоит в ленте группы, а видеть его нельзя", where, postID)
	}
}

// ============================================================================
// Публикация (требования 1–4, 10, 11)
// ============================================================================

// Участник и хозяин группы выкладывают пост с её видимостью: 201,
// visibility == friends, visibility_group — группа, группа сама в groups;
// так же по адресу и в своих постах (требования 1, 2, 4, 10, 11).
func TestGroupVisibilityCreateByMemberAndOwner(t *testing.T) {
	baseURL := startAPI(t)
	host := newGvUser(t, baseURL, 1)
	member := newGvUser(t, baseURL, 2)

	g := grInterestGroup(t, baseURL, host, "Ромашка-видимость")
	grJoin(t, baseURL, member, g.ID, grMember)

	for _, c := range []struct {
		name string
		who  dachnik
	}{{"хозяин", host}, {"участник", member}} {
		post := gvPublish(t, baseURL, c.who, g, nil)
		gvRequireGroups(t, post, []gpBrief{gpBriefOf(g)}, c.name+": groups в ответе")

		got := gvGet(t, baseURL, c.who, post.ID, c.name)
		gvRequireGroupVis(t, got, g, c.name+": GET /posts/{id}")
		gvRequireGroups(t, got, []gpBrief{gpBriefOf(g)}, c.name+": groups по адресу")

		if own := gvInUserPosts(t, baseURL, c.who, c.who.id, post.ID, c.name); own != nil {
			gvRequireGroupVis(t, *own, g, c.name+": свои посты")
		}
	}
}

// Группа видимости сама добавляется в groups: повтор в group_ids
// схлопывается, другие группы стоят рядом по названию (требование 4).
func TestGroupVisibilityGroupJoinsGroupIDs(t *testing.T) {
	baseURL := startAPI(t)
	author := newGvUser(t, baseURL, 1)

	a := grInterestGroup(t, baseURL, author, "Ромашка-А")
	b := grInterestGroup(t, baseURL, author, "Ромашка-Б")

	same := gvPublish(t, baseURL, author, b, map[string]any{"group_ids": []string{b.ID}})
	gvRequireGroups(t, same, []gpBrief{gpBriefOf(b)}, "та же группа и в group_ids")

	other := gvPublish(t, baseURL, author, b, map[string]any{"group_ids": []string{a.ID}})
	gvRequireGroups(t, other, []gpBrief{gpBriefOf(a), gpBriefOf(b)}, "другая группа в group_ids")

	empty := gvPublish(t, baseURL, author, a, map[string]any{"group_ids": []string{}})
	gvRequireGroups(t, empty, []gpBrief{gpBriefOf(a)}, "пустой group_ids")
}

// visibility_group_id есть — visibility из запроса не учитывается: all,
// friends, me и отсутствие поля дают одну видимость группы; посторонний
// пост не видит (требование 3, edge case «+ all»).
func TestGroupVisibilityIgnoresVisibilityField(t *testing.T) {
	baseURL := startAPI(t)
	author := newGvUser(t, baseURL, 1)
	stranger := newGvUser(t, baseURL, 2)
	friend := newGvUser(t, baseURL, 3)
	makeFriends(t, baseURL, author, friend)

	g := grInterestGroup(t, baseURL, author, "Ромашка-поле")

	for _, visibility := range []string{visibilityAll, visibilityFriends, visibilityMe, ""} {
		where := fmt.Sprintf("visibility %q", visibility)
		body := map[string]any{"visibility_group_id": g.ID}
		if visibility != "" {
			body["visibility"] = visibility
		}

		resp, _ := gvPublishReq(t, baseURL, author, body)
		post := gvPostOK(t, resp, http.StatusCreated, where)
		gvRequireGroupVis(t, post, g, where)

		gvRequireHidden(t, baseURL, stranger, author.id, post.ID, g.ID, where+": посторонний")
		gvRequireHidden(t, baseURL, friend, author.id, post.ID, g.ID, where+": друг не участник")
	}
}

// Неизвестная visibility рядом с группой — 400 invalid_request, как
// раньше; пост не создаётся (требование 3).
func TestGroupVisibilityRejectsUnknownVisibility(t *testing.T) {
	baseURL := startAPI(t)
	author := newGvUser(t, baseURL, 1)
	g := grInterestGroup(t, baseURL, author, "Ромашка-public")

	resp, photo := gvPublishReq(t, baseURL, author, map[string]any{"visibility": "public", "visibility_group_id": g.ID})
	grRequireCode(t, resp, http.StatusBadRequest, "invalid_request", "visibility public с группой")
	gvRequireNotCreated(t, baseURL, author, photo, "visibility public с группой")
}

// Нет поля, null или пустая строка — видимость по visibility, без группы
// (требования 1, 11). У обычных постов visibility_group нет или null везде.
func TestGroupVisibilityEmptyMeansOrdinary(t *testing.T) {
	baseURL := startAPI(t)
	author := newGvUser(t, baseURL, 1)
	friend := newGvUser(t, baseURL, 2)
	makeFriends(t, baseURL, author, friend)

	cases := []struct {
		name       string
		extra      map[string]any
		visibility string
	}{
		{"без поля", map[string]any{}, visibilityAll},
		{"null", map[string]any{"visibility_group_id": nil, "visibility": visibilityAll}, visibilityAll},
		{"пустая строка, friends", map[string]any{"visibility_group_id": "", "visibility": visibilityFriends}, visibilityFriends},
		{"пустая строка, me", map[string]any{"visibility_group_id": "", "visibility": visibilityMe}, visibilityMe},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, _ := gvPublishReq(t, baseURL, author, c.extra)
			post := gvPostOK(t, resp, http.StatusCreated, c.name)
			gvRequirePlain(t, post, c.visibility, c.name+": ответ на публикацию")
			gvRequireGroups(t, post, nil, c.name+": groups")
			gvRequirePlain(t, gvGet(t, baseURL, author, post.ID, c.name), c.visibility, c.name+": GET автором")
			if own := gvInUserPosts(t, baseURL, author, author.id, post.ID, c.name); own != nil {
				gvRequirePlain(t, *own, c.visibility, c.name+": свои посты")
			}

			if c.visibility == visibilityMe {
				requireNotFound(t, fetchPost(t, baseURL, friend.token, post.ID), "GET /posts/{id}", c.name+": друг")
				return
			}
			gvRequirePlain(t, gvGet(t, baseURL, friend, post.ID, c.name), c.visibility, c.name+": GET другом")
			if got := gvInScope(t, baseURL, friend, "all", post.ID, c.name+": друг"); got != nil {
				gvRequirePlain(t, *got, c.visibility, c.name+": «Все» друга")
			}
		})
	}
}

// Группа, где автор не участник, — 403 not_group_member, пост не
// создаётся: чужая, только приглашён, вышел, группы нет, не UUID, группа
// автору не видна (требование 2).
func TestGroupVisibilityRejectsGroupsWhereAuthorIsNotMember(t *testing.T) {
	baseURL := startAPI(t)
	author := newGvUser(t, baseURL, 1)
	host := newGvUser(t, baseURL, 2)

	foreign := grInterestGroup(t, baseURL, host, "Чужая")

	invited := grInterestGroup(t, baseURL, host, "Пригласили")
	grInvite(t, baseURL, host, invited.ID, author.id)
	grRequireMembership(t, baseURL, author, invited.ID, grInvited, "приглашение")

	left := grInterestGroup(t, baseURL, host, "Вышел")
	grJoin(t, baseURL, author, left.ID, grMember)
	grLeave(t, baseURL, author, left.ID)

	removed := grInterestGroup(t, baseURL, host, "Убрали")
	grJoin(t, baseURL, author, removed.ID, grMember)
	grRemove(t, baseURL, host, removed.ID, author.id)

	// Хозяин заблокировал автора: автор участник, но группа ему не видна
	// (029, требование 16).
	blocker := newGvUser(t, baseURL, 3)
	hidden := grInterestGroup(t, baseURL, blocker, "Скрытая")
	grJoin(t, baseURL, author, hidden.ID, grMember)
	blockOK(t, baseURL, blocker, author.id)

	cases := []struct {
		name    string
		groupID string
	}{
		{"не участник", foreign.ID},
		{"только приглашён", invited.ID},
		{"вышел", left.ID},
		{"убрали", removed.ID},
		{"группы нет", grNoGroup},
		{"не UUID", gpNotUUID},
		{"группа не видна автору", hidden.ID},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, photo := gvPublishReq(t, baseURL, author, map[string]any{
				"visibility": visibilityMe, "visibility_group_id": c.groupID,
			})
			grRequireCode(t, resp, http.StatusForbidden, "not_group_member", c.name)
			gvRequireNotCreated(t, baseURL, author, photo, c.name)
		})
	}

	for _, g := range []grGroup{foreign, invited, left, removed} {
		gpRequireIDs(t, gpIDs(gpGroupFeed(t, baseURL, host, g.ID)), nil, "лента группы "+g.Name+" после отказов")
	}
}

// Группа видимости проверяется вместе с group_ids: после подписи, до места
// (требование 2). Тэги — хэштеги подписи и отказом не бывают (028,
// требования 4, 5, 9).
func TestGroupVisibilityCheckOrderOnCreate(t *testing.T) {
	baseURL, _ := startPlaces(t)
	author := newGvUser(t, baseURL, 1)
	host := newGvUser(t, baseURL, 2)
	foreign := grInterestGroup(t, baseURL, host, "Чужая-порядок")
	own := grInterestGroup(t, baseURL, author, "Своя-порядок")

	cases := []struct {
		name   string
		extra  map[string]any
		status int
		code   string
	}{
		{
			"длинная подпись и чужая группа",
			map[string]any{"caption": strings.Repeat("я", 1001), "visibility_group_id": foreign.ID},
			http.StatusBadRequest, "invalid_caption",
		},
		{
			"одиннадцать хэштегов, поле tags и чужая группа",
			map[string]any{
				"caption":             ptCaption(ptManyTags(11)...) + " #---",
				"tags":                []string{"зелёный лук"},
				"visibility_group_id": foreign.ID,
			},
			http.StatusForbidden, "not_group_member",
		},
		{
			"чужая группа и неизвестное место",
			map[string]any{"place_id": unknownID, "visibility_group_id": foreign.ID},
			http.StatusForbidden, "not_group_member",
		},
		{
			"несуществующая группа и неизвестное место",
			map[string]any{"place_id": unknownID, "visibility_group_id": grNoGroup},
			http.StatusForbidden, "not_group_member",
		},
		{
			"своя группа и неизвестное место",
			map[string]any{"place_id": unknownID, "visibility_group_id": own.ID},
			http.StatusBadRequest, "unknown_place",
		},
		{
			"своя группа видимости и чужая в group_ids",
			map[string]any{"visibility_group_id": own.ID, "group_ids": []string{foreign.ID}},
			http.StatusForbidden, "not_group_member",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.extra["visibility"] = visibilityMe
			resp, _ := gvPublishReq(t, baseURL, author, c.extra)
			grRequireCode(t, resp, c.status, c.code, c.name)
		})
	}
}

// ============================================================================
// Кто видит (требования 5–8)
// ============================================================================

// Сценарий спецификации: Николай выкладывает пост «только участники снт
// Ромашка». Галина (соседка по геогруппе) видит его везде, отмечает и
// комментирует; Сергей — взаимный подписчик, не участник — не видит нигде.
// Николай выходит из геогруппы: Галина видит, Николай видит как свой
// (требования 5, 6, 8).
func TestGroupVisibilityGeoGroupScenario(t *testing.T) {
	baseURL, fake := startGrPlaces(t)
	nikolay := newGvUser(t, baseURL, 1)
	galina := newGvUser(t, baseURL, 2)
	sergey := newGvUser(t, baseURL, 3)
	fisher := newGvUser(t, baseURL, 4)

	p := grLearnPlain(t, baseURL, fake, nikolay.token, "снт Ромашка")
	grSettle(t, baseURL, nikolay, p)
	grSettle(t, baseURL, galina, p)
	geo := grGeoOf(t, baseURL, nikolay, p)

	fishing := grInterestGroup(t, baseURL, fisher, "Любители рыбалки")
	grJoin(t, baseURL, nikolay, fishing.ID, grMember)
	makeFriends(t, baseURL, nikolay, sergey)

	post := gvPublish(t, baseURL, nikolay, geo, nil)
	gvRequireGroups(t, post, []gpBrief{gpBriefOf(geo)}, "ответ на создание")

	gvRequireSees(t, baseURL, nikolay, nikolay.id, post.ID, geo, "Николай")
	gvRequireSees(t, baseURL, galina, nikolay.id, post.ID, geo, "Галина")

	grStatus(t, likePost(t, baseURL, galina.token, post.ID), http.StatusOK, "лайк Галины")
	createdComment(t, addCommentText(t, baseURL, galina.token, post.ID, commentText))

	gvRequireHidden(t, baseURL, sergey, nikolay.id, post.ID, geo.ID, "Сергей, взаимный подписчик")
	gvRequireHidden(t, baseURL, fisher, nikolay.id, post.ID, geo.ID, "хозяин другой группы Николая")

	// Николай выходит из «снт Ромашка».
	grLeave(t, baseURL, nikolay, geo.ID)

	gvRequireSees(t, baseURL, galina, nikolay.id, post.ID, geo, "Галина после выхода Николая")

	where := "Николай после выхода"
	gvRequireGroupVis(t, gvGet(t, baseURL, nikolay, post.ID, where), geo, where+": GET /posts/{id}")
	if own := gvInUserPosts(t, baseURL, nikolay, nikolay.id, post.ID, where); own != nil {
		gvRequireGroupVis(t, *own, geo, where+": свои посты")
	}
	for _, scope := range []string{"all", "following"} {
		if got := gvInScope(t, baseURL, nikolay, scope, post.ID, where); got != nil {
			gvRequireGroupVis(t, *got, geo, where+": лента "+scope)
		}
	}
	grStatus(t, likePost(t, baseURL, nikolay.token, post.ID), http.StatusOK, where+": лайк")
	createdComment(t, addCommentText(t, baseURL, nikolay.token, post.ID, "Воду дали"))

	gvRequireHidden(t, baseURL, sergey, nikolay.id, post.ID, geo.ID, "Сергей после выхода Николая")
}

// Пост группы не видят ни друзья, ни подписчики, ни приглашённые, ни
// посторонние; участник видит. Число постов в профиле — по видимости
// (требования 5, 6).
func TestGroupVisibilityHiddenFromNonMembers(t *testing.T) {
	baseURL := startAPI(t)
	author := newGvUser(t, baseURL, 1)
	member := newGvUser(t, baseURL, 2)
	friend := newGvUser(t, baseURL, 3)
	follower := newGvUser(t, baseURL, 4)
	invited := newGvUser(t, baseURL, 5)
	stranger := newGvUser(t, baseURL, 6)

	g := grInterestGroup(t, baseURL, author, "Ромашка-свои")
	grJoin(t, baseURL, member, g.ID, grMember)
	grInvite(t, baseURL, author, g.ID, invited.id)
	makeFriends(t, baseURL, author, friend)
	followOK(t, baseURL, follower, author.id)

	ordinary := gpPublish(t, baseURL, author, visibilityAll)
	post := gvPublish(t, baseURL, author, g, nil)

	gvRequireSees(t, baseURL, member, author.id, post.ID, g, "участник")
	gvRequireSees(t, baseURL, author, author.id, post.ID, g, "автор")

	for _, c := range []struct {
		who    string
		viewer dachnik
	}{
		{"друг", friend},
		{"подписчик", follower},
		{"приглашённый", invited},
		{"посторонний", stranger},
	} {
		gvRequireHidden(t, baseURL, c.viewer, author.id, post.ID, g.ID, c.who)
		requireProfilePosts(t, baseURL, c.viewer, author.id, 1, c.who+": число постов в профиле")

		// Обычный пост автора им виден — значит, дело в видимости группы.
		if got := gvGet(t, baseURL, c.viewer, ordinary.ID, c.who+": обычный пост"); got.VisibilityGroup != nil {
			t.Errorf("%s: у обычного поста visibility_group = %+v", c.who, *got.VisibilityGroup)
		}
	}

	requireProfilePosts(t, baseURL, member, author.id, 2, "участник: число постов в профиле")
	requireProfilePosts(t, baseURL, author, author.id, 2, "автор: число постов в своём профиле")
}

// Видимость считается в момент запроса: вступил — пост появился; вышел,
// убрали — пропал; приглашённый принял приглашение — появился
// (требование 7).
func TestGroupVisibilityFollowsMembershipNow(t *testing.T) {
	baseURL := startAPI(t)
	host := newGvUser(t, baseURL, 1)
	author := newGvUser(t, baseURL, 2)
	reader := newGvUser(t, baseURL, 3)
	guest := newGvUser(t, baseURL, 4)

	g := grInterestGroup(t, baseURL, host, "Ромашка-сейчас")
	grJoin(t, baseURL, author, g.ID, grMember)

	post := gvPublish(t, baseURL, author, g, nil)

	gvRequireHidden(t, baseURL, reader, author.id, post.ID, g.ID, "до вступления")
	requireProfilePosts(t, baseURL, reader, author.id, 0, "до вступления: число постов")

	grJoin(t, baseURL, reader, g.ID, grMember)
	gvRequireSees(t, baseURL, reader, author.id, post.ID, g, "вступил")
	requireProfilePosts(t, baseURL, reader, author.id, 1, "вступил: число постов")

	grLeave(t, baseURL, reader, g.ID)
	gvRequireHidden(t, baseURL, reader, author.id, post.ID, g.ID, "вышел")
	requireProfilePosts(t, baseURL, reader, author.id, 0, "вышел: число постов")

	grJoin(t, baseURL, reader, g.ID, grMember)
	gvRequireSees(t, baseURL, reader, author.id, post.ID, g, "вступил снова")

	grRemove(t, baseURL, host, g.ID, reader.id)
	gvRequireHidden(t, baseURL, reader, author.id, post.ID, g.ID, "убрали")

	// Приглашённый не видит, пока не вступит.
	grInvite(t, baseURL, host, g.ID, guest.id)
	gvRequireHidden(t, baseURL, guest, author.id, post.ID, g.ID, "приглашённый")
	grJoin(t, baseURL, guest, g.ID, grMember)
	gvRequireSees(t, baseURL, guest, author.id, post.ID, g, "принял приглашение")
}

// Автор вышел из группы или его убрали — участники пост видят, автор видит
// его как свой; посторонние по-прежнему нет (требование 8).
func TestGroupVisibilityAfterAuthorLeavesOrIsRemoved(t *testing.T) {
	baseURL := startAPI(t)
	host := newGvUser(t, baseURL, 1)
	leaver := newGvUser(t, baseURL, 2)
	removed := newGvUser(t, baseURL, 3)
	reader := newGvUser(t, baseURL, 4)
	stranger := newGvUser(t, baseURL, 5)

	g := grInterestGroup(t, baseURL, host, "Ромашка-ушёл")
	for _, d := range []dachnik{leaver, removed, reader} {
		grJoin(t, baseURL, d, g.ID, grMember)
	}

	byLeaver := gvPublish(t, baseURL, leaver, g, nil)
	byRemoved := gvPublish(t, baseURL, removed, g, nil)

	grLeave(t, baseURL, leaver, g.ID)
	grRemove(t, baseURL, host, g.ID, removed.id)

	for _, c := range []struct {
		name string
		post gvPost
		who  dachnik
	}{{"вышел сам", byLeaver, leaver}, {"убрали", byRemoved, removed}} {
		gvRequireSees(t, baseURL, reader, c.who.id, c.post.ID, g, "участник, автор "+c.name)
		gvRequireSees(t, baseURL, host, c.who.id, c.post.ID, g, "хозяин, автор "+c.name)

		where := "автор " + c.name
		gvRequireGroupVis(t, gvGet(t, baseURL, c.who, c.post.ID, where), g, where+": GET /posts/{id}")
		if own := gvInUserPosts(t, baseURL, c.who, c.who.id, c.post.ID, where); own != nil {
			gvRequireGroupVis(t, *own, g, where+": свои посты")
		}
		gvInScope(t, baseURL, c.who, "all", c.post.ID, where)
		gvInScope(t, baseURL, c.who, "following", c.post.ID, where)
		requireProfilePosts(t, baseURL, c.who, c.who.id, 1, where+": число постов в своём профиле")

		gvRequireHidden(t, baseURL, stranger, c.who.id, c.post.ID, g.ID, "посторонний, автор "+c.name)
	}
}

// Блокировки (022) действуют как всегда: участник заблокировал автора или
// автор участника — поста нет; хозяин группы заблокировал участника —
// группа тому не видна, пост тоже (требование 6, edge cases).
func TestGroupVisibilityRespectsBlocks(t *testing.T) {
	baseURL := startAPI(t)
	host := newGvUser(t, baseURL, 1)
	author := newGvUser(t, baseURL, 2)
	blocksAuthor := newGvUser(t, baseURL, 3)
	blockedByAuthor := newGvUser(t, baseURL, 4)
	blockedByHost := newGvUser(t, baseURL, 5)
	reader := newGvUser(t, baseURL, 6)

	g := grInterestGroup(t, baseURL, host, "Ромашка-блок")
	for _, d := range []dachnik{author, blocksAuthor, blockedByAuthor, blockedByHost, reader} {
		grJoin(t, baseURL, d, g.ID, grMember)
	}

	post := gvPublish(t, baseURL, author, g, nil)
	for _, d := range []dachnik{blocksAuthor, blockedByAuthor, blockedByHost} {
		gvRequireGroupVis(t, gvGet(t, baseURL, d, post.ID, "до блокировок"), g, "до блокировок")
	}

	blockOK(t, baseURL, blocksAuthor, author.id)
	blockOK(t, baseURL, author, blockedByAuthor.id)
	blockOK(t, baseURL, host, blockedByHost.id)

	for _, c := range []struct {
		who    string
		viewer dachnik
	}{
		{"участник заблокировал автора", blocksAuthor},
		{"автор заблокировал участника", blockedByAuthor},
		{"хозяин группы заблокировал участника", blockedByHost},
	} {
		requireNotFound(t, fetchPost(t, baseURL, c.viewer.token, post.ID), "GET /posts/{id}", c.who)
		requireNotFound(t, likePost(t, baseURL, c.viewer.token, post.ID), "лайк", c.who)
		requireNotFound(t, addCommentText(t, baseURL, c.viewer.token, post.ID, commentText), "комментарий", c.who)
		for _, scope := range []string{"all", "following"} {
			items := gvItems(t, fetchFeed(t, baseURL, c.viewer.token, scopeParams(scope, 50, "")), c.who+": лента "+scope)
			if gvFind(items, post.ID) != nil {
				t.Errorf("%s: пост стоит во вкладке %s", c.who, scope)
			}
		}
	}

	// Хозяин заблокировал не автора — остальным пост виден.
	gvRequireSees(t, baseURL, reader, author.id, post.ID, g, "участник без блокировок")
}

// Закрытый профиль автора не мешает участнику: закрытость касается только
// all. Подписчик закрытого профиля, не участник, пост не видит
// (edge cases).
func TestGroupVisibilityClosedProfileDoesNotHideFromMembers(t *testing.T) {
	baseURL := startAPI(t)
	author := newGvUser(t, baseURL, 1)
	member := newGvUser(t, baseURL, 2)
	follower := newGvUser(t, baseURL, 3)

	g := grInterestGroup(t, baseURL, author, "Ромашка-закрытая")
	grJoin(t, baseURL, member, g.ID, grMember)
	followOK(t, baseURL, follower, author.id)
	setClosed(t, baseURL, author.token, true)

	post := gvPublish(t, baseURL, author, g, nil)

	where := "участник, профиль автора закрыт"
	gvRequireGroupVis(t, gvGet(t, baseURL, member, post.ID, where), g, where+": GET /posts/{id}")
	for _, scope := range []string{"all", "following"} {
		if got := gvInScope(t, baseURL, member, scope, post.ID, where); got != nil {
			gvRequireGroupVis(t, *got, g, where+": лента "+scope)
		}
	}
	if got := gvInGroupFeed(t, baseURL, member, g.ID, post.ID, where); got != nil {
		gvRequireGroupVis(t, *got, g, where+": лента группы")
	}
	grStatus(t, likePost(t, baseURL, member.token, post.ID), http.StatusOK, where+": лайк")
	createdComment(t, addCommentText(t, baseURL, member.token, post.ID, commentText))

	gvRequireHidden(t, baseURL, follower, author.id, post.ID, g.ID, "подписчик закрытого профиля, не участник")
}

// ============================================================================
// Удаление группы (требование 9)
// ============================================================================

// Группу по интересам удалили — пост остаётся у автора «только мне»:
// visibility == me, visibility_group нет, в groups группы нет; бывшие
// участники его не видят.
func TestGroupVisibilityGroupDeletedLeavesPostToAuthor(t *testing.T) {
	baseURL := startAPI(t)
	host := newGvUser(t, baseURL, 1)
	author := newGvUser(t, baseURL, 2)
	member := newGvUser(t, baseURL, 3)
	friend := newGvUser(t, baseURL, 4)
	makeFriends(t, baseURL, author, friend)

	g := grInterestGroup(t, baseURL, host, "Ромашка-удаляемая")
	grJoin(t, baseURL, author, g.ID, grMember)
	grJoin(t, baseURL, member, g.ID, grMember)

	post := gvPublish(t, baseURL, author, g, nil)
	gvRequireSees(t, baseURL, member, author.id, post.ID, g, "участник до удаления")

	requireEmpty204(t, grDeleteReq(t, baseURL, host.token, g.ID), "удаление группы")

	where := "автор после удаления группы"
	got := gvGet(t, baseURL, author, post.ID, where)
	gvRequirePlain(t, got, visibilityMe, where+": GET /posts/{id}")
	gvRequireGroups(t, got, nil, where+": groups")
	if own := gvInUserPosts(t, baseURL, author, author.id, post.ID, where); own != nil {
		gvRequirePlain(t, *own, visibilityMe, where+": свои посты")
	}
	requireProfilePosts(t, baseURL, author, author.id, 1, where+": число постов в своём профиле")

	for _, c := range []struct {
		who    string
		viewer dachnik
	}{
		{"бывший участник", member},
		{"бывший хозяин", host},
		{"друг", friend},
	} {
		gvRequireHidden(t, baseURL, c.viewer, author.id, post.ID, g.ID, c.who+" после удаления группы")
		requireProfilePosts(t, baseURL, c.viewer, author.id, 0, c.who+": число постов в профиле")
	}
}

// ============================================================================
// Смена видимости (требование 12)
// ============================================================================

// PUT visibility all, friends или me снимает видимость группы: дальше пост
// виден по новому значению, visibility_group нет, группа остаётся в groups.
func TestGroupVisibilityChangeRemovesGroup(t *testing.T) {
	baseURL := startAPI(t)
	author := newGvUser(t, baseURL, 1)
	member := newGvUser(t, baseURL, 2)
	friend := newGvUser(t, baseURL, 3)
	stranger := newGvUser(t, baseURL, 4)
	makeFriends(t, baseURL, author, friend)

	g := grInterestGroup(t, baseURL, author, "Ромашка-смена")
	grJoin(t, baseURL, member, g.ID, grMember)
	want := []gpBrief{gpBriefOf(g)}

	for _, visibility := range []string{visibilityAll, visibilityFriends, visibilityMe} {
		where := "смена на " + visibility
		post := gvPublish(t, baseURL, author, g, nil)

		changed := gvPostOK(t, setVisibility(t, baseURL, author.token, post.ID, map[string]any{"visibility": visibility}),
			http.StatusOK, where)
		gvRequirePlain(t, changed, visibility, where+": ответ")
		gvRequireGroups(t, changed, want, where+": groups в ответе")

		got := gvGet(t, baseURL, author, post.ID, where)
		gvRequirePlain(t, got, visibility, where+": GET автором")
		gvRequireGroups(t, got, want, where+": groups по адресу")

		switch visibility {
		case visibilityAll:
			requireVisibleTo(t, baseURL, stranger, author.id, post.ID, visibilityAll, where+": посторонний")
			requireVisibleTo(t, baseURL, friend, author.id, post.ID, visibilityAll, where+": друг")
			requireVisibleTo(t, baseURL, member, author.id, post.ID, visibilityAll, where+": участник")
			if p := gvFind(gvItems(t, fetchFeed(t, baseURL, stranger.token, scopeParams("all", 50, "")), where), post.ID); p != nil {
				gvRequirePlain(t, *p, visibilityAll, where+": «Все» постороннего")
			}
		case visibilityFriends:
			requireVisibleTo(t, baseURL, friend, author.id, post.ID, visibilityFriends, where+": друг не участник")
			gvRequireHidden(t, baseURL, member, author.id, post.ID, g.ID, where+": участник не друг")
			gvRequireHidden(t, baseURL, stranger, author.id, post.ID, g.ID, where+": посторонний")
		case visibilityMe:
			gvRequireHidden(t, baseURL, member, author.id, post.ID, g.ID, where+": участник")
			gvRequireHidden(t, baseURL, friend, author.id, post.ID, g.ID, where+": друг")
		}

		requireDeleted(t, deletePost(t, baseURL, author.token, post.ID))
	}
}
