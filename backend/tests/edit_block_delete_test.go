package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// Тесты правки, блокировки и удаления аккаунта
// (specs/022-edit-block-delete.md). Написаны по спецификации и контракту,
// не глядя в реализацию (ADR-0002). Подписки и закрытые профили — из
// specs/012-follows.md, видимость постов — из specs/013-post-visibility.md.

// Подпись, на которую автор исправляет опечатку (сценарий «Правка»).
const fixedCaption = "Первая клубника в этом году!"

// Текст, на который автор исправляет комментарий (контракт, CommentDraft).
const fixedCommentText = "И у нас такая же, брызгали содой"

// Предел длины подписи — как при создании поста (ФТ-2, 003-posts).
const captionLimit = 1000

// Как автор отзыва выглядит у владельца после удаления аккаунта (ФТ-23).
const deletedAuthor = "удалённый пользователь"

// --- Представления из контракта -------------------------------------------

// ebdPost — пост с полями, важными правке (schema Post): всё, что правка
// обязана не трогать, и edited_at. edited_at — указатель: у неизменённого
// поста его нет или он null (ФТ-4).
type ebdPost struct {
	visPostPayload
	EditedAt *string `json:"edited_at"`
}

// ebdFeed — страница ленты с edited_at у каждого поста.
type ebdFeed struct {
	Items      []ebdPost `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}

// ebdComment — комментарий с edited_at (schema Comment).
type ebdComment struct {
	commentPayload
	EditedAt *string `json:"edited_at"`
}

// ebdComments — комментарии поста (schema Comments) с edited_at.
type ebdComments struct {
	Items []ebdComment `json:"items"`
}

// ebdProfile — профиль пользователя (schema UserProfile) с полем blocked.
// Поля — указатели: обязательное поле обязано прийти.
type ebdProfile struct {
	ID        string `json:"id"`
	Posts     *int   `json:"posts"`
	Followers *int   `json:"followers"`
	Following *int   `json:"following"`
	Blocked   *bool  `json:"blocked"`
}

// --- Хелперы: правка ------------------------------------------------------

// editCaptionReq меняет подпись поста; body — как есть.
func editCaptionReq(t *testing.T, baseURL, token, postID string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/posts/"+postID+"/caption", token, body)
}

// editCaptionText меняет подпись так, как это делает приложение.
func editCaptionText(t *testing.T, baseURL, token, postID, caption string) *http.Response {
	t.Helper()
	return editCaptionReq(t, baseURL, token, postID, map[string]any{"caption": caption})
}

// editCommentReq меняет комментарий; body — как есть.
func editCommentReq(t *testing.T, baseURL, token, postID, commentID string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/posts/"+postID+"/comments/"+commentID, token, body)
}

// editCommentText меняет текст комментария так, как это делает приложение.
func editCommentText(t *testing.T, baseURL, token, postID, commentID, text string) *http.Response {
	t.Helper()
	return editCommentReq(t, baseURL, token, postID, commentID, map[string]any{"text": text})
}

// ebdRaw отправляет тело запроса как есть: телу, которое вовсе не JSON,
// объекта в Go не соответствует.
func ebdRaw(t *testing.T, method, address, token, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(method, address, strings.NewReader(body))
	if err != nil {
		t.Fatalf("не удалось собрать запрос %s %s: %v", method, address, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("запрос %s %s не прошёл: %v", method, address, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// ebdPostOK требует 200 и возвращает пост.
func ebdPostOK(t *testing.T, resp *http.Response, where string) ebdPost {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: ожидался статус 200, получен %d", where, resp.StatusCode)
	}

	var body ebdPost
	decode(t, resp, &body)

	return body
}

// captionEdited меняет подпись своего поста и требует 200 с тем же постом.
func captionEdited(t *testing.T, baseURL, token, postID, caption string) ebdPost {
	t.Helper()

	post := ebdPostOK(t, editCaptionText(t, baseURL, token, postID, caption), "правка подписи")
	if post.ID != postID {
		t.Fatalf("правка подписи поста %s вернула пост %s", postID, post.ID)
	}

	return post
}

// ebdPostAt открывает пост по адресу.
func ebdPostAt(t *testing.T, baseURL, token, postID string) ebdPost {
	t.Helper()
	return ebdPostOK(t, fetchPost(t, baseURL, token, postID), "GET /posts/{id}")
}

// ebdFeedItem находит пост на первой странице ленты (limit 50).
func ebdFeedItem(t *testing.T, baseURL, token, postID string) ebdPost {
	t.Helper()

	resp := fetchFeed(t, baseURL, token, feedParams(50, ""))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("лента: ожидался статус 200, получен %d", resp.StatusCode)
	}

	var page ebdFeed
	decode(t, resp, &page)

	for _, item := range page.Items {
		if item.ID == postID {
			return item
		}
	}

	t.Fatalf("поста %s нет в ленте", postID)

	return ebdPost{}
}

// requireEditedAt требует, чтобы edited_at был и разбирался как время
// не раньше created_at; возвращает его.
func requireEditedAt(t *testing.T, editedAt *string, createdAt, where string) string {
	t.Helper()

	if editedAt == nil || *editedAt == "" {
		t.Fatalf("%s: после правки должно быть edited_at, а его нет", where)
	}

	edited, err := time.Parse(time.RFC3339Nano, *editedAt)
	if err != nil {
		t.Fatalf("%s: edited_at %q не разбирается как время: %v", where, *editedAt, err)
	}
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		t.Fatalf("%s: created_at %q не разбирается как время: %v", where, createdAt, err)
	}
	if edited.Before(created.Truncate(time.Second)) {
		t.Errorf("%s: edited_at %s раньше created_at %s", where, *editedAt, createdAt)
	}

	return *editedAt
}

// requireNotEdited требует, чтобы edited_at не было (или он был null).
func requireNotEdited(t *testing.T, editedAt *string, where string) {
	t.Helper()

	if editedAt != nil {
		t.Errorf("%s: у неизменённого edited_at = %q, а его быть не должно", where, *editedAt)
	}
}

// ebdCommentOK требует 200 и возвращает комментарий.
func ebdCommentOK(t *testing.T, resp *http.Response, where string) ebdComment {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: ожидался статус 200, получен %d", where, resp.StatusCode)
	}

	var body ebdComment
	decode(t, resp, &body)

	return body
}

// ebdCommentsOf читает комментарии поста с edited_at.
func ebdCommentsOf(t *testing.T, baseURL, token, postID string) []ebdComment {
	t.Helper()

	resp := fetchComments(t, baseURL, token, postID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("комментарии поста %s: ожидался статус 200, получен %d", postID, resp.StatusCode)
	}

	var body ebdComments
	decode(t, resp, &body)

	return body.Items
}

// ebdCommentIDs — идентификаторы комментариев по порядку.
func ebdCommentIDs(items []ebdComment) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	return ids
}

// requireCaptionStays требует, чтобы подпись поста осталась прежней.
func requireCaptionStays(t *testing.T, baseURL, token, postID, want, where string) {
	t.Helper()

	post := ebdPostAt(t, baseURL, token, postID)
	if post.Caption != want {
		t.Errorf("%s: подпись стала %q, а должна была остаться %q", where, post.Caption, want)
	}
	requireNotEdited(t, post.EditedAt, where)
}

// requireCommentTextStays требует, чтобы текст комментария остался прежним.
func requireCommentTextStays(t *testing.T, baseURL, token, postID, commentID, want, where string) {
	t.Helper()

	for _, c := range ebdCommentsOf(t, baseURL, token, postID) {
		if c.ID != commentID {
			continue
		}
		if c.Text != want {
			t.Errorf("%s: текст комментария стал %q, а должен был остаться %q", where, c.Text, want)
		}
		requireNotEdited(t, c.EditedAt, where)
		return
	}

	t.Fatalf("%s: комментария %s под постом нет", where, commentID)
}

// --- Хелперы: блокировка --------------------------------------------------

// blockAddress — адрес блокировки пользователя.
func blockAddress(baseURL, userID string) string {
	return userAddress(baseURL, userID) + "/block"
}

// blockReq блокирует пользователя.
func blockReq(t *testing.T, baseURL, token, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, blockAddress(baseURL, userID), token, nil)
}

// unblockReq разблокирует пользователя.
func unblockReq(t *testing.T, baseURL, token, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, blockAddress(baseURL, userID), token, nil)
}

// requireEmpty204 требует 204 и пустое тело.
func requireEmpty204(t *testing.T, resp *http.Response, where string) {
	t.Helper()

	if resp.StatusCode != http.StatusNoContent {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус 204, получен %d %s", where, resp.StatusCode, code)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s: не удалось прочитать тело ответа: %v", where, err)
	}
	if len(body) != 0 {
		t.Errorf("%s: ответ 204 обязан быть пустым, пришло %q", where, body)
	}
}

// blockOK блокирует и требует 204.
func blockOK(t *testing.T, baseURL string, who dachnik, whom string) {
	t.Helper()
	requireEmpty204(t, blockReq(t, baseURL, who.token, whom), "блокировка")
}

// unblockOK разблокирует и требует 204.
func unblockOK(t *testing.T, baseURL string, who dachnik, whom string) {
	t.Helper()
	requireEmpty204(t, unblockReq(t, baseURL, who.token, whom), "разблокировка")
}

// blockedList читает «Заблокированные» и требует 200.
func blockedList(t *testing.T, baseURL, token string) authorListPayload {
	t.Helper()

	resp := do(t, http.MethodGet, baseURL+"/me/blocked", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /me/blocked: ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body authorListPayload
	decode(t, resp, &body)

	if body.Items == nil {
		t.Fatalf("GET /me/blocked: поле items обязательно, даже пустое")
	}
	if body.NextCursor != nil {
		t.Errorf("GET /me/blocked: список целиком, next_cursor быть не должно, а он %q", *body.NextCursor)
	}

	return body
}

// blockedIDs — идентификаторы заблокированных по порядку.
func blockedIDs(t *testing.T, baseURL, token string) []string {
	t.Helper()

	items := blockedList(t, baseURL, token).Items
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	return ids
}

// ebdProfileOf открывает профиль, требует 200 и все поля.
func ebdProfileOf(t *testing.T, baseURL, token, userID string) ebdProfile {
	t.Helper()

	resp := fetchUser(t, baseURL, token, userID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("профиль %s: ожидался статус 200, получен %d", userID, resp.StatusCode)
	}

	var body ebdProfile
	decode(t, resp, &body)

	if body.Posts == nil || body.Followers == nil || body.Following == nil || body.Blocked == nil {
		t.Fatalf("профиль %s: нет одного из полей posts, followers, following, blocked: %+v", userID, body)
	}

	return body
}

// requireUserNotFound требует 404 user_not_found.
func requireUserNotFound(t *testing.T, resp *http.Response, where string) {
	t.Helper()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("%s: ожидался 404 user_not_found, получен статус %d", where, resp.StatusCode)
		return
	}
	if code := errorCode(t, resp); code != "user_not_found" {
		t.Errorf("%s: ожидалась ошибка user_not_found, получена %q", where, code)
	}
}

// requireCodeE — то же, что requireError, но без остановки теста.
func requireCodeE(t *testing.T, resp *http.Response, status int, code, where string) {
	t.Helper()

	if resp.StatusCode != status {
		t.Errorf("%s: ожидался статус %d %s, получен %d", where, status, code, resp.StatusCode)
		return
	}
	if got := errorCode(t, resp); got != code {
		t.Errorf("%s: ожидалась ошибка %s, получена %q", where, code, got)
	}
}

// requirePostUnreachable требует, чтобы пост для смотрящего не существовал:
// ни по адресу, ни в ленте, а лайк, комментарий и жалоба — 404
// post_not_found (ФТ-12). Посты автора не смотрятся: для заблокированного
// сам автор — 404 user_not_found, это проверяется отдельно.
func requirePostUnreachable(t *testing.T, baseURL string, viewer dachnik, postID, where string) {
	t.Helper()

	requireNotFound(t, fetchPost(t, baseURL, viewer.token, postID), "GET /posts/{id}", where)
	requireNotInFeedScope(t, baseURL, viewer, "all", postID, where)
	requireNotInFeedScope(t, baseURL, viewer, "following", postID, where)
	requireNotFound(t, likePost(t, baseURL, viewer.token, postID), "лайк", where)
	requireNotFound(t, fetchComments(t, baseURL, viewer.token, postID), "чтение комментариев", where)
	requireNotFound(t, addCommentText(t, baseURL, viewer.token, postID, commentText), "комментарий", where)
	requireNotFound(t, reportPostReason(t, baseURL, viewer.token, postID, reportReason), "жалоба на пост", where)
}

// actorIDs — кто сделал каждую строку уведомлений.
func actorIDs(items []notificationPayload) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.Actor != nil {
			ids = append(ids, item.Actor.ID)
		}
	}

	return ids
}

// --- Хелперы: удаление аккаунта --------------------------------------------

// deleteMeReq удаляет свой аккаунт.
func deleteMeReq(t *testing.T, baseURL, token string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, baseURL+"/me", token, nil)
}

// deleteMeOK удаляет свой аккаунт и требует 204 без тела (ФТ-25).
func deleteMeOK(t *testing.T, baseURL, token string) {
	t.Helper()
	requireEmpty204(t, deleteMeReq(t, baseURL, token), "DELETE /me")
}

// requireUnauthorized требует 401 unauthorized.
func requireUnauthorized(t *testing.T, resp *http.Response, where string) {
	t.Helper()
	requireCodeE(t, resp, http.StatusUnauthorized, "unauthorized", where)
}

// ============================================================================
// PUT /api/posts/{postId}/caption — правка подписи
// ============================================================================

// Автор меняет подпись: 200, новая подпись и edited_at в ответе, по адресу
// поста и в ленте — тоже новая, у соседа — тоже («Автор меняет подпись»,
// ФТ-1, ФТ-4, ФТ-7).
func TestEditCaptionChangesItEverywhere(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	viewer := newDachnik(t, baseURL, 2)

	post := publishPost(t, baseURL, author.token, postCaption)

	before := ebdPostAt(t, baseURL, author.token, post.ID)
	requireNotEdited(t, before.EditedAt, "свежий пост")

	edited := captionEdited(t, baseURL, author.token, post.ID, fixedCaption)
	if edited.Caption != fixedCaption {
		t.Errorf("в ответе на правку подпись %q, ожидалась %q", edited.Caption, fixedCaption)
	}
	requireEditedAt(t, edited.EditedAt, edited.CreatedAt, "ответ на правку")
	if edited.CreatedAt != post.CreatedAt {
		t.Errorf("правка сдвинула created_at: было %s, стало %s", post.CreatedAt, edited.CreatedAt)
	}
	if edited.Author.ID != author.id {
		t.Errorf("в ответе на правку автор %s, ожидался %s", edited.Author.ID, author.id)
	}

	for _, who := range []struct {
		name  string
		token string
	}{{"автор", author.token}, {"сосед", viewer.token}} {
		byAddress := ebdPostAt(t, baseURL, who.token, post.ID)
		if byAddress.Caption != fixedCaption {
			t.Errorf("%s: по адресу поста подпись %q, ожидалась %q", who.name, byAddress.Caption, fixedCaption)
		}
		requireEditedAt(t, byAddress.EditedAt, byAddress.CreatedAt, who.name+": пост по адресу")

		inFeed := ebdFeedItem(t, baseURL, who.token, post.ID)
		if inFeed.Caption != fixedCaption {
			t.Errorf("%s: в ленте подпись %q, ожидалась %q", who.name, inFeed.Caption, fixedCaption)
		}
		requireEditedAt(t, inFeed.EditedAt, inFeed.CreatedAt, who.name+": пост в ленте")
	}
}

// Края подписи обрезаются, как при публикации («Подпись с пробелами по
// краям», ФТ-2).
func TestEditCaptionTrimsEdges(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishPost(t, baseURL, author.token, postCaption)

	edited := captionEdited(t, baseURL, author.token, post.ID, "  \n\t"+fixedCaption+" \n ")
	if edited.Caption != fixedCaption {
		t.Errorf("ожидалась подпись без пробелов по краям %q, получена %q", fixedCaption, edited.Caption)
	}
	if got := ebdPostAt(t, baseURL, author.token, post.ID).Caption; got != fixedCaption {
		t.Errorf("по адресу поста подпись %q, ожидалась %q", got, fixedCaption)
	}
}

// Подпись можно стереть: пустая подпись допустима, и из одних пробелов —
// тоже, она обрезается до пустой («Подпись меняется на пустую», ФТ-2).
func TestEditCaptionToEmpty(t *testing.T) {
	for name, caption := range map[string]string{"пустая": "", "из пробелов": "   \n "} {
		t.Run(name, func(t *testing.T) {
			baseURL := startAPI(t)
			author := newDachnik(t, baseURL, 1)
			post := publishPost(t, baseURL, author.token, postCaption)

			edited := captionEdited(t, baseURL, author.token, post.ID, caption)
			if edited.Caption != "" {
				t.Errorf("ожидалась пустая подпись, получена %q", edited.Caption)
			}
			requireEditedAt(t, edited.EditedAt, edited.CreatedAt, "подпись стёрта")
			if got := ebdPostAt(t, baseURL, author.token, post.ID).Caption; got != "" {
				t.Errorf("по адресу поста подпись %q, ожидалась пустая", got)
			}
		})
	}
}

// Длина считается после обрезки и в символах: 1000 символов с пробелами
// по краям — можно, 1001 — 400 invalid_caption, и подпись остаётся
// прежней («Подпись длиннее 1000 символов после обрезки», ФТ-2).
func TestEditCaptionLengthLimit(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishPost(t, baseURL, author.token, postCaption)

	tooLong := repeatRunes("клубника ", captionLimit) + "я"
	requireError(t, editCaptionText(t, baseURL, author.token, post.ID, tooLong), http.StatusBadRequest, "invalid_caption")
	requireCaptionStays(t, baseURL, author.token, post.ID, postCaption, "после отвергнутой длинной подписи")

	longest := repeatRunes("ягода", captionLimit)
	edited := captionEdited(t, baseURL, author.token, post.ID, "   "+longest+"   ")
	if edited.Caption != longest {
		t.Errorf("подпись ровно в %d символов после обрезки должна приниматься целиком, получено %d символов",
			captionLimit, len([]rune(edited.Caption)))
	}
}

// Подпись не поменялась (с учётом обрезки) — 200, а edited_at не появляется;
// у уже изменённого — не сдвигается («Подпись не поменялась», ФТ-4).
func TestEditCaptionWithSameTextChangesNothing(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishPost(t, baseURL, author.token, postCaption)

	same := captionEdited(t, baseURL, author.token, post.ID, "  "+postCaption+" ")
	if same.Caption != postCaption {
		t.Errorf("подпись %q, ожидалась прежняя %q", same.Caption, postCaption)
	}
	requireNotEdited(t, same.EditedAt, "ответ на правку тем же текстом")
	requireNotEdited(t, ebdPostAt(t, baseURL, author.token, post.ID).EditedAt, "пост по адресу после правки тем же текстом")

	first := captionEdited(t, baseURL, author.token, post.ID, fixedCaption)
	editedAt := requireEditedAt(t, first.EditedAt, first.CreatedAt, "первая правка")

	time.Sleep(20 * time.Millisecond)

	again := captionEdited(t, baseURL, author.token, post.ID, fixedCaption+"  ")
	if again.EditedAt == nil || *again.EditedAt != editedAt {
		t.Errorf("правка тем же текстом сдвинула edited_at: было %s, стало %v", editedAt, again.EditedAt)
	}
	if got := ebdPostAt(t, baseURL, author.token, post.ID).EditedAt; got == nil || *got != editedAt {
		t.Errorf("по адресу поста edited_at %v, ожидался прежний %s", got, editedAt)
	}
}

// Правка подписи не трогает ничего, кроме подписи: фотографии в том же
// порядке, лайки, комментарии, видимость, время публикации («Правка
// подписи: фотографии, лайки, комментарии, видимость», ФТ-1, ФТ-7).
func TestEditCaptionKeepsPhotosLikesCommentsAndVisibility(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	friend := newDachnik(t, baseURL, 2)
	makeFriends(t, baseURL, author, friend)

	photos := []string{
		photoOf(t, baseURL, author.token, 60, 40).ID,
		photoOf(t, baseURL, author.token, 40, 60).ID,
		photoOf(t, baseURL, author.token, 50, 50).ID,
	}
	created := visPostOK(t, createPost(t, baseURL, author.token, map[string]any{
		"media_ids": photos, "caption": postCaption, "visibility": visibilityFriends,
	}), http.StatusCreated, "публикация")

	likeOK(t, baseURL, friend, created.ID)
	commentOf(t, baseURL, friend.token, created.ID, commentText)

	before := ebdPostAt(t, baseURL, author.token, created.ID)

	edited := captionEdited(t, baseURL, author.token, created.ID, fixedCaption)

	for _, got := range []struct {
		where string
		post  ebdPost
	}{
		{"ответ на правку", edited},
		{"пост по адресу", ebdPostAt(t, baseURL, author.token, created.ID)},
	} {
		if !reflect.DeepEqual(got.post.Media, before.Media) {
			t.Errorf("%s: фотографии изменились:\nбыло:  %+v\nстало: %+v", got.where, before.Media, got.post.Media)
		}
		if got.post.Likes != 1 || got.post.Likes != before.Likes {
			t.Errorf("%s: лайков %d, ожидался %d", got.where, got.post.Likes, before.Likes)
		}
		if got.post.Comments != 1 {
			t.Errorf("%s: комментариев %d, ожидался 1", got.where, got.post.Comments)
		}
		requireVisibility(t, got.post.visPostPayload, visibilityFriends, got.where)
		if got.post.CreatedAt != before.CreatedAt {
			t.Errorf("%s: created_at %s, было %s", got.where, got.post.CreatedAt, before.CreatedAt)
		}
		if got.post.Author.ID != author.id {
			t.Errorf("%s: автор %s, ожидался %s", got.where, got.post.Author.ID, author.id)
		}
	}

	requireCommentTexts(t, baseURL, author.token, created.ID, []string{commentText}, "комментарии после правки подписи")

	// Друг видит пост с новой подписью и прежним лайком.
	seen := ebdPostAt(t, baseURL, friend.token, created.ID)
	if seen.Caption != fixedCaption || !seen.Liked {
		t.Errorf("друг видит подпись %q и liked=%v, ожидались %q и true", seen.Caption, seen.Liked, fixedCaption)
	}
}

// Чужой пост — 403 not_your_post, подпись прежняя; проверка «своё ли»
// раньше проверки текста («Чужой пост», ФТ-5, ФТ-6).
func TestEditCaptionOfAnotherUsersPostIsForbidden(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	stranger := newDachnik(t, baseURL, 2)
	post := publishPost(t, baseURL, author.token, postCaption)

	requireError(t, editCaptionText(t, baseURL, stranger.token, post.ID, fixedCaption), http.StatusForbidden, "not_your_post")
	requireCaptionStays(t, baseURL, author.token, post.ID, postCaption, "после попытки чужой правки")

	tooLong := repeatRunes("ы", captionLimit+1)
	requireError(t, editCaptionText(t, baseURL, stranger.token, post.ID, tooLong), http.StatusForbidden, "not_your_post")
	requireCaptionStays(t, baseURL, author.token, post.ID, postCaption, "после чужой длинной подписи")
}

// Пост, которого нет, который смотрящему не виден или идентификатор
// которого не UUID, — 404 post_not_found, и раньше проверки текста
// («Пост не виден смотрящему или его нет, или не UUID», ФТ-6).
func TestEditCaptionOfMissingOrInvisiblePostIsNotFound(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	stranger := newDachnik(t, baseURL, 2)

	hidden := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityMe)
	tooLong := repeatRunes("ы", captionLimit+1)

	for _, tc := range []struct {
		name   string
		postID string
	}{
		{"поста нет", unknownID},
		{"не UUID", notAnID},
		{"пост только для автора", hidden.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireError(t, editCaptionText(t, baseURL, stranger.token, tc.postID, fixedCaption),
				http.StatusNotFound, "post_not_found")
			requireError(t, editCaptionText(t, baseURL, stranger.token, tc.postID, tooLong),
				http.StatusNotFound, "post_not_found")
		})
	}

	requireCaptionStays(t, baseURL, author.token, hidden.ID, postCaption, "невидимый пост после попыток правки")
}

// Без токена — 401 unauthorized, к своему и к чужому посту, и подпись
// прежняя («Без токена», ФТ-6).
func TestEditCaptionWithoutTokenIsUnauthorized(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishPost(t, baseURL, author.token, postCaption)

	requireError(t, editCaptionText(t, baseURL, "", post.ID, fixedCaption), http.StatusUnauthorized, "unauthorized")
	requireError(t, editCaptionText(t, baseURL, "", unknownID, fixedCaption), http.StatusUnauthorized, "unauthorized")
	requireError(t, editCaptionText(t, baseURL, "не-токен", post.ID, fixedCaption), http.StatusUnauthorized, "unauthorized")

	requireCaptionStays(t, baseURL, author.token, post.ID, postCaption, "после правки без токена")
}

// Тело без caption или не JSON — 400 invalid_request, подпись прежняя
// («Тело без caption или не JSON»).
func TestEditCaptionWithBadBodyIsInvalidRequest(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishPost(t, baseURL, author.token, postCaption)
	address := baseURL + "/posts/" + post.ID + "/caption"

	for name, body := range map[string]string{
		"пустой объект":    `{}`,
		"другое поле":      `{"text": "Первая клубника"}`,
		"не JSON":          `подпись`,
		"обрезанный JSON":  `{"caption": "Первая`,
		"подпись не текст": `{"caption": 42}`,
	} {
		t.Run(name, func(t *testing.T) {
			requireError(t, ebdRaw(t, http.MethodPut, address, author.token, body), http.StatusBadRequest, "invalid_request")
		})
	}

	requireCaptionStays(t, baseURL, author.token, post.ID, postCaption, "после непригодных тел")
}

