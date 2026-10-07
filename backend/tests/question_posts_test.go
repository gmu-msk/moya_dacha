package tests

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// Тесты поста-вопроса (specs/033-question-posts.md). Написаны по
// спецификации и контракту, не глядя в реализацию (ADR-0002). Каждый тест
// ссылается на строку раздела «Ограничения и edge cases» и на требование
// (ФТ-N). Требования 13–20 — про приложение, по HTTP не проверяются.

// --- Представления из контракта -------------------------------------------

// qpPost — пост с полями вопроса (schema Post: `question`, `solved`,
// `answer_comment_id`). Сервер с вопросами отдаёт все три поля всегда
// («Пост в ответе»), поэтому question и solved — указатели: отсутствие
// поля — ошибка, а не false. answer_comment_id — сырой JSON: так видно
// разницу между `null` и отсутствием поля.
type qpPost struct {
	ID              string          `json:"id"`
	Caption         string          `json:"caption"`
	Question        *bool           `json:"question"`
	Solved          *bool           `json:"solved"`
	AnswerCommentID json.RawMessage `json:"answer_comment_id"`
}

// qpFeed — страница постов (schema Feed): лента, посты профиля, лента
// тэга и группы, «Сохранённые».
type qpFeed struct {
	Items      *[]qpPost `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}

// qpTag — тэг поста в тесте полей: слово из стартового словаря.
const qpTag = "огурцы"

// qpCaption — подпись вопроса из сценария.
const qpCaption = "Что с огурцами? Листья пожелтели"

// --- Хелперы: запросы -----------------------------------------------------

// qpSolvedURL — адрес статуса вопроса.
func qpSolvedURL(baseURL, postID string) string {
	return baseURL + "/posts/" + postID + "/solved"
}

// qpAnswerURL — адрес отметки решения.
func qpAnswerURL(baseURL, postID string) string {
	return baseURL + "/posts/" + postID + "/answer"
}

// qpSolvedReq — PUT /posts/{id}/solved; body — как есть.
func qpSolvedReq(t *testing.T, baseURL, token, postID string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, qpSolvedURL(baseURL, postID), token, body)
}

// qpMarkReq — PUT /posts/{id}/answer; body — как есть.
func qpMarkReq(t *testing.T, baseURL, token, postID string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, qpAnswerURL(baseURL, postID), token, body)
}

// qpMarkComment — PUT /posts/{id}/answer так, как это делает приложение.
func qpMarkComment(t *testing.T, baseURL, token, postID, commentID string) *http.Response {
	t.Helper()
	return qpMarkReq(t, baseURL, token, postID, map[string]any{"comment_id": commentID})
}

// qpUnmarkReq — DELETE /posts/{id}/answer.
func qpUnmarkReq(t *testing.T, baseURL, token, postID string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, qpAnswerURL(baseURL, postID), token, nil)
}

// --- Хелперы: разбор ------------------------------------------------------

// qpRequireFields требует у поста все три поля вопроса.
func qpRequireFields(t *testing.T, post qpPost, where string) {
	t.Helper()

	if post.Question == nil {
		t.Fatalf("%s: у поста %s нет поля question, а сервис отдаёт его всегда", where, post.ID)
	}
	if post.Solved == nil {
		t.Fatalf("%s: у поста %s нет поля solved, а сервис отдаёт его всегда", where, post.ID)
	}
	// Без поля — не Fatal: остальные проверки теста дальше считают
	// отсутствие за null и идут дальше, но ошибка засчитана.
	if len(post.AnswerCommentID) == 0 {
		t.Errorf("%s: у поста %s нет поля answer_comment_id, а сервис отдаёт его всегда (null, если не отмечено)", where, post.ID)
	}
}

// qpAnswer — отмеченный комментарий: "" — null.
func qpAnswer(t *testing.T, post qpPost, where string) string {
	t.Helper()

	if len(post.AnswerCommentID) == 0 || string(post.AnswerCommentID) == "null" {
		return ""
	}

	var id string
	if err := json.Unmarshal(post.AnswerCommentID, &id); err != nil {
		t.Fatalf("%s: answer_comment_id = %s — не строка и не null", where, post.AnswerCommentID)
	}
	if id == "" {
		t.Errorf("%s: answer_comment_id — пустая строка, а без отметки ожидался null", where)
	}

	return id
}

// qpPostOK требует статус и возвращает пост с полями вопроса.
func qpPostOK(t *testing.T, resp *http.Response, status int, where string) qpPost {
	t.Helper()

	if resp.StatusCode != status {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус %d, получен %d %s", where, status, resp.StatusCode, code)
	}

	var post qpPost
	decode(t, resp, &post)
	qpRequireFields(t, post, where)

	return post
}

// qpFeedOK требует 200 и разбирает страницу постов, требуя поля вопроса
// у каждого поста.
func qpFeedOK(t *testing.T, resp *http.Response, where string) qpFeed {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус 200, получен %d %s", where, resp.StatusCode, code)
	}

	var page qpFeed
	decode(t, resp, &page)

	if page.Items == nil {
		t.Fatalf("%s: в ответе нет items", where)
	}
	for _, item := range *page.Items {
		qpRequireFields(t, item, where)
	}

	return page
}

// qpFind — пост страницы с этим идентификатором или nil.
func qpFind(page qpFeed, postID string) *qpPost {
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

// qpRequire требует у поста именно такие question, solved
// и answer_comment_id (answer == "" — null).
func qpRequire(t *testing.T, post qpPost, question, solved bool, answer, where string) {
	t.Helper()

	qpRequireFields(t, post, where)

	if *post.Question != question {
		t.Errorf("%s: question = %v, ожидалось %v", where, *post.Question, question)
	}
	if *post.Solved != solved {
		t.Errorf("%s: solved = %v, ожидалось %v", where, *post.Solved, solved)
	}
	got := qpAnswer(t, post, where)
	if got != answer {
		want := answer
		if want == "" {
			want = "null"
		}
		shown := got
		if shown == "" {
			shown = "null"
		}
		t.Errorf("%s: answer_comment_id = %s, ожидалось %s", where, shown, want)
	}
}

// --- Хелперы: подготовка --------------------------------------------------

// qpPublishReq публикует пост с одной фотографией; question == nil — поля
// в теле нет вовсе. extra добавляется к телу как есть.
func qpPublishReq(t *testing.T, baseURL, token, caption string, question any, extra map[string]any) *http.Response {
	t.Helper()

	photo := photoOf(t, baseURL, token, 60, 40)
	body := map[string]any{"media_ids": []string{photo.ID}, "caption": caption}
	if question != nil {
		body["question"] = question
	}
	for k, v := range extra {
		body[k] = v
	}

	return createPost(t, baseURL, token, body)
}

// qpQuestion публикует вопрос и требует свежий вопрос в ответе (ФТ-2).
func qpQuestion(t *testing.T, baseURL string, who dachnik, caption string) qpPost {
	t.Helper()

	post := qpPostOK(t, qpPublishReq(t, baseURL, who.token, caption, true, nil), http.StatusCreated, "публикация вопроса")
	qpRequire(t, post, true, false, "", "свежий вопрос")

	return post
}

// qpOrdinary публикует обычный пост.
func qpOrdinary(t *testing.T, baseURL string, who dachnik, caption string) qpPost {
	t.Helper()
	return qpPostOK(t, qpPublishReq(t, baseURL, who.token, caption, nil, nil), http.StatusCreated, "публикация обычного поста")
}

// qpAt открывает пост по адресу и требует 200.
func qpAt(t *testing.T, baseURL string, who dachnik, postID string) qpPost {
	t.Helper()
	return qpPostOK(t, fetchPost(t, baseURL, who.token, postID), http.StatusOK, "GET /posts/"+postID)
}

// qpSolve ставит статус своему вопросу и требует 200 с тем же постом.
func qpSolve(t *testing.T, baseURL string, who dachnik, postID string, solved bool) qpPost {
	t.Helper()

	post := qpPostOK(t, qpSolvedReq(t, baseURL, who.token, postID, map[string]any{"solved": solved}),
		http.StatusOK, "PUT /posts/{id}/solved")
	if post.ID != postID {
		t.Fatalf("статус вопроса %s вернул пост %s", postID, post.ID)
	}

	return post
}

// qpMark отмечает комментарий решением и требует 200 с тем же постом.
func qpMark(t *testing.T, baseURL string, who dachnik, postID, commentID string) qpPost {
	t.Helper()

	post := qpPostOK(t, qpMarkComment(t, baseURL, who.token, postID, commentID), http.StatusOK, "PUT /posts/{id}/answer")
	if post.ID != postID {
		t.Fatalf("отметка решения у %s вернула пост %s", postID, post.ID)
	}

	return post
}

// qpUnmark снимает отметку и требует 200 с тем же постом.
func qpUnmark(t *testing.T, baseURL string, who dachnik, postID string) qpPost {
	t.Helper()

	post := qpPostOK(t, qpUnmarkReq(t, baseURL, who.token, postID), http.StatusOK, "DELETE /posts/{id}/answer")
	if post.ID != postID {
		t.Fatalf("снятие отметки у %s вернуло пост %s", postID, post.ID)
	}

	return post
}

// qpRequireAt открывает пост от имени who и требует такие поля вопроса.
func qpRequireAt(t *testing.T, baseURL string, who dachnik, postID string, solved bool, answer, where string) {
	t.Helper()
	qpRequire(t, qpAt(t, baseURL, who, postID), true, solved, answer, where)
}

// qpSolvedWithAnswer — вопрос автора с комментарием читателя,
// отмеченным решением.
func qpSolvedWithAnswer(t *testing.T, baseURL string, author, reader dachnik) (qpPost, commentPayload) {
	t.Helper()

	post := qpQuestion(t, baseURL, author, qpCaption)
	answer := commentOf(t, baseURL, reader.token, post.ID, "Это мучнистая роса, брызгайте содой")
	qpRequire(t, qpMark(t, baseURL, author, post.ID, answer.ID), true, true, answer.ID, "отметка решения")

	return post, answer
}

// ============================================================================
// Публикация
// ============================================================================

// «Публикация с question: true»: 201, question: true, solved: false,
// answer_comment_id: null — все три поля есть в самом ответе (ФТ-1, ФТ-2).
func TestQuestionPublishAsQuestion(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)

	resp := qpPublishReq(t, baseURL, author.token, qpCaption, true, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("публикация вопроса: ожидался статус 201, получен %d", resp.StatusCode)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON(t, resp), &fields); err != nil {
		t.Fatalf("ответ на публикацию не разобрался как JSON: %v", err)
	}
	want := map[string]string{"question": "true", "solved": "false", "answer_comment_id": "null"}
	for name, value := range want {
		got, ok := fields[name]
		if !ok {
			t.Errorf("в ответе на публикацию нет поля %s", name)
			continue
		}
		if string(got) != value {
			t.Errorf("в ответе на публикацию %s = %s, ожидалось %s", name, got, value)
		}
	}

	var id string
	if err := json.Unmarshal(fields["id"], &id); err != nil || id == "" {
		t.Fatalf("в ответе на публикацию нет id поста")
	}
	qpRequire(t, qpAt(t, baseURL, author, id), true, false, "", "вопрос по адресу")
}

// «Публикация без question или с false»: обычный пост — question: false,
// solved: false, answer_comment_id: null (ФТ-1, ФТ-2).
func TestQuestionPublishOrdinary(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)

	cases := []struct {
		name     string
		question any
	}{
		{"без поля question", nil},
		{"question: false", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			post := qpPostOK(t, qpPublishReq(t, baseURL, author.token, "Первая клубника", tc.question, nil),
				http.StatusCreated, "публикация "+tc.name)
			qpRequire(t, post, false, false, "", "ответ на публикацию")
			qpRequire(t, qpAt(t, baseURL, author, post.ID), false, false, "", "пост по адресу")
		})
	}
}

// «Поля в ленте, постах профиля, „Сохранённых“, на посте, в ответе
// на лайк»: question, solved, answer_comment_id есть везде, где приходит
// пост, — во всех вкладках ленты, в ленте тэга и группы, в постах профиля,
// в «Сохранённых», по адресу, в ответах на лайк и закладку. У обычного
// поста рядом — false, false, null (ФТ-2).
func TestQuestionFieldsEverywhere(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	group := grInterestGroup(t, baseURL, author, "Огородники")
	grJoin(t, baseURL, reader, group.ID, grMember)
	followOK(t, baseURL, reader, author.id)

	extra := map[string]any{"group_ids": []string{group.ID}}
	question := qpPostOK(t, qpPublishReq(t, baseURL, author.token, "Что с листьями? #"+qpTag, true, extra),
		http.StatusCreated, "публикация вопроса с тэгом в группе")
	ordinary := qpPostOK(t, qpPublishReq(t, baseURL, author.token, "Урожай #"+qpTag, nil, extra),
		http.StatusCreated, "публикация обычного поста с тэгом в группе")

	answer := commentOf(t, baseURL, reader.token, question.ID, "Это мучнистая роса")
	qpMark(t, baseURL, author, question.ID, answer.ID)

	expect := func(t *testing.T, post qpPost, where string) {
		t.Helper()
		switch post.ID {
		case question.ID:
			qpRequire(t, post, true, true, answer.ID, where+", вопрос")
		case ordinary.ID:
			qpRequire(t, post, false, false, "", where+", обычный пост")
		}
	}

	for _, v := range []struct {
		name string
		who  dachnik
	}{
		{"автор", author},
		{"читатель", reader},
	} {
		t.Run(v.name, func(t *testing.T) {
			for _, id := range []string{question.ID, ordinary.ID} {
				expect(t, qpAt(t, baseURL, v.who, id), "пост по адресу")
			}

			feeds := map[string]*http.Response{
				"лента без вкладки": fetchFeed(t, baseURL, v.who.token, feedParams(50, "")),
				"вкладка all":       fetchFeed(t, baseURL, v.who.token, scopeParams("all", 50, "")),
				"лента тэга":        fetchFeedRaw(t, baseURL, v.who.token, url.Values{"limit": {"50"}, "tag": {qpTag}}.Encode()),
				"лента группы":      gpGroupPostsReq(t, baseURL, v.who.token, group.ID, "limit=50"),
				"посты профиля":     fetchUserPosts(t, baseURL, v.who.token, author.id, feedParams(50, "")),
			}
			if v.who.id != author.id {
				feeds["вкладка following"] = fetchFeed(t, baseURL, v.who.token, scopeParams("following", 50, ""))
			}

			for where, resp := range feeds {
				page := qpFeedOK(t, resp, where)
				for _, id := range []string{question.ID, ordinary.ID} {
					item := qpFind(page, id)
					if item == nil {
						t.Errorf("%s: поста %s нет, а он виден", where, id)
						continue
					}
					expect(t, *item, where)
				}
			}

			for _, id := range []string{question.ID, ordinary.ID} {
				expect(t, qpPostOK(t, likePost(t, baseURL, v.who.token, id), http.StatusOK, "лайк"), "ответ на лайк")
				expect(t, qpPostOK(t, unlikePost(t, baseURL, v.who.token, id), http.StatusOK, "снятие лайка"), "ответ на снятие лайка")
				expect(t, qpPostOK(t, bookmarkPutReq(t, baseURL, v.who.token, id), http.StatusOK, "закладка"), "ответ на закладку")
			}

			page := qpFeedOK(t, bookmarkListReq(t, baseURL, v.who.token, "limit=50"), "«Сохранённые»")
			for _, id := range []string{question.ID, ordinary.ID} {
				item := qpFind(page, id)
				if item == nil {
					t.Errorf("«Сохранённые»: поста %s нет, а он сохранён", id)
					continue
				}
				expect(t, *item, "«Сохранённые»")
			}

			for _, id := range []string{question.ID, ordinary.ID} {
				expect(t, qpPostOK(t, bookmarkDeleteReq(t, baseURL, v.who.token, id), http.StatusOK, "снятие закладки"), "ответ на снятие закладки")
			}
		})
	}
}

// ============================================================================
// PUT /api/posts/{postId}/solved
// ============================================================================

// «Автор: solved: true»: 200, solved: true, answer_comment_id не меняется —
// ни null без отметки, ни отмеченный комментарий (ФТ-3, ФТ-5).
func TestQuestionSolvedTrueKeepsAnswer(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	t.Run("без отметки", func(t *testing.T) {
		post := qpQuestion(t, baseURL, author, "Разобрался сам")
		qpRequire(t, qpSolve(t, baseURL, author, post.ID, true), true, true, "", "ответ на solved: true")
		qpRequireAt(t, baseURL, author, post.ID, true, "", "пост по адресу")
	})

	t.Run("при отмеченном решении", func(t *testing.T) {
		post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)
		qpRequire(t, qpSolve(t, baseURL, author, post.ID, true), true, true, answer.ID, "ответ на solved: true")
		qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "пост по адресу")
	})
}

// «Автор: solved: false при отмеченном решении»: 200, solved: false,
// answer_comment_id: null (ФТ-5).
func TestQuestionSolvedFalseClearsAnswer(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post, _ := qpSolvedWithAnswer(t, baseURL, author, reader)

	qpRequire(t, qpSolve(t, baseURL, author, post.ID, false), true, false, "", "ответ на solved: false")
	qpRequireAt(t, baseURL, author, post.ID, false, "", "у автора")
	qpRequireAt(t, baseURL, reader, post.ID, false, "", "у читателя")
}

// «Повтор solved с тем же значением»: 200, ничего не меняется (ФТ-5).
func TestQuestionSolvedRepeatIsNoop(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	t.Run("false у нерешённого", func(t *testing.T) {
		post := qpQuestion(t, baseURL, author, "Нерешённый")
		qpRequire(t, qpSolve(t, baseURL, author, post.ID, false), true, false, "", "первый false")
		qpRequire(t, qpSolve(t, baseURL, author, post.ID, false), true, false, "", "повтор false")
	})

	t.Run("true дважды", func(t *testing.T) {
		post := qpQuestion(t, baseURL, author, "Решённый")
		qpRequire(t, qpSolve(t, baseURL, author, post.ID, true), true, true, "", "первый true")
		qpRequire(t, qpSolve(t, baseURL, author, post.ID, true), true, true, "", "повтор true")
	})

	t.Run("true у решённого с отметкой", func(t *testing.T) {
		post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)
		qpRequire(t, qpSolve(t, baseURL, author, post.ID, true), true, true, answer.ID, "повтор true")
		qpRequire(t, qpSolve(t, baseURL, author, post.ID, true), true, true, answer.ID, "ещё повтор true")
		qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "пост по адресу")
	})
}

// «Тело {} или не JSON у solved»: 400 invalid_request; тела нет вовсе —
// тоже. Статус и отметка не меняются (ФТ-10, «PUT …/solved»).
func TestQuestionSolvedBadBody(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)
	address := qpSolvedURL(baseURL, post.ID)

	cases := map[string]*http.Response{
		"тело {}": qpSolvedReq(t, baseURL, author.token, post.ID, map[string]any{}),
		"чужое поле вместо solved":  qpSolvedReq(t, baseURL, author.token, post.ID, map[string]any{"resolved": false}),
		"тело не JSON":              ebdRaw(t, http.MethodPut, address, author.token, "solved=false"),
		"обрезанный JSON":           ebdRaw(t, http.MethodPut, address, author.token, `{"solved": fal`),
		"тела нет вовсе":            ebdRaw(t, http.MethodPut, address, author.token, ""),
		"solved не булево (строка)": qpSolvedReq(t, baseURL, author.token, post.ID, map[string]any{"solved": "false"}),
	}

	for name, resp := range cases {
		requireCodeE(t, resp, http.StatusBadRequest, "invalid_request", name)
	}

	qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "после отказов")
}

// ============================================================================
// PUT /api/posts/{postId}/answer
// ============================================================================

// «Автор отмечает чужой комментарий»: 200, solved: true,
// answer_comment_id — он; видно и автору, и отвечавшему (ФТ-6).
func TestQuestionMarkOthersComment(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := qpQuestion(t, baseURL, author, qpCaption)
	answer := commentOf(t, baseURL, reader.token, post.ID, "Это мучнистая роса, брызгайте содой")

	qpRequire(t, qpMark(t, baseURL, author, post.ID, answer.ID), true, true, answer.ID, "ответ на отметку")
	qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "у автора")
	qpRequireAt(t, baseURL, reader, post.ID, true, answer.ID, "у отвечавшего")
}

// «Автор отмечает свой комментарий»: 200, отмечается (ФТ-6).
func TestQuestionMarkOwnComment(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)

	post := qpQuestion(t, baseURL, author, qpCaption)
	own := commentOf(t, baseURL, author.token, post.ID, "Разобрался: полил тёплой водой, прошло")

	qpRequire(t, qpMark(t, baseURL, author, post.ID, own.ID), true, true, own.ID, "ответ на отметку своего")
	qpRequireAt(t, baseURL, author, post.ID, true, own.ID, "пост по адресу")
}

// «Отметка другого комментария»: answer_comment_id — новый, прежний
// не отмечен: решение у вопроса одно (ФТ-6, сценарий шаг 5).
func TestQuestionMarkAnotherCommentMovesMark(t *testing.T) {
	baseURL := startAPI(t)
	author, anna, petr := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	post := qpQuestion(t, baseURL, author, qpCaption)
	fromAnna := commentOf(t, baseURL, anna.token, post.ID, "Это мучнистая роса")
	fromPetr := commentOf(t, baseURL, petr.token, post.ID, "Не хватает азота, подкормите")

	qpMark(t, baseURL, author, post.ID, fromAnna.ID)
	moved := qpMark(t, baseURL, author, post.ID, fromPetr.ID)

	qpRequire(t, moved, true, true, fromPetr.ID, "ответ на вторую отметку")
	for _, who := range []dachnik{author, anna, petr} {
		qpRequireAt(t, baseURL, who, post.ID, true, fromPetr.ID, "пост по адресу")
	}

	back := qpMark(t, baseURL, author, post.ID, fromAnna.ID)
	qpRequire(t, back, true, true, fromAnna.ID, "отметка вернулась к первому")
}

// «Повторная отметка того же»: 200, ничего не меняется (ФТ-6).
func TestQuestionMarkSameCommentAgain(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)

	qpRequire(t, qpMark(t, baseURL, author, post.ID, answer.ID), true, true, answer.ID, "повторная отметка")
	qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "пост по адресу")
}

// «Отметка при solved: true без решения»: answer_comment_id ставится,
// solved: true (ФТ-3, ФТ-6).
func TestQuestionMarkWhenSolvedWithoutAnswer(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := qpQuestion(t, baseURL, author, qpCaption)
	qpSolve(t, baseURL, author, post.ID, true)
	answer := commentOf(t, baseURL, reader.token, post.ID, "А вот почему")

	qpRequire(t, qpMark(t, baseURL, author, post.ID, answer.ID), true, true, answer.ID, "отметка у решённого")
	qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "пост по адресу")
}

// ============================================================================
// DELETE /api/posts/{postId}/answer
// ============================================================================

// «DELETE /answer при отмеченном»: 200, answer_comment_id: null,
// solved: false (ФТ-7).
func TestQuestionUnmarkMarked(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post, _ := qpSolvedWithAnswer(t, baseURL, author, reader)

	qpRequire(t, qpUnmark(t, baseURL, author, post.ID), true, false, "", "ответ на снятие отметки")
	qpRequireAt(t, baseURL, author, post.ID, false, "", "у автора")
	qpRequireAt(t, baseURL, reader, post.ID, false, "", "у читателя")
}

// «DELETE /answer без отметки при solved: true»: 200, solved: false (ФТ-7).
func TestQuestionUnmarkWithoutMarkWhenSolved(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)

	post := qpQuestion(t, baseURL, author, qpCaption)
	qpSolve(t, baseURL, author, post.ID, true)

	qpRequire(t, qpUnmark(t, baseURL, author, post.ID), true, false, "", "ответ на снятие несуществующей отметки")
	qpRequireAt(t, baseURL, author, post.ID, false, "", "пост по адресу")
}

// «DELETE /answer без отметки при solved: false»: 200, ничего
// не меняется (ФТ-7).
func TestQuestionUnmarkWithoutMarkWhenOpen(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)

	post := qpQuestion(t, baseURL, author, qpCaption)

	qpRequire(t, qpUnmark(t, baseURL, author, post.ID), true, false, "", "первое снятие")
	qpRequire(t, qpUnmark(t, baseURL, author, post.ID), true, false, "", "повтор снятия")
	qpRequireAt(t, baseURL, author, post.ID, false, "", "пост по адресу")
}

// ============================================================================
// Какой комментарий можно отметить
// ============================================================================

// «Комментарий под другим постом»: 404 comment_not_found — и под чужим
// постом, и под другим своим вопросом. Отметка не меняется (ФТ-8).
func TestQuestionMarkCommentOfOtherPost(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)

	foreign := qpOrdinary(t, baseURL, reader, "Пост читателя")
	underForeign := commentOf(t, baseURL, author.token, foreign.ID, "Комментарий автора под чужим постом")

	second := qpQuestion(t, baseURL, author, "Второй вопрос")
	underSecond := commentOf(t, baseURL, reader.token, second.ID, "Ответ на второй вопрос")

	requireCodeE(t, qpMarkComment(t, baseURL, author.token, post.ID, underForeign.ID),
		http.StatusNotFound, "comment_not_found", "комментарий под чужим постом")
	requireCodeE(t, qpMarkComment(t, baseURL, author.token, post.ID, underSecond.ID),
		http.StatusNotFound, "comment_not_found", "комментарий под другим своим вопросом")

	qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "первый вопрос после отказов")
	qpRequireAt(t, baseURL, author, second.ID, false, "", "второй вопрос не тронут")
}

// «Комментария нет / comment_id не UUID»: 404 comment_not_found, в том
// числе удалённый комментарий (ФТ-8, «PUT …/answer»).
func TestQuestionMarkUnknownComment(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := qpQuestion(t, baseURL, author, qpCaption)
	gone := commentOf(t, baseURL, reader.token, post.ID, "Передумал")
	requireDeleted(t, deleteComment(t, baseURL, reader.token, post.ID, gone.ID))

	ids := map[string]string{
		"несуществующий комментарий": unknownID,
		"вовсе не UUID":              notAnID,
		"удалённый комментарий":      gone.ID,
	}

	for name, id := range ids {
		requireCodeE(t, qpMarkComment(t, baseURL, author.token, post.ID, id),
			http.StatusNotFound, "comment_not_found", name)
	}

	qpRequireAt(t, baseURL, author, post.ID, false, "", "после отказов")
}

// «Комментарий того, с кем у автора блокировка»: 404 comment_not_found —
// в обе стороны блокировки: такой комментарий автору не виден (ФТ-8;
// 022, ФТ-13).
func TestQuestionMarkCommentOfBlocked(t *testing.T) {
	cases := []struct {
		name  string
		block func(t *testing.T, baseURL string, author, commenter dachnik)
	}{
		{"автор заблокировал комментатора", func(t *testing.T, baseURL string, author, commenter dachnik) {
			blockOK(t, baseURL, author, commenter.id)
		}},
		{"комментатор заблокировал автора", func(t *testing.T, baseURL string, author, commenter dachnik) {
			blockOK(t, baseURL, commenter, author.id)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL := startAPI(t)
			author, commenter := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

			post := qpQuestion(t, baseURL, author, qpCaption)
			comment := commentOf(t, baseURL, commenter.token, post.ID, "Ответ до блокировки")

			tc.block(t, baseURL, author, commenter)

			requireCodeE(t, qpMarkComment(t, baseURL, author.token, post.ID, comment.ID),
				http.StatusNotFound, "comment_not_found", tc.name)
			qpRequireAt(t, baseURL, author, post.ID, false, "", "после отказа")
		})
	}
}

// «Тело без comment_id»: 400 invalid_request; пустой comment_id, тело
// не JSON и тела нет вовсе — тоже. Отметка не меняется («PUT …/answer»).
func TestQuestionMarkBadBody(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)
	address := qpAnswerURL(baseURL, post.ID)

	cases := map[string]*http.Response{
		"тело {}":           qpMarkReq(t, baseURL, author.token, post.ID, map[string]any{}),
		"пустой comment_id": qpMarkReq(t, baseURL, author.token, post.ID, map[string]any{"comment_id": ""}),
		"тело не JSON":      ebdRaw(t, http.MethodPut, address, author.token, "comment_id="+answer.ID),
		"обрезанный JSON":   ebdRaw(t, http.MethodPut, address, author.token, `{"comment_id": "`),
		"тела нет вовсе":    ebdRaw(t, http.MethodPut, address, author.token, ""),
	}

	for name, resp := range cases {
		requireCodeE(t, resp, http.StatusBadRequest, "invalid_request", name)
	}

	qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "после отказов")
}

