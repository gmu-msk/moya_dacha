package tests

// Тесты по specs/028-post-tags.md, требования 1–19: тэги поста — хэштеги
// в подписи, их пересчёт при правке подписи, лента по тэгу и подсказки
// тэгов. Написаны по спецификации и контракту, без взгляда на реализацию
// (ADR-0002).
//
// База перед каждым тестом чистая (startAPI), поэтому популярность тэгов
// в подсказках считается только по постам, которые завёл сам тест.

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// ptDictionary — стартовый словарь подсказок в порядке требования 16.
var ptDictionary = []string{
	"поделюсь", "советы", "вопрос", "дневник", "урожай", "рассада", "теплица",
	"полив", "удобрения", "вредители", "болезни", "заготовки", "цветы",
	"томаты", "огурцы", "перцы", "картофель", "капуста", "кабачки", "тыква",
	"морковь", "свёкла", "лук", "чеснок", "зелень", "клубника", "малина",
	"смородина", "яблоня", "груша", "вишня", "слива", "виноград",
}

// ptEmptyCommunity — подсказки пустого сообщества без текста: первые пять
// слов словаря.
var ptEmptyCommunity = []string{"поделюсь", "советы", "вопрос", "дневник", "урожай"}

// --- Представления из контракта -------------------------------------------

// ptPost — пост с полями, важными этой фиче (schema Post). tags —
// указатель: нет поля или null — nil, а не пустой массив.
type ptPost struct {
	ID         string    `json:"id"`
	CreatedAt  string    `json:"created_at"`
	Caption    string    `json:"caption"`
	Visibility *string   `json:"visibility"`
	EditedAt   *string   `json:"edited_at"`
	Tags       *[]string `json:"tags"`
}

// ptFeed — страница ленты или постов пользователя (schema Feed).
type ptFeed struct {
	Items      []ptPost `json:"items"`
	NextCursor *string  `json:"next_cursor"`
}

// --- Хелперы --------------------------------------------------------------

// ptPhone — номер n-го участника теста; не пересекается с другими файлами.
func ptPhone(n int) string {
	return fmt.Sprintf("+7 (900) 728-00-%02d", n)
}

// newPTUser регистрирует n-го участника теста.
func newPTUser(t *testing.T, baseURL string, n int) dachnik {
	t.Helper()

	token, id := signIn(t, baseURL, ptPhone(n))

	return dachnik{token: token, id: id}
}

// ptPublishRaw публикует пост с одной новой фотографией; extra — поля
// тела сверх media_ids (caption, visibility, place_id, tags старой сборки).
func ptPublishRaw(t *testing.T, baseURL, token string, extra map[string]any) (*http.Response, mediaPayload) {
	t.Helper()

	photo := photoOf(t, baseURL, token, 60, 40)
	body := map[string]any{"media_ids": []string{photo.ID}}
	for k, v := range extra {
		body[k] = v
	}

	return createPost(t, baseURL, token, body), photo
}

// ptPostOK требует статус и возвращает пост.
func ptPostOK(t *testing.T, resp *http.Response, status int, where string) ptPost {
	t.Helper()

	if resp.StatusCode != status {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус %d, получен %d %s", where, status, resp.StatusCode, code)
	}

	var post ptPost
	decode(t, resp, &post)

	return post
}

// ptPublish публикует пост с этой подписью (пустая — поля caption нет)
// и видимостью (пустая — поля нет), требует 201.
func ptPublish(t *testing.T, baseURL, token, caption, visibility string) ptPost {
	t.Helper()

	extra := map[string]any{}
	if caption != "" {
		extra["caption"] = caption
	}
	if visibility != "" {
		extra["visibility"] = visibility
	}

	resp, _ := ptPublishRaw(t, baseURL, token, extra)

	return ptPostOK(t, resp, http.StatusCreated, fmt.Sprintf("публикация поста с подписью %q", caption))
}

// ptCaption — подпись «Грядка» с хэштегами этих тэгов через пробел.
func ptCaption(tags ...string) string {
	var b strings.Builder
	b.WriteString("Грядка")
	for _, tag := range tags {
		b.WriteString(" #")
		b.WriteString(tag)
	}

	return b.String()
}

// ptRequireTags требует у поста ровно эти тэги в этом порядке; пустой
// want — пустой массив, а не null и не отсутствие поля.
func ptRequireTags(t *testing.T, post ptPost, want []string, where string) {
	t.Helper()

	if post.Tags == nil {
		t.Errorf("%s: у поста %s нет поля tags (или оно null), ожидалось %q", where, post.ID, want)
		return
	}
	if !slices.Equal(*post.Tags, want) && !(len(*post.Tags) == 0 && len(want) == 0) {
		t.Errorf("%s: tags = %q, ожидалось %q", where, *post.Tags, want)
	}
}

// ptRequireCaption требует подпись как написана: хэштеги остаются в тексте.
func ptRequireCaption(t *testing.T, post ptPost, want, where string) {
	t.Helper()

	if post.Caption != want {
		t.Errorf("%s: подпись %q, ожидалась %q", where, post.Caption, want)
	}
}

// ptGet открывает пост по адресу и требует 200.
func ptGet(t *testing.T, baseURL, token, postID string) ptPost {
	t.Helper()
	return ptPostOK(t, fetchPost(t, baseURL, token, postID), http.StatusOK, "GET /posts/"+postID)
}

// ptEdit меняет подпись своего поста и требует 200 с тем же постом.
func ptEdit(t *testing.T, baseURL, token, postID, caption string) ptPost {
	t.Helper()

	post := ptPostOK(t, editCaptionText(t, baseURL, token, postID, caption), http.StatusOK,
		fmt.Sprintf("правка подписи на %q", caption))
	if post.ID != postID {
		t.Fatalf("правка подписи поста %s вернула пост %s", postID, post.ID)
	}

	return post
}

// ptFeedReq запрашивает ленту с готовой строкой запроса.
func ptFeedReq(t *testing.T, baseURL, token, query string) *http.Response {
	t.Helper()
	return fetchFeedRaw(t, baseURL, token, query)
}

// ptFeedOK требует 200 и возвращает страницу.
func ptFeedOK(t *testing.T, resp *http.Response, where string) ptFeed {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус 200, получен %d %s", where, resp.StatusCode, code)
	}

	var page ptFeed
	decode(t, resp, &page)

	return page
}

// ptTagFeed — первая страница ленты (limit 50) с тэгом tag и вкладкой
// scope (пустая — параметра нет).
func ptTagFeed(t *testing.T, baseURL, token, tag, scope string) ptFeed {
	t.Helper()

	params := url.Values{"limit": {"50"}, "tag": {tag}}
	if scope != "" {
		params.Set("scope", scope)
	}

	return ptFeedOK(t, ptFeedReq(t, baseURL, token, params.Encode()), "лента ?"+params.Encode())
}

