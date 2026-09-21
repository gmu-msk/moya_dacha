package tests

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"
)

// Идентификатор комментария, которого сервис никогда не выдавал: UUID
// правильного вида, но такого комментария нет.
const unknownCommentID = "3c8f2b01-6d4e-4a32-8b9c-7d6e5f4a3b21"

// Идентификатор, который вовсе не похож на UUID: сервис таких не выдаёт,
// и для него ответ тот же, что и для несуществующего («Ограничения
// и edge cases»).
const notAnID = "не-идентификатор"

// --- Хелперы --------------------------------------------------------------

// deletePost удаляет пост по его адресу.
func deletePost(t *testing.T, baseURL, token, postID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, baseURL+"/posts/"+postID, token, nil)
}

// deleteComment удаляет комментарий по адресу его поста: пара «пост
// и комментарий» должна сойтись (ФТ-10).
func deleteComment(t *testing.T, baseURL, token, postID, commentID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, baseURL+"/posts/"+postID+"/comments/"+commentID, token, nil)
}

// requireDeleted требует, чтобы удаление прошло и ответ был пустым:
// отдавать нечего, а пост, которого больше нет, в ответ не положишь
// (ФТ-11).
func requireDeleted(t *testing.T, resp *http.Response) {
	t.Helper()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("на удаление ожидался статус 204, получен %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("не удалось прочитать тело ответа на удаление: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("ответ на удаление обязан быть пустым, пришло %q", body)
	}
}

// postWithPhotos публикует пост из count фотографий с подписью
// и возвращает его: удалению всё равно, что на фотографиях, но важно,
// что они есть и лежат в хранилище.
func postWithPhotos(t *testing.T, baseURL, token string, count int) postPayload {
	t.Helper()

	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		ids = append(ids, photoOf(t, baseURL, token, 60+i, 40+i).ID)
	}

	return createdPost(t, createPost(t, baseURL, token,
		map[string]any{"media_ids": ids, "caption": postCaption}))
}

