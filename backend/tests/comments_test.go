package tests

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Текст комментария из контракта: им комментируют там, где сам текст
// значения не имеет (specs/openapi.yaml, schema Comment).
const commentText = "И у нас такая же напасть, брызгали содой"

// Предел длины комментария — тот же, что у подписи поста, и в тех же
// единицах: символы, а не байты (ФТ-4, «Ограничения и edge cases»).
const commentLimit = 1000

// --- Представления из контракта -------------------------------------------

// commentPayload — комментарий целиком (schema Comment). Ни лайков,
// ни ссылки на другой комментарий в нём нет и быть не может (ФТ-2).
type commentPayload struct {
	ID        string        `json:"id"`
	CreatedAt string        `json:"created_at"`
	Text      string        `json:"text"`
	Author    authorPayload `json:"author"`
}

// commentsPayload — комментарии одного поста (schema Comments). Курсора
// здесь нет: разговор приходит целиком (ФТ-6).
type commentsPayload struct {
	Items []commentPayload `json:"items"`
}

// commentedPostPayload — пост с числом комментариев (schema Post, поле
// `comments`). Отдельный тип, а не расширенный postPayload: остальные
// фичи про комментарии не знают, а здесь важно, что поле приходит
// вместе с постом (ФТ-7).
type commentedPostPayload struct {
	postPayload
	Comments int `json:"comments"`
}

// commentedFeedPayload — страница ленты, разобранная с числом
// комментариев: оно приходит у каждого поста ленты («Число комментариев
// в ленте»).
type commentedFeedPayload struct {
	Items []commentedPostPayload `json:"items"`
}

// --- Хелперы --------------------------------------------------------------

// fetchComments читает комментарии поста.
func fetchComments(t *testing.T, baseURL, token, postID string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, baseURL+"/posts/"+postID+"/comments", token, nil)
}

// addComment оставляет комментарий. body передаётся как есть: тестам про
// непригодное тело нужен не объект, а что угодно.
func addComment(t *testing.T, baseURL, token, postID string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, baseURL+"/posts/"+postID+"/comments", token, body)
}

// addCommentText оставляет комментарий с этим текстом — так это делает
// приложение.
func addCommentText(t *testing.T, baseURL, token, postID, text string) *http.Response {
	t.Helper()
	return addComment(t, baseURL, token, postID, map[string]any{"text": text})
}

