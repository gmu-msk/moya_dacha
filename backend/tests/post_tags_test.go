package tests

// Тесты по specs/028-post-tags.md, требования 1–19: тэги у поста, правка
// тэгов, лента по тэгу и подсказки тэгов. Написаны по спецификации и
// контракту, без взгляда на реализацию (ADR-0002).
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
// тела сверх media_ids (tags, caption, visibility, place_id).
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

// ptPublish публикует пост с этими тэгами (nil — поля tags нет) и
// видимостью (пустая — поля нет), требует 201.
func ptPublish(t *testing.T, baseURL, token string, tags []string, visibility string) ptPost {
	t.Helper()

	extra := map[string]any{"caption": "Грядка"}
	if tags != nil {
		extra["tags"] = tags
	}
	if visibility != "" {
		extra["visibility"] = visibility
	}

	resp, _ := ptPublishRaw(t, baseURL, token, extra)

	return ptPostOK(t, resp, http.StatusCreated, fmt.Sprintf("публикация поста с тэгами %q", tags))
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

// ptGet открывает пост по адресу и требует 200.
func ptGet(t *testing.T, baseURL, token, postID string) ptPost {
	t.Helper()
	return ptPostOK(t, fetchPost(t, baseURL, token, postID), http.StatusOK, "GET /posts/"+postID)
}

// ptSetTagsReq — PUT /posts/{id}/tags с телом как есть.
func ptSetTagsReq(t *testing.T, baseURL, token, postID string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/posts/"+postID+"/tags", token, body)
}

// ptSetTags меняет тэги своего поста и требует 200 с тем же постом.
func ptSetTags(t *testing.T, baseURL, token, postID string, tags []string) ptPost {
	t.Helper()

	resp := ptSetTagsReq(t, baseURL, token, postID, map[string]any{"tags": tags})
	post := ptPostOK(t, resp, http.StatusOK, fmt.Sprintf("PUT tags %q", tags))
	if post.ID != postID {
		t.Fatalf("правка тэгов поста %s вернула пост %s", postID, post.ID)
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

// ptSuggest — подсказки для текста text (пустой — параметра нет) без
// exclude перечисленных тэгов; требует 200 и массив items.
func ptSuggest(t *testing.T, baseURL, token, text string, exclude ...string) []string {
	t.Helper()

	params := url.Values{}
	if text != "" {
		params.Set("text", text)
	}
	if len(exclude) > 0 {
		params["exclude"] = exclude
	}

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

// ptRequireSuggestions требует ровно эти подсказки в этом порядке.
func ptRequireSuggestions(t *testing.T, got, want []string, where string) {
	t.Helper()

	if !slices.Equal(got, want) && !(len(got) == 0 && len(want) == 0) {
		t.Errorf("%s: подсказки %q, ожидалось %q", where, got, want)
	}
}

// ptTagged публикует count постов с этими тэгами.
func ptTagged(t *testing.T, baseURL, token string, count int, visibility string, tags ...string) {
	t.Helper()

	for i := 0; i < count; i++ {
		ptPublish(t, baseURL, token, tags, visibility)
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

// --- Тэг: нормализация при публикации (требования 1–4, 6, 7) ---------------

// Регистр, `#` в начале (сколько угодно), пробелы по краям; «ё» остаётся
// «ё» (требование 2).
func TestPostTagsNormalizedOnCreate(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	cases := []struct {
		name string
		tags []string
		want []string
	}{
		{"регистр", []string{"Груша", "ПОДЕЛЮСЬ", "СоРт"}, []string{"груша", "поделюсь", "сорт"}},
		{"решётки в начале", []string{"#груша", "##Сорт", "###урожай"}, []string{"груша", "сорт", "урожай"}},
		{"пробелы по краям", []string{" груша ", "  сорт", "  #Урожай  "}, []string{"груша", "сорт", "урожай"}},
		{"ё не заменяется", []string{"Свёкла", "ЁЛКА"}, []string{"свёкла", "ёлка"}},
		{"цифры, дефис, подчёркивание, латиница, другие алфавиты",
			[]string{"Сорт-2", "f1_гибрид", "2026", "Pear", "მსხალი"},
			[]string{"сорт-2", "f1_гибрид", "2026", "pear", "მსხალი"}},
		{"ровно 30 знаков", []string{strings.Repeat("Я", 30)}, []string{strings.Repeat("я", 30)}},
		{"буква внутри дефисов", []string{"-а-", "_1_"}, []string{"-а-", "_1_"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			created := ptPublish(t, baseURL, author.token, c.tags, "")
			ptRequireTags(t, created, c.want, "ответ на публикацию")
			ptRequireTags(t, ptGet(t, baseURL, author.token, created.ID), c.want, "GET /posts/{id}")
		})
	}
}

// Пустой после нормализации тэг пропускается молча; повторы схлопываются,
// порядок — по первому появлению (требования 3, 4).
func TestPostTagsEmptySkippedAndDuplicatesCollapsed(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	cases := []struct {
		name string
		tags []string
		want []string
	}{
		{"пустые", []string{"", "груша", "   ", "#", "##", " # "}, []string{"груша"}},
		{"только пустые", []string{"", " ", "#"}, []string{}},
		{"повторы", []string{"груша", "сорт", "#Груша", "СОРТ", " груша ", "урожай"}, []string{"груша", "сорт", "урожай"}},
		{"порядок первого появления", []string{"урожай", "груша", "Урожай"}, []string{"урожай", "груша"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			created := ptPublish(t, baseURL, author.token, c.tags, "")
			ptRequireTags(t, created, c.want, "ответ на публикацию")
			ptRequireTags(t, ptGet(t, baseURL, author.token, created.ID), c.want, "GET /posts/{id}")
		})
	}
}

// Пост без поля tags и с tags: null — без тэгов, и в ответах у него
// пустой массив, а не null и не отсутствие поля (требования 6, 7).
func TestPostTagsAbsentMeansEmptyArrayEverywhere(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	withoutField := ptPublish(t, baseURL, author.token, nil, "")
	resp, _ := ptPublishRaw(t, baseURL, author.token, map[string]any{"tags": nil})
	withNull := ptPostOK(t, resp, http.StatusCreated, "публикация с tags: null")
	withEmpty := ptPublish(t, baseURL, author.token, []string{}, "")

	feed := ptFeedOK(t, fetchFeed(t, baseURL, author.token, feedParams(50, "")), "лента")
	profile := ptUserPosts(t, baseURL, author.token, author.id)

	for name, post := range map[string]ptPost{"без поля": withoutField, "null": withNull, "пустой массив": withEmpty} {
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
// автора, в ответах на правку подписи и видимости; в порядке автора
// (требование 7).
func TestPostTagsInEveryPostResponse(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	reader := newPTUser(t, baseURL, 2)

	want := []string{"сорт", "груша", "урожай"}
	created := ptPublish(t, baseURL, author.token, []string{"Сорт", "#груша", "урожай"}, "")
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

	edited := ptPostOK(t, editCaptionText(t, baseURL, author.token, created.ID, "Новая подпись"), http.StatusOK, "правка подписи")
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

	post := ptPublish(t, baseURL, author.token, []string{"груша"}, visibilityFriends)

	ptRequireTags(t, ptGet(t, baseURL, friend.token, post.ID), []string{"груша"}, "друг: GET /posts/{id}")

	feed := ptFeedOK(t, fetchFeed(t, baseURL, friend.token, feedParams(50, "")), "лента друга")
	if item := ptFind(feed, post.ID); item == nil {
		t.Fatal("поста «друзьям» нет в ленте друга")
	} else {
		ptRequireTags(t, *item, []string{"груша"}, "лента друга")
	}
}

// --- Тэг: отказы при публикации (требования 1, 3, 5, 6) --------------------

// Тэг, не подходящий под требование 1, — 400 invalid_tag, пост не создан,
// фотография свободна (требование 3, «Ограничения и edge cases»).
func TestPostTagsInvalidTagRejectsPost(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	cases := []struct {
		name string
		tag  string
	}{
		{"пробел внутри", "зелёный лук"},
		{"31 знак", strings.Repeat("я", 31)},
		{"31 знак после нормализации", " #" + strings.Repeat("Я", 31) + " "},
		{"только дефисы", "---"},
		{"только дефисы и подчёркивания", "_-_"},
		{"решётка после нормализации", "#-#"},
		{"восклицательный знак", "груша!"},
		{"точка", "a.b"},
		{"запятая", "груша,сорт"},
		{"решётка внутри", "груша#сорт"},
		{"эмодзи", "груша🍐"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, photo := ptPublishRaw(t, baseURL, author.token, map[string]any{"tags": []string{"груша", c.tag}})
			requireCodeE(t, resp, http.StatusBadRequest, "invalid_tag", fmt.Sprintf("публикация с тэгом %q", c.tag))

			if n := len(ptUserPosts(t, baseURL, author.token, author.id).Items); n != 0 {
				t.Fatalf("после отказа invalid_tag у автора %d постов, ожидалось 0", n)
			}

			// Пост не создан — та же фотография публикуется.
			again := createPost(t, baseURL, author.token, map[string]any{"media_ids": []string{photo.ID}})
			post := ptPostOK(t, again, http.StatusCreated, "публикация той же фотографии без тэгов")
			requireDeleted(t, deletePost(t, baseURL, author.token, post.ID))
		})
	}
}

// Ровно 10 тэгов после схлопывания — можно; 11 — 400 too_many_tags, пост
// не создан (требование 5).
func TestPostTagsAtMostTen(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	t.Run("ровно 10", func(t *testing.T) {
		ten := ptManyTags(10)
		post := ptPublish(t, baseURL, author.token, ten, "")
		ptRequireTags(t, post, ten, "10 тэгов")
	})

	t.Run("12 с повторами и пустыми — 10 после схлопывания", func(t *testing.T) {
		ten := ptManyTags(10)
		sent := append(append([]string{}, ten...), "#ТЭГ1", "")
		post := ptPublish(t, baseURL, author.token, sent, "")
		ptRequireTags(t, post, ten, "10 тэгов после схлопывания")
	})

	t.Run("11", func(t *testing.T) {
		before := len(ptUserPosts(t, baseURL, author.token, author.id).Items)

		resp, _ := ptPublishRaw(t, baseURL, author.token, map[string]any{"tags": ptManyTags(11)})
		requireCodeE(t, resp, http.StatusBadRequest, "too_many_tags", "публикация с 11 тэгами")

		if after := len(ptUserPosts(t, baseURL, author.token, author.id).Items); after != before {
			t.Errorf("после отказа too_many_tags постов у автора %d, было %d", after, before)
		}
	})
}

// Проверки тэгов — после подписи и видимости, до места (требование 6).
func TestPostTagsCheckOrderOnCreate(t *testing.T) {
	baseURL, _ := startPlaces(t)
	author := newPTUser(t, baseURL, 1)

	resp, _ := ptPublishRaw(t, baseURL, author.token, map[string]any{
		"caption": strings.Repeat("я", 1001),
		"tags":    []string{"зелёный лук"},
	})
	requireCodeE(t, resp, http.StatusBadRequest, "invalid_caption", "длинная подпись и плохой тэг")

	resp, _ = ptPublishRaw(t, baseURL, author.token, map[string]any{
		"visibility": "соседям",
		"tags":       []string{"зелёный лук"},
	})
	requireCodeE(t, resp, http.StatusBadRequest, "invalid_request", "неизвестная видимость и плохой тэг")

	resp, _ = ptPublishRaw(t, baseURL, author.token, map[string]any{
		"place_id": unknownID,
		"tags":     []string{"зелёный лук"},
	})
	requireCodeE(t, resp, http.StatusBadRequest, "invalid_tag", "плохой тэг и неизвестное место")

	resp, _ = ptPublishRaw(t, baseURL, author.token, map[string]any{
		"place_id": unknownID,
		"tags":     ptManyTags(11),
	})
	requireCodeE(t, resp, http.StatusBadRequest, "too_many_tags", "11 тэгов и неизвестное место")
}

// --- PUT /api/posts/{postId}/tags (требования 9, 10) -----------------------

// Тэги заменяются целиком, с нормализацией; новые видны по адресу, в ленте
// и в постах автора (требования 7, 9).
func TestPostTagsReplacedWhole(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	post := ptPublish(t, baseURL, author.token, []string{"груша", "сорт"}, "")

	updated := ptSetTags(t, baseURL, author.token, post.ID, []string{"Урожай", "#груша", "урожай", ""})
	want := []string{"урожай", "груша"}
	ptRequireTags(t, updated, want, "ответ на правку тэгов")
	if updated.Caption != post.Caption {
		t.Errorf("правка тэгов изменила подпись: %q, было %q", updated.Caption, post.Caption)
	}

	ptRequireTags(t, ptGet(t, baseURL, author.token, post.ID), want, "GET /posts/{id}")

	feed := ptFeedOK(t, fetchFeed(t, baseURL, author.token, feedParams(50, "")), "лента")
	if item := ptFind(feed, post.ID); item == nil {
		t.Fatal("поста нет в ленте")
	} else {
		ptRequireTags(t, *item, want, "лента")
	}

	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, author.token, "сорт", "").Items), nil, "лента по снятому тэгу «сорт»")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, author.token, "урожай", "").Items), []string{post.ID}, "лента по новому тэгу «урожай»")

	// Тэги можно и добавить посту, у которого их не было.
	bare := ptPublish(t, baseURL, author.token, nil, "")
	ptRequireTags(t, ptSetTags(t, baseURL, author.token, bare.ID, []string{"дневник"}), []string{"дневник"}, "тэг посту без тэгов")
}

// Пустой массив снимает все тэги (требование 9).
func TestPostTagsEmptyArrayRemovesAll(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	post := ptPublish(t, baseURL, author.token, []string{"груша", "сорт"}, "")

	ptRequireTags(t, ptSetTags(t, baseURL, author.token, post.ID, []string{}), []string{}, "ответ на снятие тэгов")
	ptRequireTags(t, ptGet(t, baseURL, author.token, post.ID), []string{}, "GET /posts/{id}")
	ptRequireIDs(t, ptIDs(ptTagFeed(t, baseURL, author.token, "груша", "").Items), nil, "лента по снятому тэгу")

	// Массив из одних пустых тэгов — то же самое.
	ptSetTags(t, baseURL, author.token, post.ID, []string{"груша"})
	ptRequireTags(t, ptSetTags(t, baseURL, author.token, post.ID, []string{"", " # "}), []string{}, "массив пустых тэгов")
}

// Правка тэгов не меняет edited_at: ни у неизменённого поста, ни у поста
// с правленой подписью (требование 10).
func TestPostTagsEditDoesNotTouchEditedAt(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	post := ptPublish(t, baseURL, author.token, []string{"груша"}, "")
	requireNotEdited(t, post.EditedAt, "только что опубликованный")

	updated := ptSetTags(t, baseURL, author.token, post.ID, []string{"сорт"})
	requireNotEdited(t, updated.EditedAt, "ответ на правку тэгов неизменённого поста")
	requireNotEdited(t, ptGet(t, baseURL, author.token, post.ID).EditedAt, "GET после правки тэгов")

	edited := ptPostOK(t, editCaptionText(t, baseURL, author.token, post.ID, "Новая подпись"), http.StatusOK, "правка подписи")
	editedAt := requireEditedAt(t, edited.EditedAt, edited.CreatedAt, "после правки подписи")

	again := ptSetTags(t, baseURL, author.token, post.ID, []string{"урожай"})
	if again.EditedAt == nil || *again.EditedAt != editedAt {
		t.Errorf("правка тэгов сдвинула edited_at: %v, было %q", again.EditedAt, editedAt)
	}
	if again.Caption != "Новая подпись" {
		t.Errorf("правка тэгов изменила подпись: %q", again.Caption)
	}
	if got := ptGet(t, baseURL, author.token, post.ID).EditedAt; got == nil || *got != editedAt {
		t.Errorf("GET: правка тэгов сдвинула edited_at: %v, было %q", got, editedAt)
	}
}

// Отказы правки тэгов: тэги поста остаются прежними (требование 9).
func TestPostTagsEditRejections(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)
	stranger := newPTUser(t, baseURL, 2)

	original := []string{"груша", "сорт"}
	post := ptPublish(t, baseURL, author.token, original, "")
	hidden := ptPublish(t, baseURL, author.token, original, visibilityMe)
	forFriends := ptPublish(t, baseURL, author.token, original, visibilityFriends)

	requireStays := func(t *testing.T, postID, where string) {
		t.Helper()
		ptRequireTags(t, ptGet(t, baseURL, author.token, postID), original, where+": тэги после отказа")
	}

	t.Run("без токена", func(t *testing.T) {
		resp := ptSetTagsReq(t, baseURL, "", post.ID, map[string]any{"tags": []string{"урожай"}})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("без токена: ожидался статус 401, получен %d", resp.StatusCode)
		}
		requireStays(t, post.ID, "без токена")
	})

	t.Run("чужой пост", func(t *testing.T) {
		resp := ptSetTagsReq(t, baseURL, stranger.token, post.ID, map[string]any{"tags": []string{"урожай"}})
		requireCodeE(t, resp, http.StatusForbidden, "not_your_post", "чужой пост")
		requireStays(t, post.ID, "чужой пост")
	})

	t.Run("чужой пост и плохие тэги — сначала «своё ли»", func(t *testing.T) {
		resp := ptSetTagsReq(t, baseURL, stranger.token, post.ID, map[string]any{"tags": []string{"зелёный лук"}})
		requireCodeE(t, resp, http.StatusForbidden, "not_your_post", "чужой пост с плохим тэгом")
		resp = ptSetTagsReq(t, baseURL, stranger.token, post.ID, map[string]any{})
		requireCodeE(t, resp, http.StatusForbidden, "not_your_post", "чужой пост без tags")
		requireStays(t, post.ID, "чужой пост с плохим телом")
	})

	notFound := []struct {
		name   string
		postID string
	}{
		{"несуществующий", unknownID},
		{"не UUID", notAnID},
		{"чужой «только мне»", hidden.ID},
		{"чужой «друзьям», не друг", forFriends.ID},
	}
	for _, c := range notFound {
		t.Run("404 "+c.name, func(t *testing.T) {
			resp := ptSetTagsReq(t, baseURL, stranger.token, c.postID, map[string]any{"tags": []string{"урожай"}})
			requireCodeE(t, resp, http.StatusNotFound, "post_not_found", c.name)
		})
	}
	requireStays(t, hidden.ID, "«только мне»")
	requireStays(t, forFriends.ID, "«друзьям»")

	invalidRequest := []struct {
		name string
		body string
	}{
		{"пустой объект", `{}`},
		{"без tags", `{"caption": "груша"}`},
		{"не JSON", `{"tags": [`},
		{"пустое тело", ``},
	}
	for _, c := range invalidRequest {
		t.Run("invalid_request "+c.name, func(t *testing.T) {
			resp := ebdRaw(t, http.MethodPut, baseURL+"/posts/"+post.ID+"/tags", author.token, c.body)
			requireCodeE(t, resp, http.StatusBadRequest, "invalid_request", c.name)
			requireStays(t, post.ID, c.name)
		})
	}

	invalidTags := []struct {
		name string
		tags []string
		code string
	}{
		{"пробел внутри", []string{"урожай", "зелёный лук"}, "invalid_tag"},
		{"31 знак", []string{strings.Repeat("я", 31)}, "invalid_tag"},
		{"только дефисы", []string{"---"}, "invalid_tag"},
		{"недопустимый знак", []string{"груша!"}, "invalid_tag"},
		{"11 тэгов", ptManyTags(11), "too_many_tags"},
	}
	for _, c := range invalidTags {
		t.Run(c.code+" "+c.name, func(t *testing.T) {
			resp := ptSetTagsReq(t, baseURL, author.token, post.ID, map[string]any{"tags": c.tags})
			requireCodeE(t, resp, http.StatusBadRequest, c.code, c.name)
			requireStays(t, post.ID, c.name)
		})
	}

	t.Run("ровно 10 — можно", func(t *testing.T) {
		ten := ptManyTags(10)
		ptRequireTags(t, ptSetTags(t, baseURL, author.token, post.ID, append(append([]string{}, ten...), "ТЭГ10")), ten, "10 тэгов")
	})
}