// requirePostGone требует, чтобы поста не было ни на его адресе, ни
// в ленте: удалённый пост исчезает отовсюду (ФТ-7).
func requirePostGone(t *testing.T, baseURL, token, postID, where string) {
	t.Helper()

	resp := fetchPost(t, baseURL, token, postID)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("%s: пост удалён, а его адрес отвечает %d", where, resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "post_not_found" {
		t.Fatalf("%s: на адресе удалённого поста ожидалась ошибка post_not_found, получена %q", where, code)
	}

	requireNotInFeed(t, baseURL, token, postID, where)
}

// requireNotInFeed требует, чтобы поста не было в ленте.
func requireNotInFeed(t *testing.T, baseURL, token, postID, where string) {
	t.Helper()

	for _, id := range postIDs(feedPage(t, baseURL, token, feedParams(feedMaxLimit, "")).Items) {
		if id == postID {
			t.Fatalf("%s: удалённый пост %s всё ещё стоит в ленте", where, postID)
		}
	}
}

// requirePostAlive требует, чтобы пост остался на месте: неудавшееся
// удаление ничего не трогает («Чужой пост», «Остальные посты»).
func requirePostAlive(t *testing.T, baseURL, token, postID, where string) {
	t.Helper()

	post := fetchedPost(t, fetchPost(t, baseURL, token, postID))
	if post.ID != postID {
		t.Fatalf("%s: по адресу поста %s отдан пост %s", where, postID, post.ID)
	}

	for _, id := range postIDs(feedPage(t, baseURL, token, feedParams(feedMaxLimit, "")).Items) {
		if id == postID {
			return
		}
	}

	t.Fatalf("%s: пост %s пропал из ленты, а его не удаляли", where, postID)
}

// requireDeletedPhotoGone требует, чтобы фотография удалённого поста
// перестала отдаваться по своему адресу. Ждать нечего: пока хранилище —
// локальный диск, файлы убираются в том же запросе, сразу после того, как
// транзакция прошла (ФТ-5, ADR-0007, «Уточнение»).
func requireDeletedPhotoGone(t *testing.T, baseURL string, photo mediaPayload) {
	t.Helper()
	requireFileGone(t, baseURL, photo.URL)
}

// countSQL считает строки в базе: флага «удалён» в модели нет, поэтому
// удалённое проверяется тем, что его в таблице нет (ФТ-4, ADR-0007).
func countSQL(t *testing.T, sql string, args ...any) int {
	t.Helper()

	pool := connect(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int
	if err := pool.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		t.Fatalf("не удалось посчитать строки в базе: %v", err)
	}

	return count
}

// requireNoRows требует, чтобы строк с таким post_id не осталось ни
// в одной из связанных таблиц.
func requireNoRows(t *testing.T, table, column, id, where string) {
	t.Helper()

	count := countSQL(t, "SELECT count(*) FROM "+table+" WHERE "+column+" = $1", id)
	if count != 0 {
		t.Errorf("%s: в таблице %s осталось строк: %d — удаление жёсткое, флага «удалён» нет", where, table, count)
	}
}

// --- DELETE /api/posts/{postId}: удалить свой пост ------------------------

// Автор удаляет свой пост: 204, тела нет, и поста нет ни на его адресе,
// ни в ленте («Автор удаляет свой пост», ФТ-1, ФТ-7, ФТ-11,
// пользовательский сценарий, шаг 3).
func TestDeleteOwnPostRemovesItFromItsAddressAndFromTheFeed(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postWithPhotos(t, baseURL, token, 1)

	requirePostAlive(t, baseURL, token, post.ID, "перед удалением")

	requireDeleted(t, deletePost(t, baseURL, token, post.ID))

	requirePostGone(t, baseURL, token, post.ID, "после удаления")
}

// Чужому пост тоже перестаёт быть виден: лента одна на всех, и удалённого
// поста в ней нет ни у кого (ФТ-7, 004-feed).
func TestDeletedPostIsGoneForEveryoneNotOnlyForItsAuthor(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	viewer, _ := signIn(t, baseURL, otherPhonePretty)

	post := postWithPhotos(t, baseURL, author, 1)
	requirePostAlive(t, baseURL, viewer, post.ID, "сосед перед удалением")

	requireDeleted(t, deletePost(t, baseURL, author, post.ID))

	requirePostGone(t, baseURL, viewer, post.ID, "сосед после удаления")
}

// Вместе с постом уходят его фотографии: по своим адресам они больше
// не отдаются («Удаление уносит фотографии поста», ФТ-1, ФТ-5).
func TestDeletePostRemovesItsPhotosFromStorage(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postWithPhotos(t, baseURL, token, 4)

	if len(post.Media) != 4 {
		t.Fatalf("в посте ожидалось 4 фотографии, получено %d", len(post.Media))
	}

	for _, photo := range post.Media {
		// Пока пост жив, файл отдаётся: иначе проверять после удаления
		// было бы нечего.
		requireStoredPhoto(t, baseURL, photo)
	}

	requireDeleted(t, deletePost(t, baseURL, token, post.ID))

	for _, photo := range post.Media {
		requireDeletedPhotoGone(t, baseURL, photo)
	}
}

// Фотографии соседнего поста удаление не трогает: уходят только файлы
// того поста, который удалили (ФТ-5, «Остальные посты»).
func TestDeletePostKeepsPhotosOfOtherPosts(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	doomed := postWithPhotos(t, baseURL, token, 2)
	survivor := postWithPhotos(t, baseURL, token, 2)

	requireDeleted(t, deletePost(t, baseURL, token, doomed.ID))

	for _, photo := range doomed.Media {
		requireDeletedPhotoGone(t, baseURL, photo)
	}
	for _, photo := range survivor.Media {
		requireStoredPhoto(t, baseURL, photo)
	}
}

// Вместе с постом уходят его лайки, а у остальных постов лайки остаются
// на месте («Удаление уносит лайки поста», ФТ-1).
func TestDeletePostRemovesItsLikesAndKeepsLikesOfOtherPosts(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	neighbour, _ := signIn(t, baseURL, otherPhonePretty)

	doomed := postWithPhotos(t, baseURL, author, 1)
	survivor := postWithPhotos(t, baseURL, author, 1)

	likedOK(t, likePost(t, baseURL, neighbour, doomed.ID), http.StatusOK)
	likedOK(t, likePost(t, baseURL, author, doomed.ID), http.StatusOK)
	likedOK(t, likePost(t, baseURL, neighbour, survivor.ID), http.StatusOK)

	requireDeleted(t, deletePost(t, baseURL, author, doomed.ID))

	// Лайк живёт вместе с постом: поста нет — и лайкать нечего.
	requireError(t, likePost(t, baseURL, neighbour, doomed.ID), http.StatusNotFound, "post_not_found")
	requireNoRows(t, "post_likes", "post_id", doomed.ID, "лайки удалённого поста")

	// У соседнего поста лайк на месте и считается тому, кто спрашивает.
	requireLikesEverywhere(t, baseURL, neighbour, survivor.ID, 1, true, "сосед")
	requireLikesEverywhere(t, baseURL, author, survivor.ID, 1, false, "автор")
}

// Вместе с постом уходят его комментарии, в том числе чужие: комментарий
// не существует отдельно от поста («Удаление уносит комментарии поста,
// включая чужие», ФТ-1, CONTEXT.md).
func TestDeletePostRemovesItsCommentsIncludingThoseOfOtherPeople(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	guest, _ := signIn(t, baseURL, otherPhonePretty)

	post := postWithPhotos(t, baseURL, author, 1)

	commentOf(t, baseURL, author, post.ID, "Своя реплика")
	commentOf(t, baseURL, guest, post.ID, commentText)

	requireDeleted(t, deletePost(t, baseURL, author, post.ID))

	requireError(t, fetchComments(t, baseURL, guest, post.ID), http.StatusNotFound, "post_not_found")
	requireError(t, addCommentText(t, baseURL, guest, post.ID, commentText), http.StatusNotFound, "post_not_found")
	requireNoRows(t, "comments", "post_id", post.ID, "комментарии удалённого поста")
}

// Удаление необратимо: из базы уходит и сам пост, и всё, что к нему
// привязано. Флага «удалён» нет — проверяется тем, что строк не осталось
// (ФТ-4, ADR-0007, «Модель данных»).
func TestDeletedPostLeavesNoRowsInTheDatabase(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	guest, _ := signIn(t, baseURL, otherPhonePretty)

	post := postWithPhotos(t, baseURL, author, 2)
	commentOf(t, baseURL, guest, post.ID, commentText)
	likedOK(t, likePost(t, baseURL, guest, post.ID), http.StatusOK)

	requireDeleted(t, deletePost(t, baseURL, author, post.ID))

	requireNoRows(t, "posts", "id", post.ID, "сам пост")
	requireNoRows(t, "media", "post_id", post.ID, "фотографии поста")
	requireNoRows(t, "comments", "post_id", post.ID, "комментарии поста")
	requireNoRows(t, "post_likes", "post_id", post.ID, "лайки поста")
}

// Чужой пост не удаляется, и сервис говорит об этом прямо: 403
// not_your_post, а не «такого поста нет». Пост остаётся на месте
// («Чужой пост», ФТ-3, ФТ-9).
func TestDeleteAnotherUsersPostIsForbiddenAndLeavesItInPlace(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	stranger, _ := signIn(t, baseURL, otherPhonePretty)

	post := postWithPhotos(t, baseURL, author, 1)

	requireError(t, deletePost(t, baseURL, stranger, post.ID), http.StatusForbidden, "not_your_post")

	requirePostAlive(t, baseURL, author, post.ID, "после чужой попытки удалить")
	for _, photo := range post.Media {
		requireStoredPhoto(t, baseURL, photo)
	}
}

// Повторное удаление своего поста — ошибка, а не «и так всё в порядке»:
// второй запрос означает, что человек видит то, чего нет
// («Повторное удаление своего поста», ФТ-6).
func TestDeleteOwnPostTwiceIsNotFound(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postWithPhotos(t, baseURL, token, 1)

	requireDeleted(t, deletePost(t, baseURL, token, post.ID))
	requireError(t, deletePost(t, baseURL, token, post.ID), http.StatusNotFound, "post_not_found")
}

// Поста нет — 404 post_not_found. Не похожий на UUID идентификатор — тот
// же ответ: поста с таким идентификатором нет («Несуществующий пост»,
// «Идентификатор поста не похож на UUID»).
func TestDeleteUnknownPostIsNotFound(t *testing.T) {
	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownID,
		"вовсе не UUID":                 notAnID,
	}

	for caseName, id := range ids {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			requireError(t, deletePost(t, baseURL, token, id), http.StatusNotFound, "post_not_found")
		})
	}
}