// addCommentRaw отправляет тело запроса как есть, не собирая его из
// объекта: телу, которое вовсе не JSON, объекта в Go не соответствует.
func addCommentRaw(t *testing.T, baseURL, token, postID, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, baseURL+"/posts/"+postID+"/comments", strings.NewReader(body))
	if err != nil {
		t.Fatalf("не удалось собрать запрос на комментарий: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("запрос на комментарий не прошёл: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// commentOK требует ожидаемого статуса и возвращает разобранный
// комментарий.
func commentOK(t *testing.T, resp *http.Response, wantStatus int) commentPayload {
	t.Helper()

	if resp.StatusCode != wantStatus {
		t.Fatalf("ожидался статус %d, получен %d", wantStatus, resp.StatusCode)
	}

	var body commentPayload
	decode(t, resp, &body)

	return body
}

// createdComment требует, чтобы комментарий был оставлен, и возвращает
// его: ответ — созданный комментарий, а не пост целиком («API»).
func createdComment(t *testing.T, resp *http.Response) commentPayload {
	t.Helper()
	return commentOK(t, resp, http.StatusCreated)
}

// commentOf оставляет комментарий с этим текстом и возвращает созданный.
func commentOf(t *testing.T, baseURL, token, postID, text string) commentPayload {
	t.Helper()
	return createdComment(t, addCommentText(t, baseURL, token, postID, text))
}

// commentsOf читает комментарии поста и требует, чтобы они прочитались.
func commentsOf(t *testing.T, baseURL, token, postID string) commentsPayload {
	t.Helper()

	resp := fetchComments(t, baseURL, token, postID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на чтение комментариев ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body commentsPayload
	decode(t, resp, &body)

	return body
}

// commentIDs — идентификаторы комментариев в том порядке, в котором они
// пришли.
func commentIDs(items []commentPayload) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	return ids
}

// commentTexts — тексты комментариев в том порядке, в котором они пришли.
func commentTexts(items []commentPayload) []string {
	texts := make([]string, 0, len(items))
	for _, item := range items {
		texts = append(texts, item.Text)
	}

	return texts
}

// requireCommentTexts требует, чтобы под постом стояли ровно эти
// комментарии и ровно в этом порядке.
func requireCommentTexts(t *testing.T, baseURL, token, postID string, want []string, where string) {
	t.Helper()

	got := commentTexts(commentsOf(t, baseURL, token, postID).Items)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: ожидались комментарии %v, получены %v", where, want, got)
	}
}

// commentedPostOK требует ожидаемого статуса и возвращает пост с числом
// комментариев.
func commentedPostOK(t *testing.T, resp *http.Response, wantStatus int) commentedPostPayload {
	t.Helper()

	if resp.StatusCode != wantStatus {
		t.Fatalf("ожидался статус %d, получен %d", wantStatus, resp.StatusCode)
	}

	var body commentedPostPayload
	decode(t, resp, &body)

	return body
}

// postToComment публикует пост с одной фотографией: комментариям всё
// равно, что на ней, важен только сам пост.
func postToComment(t *testing.T, baseURL, token string) commentedPostPayload {
	t.Helper()

	photo := photoOf(t, baseURL, token, 60, 40)

	return commentedPostOK(t, createPost(t, baseURL, token,
		map[string]any{"media_ids": []string{photo.ID}, "caption": postCaption}), http.StatusCreated)
}

// commentedPostPage открывает пост по его адресу и возвращает его
// с числом комментариев.
func commentedPostPage(t *testing.T, baseURL, token, postID string) commentedPostPayload {
	t.Helper()
	return commentedPostOK(t, fetchPost(t, baseURL, token, postID), http.StatusOK)
}

// commentedFeedItem находит пост в ленте и возвращает его: число
// комментариев приходит у каждого поста ленты.
func commentedFeedItem(t *testing.T, baseURL, token, postID string) commentedPostPayload {
	t.Helper()

	resp := fetchFeed(t, baseURL, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на страницу ленты ожидался статус 200, получен %d", resp.StatusCode)
	}

	var page commentedFeedPayload
	decode(t, resp, &page)

	for _, item := range page.Items {
		if item.ID == postID {
			return item
		}
	}

	t.Fatalf("поста %s нет в ленте, а он опубликован", postID)

	return commentedPostPayload{}
}

// requireCommentCount требует у поста ровно этого числа комментариев.
func requireCommentCount(t *testing.T, post commentedPostPayload, want int, where string) {
	t.Helper()

	if post.Comments != want {
		t.Errorf("%s: ожидалось комментариев %d, получено %d", where, want, post.Comments)
	}
}

// requireCommentCountEverywhere требует одного и того же числа всюду,
// где приходит пост: на экране поста и в ленте (ФТ-7).
func requireCommentCountEverywhere(t *testing.T, baseURL, token, postID string, want int, who string) {
	t.Helper()

	requireCommentCount(t, commentedPostPage(t, baseURL, token, postID), want, who+": пост по своему адресу")
	requireCommentCount(t, commentedFeedItem(t, baseURL, token, postID), want, who+": пост в ленте")
}

// requireCommentsField требует, чтобы поле было в ответе на самом деле:
// пост без комментариев приходит с "comments": 0, а не без поля
// («Пост в ответе»).
func requireCommentsField(t *testing.T, raw []byte, where string) {
	t.Helper()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON: %v", where, err)
	}

	if _, ok := fields["comments"]; !ok {
		t.Errorf("%s: в посте нет поля \"comments\", а оно обязательное", where)
	}
}

// requireCommentShape требует, чтобы комментарий состоял ровно из полей
// контракта: комментарий — это текст и ничего больше, лишнему полю
// (ответу на другой комментарий, лайкам, фотографиям) в нём места нет
// (ФТ-2, schema Comment).
func requireCommentShape(t *testing.T, raw []byte, where string) {
	t.Helper()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON: %v", where, err)
	}

	expected := map[string]bool{"id": true, "created_at": true, "text": true, "author": true}

	for field := range expected {
		if _, ok := fields[field]; !ok {
			t.Errorf("%s: в комментарии нет обязательного поля %q", where, field)
		}
	}
	for field := range fields {
		if !expected[field] {
			t.Errorf("%s: в комментарии лишнее поле %q — комментарий это текст и ничего больше", where, field)
		}
	}
}

// setSameCommentCreatedAt ставит всем перечисленным комментариям одно
// и то же время: сервис такого не сделает, а порядок разговора обязан
// остаться устойчивым («Модель данных»).
func setSameCommentCreatedAt(t *testing.T, ids []string) {
	t.Helper()

	// Микросекунды — предел точности timestamptz: без обрезки время,
	// записанное из Go, совпало бы не полностью.
	at := time.Now().UTC().Truncate(time.Microsecond)

	for _, id := range ids {
		if affected := execSQL(t, `UPDATE comments SET created_at = $1 WHERE id = $2`, at, id); affected != 1 {
			t.Fatalf("время комментария %s не проставилось: изменено строк %d", id, affected)
		}
	}
}

// repeatRunes собирает текст ровно из count символов: длина считается
// в символах, поэтому строится она тоже в них («Длина считается
// в символах»).
func repeatRunes(runes string, count int) string {
	var builder strings.Builder

	source := []rune(runes)
	for i := 0; i < count; i++ {
		builder.WriteRune(source[i%len(source)])
	}

	return builder.String()
}

// --- POST /api/posts/{postId}/comments: оставить комментарий --------------

