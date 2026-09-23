package tests

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// Тесты никнейма (specs/010-nicknames.md). Каждый тест — строка таблицы
// «Ограничения и edge cases» или функциональное требование, на котором
// она держится.

// temporaryNicknamePattern — временный никнейм: dachnik_ и десять
// шестнадцатеричных цифр (ФТ-4).
var temporaryNicknamePattern = regexp.MustCompile(`^dachnik_[0-9a-f]{10}$`)

// Никнейм из примера спецификации и тот же никнейм в других регистрах:
// уникальность — без учёта регистра (ФТ-3).
const (
	nickname      = "Valya_dacha"
	nicknameLower = "valya_dacha"
	nicknameUpper = "VALYA_DACHA"
)

// userBeforeNicknamesID — идентификатор пользователя, заведённого в базе
// в обход сервиса, как будто до миграции с никнеймами.
const userBeforeNicknamesID = "5d2c9e10-7a4b-4c3d-8e2f-1a0b9c8d7e6f"

// --- Представления из контракта -------------------------------------------

// currentUserPayload — пользователь, каким его видит он сам (schema
// CurrentUser): его возвращают вход, GET /me, GET /auth/session и все
// операции профиля.
type currentUserPayload struct {
	ID             string  `json:"id"`
	Phone          string  `json:"phone"`
	CreatedAt      string  `json:"created_at"`
	Nickname       string  `json:"nickname"`
	NicknameChosen bool    `json:"nickname_chosen"`
	Name           string  `json:"name"`
	About          string  `json:"about"`
	AvatarURL      *string `json:"avatar_url"`
}

// --- Хелперы --------------------------------------------------------------

// setNickname выбирает или меняет никнейм. body передаётся как есть:
// тестам про непригодное тело нужен не объект, а что угодно.
func setNickname(t *testing.T, baseURL, token string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/me/nickname", token, body)
}

// setNicknameRaw отправляет тело запроса как есть, не собирая его из
// объекта: телу, которое вовсе не JSON, объекта в Go не соответствует.
func setNicknameRaw(t *testing.T, baseURL, token, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPut, baseURL+"/me/nickname", strings.NewReader(body))
	if err != nil {
		t.Fatalf("не удалось собрать запрос на никнейм: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("запрос на никнейм не прошёл: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// currentUserOK требует успешного ответа и возвращает пользователя,
// попутно проверяя, что оба поля никнейма в ответе есть на самом деле:
// nickname_chosen: false — это поле со значением false, а не его
// отсутствие (schema CurrentUser).
func currentUserOK(t *testing.T, resp *http.Response, where string) currentUserPayload {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: ожидался статус 200, получен %d", where, resp.StatusCode)
	}

	return currentUserFrom(t, rawJSON(t, resp), where)
}

// currentUserFrom разбирает пользователя из сырого JSON и требует, чтобы
// nickname был строкой, а nickname_chosen — логическим значением.
func currentUserFrom(t *testing.T, raw []byte, where string) currentUserPayload {
	t.Helper()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("%s: ответ не разобрался как JSON: %v", where, err)
	}

	var nick string
	if err := json.Unmarshal(fields["nickname"], &nick); err != nil {
		t.Errorf("%s: поле nickname должно быть строкой, получено %s", where, fields["nickname"])
	}
	var chosen bool
	if err := json.Unmarshal(fields["nickname_chosen"], &chosen); err != nil {
		t.Errorf("%s: поле nickname_chosen должно быть true или false, получено %s", where, fields["nickname_chosen"])
	}

	var user currentUserPayload
	if err := json.Unmarshal(raw, &user); err != nil {
		t.Fatalf("%s: пользователь не разобрался: %v", where, err)
	}

	return user
}

// me — свой профиль по GET /api/me.
func me(t *testing.T, baseURL, token string) currentUserPayload {
	t.Helper()
	return currentUserOK(t, getProfile(t, baseURL, token), "GET /me")
}