// --- Удаление поста (требование 11) ----------------------------------------

// Удалённый пост уходит вместе с тэгами: его нет в ленте по тэгу, тэг не
// подсказывается как тэг сообщества, строк в post_tags не осталось.
func TestPostTagsGoAwayWithPost(t *testing.T) {
	baseURL := startAPI(t)
	author := newPTUser(t, baseURL, 1)

	post := ptPublish(t, baseURL, author.token, []string{"альфа", "бета"}, "")
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

	pear := ptPublish(t, baseURL, author.token, []string{"груша"}, "")
	ptPublish(t, baseURL, author.token, nil, "")
	ptPublish(t, baseURL, author.token, []string{"груши"}, "")
	ptPublish(t, baseURL, author.token, []string{"грушевый"}, "")
	pearAndSort := ptPublish(t, baseURL, reader.token, []string{"сорт", "груша"}, "")
	ptPublish(t, baseURL, author.token, []string{"свёкла"}, "")

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

	pear := ptPublish(t, baseURL, author.token, []string{"груша"}, "")
	bare := ptPublish(t, baseURL, author.token, nil, "")
	other := ptPublish(t, baseURL, author.token, []string{"сорт"}, "")

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

	public := ptPublish(t, baseURL, author.token, []string{"груша"}, "")
	forFriends := ptPublish(t, baseURL, author.token, []string{"груша"}, visibilityFriends)
	onlyMe := ptPublish(t, baseURL, author.token, []string{"груша"}, visibilityMe)
	closedPost := ptPublish(t, baseURL, closed.token, []string{"груша"}, "")
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

	fromFollowed := ptPublish(t, baseURL, followed.token, []string{"груша"}, "")
	ptPublish(t, baseURL, followed.token, []string{"сорт"}, "")
	fromOther := ptPublish(t, baseURL, other.token, []string{"груша"}, "")
	own := ptPublish(t, baseURL, viewer.token, []string{"груша"}, "")

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
		tagged = append(tagged, ptPublish(t, baseURL, author.token, []string{"груша"}, "").ID)
		ptPublish(t, baseURL, author.token, []string{"сорт"}, "")
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
