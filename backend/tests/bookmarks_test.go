package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// Тесты закладок (specs/032-bookmarks.md). Написаны по спецификации
// и контракту, не глядя в реализацию (ADR-0002). Каждый тест ссылается
// на строку раздела «Ограничения и edge cases» и на требование (ФТ-N).
// Требование 11 по HTTP не проверяется, 12–17 — про приложение.

// --- Представления из контракта -------------------------------------------

// bookmarkPostPayload — пост с полями закладок (schema Post, `bookmarks`
// и `bookmarked`) и лайков. Поля — указатели: сервис с закладками обязан
// отдавать их всегда («Пост в ответе»), отсутствие — ошибка, а не ноль.
// Отдельный тип, а не расширенный postPayload: остальные фичи про
// закладки не знают.
type bookmarkPostPayload struct {
	postPayload
	Likes      *int  `json:"likes"`
	Liked      *bool `json:"liked"`
	Bookmarks  *int  `json:"bookmarks"`
	Bookmarked *bool `json:"bookmarked"`
}

// bookmarkFeedPayload — страница постов (schema Feed): лента, посты
// профиля, лента группы, «Сохранённые». items — указатель: пустой список
// приходит как [], а не без поля.
type bookmarkFeedPayload struct {
	Items      *[]bookmarkPostPayload `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}

// bookmarkTag — тэг поста в тесте полей: слово из стартового словаря.
const bookmarkTag = "томаты"

// --- Хелперы: запросы -----------------------------------------------------

// bookmarkURL — адрес закладки поста.
func bookmarkURL(baseURL, postID string) string {
	return baseURL + "/posts/" + postID + "/bookmark"
}

// bookmarkPutReq добавляет пост в закладки.
func bookmarkPutReq(t *testing.T, baseURL, token, postID string) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, bookmarkURL(baseURL, postID), token, nil)
}

// bookmarkDeleteReq убирает пост из закладок.
func bookmarkDeleteReq(t *testing.T, baseURL, token, postID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, bookmarkURL(baseURL, postID), token, nil)
}

// bookmarkListReq — GET /me/bookmarks с готовой строкой запроса: она нужна
// там, где параметр нарочно неправильный.
func bookmarkListReq(t *testing.T, baseURL, token, query string) *http.Response {
	t.Helper()

	address := baseURL + "/me/bookmarks"
	if query != "" {
		address += "?" + query
	}

	return do(t, http.MethodGet, address, token, nil)
}

// --- Хелперы: разбор ------------------------------------------------------

// bookmarkRequireFields требует оба поля закладок у поста.
func bookmarkRequireFields(t *testing.T, post bookmarkPostPayload, where string) {
	t.Helper()

	if post.Bookmarks == nil {
		t.Fatalf("%s: у поста %s нет поля bookmarks, а сервис отдаёт его всегда", where, post.ID)
	}
	if post.Bookmarked == nil {
		t.Fatalf("%s: у поста %s нет поля bookmarked, а сервис отдаёт его всегда", where, post.ID)
	}
}

// bookmarkPostOK требует статус и возвращает пост с полями закладок.
func bookmarkPostOK(t *testing.T, resp *http.Response, status int, where string) bookmarkPostPayload {
	t.Helper()

	if resp.StatusCode != status {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус %d, получен %d %s", where, status, resp.StatusCode, code)
	}

	var post bookmarkPostPayload
	decode(t, resp, &post)
	bookmarkRequireFields(t, post, where)

	return post
}

// bookmarkFeedOK требует 200 и разбирает страницу постов, требуя items
// и поля закладок у каждого поста.
func bookmarkFeedOK(t *testing.T, resp *http.Response, where string) bookmarkFeedPayload {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус 200, получен %d %s", where, resp.StatusCode, code)
	}

	var page bookmarkFeedPayload
	decode(t, resp, &page)

	if page.Items == nil {
		t.Fatalf("%s: в ответе нет items (пустой список — [], а не отсутствие поля)", where)
	}
	for _, item := range *page.Items {
		bookmarkRequireFields(t, item, where)
	}

	return page
}

// bookmarkRequire требует у поста ровно это число закладок и такой признак
// «я сохранил».
func bookmarkRequire(t *testing.T, post bookmarkPostPayload, wantCount int, wantMine bool, where string) {
	t.Helper()

	bookmarkRequireFields(t, post, where)

	if *post.Bookmarks != wantCount {
		t.Errorf("%s: ожидалось закладок %d, получено %d", where, wantCount, *post.Bookmarks)
	}
	if *post.Bookmarked != wantMine {
		t.Errorf("%s: ожидалось bookmarked=%t, получено %t", where, wantMine, *post.Bookmarked)
	}
}

// bookmarkRequireLikes требует у поста ровно эти лайки.
func bookmarkRequireLikes(t *testing.T, post bookmarkPostPayload, wantLikes int, wantLiked bool, where string) {
	t.Helper()

	if post.Likes == nil || post.Liked == nil {
		t.Fatalf("%s: у поста %s нет поля likes или liked", where, post.ID)
	}
	if *post.Likes != wantLikes {
		t.Errorf("%s: ожидалось лайков %d, получено %d", where, wantLikes, *post.Likes)
	}
	if *post.Liked != wantLiked {
		t.Errorf("%s: ожидалось liked=%t, получено %t", where, wantLiked, *post.Liked)
	}
}

// --- Хелперы: действия ----------------------------------------------------

// bookmarkPause разводит закладки во времени: порядок «Сохранённых» —
// по времени закладки, и двум закладкам подряд не стоит делить одно
// мгновение.
func bookmarkPause() {
	time.Sleep(5 * time.Millisecond)
}

// bookmarkPublish публикует пост с одной фотографией и подписью.
func bookmarkPublish(t *testing.T, baseURL string, who dachnik, caption string) bookmarkPostPayload {
	t.Helper()

	photo := photoOf(t, baseURL, who.token, 60, 40)
	resp := createPost(t, baseURL, who.token, map[string]any{"media_ids": []string{photo.ID}, "caption": caption})

	return bookmarkPostOK(t, resp, http.StatusCreated, "публикация поста")
}

// bookmarkPublishMany публикует count постов и возвращает их
// идентификаторы в порядке публикации.
func bookmarkPublishMany(t *testing.T, baseURL string, who dachnik, count int) []string {
	t.Helper()

	ids := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		ids = append(ids, bookmarkPublish(t, baseURL, who, fmt.Sprintf("Грядка для закладки № %d", i)).ID)
	}

	return ids
}

// bookmarkAdd добавляет пост в закладки и требует 200 с этим постом.
func bookmarkAdd(t *testing.T, baseURL string, who dachnik, postID string) bookmarkPostPayload {
	t.Helper()

	post := bookmarkPostOK(t, bookmarkPutReq(t, baseURL, who.token, postID), http.StatusOK, "PUT /posts/{id}/bookmark")
	if post.ID != postID {
		t.Fatalf("закладка поста %s вернула пост %s", postID, post.ID)
	}

	return post
}

// bookmarkAddAll добавляет посты в закладки по очереди, с паузой между
// ними: первый в списке окажется в «Сохранённых» последним.
func bookmarkAddAll(t *testing.T, baseURL string, who dachnik, postIDs ...string) {
	t.Helper()

	for _, id := range postIDs {
		bookmarkAdd(t, baseURL, who, id)
		bookmarkPause()
	}
}

// bookmarkRemove убирает пост из закладок и требует 200 с этим постом.
func bookmarkRemove(t *testing.T, baseURL string, who dachnik, postID string) bookmarkPostPayload {
	t.Helper()

	post := bookmarkPostOK(t, bookmarkDeleteReq(t, baseURL, who.token, postID), http.StatusOK, "DELETE /posts/{id}/bookmark")
	if post.ID != postID {
		t.Fatalf("снятие закладки поста %s вернуло пост %s", postID, post.ID)
	}

	return post
}

// bookmarkPostAt открывает пост по адресу и требует 200.
func bookmarkPostAt(t *testing.T, baseURL string, who dachnik, postID string) bookmarkPostPayload {
	t.Helper()
	return bookmarkPostOK(t, fetchPost(t, baseURL, who.token, postID), http.StatusOK, "GET /posts/"+postID)
}

// --- Хелперы: «Сохранённые» -----------------------------------------------

// bookmarkPage — страница «Сохранённых» с этими параметрами, 200.
func bookmarkPage(t *testing.T, baseURL string, who dachnik, params url.Values) bookmarkFeedPayload {
	t.Helper()

	query := params.Encode()

	return bookmarkFeedOK(t, bookmarkListReq(t, baseURL, who.token, query), "GET /me/bookmarks?"+query)
}

// bookmarkIDs — идентификаторы постов страницы по порядку.
func bookmarkIDs(page bookmarkFeedPayload) []string {
	ids := []string{}
	if page.Items == nil {
		return ids
	}
	for _, item := range *page.Items {
		ids = append(ids, item.ID)
	}

	return ids
}

// bookmarkFind — пост страницы с этим идентификатором или nil.
func bookmarkFind(page bookmarkFeedPayload, postID string) *bookmarkPostPayload {
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

// bookmarkCursor требует курсор на продолжение и возвращает его.
func bookmarkCursor(t *testing.T, page bookmarkFeedPayload, where string) string {
	t.Helper()

	if page.NextCursor == nil || *page.NextCursor == "" {
		t.Fatalf("%s: ожидался курсор на продолжение, его нет", where)
	}

	return *page.NextCursor
}

// bookmarkRequireEnd требует, чтобы за страницей ничего не было.
func bookmarkRequireEnd(t *testing.T, page bookmarkFeedPayload, where string) {
	t.Helper()

	if page.NextCursor != nil {
		t.Errorf("%s: ожидалась последняя страница без next_cursor, получен курсор %q", where, *page.NextCursor)
	}
}

// bookmarkSaved — «Сохранённые» одной страницей (limit 50): в тестах их
// меньше, поэтому курсора быть не должно.
func bookmarkSaved(t *testing.T, baseURL string, who dachnik) []string {
	t.Helper()

	page := bookmarkPage(t, baseURL, who, feedParams(50, ""))
	bookmarkRequireEnd(t, page, "«Сохранённые», limit=50")

	return bookmarkIDs(page)
}

// bookmarkWalk проходит «Сохранённые» страницами по limit и возвращает
// идентификаторы подряд. Требует, чтобы страница с курсором была полной:
// скрытые посты не укорачивают страницу (ФТ-8).
func bookmarkWalk(t *testing.T, baseURL string, who dachnik, limit int) []string {
	t.Helper()

	var ids []string
	cursor := ""

	for n := 1; ; n++ {
		if n > 100 {
			t.Fatalf("«Сохранённые» не кончаются: за %d страниц отдано %d постов", n-1, len(ids))
		}

		page := bookmarkPage(t, baseURL, who, feedParams(limit, cursor))
		got := bookmarkIDs(page)
		ids = append(ids, got...)

		if page.NextCursor == nil {
			break
		}
		if len(got) != limit {
			t.Errorf("страница %d с курсором короче limit=%d: %d постов", n, limit, len(got))
		}
		cursor = bookmarkCursor(t, page, fmt.Sprintf("страница %d", n))
	}

	return ids
}

// bookmarkRequireIDs требует ровно эти посты в этом порядке.
func bookmarkRequireIDs(t *testing.T, got, want []string, where string) {
	t.Helper()

	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: ожидались посты %v, получены %v", where, want, got)
	}
}

// bookmarkRows — сколько строк закладок у поста в таблице post_bookmarks
// («Модель данных»): удаление жёсткое, проверяется отсутствием строк.
func bookmarkRows(t *testing.T, column, id string) int {
	t.Helper()
	return countSQL(t, "SELECT count(*) FROM post_bookmarks WHERE "+column+" = $1", id)
}

// ============================================================================
// PUT и DELETE /api/posts/{postId}/bookmark
// ============================================================================

// «Добавить пост без закладок»: 200, bookmarks: 1, bookmarked: true; и на
// самом посте то же (ФТ-1, ФТ-3, ФТ-5).
func TestBookmarkAddToPostWithoutBookmarks(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := bookmarkPublish(t, baseURL, author, "Как подвязываю томаты")

	got := bookmarkAdd(t, baseURL, reader, post.ID)
	bookmarkRequire(t, got, 1, true, "ответ на закладку")
	if got.Caption != post.Caption || got.Author.ID != author.id || len(got.Media) != 1 {
		t.Errorf("ответ на закладку — не пост целиком: %+v", got.postPayload)
	}

	bookmarkRequire(t, bookmarkPostAt(t, baseURL, reader, post.ID), 1, true, "пост по адресу у сохранившего")
}

// «Добавить тот же пост второй раз»: 200, bookmarks не растёт,
// bookmarked: true (ФТ-1, ФТ-3).
func TestBookmarkAddTwiceDoesNotGrow(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := bookmarkPublish(t, baseURL, author, "Схема капельного полива")

	bookmarkRequire(t, bookmarkAdd(t, baseURL, reader, post.ID), 1, true, "первая закладка")
	bookmarkRequire(t, bookmarkAdd(t, baseURL, reader, post.ID), 1, true, "повторная закладка")
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, reader, post.ID), 1, true, "пост после повтора")
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{post.ID}, "«Сохранённые» после повтора")
}

// «Повторное добавление»: пост в «Сохранённых» на прежнем месте, наверх
// не поднимается — время закладки остаётся первым (ФТ-3).
func TestBookmarkRepeatAddKeepsPlaceInSaved(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	ids := bookmarkPublishMany(t, baseURL, author, 2)
	first, second := ids[0], ids[1]

	bookmarkAddAll(t, baseURL, reader, first, second)
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{second, first}, "до повтора")

	bookmarkAdd(t, baseURL, reader, first)
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{second, first}, "после повтора закладки первого")
}

// «Убрать свою закладку»: 200, bookmarks уменьшается, bookmarked: false,
// из «Сохранённых» пропадает (ФТ-1, ФТ-3, ФТ-7).
func TestBookmarkRemoveOwnBookmark(t *testing.T) {
	baseURL := startAPI(t)
	author, reader, other := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	post := bookmarkPublish(t, baseURL, author, "Отдам рассаду в субботу")
	kept := bookmarkPublish(t, baseURL, author, "Мульчирую опилками")

	bookmarkAddAll(t, baseURL, reader, kept.ID, post.ID)
	bookmarkAdd(t, baseURL, other, post.ID)

	bookmarkRequire(t, bookmarkRemove(t, baseURL, reader, post.ID), 1, false, "ответ на снятие закладки")
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, reader, post.ID), 1, false, "пост у снявшего")
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, other, post.ID), 1, true, "пост у второго сохранившего")

	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{kept.ID}, "«Сохранённые» снявшего")
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, other), []string{post.ID}, "«Сохранённые» второго")
}

// «Убрать закладку, которой не было»: 200, ничего не меняется,
// bookmarked: false; чужие закладки запрос не трогает (ФТ-3).
func TestBookmarkRemoveAbsentBookmarkIsNoop(t *testing.T) {
	baseURL := startAPI(t)
	author, reader, other := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	t.Run("у поста без закладок", func(t *testing.T) {
		post := bookmarkPublish(t, baseURL, author, "Пикирую перцы")

		bookmarkRequire(t, bookmarkRemove(t, baseURL, reader, post.ID), 0, false, "ответ")
		bookmarkRequire(t, bookmarkRemove(t, baseURL, reader, post.ID), 0, false, "повтор")
		bookmarkRequire(t, bookmarkPostAt(t, baseURL, reader, post.ID), 0, false, "пост после")
	})

	t.Run("у поста с чужой закладкой", func(t *testing.T) {
		post := bookmarkPublish(t, baseURL, author, "Чем подкормить огурцы")
		bookmarkAdd(t, baseURL, other, post.ID)

		bookmarkRequire(t, bookmarkRemove(t, baseURL, reader, post.ID), 1, false, "ответ")
		bookmarkRequire(t, bookmarkPostAt(t, baseURL, other, post.ID), 1, true, "чужая закладка на месте")
		if !contains(bookmarkSaved(t, baseURL, other), post.ID) {
			t.Errorf("пост пропал из «Сохранённых» того, кто его сохранил")
		}
	})
}

// «Убрать и снова добавить»: это новая закладка — пост в «Сохранённых»
// первым (ФТ-4).
func TestBookmarkRemoveAndAddAgainGoesFirst(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	ids := bookmarkPublishMany(t, baseURL, author, 3)
	bookmarkAddAll(t, baseURL, reader, ids...)
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{ids[2], ids[1], ids[0]}, "до")

	bookmarkRemove(t, baseURL, reader, ids[0])
	bookmarkPause()
	bookmarkRequire(t, bookmarkAdd(t, baseURL, reader, ids[0]), 1, true, "снова добавлен")

	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{ids[0], ids[2], ids[1]}, "после повторного добавления")
}

// «Сохранить свой пост»: сохраняется (ФТ-2).
func TestBookmarkOwnPost(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)

	post := bookmarkPublish(t, baseURL, author, "Мой дневник теплицы")

	bookmarkRequire(t, bookmarkAdd(t, baseURL, author, post.ID), 1, true, "ответ")
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, author, post.ID), 1, true, "свой пост по адресу")
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, author), []string{post.ID}, "свои «Сохранённые»")
}

// «Два человека сохранили пост»: bookmarks: 2, у каждого bookmarked: true,
// у третьего false; число видно и автору, а кто сохранил — нет (ФТ-5, ФТ-6).
func TestBookmarkTwoPeopleCountedThirdSeesFalse(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	first, second, third := newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3), newDachnik(t, baseURL, 4)

	post := bookmarkPublish(t, baseURL, author, "Схема полива")

	bookmarkRequire(t, bookmarkAdd(t, baseURL, first, post.ID), 1, true, "первая закладка")
	bookmarkRequire(t, bookmarkAdd(t, baseURL, second, post.ID), 2, true, "вторая закладка")

	bookmarkRequire(t, bookmarkPostAt(t, baseURL, first, post.ID), 2, true, "у первого")
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, second, post.ID), 2, true, "у второго")
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, third, post.ID), 2, false, "у третьего")
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, author, post.ID), 2, false, "у автора")
}

// «Закладка и лайк»: независимы — закладка не ставит лайк и не меняет
// likes, лайк не трогает bookmarks (ФТ-5).
func TestBookmarkIndependentFromLikes(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := bookmarkPublish(t, baseURL, author, "Первая клубника")

	got := bookmarkAdd(t, baseURL, reader, post.ID)
	bookmarkRequireLikes(t, got, 0, false, "ответ на закладку")

	liked := bookmarkPostOK(t, likePost(t, baseURL, reader.token, post.ID), http.StatusOK, "лайк")
	bookmarkRequire(t, liked, 1, true, "ответ на лайк")
	bookmarkRequireLikes(t, liked, 1, true, "ответ на лайк")

	removed := bookmarkRemove(t, baseURL, reader, post.ID)
	bookmarkRequireLikes(t, removed, 1, true, "ответ на снятие закладки")
	bookmarkRequire(t, removed, 0, false, "ответ на снятие закладки")

	bookmarkAdd(t, baseURL, reader, post.ID)
	unliked := bookmarkPostOK(t, unlikePost(t, baseURL, reader.token, post.ID), http.StatusOK, "снятие лайка")
	bookmarkRequire(t, unliked, 1, true, "ответ на снятие лайка")
	bookmarkRequireLikes(t, unliked, 0, false, "ответ на снятие лайка")
}

// «Поля в ленте, постах профиля, ленте тэга и группы, на посте, в ответе
// на лайк»: bookmarks и bookmarked есть, bookmarked — для того, кто
// спрашивает (ФТ-5). Заодно «Пост со своей закладкой в ленте».
func TestBookmarkFieldsEverywhere(t *testing.T) {
	baseURL := startAPI(t)
	author, saver, other := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	group := grInterestGroup(t, baseURL, author, "Томатоводы")
	grJoin(t, baseURL, saver, group.ID, grMember)
	grJoin(t, baseURL, other, group.ID, grMember)
	followOK(t, baseURL, saver, author.id)
	followOK(t, baseURL, other, author.id)

	photo := photoOf(t, baseURL, author.token, 60, 40)
	post := bookmarkPostOK(t, createPost(t, baseURL, author.token, map[string]any{
		"media_ids": []string{photo.ID},
		"caption":   "Подвязываю #" + bookmarkTag,
		"group_ids": []string{group.ID},
	}), http.StatusCreated, "публикация поста с тэгом в группе")

	bookmarkAdd(t, baseURL, saver, post.ID)

	viewers := []struct {
		name string
		who  dachnik
		mine bool
	}{
		{"сохранивший", saver, true},
		{"другой читатель", other, false},
		{"автор", author, false},
	}

	for _, v := range viewers {
		t.Run(v.name, func(t *testing.T) {
			bookmarkRequire(t, bookmarkPostAt(t, baseURL, v.who, post.ID), 1, v.mine, "пост по адресу")

			feeds := map[string]*http.Response{
				"лента без вкладки": fetchFeed(t, baseURL, v.who.token, feedParams(50, "")),
				"вкладка all":       fetchFeed(t, baseURL, v.who.token, scopeParams("all", 50, "")),
				"лента тэга":        fetchFeedRaw(t, baseURL, v.who.token, url.Values{"limit": {"50"}, "tag": {bookmarkTag}}.Encode()),
				"лента группы":      gpGroupPostsReq(t, baseURL, v.who.token, group.ID, "limit=50"),
				"посты профиля":     fetchUserPosts(t, baseURL, v.who.token, author.id, feedParams(50, "")),
			}
			if v.who.id != author.id {
				feeds["вкладка following"] = fetchFeed(t, baseURL, v.who.token, scopeParams("following", 50, ""))
			}

			for where, resp := range feeds {
				page := bookmarkFeedOK(t, resp, where)
				item := bookmarkFind(page, post.ID)
				if item == nil {
					t.Errorf("%s: поста %s нет, а он виден", where, post.ID)
					continue
				}
				bookmarkRequire(t, *item, 1, v.mine, where)
			}

			liked := bookmarkPostOK(t, likePost(t, baseURL, v.who.token, post.ID), http.StatusOK, "лайк")
			bookmarkRequire(t, liked, 1, v.mine, "ответ на лайк")
			unliked := bookmarkPostOK(t, unlikePost(t, baseURL, v.who.token, post.ID), http.StatusOK, "снятие лайка")
			bookmarkRequire(t, unliked, 1, v.mine, "ответ на снятие лайка")
		})
	}

	t.Run("в «Сохранённых»", func(t *testing.T) {
		page := bookmarkPage(t, baseURL, saver, feedParams(50, ""))
		item := bookmarkFind(page, post.ID)
		if item == nil {
			t.Fatalf("сохранённого поста нет в «Сохранённых»")
		}
		bookmarkRequire(t, *item, 1, true, "«Сохранённые»")
	})
}

// «Свежий пост в ответе на публикацию»: bookmarks: 0, bookmarked: false —
// поля есть в самом ответе, а не пропущены (ФТ-5, «Пост в ответе»).
func TestBookmarkFreshPostHasZeroFields(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)

	photo := photoOf(t, baseURL, author.token, 60, 40)
	resp := createPost(t, baseURL, author.token, map[string]any{"media_ids": []string{photo.ID}, "caption": "Свежий"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("публикация: ожидался статус 201, получен %d", resp.StatusCode)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON(t, resp), &fields); err != nil {
		t.Fatalf("ответ на публикацию не разобрался как JSON: %v", err)
	}
	if got := string(fields["bookmarks"]); got != "0" {
		t.Errorf("у свежего поста bookmarks = %q, ожидалось 0", got)
	}
	if got := string(fields["bookmarked"]); got != "false" {
		t.Errorf("у свежего поста bookmarked = %q, ожидалось false", got)
	}
}

// «Закладка несуществующему посту», «Идентификатор поста не похож на
// UUID», «Убрать закладку у несуществующего поста»: 404 post_not_found
// (ФТ-9, «Ошибки»).
func TestBookmarkUnknownPostNotFound(t *testing.T) {
	baseURL := startAPI(t)
	reader := newDachnik(t, baseURL, 1)

	ids := map[string]string{
		"несуществующий пост": unknownID,
		"вовсе не UUID":       notAnID,
	}

	for name, id := range ids {
		t.Run(name, func(t *testing.T) {
			requireNotFound(t, bookmarkPutReq(t, baseURL, reader.token, id), "добавление закладки", name)
			requireNotFound(t, bookmarkDeleteReq(t, baseURL, reader.token, id), "снятие закладки", name)
		})
	}

	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), nil, "«Сохранённые» после отказов")
}

// «Закладка посту „Только я“ чужого автора»: 404 post_not_found — для
// смотрящего такого поста нет; снять тоже нельзя. Автор свой пост
// «Только я» сохранить может: он ему виден (ФТ-2, ФТ-9).
func TestBookmarkOnlyMePostOfOtherNotFound(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := publishWithVisibility(t, baseURL, author.token, "Только для себя", visibilityMe)

	requireNotFound(t, bookmarkPutReq(t, baseURL, reader.token, post.ID), "добавление закладки", "чужой пост «Только я»")
	requireNotFound(t, bookmarkDeleteReq(t, baseURL, reader.token, post.ID), "снятие закладки", "чужой пост «Только я»")
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), nil, "«Сохранённые» читателя")

	bookmarkRequire(t, bookmarkAdd(t, baseURL, author, post.ID), 1, true, "автор сохраняет свой пост «Только я»")
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, author), []string{post.ID}, "«Сохранённые» автора")
}

// «Любой из трёх запросов без токена» и «Без токена к несуществующему
// посту»: 401 unauthorized — токен проверяется раньше поста («Ошибки»).
func TestBookmarkRequiresToken(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)

	post := bookmarkPublish(t, baseURL, author, "Пост для проверки входа")
	bookmarkAdd(t, baseURL, author, post.ID)

	tokens := map[string]string{
		"без токена":             "",
		"недействительный токен": "не-токен-вовсе",
	}

	for name, token := range tokens {
		t.Run(name, func(t *testing.T) {
			requireUnauthorized(t, do(t, http.MethodPut, bookmarkURL(baseURL, post.ID), token, nil), "PUT закладки")
			requireUnauthorized(t, do(t, http.MethodDelete, bookmarkURL(baseURL, post.ID), token, nil), "DELETE закладки")
			requireUnauthorized(t, bookmarkListReq(t, baseURL, token, ""), "GET /me/bookmarks")

			requireUnauthorized(t, do(t, http.MethodPut, bookmarkURL(baseURL, unknownID), token, nil), "PUT к несуществующему посту")
			requireUnauthorized(t, do(t, http.MethodDelete, bookmarkURL(baseURL, unknownID), token, nil), "DELETE к несуществующему посту")
		})
	}

	bookmarkRequire(t, bookmarkPostAt(t, baseURL, author, post.ID), 1, true, "закладка после отказов")
}

// «Тело в запросе закладки»: пропускается — и JSON, и не JSON («API»).
func TestBookmarkIgnoresRequestBody(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := bookmarkPublish(t, baseURL, author, "Пост с телом запроса")
	address := bookmarkURL(baseURL, post.ID)

	bookmarkRequire(t, bookmarkPostOK(t, do(t, http.MethodPut, address, reader.token, map[string]any{"bookmarked": false}),
		http.StatusOK, "PUT с JSON-телом"), 1, true, "PUT с JSON-телом")
	bookmarkRequire(t, bookmarkPostOK(t, do(t, http.MethodDelete, address, reader.token, map[string]any{"bookmarked": true}),
		http.StatusOK, "DELETE с JSON-телом"), 0, false, "DELETE с JSON-телом")

	bookmarkRequire(t, bookmarkPostOK(t, ebdRaw(t, http.MethodPut, address, reader.token, "не json {"),
		http.StatusOK, "PUT с телом не-JSON"), 1, true, "PUT с телом не-JSON")
	bookmarkRequire(t, bookmarkPostOK(t, ebdRaw(t, http.MethodDelete, address, reader.token, "не json {"),
		http.StatusOK, "DELETE с телом не-JSON"), 0, false, "DELETE с телом не-JSON")
}

// ============================================================================
// GET /api/me/bookmarks — «Сохранённые»
// ============================================================================

// «„Сохранённые“ без закладок»: 200, items: [], next_cursor нет (ФТ-7).
func TestBookmarkSavedEmpty(t *testing.T) {
	baseURL := startAPI(t)
	reader := newDachnik(t, baseURL, 1)

	resp := bookmarkListReq(t, baseURL, reader.token, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("пустые «Сохранённые»: ожидался статус 200, получен %d", resp.StatusCode)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON(t, resp), &fields); err != nil {
		t.Fatalf("ответ не разобрался как JSON: %v", err)
	}
	if got := string(fields["items"]); got != "[]" {
		t.Errorf("пустые «Сохранённые»: items = %q, ожидалось []", got)
	}
	if raw, ok := fields["next_cursor"]; ok && string(raw) != "null" {
		t.Errorf("пустые «Сохранённые»: next_cursor = %s, а его быть не должно", raw)
	}
}

// «Порядок „Сохранённых“»: по времени закладки, новые сверху, а не по
// времени поста (ФТ-7).
func TestBookmarkSavedOrderByBookmarkTime(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	ids := bookmarkPublishMany(t, baseURL, author, 3)
	oldest, middle, newest := ids[0], ids[1], ids[2]

	// Сохраняет не в порядке публикации: средний, самый старый, самый свежий.
	bookmarkAddAll(t, baseURL, reader, middle, oldest, newest)

	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{newest, oldest, middle}, "«Сохранённые»")
}

// «Чужие закладки»: в моих «Сохранённых» их нет (ФТ-7).
func TestBookmarkSavedOnlyMine(t *testing.T) {
	baseURL := startAPI(t)
	author, reader, other := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	ids := bookmarkPublishMany(t, baseURL, author, 3)

	bookmarkAdd(t, baseURL, reader, ids[0])
	bookmarkAddAll(t, baseURL, other, ids[1], ids[2])

	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{ids[0]}, "мои «Сохранённые»")
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, other), []string{ids[2], ids[1]}, "чужие «Сохранённые»")
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, author), nil, "«Сохранённые» автора")
}

// «Тот же человек с другого устройства»: видит свои закладки — закладка
// принадлежит человеку, а не сессии (ФТ-1).
func TestBookmarkSavedSameUserOtherDevice(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 50 * time.Millisecond})
	author := newDachnik(t, baseURL, 1)
	phone := dachnikPhone(2)

	firstToken, userID := signIn(t, baseURL, phone)
	first := dachnik{token: firstToken, id: userID}

	post := bookmarkPublish(t, baseURL, author, "Сохраню с телефона")
	bookmarkAdd(t, baseURL, first, post.ID)

	time.Sleep(60 * time.Millisecond)
	secondToken, sameID := signIn(t, baseURL, phone)
	if sameID != userID || secondToken == firstToken {
		t.Fatalf("второй вход: ожидался тот же пользователь %s с новым токеном, получен %s", userID, sameID)
	}
	second := dachnik{token: secondToken, id: sameID}

	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, second), []string{post.ID}, "«Сохранённые» со второго устройства")
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, second, post.ID), 1, true, "пост со второго устройства")
}

// «3 закладки, limit=2»: первая страница — 2 поста и next_cursor; по нему
// — третий, и next_cursor нет (ФТ-7, ADR-0015).
func TestBookmarkSavedPagination(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	ids := bookmarkPublishMany(t, baseURL, author, 3)
	bookmarkAddAll(t, baseURL, reader, ids...)

	first := bookmarkPage(t, baseURL, reader, feedParams(2, ""))
	bookmarkRequireIDs(t, bookmarkIDs(first), []string{ids[2], ids[1]}, "первая страница")
	cursor := bookmarkCursor(t, first, "первая страница")

	second := bookmarkPage(t, baseURL, reader, feedParams(2, cursor))
	bookmarkRequireIDs(t, bookmarkIDs(second), []string{ids[0]}, "вторая страница")
	bookmarkRequireEnd(t, second, "вторая страница")
}

// limit по умолчанию — 20; ровно limit закладок — без курсора; пустой
// cursor= — первая страница (ФТ-7).
func TestBookmarkSavedDefaultLimitAndEdges(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	ids := bookmarkPublishMany(t, baseURL, author, feedDefaultLimit+1)
	bookmarkAddAll(t, baseURL, reader, ids...)
	newest := newestFirst(ids)

	t.Run("без limit — 20 и курсор", func(t *testing.T) {
		page := bookmarkPage(t, baseURL, reader, url.Values{})
		bookmarkRequireIDs(t, bookmarkIDs(page), newest[:feedDefaultLimit], "первая страница по умолчанию")
		cursor := bookmarkCursor(t, page, "первая страница по умолчанию")

		rest := bookmarkPage(t, baseURL, reader, feedParams(0, cursor))
		bookmarkRequireIDs(t, bookmarkIDs(rest), newest[feedDefaultLimit:], "вторая страница по умолчанию")
		bookmarkRequireEnd(t, rest, "вторая страница по умолчанию")
	})

	t.Run("пустой cursor — первая страница", func(t *testing.T) {
		page := bookmarkFeedOK(t, bookmarkListReq(t, baseURL, reader.token, "limit=2&cursor="), "cursor=")
		bookmarkRequireIDs(t, bookmarkIDs(page), newest[:2], "cursor=")
	})

	t.Run("ровно limit — без курсора", func(t *testing.T) {
		page := bookmarkPage(t, baseURL, reader, feedParams(feedDefaultLimit+1, ""))
		bookmarkRequireIDs(t, bookmarkIDs(page), newest, "limit = числу закладок")
		bookmarkRequireEnd(t, page, "limit = числу закладок")
	})

	t.Run("проход по одной", func(t *testing.T) {
		bookmarkRequireIDs(t, bookmarkWalk(t, baseURL, reader, 1), newest, "проход limit=1")
	})
}

// «limit=0, limit=51, limit=abc»: 400 invalid_request, как у ленты
// (ФТ-7, «Ошибки»).
func TestBookmarkSavedRejectsBadLimit(t *testing.T) {
	baseURL := startAPI(t)
	reader := newDachnik(t, baseURL, 1)

	for _, query := range []string{"limit=0", "limit=51", "limit=-1", "limit=abc"} {
		t.Run(query, func(t *testing.T) {
			requireError(t, bookmarkListReq(t, baseURL, reader.token, query), http.StatusBadRequest, "invalid_request")
		})
	}

	t.Run("limit=50 принимается", func(t *testing.T) {
		bookmarkFeedOK(t, bookmarkListReq(t, baseURL, reader.token, "limit=50"), "limit=50")
	})
}

// «Курсор-мусор»: 400 invalid_cursor («Ошибки»).
func TestBookmarkSavedRejectsBrokenCursor(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	ids := bookmarkPublishMany(t, baseURL, author, 3)
	bookmarkAddAll(t, baseURL, reader, ids...)

	issued := bookmarkCursor(t, bookmarkPage(t, baseURL, reader, feedParams(1, "")), "первая страница")
	if len(issued) < 4 {
		t.Fatalf("курсор %q подозрительно короткий: тест собран неправильно", issued)
	}

	cases := map[string]string{
		"не курсор вовсе":                    "курсор",
		"мусор в base64":                     "0L3QtSDQutGD0YDRgdC-0YA=",
		"обрезанный настоящий курсор":        issued[:len(issued)-3],
		"идентификатор поста вместо курсора": ids[0],
	}

	for name, cursor := range cases {
		t.Run(name, func(t *testing.T) {
			resp := bookmarkListReq(t, baseURL, reader.token, "cursor="+url.QueryEscape(cursor))
			requireError(t, resp, http.StatusBadRequest, "invalid_cursor")
		})
	}
}

// ============================================================================
// Видимость, удаление, блокировка
// ============================================================================

// «Пост из закладок удалили»: пропадает из «Сохранённых», число у других
// постов не меняется; закладки уходят вместе с постом (ФТ-10).
func TestBookmarkDeletedPostLeavesSaved(t *testing.T) {
	baseURL := startAPI(t)
	author, reader, other := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	ids := bookmarkPublishMany(t, baseURL, author, 2)
	doomed, kept := ids[0], ids[1]

	bookmarkAddAll(t, baseURL, reader, doomed, kept)
	bookmarkAdd(t, baseURL, other, doomed)
	bookmarkAdd(t, baseURL, other, kept)

	requireDeleted(t, deletePost(t, baseURL, author.token, doomed))

	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{kept}, "«Сохранённые» после удаления поста")
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, other), []string{kept}, "«Сохранённые» второго после удаления поста")
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, reader, kept), 2, true, "оставшийся пост")

	requireNotFound(t, bookmarkPutReq(t, baseURL, reader.token, doomed), "добавление закладки", "удалённый пост")
	requireNotFound(t, bookmarkDeleteReq(t, baseURL, reader.token, doomed), "снятие закладки", "удалённый пост")

	if n := bookmarkRows(t, "post_id", doomed); n != 0 {
		t.Errorf("у удалённого поста осталось закладок в базе: %d", n)
	}
}

// «Автор сузил пост до „Только я“»: пропадает из «Сохранённых» у других;
// вернул «Все» — пост снова на прежнем месте (ФТ-8).
func TestBookmarkOnlyMeHidesAndRestores(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	ids := bookmarkPublishMany(t, baseURL, author, 3)
	bookmarkAddAll(t, baseURL, reader, ids...)
	bookmarkAdd(t, baseURL, author, ids[1])

	changeVisibility(t, baseURL, author.token, ids[1], visibilityMe)
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{ids[2], ids[0]}, "после сужения до «Только я»")
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, author), []string{ids[1]}, "у самого автора пост на месте")

	changeVisibility(t, baseURL, author.token, ids[1], visibilityAll)
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{ids[2], ids[1], ids[0]}, "после возврата «Все»")
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, reader, ids[1]), 2, true, "вернувшийся пост")
}

// «Автор закрыл профиль, я не подписан»: его посты пропадают из моих
// «Сохранённых»; открыл — возвращаются (ФТ-8).
func TestBookmarkClosedProfileHidesPosts(t *testing.T) {
	baseURL := startAPI(t)
	author, reader, neighbour := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	authorPost := bookmarkPublish(t, baseURL, author, "Пост автора")
	neighbourPost := bookmarkPublish(t, baseURL, neighbour, "Пост соседа")
	bookmarkAddAll(t, baseURL, reader, authorPost.ID, neighbourPost.ID)

	setClosed(t, baseURL, author.token, true)
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{neighbourPost.ID}, "после закрытия профиля")

	setClosed(t, baseURL, author.token, false)
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{neighbourPost.ID, authorPost.ID}, "после открытия профиля")
}

// «Блокировка в любую сторону»: посты пропадают из «Сохранённых» обоих;
// закладка не стирается — после разблокировки пост на своём месте
// (ФТ-8, specs/022-edit-block-delete.md).
func TestBookmarkBlockHidesBothWays(t *testing.T) {
	directions := []struct {
		name    string
		blocker int // 0 — первый блокирует второго, 1 — наоборот
	}{
		{"первый блокирует второго", 0},
		{"второй блокирует первого", 1},
	}

	for _, d := range directions {
		t.Run(d.name, func(t *testing.T) {
			baseURL := startAPI(t)
			people := newDachniks(t, baseURL, 1, 3)
			first, second, neighbour := people[0], people[1], people[2]

			firstPost := bookmarkPublish(t, baseURL, first, "Пост первого")
			secondPost := bookmarkPublish(t, baseURL, second, "Пост второго")
			neighbourPost := bookmarkPublish(t, baseURL, neighbour, "Пост соседа")

			bookmarkAddAll(t, baseURL, first, secondPost.ID, neighbourPost.ID, firstPost.ID)
			bookmarkAddAll(t, baseURL, second, firstPost.ID, neighbourPost.ID, secondPost.ID)

			blocker, blocked := first, second
			if d.blocker == 1 {
				blocker, blocked = second, first
			}
			blockOK(t, baseURL, blocker, blocked.id)

			bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, first), []string{firstPost.ID, neighbourPost.ID}, "«Сохранённые» первого")
			bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, second), []string{secondPost.ID, neighbourPost.ID}, "«Сохранённые» второго")

			requireNotFound(t, bookmarkPutReq(t, baseURL, first.token, secondPost.ID), "добавление закладки", "пост второго у первого")
			requireNotFound(t, bookmarkDeleteReq(t, baseURL, second.token, firstPost.ID), "снятие закладки", "пост первого у второго")

			unblockOK(t, baseURL, blocker, blocked.id)

			bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, first),
				[]string{firstPost.ID, neighbourPost.ID, secondPost.ID}, "«Сохранённые» первого после разблокировки")
			bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, second),
				[]string{secondPost.ID, neighbourPost.ID, firstPost.ID}, "«Сохранённые» второго после разблокировки")
		})
	}
}

// «Скрытый пост в середине списка»: страница не короче limit, курсор
// не сбивается (ФТ-8).
func TestBookmarkHiddenPostInMiddleKeepsPageFull(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	ids := bookmarkPublishMany(t, baseURL, author, 6)
	bookmarkAddAll(t, baseURL, reader, ids...)
	// В «Сохранённых»: ids[5], ids[4], ids[3], ids[2], ids[1], ids[0].

	t.Run("скрытый на первой странице", func(t *testing.T) {
		changeVisibility(t, baseURL, author.token, ids[4], visibilityMe)
		t.Cleanup(func() { changeVisibility(t, baseURL, author.token, ids[4], visibilityAll) })

		page := bookmarkPage(t, baseURL, reader, feedParams(2, ""))
		bookmarkRequireIDs(t, bookmarkIDs(page), []string{ids[5], ids[3]}, "первая страница")
		bookmarkCursor(t, page, "первая страница")

		bookmarkRequireIDs(t, bookmarkWalk(t, baseURL, reader, 2),
			[]string{ids[5], ids[3], ids[2], ids[1], ids[0]}, "проход limit=2")
	})

	t.Run("скрыли следующий за курсором", func(t *testing.T) {
		page := bookmarkPage(t, baseURL, reader, feedParams(2, ""))
		bookmarkRequireIDs(t, bookmarkIDs(page), []string{ids[5], ids[4]}, "первая страница")
		cursor := bookmarkCursor(t, page, "первая страница")

		changeVisibility(t, baseURL, author.token, ids[3], visibilityMe)
		t.Cleanup(func() { changeVisibility(t, baseURL, author.token, ids[3], visibilityAll) })

		next := bookmarkPage(t, baseURL, reader, feedParams(2, cursor))
		bookmarkRequireIDs(t, bookmarkIDs(next), []string{ids[2], ids[1]}, "вторая страница по старому курсору")
		last := bookmarkPage(t, baseURL, reader, feedParams(2, bookmarkCursor(t, next, "вторая страница")))
		bookmarkRequireIDs(t, bookmarkIDs(last), []string{ids[0]}, "третья страница")
		bookmarkRequireEnd(t, last, "третья страница")
	})
}

// «Пост стал невидим, убрать закладку»: 404 post_not_found; откроется
// снова — закладка на месте (ФТ-8, ФТ-9).
func TestBookmarkHiddenPostCannotBeUnbookmarked(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	ids := bookmarkPublishMany(t, baseURL, author, 3)
	bookmarkAddAll(t, baseURL, reader, ids...)

	changeVisibility(t, baseURL, author.token, ids[1], visibilityMe)

	requireNotFound(t, bookmarkDeleteReq(t, baseURL, reader.token, ids[1]), "снятие закладки", "пост «Только я»")
	requireNotFound(t, bookmarkPutReq(t, baseURL, reader.token, ids[1]), "добавление закладки", "пост «Только я»")
	if n := bookmarkRows(t, "post_id", ids[1]); n != 1 {
		t.Errorf("закладка невидимого поста должна остаться в базе, строк: %d", n)
	}

	changeVisibility(t, baseURL, author.token, ids[1], visibilityAll)

	bookmarkRequire(t, bookmarkPostAt(t, baseURL, reader, ids[1]), 1, true, "пост снова виден")
	bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, reader), []string{ids[2], ids[1], ids[0]}, "«Сохранённые» после возврата")
}

// «Удалили аккаунт сохранившего»: его закладки исчезают, bookmarks у поста
// уменьшается (ФТ-10). Удалил аккаунт автор — закладки на его посты
// уходят вместе с постами.
func TestBookmarkDeletedAccountBookmarksGone(t *testing.T) {
	baseURL := startAPI(t)
	author, leaver, stayer := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)
	other := newDachnik(t, baseURL, 4)

	post := bookmarkPublish(t, baseURL, author, "Пост, который сохранили двое")
	otherPost := bookmarkPublish(t, baseURL, other, "Пост автора, который уйдёт")

	bookmarkAdd(t, baseURL, leaver, post.ID)
	bookmarkAdd(t, baseURL, stayer, post.ID)
	bookmarkAdd(t, baseURL, stayer, otherPost.ID)
	bookmarkRequire(t, bookmarkPostAt(t, baseURL, author, post.ID), 2, false, "до удаления аккаунта")

	t.Run("удалил аккаунт сохранивший", func(t *testing.T) {
		deleteMeOK(t, baseURL, leaver.token)

		bookmarkRequire(t, bookmarkPostAt(t, baseURL, author, post.ID), 1, false, "у автора")
		bookmarkRequire(t, bookmarkPostAt(t, baseURL, stayer, post.ID), 1, true, "у оставшегося")
		if n := bookmarkRows(t, "user_id", leaver.id); n != 0 {
			t.Errorf("у удалённого аккаунта осталось закладок в базе: %d", n)
		}
	})

	t.Run("удалил аккаунт автор сохранённого поста", func(t *testing.T) {
		deleteMeOK(t, baseURL, other.token)

		bookmarkRequireIDs(t, bookmarkSaved(t, baseURL, stayer), []string{post.ID}, "«Сохранённые» оставшегося")
	})
}

// «Пост со своей закладкой в ленте»: bookmarked: true и в ленте (ФТ-5).
func TestBookmarkOwnBookmarkShownInFeed(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	saved := bookmarkPublish(t, baseURL, author, "Сохранённый пост")
	plain := bookmarkPublish(t, baseURL, author, "Несохранённый пост")
	bookmarkAdd(t, baseURL, reader, saved.ID)

	page := bookmarkFeedOK(t, fetchFeed(t, baseURL, reader.token, feedParams(50, "")), "лента")

	for _, c := range []struct {
		id    string
		count int
		mine  bool
		what  string
	}{
		{saved.ID, 1, true, "сохранённый пост в ленте"},
		{plain.ID, 0, false, "несохранённый пост в ленте"},
	} {
		item := bookmarkFind(page, c.id)
		if item == nil {
			t.Errorf("%s: поста нет в ленте", c.what)
			continue
		}
		bookmarkRequire(t, *item, c.count, c.mine, c.what)
	}
}