// ============================================================================
// Кто и где может менять
// ============================================================================

// «Не автор: любая из трёх ручек»: 403 not_your_post, ничего не меняется —
// и у того, кто ответил, и у постороннего (ФТ-4).
func TestQuestionNotAuthorIsForbidden(t *testing.T) {
	baseURL := startAPI(t)
	author, answerer, stranger := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	post, answer := qpSolvedWithAnswer(t, baseURL, author, answerer)
	other := commentOf(t, baseURL, stranger.token, post.ID, "Ещё вариант")

	for _, v := range []struct {
		name string
		who  dachnik
	}{
		{"автор решения", answerer},
		{"посторонний", stranger},
	} {
		t.Run(v.name, func(t *testing.T) {
			requests := map[string]*http.Response{
				"solved: false":         qpSolvedReq(t, baseURL, v.who.token, post.ID, map[string]any{"solved": false}),
				"solved: true":          qpSolvedReq(t, baseURL, v.who.token, post.ID, map[string]any{"solved": true}),
				"отметка другого":       qpMarkComment(t, baseURL, v.who.token, post.ID, other.ID),
				"отметка того же":       qpMarkComment(t, baseURL, v.who.token, post.ID, answer.ID),
				"снятие отметки":        qpUnmarkReq(t, baseURL, v.who.token, post.ID),
				"solved с телом {}":     qpSolvedReq(t, baseURL, v.who.token, post.ID, map[string]any{}),
				"answer с телом {}":     qpMarkReq(t, baseURL, v.who.token, post.ID, map[string]any{}),
				"отметка несуществующ.": qpMarkComment(t, baseURL, v.who.token, post.ID, unknownID),
			}
			for name, resp := range requests {
				requireCodeE(t, resp, http.StatusForbidden, "not_your_post", name)
			}

			qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "у автора после отказов")
			qpRequireAt(t, baseURL, v.who, post.ID, true, answer.ID, "у пытавшегося после отказов")
		})
	}
}