// sessionUser — пользователь по GET /api/auth/session.
func sessionUser(t *testing.T, baseURL, token string) currentUserPayload {
	t.Helper()

	resp := currentSession(t, baseURL, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /auth/session: ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body struct {
		User json.RawMessage `json:"user"`
	}
	if err := json.Unmarshal(rawJSON(t, resp), &body); err != nil {
		t.Fatalf("GET /auth/session: ответ не разобрался как JSON: %v", err)
	}

	return currentUserFrom(t, body.User, "GET /auth/session")
}

// chooseNickname выбирает никнейм и требует, чтобы он сохранился.
func chooseNickname(t *testing.T, baseURL, token, nick string) currentUserPayload {
	t.Helper()
	return currentUserOK(t, setNickname(t, baseURL, token, map[string]any{"nickname": nick}), "PUT /me/nickname "+nick)
}

// requireTemporaryNickname требует временный никнейм, который пользователь
// ещё не выбирал сам (ФТ-4, ФТ-5).
func requireTemporaryNickname(t *testing.T, user currentUserPayload, where string) {
	t.Helper()

	if !temporaryNicknamePattern.MatchString(user.Nickname) {
		t.Errorf("%s: ожидался временный никнейм вида dachnik_ и 10 hex-цифр, получен %q", where, user.Nickname)
	}
	if user.NicknameChosen {
		t.Errorf("%s: никнейм пользователь не выбирал, а nickname_chosen = true", where)
	}
}

// requireNickname требует выбранный никнейм ровно в этом написании.
func requireNickname(t *testing.T, user currentUserPayload, want, where string) {
	t.Helper()

	if user.Nickname != want {
		t.Errorf("%s: ожидался никнейм %q, получен %q", where, want, user.Nickname)
	}
	if !user.NicknameChosen {
		t.Errorf("%s: никнейм выбран, а nickname_chosen = false", where)
	}
}

// requireNicknameUnchanged требует, чтобы отказ не тронул никнейм: он тот же
// и так же выбран или не выбран, как до запроса.
func requireNicknameUnchanged(t *testing.T, baseURL, token string, before currentUserPayload, where string) {
	t.Helper()

	after := me(t, baseURL, token)
	if after.Nickname != before.Nickname {
		t.Errorf("%s: никнейм не должен был измениться: был %q, стал %q", where, before.Nickname, after.Nickname)
	}
	if after.NicknameChosen != before.NicknameChosen {
		t.Errorf("%s: nickname_chosen не должен был измениться: был %v, стал %v", where, before.NicknameChosen, after.NicknameChosen)
	}
}

// feedItem находит пост на первой странице ленты.
func feedItem(t *testing.T, baseURL, token, postID string) postPayload {
	t.Helper()

	page := feedPage(t, baseURL, token, nil)
	for _, item := range page.Items {
		if item.ID == postID {
			return item
		}
	}

	t.Fatalf("поста %s нет в ленте, а он опубликован", postID)

	return postPayload{}
}

// requireNicknameEverywhere требует, чтобы сосед видел у автора этот
// никнейм везде, где автора подписывают: в ленте, на экране поста,
// у комментария и на странице пользователя (ФТ-8).
func requireNicknameEverywhere(t *testing.T, baseURL, viewerToken, authorID, postID, want, when string) {
	t.Helper()

	if got := feedItem(t, baseURL, viewerToken, postID).Author; got.ID != authorID || got.Nickname != want {
		t.Errorf("%s, лента: ожидался автор %s с никнеймом %q, получен %s с %q", when, authorID, want, got.ID, got.Nickname)
	}

	if got := fetchedPost(t, fetchPost(t, baseURL, viewerToken, postID)).Author; got.ID != authorID || got.Nickname != want {
		t.Errorf("%s, экран поста: ожидался автор %s с никнеймом %q, получен %s с %q", when, authorID, want, got.ID, got.Nickname)
	}

	comments := commentsOf(t, baseURL, viewerToken, postID).Items
	if len(comments) == 0 {
		t.Fatalf("%s: у поста нет комментариев, а автор его прокомментировал", when)
	}
	for _, comment := range comments {
		if comment.Author.ID != authorID || comment.Author.Nickname != want {
			t.Errorf("%s, комментарий: ожидался автор %s с никнеймом %q, получен %s с %q",
				when, authorID, want, comment.Author.ID, comment.Author.Nickname)
		}
	}

	if got := userProfile(t, baseURL, viewerToken, authorID); got.Nickname != want {
		t.Errorf("%s, страница пользователя: ожидался никнейм %q, получен %q", when, want, got.Nickname)
	}
}

// --- Временный никнейм ----------------------------------------------------

// Новый пользователь сразу после входа: никнейм вида dachnik_ и десять
// hex-цифр, nickname_chosen: false — и в ответе входа, и в GET /me, и в
// GET /auth/session, и везде один и тот же («Новый пользователь после
// входа», ФТ-1, ФТ-4, ФТ-5).
func TestNewUserHasTemporaryNicknameNotChosenYet(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)
	resp := createSession(t, baseURL, phonePretty, authCode)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на вход ожидался статус 200, получен %d", resp.StatusCode)
	}

	var session struct {
		Token     string          `json:"token"`
		IsNewUser bool            `json:"is_new_user"`
		User      json.RawMessage `json:"user"`
	}
	if err := json.Unmarshal(rawJSON(t, resp), &session); err != nil {
		t.Fatalf("ответ входа не разобрался как JSON: %v", err)
	}
	if !session.IsNewUser {
		t.Fatal("первый вход по номеру должен помечаться is_new_user=true")
	}

	fromSignIn := currentUserFrom(t, session.User, "ответ входа")
	requireTemporaryNickname(t, fromSignIn, "ответ входа")

	fromMe := me(t, baseURL, session.Token)
	requireTemporaryNickname(t, fromMe, "GET /me")

	fromSession := sessionUser(t, baseURL, session.Token)
	requireTemporaryNickname(t, fromSession, "GET /auth/session")

	if fromMe.Nickname != fromSignIn.Nickname || fromSession.Nickname != fromSignIn.Nickname {
		t.Errorf("временный никнейм должен быть один и тот же: вход %q, /me %q, /auth/session %q",
			fromSignIn.Nickname, fromMe.Nickname, fromSession.Nickname)
	}
}