// ============================================================================
// PUT /api/posts/{postId}/comments/{commentId} — правка комментария
// ============================================================================

// Автор меняет комментарий: 200, новый текст и edited_at, в списке он на
// прежнем месте, остальные не тронуты, число комментариев то же («Автор
// меняет комментарий», ФТ-3, ФТ-4, ФТ-7).
func TestEditCommentKeepsItsPlace(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	neighbour := newDachnik(t, baseURL, 2)

	post := publishPost(t, baseURL, author.token, postCaption)
	first := commentOf(t, baseURL, neighbour.token, post.ID, "первый")
	second := commentOf(t, baseURL, author.token, post.ID, "второй")
	third := commentOf(t, baseURL, neighbour.token, post.ID, "третий")

	edited := ebdCommentOK(t, editCommentText(t, baseURL, neighbour.token, post.ID, first.ID, "  "+fixedCommentText+" "), "правка комментария")
	if edited.ID != first.ID {
		t.Fatalf("правка комментария %s вернула комментарий %s", first.ID, edited.ID)
	}
	if edited.Text != fixedCommentText {
		t.Errorf("в ответе текст %q, ожидался %q (края обрезаются)", edited.Text, fixedCommentText)
	}
	if edited.Author.ID != neighbour.id {
		t.Errorf("в ответе автор %s, ожидался %s", edited.Author.ID, neighbour.id)
	}
	if edited.CreatedAt != first.CreatedAt {
		t.Errorf("правка сдвинула created_at: было %s, стало %s", first.CreatedAt, edited.CreatedAt)
	}
	requireEditedAt(t, edited.EditedAt, edited.CreatedAt, "ответ на правку комментария")

	for _, who := range []dachnik{author, neighbour} {
		items := ebdCommentsOf(t, baseURL, who.token, post.ID)
		if got, want := ebdCommentIDs(items), []string{first.ID, second.ID, third.ID}; !reflect.DeepEqual(got, want) {
			t.Fatalf("порядок комментариев после правки %v, ожидался прежний %v", got, want)
		}
		if items[0].Text != fixedCommentText {
			t.Errorf("в списке текст изменённого %q, ожидался %q", items[0].Text, fixedCommentText)
		}
		requireEditedAt(t, items[0].EditedAt, items[0].CreatedAt, "изменённый комментарий в списке")
		if items[1].Text != "второй" || items[2].Text != "третий" {
			t.Errorf("остальные комментарии изменились: %q, %q", items[1].Text, items[2].Text)
		}
		requireNotEdited(t, items[1].EditedAt, "неизменённый комментарий в списке")
		requireNotEdited(t, items[2].EditedAt, "неизменённый комментарий в списке")
	}

	requireCommentCountEverywhere(t, baseURL, author.token, post.ID, 3, "автор поста после правки")
}