// ptIDs — идентификаторы постов страницы по порядку.
func ptIDs(items []ptPost) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	return ids
}

// ptRequireIDs требует ровно эти посты в этом порядке.
func ptRequireIDs(t *testing.T, got, want []string, where string) {
	t.Helper()

	if !slices.Equal(got, want) && !(len(got) == 0 && len(want) == 0) {
		t.Errorf("%s: посты %q, ожидалось %q", where, got, want)
	}
}

// ptFind — пост с этим id на странице или nil.
func ptFind(page ptFeed, postID string) *ptPost {
	for i := range page.Items {
		if page.Items[i].ID == postID {
			return &page.Items[i]
		}
	}

	return nil
}

// ptUserPosts — первая страница постов пользователя (limit 50).
func ptUserPosts(t *testing.T, baseURL, token, userID string) ptFeed {
	t.Helper()
	return ptFeedOK(t, fetchUserPosts(t, baseURL, token, userID, feedParams(50, "")), "посты пользователя")
}

// ptSuggestReq — GET /tags/suggestions с готовыми параметрами.
func ptSuggestReq(t *testing.T, baseURL, token string, params url.Values) *http.Response {
	t.Helper()

	address := baseURL + "/tags/suggestions"
	if len(params) > 0 {
		address += "?" + params.Encode()
	}

	return do(t, http.MethodGet, address, token, nil)
}

// ptSuggestParams — подсказки с готовыми параметрами; требует 200
// и массив items не длиннее пяти.
func ptSuggestParams(t *testing.T, baseURL, token string, params url.Values) []string {
	t.Helper()

	resp := ptSuggestReq(t, baseURL, token, params)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("подсказки ?%s: ожидался статус 200, получен %d", params.Encode(), resp.StatusCode)
	}

	var body struct {
		Items *[]string `json:"items"`
	}
	decode(t, resp, &body)

	if body.Items == nil {
		t.Fatalf("подсказки ?%s: нет массива items (или он null)", params.Encode())
	}
	if len(*body.Items) > 5 {
		t.Errorf("подсказки ?%s: %d тэгов, а больше пяти быть не может", params.Encode(), len(*body.Items))
	}

	return *body.Items
}

// ptSuggest — подсказки для текста text (пустой — параметра нет) без
// exclude перечисленных тэгов.
func ptSuggest(t *testing.T, baseURL, token, text string, exclude ...string) []string {
	t.Helper()
	return ptSuggestPrefix(t, baseURL, token, text, "", exclude...)
}

// ptSuggestPrefix — то же с параметром prefix (пустой — параметра нет).
func ptSuggestPrefix(t *testing.T, baseURL, token, text, prefix string, exclude ...string) []string {
	t.Helper()

	params := url.Values{}
	if text != "" {
		params.Set("text", text)
	}
	if prefix != "" {
		params.Set("prefix", prefix)
	}
	if len(exclude) > 0 {
		params["exclude"] = exclude
	}

	return ptSuggestParams(t, baseURL, token, params)
}

// ptRequireSuggestions требует ровно эти подсказки в этом порядке.
func ptRequireSuggestions(t *testing.T, got, want []string, where string) {
	t.Helper()

	if !slices.Equal(got, want) && !(len(got) == 0 && len(want) == 0) {
		t.Errorf("%s: подсказки %q, ожидалось %q", where, got, want)
	}
}

// ptTagged публикует count постов с хэштегами этих тэгов в подписи.
func ptTagged(t *testing.T, baseURL, token string, count int, visibility string, tags ...string) {
	t.Helper()

	for i := 0; i < count; i++ {
		ptPublish(t, baseURL, token, ptCaption(tags...), visibility)
	}
}

// ptManyTags — count разных правильных тэгов.
func ptManyTags(count int) []string {
	tags := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		tags = append(tags, fmt.Sprintf("тэг%d", i))
	}

	return tags
}

// --- Разбор хэштегов при публикации (требования 1–5, 7) --------------------

// Где начинается и где кончается хэштег (требование 3, «Ограничения
// и edge cases»), что за слово становится тэгом (требования 1, 2, 4).
// Подпись остаётся как написана (требование 6).
func TestPostTagsParsedFromCaption(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	cases := []struct {
		name    string
		caption string
		want    []string
	}{
		{"в начале подписи", "#груша поспела", []string{"груша"}},
		{"после пробела", "Поспела #груша", []string{"груша"}},
		{"после перевода строки", "Поспела\n#груша", []string{"груша"}},
		{"после запятой", "урожай,#груша", []string{"груша"}},
		{"после скобки", "Поспела (#груша)", []string{"груша"}},
		{"двойная решётка", "##груша", []string{"груша"}},
		{"внутри слова — не хэштег", "яблоки#груша", []string{}},
		{"после подчёркивания — не хэштег", "яблоки_#груша", []string{}},
		{"после латинской буквы — не хэштег", "e-mail#1", []string{}},
		{"цифра в начале", "#1 место", []string{"1"}},
		{"кончается на точке", "Поспела #груша.", []string{"груша"}},
		{"кончается на восклицательном знаке", "Поспела #груша!", []string{"груша"}},
		{"кончается на эмодзи", "Поспела #груша🍐", []string{"груша"}},
		{"кончается на пробеле", "#зелёный лук", []string{"зелёный"}},
		{"вторая решётка вплотную — не хэштег", "#груша#сорт", []string{"груша"}},
		{"решётка без слова", "# груша", []string{}},
		{"одна решётка", "#", []string{}},
		{"регистр", "#Груша #ПОДЕЛЮСЬ #СоРт", []string{"груша", "поделюсь", "сорт"}},
		{"ё не заменяется", "#Свёкла #ЁЛКА", []string{"свёкла", "ёлка"}},
		{"цифры, дефис, подчёркивание, латиница, другие алфавиты",
			"#Сорт-2 #f1_гибрид #2026 #Pear #მსხალი",
			[]string{"сорт-2", "f1_гибрид", "2026", "pear", "მსხალი"}},
		{"ровно 30 знаков", "#" + strings.Repeat("Я", 30), []string{strings.Repeat("я", 30)}},
		{"буква внутри дефисов", "#-а- #_1_", []string{"-а-", "_1_"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			created := ptPublish(t, baseURL, author.token, c.caption, "")
			ptRequireTags(t, created, c.want, "ответ на публикацию")
			ptRequireCaption(t, created, c.caption, "ответ на публикацию")

			got := ptGet(t, baseURL, author.token, created.ID)
			ptRequireTags(t, got, c.want, "GET /posts/{id}")
			ptRequireCaption(t, got, c.caption, "GET /posts/{id}")
		})
	}
}