// Комментарий к посту, у которого комментариев не было: 201, у поста
// стало `comments: 1`, и сам он появился под постом («Комментарий
// к посту без комментариев», пользовательский сценарий, шаги 2-3).
func TestCommentOnPostWithoutCommentsMakesItOne(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	post := postToComment(t, baseURL, token)
	requireCommentCount(t, post, 0, "только что опубликованный пост")

	comment := createdComment(t, addCommentText(t, baseURL, token, post.ID, commentText))

	if comment.Text != commentText {
		t.Errorf("ожидался текст %q, получен %q", commentText, comment.Text)
	}
	if !uuidPattern.MatchString(comment.ID) {
		t.Errorf("идентификатор комментария %q не похож на UUID", comment.ID)
	}
	if comment.CreatedAt == "" {
		t.Error("у комментария нет времени, а оно приходит рядом с именем")
	}
	if comment.Author.ID != userID {
		t.Errorf("автором комментария ожидался %s, получен %s", userID, comment.Author.ID)
	}

	requireCommentTexts(t, baseURL, token, post.ID, []string{commentText}, "пост с одним комментарием")
	requireCommentCountEverywhere(t, baseURL, token, post.ID, 1, "прокомментировавший")
}

// Второй комментарий того же человека под тем же постом — это
// нормальный разговор: 201 и оба комментария на месте («Второй
// комментарий того же человека»).
func TestSecondCommentOfTheSamePersonIsAccepted(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	post := postToComment(t, baseURL, token)

	first := commentOf(t, baseURL, token, post.ID, "Чем брызгали?")
	second := commentOf(t, baseURL, token, post.ID, "А, вижу, содой")
	third := commentOf(t, baseURL, token, post.ID, "Спасибо!")

	if first.ID == second.ID || second.ID == third.ID || first.ID == third.ID {
		t.Error("каждая реплика — свой комментарий, а идентификаторы совпали")
	}

	requireCommentTexts(t, baseURL, token, post.ID,
		[]string{"Чем брызгали?", "А, вижу, содой", "Спасибо!"}, "три реплики одного человека")
	requireCommentCountEverywhere(t, baseURL, token, post.ID, 3, "написавший три реплики")
}

// Комментировать можно и свой пост: лента одна на всех, и запрет ничего
// не бережёт («Комментарий к своему посту», ФТ-1).
func TestCommentOnOwnPostIsAllowed(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	post := postToComment(t, baseURL, token)
	if post.Author.ID != userID {
		t.Fatalf("пост должен быть своим: ожидался автор %s, получен %s", userID, post.Author.ID)
	}

	comment := commentOf(t, baseURL, token, post.ID, commentText)

	if comment.Author.ID != userID {
		t.Errorf("автором комментария ожидался %s, получен %s", userID, comment.Author.ID)
	}
	requireCommentCountEverywhere(t, baseURL, token, post.ID, 1, "автор поста, прокомментировавший себя")
}

// Комментарий чужого человека виден всем: и автору поста, и тому, кто
// просто смотрит («Комментарий чужого человека», ФТ-12).
func TestCommentOfAnotherPersonIsVisibleToEveryone(t *testing.T) {
	baseURL := startAPI(t)

	authorToken, authorID := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, authorToken, profileName)

	guestToken, guestID := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, guestToken, "Мария")

	strangerToken, _ := signIn(t, baseURL, thirdPhonePretty)

	post := postToComment(t, baseURL, authorToken)

	comment := commentOf(t, baseURL, guestToken, post.ID, commentText)
	if comment.Author.ID != guestID {
		t.Errorf("автором комментария ожидался %s, получен %s", guestID, comment.Author.ID)
	}
	if comment.Author.ID == authorID {
		t.Error("комментарий приписан автору поста, а писал его другой человек")
	}

	for who, token := range map[string]string{
		"автор поста":            authorToken,
		"написавший комментарий": guestToken,
		"посторонний":            strangerToken,
	} {
		requireCommentTexts(t, baseURL, token, post.ID, []string{commentText}, who)
		requireCommentCountEverywhere(t, baseURL, token, post.ID, 1, who)
	}
}

// --- Текст комментария ----------------------------------------------------

// Пустая строка в тексте — 400 empty_comment: под постом нечего
// показывать («Пустая строка в `text`», ФТ-3).
func TestEmptyCommentTextIsRejected(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	requireError(t, addCommentText(t, baseURL, token, post.ID, ""), http.StatusBadRequest, "empty_comment")

	requireCommentTexts(t, baseURL, token, post.ID, []string{}, "после отвергнутого пустого комментария")
	requireCommentCountEverywhere(t, baseURL, token, post.ID, 0, "после отвергнутого пустого комментария")
}

// Текст из одних пробелов, табуляций и переводов строки — тоже пустой
// комментарий: после обрезки краёв от него ничего не остаётся («Текст
// из одних пробелов и переводов строки», ФТ-3).
func TestCommentOfOnlySpacesAndLineBreaksIsRejected(t *testing.T) {
	texts := map[string]string{
		"один пробел":        " ",
		"несколько пробелов": "     ",
		"переводы строки":    "\n\n\n",
		"пробелы и переводы": "  \n \t \r\n  ",
		"табуляции":          "\t\t",
	}

	for caseName, text := range texts {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)
			post := postToComment(t, baseURL, token)

			requireError(t, addCommentText(t, baseURL, token, post.ID, text), http.StatusBadRequest, "empty_comment")
			requireCommentCount(t, commentedPostPage(t, baseURL, token, post.ID), 0, "после отвергнутого комментария")
		})
	}
}