// Пустой текст и текст из пробелов — 400 empty_comment, текст прежний
// («Комментарий меняется на пустой или из пробелов», ФТ-3).
func TestEditCommentToEmptyIsRejected(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishPost(t, baseURL, author.token, postCaption)
	comment := commentOf(t, baseURL, author.token, post.ID, commentText)

	for name, text := range map[string]string{"пустой": "", "из пробелов": "  \n\t "} {
		t.Run(name, func(t *testing.T) {
			requireError(t, editCommentText(t, baseURL, author.token, post.ID, comment.ID, text),
				http.StatusBadRequest, "empty_comment")
		})
	}

	requireCommentTextStays(t, baseURL, author.token, post.ID, comment.ID, commentText, "после пустых правок")
}

// Длиннее 1000 символов после обрезки — 400 invalid_comment; ровно 1000
// с пробелами по краям — можно («Комментарий длиннее 1000 символов», ФТ-3).
func TestEditCommentLengthLimit(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishPost(t, baseURL, author.token, postCaption)
	comment := commentOf(t, baseURL, author.token, post.ID, commentText)

	requireError(t, editCommentText(t, baseURL, author.token, post.ID, comment.ID, repeatRunes("огурец", commentLimit+1)),
		http.StatusBadRequest, "invalid_comment")
	requireCommentTextStays(t, baseURL, author.token, post.ID, comment.ID, commentText, "после длинной правки")

	longest := repeatRunes("томат", commentLimit)
	edited := ebdCommentOK(t, editCommentText(t, baseURL, author.token, post.ID, comment.ID, " "+longest+" "), "правка в 1000 символов")
	if edited.Text != longest {
		t.Errorf("комментарий ровно в %d символов должен приниматься целиком, получено %d символов",
			commentLimit, len([]rune(edited.Text)))
	}
}