// Повторный вход не выдаёт нового временного никнейма: он у пользователя
// уже есть (ФТ-1).
func TestTemporaryNicknameSurvivesSignInAgain(t *testing.T) {
	baseURL := startAPI(t)

	firstToken, _ := signIn(t, baseURL, phonePretty)
	first := me(t, baseURL, firstToken)

	secondToken, _ := signIn(t, baseURL, phonePretty)
	second := me(t, baseURL, secondToken)

	requireTemporaryNickname(t, second, "после повторного входа")
	if second.Nickname != first.Nickname {
		t.Errorf("повторный вход сменил временный никнейм: был %q, стал %q", first.Nickname, second.Nickname)
	}
}

// Два (и три) новых пользователя: временные никнеймы разные, в том числе
// без учёта регистра («Два новых пользователя», ФТ-4).
func TestNewUsersGetDifferentTemporaryNicknames(t *testing.T) {
	baseURL := startAPI(t)

	seen := map[string]string{}
	for _, phone := range []string{phonePretty, otherPhonePretty, thirdPhonePretty} {
		token, _ := signIn(t, baseURL, phone)
		user := me(t, baseURL, token)
		requireTemporaryNickname(t, user, "новый пользователь "+phone)

		key := strings.ToLower(user.Nickname)
		if other, ok := seen[key]; ok {
			t.Errorf("у %s и %s один и тот же временный никнейм %q", other, phone, user.Nickname)
		}
		seen[key] = phone
	}
}

// Пользователь, заведённый до появления никнеймов, тоже с временным
// никнеймом: его выдаёт база значением по умолчанию («Модель данных»,
// ФТ-1, «Если пользователь зарегистрировался до появления никнеймов»).
func TestUserCreatedWithoutNicknameGetsTemporaryOneFromDatabase(t *testing.T) {
	baseURL := startAPI(t)

	execSQL(t, `INSERT INTO users (id, phone, created_at) VALUES ($1, $2, now())`,
		userBeforeNicknamesID, phoneStored)

	token, userID := signIn(t, baseURL, phonePretty)
	if userID != userBeforeNicknamesID {
		t.Fatalf("вход по номеру должен был найти заведённого пользователя %s, получен %s", userBeforeNicknamesID, userID)
	}

	old := me(t, baseURL, token)
	requireTemporaryNickname(t, old, "пользователь до никнеймов")

	newToken, _ := signIn(t, baseURL, otherPhonePretty)
	fresh := me(t, baseURL, newToken)
	if strings.EqualFold(fresh.Nickname, old.Nickname) {
		t.Errorf("у старого и нового пользователя один временный никнейм %q", old.Nickname)
	}
}