// Существование проверяется раньше, чем «своё ли»: тот, у кого чужие
// посты есть, на удаление несуществующего получает 404, а не 403
// («Удаление несуществующего чужого поста», ФТ-8).
func TestDeleteUnknownPostOfSomeoneElseIsNotFoundNotForbidden(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	stranger, _ := signIn(t, baseURL, otherPhonePretty)

	// У автора пост есть, но удаляется не он, а тот, которого нет.
	postWithPhotos(t, baseURL, author, 1)

	requireError(t, deletePost(t, baseURL, stranger, unknownID), http.StatusNotFound, "post_not_found")
}

// Удалять может только вошедший: без токена, с неизвестным токеном
// и с токеном закончившейся сессии — 401 unauthorized, и пост остаётся
// («Удаление поста без токена», ФТ-13, 001-auth).
func TestDeletePostRequiresValidToken(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postWithPhotos(t, baseURL, token, 1)

	goneToken, _ := signIn(t, baseURL, thirdPhonePretty)
	if resp := signOut(t, baseURL, goneToken); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("на выход ожидался статус 204, получен %d", resp.StatusCode)
	}

	tokens := map[string]string{
		"без токена":         "",
		"неизвестный токен":  "нет-такого-токена",
		"токен после выхода": goneToken,
	}

	for name, bad := range tokens {
		t.Run(name, func(t *testing.T) {
			requireError(t, deletePost(t, baseURL, bad, post.ID), http.StatusUnauthorized, "unauthorized")
		})
	}

	requirePostAlive(t, baseURL, token, post.ID, "после попыток удалить без токена")
}

