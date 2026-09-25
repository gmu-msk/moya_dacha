package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Подпись, с которой публикуются посты ленты: у каждого своя, чтобы
// в проверках порядка было видно, какой пост куда встал.
const feedCaption = "Грядка № %d"

// Размер страницы по умолчанию и потолок из спеки (specs/004-feed.md,
// ФТ-4, «API / контракт данных»).
const (
	feedDefaultLimit = 20
	feedMaxLimit     = 50
)

// --- Представления из контракта -------------------------------------------

// feedPayload — страница ленты (schema Feed). Курсор — указатель, а не
// строка: его отсутствие и есть признак конца ленты (ФТ-6).
type feedPayload struct {
	Items      []postPayload `json:"items"`
	NextCursor *string       `json:"next_cursor"`
}

// feedRawPayload — та же страница, но посты не разобраны: тесту про вид
// поста важно всё тело целиком, а не поля, которые он знает.
type feedRawPayload struct {
	Items      []json.RawMessage `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

// --- Хелперы --------------------------------------------------------------

// feedParams собирает параметры страницы: limit == 0 и пустой cursor
// означают «параметра нет» — так приложение просит первую страницу.
func feedParams(limit int, cursor string) url.Values {
	params := url.Values{}
	if limit != 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	return params
}

// fetchFeedRaw запрашивает ленту с готовой строкой запроса: она нужна там,
// где параметр нарочно неправильный (`limit=abc`) или пустой (`cursor=`).
func fetchFeedRaw(t *testing.T, baseURL, token, query string) *http.Response {
	t.Helper()

	address := baseURL + "/feed"
	if query != "" {
		address += "?" + query
	}

	return do(t, http.MethodGet, address, token, nil)
}

// fetchFeed запрашивает страницу ленты с этими параметрами.
func fetchFeed(t *testing.T, baseURL, token string, params url.Values) *http.Response {
	t.Helper()

	query := ""
	if len(params) > 0 {
		query = params.Encode()
	}

	return fetchFeedRaw(t, baseURL, token, query)
}

// feedOK требует, чтобы лента отдалась, и возвращает разобранную страницу.
func feedOK(t *testing.T, resp *http.Response) feedPayload {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на страницу ленты ожидался статус 200, получен %d", resp.StatusCode)
	}

	var page feedPayload
	decode(t, resp, &page)

	return page
}

// feedPage — самый частый случай: страница ленты, которая должна отдаться.
func feedPage(t *testing.T, baseURL, token string, params url.Values) feedPayload {
	t.Helper()
	return feedOK(t, fetchFeed(t, baseURL, token, params))
}

// postIDs — идентификаторы постов страницы, в том порядке, в котором они
// пришли.
func postIDs(posts []postPayload) []string {
	ids := make([]string, 0, len(posts))
	for _, post := range posts {
		ids = append(ids, post.ID)
	}

	return ids
}

// newestFirst переворачивает список опубликованных постов: публикуются они
// от старого к новому, а в ленте стоят наоборот (ФТ-1).
func newestFirst(ids []string) []string {
	reversed := make([]string, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		reversed = append(reversed, ids[i])
	}

	return reversed
}

// requireFeedPosts требует, чтобы на странице стояли ровно эти посты и
// ровно в этом порядке.
func requireFeedPosts(t *testing.T, page feedPayload, want []string, where string) {
	t.Helper()

	got := postIDs(page.Items)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: ожидались посты %v, получены %v", where, want, got)
	}
}

// cursorOf требует курсор на продолжение и возвращает его.
func cursorOf(t *testing.T, page feedPayload, where string) string {
	t.Helper()

	if page.NextCursor == nil {
		t.Fatalf("%s: ожидался курсор на продолжение, его нет", where)
	}
	if *page.NextCursor == "" {
		t.Fatalf("%s: курсор на продолжение пустой", where)
	}

	return *page.NextCursor
}

// requireFeedEnd требует, чтобы за этой страницей ничего не было: конец
// ленты — это отсутствие курсора, и ничего больше (ФТ-6).
func requireFeedEnd(t *testing.T, page feedPayload, where string) {
	t.Helper()

	if page.NextCursor != nil {
		t.Errorf("%s: ожидался конец ленты без курсора, получен курсор %q", where, *page.NextCursor)
	}
}

// publishPost публикует пост с одной фотографией и подписью.
func publishPost(t *testing.T, baseURL, token, caption string) postPayload {
	t.Helper()

	photo := photoOf(t, baseURL, token, 60, 40)

	return createdPost(t, createPost(t, baseURL, token,
		map[string]any{"media_ids": []string{photo.ID}, "caption": caption}))
}

// publishPosts публикует count постов подряд и возвращает их
// идентификаторы в порядке публикации: от самого старого к самому свежему.
func publishPosts(t *testing.T, baseURL, token string, count int) []string {
	t.Helper()

	ids := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		ids = append(ids, publishPost(t, baseURL, token, fmt.Sprintf(feedCaption, i)).ID)
	}

	return ids
}

// walkFeed проходит ленту страницами по limit постов — так приложение
// подгружает посты при прокрутке — и возвращает идентификаторы подряд.
// Попутно требует, чтобы пустая страница не приходила с курсором и чтобы
// проход был конечен.
func walkFeed(t *testing.T, baseURL, token string, limit int) []string {
	t.Helper()

	var ids []string
	cursor := ""

	for page := 1; ; page++ {
		if page > 200 {
			t.Fatalf("лента не кончается: за %d страниц отдано %d постов", page-1, len(ids))
		}

		got := feedPage(t, baseURL, token, feedParams(limit, cursor))
		if len(got.Items) == 0 && page > 1 {
			t.Fatalf("страница %d пуста, хотя курсор обещал продолжение", page)
		}
		ids = append(ids, postIDs(got.Items)...)

		if got.NextCursor == nil {
			break
		}
		cursor = cursorOf(t, got, fmt.Sprintf("страница %d", page))
	}

	return ids
}

// --- Подготовка того, чего нет в API --------------------------------------
//
// Две строки «Ограничений и edge cases» через HTTP не воспроизводятся:
// одинаковое время публикации у двух постов (его ставит сервис) и пост,
// которого больше нет (удаления в MVP нет — 007-deletion). Тест готовит их
// прямо в таблице posts, описанной в specs/003-posts.md, «Модель данных».

// execSQL выполняет подготовительный запрос к базе и возвращает число
// затронутых строк.
func execSQL(t *testing.T, sql string, args ...any) int64 {
	t.Helper()

	pool := connect(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tag, err := pool.Exec(ctx, sql, args...)
	if err != nil {
		t.Fatalf("не удалось подготовить данные в базе: %v", err)
	}

	return tag.RowsAffected()
}

// setSameCreatedAt ставит всем перечисленным постам одно и то же время
// публикации: сервис такого не сделает, а лента обязана остаться устойчивой.
func setSameCreatedAt(t *testing.T, ids []string) {
	t.Helper()

	// Микросекунды — предел точности timestamptz: без обрезки время,
	// записанное из Go, у постов совпало бы не полностью.
	at := time.Now().UTC().Truncate(time.Microsecond)

	for _, id := range ids {
		if affected := execSQL(t, `UPDATE posts SET created_at = $1 WHERE id = $2`, at, id); affected != 1 {
			t.Fatalf("время публикации поста %s не проставилось: изменено строк %d", id, affected)
		}
	}
}

// removePostFromDatabase убирает пост так, как его когда-нибудь уберёт
// удаление поста (007-deletion): вместе с медиа, по ON DELETE CASCADE.
func removePostFromDatabase(t *testing.T, id string) {
	t.Helper()

	if affected := execSQL(t, `DELETE FROM posts WHERE id = $1`, id); affected != 1 {
		t.Fatalf("пост %s не удалился: удалено строк %d", id, affected)
	}
}

// --- GET /api/feed: лента ---------------------------------------------------

// Постов нет ни у кого: лента отдаётся пустым списком и без курсора —
// приложению этого хватает, чтобы предложить выложить первый пост
// (ФТ-6, «Ограничения и edge cases»).
func TestFeedIsEmptyWhenNobodyHasPosted(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	resp := fetchFeed(t, baseURL, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на пустую ленту ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body map[string]any
	decode(t, resp, &body)

	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("в ответе ожидался список items, получено %v", body["items"])
	}
	if len(items) != 0 {
		t.Errorf("постов ни у кого нет, а в ленте %d", len(items))
	}
	if cursor, present := body["next_cursor"]; present && cursor != nil {
		t.Errorf("у пустой ленты курсора быть не должно, получен %v", cursor)
	}
}

// Лента — все посты всех пользователей, новые сверху (ФТ-1).
func TestFeedShowsNewestPostsFirst(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	published := publishPosts(t, baseURL, token, 3)

	page := feedPage(t, baseURL, token, nil)
	requireFeedPosts(t, page, newestFirst(published), "лента")
	requireFeedEnd(t, page, "лента из трёх постов")

	// Время публикации в ленте не растёт сверху вниз.
	for i := 1; i < len(page.Items); i++ {
		previous, current := page.Items[i-1].CreatedAt, page.Items[i].CreatedAt
		if previous < current {
			t.Errorf("пост %d опубликован в %s, а стоит выше поста от %s", i+1, current, previous)
		}
	}
}

// Своих постов лента не прячет и чужих не выделяет: оба пользователя
// видят одну и ту же ленту, и опубликованный пост появляется в ней сразу
// (ФТ-1, ФТ-7, ФТ-8, «Свои посты»).
func TestFeedShowsOwnPostsAlongsideOthers(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, token, profileName)
	introduce(t, baseURL, otherToken, "Соседка")

	// Посты чередуются: сосед — я — сосед.
	first := publishPost(t, baseURL, otherToken, "Кабачки у соседа").ID
	second := publishPost(t, baseURL, token, "Моя клубника").ID
	third := publishPost(t, baseURL, otherToken, "Соседские яблоки").ID

	want := []string{third, second, first}

	mine := feedPage(t, baseURL, token, nil)
	requireFeedPosts(t, mine, want, "моя лента")

	neighbours := feedPage(t, baseURL, otherToken, nil)
	requireFeedPosts(t, neighbours, want, "лента соседа")

	// Только что опубликованный пост стоит первым: отложенной публикации
	// в проекте нет.
	justPublished := publishPost(t, baseURL, token, "Ещё одна грядка").ID

	refreshed := feedPage(t, baseURL, token, nil)
	requireFeedPosts(t, refreshed, append([]string{justPublished}, want...), "лента после публикации")
}

// Пост приходит в ленте в том же виде, что и по своему адресу: отдельного
// усечённого представления нет (ФТ-5, ФТ-10, «Пост с десятью
// фотографиями»).
func TestFeedItemIsTheWholePostJustLikeItsOwnPage(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	avatar := avatarOf(t, baseURL, token, imageBytes(t, "png", 300, 300))

	// Десять — наибольшее число фотографий в посте (003-posts, ФТ-1).
	photos := []mediaPayload{
		photoOf(t, baseURL, token, 400, 300),
		photoOf(t, baseURL, token, 300, 400),
		photoOf(t, baseURL, token, 200, 200),
		photoOf(t, baseURL, token, 640, 480),
		photoOf(t, baseURL, token, 480, 640),
		photoOf(t, baseURL, token, 320, 240),
		photoOf(t, baseURL, token, 240, 320),
		photoOf(t, baseURL, token, 500, 500),
		photoOf(t, baseURL, token, 600, 400),
		photoOf(t, baseURL, token, 400, 600),
	}
	mediaIDs := make([]string, 0, len(photos))
	for _, photo := range photos {
		mediaIDs = append(mediaIDs, photo.ID)
	}

	created := createdPost(t, createPost(t, baseURL, token,
		map[string]any{"media_ids": mediaIDs, "caption": postCaption}))

	resp := fetchFeed(t, baseURL, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на страницу ленты ожидался статус 200, получен %d", resp.StatusCode)
	}

	var raw feedRawPayload
	decode(t, resp, &raw)

	if len(raw.Items) != 1 {
		t.Fatalf("в ленте ожидался один пост, получено %d", len(raw.Items))
	}

	var fromFeed map[string]any
	if err := json.Unmarshal(raw.Items[0], &fromFeed); err != nil {
		t.Fatalf("пост из ленты не разобрался как JSON: %v", err)
	}

	fetched := fetchPost(t, baseURL, token, created.ID)
	if fetched.StatusCode != http.StatusOK {
		t.Fatalf("на чтение поста ожидался статус 200, получен %d", fetched.StatusCode)
	}

	var fromPage map[string]any
	if err := json.Unmarshal(rawJSON(t, fetched), &fromPage); err != nil {
		t.Fatalf("пост по своему адресу не разобрался как JSON: %v", err)
	}

	if !reflect.DeepEqual(fromFeed, fromPage) {
		t.Errorf("пост в ленте отличается от поста по своему адресу:\nв ленте: %v\nпо адресу: %v", fromFeed, fromPage)
	}

	// Все десять фотографий, по порядку и с размерами: лента занимает
	// место под картинку до того, как та загрузится (ФТ-10).
	var item postPayload
	if err := json.Unmarshal(raw.Items[0], &item); err != nil {
		t.Fatalf("пост из ленты не разобрался как пост: %v", err)
	}

	requireMediaOrder(t, item, photos)
	if item.Caption != postCaption {
		t.Errorf("ожидалась подпись %q, получена %q", postCaption, item.Caption)
	}
	if item.Author.AvatarURL == nil || *item.Author.AvatarURL != avatar {
		t.Errorf("ожидался аватар автора %q, получен %v", avatar, item.Author.AvatarURL)
	}
	if item.Author.Name != profileName {
		t.Errorf("ожидалось имя автора %q, получено %q", profileName, item.Author.Name)
	}
}

// Пост без подписи приходит с пустой подписью, а у автора без аватара
// ссылки на аватар нет («Ограничения и edge cases»).
func TestFeedShowsPostWithoutCaptionAndAuthorWithoutAvatar(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	photo := photoOf(t, baseURL, token, 320, 240)
	createdPost(t, createPostOf(t, baseURL, token, photo.ID))

	page := feedPage(t, baseURL, token, nil)
	if len(page.Items) != 1 {
		t.Fatalf("в ленте ожидался один пост, получено %d", len(page.Items))
	}

	item := page.Items[0]
	if item.Caption != "" {
		t.Errorf("подпись не задавалась, а в ленте она %q", item.Caption)
	}
	if item.Author.ID != userID {
		t.Errorf("ожидался автор %q, получен %q", userID, item.Author.ID)
	}
	requireNoAvatar(t, item.Author.AvatarURL)
	requireMediaOrder(t, item, []mediaPayload{photo})
}

// Автор в ленте — публичное представление: номера телефона в ответе нет
// и быть не может (ФТ-5, specs/003-posts.md, ФТ-13).
func TestFeedNeverShowsThePhoneNumberOfTheAuthor(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	publishPost(t, baseURL, token, postCaption)

	body := string(rawJSON(t, fetchFeed(t, baseURL, token, nil)))

	for _, forbidden := range []string{phoneStored, phonePretty, phoneSpaced, phoneDigits, `"phone"`} {
		if strings.Contains(body, forbidden) {
			t.Errorf("в ленте не должно быть номера телефона, а ответ содержит %q", forbidden)
		}
	}
}

// --- Страницы и курсор ------------------------------------------------------

// Постов меньше, чем limit: все приходят одной страницей и без курсора
// («Ограничения и edge cases»).
func TestFeedReturnsAllPostsWhenThereAreFewerThanLimit(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := publishPosts(t, baseURL, token, 3)

	page := feedPage(t, baseURL, token, feedParams(10, ""))
	requireFeedPosts(t, page, newestFirst(published), "страница на десять постов")
	requireFeedEnd(t, page, "три поста при limit=10")
}

// Постов ровно limit: страница отдаётся, но курсора нет — дальше пусто
// («Ограничения и edge cases»).
func TestFeedWithExactlyLimitPostsHasNoCursorWhenNothingFollows(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := publishPosts(t, baseURL, token, 3)

	page := feedPage(t, baseURL, token, feedParams(3, ""))
	requireFeedPosts(t, page, newestFirst(published), "страница ровно на три поста")
	requireFeedEnd(t, page, "три поста при limit=3")
}

// Постов больше, чем limit: страница и курсор на продолжение, а курсор
// ведёт строго к тем постам, что старше последнего отданного (ФТ-2, ФТ-3,
// «Курсор из предыдущего ответа»).
func TestFeedWithMorePostsThanLimitReturnsCursorToContinue(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := newestFirst(publishPosts(t, baseURL, token, 5))

	first := feedPage(t, baseURL, token, feedParams(2, ""))
	requireFeedPosts(t, first, published[:2], "первая страница")
	cursor := cursorOf(t, first, "первая страница")

	second := feedPage(t, baseURL, token, feedParams(2, cursor))
	requireFeedPosts(t, second, published[2:4], "вторая страница")
	cursor = cursorOf(t, second, "вторая страница")

	third := feedPage(t, baseURL, token, feedParams(2, cursor))
	requireFeedPosts(t, third, published[4:], "третья страница")
	requireFeedEnd(t, third, "третья страница")
}

// Запрос без limit отдаёт 20 самых свежих постов (ФТ-4, «Запрос без
// limit»).
func TestFeedWithoutLimitReturnsTwentyNewestPosts(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := newestFirst(publishPosts(t, baseURL, token, feedDefaultLimit+1))

	page := feedPage(t, baseURL, token, nil)
	if len(page.Items) != feedDefaultLimit {
		t.Fatalf("без limit ожидалось %d постов, получено %d", feedDefaultLimit, len(page.Items))
	}
	requireFeedPosts(t, page, published[:feedDefaultLimit], "страница по умолчанию")

	last := feedPage(t, baseURL, token, feedParams(0, cursorOf(t, page, "страница по умолчанию")))
	requireFeedPosts(t, last, published[feedDefaultLimit:], "вторая страница по умолчанию")
	requireFeedEnd(t, last, "вторая страница по умолчанию")
}

// limit=1: один пост и курсор; проход по одному доходит до конца ленты,
// и последняя страница — та, у которой курсора нет (ФТ-6, «limit=1»).
func TestFeedWithLimitOneReturnsOnePostAndCursor(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := newestFirst(publishPosts(t, baseURL, token, 3))

	first := feedPage(t, baseURL, token, feedParams(1, ""))
	requireFeedPosts(t, first, published[:1], "страница на один пост")
	cursorOf(t, first, "страница на один пост")

	walked := walkFeed(t, baseURL, token, 1)
	if !reflect.DeepEqual(walked, published) {
		t.Fatalf("проход по одному посту дал %v, ожидалось %v", walked, published)
	}
}

// limit=50 — потолок: страница отдаёт до пятидесяти постов, остальное
// уходит за курсор (ФТ-4, «limit=50»).
func TestFeedWithLimitFiftyReturnsUpToFiftyPosts(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := newestFirst(publishPosts(t, baseURL, token, feedMaxLimit+1))

	page := feedPage(t, baseURL, token, feedParams(feedMaxLimit, ""))
	if len(page.Items) != feedMaxLimit {
		t.Fatalf("при limit=%d ожидалось %d постов, получено %d", feedMaxLimit, feedMaxLimit, len(page.Items))
	}
	requireFeedPosts(t, page, published[:feedMaxLimit], "страница на пятьдесят постов")

	last := feedPage(t, baseURL, token, feedParams(feedMaxLimit, cursorOf(t, page, "страница на пятьдесят постов")))
	requireFeedPosts(t, last, published[feedMaxLimit:], "вторая страница на пятьдесят постов")
	requireFeedEnd(t, last, "вторая страница на пятьдесят постов")
}

// limit вне диапазона 1..50 или не число — 400 invalid_request
// («Ограничения и edge cases», «Ошибки»).
func TestFeedRejectsLimitOutsideOneToFifty(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	publishPosts(t, baseURL, token, 1)

	cases := []struct {
		name  string
		query string
	}{
		{"больше пятидесяти", "limit=51"},
		{"ноль", "limit=0"},
		{"отрицательный", "limit=-1"},
		{"не число", "limit=abc"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			resp := fetchFeedRaw(t, baseURL, token, testCase.query)
			requireError(t, resp, http.StatusBadRequest, "invalid_request")
		})
	}
}

// Курсор, который сервис не выдавал, не разбирается — 400 invalid_cursor
// («Испорченный или чужеродный курсор»).
func TestFeedRejectsBrokenCursor(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := publishPosts(t, baseURL, token, 3)

	issued := cursorOf(t, feedPage(t, baseURL, token, feedParams(1, "")), "первая страница")
	if len(issued) < 4 {
		t.Fatalf("курсор %q подозрительно короткий: тест собран неправильно", issued)
	}

	cases := []struct {
		name   string
		cursor string
	}{
		{"не курсор вовсе", "курсор"},
		{"мусор в base64", "0L3QtSDQutGD0YDRgdC-0YA="},
		{"обрезанный настоящий курсор", issued[:len(issued)-3]},
		{"идентификатор поста вместо курсора", published[0]},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			resp := fetchFeedRaw(t, baseURL, token, "cursor="+url.QueryEscape(testCase.cursor))
			requireError(t, resp, http.StatusBadRequest, "invalid_cursor")
		})
	}
}

// Пустой cursor= — как будто курсора нет: отдаётся первая страница
// («Ограничения и edge cases»).
func TestFeedWithEmptyCursorReturnsFirstPage(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := newestFirst(publishPosts(t, baseURL, token, 3))

	page := feedOK(t, fetchFeedRaw(t, baseURL, token, "cursor="))
	requireFeedPosts(t, page, published, "лента с пустым курсором")
	requireFeedEnd(t, page, "лента с пустым курсором")

	withLimit := feedOK(t, fetchFeedRaw(t, baseURL, token, "limit=2&cursor="))
	requireFeedPosts(t, withLimit, published[:2], "первая страница с пустым курсором")
	cursorOf(t, withLimit, "первая страница с пустым курсором")
}

// Курсор — это место в ленте, а не ссылка на строку: если поста, на
// котором остановились, больше нет, страница всё равно берётся от того
// же места («Курсор поста, который больше не существует»).
func TestFeedCursorWorksWhenItsPostIsGone(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := newestFirst(publishPosts(t, baseURL, token, 4))

	first := feedPage(t, baseURL, token, feedParams(2, ""))
	requireFeedPosts(t, first, published[:2], "первая страница")
	cursor := cursorOf(t, first, "первая страница")

	// Пост, на котором остановилась первая страница, исчезает.
	removePostFromDatabase(t, published[1])

	second := feedPage(t, baseURL, token, feedParams(2, cursor))
	requireFeedPosts(t, second, published[2:], "вторая страница после исчезновения поста")
	requireFeedEnd(t, second, "вторая страница после исчезновения поста")
}

// Курсор последней страницы: постов за ним не осталось — 200, пустой
// список и никакого курсора («Курсор последней страницы»).
func TestFeedCursorPastTheLastPostReturnsEmptyPageWithoutCursor(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := newestFirst(publishPosts(t, baseURL, token, 3))

	first := feedPage(t, baseURL, token, feedParams(2, ""))
	requireFeedPosts(t, first, published[:2], "первая страница")
	cursor := cursorOf(t, first, "первая страница")

	// Единственный оставшийся пост исчезает между запросами страниц:
	// курсор указывает туда, где теперь пусто.
	removePostFromDatabase(t, published[2])

	second := feedPage(t, baseURL, token, feedParams(2, cursor))
	if len(second.Items) != 0 {
		t.Fatalf("за курсором постов не осталось, а отдано %d", len(second.Items))
	}
	requireFeedEnd(t, second, "страница за последним постом")
}

// Пост, выложенный между запросами страниц, не сдвигает и не задваивает
// уже пролистанное; в обновлённой первой странице он при этом первый
// (ФТ-3, ФТ-12, «Новый пост между запросами страниц»).
func TestNewPostBetweenPagesDoesNotShiftTheSecondPage(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)
	published := newestFirst(publishPosts(t, baseURL, token, 4))

	first := feedPage(t, baseURL, token, feedParams(2, ""))
	requireFeedPosts(t, first, published[:2], "первая страница")
	cursor := cursorOf(t, first, "первая страница")

	// Пока человек смотрит первую страницу, сосед выкладывает свой пост.
	fresh := publishPost(t, baseURL, otherToken, "Свежий пост соседа").ID

	second := feedPage(t, baseURL, token, feedParams(2, cursor))
	requireFeedPosts(t, second, published[2:], "вторая страница после чужой публикации")
	requireFeedEnd(t, second, "вторая страница после чужой публикации")

	for _, item := range second.Items {
		if item.ID == fresh {
			t.Errorf("новый пост %s не должен попадать во вторую страницу", fresh)
		}
	}

	// Обновление ленты (первая страница заново) показывает свежий пост
	// сверху.
	refreshed := feedPage(t, baseURL, token, feedParams(2, ""))
	requireFeedPosts(t, refreshed, []string{fresh, published[0]}, "обновлённая первая страница")
}

// Два поста с одинаковым временем публикации: порядок устойчив — по
// идентификатору, и при проходе страницами пост не пропадает и не
// повторяется («Два поста с одинаковым created_at», «Модель данных»).
func TestFeedKeepsStableOrderForPostsWithTheSameCreatedAt(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	published := publishPosts(t, baseURL, token, 3)

	setSameCreatedAt(t, published)

	// Спека задаёт порядок ленты как (created_at desc, id desc):
	// при равном времени сверху стоит больший идентификатор.
	want := append([]string(nil), published...)
	sort.Sort(sort.Reverse(sort.StringSlice(want)))

	page := feedPage(t, baseURL, token, nil)
	requireFeedPosts(t, page, want, "лента постов с одинаковым временем")

	walked := walkFeed(t, baseURL, token, 1)
	if !reflect.DeepEqual(walked, want) {
		t.Fatalf("проход страницами по одному посту дал %v, ожидалось %v", walked, want)
	}
}

// --- Доступ -----------------------------------------------------------------

// Ленту видит только вошедший: без токена и с недействительным токеном —
// 401 unauthorized (ФТ-7, «Запрос без токена»).
func TestFeedRequiresValidToken(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	publishPosts(t, baseURL, token, 1)

	cases := []struct {
		name  string
		token string
	}{
		{"без токена", ""},
		{"неизвестный токен", "этого-токена-сервис-не-выдавал"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			resp := fetchFeed(t, baseURL, testCase.token, feedParams(10, ""))
			requireError(t, resp, http.StatusUnauthorized, "unauthorized")
		})
	}
}