// Хэштег, слово которого не тэг (длиннее 30 знаков, из одних `-` и `_`), —
// просто текст: пост публикуется, ошибки нет (требование 4).
func TestPostTagsInvalidHashtagIsJustText(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	cases := []struct {
		name    string
		caption string
		want    []string
	}{
		{"31 знак", "#" + strings.Repeat("я", 31), []string{}},
		{"только дефисы", "Грядка #---", []string{}},
		{"только дефисы и подчёркивания", "Грядка #_-_", []string{}},
		{"длинный рядом с правильным", "#" + strings.Repeat("Я", 31) + " #груша", []string{"груша"}},
		{"дефисы рядом с правильным", "#груша #--- #сорт", []string{"груша", "сорт"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			created := ptPublish(t, baseURL, author.token, c.caption, "")
			ptRequireTags(t, created, c.want, "ответ на публикацию")
			ptRequireCaption(t, created, c.caption, "ответ на публикацию")
			ptRequireTags(t, ptGet(t, baseURL, author.token, created.ID), c.want, "GET /posts/{id}")
		})
	}
}

// Повторы схлопываются без учёта регистра, порядок — по первому появлению
// в подписи (требование 5).
func TestPostTagsDuplicatesCollapsedInCaptionOrder(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	cases := []struct {
		name    string
		caption string
		want    []string
	}{
		{"повторы", "#груша #сорт #Груша #СОРТ ##груша #урожай", []string{"груша", "сорт", "урожай"}},
		{"порядок первого появления", "#урожай и #груша, снова #Урожай", []string{"урожай", "груша"}},
		{"порядок подписи, а не алфавит", "#сорт #груша #урожай", []string{"сорт", "груша", "урожай"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			created := ptPublish(t, baseURL, author.token, c.caption, "")
			ptRequireTags(t, created, c.want, "ответ на публикацию")
			ptRequireTags(t, ptGet(t, baseURL, author.token, created.ID), c.want, "GET /posts/{id}")
		})
	}
}

// Тэгами становятся первые 10 разных хэштегов; дальше — текст, ошибки нет
// (требование 5).
func TestPostTagsFirstTenOnly(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	ten := ptManyTags(10)

	t.Run("ровно 10", func(t *testing.T) {
		ptRequireTags(t, ptPublish(t, baseURL, author.token, ptCaption(ten...), ""), ten, "10 хэштегов")
	})

	t.Run("11 — первые 10", func(t *testing.T) {
		caption := ptCaption(ptManyTags(11)...)
		post := ptPublish(t, baseURL, author.token, caption, "")
		ptRequireTags(t, post, ten, "11 хэштегов")
		ptRequireCaption(t, post, caption, "11 хэштегов")
		ptRequireTags(t, ptGet(t, baseURL, author.token, post.ID), ten, "GET /posts/{id}")
	})

	t.Run("15 — первые 10", func(t *testing.T) {
		ptRequireTags(t, ptPublish(t, baseURL, author.token, ptCaption(ptManyTags(15)...), ""), ten, "15 хэштегов")
	})

	t.Run("повторы места не занимают", func(t *testing.T) {
		tags := append(append([]string{}, ten[:5]...), "ТЭГ1", "тэг2", "тэг3")
		tags = append(tags, ten[5:]...)
		ptRequireTags(t, ptPublish(t, baseURL, author.token, ptCaption(tags...), ""), ten, "10 разных с повторами")
	})

	t.Run("хэштеги-не-тэги места не занимают", func(t *testing.T) {
		tags := append(append([]string{}, ten[:5]...), "---", strings.Repeat("я", 31))
		tags = append(tags, ten[5:]...)
		ptRequireTags(t, ptPublish(t, baseURL, author.token, ptCaption(tags...), ""), ten, "10 тэгов и два не-тэга")
	})
}

// Пост без подписи, с подписью без хэштегов и с одними хэштегами-не-тэгами —
// без тэгов, и в ответах у него пустой массив, а не null и не отсутствие
// поля (требование 7).
func TestPostTagsAbsentMeansEmptyArrayEverywhere(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	posts := map[string]ptPost{
		"без подписи":          ptPublish(t, baseURL, author.token, "", ""),
		"подпись без хэштегов": ptPublish(t, baseURL, author.token, "Грядка", ""),
		"одни не-тэги":         ptPublish(t, baseURL, author.token, "яблоки#груша #---", ""),
	}

	feed := ptFeedOK(t, fetchFeed(t, baseURL, author.token, feedParams(50, "")), "лента")
	profile := ptUserPosts(t, baseURL, author.token, author.id)

	for name, post := range posts {
		ptRequireTags(t, post, []string{}, name+": ответ на публикацию")
		ptRequireTags(t, ptGet(t, baseURL, author.token, post.ID), []string{}, name+": GET /posts/{id}")

		if item := ptFind(feed, post.ID); item == nil {
			t.Errorf("%s: поста нет в ленте", name)
		} else {
			ptRequireTags(t, *item, []string{}, name+": лента")
		}
		if item := ptFind(profile, post.ID); item == nil {
			t.Errorf("%s: поста нет в постах автора", name)
		} else {
			ptRequireTags(t, *item, []string{}, name+": посты автора")
		}
	}
}

// Тэги поста — в ответе на публикацию, по адресу, в ленте, в постах
// автора, в ответах на правку подписи и видимости; в порядке подписи
// (требование 7).
func TestPostTagsInEveryPostResponse(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	reader := newPTUser(t, baseURL, 2)

	want := []string{"сорт", "груша", "урожай"}
	created := ptPublish(t, baseURL, author.token, "#Сорт и #груша на #урожай", "")
	ptRequireTags(t, created, want, "ответ на публикацию")

	ptRequireTags(t, ptGet(t, baseURL, reader.token, created.ID), want, "GET /posts/{id} чужими глазами")

	feed := ptFeedOK(t, fetchFeed(t, baseURL, reader.token, feedParams(50, "")), "лента")
	if item := ptFind(feed, created.ID); item == nil {
		t.Fatal("поста нет в ленте")
	} else {
		ptRequireTags(t, *item, want, "лента")
	}

	profile := ptUserPosts(t, baseURL, reader.token, author.id)
	if item := ptFind(profile, created.ID); item == nil {
		t.Fatal("поста нет в постах автора")
	} else {
		ptRequireTags(t, *item, want, "посты автора")
	}

	edited := ptEdit(t, baseURL, author.token, created.ID, "Новая подпись: #сорт #груша #урожай")
	ptRequireTags(t, edited, want, "ответ на правку подписи")

	changed := ptPostOK(t, setVisibility(t, baseURL, author.token, created.ID, map[string]any{"visibility": visibilityFriends}),
		http.StatusOK, "смена видимости")
	ptRequireTags(t, changed, want, "ответ на смену видимости")
}