// --- PUT /api/me/nickname: выбор никнейма ---------------------------------

// Никнейм Valya_dacha: 200, сохраняется как введён, nickname_chosen: true;
// ответ — профиль целиком, и то же видно в GET /me и GET /auth/session
// («Никнейм Valya_dacha», ФТ-3, ФТ-6).
func TestChooseNicknameSavesItAsEnteredAndMarksItChosen(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	updated := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("имя и «о себе» должны сохраняться, получен статус %d", updated.StatusCode)
	}
	before := me(t, baseURL, token)

	user := chooseNickname(t, baseURL, token, nickname)
	requireNickname(t, user, nickname, "ответ PUT /me/nickname")

	if user.ID != userID {
		t.Errorf("ожидался пользователь %q, получен %q", userID, user.ID)
	}
	if user.Phone != phoneStored {
		t.Errorf("ответ — профиль целиком: ожидался номер %q, получен %q", phoneStored, user.Phone)
	}
	if user.CreatedAt != before.CreatedAt {
		t.Errorf("дата регистрации не должна меняться: была %q, стала %q", before.CreatedAt, user.CreatedAt)
	}
	if user.Name != profileName || user.About != profileAbout {
		t.Errorf("никнейм не трогает имя и «о себе»: ожидалось %q / %q, получено %q / %q",
			profileName, profileAbout, user.Name, user.About)
	}

	requireNickname(t, me(t, baseURL, token), nickname, "GET /me после выбора")
	requireNickname(t, sessionUser(t, baseURL, token), nickname, "GET /auth/session после выбора")
}

// Никнейм хранится и показывается так, как его ввели: смешанный регистр
// не приводится ни к нижнему, ни к верхнему (ФТ-3).
func TestChooseNicknameKeepsLetterCase(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	requireNickname(t, chooseNickname(t, baseURL, token, "VaLyA_DaCha"), "VaLyA_DaCha", "ответ")
	requireNickname(t, me(t, baseURL, token), "VaLyA_DaCha", "GET /me")
}

// Пробелы по краям обрезаются, сохраняется обрезанное; длина считается
// после обрезки («Никнейм с пробелами по краям», ФТ-2).
func TestChooseNicknameTrimsSpacesAroundIt(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"пробелы с обеих сторон", "  " + nickname + "  ", nickname},
		{"пробел в начале", " " + nickname, nickname},
		{"пробел в конце", nickname + " ", nickname},
		{"двадцать символов в пробелах", "   " + strings.Repeat("z", 20) + "   ", strings.Repeat("z", 20)},
		{"три символа в пробелах", " abc ", "abc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			requireNickname(t, chooseNickname(t, baseURL, token, tc.input), tc.want, "ответ")
			requireNickname(t, me(t, baseURL, token), tc.want, "GET /me")
		})
	}
}

// Ровно 3 и ровно 20 символов принимаются: латиница, цифры и _ в любом
// сочетании («Ровно 3 и ровно 20 символов», ФТ-2).
func TestChooseNicknameAcceptsThreeAndTwentyCharacters(t *testing.T) {
	nicknames := map[string]string{
		"три буквы":           "abc",
		"три с цифрой и _":    "a_1",
		"три цифры":           "123",
		"двадцать букв":       strings.Repeat("Q", 20),
		"двадцать вперемешку": "Valya_dacha_2026_ogo",
	}

	for caseName, nick := range nicknames {
		t.Run(caseName, func(t *testing.T) {
			if n := len([]rune(nick)); n != 3 && n != 20 {
				t.Fatalf("в тесте ошибка: %q длиной %d, а не 3 или 20", nick, n)
			}

			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			requireNickname(t, chooseNickname(t, baseURL, token, nick), nick, "ответ")
		})
	}
}

// Отказы в никнейме по правилам (ФТ-2): каждый — 400 invalid_nickname,
// и никнейм остаётся прежним, временным и невыбранным.
func requireNicknameRejected(t *testing.T, nicknames map[string]string) {
	t.Helper()

	for caseName, nick := range nicknames {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)
			before := me(t, baseURL, token)

			resp := setNickname(t, baseURL, token, map[string]any{"nickname": nick})
			requireError(t, resp, http.StatusBadRequest, "invalid_nickname")

			requireNicknameUnchanged(t, baseURL, token, before, "после отказа в "+caseName)
		})
	}
}