// Токен проверяется первым: удаление чужого поста без токена — 401,
// а не 403 («Удаление чужого поста без токена», ФТ-8).
func TestDeleteAnotherUsersPostWithoutTokenIsUnauthorized(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	post := postWithPhotos(t, baseURL, author, 1)

	requireError(t, deletePost(t, baseURL, "", post.ID), http.StatusUnauthorized, "unauthorized")

	requirePostAlive(t, baseURL, author, post.ID, "после попытки удалить без токена")
}

// Токен проверяется раньше существования: удаление несуществующего поста
// без токена — 401, а не 404 (ФТ-8).
func TestDeleteUnknownPostWithoutTokenIsUnauthorized(t *testing.T) {
	baseURL := startAPI(t)

	requireError(t, deletePost(t, baseURL, "", unknownID), http.StatusUnauthorized, "unauthorized")
}

// Ни одно удаление ничего не возвращает: 204 и пустое тело — и у поста,
// и у комментария (ФТ-11).
func TestDeleteAnswersWithNoContentAndEmptyBody(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	post := postWithPhotos(t, baseURL, token, 1)
	comment := commentOf(t, baseURL, token, post.ID, commentText)

	requireDeleted(t, deleteComment(t, baseURL, token, post.ID, comment.ID))
	requireDeleted(t, deletePost(t, baseURL, token, post.ID))
}

// Удаление одного поста не трогает ни соседние посты, ни их комментарии:
// уходит ровно то, что удаляли («Остальные посты и комментарии»).
func TestDeletePostLeavesOtherPostsAndTheirCommentsAlone(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	neighbour, _ := signIn(t, baseURL, otherPhonePretty)

	doomed := postWithPhotos(t, baseURL, author, 1)
	mine := postWithPhotos(t, baseURL, author, 1)
	theirs := postWithPhotos(t, baseURL, neighbour, 1)

	commentOf(t, baseURL, neighbour, doomed.ID, "Этот разговор уйдёт вместе с постом")
	commentOf(t, baseURL, neighbour, mine.ID, "А этот остаётся")
	commentOf(t, baseURL, author, theirs.ID, commentText)

	requireDeleted(t, deletePost(t, baseURL, author, doomed.ID))

	requirePostAlive(t, baseURL, author, mine.ID, "соседний свой пост")
	requirePostAlive(t, baseURL, author, theirs.ID, "соседний чужой пост")

	requireCommentTexts(t, baseURL, author, mine.ID, []string{"А этот остаётся"}, "соседний свой пост")
	requireCommentTexts(t, baseURL, author, theirs.ID, []string{commentText}, "соседний чужой пост")

	requireCommentCountEverywhere(t, baseURL, author, mine.ID, 1, "соседний свой пост")
	requireCommentCountEverywhere(t, baseURL, author, theirs.ID, 1, "соседний чужой пост")
}