// Текста в теле нет вовсе — запрос понятен, а текста в нём нет:
// 400 empty_comment, а не invalid_request («`text` отсутствует в теле»).
func TestCommentWithoutTextFieldIsEmptyComment(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	requireError(t, addComment(t, baseURL, token, post.ID, map[string]any{}),
		http.StatusBadRequest, "empty_comment")

	requireCommentCount(t, commentedPostPage(t, baseURL, token, post.ID), 0, "после тела без текста")
}

// Пробелы по краям обрезаются, и сохраняется обрезанный текст — так же,
// как у подписи поста («Пробелы по краям текста», ФТ-3).
func TestSpacesAroundCommentTextAreTrimmed(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	comment := commentOf(t, baseURL, token, post.ID, "  \n\t "+commentText+" \n  ")

	if comment.Text != commentText {
		t.Errorf("ожидался обрезанный текст %q, получен %q", commentText, comment.Text)
	}
	requireCommentTexts(t, baseURL, token, post.ID, []string{commentText}, "комментарий с пробелами по краям")
}

// Переводы строки внутри текста сохраняются как есть: обрезаются только
// края («Переводы строки внутри текста»).
func TestLineBreaksInsideCommentTextAreKept(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	text := "Брызгали содой:\n\nстакан на ведро,\nраз в неделю."

	comment := commentOf(t, baseURL, token, post.ID, " "+text+" ")

	if comment.Text != text {
		t.Errorf("ожидался текст %q, получен %q", text, comment.Text)
	}
	requireCommentTexts(t, baseURL, token, post.ID, []string{text}, "комментарий с переводами строки")
}

// Ровно 1000 символов после обрезки — 201: предел включительный, как
// у подписи поста («Ровно 1000 символов после обрезки», ФТ-4).
func TestCommentOfExactlyOneThousandCharactersIsAccepted(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	text := repeatRunes("абвгдеёжзи", commentLimit)

	comment := commentOf(t, baseURL, token, post.ID, "   "+text+"   ")

	if got := utf8.RuneCountInString(comment.Text); got != commentLimit {
		t.Fatalf("ожидался текст в %d символов, получено %d", commentLimit, got)
	}
	if comment.Text != text {
		t.Error("текст в 1000 символов сохранился не таким, каким его прислали")
	}
	requireCommentCount(t, commentedPostPage(t, baseURL, token, post.ID), 1, "пост с длинным комментарием")
}

// 1001 символ после обрезки — 400 invalid_comment. Пробелы по краям тут
// же и показывают, что длина считается после обрезки, а не до неё
// («1001 символ после обрезки»).
func TestCommentLongerThanOneThousandCharactersIsRejected(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	text := repeatRunes("абвгдеёжзи", commentLimit+1)

	requireError(t, addCommentText(t, baseURL, token, post.ID, text),
		http.StatusBadRequest, "invalid_comment")
	requireError(t, addCommentText(t, baseURL, token, post.ID, "   "+text+"   "),
		http.StatusBadRequest, "invalid_comment")

	// А ровно 1000 символов с теми же пробелами по краям принимаются:
	// края не участвуют в длине.
	commentOf(t, baseURL, token, post.ID, "   "+repeatRunes("абвгдеёжзи", commentLimit)+"   ")

	requireCommentCount(t, commentedPostPage(t, baseURL, token, post.ID), 1, "пост после одного принятого комментария")
}

// Длина считается в символах, а не в байтах: 1000 эмодзи — это 1000
// символов и 4000 байт, и они принимаются, а 1001 — уже нет («Длина
// считается в символах»).
func TestCommentLengthIsCountedInCharactersNotBytes(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	accepted := map[string]string{
		"эмодзи":              repeatRunes("🍓", commentLimit),
		"буквы с диакритикой": repeatRunes("éöñ", commentLimit),
		"кириллица":           repeatRunes("клубника", commentLimit),
	}

	for caseName, text := range accepted {
		t.Run(caseName+": 1000 символов принимаются", func(t *testing.T) {
			if utf8.RuneCountInString(text) != commentLimit {
				t.Fatalf("тест собран неправильно: в тексте %d символов", utf8.RuneCountInString(text))
			}

			comment := commentOf(t, baseURL, token, post.ID, text)

			if got := utf8.RuneCountInString(comment.Text); got != commentLimit {
				t.Errorf("ожидалось %d символов, получено %d", commentLimit, got)
			}
			if comment.Text != text {
				t.Error("текст сохранился не таким, каким его прислали")
			}
		})
	}

	// Тот же текст длиной на один символ больше не принимается — и дело
	// именно в символах: байт в нём заведомо больше тысячи и у принятого.
	tooLong := repeatRunes("🍓", commentLimit+1)
	if len(tooLong) <= commentLimit {
		t.Fatalf("тест собран неправильно: в тексте %d байт", len(tooLong))
	}

	requireError(t, addCommentText(t, baseURL, token, post.ID, tooLong),
		http.StatusBadRequest, "invalid_comment")
}