// Тэги видны всем, кому виден пост: другу — у поста «друзьям» (требование 8).
func TestPostTagsVisibleToWhoeverSeesThePost(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	friend := newPTUser(t, baseURL, 2)
	makeFriends(t, baseURL, author, friend)

	post := ptPublish(t, baseURL, author.token, ptCaption("груша"), visibilityFriends)

	ptRequireTags(t, ptGet(t, baseURL, friend.token, post.ID), []string{"груша"}, "друг: GET /posts/{id}")

	feed := ptFeedOK(t, fetchFeed(t, baseURL, friend.token, feedParams(50, "")), "лента друга")
	if item := ptFind(feed, post.ID); item == nil {
		t.Fatal("поста «друзьям» нет в ленте друга")
	} else {
		ptRequireTags(t, *item, []string{"груша"}, "лента друга")
	}
}

// --- Поле tags старой сборки (требование 9) --------------------------------

// Поле tags в теле публикации сервер пропускает мимо: тэги — только из
// подписи, и ни плохой тэг, ни одиннадцать тэгов в поле не дают отказа
// (требование 9).
func TestPostTagsFieldInBodyIgnored(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	cases := []struct {
		name    string
		caption string
		tags    any
		want    []string
	}{
		{"поле и хэштеги разные", "Грядка #сорт", []string{"груша"}, []string{"сорт"}},
		{"поле без подписи", "", []string{"груша", "урожай"}, []string{}},
		{"плохой тэг в поле", "Грядка #груша", []string{"зелёный лук", "груша!"}, []string{"груша"}},
		{"одиннадцать тэгов в поле", "Грядка", ptManyTags(11), []string{}},
		{"null в поле", "#груша", nil, []string{"груша"}},
		{"пустой массив в поле", "#груша", []string{}, []string{"груша"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			extra := map[string]any{"tags": c.tags}
			if c.caption != "" {
				extra["caption"] = c.caption
			}

			resp, _ := ptPublishRaw(t, baseURL, author.token, extra)
			created := ptPostOK(t, resp, http.StatusCreated, c.name)
			ptRequireTags(t, created, c.want, "ответ на публикацию")
			ptRequireTags(t, ptGet(t, baseURL, author.token, created.ID), c.want, "GET /posts/{id}")
		})
	}
}

// Хэштеги отказом не бывают: остальные проверки публикации идут как без
// них — длинная подпись с хэштегами — invalid_caption, хэштеги и поле
// tags при неизвестном месте — unknown_place (требования 4, 5, 9).
func TestPostTagsNeverRejectPost(t *testing.T) {
	baseURL, _ := startPlaces(t)
	author := newPTUser(t, baseURL, 1)

	long := strings.Repeat("я", 990) + " #груша #сорт"
	resp, _ := ptPublishRaw(t, baseURL, author.token, map[string]any{"caption": long})
	requireCodeE(t, resp, http.StatusBadRequest, "invalid_caption", "подпись длиннее 1000 знаков с хэштегами")

	resp, _ = ptPublishRaw(t, baseURL, author.token, map[string]any{
		"place_id": unknownID,
		"caption":  ptCaption(ptManyTags(11)...) + " #" + strings.Repeat("я", 31),
		"tags":     []string{"зелёный лук"},
	})
	requireCodeE(t, resp, http.StatusBadRequest, "unknown_place", "хэштеги, поле tags и неизвестное место")

	if n := len(ptUserPosts(t, baseURL, author.token, author.id).Items); n != 0 {
		t.Errorf("после отказов у автора %d постов, ожидалось 0", n)
	}
}

// --- Правка подписи (требования 6, 7, 10) ----------------------------------

// Правка подписи заменяет тэги целиком тэгами новой подписи: убранный
// хэштег снимает тэг, новый — ставит; подпись остаётся как написана;
// новые тэги видны по адресу, в ленте и в ленте по тэгу (сценарий, шаг 7;
// требования 6, 7, 12).
func TestPostTagsCaptionEditReplacesTags(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	reader := newPTUser(t, baseURL, 2)

	post := ptPublish(t, baseURL, author.token, "Груши мелкие, но сладкие #груша #сорт", "")
	ptRequireTags(t, post, []string{"груша", "сорт"}, "до правки")

	caption := "Груши мелкие, но сладкие #Урожай #груша #урожай"
	updated := ptEdit(t, baseURL, author.token, post.ID, caption)
	want := []string{"урожай", "груша"}
	ptRequireTags(t, updated, want, "ответ на правку подписи")
	ptRequireCaption(t, updated, caption, "ответ на правку подписи")

	got := ptGet(t, baseURL, reader.token, post.ID)
	ptRequireTags(t, got, want, "GET /posts/{id}")
	ptRequireCaption(t, got, caption, "GET /posts/{id}")

	feed := ptFeedOK(t, fetchFeed(t, baseURL, reader.token, feedParams(50, "")), "лента")
	if item := ptFind(feed, post.ID); item == nil {
		t.Fatal("поста нет в ленте")
	} else {
		ptRequireTags(t, *item, want, "лента")
	}

	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, reader.token, "сорт", "").Items), nil, "лента по снятому тэгу «сорт»")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, reader.token, "урожай", "").Items), []string{post.ID}, "лента по новому тэгу «урожай»")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, reader.token, "груша", "").Items), []string{post.ID}, "лента по оставшемуся тэгу «груша»")

	// Подпись без хэштегов снимает все тэги.
	ptRequireTags(t, ptEdit(t, baseURL, author.token, post.ID, "Груши мелкие"), []string{}, "подпись без хэштегов")
	ptRequireTags(t, ptGet(t, baseURL, author.token, post.ID), []string{}, "GET после снятия всех")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, reader.token, "груша", "").Items), nil, "лента по снятому тэгу «груша»")

	// Посту без тэгов правка подписи их ставит — по тем же правилам разбора.
	bare := ptPublish(t, baseURL, author.token, "Грядка", "")
	ptRequireTags(t, ptEdit(t, baseURL, author.token, bare.ID, "Грядка #Дневник яблоки#груша #---"),
		[]string{"дневник"}, "хэштег посту без тэгов")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, reader.token, "дневник", "").Items), []string{bare.ID}, "лента по «дневник»")

	// Первые 10 разных — и при правке.
	ptRequireTags(t, ptEdit(t, baseURL, author.token, bare.ID, ptCaption(ptManyTags(12)...)),
		ptManyTags(10), "12 хэштегов при правке")
}