// Лента после удаления: поста в ней нет, остальные на месте, и курсор,
// выданный до удаления, продолжает листать — он указывает на место,
// а не на строку («Лента после удаления», ФТ-7, ADR-0015).
func TestFeedKeepsPagingWithItsCursorAfterDeletingAPost(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	published := publishPosts(t, baseURL, token, 5)
	newest := newestFirst(published)

	first := feedPage(t, baseURL, token, feedParams(2, ""))
	requireFeedPosts(t, first, newest[:2], "первая страница до удаления")
	cursor := cursorOf(t, first, "первая страница до удаления")

	// Удаляется ровно тот пост, на котором кончилась первая страница:
	// именно на него указывает выданный курсор.
	requireDeleted(t, deletePost(t, baseURL, token, newest[1]))

	second := feedPage(t, baseURL, token, feedParams(2, cursor))
	requireFeedPosts(t, second, newest[2:4], "вторая страница по прежнему курсору")

	third := feedPage(t, baseURL, token, feedParams(2, cursorOf(t, second, "вторая страница")))
	requireFeedPosts(t, third, newest[4:], "третья страница")
	requireFeedEnd(t, third, "третья страница")

	// И проход ленты с начала отдаёт прежние посты без удалённого.
	want := append([]string{newest[0]}, newest[2:]...)
	if got := walkFeed(t, baseURL, token, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("после удаления лента ожидалась %v, получена %v", want, got)
	}
}

// --- DELETE /api/posts/{postId}/comments/{commentId} ----------------------

// Автор удаляет свой комментарий: 204, комментария под постом больше нет,
// а число комментариев у поста уменьшилось («Автор удаляет свой
// комментарий», ФТ-2, пользовательский сценарий, шаг 4).
func TestDeleteOwnCommentLowersTheCommentCount(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postWithPhotos(t, baseURL, token, 1)

	comment := commentOf(t, baseURL, token, post.ID, commentText)
	commentOf(t, baseURL, token, post.ID, "Вторая реплика остаётся")

	requireCommentCountEverywhere(t, baseURL, token, post.ID, 2, "до удаления")

	requireDeleted(t, deleteComment(t, baseURL, token, post.ID, comment.ID))

	requireCommentTexts(t, baseURL, token, post.ID, []string{"Вторая реплика остаётся"}, "после удаления")
	requireCommentCountEverywhere(t, baseURL, token, post.ID, 1, "после удаления")
}

// Пост при удалении комментария не трогается: он на месте со своей
// подписью и своими фотографиями (ФТ-2).
func TestDeleteOwnCommentDoesNotTouchThePost(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postWithPhotos(t, baseURL, token, 2)

	comment := commentOf(t, baseURL, token, post.ID, commentText)

	requireDeleted(t, deleteComment(t, baseURL, token, post.ID, comment.ID))

	alive := fetchedPost(t, fetchPost(t, baseURL, token, post.ID))
	if alive.Caption != post.Caption {
		t.Errorf("подпись поста изменилась: была %q, стала %q", post.Caption, alive.Caption)
	}
	requireMediaOrder(t, alive, post.Media)
	for _, photo := range post.Media {
		requireStoredPhoto(t, baseURL, photo)
	}
}

// Удаление комментария необратимо: строки в базе не остаётся (ФТ-4,
// ADR-0007).
func TestDeletedCommentLeavesNoRowInTheDatabase(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postWithPhotos(t, baseURL, token, 1)

	comment := commentOf(t, baseURL, token, post.ID, commentText)

	requireDeleted(t, deleteComment(t, baseURL, token, post.ID, comment.ID))

	requireNoRows(t, "comments", "id", comment.ID, "удалённый комментарий")
	if count := countSQL(t, `SELECT count(*) FROM posts WHERE id = $1`, post.ID); count != 1 {
		t.Errorf("пост при удалении комментария не трогается, а строк в posts осталось %d", count)
	}
}

// Свой комментарий удаляется и под чужим постом: комментарий свой,
// а чей пост — неважно («Свой комментарий под чужим постом», ФТ-2).
func TestDeleteOwnCommentUnderAnotherUsersPostIsAllowed(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	guest, _ := signIn(t, baseURL, otherPhonePretty)

	post := postWithPhotos(t, baseURL, author, 1)
	comment := commentOf(t, baseURL, guest, post.ID, commentText)

	requireDeleted(t, deleteComment(t, baseURL, guest, post.ID, comment.ID))

	requireCommentTexts(t, baseURL, author, post.ID, []string{}, "чужой пост после удаления гостем своей реплики")
	requireCommentCountEverywhere(t, baseURL, author, post.ID, 0, "чужой пост")
	requirePostAlive(t, baseURL, author, post.ID, "чужой пост")
}