// Тело запроса, которое не разбирается, — 400 invalid_request («Тело
// запроса не JSON»).
func TestCommentRejectsMalformedBody(t *testing.T) {
	bodies := map[string]any{
		"не объект запроса":     "это не объект запроса",
		"текст не строка":       map[string]any{"text": 42},
		"текст — массив":        map[string]any{"text": []string{"а", "б"}},
		"текст вложен в объект": map[string]any{"text": map[string]any{"text": commentText}},
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)
			post := postToComment(t, baseURL, token)

			requireError(t, addComment(t, baseURL, token, post.ID, body),
				http.StatusBadRequest, "invalid_request")
			requireCommentCount(t, commentedPostPage(t, baseURL, token, post.ID), 0, "после непригодного тела")
		})
	}

	// И тело, которое вовсе не JSON, — то же самое.
	raw := map[string]string{
		"не JSON вовсе":   "брызгали содой",
		"оборванный JSON": `{"text": "брызгали содой"`,
		"пустое тело":     "",
	}

	for caseName, body := range raw {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)
			post := postToComment(t, baseURL, token)

			requireError(t, addCommentRaw(t, baseURL, token, post.ID, body),
				http.StatusBadRequest, "invalid_request")
			requireCommentCount(t, commentedPostPage(t, baseURL, token, post.ID), 0, "после непригодного тела")
		})
	}
}

// --- GET /api/posts/{postId}/comments: чтение разговора -------------------

// У поста без комментариев `items` пустой, и это не ошибка: поле есть
// всегда («Пост без комментариев»).
func TestPostWithoutCommentsReturnsEmptyItems(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	resp := fetchComments(t, baseURL, token, post.ID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на чтение комментариев ожидался статус 200, получен %d", resp.StatusCode)
	}

	raw := rawJSON(t, resp)

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("ответ не разобрался как JSON: %v", err)
	}
	if _, ok := fields["items"]; !ok {
		t.Fatal("в ответе нет поля \"items\", а оно есть всегда")
	}

	var body commentsPayload
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("ответ не разобрался как комментарии: %v", err)
	}
	if len(body.Items) != 0 {
		t.Errorf("у поста без комментариев ожидался пустой список, получено %d", len(body.Items))
	}
}

// Комментарии читаются от старого к новому: это разговор, и читается он
// сверху вниз («Порядок комментариев», ФТ-5, пользовательский сценарий,
// шаг 3).
func TestCommentsGoFromOldToNew(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, otherToken, "Мария")

	post := postToComment(t, baseURL, token)

	// Разговор: реплики двух человек вперемешку.
	want := []string{
		"Чем брызгали?",
		"Содой, стакан на ведро",
		"У нас такая же напасть",
		"Дай пару корешков осенью",
		"Приходи, накопаем",
	}
	tokens := []string{otherToken, token, otherToken, otherToken, token}

	for i, text := range want {
		commentOf(t, baseURL, tokens[i], post.ID, text)
	}

	requireCommentTexts(t, baseURL, token, post.ID, want, "разговор под постом")

	// Свежая реплика встаёт последней, а не первой.
	commentOf(t, baseURL, token, post.ID, "Спасибо!")
	requireCommentTexts(t, baseURL, otherToken, post.ID, append(append([]string(nil), want...), "Спасибо!"),
		"разговор с новой репликой")
}

// При одинаковом времени порядок — по идентификатору, тоже по
// возрастанию: одинаковые сотые доли секунды не должны давать разный
// порядок при каждом чтении («Порядок комментариев», ФТ-5).
func TestCommentsWithTheSameTimeAreOrderedById(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	var ids []string
	for _, text := range []string{"первый", "второй", "третий", "четвёртый", "пятый"} {
		ids = append(ids, commentOf(t, baseURL, token, post.ID, text).ID)
	}

	setSameCommentCreatedAt(t, ids)

	// Спека задаёт порядок как (created_at, id) по возрастанию: при
	// равном времени сверху стоит меньший идентификатор.
	want := append([]string(nil), ids...)
	sort.Strings(want)

	// Читается он одинаково сколько угодно раз.
	for attempt := 1; attempt <= 3; attempt++ {
		got := commentIDs(commentsOf(t, baseURL, token, post.ID).Items)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("чтение %d: ожидался порядок %v, получен %v", attempt, want, got)
		}
	}
}

// Комментарии приходят целиком, без страниц: сколько их ни напиши, все
// они в одном ответе, и курсора в нём нет (ФТ-6).
func TestCommentsComeWholeWithoutPages(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	// Больше, чем страница ленты: у ленты страницы есть, у разговора нет.
	const count = feedDefaultLimit + 5

	want := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		text := "Реплика № " + strconv.Itoa(i)
		commentOf(t, baseURL, token, post.ID, text)
		want = append(want, text)
	}

	resp := fetchComments(t, baseURL, token, post.ID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на чтение комментариев ожидался статус 200, получен %d", resp.StatusCode)
	}
	raw := rawJSON(t, resp)

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("ответ не разобрался как JSON: %v", err)
	}
	if _, ok := fields["next_cursor"]; ok {
		t.Error("у комментариев нет страниц, а в ответе есть курсор")
	}

	var body commentsPayload
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("ответ не разобрался как комментарии: %v", err)
	}
	if got := commentTexts(body.Items); !reflect.DeepEqual(got, want) {
		t.Fatalf("ожидались все %d реплик подряд, получено %d: %v", count, len(got), got)
	}

	requireCommentCountEverywhere(t, baseURL, token, post.ID, count, "пост с длинным разговором")
}