// edited_at — по правилам 022: правка, поменявшая только хэштеги, —
// правка; правка текста без смены тэгов — тоже; та же подпись — не правка,
// и тэги остаются (требование 10).
func TestPostTagsCaptionEditEditedAt(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	t.Run("та же подпись — не правка", func(t *testing.T) {
		post := ptPublish(t, baseURL, author.token, "Грядка #груша", "")
		requireNotEdited(t, post.EditedAt, "только что опубликованный")

		same := ptEdit(t, baseURL, author.token, post.ID, "Грядка #груша")
		requireNotEdited(t, same.EditedAt, "ответ на правку той же подписью")
		ptRequireTags(t, same, []string{"груша"}, "ответ на правку той же подписью")
		requireNotEdited(t, ptGet(t, baseURL, author.token, post.ID).EditedAt, "GET после правки той же подписью")
	})

	t.Run("поменялись только хэштеги", func(t *testing.T) {
		post := ptPublish(t, baseURL, author.token, "Грядка #груша", "")

		edited := ptEdit(t, baseURL, author.token, post.ID, "Грядка #сорт")
		requireEditedAt(t, edited.EditedAt, edited.CreatedAt, "ответ на смену хэштега")
		ptRequireTags(t, edited, []string{"сорт"}, "ответ на смену хэштега")
	})

	t.Run("текст поменялся, тэги те же", func(t *testing.T) {
		post := ptPublish(t, baseURL, author.token, "Грядка #груша", "")

		edited := ptEdit(t, baseURL, author.token, post.ID, "Большая грядка #груша")
		editedAt := requireEditedAt(t, edited.EditedAt, edited.CreatedAt, "ответ на правку текста")
		ptRequireTags(t, edited, []string{"груша"}, "ответ на правку текста")

		got := ptGet(t, baseURL, author.token, post.ID)
		if got.EditedAt == nil || *got.EditedAt != editedAt {
			t.Errorf("GET: edited_at %v, ожидался %q", got.EditedAt, editedAt)
		}
	})

	t.Run("регистр хэштега — тэги те же, подпись другая", func(t *testing.T) {
		post := ptPublish(t, baseURL, author.token, "Грядка #груша", "")

		edited := ptEdit(t, baseURL, author.token, post.ID, "Грядка #Груша")
		requireEditedAt(t, edited.EditedAt, edited.CreatedAt, "ответ на смену регистра")
		ptRequireTags(t, edited, []string{"груша"}, "ответ на смену регистра")
		ptRequireCaption(t, edited, "Грядка #Груша", "ответ на смену регистра")
	})
}

// Отказ в правке подписи оставляет и подпись, и тэги прежними; ручки
// PUT /posts/{id}/tags больше нет (требования 6, 9).
func TestPostTagsCaptionEditRejectionKeepsTags(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	stranger := newPTUser(t, baseURL, 2)

	original := []string{"груша", "сорт"}
	post := ptPublish(t, baseURL, author.token, ptCaption(original...), "")

	requireStays := func(t *testing.T, where string) {
		t.Helper()
		got := ptGet(t, baseURL, author.token, post.ID)
		ptRequireTags(t, got, original, where+": тэги после отказа")
		ptRequireCaption(t, got, post.Caption, where+": подпись после отказа")
	}

	t.Run("чужой пост", func(t *testing.T) {
		resp := editCaptionText(t, baseURL, stranger.token, post.ID, "Моя теперь #урожай")
		requireCodeE(t, resp, http.StatusForbidden, "not_your_post", "чужой пост")
		requireStays(t, "чужой пост")
	})

	t.Run("подпись длиннее 1000 знаков", func(t *testing.T) {
		resp := editCaptionText(t, baseURL, author.token, post.ID, strings.Repeat("я", 995)+" #урожай")
		requireCodeE(t, resp, http.StatusBadRequest, "invalid_caption", "длинная подпись")
		requireStays(t, "длинная подпись")
	})

	t.Run("ручки PUT /tags нет", func(t *testing.T) {
		resp := do(t, http.MethodPut, baseURL+"/posts/"+post.ID+"/tags", author.token,
			map[string]any{"tags": []string{"урожай"}})
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("PUT /posts/{id}/tags: ожидался статус 404, получен %d", resp.StatusCode)
		}
		requireStays(t, "PUT /tags")
	})
}

// --- Удаление поста (требование 11) ----------------------------------------

// Удалённый пост уходит вместе с тэгами: его нет в ленте по тэгу, тэг не
// подсказывается как тэг сообщества, строк в post_tags не осталось.
func TestPostTagsGoAwayWithPost(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	post := ptPublish(t, baseURL, author.token, ptCaption("альфа", "бета"), "")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, author.token, ""),
		[]string{"альфа", "бета", "поделюсь", "советы", "вопрос"}, "подсказки до удаления")

	requireDeleted(t, deletePost(t, baseURL, author.token, post.ID))

	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, author.token, "альфа", "").Items), nil, "лента по тэгу после удаления")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, author.token, ""), ptEmptyCommunity, "подсказки после удаления")
	requireNoRows(t, "post_tags", "post_id", post.ID, "после удаления поста")
}

// --- GET /api/feed?tag= (требования 12–14) ---------------------------------

// Лента сужается до постов с тэгом, новые сверху; совпадение точное;
// у поста в ответе все его тэги, а не только искомый (требования 12, 14).
func TestPostTagsFeedFiltersByExactTag(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	reader := newPTUser(t, baseURL, 2)

	pear := ptPublish(t, baseURL, author.token, ptCaption("груша"), "")
	ptPublish(t, baseURL, author.token, ptCaption(), "")
	ptPublish(t, baseURL, author.token, ptCaption("груши"), "")
	ptPublish(t, baseURL, author.token, ptCaption("грушевый"), "")
	pearAndSort := ptPublish(t, baseURL, reader.token, ptCaption("сорт", "груша"), "")
	ptPublish(t, baseURL, author.token, ptCaption("свёкла"), "")

	page := ptTagFeed(t, baseURL, reader.token, "груша", "")
	ptRequireIDs(t, ptIDs(page.Items), []string{pearAndSort.ID, pear.ID}, "лента по «груша»")
	if item := ptFind(page, pearAndSort.ID); item != nil {
		ptRequireTags(t, *item, []string{"сорт", "груша"}, "пост в ленте по тэгу")
	}
	if page.NextCursor != nil {
		t.Errorf("лента по «груша»: два поста на странице из 50, а next_cursor есть")
	}

	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, reader.token, "груш", "").Items), nil, "«груш» не находит «груша»")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, reader.token, "гру", "").Items), nil, "«гру»")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, reader.token, "свекла", "").Items), nil, "«свекла» не находит «свёкла»")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, reader.token, "урожай", "").Items), nil, "тэг, которого нет ни у кого")
}