// 2 символа или 21 символ — 400 invalid_nickname; длина считается после
// обрезки пробелов («2 символа или 21 символ»).
func TestChooseNicknameRejectsTooShortOrTooLong(t *testing.T) {
	requireNicknameRejected(t, map[string]string{
		"один символ":              "a",
		"два символа":              "ab",
		"два символа в пробелах":   "  ab  ",
		"двадцать один символ":     strings.Repeat("a", 21),
		"двадцать один вперемешку": "Valya_dacha_2026_ogo1",
	})
}

// Кириллица, пробел внутри, дефис, точка, @ — 400 invalid_nickname
// («Кириллица, пробел внутри, дефис, точка, @», «Вне рамок»: кириллицы
// в никнейме нет).
func TestChooseNicknameRejectsCharactersOtherThanLatinDigitsAndUnderscore(t *testing.T) {
	requireNicknameRejected(t, map[string]string{
		"кириллица": "Валя_дача",
		"кириллическая а среди латиницы": "Vаlya_dacha",
		"пробел внутри":                  "Valya dacha",
		"дефис":                          "Valya-dacha",
		"точка":                          "Valya.dacha",
		"собака":                         "Valya@dacha",
		"собака в начале":                "@Valya_dacha",
		"табуляция внутри":               "Valya\tdacha",
	})
}

// Пустой никнейм или из одних пробелов — 400 invalid_nickname, а не
// invalid_request: поле есть и это строка («Пустой никнейм или из одних
// пробелов»).
func TestChooseNicknameRejectsEmptyOrBlank(t *testing.T) {
	requireNicknameRejected(t, map[string]string{
		"пустой":       "",
		"один пробел":  " ",
		"одни пробелы": "      ",
	})
}

// Нет поля nickname или оно null — то же, что пустой никнейм:
// 400 invalid_nickname («Нет поля nickname или оно null»).
func TestChooseNicknameWithoutNicknameIsInvalidNickname(t *testing.T) {
	bodies := map[string]string{
		"пустой объект": `{}`,
		"другое поле":   `{"name": "Valya_dacha"}`,
		"null":          `{"nickname": null}`,
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)
			before := me(t, baseURL, token)

			resp := setNicknameRaw(t, baseURL, token, body)
			requireError(t, resp, http.StatusBadRequest, "invalid_nickname")

			requireNicknameUnchanged(t, baseURL, token, before, "после тела "+caseName)
		})
	}
}

// nickname не строка или тело не разбирается — 400 invalid_request
// («nickname не строка, тело не JSON», «Ошибки»).
func TestChooseNicknameRequiresNicknameAsString(t *testing.T) {
	bodies := map[string]string{
		"число":      `{"nickname": 42}`,
		"логическое": `{"nickname": true}`,
		"список":     `{"nickname": ["Valya_dacha"]}`,
		"объект":     `{"nickname": {"value": "Valya_dacha"}}`,
		"строка вместо объекта": `"Valya_dacha"`,
		"не JSON":         `nickname=Valya_dacha`,
		"оборванный JSON": `{"nickname": "Valya_dacha"`,
		"пустое тело":     ``,
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)
			before := me(t, baseURL, token)

			resp := setNicknameRaw(t, baseURL, token, body)
			requireError(t, resp, http.StatusBadRequest, "invalid_request")

			requireNicknameUnchanged(t, baseURL, token, before, "после тела "+caseName)
		})
	}
}

// --- Уникальность ---------------------------------------------------------