// «Обычный пост: любая из трёх ручек»: 400 not_a_question — даже
// с правильным телом и комментарием под этим постом; поля остаются
// false, false, null (ФТ-9).
func TestQuestionOrdinaryPostIsNotAQuestion(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := qpOrdinary(t, baseURL, author, "Первая клубника")
	comment := commentOf(t, baseURL, reader.token, post.ID, "Красота!")

	requests := map[string]*http.Response{
		"solved: true":      qpSolvedReq(t, baseURL, author.token, post.ID, map[string]any{"solved": true}),
		"solved: false":     qpSolvedReq(t, baseURL, author.token, post.ID, map[string]any{"solved": false}),
		"отметка решения":   qpMarkComment(t, baseURL, author.token, post.ID, comment.ID),
		"снятие отметки":    qpUnmarkReq(t, baseURL, author.token, post.ID),
		"отметка чужого id": qpMarkComment(t, baseURL, author.token, post.ID, unknownID),
	}
	for name, resp := range requests {
		requireCodeE(t, resp, http.StatusBadRequest, "not_a_question", name)
	}

	qpRequire(t, qpAt(t, baseURL, author, post.ID), false, false, "", "обычный пост после отказов")
}

// «Чужой обычный пост»: 403 not_your_post — сначала «свой ли», потом
// «вопрос ли» (ФТ-10).
func TestQuestionForeignOrdinaryPostIsForbidden(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := qpOrdinary(t, baseURL, author, "Первая клубника")
	comment := commentOf(t, baseURL, reader.token, post.ID, "Красота!")

	requests := map[string]*http.Response{
		"solved: true":    qpSolvedReq(t, baseURL, reader.token, post.ID, map[string]any{"solved": true}),
		"отметка решения": qpMarkComment(t, baseURL, reader.token, post.ID, comment.ID),
		"снятие отметки":  qpUnmarkReq(t, baseURL, reader.token, post.ID),
		"solved, тело {}": qpSolvedReq(t, baseURL, reader.token, post.ID, map[string]any{}),
		"answer, тело {}": qpMarkReq(t, baseURL, reader.token, post.ID, map[string]any{}),
	}
	for name, resp := range requests {
		requireCodeE(t, resp, http.StatusForbidden, "not_your_post", name)
	}
}