// У комментария виден автор — имя и аватар, как у поста, и никогда
// номер телефона. Кроме текста, времени и автора в комментарии нет
// ничего: ни лайков, ни ответов (ФТ-2, ФТ-11, CONTEXT.md).
func TestCommentShowsAuthorWithNameAndAvatarAndNothingElse(t *testing.T) {
	baseURL := startAPI(t)

	authorToken, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, authorToken)

	guestToken, guestID := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, guestToken, "Мария")
	guestAvatar := avatarOf(t, baseURL, guestToken, imageBytes(t, "png", 300, 300))

	created := addCommentText(t, baseURL, guestToken, post.ID, commentText)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("на комментарий ожидался статус 201, получен %d", created.StatusCode)
	}
	createdRaw := rawJSON(t, created)

	listResp := fetchComments(t, baseURL, authorToken, post.ID)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("на чтение комментариев ожидался статус 200, получен %d", listResp.StatusCode)
	}
	listRaw := rawJSON(t, listResp)

	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(listRaw, &list); err != nil {
		t.Fatalf("ответ на чтение комментариев не разобрался как JSON: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("ожидался один комментарий, получено %d", len(list.Items))
	}

	responses := map[string][]byte{
		"ответ на новый комментарий": createdRaw,
		"комментарий в списке":       list.Items[0],
	}

	for name, raw := range responses {
		requireCommentShape(t, raw, name)

		// Номера телефона нет ни в каком виде — ни у автора, ни рядом.
		for _, forbidden := range []string{otherPhoneStored, otherPhonePretty, `"phone"`} {
			if strings.Contains(string(raw), forbidden) {
				t.Errorf("в комментарии не должно быть номера телефона, а %s содержит %q", name, forbidden)
			}
		}

		var comment commentPayload
		if err := json.Unmarshal(raw, &comment); err != nil {
			t.Fatalf("%s не разобрался как комментарий: %v", name, err)
		}
		if comment.Author.ID != guestID {
			t.Errorf("%s: автором ожидался %s, получен %s", name, guestID, comment.Author.ID)
		}
		if comment.Author.Name != "Мария" {
			t.Errorf("%s: ожидалось имя автора %q, получено %q", name, "Мария", comment.Author.Name)
		}
		if comment.Author.AvatarURL == nil || *comment.Author.AvatarURL != guestAvatar {
			t.Errorf("%s: ожидался аватар автора %q, получен %v", name, guestAvatar, comment.Author.AvatarURL)
		}
	}

	// Аватар автора комментария действительно отдаётся сервисом.
	downloadFile(t, baseURL, guestAvatar)
}

// --- Число комментариев у поста -------------------------------------------

// Поле `comments` приходит везде, где приходит пост: в ответе на
// публикацию, на самом посте, в ленте и на лайк. У свежего поста это 0,
// и это именно ноль, а не отсутствующее поле (ФТ-7, «Пост в ответе»).
func TestCommentCountComesWithThePostEverywhere(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	photo := photoOf(t, baseURL, token, 60, 40)

	created := createPost(t, baseURL, token,
		map[string]any{"media_ids": []string{photo.ID}, "caption": postCaption})
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("на публикацию ожидался статус 201, получен %d", created.StatusCode)
	}
	createdRaw := rawJSON(t, created)
	requireCommentsField(t, createdRaw, "ответ на публикацию")

	var post commentedPostPayload
	if err := json.Unmarshal(createdRaw, &post); err != nil {
		t.Fatalf("ответ на публикацию не разобрался как пост: %v", err)
	}
	requireCommentCount(t, post, 0, "ответ на публикацию")

	requireCommentsField(t, rawJSON(t, fetchPost(t, baseURL, token, post.ID)), "пост по своему адресу")
	requireCommentCountEverywhere(t, baseURL, token, post.ID, 0, "пост без комментариев")

	// Ответ на лайк — тоже пост, и число комментариев в нём то же.
	requireCommentCount(t, commentedPostOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK),
		0, "ответ на лайк поста без комментариев")

	commentOf(t, baseURL, token, post.ID, commentText)
	commentOf(t, baseURL, token, post.ID, "И ещё одна реплика")

	requireCommentCountEverywhere(t, baseURL, token, post.ID, 2, "пост с двумя комментариями")
	requireCommentCount(t, commentedPostOK(t, likePost(t, baseURL, token, post.ID), http.StatusOK),
		2, "ответ на лайк поста с комментариями")
	requireCommentCount(t, commentedPostOK(t, unlikePost(t, baseURL, token, post.ID), http.StatusOK),
		2, "ответ на снятие лайка")
}