// valya при чужом Valya — 409 nickname_taken в любом регистре, и никнейм
// не меняется ни у того, кто просил, ни у владельца («valya при чужом
// Valya», ФТ-3, ФТ-6).
func TestChooseNicknameTakenByAnotherUserInAnyCase(t *testing.T) {
	for _, attempt := range []string{"Valya", "valya", "VALYA", "vALYA", "  valya  "} {
		t.Run(attempt, func(t *testing.T) {
			baseURL := startAPI(t)

			ownerToken, _ := signIn(t, baseURL, phonePretty)
			chooseNickname(t, baseURL, ownerToken, "Valya")

			otherToken, _ := signIn(t, baseURL, otherPhonePretty)
			before := me(t, baseURL, otherToken)

			resp := setNickname(t, baseURL, otherToken, map[string]any{"nickname": attempt})
			requireError(t, resp, http.StatusConflict, "nickname_taken")

			requireNicknameUnchanged(t, baseURL, otherToken, before, "у того, кому отказали")
			requireTemporaryNickname(t, me(t, baseURL, otherToken), "у того, кому отказали")
			requireNickname(t, me(t, baseURL, ownerToken), "Valya", "у владельца")
		})
	}
}

// Отказ из-за занятости не трогает и уже выбранный никнейм: он остаётся
// прежним и выбранным (ФТ-6).
func TestTakenNicknameKeepsAlreadyChosenOne(t *testing.T) {
	baseURL := startAPI(t)

	ownerToken, _ := signIn(t, baseURL, phonePretty)
	chooseNickname(t, baseURL, ownerToken, "Valya")

	otherToken, _ := signIn(t, baseURL, otherPhonePretty)
	chooseNickname(t, baseURL, otherToken, "Kolya_58")

	resp := setNickname(t, baseURL, otherToken, map[string]any{"nickname": "valya"})
	requireError(t, resp, http.StatusConflict, "nickname_taken")

	requireNickname(t, me(t, baseURL, otherToken), "Kolya_58", "у того, кому отказали")
}

// Чужой временный никнейм занят так же, как выбранный, и тоже без учёта
// регистра («Чужой временный никнейм», ФТ-4).
func TestChooseTemporaryNicknameOfAnotherUserIsTaken(t *testing.T) {
	baseURL := startAPI(t)

	ownerToken, _ := signIn(t, baseURL, phonePretty)
	temporary := me(t, baseURL, ownerToken).Nickname

	otherToken, _ := signIn(t, baseURL, otherPhonePretty)
	before := me(t, baseURL, otherToken)

	for _, attempt := range []string{temporary, strings.ToUpper(temporary)} {
		resp := setNickname(t, baseURL, otherToken, map[string]any{"nickname": attempt})
		requireError(t, resp, http.StatusConflict, "nickname_taken")
	}

	requireNicknameUnchanged(t, baseURL, otherToken, before, "у того, кому отказали")

	owner := me(t, baseURL, ownerToken)
	if owner.Nickname != temporary {
		t.Errorf("у владельца временный никнейм должен остаться %q, получен %q", temporary, owner.Nickname)
	}
}

// Свой же никнейм в другом регистре — 200, сохраняется новое написание,
// и чужим он от этого не становится («Свой же никнейм в другом регистре»,
// ФТ-6).
func TestChooseOwnNicknameInAnotherCaseSavesNewSpelling(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	chooseNickname(t, baseURL, token, nicknameLower)

	requireNickname(t, chooseNickname(t, baseURL, token, nickname), nickname, "ответ")
	requireNickname(t, me(t, baseURL, token), nickname, "GET /me")

	requireNickname(t, chooseNickname(t, baseURL, token, nicknameUpper), nicknameUpper, "ответ")
	requireNickname(t, me(t, baseURL, token), nicknameUpper, "GET /me")

	// Прежнее написание — тот же никнейм, он по-прежнему занят.
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)
	resp := setNickname(t, baseURL, otherToken, map[string]any{"nickname": nicknameLower})
	requireError(t, resp, http.StatusConflict, "nickname_taken")
}

// Свой же никнейм без изменений — 200, nickname_chosen: true. Это верно
// и для выбранного, и для временного: оставить временный — тоже выбор
// («Свой же никнейм без изменений», ФТ-6).
func TestChooseSameNicknameAgainIsNotAnError(t *testing.T) {
	t.Run("выбранный", func(t *testing.T) {
		baseURL := startAPI(t)

		token, _ := signIn(t, baseURL, phonePretty)
		chooseNickname(t, baseURL, token, nickname)

		requireNickname(t, chooseNickname(t, baseURL, token, nickname), nickname, "повторный выбор")
		requireNickname(t, me(t, baseURL, token), nickname, "GET /me")
	})

	t.Run("временный", func(t *testing.T) {
		baseURL := startAPI(t)

		token, _ := signIn(t, baseURL, phonePretty)
		temporary := me(t, baseURL, token).Nickname

		requireNickname(t, chooseNickname(t, baseURL, token, temporary), temporary, "временный как выбранный")
		requireNickname(t, me(t, baseURL, token), temporary, "GET /me")
	})
}