// «Свой обычный пост, тело {}»: 400 not_a_question — сначала «вопрос ли»,
// потом поля тела. Тело, которое вовсе не JSON, отклоняется раньше всех
// проверок: 400 invalid_request и у обычного поста, и у чужого (ФТ-10).
func TestQuestionOrdinaryPostChecksQuestionBeforeBody(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post := qpOrdinary(t, baseURL, author, "Первая клубника")

	requireCodeE(t, qpSolvedReq(t, baseURL, author.token, post.ID, map[string]any{}),
		http.StatusBadRequest, "not_a_question", "solved, тело {}")
	requireCodeE(t, qpMarkReq(t, baseURL, author.token, post.ID, map[string]any{}),
		http.StatusBadRequest, "not_a_question", "answer, тело {}")
	requireCodeE(t, qpMarkReq(t, baseURL, author.token, post.ID, map[string]any{"comment_id": ""}),
		http.StatusBadRequest, "not_a_question", "answer, пустой comment_id")

	t.Run("тело не JSON — раньше всех проверок", func(t *testing.T) {
		for _, who := range []struct {
			name  string
			token string
		}{
			{"свой обычный пост", author.token},
			{"чужой обычный пост", reader.token},
		} {
			requireCodeE(t, ebdRaw(t, http.MethodPut, qpSolvedURL(baseURL, post.ID), who.token, "не json"),
				http.StatusBadRequest, "invalid_request", "solved, "+who.name)
			requireCodeE(t, ebdRaw(t, http.MethodPut, qpAnswerURL(baseURL, post.ID), who.token, "не json"),
				http.StatusBadRequest, "invalid_request", "answer, "+who.name)
		}
	})
}