// Число комментариев приходит у каждого поста ленты: видно, где
// разговор идёт, а где ещё нет («Число комментариев в ленте»,
// пользовательский сценарий, шаг 4).
func TestFeedShowsCommentCountOfEveryPost(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)

	silent := postToComment(t, baseURL, token)
	quiet := postToComment(t, baseURL, token)
	loud := postToComment(t, baseURL, token)

	commentOf(t, baseURL, otherToken, quiet.ID, "Красота")
	commentOf(t, baseURL, otherToken, loud.ID, "Чем брызгали?")
	commentOf(t, baseURL, token, loud.ID, "Содой")
	commentOf(t, baseURL, otherToken, loud.ID, "Спасибо")

	expected := map[string]int{silent.ID: 0, quiet.ID: 1, loud.ID: 3}

	resp := fetchFeed(t, baseURL, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на страницу ленты ожидался статус 200, получен %d", resp.StatusCode)
	}
	raw := rawJSON(t, resp)

	var rawPage feedRawPayload
	if err := json.Unmarshal(raw, &rawPage); err != nil {
		t.Fatalf("страница ленты не разобралась как JSON: %v", err)
	}
	if len(rawPage.Items) != 3 {
		t.Fatalf("в ленте ожидались три поста, получено %d", len(rawPage.Items))
	}
	for i, item := range rawPage.Items {
		requireCommentsField(t, item, "пост ленты на месте "+strconv.Itoa(i+1))
	}

	var page commentedFeedPayload
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatalf("страница ленты не разобралась как посты: %v", err)
	}
	for _, item := range page.Items {
		want, ok := expected[item.ID]
		if !ok {
			t.Fatalf("в ленте оказался посторонний пост %s", item.ID)
		}
		requireCommentCount(t, item, want, "лента")
	}

	// У соседа та же лента и те же числа: разговор общий.
	for id, want := range expected {
		requireCommentCount(t, commentedFeedItem(t, baseURL, otherToken, id), want, "лента соседа")
	}

	// Комментарий под одним постом не трогает числа у других.
	commentOf(t, baseURL, token, silent.ID, "Тоже скажу")
	requireCommentCount(t, commentedFeedItem(t, baseURL, token, silent.ID), 1, "лента после новой реплики")
	requireCommentCount(t, commentedFeedItem(t, baseURL, token, quiet.ID), 1, "лента после новой реплики под другим постом")
	requireCommentCount(t, commentedFeedItem(t, baseURL, token, loud.ID), 3, "лента после новой реплики под другим постом")
}

// --- Жизнь комментария ----------------------------------------------------

// Комментарий правится только по адресу своего поста
// (specs/022-edit-block-delete.md): любой другой запрос на правку
// отвергается, а текст остаётся прежним (ФТ-9).
func TestCommentCannotBeChanged(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)

	comment := commentOf(t, baseURL, token, post.ID, commentText)

	attempts := map[string]struct {
		method string
		url    string
	}{
		"правка комментария":    {http.MethodPut, baseURL + "/comments/" + comment.ID},
		"частичная правка":      {http.MethodPatch, baseURL + "/posts/" + post.ID + "/comments/" + comment.ID},
		"правка списка целиком": {http.MethodPut, baseURL + "/posts/" + post.ID + "/comments"},
	}

	for name, attempt := range attempts {
		t.Run(name, func(t *testing.T) {
			resp := do(t, attempt.method, attempt.url, token, map[string]any{"text": "передумал"})
			if resp.StatusCode < 400 {
				t.Fatalf("%s: правки по этому адресу нет, а запрос принят со статусом %d", name, resp.StatusCode)
			}
		})
	}

	requireCommentTexts(t, baseURL, token, post.ID, []string{commentText}, "после попыток исправить")
}

// Комментарий живёт только вместе со своим постом: поста не стало —
// не стало и разговора (ФТ-10, ON DELETE CASCADE).
func TestCommentsDisappearWithTheirPost(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	post := postToComment(t, baseURL, token)
	survivor := postToComment(t, baseURL, token)

	commentOf(t, baseURL, token, post.ID, commentText)
	commentOf(t, baseURL, token, post.ID, "И ещё одна реплика")
	commentOf(t, baseURL, token, survivor.ID, "А этот пост остаётся")

	// Удаление поста уносит комментарии само: если бы не уносило,
	// ссылка из comments не дала бы удалить пост.
	removePostFromDatabase(t, post.ID)

	requireError(t, fetchComments(t, baseURL, token, post.ID), http.StatusNotFound, "post_not_found")
	requireError(t, addCommentText(t, baseURL, token, post.ID, commentText), http.StatusNotFound, "post_not_found")

	// Разговор под соседним постом при этом на месте.
	requireCommentTexts(t, baseURL, token, survivor.ID, []string{"А этот пост остаётся"}, "уцелевший пост")
	requireCommentCount(t, commentedPostPage(t, baseURL, token, survivor.ID), 1, "уцелевший пост")
}

// --- Ошибки и доступ ------------------------------------------------------