// Параметр tag нормализуется как тэг поста; пустой после нормализации —
// как без параметра; неподходящий — 400 invalid_tag (требование 13).
func TestPostTagsFeedTagParameterNormalized(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	pear := ptPublish(t, baseURL, author.token, ptCaption("груша"), "")
	bare := ptPublish(t, baseURL, author.token, ptCaption(), "")
	other := ptPublish(t, baseURL, author.token, ptCaption("сорт"), "")

	for _, tag := range []string{"Груша", "#груша", "##ГРУША", " груша "} {
		ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, author.token, tag, "").Items), []string{pear.ID},
			fmt.Sprintf("tag=%q", tag))
	}

	all := []string{other.ID, bare.ID, pear.ID}
	for _, query := range []string{"tag=", "tag=%20%20", "tag=%23", "tag=%23%23"} {
		page := ptFeedOK(t, ptFeedReq(t, baseURL, author.token, query+"&limit=50"), query)
		ptRequireIDs(t, ptIDs(page.Items), all, query+" — как без параметра")
	}

	for _, tag := range []string{"зелёный лук", strings.Repeat("я", 31), "---", "груша!", "a.b"} {
		resp := ptFeedReq(t, baseURL, author.token, url.Values{"tag": {tag}}.Encode())
		requireCodeE(t, resp, http.StatusBadRequest, "invalid_tag", fmt.Sprintf("лента с tag=%q", tag))
	}
}

// Лента по тэгу учитывает видимость 013: «только мне» — одному автору,
// «друзьям» — друзьям, «всем» у закрытого профиля — подписчикам
// (требования 8, 12).
func TestPostTagsFeedRespectsVisibility(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	friend := newPTUser(t, baseURL, 2)
	stranger := newPTUser(t, baseURL, 3)
	closed := newPTUser(t, baseURL, 4)
	makeFriends(t, baseURL, author, friend)

	public := ptPublish(t, baseURL, author.token, ptCaption("груша"), "")
	forFriends := ptPublish(t, baseURL, author.token, ptCaption("груша"), visibilityFriends)
	onlyMe := ptPublish(t, baseURL, author.token, ptCaption("груша"), visibilityMe)
	closedPost := ptPublish(t, baseURL, closed.token, ptCaption("груша"), "")
	setClosed(t, baseURL, closed.token, true)

	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, author.token, "груша", "").Items),
		[]string{onlyMe.ID, forFriends.ID, public.ID}, "автор")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, friend.token, "груша", "").Items),
		[]string{forFriends.ID, public.ID}, "друг")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, stranger.token, "груша", "").Items),
		[]string{public.ID}, "посторонний")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, closed.token, "груша", "").Items),
		[]string{closedPost.ID, public.ID}, "автор закрытого профиля")
}

// С тэгом работает и вкладка «Подписки» (требование 12).
func TestPostTagsFeedWithFollowingScope(t *testing.T) {
	baseURL := startAPI(t)
	viewer := newPTUser(t, baseURL, 1)
	followed := newPTUser(t, baseURL, 2)
	other := newPTUser(t, baseURL, 3)
	followOK(t, baseURL, viewer, followed.id)

	fromFollowed := ptPublish(t, baseURL, followed.token, ptCaption("груша"), "")
	ptPublish(t, baseURL, followed.token, ptCaption("сорт"), "")
	fromOther := ptPublish(t, baseURL, other.token, ptCaption("груша"), "")
	own := ptPublish(t, baseURL, viewer.token, ptCaption("груша"), "")

	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, viewer.token, "груша", "following").Items),
		[]string{own.ID, fromFollowed.ID}, "«Подписки» по тэгу")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, viewer.token, "#Груша", "all").Items),
		[]string{own.ID, fromOther.ID, fromFollowed.ID}, "«Все» по тэгу")
}

// Курсор и limit работают с тэгом: обход страницами отдаёт все посты
// с тэгом без повторов и без постов без тэга (требование 12).
func TestPostTagsFeedPagesWithCursor(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	var tagged []string
	for i := 0; i < 5; i++ {
		tagged = append(tagged, ptPublish(t, baseURL, author.token, ptCaption("груша"), "").ID)
		ptPublish(t, baseURL, author.token, ptCaption("сорт"), "")
	}
	slices.Reverse(tagged)

	var walked []string
	cursor := ""
	for page := 1; ; page++ {
		if page > 10 {
			t.Fatal("лента по тэгу не кончается за 10 страниц")
		}

		params := url.Values{"tag": {"груша"}, "limit": {"2"}}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		got := ptFeedOK(t, ptFeedReq(t, baseURL, author.token, params.Encode()), fmt.Sprintf("страница %d", page))
		if len(got.Items) > 2 {
			t.Fatalf("страница %d: %d постов при limit=2", page, len(got.Items))
		}
		walked = append(walked, ptIDs(got.Items)...)

		if got.NextCursor == nil {
			break
		}
		cursor = *got.NextCursor
	}

	ptRequireIDs(t, walked, tagged, "обход ленты по тэгу с limit=2")
}

// --- GET /api/tags/suggestions (требования 15–19) --------------------------

// Без токена — 401.
func TestPostTagsSuggestionsRequireToken(t *testing.T) {
	baseURL := startAPI(t)

	resp := ptSuggestReq(t, baseURL, "", url.Values{"text": {"груша"}})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("подсказки без токена: ожидался статус 401, получен %d", resp.StatusCode)
	}
}

// Пустое сообщество без текста — первые пять слов словаря (требования
// 16, 17).
func TestPostTagsSuggestionsEmptyCommunityGetsDictionary(t *testing.T) {
	baseURL := startAPI(t)
	viewer := newPTUser(t, baseURL, 1)

	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, ""), ptEmptyCommunity, "пустое сообщество")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "Просто хорошая погода"), ptEmptyCommunity,
		"пустое сообщество, текст без совпадений")
}

// Тэги сообщества — по популярности, при равной — по алфавиту, потом
// словарь без повторов (требование 17).
func TestPostTagsSuggestionsCommunityByPopularity(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	viewer := newPTUser(t, baseURL, 2)

	ptTagged(t, baseURL, author.token, 3, "", "яблоко")
	ptTagged(t, baseURL, author.token, 2, "", "бета")
	ptTagged(t, baseURL, author.token, 2, "", "альфа")
	ptTagged(t, baseURL, author.token, 1, "", "поделюсь")

	// «поделюсь» — тэг сообщества с популярностью 1, из словаря он второй
	// раз не приходит.
	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, ""),
		[]string{"яблоко", "альфа", "бета", "поделюсь", "советы"}, "популярность и алфавит")
}