// «Пост не виден / его нет / id не UUID»: 404 post_not_found у всех трёх
// ручек — невидимый пост как несуществующий, раньше «свой ли» (ФТ-10).
func TestQuestionPostNotFound(t *testing.T) {
	baseURL := startAPI(t)
	author, reader, blocked := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	hidden := qpPostOK(t, qpPublishReq(t, baseURL, author.token, "Только для себя", true,
		map[string]any{"visibility": visibilityMe}), http.StatusCreated, "публикация вопроса «Только я»")
	visible := qpQuestion(t, baseURL, author, qpCaption)
	blockOK(t, baseURL, author, blocked.id)

	cases := []struct {
		name   string
		who    dachnik
		postID string
	}{
		{"чужой вопрос «Только я»", reader, hidden.ID},
		{"вопрос заблокировавшего", blocked, visible.ID},
		{"несуществующий пост", reader, unknownID},
		{"вовсе не UUID", reader, notAnID},
		{"несуществующий пост у автора", author, unknownID},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireNotFound(t, qpSolvedReq(t, baseURL, tc.who.token, tc.postID, map[string]any{"solved": true}), "PUT solved", tc.name)
			requireNotFound(t, qpSolvedReq(t, baseURL, tc.who.token, tc.postID, map[string]any{}), "PUT solved с телом {}", tc.name)
			requireNotFound(t, qpMarkComment(t, baseURL, tc.who.token, tc.postID, unknownID), "PUT answer", tc.name)
			requireNotFound(t, qpMarkReq(t, baseURL, tc.who.token, tc.postID, map[string]any{}), "PUT answer с телом {}", tc.name)
			requireNotFound(t, qpUnmarkReq(t, baseURL, tc.who.token, tc.postID), "DELETE answer", tc.name)
		})
	}

	qpRequireAt(t, baseURL, author, hidden.ID, false, "", "вопрос «Только я» не тронут")
	qpRequireAt(t, baseURL, author, visible.ID, false, "", "видимый вопрос не тронут")
}