// Оба запроса к посту, которого нет, — 404 post_not_found. Не похожий
// на UUID идентификатор — тот же ответ: поста с таким идентификатором
// нет («Комментарий к несуществующему посту», «Идентификатор поста не
// похож на UUID», «Чтение комментариев несуществующего поста»).
func TestCommentRequestsOnUnknownPostAreNotFound(t *testing.T) {
	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownID,
		"вовсе не UUID":                 "не-идентификатор",
	}

	requests := map[string]func(t *testing.T, baseURL, token, postID string) *http.Response{
		"GET /api/posts/{postId}/comments": fetchComments,
		"POST /api/posts/{postId}/comments": func(t *testing.T, baseURL, token, postID string) *http.Response {
			return addCommentText(t, baseURL, token, postID, commentText)
		},
	}

	for name, request := range requests {
		for caseName, id := range ids {
			t.Run(name+": "+caseName, func(t *testing.T) {
				baseURL := startAPI(t)

				token, _ := signIn(t, baseURL, phonePretty)

				requireError(t, request(t, baseURL, token, id), http.StatusNotFound, "post_not_found")
			})
		}
	}
}

// Сервис сначала отвечает, есть ли куда писать: пустой комментарий
// к несуществующему посту — это 404 post_not_found, а не 400
// («Пустой комментарий к несуществующему посту», «API»).
func TestEmptyCommentOnUnknownPostIsNotFound(t *testing.T) {
	bodies := map[string]any{
		"пустая строка":     map[string]any{"text": ""},
		"одни пробелы":      map[string]any{"text": "   \n  "},
		"текста нет в теле": map[string]any{},
		"текст длиннее предела": map[string]any{
			"text": repeatRunes("абвгдеёжзи", commentLimit+1),
		},
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			requireError(t, addComment(t, baseURL, token, unknownID, body), http.StatusNotFound, "post_not_found")
		})
	}
}

// Без токена комментариев нет, как и всего остального: 401
// unauthorized. Недействительный токен — то же самое («Любой из
// запросов без токена», ФТ-12).
func TestCommentRequestsRequireValidToken(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)
	commentOf(t, baseURL, token, post.ID, commentText)

	requests := map[string]func(t *testing.T, baseURL, token, postID string) *http.Response{
		"GET /api/posts/{postId}/comments": fetchComments,
		"POST /api/posts/{postId}/comments": func(t *testing.T, baseURL, token, postID string) *http.Response {
			return addCommentText(t, baseURL, token, postID, "чужая реплика")
		},
	}

	tokens := map[string]string{
		"без токена":        "",
		"неизвестный токен": "этого-токена-сервис-не-выдавал",
	}

	for name, request := range requests {
		for caseName, badToken := range tokens {
			t.Run(name+": "+caseName, func(t *testing.T) {
				requireError(t, request(t, baseURL, badToken, post.ID), http.StatusUnauthorized, "unauthorized")
			})
		}
	}

	// Ни одна отвергнутая реплика не записалась.
	requireCommentTexts(t, baseURL, token, post.ID, []string{commentText}, "после отвергнутых запросов")
	requireCommentCountEverywhere(t, baseURL, token, post.ID, 1, "после отвергнутых запросов")
}

// Токен проверяется первым: запрос без токена к несуществующему посту —
// это 401, а не 404 («Запрос без токена к несуществующему посту»,
// «API»).
func TestCommentRequestWithoutTokenToUnknownPostIsUnauthorized(t *testing.T) {
	baseURL := startAPI(t)

	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownID,
		"вовсе не UUID":                 "не-идентификатор",
	}

	tokens := map[string]string{
		"без токена":        "",
		"неизвестный токен": "этого-токена-сервис-не-выдавал",
	}

	for idName, id := range ids {
		for tokenName, badToken := range tokens {
			t.Run("чтение, "+idName+", "+tokenName, func(t *testing.T) {
				requireError(t, fetchComments(t, baseURL, badToken, id), http.StatusUnauthorized, "unauthorized")
			})
			t.Run("комментарий, "+idName+", "+tokenName, func(t *testing.T) {
				requireError(t, addCommentText(t, baseURL, badToken, id, commentText),
					http.StatusUnauthorized, "unauthorized")
			})
			// И пустой комментарий без токена — тоже 401: текст
			// проверяется последним.
			t.Run("пустой комментарий, "+idName+", "+tokenName, func(t *testing.T) {
				requireError(t, addComment(t, baseURL, badToken, id, map[string]any{"text": "  "}),
					http.StatusUnauthorized, "unauthorized")
			})
		}
	}
}

// Комментарии не переживают выход из сессии: после signOut токен
// недействителен, и разговор им не прочитать (specs/001-auth.md).
func TestCommentsAreUnavailableAfterSignOut(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	post := postToComment(t, baseURL, token)
	commentOf(t, baseURL, token, post.ID, commentText)

	if resp := signOut(t, baseURL, token); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("на выход ожидался статус 204, получен %d", resp.StatusCode)
	}

	requireError(t, fetchComments(t, baseURL, token, post.ID), http.StatusUnauthorized, "unauthorized")
	requireError(t, addCommentText(t, baseURL, token, post.ID, "после выхода"),
		http.StatusUnauthorized, "unauthorized")
}