// Популярность — по числу постов, видимых смотрящему; тэг невидимого поста
// не подсказывается (требование 16, «Ограничения и edge cases»).
func TestPostTagsSuggestionsCountOnlyVisiblePosts(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	friend := newPTUser(t, baseURL, 2)
	stranger := newPTUser(t, baseURL, 3)
	makeFriends(t, baseURL, author, friend)

	ptTagged(t, baseURL, author.token, 1, visibilityMe, "секрет")
	ptTagged(t, baseURL, author.token, 1, visibilityFriends, "тайна")

	ptRequireSuggestions(t, ptSuggest(t, baseURL, stranger.token, ""), ptEmptyCommunity, "посторонний")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, friend.token, ""),
		[]string{"тайна", "поделюсь", "советы", "вопрос", "дневник"}, "друг")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, author.token, ""),
		[]string{"секрет", "тайна", "поделюсь", "советы", "вопрос"}, "автор")

	// «альфа» у постороннего — 2 видимых поста, у автора — 2 + 3 своих
	// «только мне»; «бета» — 3 видимых всем.
	other := newPTUser(t, baseURL, 4)
	ptTagged(t, baseURL, other.token, 2, "", "альфа")
	ptTagged(t, baseURL, author.token, 3, visibilityMe, "альфа")
	ptTagged(t, baseURL, other.token, 3, "", "бета")

	ptRequireSuggestions(t, ptSuggest(t, baseURL, stranger.token, ""),
		[]string{"бета", "альфа", "поделюсь", "советы", "вопрос"}, "посторонний: считаются только видимые посты")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, author.token, ""),
		[]string{"альфа", "бета", "секрет", "тайна", "поделюсь"}, "автор: считаются и свои «только мне»")

	// Закрытый профиль: его посты «всем» у не-подписчика не считаются.
	closed := newPTUser(t, baseURL, 5)
	ptTagged(t, baseURL, closed.token, 4, "", "закрытый")
	setClosed(t, baseURL, closed.token, true)
	ptRequireSuggestions(t, ptSuggest(t, baseURL, stranger.token, ""),
		[]string{"бета", "альфа", "поделюсь", "советы", "вопрос"}, "посторонний: тэги закрытого профиля не считаются")
}

// Найденные в тексте — первыми, в порядке первого совпавшего слова,
// а не в порядке словаря и не по популярности (требования 17, 18).
func TestPostTagsSuggestionsTextMatchesFirstInTextOrder(t *testing.T) {
	baseURL := startAPI(t)
	viewer := newPTUser(t, baseURL, 1)

	cases := []struct {
		name string
		text string
		want []string
	}{
		{"сценарий: груши в подписи", "Груши в этом году мелкие, но сладкие",
			[]string{"груша", "поделюсь", "советы", "вопрос", "дневник"}},
		{"порядок слов в тексте", "слива вишня яблоня малина клубника смородина",
			[]string{"слива", "вишня", "яблоня", "малина", "клубника"}},
		{"формы слова и регистр", "ТОМАТОВ много, а огурец один; и лук.",
			[]string{"томаты", "лук", "поделюсь", "советы", "вопрос"}},
		{"«томат» — основа «томаты»", "томат", []string{"томаты", "поделюсь", "советы", "вопрос", "дневник"}},
		{"«грушу» → «груша»", "сорвал грушу", []string{"груша", "поделюсь", "советы", "вопрос", "дневник"}},
		{"«грушевый» — длиннее основы на 4", "грушевый сок", ptEmptyCommunity},
		{"«огурец» не находит «огурцы»", "огурец", ptEmptyCommunity},
		{"«свёклы» находит «свёкла»", "свёклы", []string{"свёкла", "поделюсь", "советы", "вопрос", "дневник"}},
		{"«свеклы» не находит «свёкла»", "свеклы", ptEmptyCommunity},
		{"«луковица» — длиннее «лук» на 5", "луковица", ptEmptyCommunity},
		{"слово из словаря дважды", "лук и ещё лук", []string{"лук", "поделюсь", "советы", "вопрос", "дневник"}},
		{"найденный в тексте тэг словаря не повторяется", "поделюсь рассадой",
			[]string{"поделюсь", "рассада", "советы", "вопрос", "дневник"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, c.text), c.want, fmt.Sprintf("text=%q", c.text))
		})
	}
}

// Найденный в тексте тэг сообщества обгоняет более популярные; одно
// слово совпало с несколькими тэгами — они идут как в пунктах 2–3
// порядка (требование 17).
func TestPostTagsSuggestionsTextMatchesBeatPopularity(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	ptTagged(t, baseURL, author.token, 3, "", "бета")
	ptTagged(t, baseURL, author.token, 1, "", "альфа")

	ptRequireSuggestions(t, ptSuggest(t, baseURL, author.token, "Альфы взошли"),
		[]string{"альфа", "бета", "поделюсь", "советы", "вопрос"}, "тэг сообщества найден в тексте")

	// Слово «груши» совпадает с «грушу» (2 поста), «груши» (1 пост) и
	// словарной «груша» — сообщество по популярности, потом словарь.
	ptTagged(t, baseURL, author.token, 2, "", "грушу")
	ptTagged(t, baseURL, author.token, 1, "", "груши")

	ptRequireSuggestions(t, ptSuggest(t, baseURL, author.token, "груши"),
		[]string{"грушу", "груши", "груша", "бета", "альфа"}, "одно слово — три тэга")
}

// Основа короче 3 знаков — совпадает только слово, равное тэгу
// (требование 18).
func TestPostTagsSuggestionsShortStemMatchesOnlyExactly(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	ptTagged(t, baseURL, author.token, 2, "", "альфа")
	ptTagged(t, baseURL, author.token, 1, "", "ели")

	ptRequireSuggestions(t, ptSuggest(t, baseURL, author.token, "еле"),
		[]string{"альфа", "ели", "поделюсь", "советы", "вопрос"}, "«еле» при основе «ел»")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, author.token, "ели"),
		[]string{"ели", "альфа", "поделюсь", "советы", "вопрос"}, "«ели» равно тэгу")
}

// Из text учитываются первые 1000 знаков — знаков, а не байт
// (требование 15).
func TestPostTagsSuggestionsTextFirstThousandChars(t *testing.T) {
	baseURL := startAPI(t)
	viewer := newPTUser(t, baseURL, 1)

	inside := strings.Repeat("ы ", 497) + "томаты" // ровно 1000 знаков
	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, inside),
		[]string{"томаты", "поделюсь", "советы", "вопрос", "дневник"}, "слово кончается на 1000-м знаке")

	outside := strings.Repeat("ы ", 500) + "томаты" // слово начинается с 1001-го
	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, outside), ptEmptyCommunity, "слово за 1000-м знаком")
}