// «Без токена, в том числе к несуществующему посту»: 401 unauthorized —
// токен проверяется раньше всего остального (ФТ-10).
func TestQuestionRequiresToken(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)

	tokens := map[string]string{
		"без токена":             "",
		"недействительный токен": "не-токен-вовсе",
	}

	for name, token := range tokens {
		t.Run(name, func(t *testing.T) {
			for _, postID := range []string{post.ID, unknownID, notAnID} {
				requireUnauthorized(t, qpSolvedReq(t, baseURL, token, postID, map[string]any{"solved": false}), "PUT solved "+postID)
				requireUnauthorized(t, qpMarkComment(t, baseURL, token, postID, answer.ID), "PUT answer "+postID)
				requireUnauthorized(t, qpUnmarkReq(t, baseURL, token, postID), "DELETE answer "+postID)
			}
		})
	}

	qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "после отказов")
}

// ============================================================================
// Решение удалили
// ============================================================================

// «Решение удалил его автор»: answer_comment_id: null, solved: false —
// решение пропало, вопрос снова открыт (ФТ-11; 007).
func TestQuestionAnswerDeletedByItsAuthor(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)

	requireDeleted(t, deleteComment(t, baseURL, reader.token, post.ID, answer.ID))

	qpRequireAt(t, baseURL, author, post.ID, false, "", "у автора вопроса")
	qpRequireAt(t, baseURL, reader, post.ID, false, "", "у удалившего")
}