// После смены прежний никнейм свободен: его может взять другой — и
// выбранный, и временный («Смена никнейма»).
func TestChangedNicknameIsFreedForOthers(t *testing.T) {
	t.Run("выбранный", func(t *testing.T) {
		baseURL := startAPI(t)

		ownerToken, _ := signIn(t, baseURL, phonePretty)
		chooseNickname(t, baseURL, ownerToken, "Valya")
		requireNickname(t, chooseNickname(t, baseURL, ownerToken, "Valentina"), "Valentina", "смена")

		otherToken, _ := signIn(t, baseURL, otherPhonePretty)
		requireNickname(t, chooseNickname(t, baseURL, otherToken, "Valya"), "Valya", "прежний никнейм соседки")

		// А новый никнейм первой по-прежнему её.
		resp := setNickname(t, baseURL, otherToken, map[string]any{"nickname": "valentina"})
		requireError(t, resp, http.StatusConflict, "nickname_taken")
	})

	t.Run("временный", func(t *testing.T) {
		baseURL := startAPI(t)

		ownerToken, _ := signIn(t, baseURL, phonePretty)
		temporary := me(t, baseURL, ownerToken).Nickname
		chooseNickname(t, baseURL, ownerToken, nickname)

		otherToken, _ := signIn(t, baseURL, otherPhonePretty)
		requireNickname(t, chooseNickname(t, baseURL, otherToken, temporary), temporary, "прежний временный никнейм соседки")
	})
}

// --- Никнейм там, где пользователя подписывают ----------------------------

// Сосед видит у автора никнейм в ленте, на экране поста, у комментария
// и на странице пользователя: сначала временный, а после смены — сразу
// новый, и после второй смены — снова новый («Никнейм в ленте, у
// комментария, в профиле соседа», ФТ-8, «Если пользователь
// зарегистрировался до появления никнеймов»).
func TestNewNicknameIsShownEverywhereRightAfterChange(t *testing.T) {
	baseURL := startAPI(t)

	authorToken, authorID := signIn(t, baseURL, phonePretty)
	post := publishPost(t, baseURL, authorToken, postCaption)
	commentOf(t, baseURL, authorToken, post.ID, commentText)

	viewerToken, _ := signIn(t, baseURL, otherPhonePretty)

	temporary := me(t, baseURL, authorToken).Nickname
	requireNicknameEverywhere(t, baseURL, viewerToken, authorID, post.ID, temporary, "до выбора никнейма")

	chooseNickname(t, baseURL, authorToken, nickname)
	requireNicknameEverywhere(t, baseURL, viewerToken, authorID, post.ID, nickname, "после выбора никнейма")

	chooseNickname(t, baseURL, authorToken, "Valentina_58")
	requireNicknameEverywhere(t, baseURL, viewerToken, authorID, post.ID, "Valentina_58", "после второй смены")
}

// Автор в ответах на создание поста и комментария тоже подписан
// никнеймом (ФТ-8, schema Author).
func TestAuthorOfNewPostAndCommentCarriesNickname(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	chooseNickname(t, baseURL, token, nickname)

	post := publishPost(t, baseURL, token, postCaption)
	if post.Author.ID != userID || post.Author.Nickname != nickname {
		t.Errorf("новый пост: ожидался автор %s с никнеймом %q, получен %s с %q", userID, nickname, post.Author.ID, post.Author.Nickname)
	}

	comment := commentOf(t, baseURL, token, post.ID, commentText)
	if comment.Author.ID != userID || comment.Author.Nickname != nickname {
		t.Errorf("новый комментарий: ожидался автор %s с никнеймом %q, получен %s с %q", userID, nickname, comment.Author.ID, comment.Author.Nickname)
	}
}

// --- PUT /api/me: полное имя ----------------------------------------------