// Чужой комментарий не удаляется даже автором поста, под которым он
// оставлен: чужое убирают жалобой («Чужой комментарий под своим постом»,
// ФТ-3).
func TestDeleteAnotherUsersCommentUnderOwnPostIsForbidden(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	guest, _ := signIn(t, baseURL, otherPhonePretty)

	post := postWithPhotos(t, baseURL, author, 1)
	comment := commentOf(t, baseURL, guest, post.ID, commentText)

	requireError(t, deleteComment(t, baseURL, author, post.ID, comment.ID),
		http.StatusForbidden, "not_your_comment")

	requireCommentTexts(t, baseURL, author, post.ID, []string{commentText}, "после чужой попытки удалить")
	requireCommentCountEverywhere(t, baseURL, author, post.ID, 1, "после чужой попытки удалить")
}

// Чужой комментарий под чужим постом — тем более не удаляется: 403
// not_your_comment («Чужой комментарий под чужим постом», ФТ-3).
func TestDeleteAnotherUsersCommentUnderAnotherUsersPostIsForbidden(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	guest, _ := signIn(t, baseURL, otherPhonePretty)
	stranger, _ := signIn(t, baseURL, thirdPhonePretty)

	post := postWithPhotos(t, baseURL, author, 1)
	comment := commentOf(t, baseURL, guest, post.ID, commentText)

	requireError(t, deleteComment(t, baseURL, stranger, post.ID, comment.ID),
		http.StatusForbidden, "not_your_comment")

	requireCommentTexts(t, baseURL, author, post.ID, []string{commentText}, "после попытки постороннего")
}

// Повторное удаление своего комментария — 404 comment_not_found: второй
// запрос означает, что человек видит то, чего нет («Повторное удаление
// своего комментария», ФТ-6).
func TestDeleteOwnCommentTwiceIsNotFound(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postWithPhotos(t, baseURL, token, 1)
	comment := commentOf(t, baseURL, token, post.ID, commentText)

	requireDeleted(t, deleteComment(t, baseURL, token, post.ID, comment.ID))
	requireError(t, deleteComment(t, baseURL, token, post.ID, comment.ID),
		http.StatusNotFound, "comment_not_found")
}

// Пара «пост и комментарий» должна сойтись: свой комментарий, названный
// по адресу другого поста, — это 404 comment_not_found, а не удаление.
// Сам комментарий при этом остаётся там, где лежал («Комментарий,
// лежащий под другим постом», ФТ-10).
func TestDeleteCommentByTheAddressOfAnotherPostIsNotFound(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	here := postWithPhotos(t, baseURL, token, 1)
	there := postWithPhotos(t, baseURL, token, 1)

	comment := commentOf(t, baseURL, token, there.ID, commentText)

	requireError(t, deleteComment(t, baseURL, token, here.ID, comment.ID),
		http.StatusNotFound, "comment_not_found")

	requireCommentTexts(t, baseURL, token, there.ID, []string{commentText}, "комментарий под своим постом")
	requireCommentCountEverywhere(t, baseURL, token, there.ID, 1, "пост комментария")
}

// Сначала должен найтись пост: комментарий у несуществующего поста — это
// 404 post_not_found. Не похожий на UUID идентификатор поста — тот же
// ответ («Комментарий у несуществующего поста», «Идентификатор поста
// не похож на UUID», ФТ-10).
func TestDeleteCommentOfUnknownPostIsPostNotFound(t *testing.T) {
	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownID,
		"вовсе не UUID":                 notAnID,
	}

	for caseName, postID := range ids {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			// Комментарий настоящий, свой — но лежит он под другим постом,
			// и отвечает сервис всё равно про пост.
			post := postWithPhotos(t, baseURL, token, 1)
			comment := commentOf(t, baseURL, token, post.ID, commentText)

			requireError(t, deleteComment(t, baseURL, token, postID, comment.ID),
				http.StatusNotFound, "post_not_found")

			requireCommentTexts(t, baseURL, token, post.ID, []string{commentText}, "свой пост")
		})
	}
}