// «Решение удалил владелец сервиса из дашборда»: answer_comment_id: null,
// solved: false (ФТ-11; 023).
func TestQuestionAnswerDeletedFromDashboard(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)

	modDeleted(t, root, "/dashboard/comments/"+answer.ID, "удаление решения владельцем")

	qpRequireAt(t, baseURL, author, post.ID, false, "", "у автора вопроса")
	qpRequireAt(t, baseURL, reader, post.ID, false, "", "у автора решения")
}

// «Удалили аккаунт автора решения»: answer_comment_id: null,
// solved: false (ФТ-11; 022).
func TestQuestionAnswerAuthorAccountDeleted(t *testing.T) {
	baseURL := startAPI(t)
	author, leaver, other := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	post, _ := qpSolvedWithAnswer(t, baseURL, author, leaver)

	deleteMeOK(t, baseURL, leaver.token)

	qpRequireAt(t, baseURL, author, post.ID, false, "", "у автора вопроса")
	qpRequireAt(t, baseURL, other, post.ID, false, "", "у постороннего")
}

// «Удалили другой комментарий»: статус и отметка не меняются — ни
// у вопроса с отмеченным решением, ни у решённого без отметки (ФТ-11).
func TestQuestionOtherCommentDeletedKeepsStatus(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})
	author, reader, other := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	t.Run("при отмеченном решении", func(t *testing.T) {
		post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)
		byOther := commentOf(t, baseURL, other.token, post.ID, "Не знаю")
		byOwner := commentOf(t, baseURL, other.token, post.ID, "Реклама удобрений")
		leaving := newDachnik(t, baseURL, 4)
		commentOf(t, baseURL, leaving.token, post.ID, "Тоже интересно")

		requireDeleted(t, deleteComment(t, baseURL, other.token, post.ID, byOther.ID))
		qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "после удаления комментария его автором")

		modDeleted(t, root, "/dashboard/comments/"+byOwner.ID, "удаление комментария владельцем")
		qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "после удаления из дашборда")

		deleteMeOK(t, baseURL, leaving.token)
		qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "после удаления аккаунта комментатора")
	})

	t.Run("решён без отметки", func(t *testing.T) {
		post := qpQuestion(t, baseURL, author, "Разобрался сам")
		comment := commentOf(t, baseURL, other.token, post.ID, "Поздравляю")
		qpSolve(t, baseURL, author, post.ID, true)

		requireDeleted(t, deleteComment(t, baseURL, other.token, post.ID, comment.ID))
		qpRequireAt(t, baseURL, author, post.ID, true, "", "после удаления комментария")
	})
}