// Текст не поменялся (с учётом обрезки) — 200, edited_at нет; у уже
// изменённого — не сдвигается («Текст комментария не поменялся», ФТ-4).
func TestEditCommentWithSameTextChangesNothing(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishPost(t, baseURL, author.token, postCaption)
	comment := commentOf(t, baseURL, author.token, post.ID, commentText)

	same := ebdCommentOK(t, editCommentText(t, baseURL, author.token, post.ID, comment.ID, " "+commentText+"  "), "правка тем же текстом")
	if same.Text != commentText {
		t.Errorf("текст %q, ожидался прежний %q", same.Text, commentText)
	}
	requireNotEdited(t, same.EditedAt, "ответ на правку тем же текстом")
	requireCommentTextStays(t, baseURL, author.token, post.ID, comment.ID, commentText, "список после правки тем же текстом")

	first := ebdCommentOK(t, editCommentText(t, baseURL, author.token, post.ID, comment.ID, fixedCommentText), "первая правка")
	editedAt := requireEditedAt(t, first.EditedAt, first.CreatedAt, "первая правка")

	time.Sleep(20 * time.Millisecond)

	again := ebdCommentOK(t, editCommentText(t, baseURL, author.token, post.ID, comment.ID, fixedCommentText), "повтор правки")
	if again.EditedAt == nil || *again.EditedAt != editedAt {
		t.Errorf("правка тем же текстом сдвинула edited_at: было %s, стало %v", editedAt, again.EditedAt)
	}
}

// Чужой комментарий под своим постом — 403 not_your_comment: чужие слова
// не правит никто; и пустой текст к чужому — тоже 403, «своё ли» раньше
// текста («Чужой комментарий под своим постом», «Чужой пустой текст
// к чужому комментарию», ФТ-5, ФТ-6).
func TestEditCommentOfAnotherUserIsForbidden(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	neighbour := newDachnik(t, baseURL, 2)
	stranger := newDachnik(t, baseURL, 3)

	post := publishPost(t, baseURL, author.token, postCaption)
	comment := commentOf(t, baseURL, neighbour.token, post.ID, commentText)

	requireError(t, editCommentText(t, baseURL, author.token, post.ID, comment.ID, fixedCommentText),
		http.StatusForbidden, "not_your_comment")
	requireError(t, editCommentText(t, baseURL, stranger.token, post.ID, comment.ID, fixedCommentText),
		http.StatusForbidden, "not_your_comment")
	requireError(t, editCommentText(t, baseURL, stranger.token, post.ID, comment.ID, ""),
		http.StatusForbidden, "not_your_comment")
	requireError(t, editCommentText(t, baseURL, author.token, post.ID, comment.ID, repeatRunes("ы", commentLimit+1)),
		http.StatusForbidden, "not_your_comment")

	requireCommentTextStays(t, baseURL, neighbour.token, post.ID, comment.ID, commentText, "после чужих правок")
}

// Комментарий под другим постом, несуществующий и не UUID — 404
// comment_not_found; пост, которого нет или который не виден, — 404
// post_not_found («Комментарий под другим постом», «Комментарий не UUID»,
// ФТ-6).
func TestEditCommentWithWrongAddressIsNotFound(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	stranger := newDachnik(t, baseURL, 2)

	post := publishPost(t, baseURL, author.token, postCaption)
	other := publishPost(t, baseURL, author.token, "Вторая грядка")
	comment := commentOf(t, baseURL, author.token, post.ID, commentText)

	requireError(t, editCommentText(t, baseURL, author.token, other.ID, comment.ID, fixedCommentText),
		http.StatusNotFound, "comment_not_found")
	requireError(t, editCommentText(t, baseURL, author.token, post.ID, unknownCommentID, fixedCommentText),
		http.StatusNotFound, "comment_not_found")
	requireError(t, editCommentText(t, baseURL, author.token, post.ID, notAnID, fixedCommentText),
		http.StatusNotFound, "comment_not_found")
	requireError(t, editCommentText(t, baseURL, author.token, unknownID, comment.ID, fixedCommentText),
		http.StatusNotFound, "post_not_found")
	requireError(t, editCommentText(t, baseURL, author.token, notAnID, comment.ID, fixedCommentText),
		http.StatusNotFound, "post_not_found")

	// Пост, скрытый от смотрящего, для него — как несуществующий, даже
	// если текст негодный.
	hidden := publishWithVisibility(t, baseURL, author.token, postCaption, visibilityMe)
	hiddenComment := commentOf(t, baseURL, author.token, hidden.ID, commentText)
	requireError(t, editCommentText(t, baseURL, stranger.token, hidden.ID, hiddenComment.ID, fixedCommentText),
		http.StatusNotFound, "post_not_found")
	requireError(t, editCommentText(t, baseURL, stranger.token, hidden.ID, hiddenComment.ID, ""),
		http.StatusNotFound, "post_not_found")

	requireCommentTextStays(t, baseURL, author.token, post.ID, comment.ID, commentText, "после правок по неверным адресам")
}