// Полное имя пустое — 200, имя очищается, никнейм при этом не меняется;
// автор с пустым именем подписан никнеймом («Полное имя пустое в
// PUT /api/me», ФТ-7).
func TestEmptyFullNameIsClearedAndNicknameStays(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	chooseNickname(t, baseURL, token, nickname)
	introduce(t, baseURL, token, profileName)

	user := currentUserOK(t, updateProfile(t, baseURL, token, map[string]any{"name": "", "about": profileAbout}), "PUT /me")
	if user.Name != "" {
		t.Errorf("ожидалось пустое имя, получено %q", user.Name)
	}
	requireNickname(t, user, nickname, "ответ PUT /me")
	requireNickname(t, me(t, baseURL, token), nickname, "GET /me после PUT /me")

	viewerToken, _ := signIn(t, baseURL, otherPhonePretty)
	neighbour := userProfile(t, baseURL, viewerToken, userID)
	if neighbour.Name != "" || neighbour.Nickname != nickname {
		t.Errorf("сосед должен видеть никнейм %q и пустое имя, видит %q и %q", nickname, neighbour.Nickname, neighbour.Name)
	}
}

// PUT /api/me не меняет никнейм и не отмечает его выбранным: у нового
// пользователя после знакомства по имени никнейм так и остаётся
// временным (ФТ-5, «PUT /api/me»).
func TestUpdateProfileDoesNotChooseNickname(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	before := me(t, baseURL, token)

	user := currentUserOK(t, updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout}), "PUT /me")
	requireTemporaryNickname(t, user, "ответ PUT /me")
	if user.Nickname != before.Nickname {
		t.Errorf("PUT /me сменил никнейм: был %q, стал %q", before.Nickname, user.Nickname)
	}
	requireNicknameUnchanged(t, baseURL, token, before, "после PUT /me")
}

// Поле nickname в теле PUT /api/me никнейм не меняет: никнейм меняется
// только отдельной операцией. Лишнее поле сервис может проигнорировать
// или отклонить — важно, что никнейм остался прежним («PUT /api/me»).
func TestUpdateProfileIgnoresNicknameInBody(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	chooseNickname(t, baseURL, token, nickname)
	before := me(t, baseURL, token)

	updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": "", "nickname": "Hacker"})

	requireNicknameUnchanged(t, baseURL, token, before, "после PUT /me с полем nickname")
}

// Операции с аватаром возвращают профиль целиком — с никнеймом
// и nickname_chosen («Что добавляется в ответы»).
func TestAvatarOperationsReturnNickname(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	chooseNickname(t, baseURL, token, nickname)

	set := currentUserOK(t, putAvatar(t, baseURL, token, "avatar.png", imageBytes(t, "png", 100, 100)), "PUT /me/avatar")
	requireNickname(t, set, nickname, "PUT /me/avatar")

	removed := currentUserOK(t, deleteAvatar(t, baseURL, token), "DELETE /me/avatar")
	requireNickname(t, removed, nickname, "DELETE /me/avatar")
}

// --- Доступ ---------------------------------------------------------------

// Запрос без токена или с неизвестным токеном — 401 unauthorized,
// и чужой никнейм им не занять («Запрос без токена»).
func TestChooseNicknameWithoutTokenIsUnauthorized(t *testing.T) {
	tokens := map[string]string{
		"без токена":            "",
		"с неизвестным токеном": "не-токен-вовсе",
	}

	for caseName, token := range tokens {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			resp := setNickname(t, baseURL, token, map[string]any{"nickname": nickname})
			requireError(t, resp, http.StatusUnauthorized, "unauthorized")

			// Никнейм никому не достался: его свободно берёт первый вошедший.
			userToken, _ := signIn(t, baseURL, phonePretty)
			requireNickname(t, chooseNickname(t, baseURL, userToken, nickname), nickname, "после запроса без токена")
		})
	}
}

// После выхода токен недействителен и для смены никнейма (ФТ-6,
// specs/001-auth.md).
func TestChooseNicknameAfterSignOutIsUnauthorized(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	if resp := signOut(t, baseURL, token); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("выход должен был пройти, получен статус %d", resp.StatusCode)
	}

	resp := setNickname(t, baseURL, token, map[string]any{"nickname": nickname})
	requireError(t, resp, http.StatusUnauthorized, "unauthorized")
}