// ============================================================================
// Правки и чужой взгляд
// ============================================================================

// «Правка текста решения»: отметка остаётся (ФТ-12; 022).
func TestQuestionAnswerEditKeepsMark(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)

	resp := editCommentText(t, baseURL, reader.token, post.ID, answer.ID, "Это мучнистая роса: сода, ложка на литр")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("правка решения: ожидался статус 200, получен %d", resp.StatusCode)
	}

	qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "после правки решения")
}

// «Правка подписи вопроса»: question, solved, answer_comment_id
// не меняются — ни в ответе на правку, ни по адресу (ФТ-2; 022).
func TestQuestionCaptionEditKeepsFields(t *testing.T) {
	baseURL := startAPI(t)
	author, reader := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2)

	t.Run("решённый с отметкой", func(t *testing.T) {
		post, answer := qpSolvedWithAnswer(t, baseURL, author, reader)
		edited := qpPostOK(t, editCaptionText(t, baseURL, author.token, post.ID, "Что с огурцами? Уже понял"),
			http.StatusOK, "правка подписи")
		qpRequire(t, edited, true, true, answer.ID, "ответ на правку подписи")
		qpRequireAt(t, baseURL, author, post.ID, true, answer.ID, "по адресу после правки")
	})

	t.Run("нерешённый", func(t *testing.T) {
		post := qpQuestion(t, baseURL, author, qpCaption)
		edited := qpPostOK(t, editCaptionText(t, baseURL, author.token, post.ID, "Что с огурцами? Фото добавил"),
			http.StatusOK, "правка подписи")
		qpRequire(t, edited, true, false, "", "ответ на правку подписи")
	})

	t.Run("обычный пост", func(t *testing.T) {
		post := qpOrdinary(t, baseURL, author, "Первая клубника")
		edited := qpPostOK(t, editCaptionText(t, baseURL, author.token, post.ID, "Первая клубника!"),
			http.StatusOK, "правка подписи")
		qpRequire(t, edited, false, false, "", "ответ на правку подписи обычного поста")
	})

	t.Run("поле question в правке подписи вопрос не меняет", func(t *testing.T) {
		post := qpQuestion(t, baseURL, author, qpCaption)
		resp := editCaptionReq(t, baseURL, author.token, post.ID, map[string]any{"caption": "Новая", "question": false})
		if resp.StatusCode == http.StatusOK {
			qpRequire(t, qpPostOK(t, resp, http.StatusOK, "правка подписи с question"), true, false, "", "ответ на правку")
		}
		qpRequireAt(t, baseURL, author, post.ID, false, "", "вопрос остался вопросом")
	})
}

// «Другой пользователь смотрит решённый вопрос»: видит solved: true
// и answer_comment_id — по адресу и в ленте (ФТ-4).
func TestQuestionOthersSeeSolved(t *testing.T) {
	baseURL := startAPI(t)
	author, answerer, stranger := newDachnik(t, baseURL, 1), newDachnik(t, baseURL, 2), newDachnik(t, baseURL, 3)

	post, answer := qpSolvedWithAnswer(t, baseURL, author, answerer)

	for _, v := range []struct {
		name string
		who  dachnik
	}{
		{"автор решения", answerer},
		{"посторонний", stranger},
	} {
		t.Run(v.name, func(t *testing.T) {
			qpRequireAt(t, baseURL, v.who, post.ID, true, answer.ID, "по адресу")

			page := qpFeedOK(t, fetchFeed(t, baseURL, v.who.token, scopeParams("all", 50, "")), "вкладка all")
			item := qpFind(page, post.ID)
			if item == nil {
				t.Fatalf("вопроса %s нет во вкладке all", post.ID)
			}
			qpRequire(t, *item, true, true, answer.ID, "вкладка all")
		})
	}
}