// Без токена — 401; тело без text или не JSON — 400 invalid_request.
func TestEditCommentWithoutTokenOrWithBadBody(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := publishPost(t, baseURL, author.token, postCaption)
	comment := commentOf(t, baseURL, author.token, post.ID, commentText)

	requireError(t, editCommentText(t, baseURL, "", post.ID, comment.ID, fixedCommentText), http.StatusUnauthorized, "unauthorized")

	address := baseURL + "/posts/" + post.ID + "/comments/" + comment.ID
	for name, body := range map[string]string{
		"пустой объект":   `{}`,
		"не JSON":         `комментарий`,
		"текст не строка": `{"text": 7}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp := ebdRaw(t, http.MethodPut, address, author.token, body)
			// Тело без text — это «текста нет»: контракт допускает и
			// empty_comment (006-comments), и invalid_request.
			if name == "пустой объект" && resp.StatusCode == http.StatusBadRequest {
				if code := errorCode(t, resp); code != "invalid_request" && code != "empty_comment" {
					t.Errorf("тело без text: ожидалась ошибка invalid_request или empty_comment, получена %q", code)
				}
				return
			}
			requireError(t, resp, http.StatusBadRequest, "invalid_request")
		})
	}

	requireCommentTextStays(t, baseURL, author.token, post.ID, comment.ID, commentText, "после непригодных правок")
}

// Уведомление о комментарии после правки показывает новый текст, а новых
// уведомлений правка не создаёт («Уведомление о комментарии после правки»,
// ФТ-8).
func TestEditCommentUpdatesNotificationText(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	neighbour := newDachnik(t, baseURL, 2)

	post := ownPost(t, baseURL, author)
	comment := commentOf(t, baseURL, neighbour.token, post.ID, commentText)

	before := onlyNotification(t, baseURL, author, kindComment, "до правки")
	if before.Comment == nil || *before.Comment != commentText {
		t.Fatalf("до правки в уведомлении текст %v, ожидался %q", before.Comment, commentText)
	}

	ebdCommentOK(t, editCommentText(t, baseURL, neighbour.token, post.ID, comment.ID, fixedCommentText), "правка")

	after := onlyNotification(t, baseURL, author, kindComment, "после правки")
	if after.Comment == nil || *after.Comment != fixedCommentText {
		t.Errorf("после правки в уведомлении текст %v, ожидался новый %q", after.Comment, fixedCommentText)
	}
	if after.ID != before.ID {
		t.Errorf("правка заменила строку уведомления: была %s, стала %s", before.ID, after.ID)
	}
	requireActor(t, after, neighbour.id, "после правки")
}

// ============================================================================
// PUT/DELETE /api/users/{userId}/block, GET /api/me/blocked — блокировка
// ============================================================================

// Блокировка — 204, подписки в обе стороны пропадают: счётчики у обоих
// нули, отношений нет; повторная — тоже 204 и ничего не меняет
// («Блокировка», «Повторная блокировка», ФТ-10, ФТ-11).
func TestBlockBreaksFollowsBothWays(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)
	makeFriends(t, baseURL, valya, kolya)

	requireCounts(t, baseURL, valya.token, valya.id, 1, 1, "Валя до блокировки")

	blockOK(t, baseURL, valya, kolya.id)

	requireCounts(t, baseURL, valya.token, valya.id, 0, 0, "Валя после блокировки")
	requireCounts(t, baseURL, kolya.token, kolya.id, 0, 0, "Коля после блокировки")
	if ids := walkFollowList(t, baseURL, valya.token, valya.id, "followers", 50); len(ids) != 0 {
		t.Errorf("подписчики Вали после блокировки %v, ожидалось пусто", ids)
	}
	if ids := walkFollowList(t, baseURL, valya.token, valya.id, "following", 50); len(ids) != 0 {
		t.Errorf("подписки Вали после блокировки %v, ожидалось пусто", ids)
	}

	blockOK(t, baseURL, valya, kolya.id)
	if got := blockedIDs(t, baseURL, valya.token); !reflect.DeepEqual(got, []string{kolya.id}) {
		t.Errorf("после повторной блокировки список %v, ожидался один Коля", got)
	}

	// Разблокировка подписок не возвращает (ФТ-17).
	unblockOK(t, baseURL, valya, kolya.id)
	requireCounts(t, baseURL, valya.token, valya.id, 0, 0, "Валя после разблокировки")
	requireCounts(t, baseURL, kolya.token, kolya.id, 0, 0, "Коля после разблокировки")
	requireProfileRelation(t, baseURL, valya, kolya.id, followingNone, false, "Валя смотрит Колю после разблокировки")
	requireProfileRelation(t, baseURL, kolya, valya.id, followingNone, false, "Коля смотрит Валю после разблокировки")
}

// Заявки в обе стороны блокировка тоже удаляет: после разблокировки их нет
// («Блокировка», ФТ-11, ФТ-17).
func TestBlockRemovesFollowRequestsBothWays(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)
	setClosed(t, baseURL, valya.token, true)
	setClosed(t, baseURL, kolya.token, true)

	requireRelation(t, ptrRelation(followOK(t, baseURL, kolya, valya.id)), followingRequested, false, "заявка Коли")
	requireRelation(t, ptrRelation(followOK(t, baseURL, valya, kolya.id)), followingRequested, false, "заявка Вали")

	blockOK(t, baseURL, valya, kolya.id)

	if ids := requesterIDs(t, baseURL, valya.token); len(ids) != 0 {
		t.Errorf("у Вали после блокировки заявки %v", ids)
	}

	unblockOK(t, baseURL, valya, kolya.id)

	if ids := requesterIDs(t, baseURL, valya.token); len(ids) != 0 {
		t.Errorf("у Вали после разблокировки вернулись заявки %v", ids)
	}
	if ids := requesterIDs(t, baseURL, kolya.token); len(ids) != 0 {
		t.Errorf("у Коли после разблокировки вернулись заявки %v", ids)
	}
	requireProfileRelation(t, baseURL, valya, kolya.id, followingNone, false, "Валя смотрит Колю")
	requireProfileRelation(t, baseURL, kolya, valya.id, followingNone, false, "Коля смотрит Валю")
}

// Себя — 400 cannot_block_self; несуществующего и не UUID — 404
// user_not_found; того, кто уже заблокировал смотрящего, — тоже 404
// («Блокировка себя», «Блокировка несуществующего или не UUID»,
// «Блокировка того, кто уже заблокировал смотрящего», ФТ-10, ФТ-14).
func TestBlockErrors(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)

	requireCodeE(t, blockReq(t, baseURL, valya.token, valya.id), http.StatusBadRequest, "cannot_block_self", "блокировка себя")
	requireUserNotFound(t, blockReq(t, baseURL, valya.token, unknownID), "блокировка несуществующего")
	requireUserNotFound(t, blockReq(t, baseURL, valya.token, notAnID), "блокировка не UUID")
	requireCodeE(t, blockReq(t, baseURL, "", kolya.id), http.StatusUnauthorized, "unauthorized", "блокировка без токена")

	if ids := blockedIDs(t, baseURL, valya.token); len(ids) != 0 {
		t.Errorf("после отвергнутых блокировок список %v, ожидалось пусто", ids)
	}

	blockOK(t, baseURL, valya, kolya.id)
	requireUserNotFound(t, blockReq(t, baseURL, kolya.token, valya.id), "ответная блокировка заблокированным")
	if ids := blockedIDs(t, baseURL, kolya.token); len(ids) != 0 {
		t.Errorf("у Коли в заблокированных %v, а его блокировка должна была не пройти", ids)
	}
}

// Разблокировка незаблокированного — 204; несуществующего и не UUID —
// 404 user_not_found; для заблокированного блокирующий не существует,
// и разблокировать его — тоже 404 («Разблокировка не заблокированного»,
// «Разблокировка несуществующего или не UUID», ФТ-14, ФТ-17).
func TestUnblockEdgeCases(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)

	unblockOK(t, baseURL, valya, kolya.id)
	requireUserNotFound(t, unblockReq(t, baseURL, valya.token, unknownID), "разблокировка несуществующего")
	requireUserNotFound(t, unblockReq(t, baseURL, valya.token, notAnID), "разблокировка не UUID")
	requireCodeE(t, unblockReq(t, baseURL, "", kolya.id), http.StatusUnauthorized, "unauthorized", "разблокировка без токена")

	blockOK(t, baseURL, valya, kolya.id)
	requireUserNotFound(t, unblockReq(t, baseURL, kolya.token, valya.id), "разблокировка блокирующего заблокированным")

	if got := ebdProfileOf(t, baseURL, valya.token, kolya.id); !*got.Blocked {
		t.Errorf("после попытки Коли снять блокировку Валя всё ещё должна видеть blocked: true")
	}
}

// Лента блокирующего: постов заблокированного нет, остальные на месте
// и по порядку, курсор проходит всё до конца («Блокирующий в ленте»,
// ФТ-12).
func TestBlockerFeedHasNoPostsOfBlocked(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)
	masha := newDachnik(t, baseURL, 3)

	var mashaPosts []string
	for i := 0; i < 3; i++ {
		mashaPosts = append(mashaPosts, publishPost(t, baseURL, masha.token, "Маша").ID)
		publishPost(t, baseURL, kolya.token, "Коля")
		publishPost(t, baseURL, kolya.token, "Коля ещё")
	}

	blockOK(t, baseURL, valya, kolya.id)

	for _, limit := range []int{1, 2, 50} {
		if got := walkScopeFeed(t, baseURL, valya.token, "", limit); !reflect.DeepEqual(got, newestFirst(mashaPosts)) {
			t.Errorf("лента Вали по %d: %v, ожидались только посты Маши %v", limit, got, newestFirst(mashaPosts))
		}
	}

	// У третьего лента целая.
	if got := walkScopeFeed(t, baseURL, masha.token, "", 50); len(got) != 9 {
		t.Errorf("у Маши в ленте %d постов, ожидалось 9", len(got))
	}
}

// Заблокированный в ленте постов блокирующего не видит, а пост блокирующего
// по адресу для него — 404, как и лайк, комментарий, жалоба («Заблокированный
// в ленте», «Пост блокирующего по адресу, у заблокированного», ФТ-12).
func TestBlockedDoesNotSeeBlockerPosts(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)
	masha := newDachnik(t, baseURL, 3)

	valyaPost := publishPost(t, baseURL, valya.token, "Валя")
	mashaPost := publishPost(t, baseURL, masha.token, "Маша")

	blockOK(t, baseURL, valya, kolya.id)

	requirePostUnreachable(t, baseURL, kolya, valyaPost.ID, "Коля и пост Вали")
	if got := walkScopeFeed(t, baseURL, kolya.token, "", 50); !reflect.DeepEqual(got, []string{mashaPost.ID}) {
		t.Errorf("лента Коли %v, ожидался только пост Маши", got)
	}

	// Сама Валя свой пост видит, Маша — тоже.
	requirePostAlive(t, baseURL, valya.token, valyaPost.ID, "Валя и свой пост")
	requirePostAlive(t, baseURL, masha.token, valyaPost.ID, "Маша и пост Вали")
}

// Пост заблокированного по адресу у блокирующего — 404 post_not_found,
// лайк, комментарий, жалоба — тоже 404; в постах его профиля пусто
// («Пост заблокированного по адресу, у блокирующего», ФТ-12).
func TestBlockerDoesNotSeeBlockedPosts(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)

	post := publishPost(t, baseURL, kolya.token, "Коля")
	comment := commentOf(t, baseURL, kolya.token, post.ID, commentText)

	blockOK(t, baseURL, valya, kolya.id)

	requireHiddenFrom(t, baseURL, valya, kolya.id, post.ID, "Валя и пост Коли")
	requireNotFound(t, reportCommentReason(t, baseURL, valya.token, post.ID, comment.ID, commentReportReason),
		"жалоба на комментарий", "Валя и пост Коли")
	requireNotFound(t, editCommentText(t, baseURL, valya.token, post.ID, comment.ID, fixedCommentText),
		"правка комментария", "Валя и пост Коли")

	requirePostAlive(t, baseURL, kolya.token, post.ID, "Коля и свой пост")
}

// Комментарии заблокированного под постом третьего блокирующему не видны
// и не входят в число comments, и наоборот; остальным видны все
// («Комментарии заблокированного под постом третьего», ФТ-13).
func TestBlockHidesCommentsUnderThirdPartyPosts(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)
	masha := newDachnik(t, baseURL, 3)
	petya := newDachnik(t, baseURL, 4)

	post := publishPost(t, baseURL, masha.token, "Маша")
	fromValya := commentOf(t, baseURL, valya.token, post.ID, "от Вали")
	fromKolya := commentOf(t, baseURL, kolya.token, post.ID, "от Коли")
	fromPetya := commentOf(t, baseURL, petya.token, post.ID, "от Пети")

	blockOK(t, baseURL, valya, kolya.id)

	for _, tc := range []struct {
		name string
		who  dachnik
		want []string
	}{
		{"Валя", valya, []string{fromValya.ID, fromPetya.ID}},
		{"Коля", kolya, []string{fromKolya.ID, fromPetya.ID}},
		{"Маша", masha, []string{fromValya.ID, fromKolya.ID, fromPetya.ID}},
		{"Петя", petya, []string{fromValya.ID, fromKolya.ID, fromPetya.ID}},
	} {
		got := commentIDs(commentsOf(t, baseURL, tc.who.token, post.ID).Items)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s видит комментарии %v, ожидались %v", tc.name, got, tc.want)
		}
		requireCommentCountEverywhere(t, baseURL, tc.who.token, post.ID, len(tc.want), tc.name)
	}
}

// Профиль заблокированного у блокирующего — 200, blocked: true, постов 0,
// страница постов пустая; у своего и у чужого профиля blocked: false
// («Профиль заблокированного у блокирующего», ФТ-15).
func TestBlockedProfileAsSeenByBlocker(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)
	masha := newDachnik(t, baseURL, 3)
	publishPosts(t, baseURL, kolya.token, 2)

	if p := ebdProfileOf(t, baseURL, valya.token, kolya.id); *p.Blocked || *p.Posts != 2 {
		t.Fatalf("до блокировки: blocked=%v posts=%d, ожидались false и 2", *p.Blocked, *p.Posts)
	}

	blockOK(t, baseURL, valya, kolya.id)

	p := ebdProfileOf(t, baseURL, valya.token, kolya.id)
	if p.ID != kolya.id {
		t.Errorf("профиль %s вместо %s", p.ID, kolya.id)
	}
	if !*p.Blocked {
		t.Error("у блокирующего в профиле заблокированного должно быть blocked: true")
	}
	if *p.Posts != 0 {
		t.Errorf("у блокирующего в профиле заблокированного постов %d, ожидалось 0", *p.Posts)
	}
	page := userPostsPage(t, baseURL, valya.token, kolya.id, feedParams(50, ""))
	if len(page.Items) != 0 || page.NextCursor != nil {
		t.Errorf("страница постов заблокированного у блокирующего: %d постов, курсор %v; ожидалась пустая",
			len(page.Items), page.NextCursor)
	}

	if own := ebdProfileOf(t, baseURL, valya.token, valya.id); *own.Blocked {
		t.Error("в своём профиле blocked должно быть false")
	}
	if other := ebdProfileOf(t, baseURL, masha.token, kolya.id); *other.Blocked || *other.Posts != 2 {
		t.Errorf("у третьего профиль Коли: blocked=%v posts=%d, ожидались false и 2", *other.Blocked, *other.Posts)
	}
}

// Для заблокированного блокирующий не существует: профиль, посты,
// подписчики, подписки, подписка на него — 404 user_not_found
// («Профиль блокирующего у заблокированного», «Подписка заблокированного
// на блокирующего», ФТ-14).
func TestBlockerDoesNotExistForBlocked(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)
	publishPost(t, baseURL, valya.token, "Валя")

	blockOK(t, baseURL, valya, kolya.id)

	requireUserNotFound(t, fetchUser(t, baseURL, kolya.token, valya.id), "профиль")
	requireUserNotFound(t, fetchUserPosts(t, baseURL, kolya.token, valya.id, feedParams(50, "")), "посты")
	requireUserNotFound(t, fetchFollowList(t, baseURL, kolya.token, valya.id, "followers", feedParams(50, "")), "подписчики")
	requireUserNotFound(t, fetchFollowList(t, baseURL, kolya.token, valya.id, "following", feedParams(50, "")), "подписки")
	requireUserNotFound(t, follow(t, baseURL, kolya.token, valya.id), "подписка")

	requireCounts(t, baseURL, valya.token, valya.id, 0, 0, "Валя после попытки Коли подписаться")
}

// Подписка блокирующего на заблокированного — 409 user_blocked: сначала
// разблокировать; после разблокировки — можно («Подписка блокирующего на
// заблокированного», ФТ-15).
func TestBlockerCannotFollowBlocked(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)

	blockOK(t, baseURL, valya, kolya.id)

	requireCodeE(t, follow(t, baseURL, valya.token, kolya.id), http.StatusConflict, "user_blocked", "подписка на заблокированного")
	requireCounts(t, baseURL, valya.token, kolya.id, 0, 0, "Коля после попытки подписки")

	unblockOK(t, baseURL, valya, kolya.id)
	requireRelation(t, ptrRelation(followOK(t, baseURL, valya, kolya.id)), followingYes, false, "подписка после разблокировки")
}

// В чужих списках подписчиков и подписок человек, связанный со смотрящим
// блокировкой в любую сторону, смотрящему не показывается; остальным —
// на месте («Заблокированный в чужих списках подписчиков и подписок»,
// ФТ-16).
func TestBlockedHiddenInThirdPartyFollowLists(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2) // его заблокировала Валя
	masha := newDachnik(t, baseURL, 3) // чьи списки смотрят
	petya := newDachnik(t, baseURL, 4) // посторонний
	zina := newDachnik(t, baseURL, 5)  // заблокировала Валю

	for _, who := range []dachnik{kolya, petya, zina} {
		followOK(t, baseURL, who, masha.id)
		followOK(t, baseURL, masha, who.id)
	}

	blockOK(t, baseURL, valya, kolya.id)
	blockOK(t, baseURL, zina, valya.id)

	for _, which := range []string{"followers", "following"} {
		got := walkFollowList(t, baseURL, valya.token, masha.id, which, 50)
		if !reflect.DeepEqual(got, []string{petya.id}) {
			t.Errorf("Валя видит в %s Маши %v, ожидался только Петя %s", which, got, petya.id)
		}

		all := sortedCopy([]string{kolya.id, petya.id, zina.id})
		if got := sortedCopy(walkFollowList(t, baseURL, petya.token, masha.id, which, 50)); !reflect.DeepEqual(got, all) {
			t.Errorf("Петя видит в %s Маши %v, ожидались все трое %v", which, got, all)
		}
		if got := sortedCopy(walkFollowList(t, baseURL, masha.token, masha.id, which, 50)); !reflect.DeepEqual(got, all) {
			t.Errorf("Маша видит в своих %s %v, ожидались все трое %v", which, got, all)
		}
	}
}

// Уведомления между ними до блокировки удалены в обе стороны, чужие — на
// месте («Уведомления между ними до блокировки», ФТ-11).
func TestBlockRemovesNotificationsBetweenThem(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)
	petya := newDachnik(t, baseURL, 3)

	valyaPost := ownPost(t, baseURL, valya)
	kolyaPost := ownPost(t, baseURL, kolya)

	followOK(t, baseURL, kolya, valya.id)
	likeOK(t, baseURL, kolya, valyaPost.ID)
	commentOf(t, baseURL, kolya.token, valyaPost.ID, commentText)
	followOK(t, baseURL, valya, kolya.id)
	likeOK(t, baseURL, valya, kolyaPost.ID)
	commentOf(t, baseURL, valya.token, kolyaPost.ID, commentText)
	followOK(t, baseURL, petya, valya.id)
	followOK(t, baseURL, petya, kolya.id)

	if got := notificationsOf(t, baseURL, valya); len(got) < 4 {
		t.Fatalf("до блокировки у Вали %d уведомлений, ожидалось не меньше 4", len(got))
	}

	blockOK(t, baseURL, valya, kolya.id)

	for _, tc := range []struct {
		name  string
		who   dachnik
		other dachnik
	}{{"Валя", valya, kolya}, {"Коля", kolya, valya}} {
		items := notificationsOf(t, baseURL, tc.who)
		if contains(actorIDs(items), tc.other.id) {
			t.Errorf("у %s после блокировки остались уведомления о действиях другого: %v", tc.name, actorIDs(items))
		}
		if got := actorIDs(items); !reflect.DeepEqual(got, []string{petya.id}) {
			t.Errorf("у %s после блокировки уведомления от %v, ожидалась только подписка Пети", tc.name, got)
		}
	}
}

// Лайки, поставленные до блокировки, остаются: число лайков у поста то же
// (ФТ-19).
func TestBlockKeepsLikes(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)

	post := publishPost(t, baseURL, valya.token, "Валя")
	likeOK(t, baseURL, kolya, post.ID)

	blockOK(t, baseURL, valya, kolya.id)

	requireLikesEverywhere(t, baseURL, valya.token, post.ID, 1, false, "Валя после блокировки")
}

// Разблокировка — 204; посты снова видны по обычным правилам: открытые —
// да, «для друзей» — нет, ведь подписки не вернулись («Разблокировка»,
// ФТ-17).
func TestUnblockRestoresVisibilityButNotFollows(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)
	makeFriends(t, baseURL, valya, kolya)

	open := publishWithVisibility(t, baseURL, kolya.token, "всем", visibilityAll)
	friendsOnly := publishWithVisibility(t, baseURL, kolya.token, "друзьям", visibilityFriends)
	valyaPost := publishWithVisibility(t, baseURL, valya.token, "Валя", visibilityAll)

	requireVisibleTo(t, baseURL, valya, kolya.id, friendsOnly.ID, visibilityFriends, "до блокировки")

	blockOK(t, baseURL, valya, kolya.id)
	requireHiddenFrom(t, baseURL, valya, kolya.id, open.ID, "во время блокировки")

	unblockOK(t, baseURL, valya, kolya.id)

	if p := ebdProfileOf(t, baseURL, valya.token, kolya.id); *p.Blocked {
		t.Error("после разблокировки blocked должно быть false")
	}
	requireVisibleTo(t, baseURL, valya, kolya.id, open.ID, visibilityAll, "после разблокировки")
	requireVisibleTo(t, baseURL, kolya, valya.id, valyaPost.ID, visibilityAll, "Коля после разблокировки")
	requireHiddenFrom(t, baseURL, valya, kolya.id, friendsOnly.ID, "пост для друзей после разблокировки")
	if ids := blockedIDs(t, baseURL, valya.token); len(ids) != 0 {
		t.Errorf("после разблокировки в списке %v", ids)
	}
}

// «Заблокированные» — свои блокировки, последние сверху, без курсора;
// повторная блокировка место не меняет; чужих блокировок в нём нет
// («Список «Заблокированные»», ФТ-18).
func TestBlockedList(t *testing.T) {
	baseURL := startAPI(t)
	valya := newDachnik(t, baseURL, 1)
	kolya := newDachnik(t, baseURL, 2)
	petya := newDachnik(t, baseURL, 3)
	masha := newDachnik(t, baseURL, 4)

	if ids := blockedIDs(t, baseURL, valya.token); len(ids) != 0 {
		t.Fatalf("у нового пользователя список %v, ожидалось пусто", ids)
	}
	requireCodeE(t, do(t, http.MethodGet, baseURL+"/me/blocked", "", nil), http.StatusUnauthorized, "unauthorized", "список без токена")

	blockOK(t, baseURL, valya, kolya.id)
	time.Sleep(20 * time.Millisecond)
	blockOK(t, baseURL, valya, petya.id)
	time.Sleep(20 * time.Millisecond)
	blockOK(t, baseURL, valya, kolya.id)
	blockOK(t, baseURL, masha, petya.id)

	list := blockedList(t, baseURL, valya.token)
	got := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		got = append(got, item.ID)
		if item.Nickname == "" {
			t.Errorf("у человека %s в списке нет ника", item.ID)
		}
	}
	if want := []string{petya.id, kolya.id}; !reflect.DeepEqual(got, want) {
		t.Errorf("у Вали в списке %v, ожидались %v (последние сверху)", got, want)
	}

	if got := blockedIDs(t, baseURL, masha.token); !reflect.DeepEqual(got, []string{petya.id}) {
		t.Errorf("у Маши в списке %v, ожидался только Петя", got)
	}
	if ids := blockedIDs(t, baseURL, petya.token); len(ids) != 0 {
		t.Errorf("у Пети, которого заблокировали, свой список %v, ожидалось пусто", ids)
	}
}

// ============================================================================
// DELETE /api/me — удаление аккаунта
// ============================================================================

// Удаление — 204 без тела; тот же токен и другие сессии — 401, профиль —
// 404 user_not_found («Удаление аккаунта», «Профиль удалённого», ФТ-20,
// ФТ-24, ФТ-25).
func TestDeleteAccountEndsAllSessions(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 50 * time.Millisecond})
	kolya := newDachnik(t, baseURL, 1)
	masha := newDachnik(t, baseURL, 2)

	time.Sleep(60 * time.Millisecond)
	secondToken, secondID := signIn(t, baseURL, dachnikPhone(1))
	if secondID != kolya.id {
		t.Fatalf("второй вход по тому же номеру дал другого пользователя: %s вместо %s", secondID, kolya.id)
	}

	requireUnauthorized(t, deleteMeReq(t, baseURL, ""), "удаление без токена")
	deleteMeOK(t, baseURL, kolya.token)

	for name, token := range map[string]string{"тот же токен": kolya.token, "другая сессия": secondToken} {
		requireUnauthorized(t, getProfile(t, baseURL, token), name+": GET /me")
		requireUnauthorized(t, currentSession(t, baseURL, token), name+": GET /auth/session")
		requireUnauthorized(t, fetchFeed(t, baseURL, token, nil), name+": лента")
		requireUnauthorized(t, deleteMeReq(t, baseURL, token), name+": повторное удаление")
	}

	requireUserNotFound(t, fetchUser(t, baseURL, masha.token, kolya.id), "профиль удалённого")
	requireUserNotFound(t, fetchUserPosts(t, baseURL, masha.token, kolya.id, feedParams(50, "")), "посты удалённого")
	requireUserNotFound(t, follow(t, baseURL, masha.token, kolya.id), "подписка на удалённого")
	requireUserNotFound(t, blockReq(t, baseURL, masha.token, kolya.id), "блокировка удалённого")

	if n := countSQL(t, `SELECT count(*) FROM users WHERE id = $1`, kolya.id); n != 0 {
		t.Errorf("удаление жёсткое, а строка пользователя осталась (%d)", n)
	}
	if n := countSQL(t, `SELECT count(*) FROM users WHERE phone = $1`, "+79007000001"); n != 0 {
		t.Errorf("номер удалённого остался в базе: строк %d", n)
	}
}

// Посты удалённого пропадают у всех, файлы их фотографий, неопубликованных
// фотографий и аватара не отдаются; чужие посты на месте («Посты
// удалённого», «Аватар удалённого», ФТ-21, ФТ-22).
func TestDeleteAccountRemovesPostsAndFiles(t *testing.T) {
	baseURL := startAPI(t)
	kolya := newDachnik(t, baseURL, 1)
	masha := newDachnik(t, baseURL, 2)

	first := postWithPhotos(t, baseURL, kolya.token, 3)
	second := postWithPhotos(t, baseURL, kolya.token, 1)
	unpublished := photoOf(t, baseURL, kolya.token, 70, 50)
	avatar := avatarOf(t, baseURL, kolya.token, imageBytes(t, "png", 200, 200))
	mashaPost := publishPost(t, baseURL, masha.token, "Маша")

	// Под своим постом — комментарий и лайк соседки: они уходят вместе
	// с постом.
	likeOK(t, baseURL, masha, first.ID)
	commentOf(t, baseURL, masha.token, first.ID, commentText)

	var files []string
	for _, post := range []postPayload{first, second} {
		for _, photo := range post.Media {
			files = append(files, photo.URL)
			downloadFile(t, baseURL, photo.URL)
		}
	}
	files = append(files, unpublished.URL, avatar)
	downloadFile(t, baseURL, unpublished.URL)
	downloadFile(t, baseURL, avatar)

	deleteMeOK(t, baseURL, kolya.token)

	requirePostGone(t, baseURL, masha.token, first.ID, "Маша, первый пост Коли")
	requirePostGone(t, baseURL, masha.token, second.ID, "Маша, второй пост Коли")
	requirePostAlive(t, baseURL, masha.token, mashaPost.ID, "Маша, свой пост")

	for _, link := range files {
		requireFileGone(t, baseURL, link)
	}

	for _, table := range []string{"posts", "media", "comments"} {
		requireNoRows(t, table, "author_id", kolya.id, "после удаления аккаунта")
	}
	requireNoRows(t, "comments", "post_id", first.ID, "комментарии под постом удалённого")
}

// Комментарии и лайки удалённого под чужими постами пропадают, числа
// comments и likes уменьшаются («Комментарии удалённого под чужими
// постами», «Лайки удалённого», ФТ-21).
func TestDeleteAccountRemovesCommentsAndLikesUnderOthersPosts(t *testing.T) {
	baseURL := startAPI(t)
	kolya := newDachnik(t, baseURL, 1)
	masha := newDachnik(t, baseURL, 2)
	petya := newDachnik(t, baseURL, 3)

	post := publishPost(t, baseURL, masha.token, "Маша")
	commentOf(t, baseURL, kolya.token, post.ID, "от Коли 1")
	commentOf(t, baseURL, petya.token, post.ID, "от Пети")
	commentOf(t, baseURL, kolya.token, post.ID, "от Коли 2")
	likeOK(t, baseURL, kolya, post.ID)
	likeOK(t, baseURL, petya, post.ID)

	requireCommentCountEverywhere(t, baseURL, masha.token, post.ID, 3, "Маша до удаления")
	requireLikesEverywhere(t, baseURL, masha.token, post.ID, 2, false, "Маша до удаления")

	deleteMeOK(t, baseURL, kolya.token)

	requireCommentTexts(t, baseURL, masha.token, post.ID, []string{"от Пети"}, "Маша после удаления Коли")
	requireCommentCountEverywhere(t, baseURL, masha.token, post.ID, 1, "Маша после удаления")
	requireCommentCountEverywhere(t, baseURL, petya.token, post.ID, 1, "Петя после удаления")
	requireLikesEverywhere(t, baseURL, masha.token, post.ID, 1, false, "Маша после удаления")
	requireLikesEverywhere(t, baseURL, petya.token, post.ID, 1, true, "Петя после удаления")
}

// Подписки, подписчики и заявки удалённого пропадают у обеих сторон,
// счётчики уменьшаются; его блокировки и блокировки его — тоже
// («Подписчики и подписки удалённого», ФТ-21).
func TestDeleteAccountRemovesFollowsRequestsAndBlocks(t *testing.T) {
	baseURL := startAPI(t)
	kolya := newDachnik(t, baseURL, 1)
	masha := newDachnik(t, baseURL, 2)
	petya := newDachnik(t, baseURL, 3)
	zina := newDachnik(t, baseURL, 4)
	valya := newDachnik(t, baseURL, 5)

	followOK(t, baseURL, kolya, masha.id)
	followOK(t, baseURL, petya, kolya.id)
	followOK(t, baseURL, petya, masha.id)
	setClosed(t, baseURL, zina.token, true)
	followOK(t, baseURL, kolya, zina.id)
	blockOK(t, baseURL, valya, kolya.id)

	requireCounts(t, baseURL, masha.token, masha.id, 2, 0, "Маша до удаления")
	requireCounts(t, baseURL, petya.token, petya.id, 0, 2, "Петя до удаления")

	deleteMeOK(t, baseURL, kolya.token)

	requireCounts(t, baseURL, masha.token, masha.id, 1, 0, "Маша после удаления")
	requireCounts(t, baseURL, petya.token, petya.id, 0, 1, "Петя после удаления")
	if got := walkFollowList(t, baseURL, masha.token, masha.id, "followers", 50); !reflect.DeepEqual(got, []string{petya.id}) {
		t.Errorf("подписчики Маши %v, ожидался только Петя", got)
	}
	if got := walkFollowList(t, baseURL, petya.token, petya.id, "following", 50); !reflect.DeepEqual(got, []string{masha.id}) {
		t.Errorf("подписки Пети %v, ожидалась только Маша", got)
	}
	if ids := requesterIDs(t, baseURL, zina.token); len(ids) != 0 {
		t.Errorf("у Зины осталась заявка удалённого: %v", ids)
	}
	if ids := blockedIDs(t, baseURL, valya.token); len(ids) != 0 {
		t.Errorf("у Вали в заблокированных остался удалённый: %v", ids)
	}
}

// Удалённый блокировал кого-то — его блокировка уходит вместе с ним:
// заблокированный снова видит всех как обычно (ФТ-21).
func TestDeleteAccountRemovesBlocksMadeByIt(t *testing.T) {
	baseURL := startAPI(t)
	kolya := newDachnik(t, baseURL, 1)
	masha := newDachnik(t, baseURL, 2)

	blockOK(t, baseURL, kolya, masha.id)
	deleteMeOK(t, baseURL, kolya.token)

	requireUserNotFound(t, fetchUser(t, baseURL, masha.token, kolya.id), "Маша и профиль удалённого")
	if n := countSQL(t, `SELECT count(*) FROM blocks WHERE blocker_id = $1 OR blocked_id = $1`, kolya.id); n != 0 {
		t.Errorf("блокировки удалённого остались в базе: %d", n)
	}
}

// Ник удалённого свободен: другой может его взять («Ник удалённого»,
// ФТ-24).
func TestDeleteAccountFreesNickname(t *testing.T) {
	baseURL := startAPI(t)
	kolya := newDachnik(t, baseURL, 1)
	masha := newDachnik(t, baseURL, 2)

	chooseNickname(t, baseURL, kolya.token, "kolya_dacha")
	if resp := setNickname(t, baseURL, masha.token, map[string]any{"nickname": "kolya_dacha"}); resp.StatusCode == http.StatusOK {
		t.Fatalf("занятый ник не должен был достаться другому до удаления")
	}

	deleteMeOK(t, baseURL, kolya.token)

	got := chooseNickname(t, baseURL, masha.token, "kolya_dacha")
	requireNickname(t, got, "kolya_dacha", "ник удалённого у Маши")
}

// Уведомления о действиях удалённого пропадают, чужие остаются
// («Уведомления о действиях удалённого», ФТ-21).
func TestDeleteAccountRemovesNotificationsAboutIt(t *testing.T) {
	baseURL := startAPI(t)
	kolya := newDachnik(t, baseURL, 1)
	masha := newDachnik(t, baseURL, 2)
	petya := newDachnik(t, baseURL, 3)

	mashaPost := ownPost(t, baseURL, masha)
	followOK(t, baseURL, kolya, masha.id)
	commentOf(t, baseURL, kolya.token, mashaPost.ID, commentText)
	likeOK(t, baseURL, kolya, mashaPost.ID)
	followOK(t, baseURL, petya, masha.id)

	if got := actorIDs(notificationsOf(t, baseURL, masha)); !contains(got, kolya.id) {
		t.Fatalf("до удаления у Маши нет уведомлений о Коле: %v", got)
	}

	deleteMeOK(t, baseURL, kolya.token)

	items := notificationsOf(t, baseURL, masha)
	requireKinds(t, items, []string{kindFollow}, "уведомления Маши после удаления Коли")
	requireActor(t, items[0], petya.id, "оставшееся уведомление")
}

// Жалоба удалённого на чужой пост остаётся, но без автора; пост на месте
// и виден остальным («Жалоба удалённого на чужой пост», ФТ-23).
func TestDeleteAccountKeepsItsReportsWithoutAuthor(t *testing.T) {
	baseURL := startAPI(t)
	kolya := newDachnik(t, baseURL, 1)
	masha := newDachnik(t, baseURL, 2)
	petya := newDachnik(t, baseURL, 3)

	post := postToReport(t, baseURL, masha.token)
	comment := commentOf(t, baseURL, masha.token, post.ID, commentText)
	requireAccepted(t, reportPostReason(t, baseURL, kolya.token, post.ID, reportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, kolya.token, post.ID, comment.ID, commentReportReason))

	deleteMeOK(t, baseURL, kolya.token)

	requirePostAlive(t, baseURL, petya.token, post.ID, "Петя и пост, на который жаловались")
	requirePostAlive(t, baseURL, masha.token, post.ID, "Маша и свой пост")

	if n := countSQL(t, `SELECT count(*) FROM reports WHERE post_id = $1`, post.ID); n != 1 {
		t.Errorf("жалоба удалённого на пост должна остаться, в базе %d", n)
	}
	if n := countSQL(t, `SELECT count(*) FROM reports WHERE post_id = $1 AND reporter_id IS NULL AND reason = $2`, post.ID, reportReason); n != 1 {
		t.Errorf("жалоба удалённого на пост должна остаться без автора и с причиной, таких %d", n)
	}
	if n := countSQL(t, `SELECT count(*) FROM reports WHERE comment_id = $1 AND reporter_id IS NULL`, comment.ID); n != 1 {
		t.Errorf("жалоба удалённого на комментарий должна остаться без автора, таких %d", n)
	}
	if n := countSQL(t, `SELECT count(*) FROM reports WHERE reporter_id = $1`, kolya.id); n != 0 {
		t.Errorf("в жалобах осталась ссылка на удалённого: %d", n)
	}
}

// Отзыв удалённого остаётся, а владелец видит автором «удалённый
// пользователь» — без ника и имени («Отзыв удалённого», ФТ-23).
func TestDeleteAccountKeepsFeedbackAsDeletedUser(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	token, userID := fbUser(t, env.baseURL, 1, "Анна", "anna_dacha")

	fb := sendText(t, env.baseURL, token, "Хочу тёмную тему")

	deleteMeOK(t, env.baseURL, token)

	if n := countSQL(t, `SELECT count(*) FROM feedback WHERE id = $1 AND user_id IS NULL`, fb.ID); n != 1 {
		t.Errorf("отзыв удалённого должен остаться без user_id, таких строк %d", n)
	}
	if n := countSQL(t, `SELECT count(*) FROM feedback WHERE user_id = $1`, userID); n != 0 {
		t.Errorf("в отзывах осталась ссылка на удалённого: %d", n)
	}

	var body struct {
		Feedback struct {
			Recent []map[string]any `json:"recent"`
		} `json:"feedback"`
	}
	if err := json.Unmarshal(dashboardRaw(t, env.root), &body); err != nil {
		t.Fatalf("данные дашборда не разобрались: %v", err)
	}
	for _, item := range body.Feedback.Recent {
		id, _ := item["id"].(float64)
		if int64(id) != fb.ID {
			continue
		}
		author, _ := item["author"].(string)
		if !strings.Contains(strings.ToLower(author), deletedAuthor) {
			t.Errorf("автор отзыва удалённого на дашборде %q, ожидался «%s»", author, deletedAuthor)
		}
		if strings.Contains(author, "anna_dacha") || strings.Contains(author, "Анна") {
			t.Errorf("в авторе отзыва удалённого осталось его имя: %q", author)
		}
		if item["text"] != "Хочу тёмную тему" {
			t.Errorf("текст отзыва удалённого %v, ожидался прежний", item["text"])
		}
		return
	}
	t.Fatalf("отзыва %d нет на дашборде после удаления автора", fb.ID)
}

// Отчёт об ошибке удалённого остаётся без ссылки на человека, заходы
// в приложение уходят (ФТ-21, ФТ-23).
func TestDeleteAccountKeepsAppErrorsAndDropsAppSessions(t *testing.T) {
	baseURL := startAPI(t)
	kolya := newDachnik(t, baseURL, 1)

	startAppSession(t, baseURL, kolya.token)
	reportAppError(t, baseURL, kolya.token, map[string]any{
		"error": "Null check operator used on a null value",
		"os":    "android удалённого",
	})

	deleteMeOK(t, baseURL, kolya.token)

	if n := countSQL(t, `SELECT count(*) FROM app_errors WHERE os = $1`, "android удалённого"); n != 1 {
		t.Errorf("отчёт об ошибке удалённого должен остаться, их %d", n)
	}
	if got := appErrorUserID(t, "android удалённого"); got != "" {
		t.Errorf("отчёт об ошибке удалённого должен остаться без человека, а в нём %q", got)
	}
	requireNoRows(t, "app_sessions", "user_id", kolya.id, "заходы удалённого")
}

// Вход по номеру удалённого без нового приглашения — как у незнакомого
// номера; по новому приглашению — новый пользователь с чистого листа
// («Вход по номеру удалённого без нового приглашения», ФТ-24, 015-invites).
func TestDeletedPhoneNeedsNewInvite(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)

	code := issueInvite(t, pool, phonePretty)
	first := requireSignedIn(t, createSession(t, baseURL, phonePretty, code), "первый вход по приглашению")
	chooseNickname(t, baseURL, first.Token, "kolya_dacha")

	deleteMeOK(t, baseURL, first.Token)

	if n := countSQL(t, `SELECT count(*) FROM users WHERE phone = $1`, phoneStored); n != 0 {
		t.Errorf("номер удалённого остался в базе: строк %d", n)
	}

	requireInviteError(t, requestInviteCode(t, baseURL, phonePretty),
		http.StatusForbidden, "not_invited", "запрос кода на номер удалённого без приглашения")
	requireInviteError(t, createSession(t, baseURL, phonePretty, code),
		http.StatusUnauthorized, "invalid_code", "вход прежним кодом на номер удалённого")
	requireInviteError(t, createSession(t, baseURL, phonePretty, authCode),
		http.StatusUnauthorized, "invalid_code", "вход кодом стенда на номер удалённого")

	fresh := issueInviteNotEqual(t, pool, phonePretty, code, authCode)
	second := requireSignedIn(t, createSession(t, baseURL, phonePretty, fresh), "вход по новому приглашению")

	if !second.IsNewUser {
		t.Error("вход по номеру удалённого по новому приглашению должен быть is_new_user=true")
	}
	if second.User.ID == first.User.ID {
		t.Errorf("по номеру удалённого должен завестись новый пользователь, а вернулся прежний %s", first.User.ID)
	}
	user := me(t, baseURL, second.Token)
	if user.Nickname == "kolya_dacha" {
		t.Error("новый пользователь не должен получить ник удалённого")
	}
	if ids := walkUserPosts(t, baseURL, second.Token, second.User.ID, 50); len(ids) != 0 {
		t.Errorf("у нового пользователя посты %v", ids)
	}
}

// Без режима приглашений вход по номеру удалённого заводит нового
// пользователя, а не возвращает прежнего (ФТ-24).
func TestDeletedPhoneSignsInAsNewUser(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 50 * time.Millisecond})
	kolya := newDachnik(t, baseURL, 1)

	deleteMeOK(t, baseURL, kolya.token)
	time.Sleep(60 * time.Millisecond)

	requestCode(t, baseURL, dachnikPhone(1))
	resp := createSession(t, baseURL, dachnikPhone(1), authCode)
	body := requireSignedIn(t, resp, "вход по номеру удалённого")
	if !body.IsNewUser {
		t.Error("вход по номеру удалённого должен быть is_new_user=true")
	}
	if body.User.ID == kolya.id {
		t.Errorf("по номеру удалённого вернулся прежний пользователь %s", kolya.id)
	}
}
