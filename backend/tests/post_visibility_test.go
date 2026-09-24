package tests

import (
	"fmt"
	"net/http"
	"testing"
)

// Тесты видимости постов (specs/013-post-visibility.md). Написаны по
// спецификации и контракту, не глядя в реализацию (ADR-0002). Понятия
// подписки, друзей и закрытого профиля — из specs/012-follows.md.

// Значения PostVisibility из контракта.
const (
	visibilityAll     = "all"
	visibilityFriends = "friends"
	visibilityMe      = "me"
)

// --- Представления из контракта -------------------------------------------

// visPostPayload — пост с полями, которые важны этой фиче (schema Post).
// visibility — указатель: поле обязательное, его отсутствие — ошибка,
// а не пустая строка.
type visPostPayload struct {
	postPayload
	Visibility *string `json:"visibility"`
	Likes      int     `json:"likes"`
	Liked      bool    `json:"liked"`
	Comments   int     `json:"comments"`
}

// visFeedPayload — страница ленты или постов пользователя (schema Feed),
// разобранная с видимостью у каждого поста.
type visFeedPayload struct {
	Items      []visPostPayload `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

// --- Хелперы --------------------------------------------------------------

// publishWithVisibility публикует пост с одной фотографией и этой
// видимостью; пустая видимость — поля в запросе нет вовсе.
func publishWithVisibility(t *testing.T, baseURL, token, caption, visibility string) visPostPayload {
	t.Helper()

	photo := photoOf(t, baseURL, token, 60, 40)
	body := map[string]any{"media_ids": []string{photo.ID}, "caption": caption}
	if visibility != "" {
		body["visibility"] = visibility
	}

	resp := createPost(t, baseURL, token, body)
	post := visPostOK(t, resp, http.StatusCreated, "публикация поста "+visibility)
	requireVisibility(t, post, orAll(visibility), "ответ на публикацию")

	return post
}

// orAll — видимость, которую пост должен получить: без поля — all.
func orAll(visibility string) string {
	if visibility == "" {
		return visibilityAll
	}

	return visibility
}

// visPostOK требует ожидаемого статуса и возвращает пост с видимостью.
func visPostOK(t *testing.T, resp *http.Response, wantStatus int, where string) visPostPayload {
	t.Helper()

	if resp.StatusCode != wantStatus {
		t.Fatalf("%s: ожидался статус %d, получен %d", where, wantStatus, resp.StatusCode)
	}

	var body visPostPayload
	decode(t, resp, &body)

	return body
}

// requireVisibility требует у поста ровно эту видимость.
func requireVisibility(t *testing.T, post visPostPayload, want, where string) {
	t.Helper()

	if post.Visibility == nil {
		t.Fatalf("%s: у поста %s нет поля visibility", where, post.ID)
	}
	if *post.Visibility != want {
		t.Errorf("%s: visibility = %q, ожидалось %q", where, *post.Visibility, want)
	}
}

// setVisibility меняет видимость поста; body — как есть.
func setVisibility(t *testing.T, baseURL, token, postID string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/posts/"+postID+"/visibility", token, body)
}

// changeVisibility меняет видимость своего поста, требует 200 и пост
// с новой видимостью в ответе.
func changeVisibility(t *testing.T, baseURL, token, postID, visibility string) visPostPayload {
	t.Helper()

	resp := setVisibility(t, baseURL, token, postID, map[string]any{"visibility": visibility})
	post := visPostOK(t, resp, http.StatusOK, "смена видимости на "+visibility)
	if post.ID != postID {
		t.Fatalf("смена видимости поста %s вернула пост %s", postID, post.ID)
	}
	requireVisibility(t, post, visibility, "ответ на смену видимости")

	return post
}

// visPost открывает пост по адресу и требует, чтобы он открылся.
func visPost(t *testing.T, baseURL, token, postID, where string) visPostPayload {
	t.Helper()
	return visPostOK(t, fetchPost(t, baseURL, token, postID), http.StatusOK, where+": GET /posts/{id}")
}

// visFeed — первая страница вкладки ленты (limit 50).
func visFeed(t *testing.T, baseURL, token, scope string) visFeedPayload {
	t.Helper()

	resp := fetchFeed(t, baseURL, token, scopeParams(scope, 50, ""))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("лента %s: ожидался статус 200, получен %d", scope, resp.StatusCode)
	}

	var page visFeedPayload
	decode(t, resp, &page)

	return page
}

// visUserPosts — первая страница постов пользователя (limit 50). ok ==
// false — профиль закрыт для смотрящего (403 profile_closed): постов он не
// видит никаких.
func visUserPosts(t *testing.T, baseURL, token, userID string) (page visFeedPayload, ok bool) {
	t.Helper()

	resp := fetchUserPosts(t, baseURL, token, userID, feedParams(50, ""))
	if resp.StatusCode == http.StatusForbidden {
		if code := errorCode(t, resp); code != "profile_closed" {
			t.Fatalf("посты пользователя: 403 с кодом %q, ожидался profile_closed", code)
		}
		return visFeedPayload{}, false
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("посты пользователя: ожидался статус 200, получен %d", resp.StatusCode)
	}

	decode(t, resp, &page)

	return page, true
}

// findPost — пост с этим id на странице или nil.
func findPost(page visFeedPayload, postID string) *visPostPayload {
	for i := range page.Items {
		if page.Items[i].ID == postID {
			return &page.Items[i]
		}
	}

	return nil
}

// visIDs — идентификаторы постов страницы в порядке ответа.
func visIDs(page visFeedPayload) []string {
	ids := make([]string, 0, len(page.Items))
	for _, post := range page.Items {
		ids = append(ids, post.ID)
	}

	return ids
}

// requireInFeed требует, чтобы пост стоял во вкладке ленты с этой
// видимостью.
func requireInFeed(t *testing.T, baseURL string, viewer dachnik, scope, postID, visibility, where string) {
	t.Helper()

	post := findPost(visFeed(t, baseURL, viewer.token, scope), postID)
	if post == nil {
		t.Fatalf("%s: поста %s нет во вкладке %s", where, postID, scope)
	}
	requireVisibility(t, *post, visibility, where+": вкладка "+scope)
}

// requireNotInFeedScope требует, чтобы поста не было во вкладке ленты.
func requireNotInFeedScope(t *testing.T, baseURL string, viewer dachnik, scope, postID, where string) {
	t.Helper()

	if findPost(visFeed(t, baseURL, viewer.token, scope), postID) != nil {
		t.Errorf("%s: пост %s стоит во вкладке %s, а видеть его нельзя", where, postID, scope)
	}
}

// requireVisibleTo требует, чтобы пост был виден смотрящему там, где
// пост может попасть к человеку при чтении: по адресу, во вкладке «Все»,
// в постах автора, и чтобы читались его комментарии (ФТ-4).
func requireVisibleTo(t *testing.T, baseURL string, viewer dachnik, authorID, postID, visibility, where string) {
	t.Helper()

	requireVisibility(t, visPost(t, baseURL, viewer.token, postID, where), visibility, where+": GET /posts/{id}")
	requireInFeed(t, baseURL, viewer, "all", postID, visibility, where)

	page, ok := visUserPosts(t, baseURL, viewer.token, authorID)
	if !ok {
		t.Fatalf("%s: посты автора отвечают 403 profile_closed, а пост должен быть виден", where)
	}
	post := findPost(page, postID)
	if post == nil {
		t.Fatalf("%s: поста %s нет в постах автора", where, postID)
	}
	requireVisibility(t, *post, visibility, where+": посты автора")

	if resp := fetchComments(t, baseURL, viewer.token, postID); resp.StatusCode != http.StatusOK {
		t.Errorf("%s: комментарии видимого поста отвечают %d, ожидался 200", where, resp.StatusCode)
	}
}

// requireNotFound требует 404 post_not_found с понятным местом в сообщении.
func requireNotFound(t *testing.T, resp *http.Response, what, where string) {
	t.Helper()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("%s: %s невидимого поста ответил %d, ожидался 404 post_not_found", where, what, resp.StatusCode)
		return
	}
	if code := errorCode(t, resp); code != "post_not_found" {
		t.Errorf("%s: %s невидимого поста ответил 404 с кодом %q, ожидался post_not_found", where, what, code)
	}
}

// requireHiddenFrom требует, чтобы поста для смотрящего не было нигде:
// ни в обеих вкладках ленты, ни в постах автора (или посты автора целиком
// закрыты), а все ручки поста отвечают 404 post_not_found, как у
// несуществующего (ФТ-4). Удаление поста не проверяется: его делает
// requireHiddenPostNotDeletable, потому что при ошибке реализации пост
// пропал бы.
func requireHiddenFrom(t *testing.T, baseURL string, viewer dachnik, authorID, postID, where string) {
	t.Helper()

	requireNotFound(t, fetchPost(t, baseURL, viewer.token, postID), "GET /posts/{id}", where)
	requireNotInFeedScope(t, baseURL, viewer, "all", postID, where)
	requireNotInFeedScope(t, baseURL, viewer, "following", postID, where)

	if page, ok := visUserPosts(t, baseURL, viewer.token, authorID); ok && findPost(page, postID) != nil {
		t.Errorf("%s: пост %s стоит в постах автора, а видеть его нельзя", where, postID)
	}

	requireNotFound(t, likePost(t, baseURL, viewer.token, postID), "лайк", where)
	requireNotFound(t, unlikePost(t, baseURL, viewer.token, postID), "снятие лайка", where)
	requireNotFound(t, fetchComments(t, baseURL, viewer.token, postID), "чтение комментариев", where)
	requireNotFound(t, addCommentText(t, baseURL, viewer.token, postID, commentText), "комментарий", where)
	requireNotFound(t, reportPostReason(t, baseURL, viewer.token, postID, reportReason), "жалоба на пост", where)
	requireNotFound(t, setVisibility(t, baseURL, viewer.token, postID, map[string]any{"visibility": visibilityAll}),
		"смена видимости", where)
}

// requireHiddenPostNotDeletable требует, чтобы удаление невидимого чужого
// поста отвечало 404, а не 403: иначе ответ выдал бы, что пост есть.
func requireHiddenPostNotDeletable(t *testing.T, baseURL string, viewer dachnik, postID, where string) {
	t.Helper()
	requireNotFound(t, deletePost(t, baseURL, viewer.token, postID), "удаление", where)
}

// makeFriends делает двоих друзьями — подписывает друг на друга
// (specs/012-follows.md, ФТ-5). Оба профиля должны быть открыты.
func makeFriends(t *testing.T, baseURL string, a, b dachnik) {
	t.Helper()

	requireRelation(t, ptrRelation(followOK(t, baseURL, a, b.id)), followingYes, false, "первая подписка дружбы")
	requireRelation(t, ptrRelation(followOK(t, baseURL, b, a.id)), followingYes, true, "ответная подписка дружбы")
}

func ptrRelation(rel relationPayload) *relationPayload { return &rel }

// profilePosts — число постов в профиле глазами смотрящего.
func profilePosts(t *testing.T, baseURL, token, userID string) int {
	t.Helper()
	return *followProfile(t, baseURL, token, userID).Posts
}

// requireProfilePosts требует такое число постов в профиле.
func requireProfilePosts(t *testing.T, baseURL string, viewer dachnik, userID string, want int, where string) {
	t.Helper()

	if got := profilePosts(t, baseURL, viewer.token, userID); got != want {
		t.Errorf("%s: в профиле %d постов, ожидалось %d", where, got, want)
	}
}

// --- Видимость при публикации ---------------------------------------------

// Пост без visibility — all: в ответе на публикацию, по адресу, в ленте
// и в постах автора (ФТ-1, «Пост без visibility»).
func TestPostVisibilityDefaultsToAll(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, stranger := people[0], people[1]

	post := publishWithVisibility(t, baseURL, author.token, postCaption, "")

	requireVisibleTo(t, baseURL, author, author.id, post.ID, visibilityAll, "автор")
	requireVisibleTo(t, baseURL, stranger, author.id, post.ID, visibilityAll, "посторонний")
	requireInFeed(t, baseURL, author, "following", post.ID, visibilityAll, "автор")
}

// Видимость, выбранная при публикации, приходит у поста везде, где пост
// видит автор: в ответе, по адресу, в обеих вкладках и в его постах (ФТ-1,
// контракт: Post.visibility обязательно).
func TestPostVisibilityIsReturnedAsChosen(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)

	for _, visibility := range []string{visibilityAll, visibilityFriends, visibilityMe} {
		post := publishWithVisibility(t, baseURL, author.token, "пост "+visibility, visibility)
		where := "автор, пост " + visibility

		requireVisibleTo(t, baseURL, author, author.id, post.ID, visibility, where)
		requireInFeed(t, baseURL, author, "following", post.ID, visibility, where)
	}
}

// Неизвестная видимость при публикации — 400 invalid_request, и пост не
// создан: у автора постов нет, а фотография осталась неопубликованной
// и годится для нового поста («visibility: "public"»).
func TestCreatePostRejectsUnknownVisibility(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	photo := photoOf(t, baseURL, author.token, 60, 40)

	for _, bad := range []any{"public", "", "ALL", "Friends", 1, true, []string{"all"}} {
		t.Run(fmt.Sprintf("%v", bad), func(t *testing.T) {
			resp := createPost(t, baseURL, author.token,
				map[string]any{"media_ids": []string{photo.ID}, "caption": postCaption, "visibility": bad})
			requireError(t, resp, http.StatusBadRequest, "invalid_request")
		})
	}

	page, _ := visUserPosts(t, baseURL, author.token, author.id)
	if len(page.Items) != 0 {
		t.Fatalf("после отклонённых публикаций у автора %d постов, ожидалось 0", len(page.Items))
	}
	if n := len(visFeed(t, baseURL, author.token, "all").Items); n != 0 {
		t.Fatalf("после отклонённых публикаций в ленте %d постов, ожидалось 0", n)
	}
	requireProfilePosts(t, baseURL, author, author.id, 0, "после отклонённых публикаций")

	// Фотография не ушла в несозданный пост: из неё публикуется новый.
	resp := createPost(t, baseURL, author.token,
		map[string]any{"media_ids": []string{photo.ID}, "visibility": visibilityFriends})
	requireVisibility(t, visPostOK(t, resp, http.StatusCreated, "публикация после отказа"), visibilityFriends, "публикация после отказа")
}

// --- Кто видит пост -------------------------------------------------------

// Пост «друзьям» другу виден везде: по адресу, в обеих вкладках, в постах
// автора; друг может отметить, снять отметку, прочитать и оставить
// комментарий, удалить свой комментарий и пожаловаться (ФТ-2, ФТ-4,
// сценарий, шаг 3, «Пост „друзьям“, смотрит друг»).
func TestFriendsPostIsVisibleToFriend(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, friend := people[0], people[1]
	makeFriends(t, baseURL, author, friend)

	post := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityFriends)

	requireVisibleTo(t, baseURL, friend, author.id, post.ID, visibilityFriends, "друг")
	requireInFeed(t, baseURL, friend, "following", post.ID, visibilityFriends, "друг")

	liked := visPostOK(t, likePost(t, baseURL, friend.token, post.ID), http.StatusOK, "лайк друга")
	if liked.Likes != 1 || !liked.Liked {
		t.Errorf("после лайка друга likes = %d, liked = %v; ожидалось 1 и true", liked.Likes, liked.Liked)
	}
	requireVisibility(t, liked, visibilityFriends, "ответ на лайк")

	unliked := visPostOK(t, unlikePost(t, baseURL, friend.token, post.ID), http.StatusOK, "снятие лайка другом")
	if unliked.Likes != 0 || unliked.Liked {
		t.Errorf("после снятия лайка likes = %d, liked = %v; ожидалось 0 и false", unliked.Likes, unliked.Liked)
	}

	comment := createdComment(t, addCommentText(t, baseURL, friend.token, post.ID, commentText))
	if got := commentsOf(t, baseURL, friend.token, post.ID); len(got.Items) != 1 {
		t.Errorf("друг видит %d комментариев, ожидался 1", len(got.Items))
	}
	requireAccepted(t, reportPostReason(t, baseURL, friend.token, post.ID, reportReason))

	own := createdComment(t, addCommentText(t, baseURL, author.token, post.ID, "Спасибо!"))
	requireAccepted(t, reportCommentReason(t, baseURL, friend.token, post.ID, own.ID, commentReportReason))

	requireDeleted(t, deleteComment(t, baseURL, friend.token, post.ID, comment.ID))
}

// Пост «друзьям» не виден тому, кто подписан на автора без взаимности,
// тому, на кого автор подписан без взаимности, и постороннему: 404
// post_not_found на все ручки поста, в лентах и постах автора его нет
// (ФТ-2, ФТ-4, сценарий, шаг 3, «Пост „друзьям“, смотрит подписчик без
// взаимности», «Лайк или комментарий к невидимому посту», «Жалоба на
// невидимый пост»).
func TestFriendsPostIsHiddenFromNonFriends(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 4)
	author, follower, followee, stranger := people[0], people[1], people[2], people[3]

	followOK(t, baseURL, follower, author.id)
	followOK(t, baseURL, author, followee.id)

	post := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityFriends)
	open := publishWithVisibility(t, baseURL, author.token, "для всех", visibilityAll)

	for _, c := range []struct {
		who    string
		viewer dachnik
	}{
		{"подписчик без взаимности", follower},
		{"тот, на кого автор подписан без взаимности", followee},
		{"посторонний", stranger},
	} {
		requireHiddenFrom(t, baseURL, c.viewer, author.id, post.ID, c.who)
		requireHiddenPostNotDeletable(t, baseURL, c.viewer, post.ID, c.who)
		// Пост «Всем» того же автора им виден: прячется только пост «друзьям».
		requireVisibleTo(t, baseURL, c.viewer, author.id, open.ID, visibilityAll, c.who)
	}

	// Подписчик без взаимности видит у автора в «Подписках» только пост «Всем».
	requireInFeed(t, baseURL, follower, "following", open.ID, visibilityAll, "подписчик без взаимности")

	// Ни одна попытка ничего не оставила на посте, и сам пост на месте.
	got := visPost(t, baseURL, author.token, post.ID, "автор после попыток")
	if got.Likes != 0 || got.Comments != 0 {
		t.Errorf("после попыток невидящих у поста likes = %d, comments = %d; ожидалось 0 и 0", got.Likes, got.Comments)
	}
}

// Друзья — две действующие подписки, заявка не в счёт: пока заявка
// к закрытому профилю не принята, пост «друзьям» не виден; после
// принятия — виден (specs/012-follows.md, ФТ-5; ФТ-2, ФТ-5).
func TestFriendsRequireAcceptedFollowsNotRequests(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, viewer := people[0], people[1]

	setClosed(t, baseURL, author.token, true)
	followOK(t, baseURL, author, viewer.id)
	requireRelation(t, ptrRelation(followOK(t, baseURL, viewer, author.id)), followingRequested, true, "заявка к закрытому")

	post := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityFriends)
	requireHiddenFrom(t, baseURL, viewer, author.id, post.ID, "заявитель, на которого автор подписан")

	requireNoContent(t, acceptRequest(t, baseURL, author.token, viewer.id), "принятие заявки")
	requireVisibleTo(t, baseURL, viewer, author.id, post.ID, visibilityFriends, "друг после принятия заявки")
	requireInFeed(t, baseURL, viewer, "following", post.ID, visibilityFriends, "друг после принятия заявки")
}

// Пост «только мне» виден только автору, в том числе в его «Подписках»;
// другу, подписчику и постороннему его нет нигде (ФТ-2, «Пост „только
// мне“»).
func TestMePostIsVisibleOnlyToAuthor(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 4)
	author, friend, follower, stranger := people[0], people[1], people[2], people[3]
	makeFriends(t, baseURL, author, friend)
	followOK(t, baseURL, follower, author.id)

	post := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityMe)

	requireVisibleTo(t, baseURL, author, author.id, post.ID, visibilityMe, "автор")
	requireInFeed(t, baseURL, author, "following", post.ID, visibilityMe, "автор")

	for _, c := range []struct {
		who    string
		viewer dachnik
	}{
		{"друг", friend},
		{"подписчик", follower},
		{"посторонний", stranger},
	} {
		requireHiddenFrom(t, baseURL, c.viewer, author.id, post.ID, c.who)
		requireHiddenPostNotDeletable(t, baseURL, c.viewer, post.ID, c.who)
	}

	// Автор со своим постом «только мне» делает всё, что с любым своим.
	liked := visPostOK(t, likePost(t, baseURL, author.token, post.ID), http.StatusOK, "лайк автора")
	if liked.Likes != 1 {
		t.Errorf("после лайка автора likes = %d, ожидался 1", liked.Likes)
	}
	createdComment(t, addCommentText(t, baseURL, author.token, post.ID, "Здесь посадил чеснок"))
	requireDeleted(t, deletePost(t, baseURL, author.token, post.ID))
}

// Закрытый профиль, пост «Всем» — видят только подписчики: подписчику
// пост виден везде, постороннему и заявителю — 404 post_not_found на все
// ручки поста (ФТ-2, «Закрытый профиль, пост „Всем“»).
func TestClosedProfileAllPostIsForFollowersOnly(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 4)
	author, follower, stranger, requester := people[0], people[1], people[2], people[3]

	followOK(t, baseURL, follower, author.id)
	setClosed(t, baseURL, author.token, true)
	followOK(t, baseURL, requester, author.id)

	post := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityAll)

	requireVisibleTo(t, baseURL, follower, author.id, post.ID, visibilityAll, "подписчик закрытого")
	requireInFeed(t, baseURL, follower, "following", post.ID, visibilityAll, "подписчик закрытого")
	visPostOK(t, likePost(t, baseURL, follower.token, post.ID), http.StatusOK, "лайк подписчика закрытого")
	createdComment(t, addCommentText(t, baseURL, follower.token, post.ID, commentText))

	for _, c := range []struct {
		who    string
		viewer dachnik
	}{
		{"посторонний", stranger},
		{"заявитель", requester},
	} {
		requireHiddenFrom(t, baseURL, c.viewer, author.id, post.ID, c.who)
		requireHiddenPostNotDeletable(t, baseURL, c.viewer, post.ID, c.who)
	}
}

// Видимость считается в момент запроса: автор закрыл профиль — его посты
// «Всем» пропали у не-подписчика сразу, у подписчика остались; открыл —
// вернулись (ФТ-5).
func TestClosingProfileHidesAllPostsFromNonFollowersAtOnce(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	author, follower, stranger := people[0], people[1], people[2]
	followOK(t, baseURL, follower, author.id)

	post := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityAll)
	requireVisibleTo(t, baseURL, stranger, author.id, post.ID, visibilityAll, "посторонний, профиль открыт")

	setClosed(t, baseURL, author.token, true)
	requireHiddenFrom(t, baseURL, stranger, author.id, post.ID, "посторонний, профиль закрыт")
	requireVisibleTo(t, baseURL, follower, author.id, post.ID, visibilityAll, "подписчик, профиль закрыт")

	setClosed(t, baseURL, author.token, false)
	requireVisibleTo(t, baseURL, stranger, author.id, post.ID, visibilityAll, "посторонний, профиль снова открыт")
}

// Друг отписался — дружбы нет: посты «друзьям» пропадают у обоих, и его
// у автора, и автора у него; подписались снова — появляются (ФТ-5, «Друг
// отписался»).
func TestUnfollowingFriendHidesFriendsPostsBothWays(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]
	makeFriends(t, baseURL, a, b)

	postA := publishWithVisibility(t, baseURL, a.token, "от A друзьям", visibilityFriends)
	postB := publishWithVisibility(t, baseURL, b.token, "от B друзьям", visibilityFriends)
	requireVisibleTo(t, baseURL, b, a.id, postA.ID, visibilityFriends, "B, пока друзья")
	requireVisibleTo(t, baseURL, a, b.id, postB.ID, visibilityFriends, "A, пока друзья")

	unfollowOK(t, baseURL, b, a.id)

	requireHiddenFrom(t, baseURL, b, a.id, postA.ID, "B после своей отписки")
	requireHiddenFrom(t, baseURL, a, b.id, postB.ID, "A после отписки B")

	// Свои посты у каждого остались.
	requireVisibleTo(t, baseURL, a, a.id, postA.ID, visibilityFriends, "A, свой пост")
	requireVisibleTo(t, baseURL, b, b.id, postB.ID, visibilityFriends, "B, свой пост")

	followOK(t, baseURL, b, a.id)
	requireVisibleTo(t, baseURL, b, a.id, postA.ID, visibilityFriends, "B, снова друзья")
	requireVisibleTo(t, baseURL, a, b.id, postB.ID, visibilityFriends, "A, снова друзья")
}

// --- PUT /api/posts/{postId}/visibility -----------------------------------

// Автор меняет видимость своего поста: 200 и пост с новой видимостью;
// лайки и комментарии остаются; кто перестал видеть пост, не видит и их;
// все, кому пост снова виден, видят их на месте (ФТ-6, сценарий, шаг 4,
// «Автор сменил all на me»).
func TestAuthorChangesPostVisibility(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	author, friend, stranger := people[0], people[1], people[2]
	makeFriends(t, baseURL, author, friend)

	post := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityAll)
	visPostOK(t, likePost(t, baseURL, friend.token, post.ID), http.StatusOK, "лайк друга")
	visPostOK(t, likePost(t, baseURL, stranger.token, post.ID), http.StatusOK, "лайк постороннего")
	createdComment(t, addCommentText(t, baseURL, friend.token, post.ID, "от друга"))
	strangerComment := createdComment(t, addCommentText(t, baseURL, stranger.token, post.ID, "от постороннего"))

	requireKept := func(p visPostPayload, where string) {
		t.Helper()
		if p.Likes != 2 || p.Comments != 2 {
			t.Errorf("%s: likes = %d, comments = %d; ожидалось 2 и 2 — лайки и комментарии остаются", where, p.Likes, p.Comments)
		}
	}

	// all → me: пропал у всех, кроме автора.
	requireKept(changeVisibility(t, baseURL, author.token, post.ID, visibilityMe), "ответ на смену на me")
	requireVisibleTo(t, baseURL, author, author.id, post.ID, visibilityMe, "автор после смены на me")
	requireKept(visPost(t, baseURL, author.token, post.ID, "автор после смены на me"), "автор после смены на me")
	if got := commentsOf(t, baseURL, author.token, post.ID); len(got.Items) != 2 {
		t.Errorf("автор после смены на me видит %d комментариев, ожидалось 2", len(got.Items))
	}
	requireHiddenFrom(t, baseURL, friend, author.id, post.ID, "друг после смены на me")
	requireHiddenFrom(t, baseURL, stranger, author.id, post.ID, "посторонний после смены на me")

	// Отклонённые 404 лайк, снятие лайка и комментарий ничего не тронули:
	// снятие лайка невидимого поста не снимает прежний лайк.
	requireKept(visPost(t, baseURL, author.token, post.ID, "автор после попыток невидящих"),
		"автор после отклонённых 404 лайков, снятий лайка и комментариев невидящих")

	// Посторонний не удалит и не пожалуется на свой комментарий под невидимым
	// постом: поста для него нет.
	requireNotFound(t, deleteComment(t, baseURL, stranger.token, post.ID, strangerComment.ID), "удаление своего комментария", "посторонний после смены на me")

	// me → friends: другу виден с лайками и комментариями, постороннему нет.
	requireKept(changeVisibility(t, baseURL, author.token, post.ID, visibilityFriends), "ответ на смену на friends")
	requireVisibleTo(t, baseURL, friend, author.id, post.ID, visibilityFriends, "друг после смены на friends")
	friendView := visPost(t, baseURL, friend.token, post.ID, "друг после смены на friends")
	requireKept(friendView, "друг после смены на friends")
	if !friendView.Liked {
		t.Error("друг после смены на friends: его лайк пропал — liked = false")
	}
	requireHiddenFrom(t, baseURL, stranger, author.id, post.ID, "посторонний после смены на friends")

	// friends → all: снова виден всем, всё на месте.
	requireKept(changeVisibility(t, baseURL, author.token, post.ID, visibilityAll), "ответ на смену на all")
	requireVisibleTo(t, baseURL, stranger, author.id, post.ID, visibilityAll, "посторонний после смены на all")
	strangerView := visPost(t, baseURL, stranger.token, post.ID, "посторонний после смены на all")
	requireKept(strangerView, "посторонний после смены на all")
	if !strangerView.Liked {
		t.Error("посторонний после смены на all: его лайк пропал — liked = false")
	}

	// Та же видимость ещё раз — не ошибка.
	requireKept(changeVisibility(t, baseURL, author.token, post.ID, visibilityAll), "повторная смена на all")
}

// Чужой пост, который смотрящему виден, — 403 not_your_post; невидимый —
// 404 post_not_found; несуществующий — 404. Видимость не меняется
// («Смена видимости чужого видимого поста», ФТ-4).
func TestChangeVisibilityOfAnotherUsersPost(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	author, friend, stranger := people[0], people[1], people[2]
	makeFriends(t, baseURL, author, friend)

	open := publishWithVisibility(t, baseURL, author.token, "для всех", visibilityAll)
	friendsOnly := publishWithVisibility(t, baseURL, author.token, "друзьям", visibilityFriends)

	requireError(t, setVisibility(t, baseURL, stranger.token, open.ID, map[string]any{"visibility": visibilityMe}),
		http.StatusForbidden, "not_your_post")
	requireError(t, setVisibility(t, baseURL, friend.token, friendsOnly.ID, map[string]any{"visibility": visibilityMe}),
		http.StatusForbidden, "not_your_post")
	requireError(t, setVisibility(t, baseURL, stranger.token, friendsOnly.ID, map[string]any{"visibility": visibilityAll}),
		http.StatusNotFound, "post_not_found")
	requireError(t, setVisibility(t, baseURL, author.token, unknownID, map[string]any{"visibility": visibilityAll}),
		http.StatusNotFound, "post_not_found")

	requireVisibility(t, visPost(t, baseURL, author.token, open.ID, "автор"), visibilityAll, "пост «Всем» после чужих попыток")
	requireVisibility(t, visPost(t, baseURL, author.token, friendsOnly.ID, "автор"), visibilityFriends, "пост «друзьям» после чужих попыток")
	requireHiddenFrom(t, baseURL, stranger, author.id, friendsOnly.ID, "посторонний после своей попытки открыть пост")
}

// Неизвестная видимость или её нет — 400 invalid_request, видимость
// прежняя (контракт PUT /posts/{postId}/visibility).
func TestChangeVisibilityRejectsBadValue(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityFriends)

	for _, c := range []struct {
		name string
		body any
	}{
		{"public", map[string]any{"visibility": "public"}},
		{"пустая строка", map[string]any{"visibility": ""}},
		{"не той буквой", map[string]any{"visibility": "ALL"}},
		{"число", map[string]any{"visibility": 1}},
		{"null", map[string]any{"visibility": nil}},
		{"поля нет", map[string]any{}},
		{"тела нет", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			requireError(t, setVisibility(t, baseURL, author.token, post.ID, c.body), http.StatusBadRequest, "invalid_request")
		})
	}

	requireVisibility(t, visPost(t, baseURL, author.token, post.ID, "после отказов"), visibilityFriends, "после отказов")
}

// Смена видимости без токена или с недействительным — 401.
func TestChangeVisibilityRequiresValidToken(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityAll)

	for _, token := range []string{"", "не-токен"} {
		resp := setVisibility(t, baseURL, token, post.ID, map[string]any{"visibility": visibilityMe})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("смена видимости с токеном %q: статус %d, ожидался 401", token, resp.StatusCode)
		}
	}

	requireVisibility(t, visPost(t, baseURL, author.token, post.ID, "после попыток без токена"), visibilityAll, "после попыток без токена")
}

// --- Лента и профиль ------------------------------------------------------

// Невидимые посты отбираются до страницы, а не после: страница ленты
// и постов автора приходит полной, проход курсором даёт ровно видимые
// посты («Модель данных»: условие в SQL, иначе страница короче limit).
func TestHiddenPostsDoNotShortenPages(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, stranger := people[0], people[1]

	var visible []string
	for i := 1; i <= 3; i++ {
		visible = append(visible, publishWithVisibility(t, baseURL, author.token, fmt.Sprintf("видимый %d", i), visibilityAll).ID)
		for j := 1; j <= 3; j++ {
			publishWithVisibility(t, baseURL, author.token, fmt.Sprintf("скрытый %d-%d", i, j), []string{visibilityFriends, visibilityMe}[j%2])
		}
	}
	want := newestFirst(visible)

	first := feedPage(t, baseURL, stranger.token, scopeParams("all", 2, ""))
	requireFeedPosts(t, first, want[:2], "первая страница «Всех» по 2")

	if got := walkScopeFeed(t, baseURL, stranger.token, "all", 2); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("проход «Всех» по 2: %v, ожидалось %v", got, want)
	}

	firstOwn := userPostsPage(t, baseURL, stranger.token, author.id, feedParams(2, ""))
	requireFeedPosts(t, firstOwn, want[:2], "первая страница постов автора по 2")

	if got := walkUserPosts(t, baseURL, stranger.token, author.id, 2); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("проход постов автора по 2: %v, ожидалось %v", got, want)
	}

	followOK(t, baseURL, stranger, author.id)
	if got := walkScopeFeed(t, baseURL, stranger.token, "following", 2); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("проход «Подписок» по 2 у подписчика без взаимности: %v, ожидалось %v", got, want)
	}

	// Автору его страница по 2 приходит полной, со всеми двенадцатью постами.
	if got := walkUserPosts(t, baseURL, author.token, author.id, 2); len(got) != 12 {
		t.Errorf("автор проходит свои посты и видит %d, ожидалось 12", len(got))
	}
}

// Число постов в профиле — только видимые смотрящему; у закрытого
// профиля без подписки — посты «Всем», сколько он увидит, подписавшись
// («Число постов в профиле», specs/012-follows.md, ФТ-13).
func TestProfilePostCountCountsVisiblePosts(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 5)
	author, friend, follower, stranger, requester := people[0], people[1], people[2], people[3], people[4]
	makeFriends(t, baseURL, author, friend)
	followOK(t, baseURL, follower, author.id)

	for i := 0; i < 3; i++ {
		publishWithVisibility(t, baseURL, author.token, "всем", visibilityAll)
	}
	for i := 0; i < 2; i++ {
		publishWithVisibility(t, baseURL, author.token, "друзьям", visibilityFriends)
	}
	publishWithVisibility(t, baseURL, author.token, "только мне", visibilityMe)

	requireProfilePosts(t, baseURL, author, author.id, 6, "автор, профиль открыт")
	requireProfilePosts(t, baseURL, friend, author.id, 5, "друг, профиль открыт")
	requireProfilePosts(t, baseURL, follower, author.id, 3, "подписчик, профиль открыт")
	requireProfilePosts(t, baseURL, stranger, author.id, 3, "посторонний, профиль открыт")

	setClosed(t, baseURL, author.token, true)
	followOK(t, baseURL, requester, author.id)

	requireProfilePosts(t, baseURL, author, author.id, 6, "автор, профиль закрыт")
	requireProfilePosts(t, baseURL, friend, author.id, 5, "друг, профиль закрыт")
	requireProfilePosts(t, baseURL, follower, author.id, 3, "подписчик, профиль закрыт")
	requireProfilePosts(t, baseURL, stranger, author.id, 3, "посторонний, профиль закрыт: посты «Всем», не ноль")
	requireProfilePosts(t, baseURL, requester, author.id, 3, "заявитель, профиль закрыт: посты «Всем»")

	// Смена видимости сразу меняет числа.
	page, _ := visUserPosts(t, baseURL, author.token, author.id)
	for _, post := range page.Items {
		if *post.Visibility == visibilityFriends {
			changeVisibility(t, baseURL, author.token, post.ID, visibilityMe)
		}
	}
	requireProfilePosts(t, baseURL, author, author.id, 6, "автор после смены friends на me")
	requireProfilePosts(t, baseURL, friend, author.id, 3, "друг после смены friends на me")
}