// Комментария нет — 404 comment_not_found. Не похожий на UUID
// идентификатор комментария — тот же ответ («Повторное удаление»,
// «Идентификатор комментария не похож на UUID»).
func TestDeleteUnknownCommentIsNotFound(t *testing.T) {
	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownCommentID,
		"вовсе не UUID":                 notAnID,
	}

	for caseName, commentID := range ids {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)
			post := postWithPhotos(t, baseURL, token, 1)

			requireError(t, deleteComment(t, baseURL, token, post.ID, commentID),
				http.StatusNotFound, "comment_not_found")
		})
	}
}

// Удалять комментарий может только вошедший: без токена, с неизвестным
// токеном и с токеном закончившейся сессии — 401 unauthorized,
// и комментарий остаётся («Удаление комментария без токена», ФТ-13).
func TestDeleteCommentRequiresValidToken(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postWithPhotos(t, baseURL, token, 1)
	comment := commentOf(t, baseURL, token, post.ID, commentText)

	goneToken, _ := signIn(t, baseURL, thirdPhonePretty)
	if resp := signOut(t, baseURL, goneToken); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("на выход ожидался статус 204, получен %d", resp.StatusCode)
	}

	tokens := map[string]string{
		"без токена":         "",
		"неизвестный токен":  "нет-такого-токена",
		"токен после выхода": goneToken,
	}

	for name, bad := range tokens {
		t.Run(name, func(t *testing.T) {
			requireError(t, deleteComment(t, baseURL, bad, post.ID, comment.ID),
				http.StatusUnauthorized, "unauthorized")
		})
	}

	requireCommentTexts(t, baseURL, token, post.ID, []string{commentText}, "после попыток удалить без токена")
}

// Токен проверяется первым и у комментария: запрос без токена к чужому
// комментарию — 401, а не 403; к комментарию несуществующего поста —
// тоже 401, а не 404 (ФТ-8, ФТ-13).
func TestDeleteCommentWithoutTokenIsUnauthorizedBeforeAnyOtherCheck(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	guest, _ := signIn(t, baseURL, otherPhonePretty)

	post := postWithPhotos(t, baseURL, author, 1)
	comment := commentOf(t, baseURL, guest, post.ID, commentText)

	cases := map[string][2]string{
		"чужой комментарий":                 {post.ID, comment.ID},
		"комментарий несуществующего поста": {unknownID, comment.ID},
		"несуществующий комментарий":        {post.ID, unknownCommentID},
	}

	for name, pair := range cases {
		t.Run(name, func(t *testing.T) {
			requireError(t, deleteComment(t, baseURL, "", pair[0], pair[1]),
				http.StatusUnauthorized, "unauthorized")
		})
	}

	requireCommentTexts(t, baseURL, author, post.ID, []string{commentText}, "после попыток удалить без токена")
}

// Удаление одного комментария не трогает остальные — ни свои, ни чужие,
// ни под другими постами («Остальные посты и комментарии»).
func TestDeleteCommentLeavesOtherCommentsAlone(t *testing.T) {
	baseURL := startAPI(t)

	author, _ := signIn(t, baseURL, phonePretty)
	guest, _ := signIn(t, baseURL, otherPhonePretty)

	post := postWithPhotos(t, baseURL, author, 1)
	neighbour := postWithPhotos(t, baseURL, author, 1)

	commentOf(t, baseURL, guest, post.ID, "Чужая реплика до")
	doomed := commentOf(t, baseURL, author, post.ID, commentText)
	commentOf(t, baseURL, guest, post.ID, "Чужая реплика после")
	commentOf(t, baseURL, author, neighbour.ID, "Реплика под соседним постом")

	requireDeleted(t, deleteComment(t, baseURL, author, post.ID, doomed.ID))

	requireCommentTexts(t, baseURL, author, post.ID,
		[]string{"Чужая реплика до", "Чужая реплика после"}, "пост после удаления своей реплики")
	requireCommentCountEverywhere(t, baseURL, author, post.ID, 2, "пост после удаления своей реплики")

	requireCommentTexts(t, baseURL, author, neighbour.ID,
		[]string{"Реплика под соседним постом"}, "соседний пост")
	requireCommentCountEverywhere(t, baseURL, author, neighbour.ID, 1, "соседний пост")
}