// exclude убирает тэги из ответа, нормализуется, неподходящие значения
// игнорируются, освободившиеся места занимают следующие (требование 15).
func TestPostTagsSuggestionsExclude(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	viewer := newPTUser(t, baseURL, 2)

	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "", "поделюсь", "советы"),
		[]string{"вопрос", "дневник", "урожай", "рассада", "теплица"}, "исключены два слова словаря")

	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "", "#ПОДЕЛЮСЬ", " Советы "),
		[]string{"вопрос", "дневник", "урожай", "рассада", "теплица"}, "exclude нормализуется")

	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "", "зелёный лук", "---", strings.Repeat("я", 31), ""),
		ptEmptyCommunity, "неподходящие exclude игнорируются")

	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "Груши поспели", "груша"),
		ptEmptyCommunity, "исключён найденный в тексте")

	ptTagged(t, baseURL, author.token, 2, "", "альфа")
	ptTagged(t, baseURL, author.token, 1, "", "бета")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "", "альфа"),
		[]string{"бета", "поделюсь", "советы", "вопрос", "дневник"}, "исключён тэг сообщества")
}

// Кандидатов нет — 200 и пустой items (требование 19).
func TestPostTagsSuggestionsAlways200(t *testing.T) {
	baseURL := startAPI(t)
	viewer := newPTUser(t, baseURL, 1)

	got := ptSuggest(t, baseURL, viewer.token, "груши и томаты", ptDictionary...)
	ptRequireSuggestions(t, got, []string{}, "исключён весь словарь")

	// Остались последние слова словаря — они и приходят, по порядку.
	got = ptSuggest(t, baseURL, viewer.token, "", ptDictionary[:30]...)
	ptRequireSuggestions(t, got, ptDictionary[30:], "в словаре осталось три слова")
}

// Тэги хэштегов из text не подсказываются и без exclude: они уже в посте.
// Хэштегом считается то же, что в подписи поста (требования 3–5, 15).
func TestPostTagsSuggestionsSkipTextHashtags(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	viewer := newPTUser(t, baseURL, 2)

	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "Груши поспели #груша"),
		ptEmptyCommunity, "хэштег найденного в тексте тэга")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "#поделюсь #советы"),
		[]string{"вопрос", "дневник", "урожай", "рассада", "теплица"}, "хэштеги двух слов словаря")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "Грядка ##ПОДЕЛЮСЬ, #Советы!"),
		[]string{"вопрос", "дневник", "урожай", "рассада", "теплица"}, "хэштеги разбираются как в подписи")

	// «яблоки#советы» — не хэштег: «советы» — просто слово текста, оно
	// находится в тексте и идёт первым.
	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "яблоки#советы"),
		[]string{"советы", "поделюсь", "вопрос", "дневник", "урожай"}, "решётка внутри слова — не хэштег")

	// Хэштег после десятого — не тэг поста, поэтому подсказывается: слово
	// «дневник» найдено в тексте.
	tenth := ptCaption(ptManyTags(10)...) + " #дневник"
	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, tenth),
		[]string{"дневник", "поделюсь", "советы", "вопрос", "урожай"}, "одиннадцатый хэштег")

	ptTagged(t, baseURL, author.token, 2, "", "альфа")
	ptTagged(t, baseURL, author.token, 1, "", "бета")
	ptRequireSuggestions(t, ptSuggest(t, baseURL, viewer.token, "Грядка #Альфа"),
		[]string{"бета", "поделюсь", "советы", "вопрос", "дневник"}, "хэштег тэга сообщества")
}

// prefix сужает ответ до тэгов, начинающихся с него, порядок прежний;
// нормализуется как tag ленты; тэг, равный ему, тоже подходит
// (сценарий, шаг 4; требование 15).
func TestPostTagsSuggestionsPrefix(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	viewer := newPTUser(t, baseURL, 2)

	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "", "по"),
		[]string{"поделюсь", "полив"}, "словарь на «по»")
	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "", " ##ПО "),
		[]string{"поделюсь", "полив"}, "prefix нормализуется")
	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "", "с"),
		[]string{"советы", "свёкла", "смородина", "слива"}, "словарь на «с» — в порядке словаря")
	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "", "щщ"),
		[]string{}, "ничего на «щщ»")

	ptTagged(t, baseURL, author.token, 2, "", "сорт")
	ptTagged(t, baseURL, author.token, 1, "", "сортовые")
	ptTagged(t, baseURL, author.token, 3, "", "грунт")

	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "", "со"),
		[]string{"сорт", "сортовые", "советы"}, "сообщество по популярности, потом словарь")
	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "", "сорт"),
		[]string{"сорт", "сортовые"}, "равный prefix тэг подходит")
	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "Груши мелкие", "гр"),
		[]string{"груша", "грунт"}, "найденный в тексте — первым и с prefix")
	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "", "сорт", "сорт"),
		[]string{"сортовые"}, "exclude действует и с prefix")
}

// Тэг, равный непустому prefix, не исключается по хэштегам text:
// набираемое «#сорт» — хэштег подписи, но подсказку «сорт» человек
// получает. Другие хэштеги text исключаются как обычно (требование 15).
func TestPostTagsSuggestionsPrefixNotExcludedByTextHashtag(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	viewer := newPTUser(t, baseURL, 2)

	ptTagged(t, baseURL, author.token, 2, "", "сорт")
	ptTagged(t, baseURL, author.token, 1, "", "сортовые")

	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "Груши #груша #сорт", "сорт"),
		[]string{"сорт", "сортовые"}, "набирается «#сорт»")
	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "Груши #сорт #со", "со"),
		[]string{"сортовые", "советы"}, "набирается «#со», а «#сорт» уже в подписи")
	ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "#поделюсь #по", "по"),
		[]string{"полив"}, "хэштег словарного тэга исключается и с prefix")
}

// Неподходящий prefix и пустой после нормализации — как без параметра
// (требование 15).
func TestPostTagsSuggestionsBadPrefixIgnored(t *testing.T) {
	baseURL := startAPI(t)
	viewer := newPTUser(t, baseURL, 1)

	for _, prefix := range []string{"#", " ## ", "зелёный лук", "---", strings.Repeat("я", 31), "груш!"} {
		ptRequireSuggestions(t, ptSuggestParams(t, baseURL, viewer.token, url.Values{"prefix": {prefix}}),
			ptEmptyCommunity, fmt.Sprintf("prefix=%q без текста", prefix))
		ptRequireSuggestions(t, ptSuggestPrefix(t, baseURL, viewer.token, "Груши поспели #сорт", prefix),
			[]string{"груша", "поделюсь", "советы", "вопрос", "дневник"}, fmt.Sprintf("prefix=%q с текстом", prefix))
	}

	ptRequireSuggestions(t, ptSuggestParams(t, baseURL, viewer.token, url.Values{"prefix": {""}}),
		ptEmptyCommunity, "пустой prefix")
}
