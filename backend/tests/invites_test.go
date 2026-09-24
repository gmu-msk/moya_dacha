package tests

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
	"github.com/gmu-msk/moya_dacha/backend/internal/auth"
)

// Вход по коду приглашения (specs/015-invites.md).
//
// Приглашения выдаёт владелец сервиса командой на сервере (ФТ-11); тесты
// выдают их той же функцией, что стоит за командой, — auth.IssueInvite.
// Всё остальное проверяется так же, как ходит приложение: по HTTP.

// startInvitesAPI поднимает сервис в режиме приглашений (AUTH_INVITES=1)
// и возвращает его адрес и пул к той же базе — через него выдаются
// приглашения. Пул берётся после старта: старт чистит базу.
//
// FixedCode задан нарочно: в режиме приглашений он действовать не должен
// (ФТ-2), и каждый тест здесь заодно проверяет, что стенд с AUTH_FIXED_CODE
// не пускает мимо приглашений.
func startInvitesAPI(t *testing.T) (string, *pgxpool.Pool) {
	t.Helper()

	baseURL := startAPIWith(t, api.Config{Invites: true, FixedCode: authCode})
	return baseURL, connect(t)
}

// issueInvite выдаёт приглашение на номер и возвращает код из него.
func issueInvite(t *testing.T, pool *pgxpool.Pool, phone string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, code, err := auth.IssueInvite(ctx, pool, phone)
	if err != nil {
		t.Fatalf("не удалось выдать приглашение на %q: %v", phone, err)
	}
	if len(code) != 4 {
		t.Fatalf("код приглашения должен быть из четырёх цифр, получен %q", code)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			t.Fatalf("код приглашения должен быть из четырёх цифр, получен %q", code)
		}
	}

	return code
}

// issueInviteNotEqual выдаёт приглашение, код которого отличается от
// каждого из avoid. Код случайный, и совпадение с фиксированным кодом стенда
// или с прежним кодом (1 из 10 000) сделало бы тест ложно красным.
func issueInviteNotEqual(t *testing.T, pool *pgxpool.Pool, phone string, avoid ...string) string {
	t.Helper()

	for range 20 {
		code := issueInvite(t, pool, phone)
		clash := false
		for _, a := range avoid {
			if code == a {
				clash = true
			}
		}
		if !clash {
			return code
		}
	}

	t.Fatalf("двадцать приглашений подряд выдали один из кодов %v: код не случайный", avoid)
	return ""
}

// revokeInvite отзывает приглашение на номер (команда uninvite, ФТ-11).
func revokeInvite(t *testing.T, pool *pgxpool.Pool, phone string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := auth.RevokeInvite(ctx, pool, phone); err != nil {
		t.Fatalf("не удалось отозвать приглашение на %q: %v", phone, err)
	}
}

// wrongCodeFor — заведомо неверный код для этого приглашения.
func wrongCodeFor(code string) string {
	if code == "0000" {
		return "1111"
	}
	return "0000"
}

// requireInviteDelivery требует ответ «код уже в приглашении»:
// 202, delivery=invite и нули в сроках (ФТ-7).
func requireInviteDelivery(t *testing.T, resp *http.Response) {
	t.Helper()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("на запрос кода по приглашению ожидался статус 202, получен %d", resp.StatusCode)
	}

	var body struct {
		Delivery    string `json:"delivery"`
		ResendAfter *int   `json:"resend_after"`
		CodeTTL     *int   `json:"code_ttl"`
	}
	decode(t, resp, &body)

	if body.Delivery != "invite" {
		t.Errorf("ожидался delivery=invite, получен %q", body.Delivery)
	}
	if body.ResendAfter == nil || *body.ResendAfter != 0 {
		t.Errorf("в режиме приглашений resend_after должен быть 0, получен %v", deref(body.ResendAfter))
	}
	if body.CodeTTL == nil || *body.CodeTTL != 0 {
		t.Errorf("в режиме приглашений code_ttl должен быть 0, получен %v", deref(body.CodeTTL))
	}
}

// deref показывает в сообщении отсутствующее поле как «нет поля».
func deref(v *int) any {
	if v == nil {
		return "нет поля"
	}
	return *v
}

// requireInviteError требует ошибку с этим статусом и машиночитаемым кодом.
func requireInviteError(t *testing.T, resp *http.Response, status int, code, what string) {
	t.Helper()

	if resp.StatusCode != status {
		t.Fatalf("%s: ожидался статус %d, получен %d", what, status, resp.StatusCode)
	}
	if got := errorCode(t, resp); got != code {
		t.Fatalf("%s: ожидалась ошибка %s, получена %q", what, code, got)
	}
}

// requestInviteCode запрашивает код в режиме приглашений и возвращает
// сырой ответ.
func requestInviteCode(t *testing.T, baseURL, phone string) *http.Response {
	t.Helper()
	return postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phone})
}

// sessionBody — то, что тесты читают из успешного входа.
type sessionBody struct {
	Token     string `json:"token"`
	IsNewUser bool   `json:"is_new_user"`
	User      struct {
		ID    string `json:"id"`
		Phone string `json:"phone"`
	} `json:"user"`
}

// requireSignedIn требует успешный вход и возвращает тело ответа.
func requireSignedIn(t *testing.T, resp *http.Response, what string) sessionBody {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: ожидался статус 200, получен %d", what, resp.StatusCode)
	}

	var body sessionBody
	decode(t, resp, &body)

	if body.Token == "" {
		t.Fatalf("%s: вход прошёл, но токен пустой", what)
	}

	return body
}

// --- Режим выключен ---------------------------------------------------------

// Без AUTH_INVITES всё как в 001-auth, а ответ на запрос кода сообщает,
// что код отправлен: delivery=sent и настоящие сроки (ФТ-8).
func TestRequestCodeWithoutInvitesModeReportsSentDelivery(t *testing.T) {
	baseURL := startAPI(t)

	resp := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phonePretty})

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("ожидался статус 202, получен %d", resp.StatusCode)
	}

	var body struct {
		Delivery    string `json:"delivery"`
		ResendAfter int    `json:"resend_after"`
		CodeTTL     int    `json:"code_ttl"`
	}
	decode(t, resp, &body)

	if body.Delivery != "sent" {
		t.Errorf("вне режима приглашений ожидался delivery=sent, получен %q", body.Delivery)
	}
	if body.ResendAfter != 60 {
		t.Errorf("ожидался resend_after=60, получен %d", body.ResendAfter)
	}
	if body.CodeTTL != 300 {
		t.Errorf("ожидался code_ttl=300, получен %d", body.CodeTTL)
	}
}

// Вне режима приглашений выданное приглашение ничего не меняет: запрос
// кода идёт как в 001-auth (с ограничением раз в минуту), а войти можно
// обычным кодом, но не кодом приглашения (ФТ-1).
func TestInviteIsIgnoredWithoutInvitesMode(t *testing.T) {
	baseURL := startAPI(t)
	pool := connect(t)
	inviteCode := issueInviteNotEqual(t, pool, phonePretty, authCode)

	first := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phonePretty})
	if first.StatusCode != http.StatusAccepted {
		t.Fatalf("на запрос кода ожидался статус 202, получен %d", first.StatusCode)
	}
	var body struct {
		Delivery string `json:"delivery"`
	}
	decode(t, first, &body)
	if body.Delivery != "sent" {
		t.Errorf("вне режима приглашений ожидался delivery=sent и при выданном приглашении, получен %q", body.Delivery)
	}

	tooSoon(t, postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phonePretty}))

	requireInviteError(t, createSession(t, baseURL, phonePretty, inviteCode),
		http.StatusUnauthorized, "invalid_code", "вход кодом приглашения вне режима приглашений")

	requireSignedIn(t, createSession(t, baseURL, phonePretty, authCode), "вход обычным кодом вне режима приглашений")
}

// --- Режим включён: запрос кода ---------------------------------------------

// На номер без приглашения код не выдаётся: 403 not_invited, и в ошибке
// есть текст — приложение показывает его под полем номера (ФТ-7, ФТ-14).
func TestRequestCodeWithoutInviteIsNotInvited(t *testing.T) {
	baseURL, _ := startInvitesAPI(t)

	requireInviteError(t, requestInviteCode(t, baseURL, phonePretty),
		http.StatusForbidden, "not_invited", "запрос кода на номер без приглашения")
}

// Вход по номеру без приглашения — 401 invalid_code, даже фиксированным
// кодом стенда.
func TestSignInWithoutInviteIsInvalidCode(t *testing.T) {
	baseURL, _ := startInvitesAPI(t)

	requireInviteError(t, createSession(t, baseURL, phonePretty, authCode),
		http.StatusUnauthorized, "invalid_code", "вход по номеру без приглашения")
}

// На номер с приглашением — 202, delivery=invite и нули в сроках.
func TestRequestCodeWithInviteReportsInviteDelivery(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	issueInvite(t, pool, phonePretty)

	requireInviteDelivery(t, requestInviteCode(t, baseURL, phonePretty))
}

// Запрос кода в режиме приглашений ничего не выдаёт и не отправляет:
// кодов подтверждения в auth_codes не появляется (ФТ-2).
func TestRequestCodeWithInviteIssuesNoConfirmationCode(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	issueInvite(t, pool, phonePretty)

	requireInviteDelivery(t, requestInviteCode(t, baseURL, phonePretty))

	if n := countSQL(t, `SELECT count(*) FROM auth_codes`); n != 0 {
		t.Errorf("в режиме приглашений коды подтверждения не выдаются, а в auth_codes %d строк", n)
	}
}

// Номер нормализуется и проверяется так же, как в 001-auth: неверный
// номер — 400 invalid_phone, а не not_invited (ФТ-7).
func TestRequestCodeInInvitesModeRejectsPhoneThatIsNotARussianMobile(t *testing.T) {
	for _, tc := range notRussianMobilePhones {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, _ := startInvitesAPI(t)

			requireInviteError(t, requestInviteCode(t, baseURL, tc.phone),
				http.StatusBadRequest, "invalid_phone", "запрос кода на номер "+tc.phone)
		})
	}
}

// Номер в приглашении и в запросе записан по-разному — это один номер:
// приглашение выдано на «8 900…», код запрошен как «+7 (900)…», вход —
// цифрами без плюса.
func TestInviteMatchesPhoneWrittenInAnotherForm(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	normalized, code, err := auth.IssueInvite(ctx, pool, phoneSpaced)
	if err != nil {
		t.Fatalf("не удалось выдать приглашение на %q: %v", phoneSpaced, err)
	}
	if normalized != phoneStored {
		t.Errorf("приглашение должно выдаваться на нормализованный номер %q, получен %q", phoneStored, normalized)
	}

	requireInviteDelivery(t, requestInviteCode(t, baseURL, phonePretty))

	body := requireSignedIn(t, createSession(t, baseURL, phoneDigits, code), "вход по номеру, записанному иначе, чем в приглашении")
	if body.User.Phone != phoneStored {
		t.Errorf("ожидался нормализованный номер %q, получен %q", phoneStored, body.User.Phone)
	}
}

// Запрос кода по приглашению дважды подряд — оба раза 202: ограничение
// «раз в минуту» в этом режиме не действует, 429 не бывает.
func TestRequestCodeWithInviteTwiceInARowIsAcceptedBothTimes(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	issueInvite(t, pool, phonePretty)

	requireInviteDelivery(t, requestInviteCode(t, baseURL, phonePretty))
	requireInviteDelivery(t, requestInviteCode(t, baseURL, phonePretty))
}

// --- Режим включён: вход ----------------------------------------------------

// Верный код приглашения впускает: новый пользователь заводится здесь же,
// токен рабочий (ФТ-10).
func TestSignInWithInviteCodeCreatesUserAndReturnsToken(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	code := issueInvite(t, pool, phonePretty)

	requireInviteDelivery(t, requestInviteCode(t, baseURL, phonePretty))

	body := requireSignedIn(t, createSession(t, baseURL, phonePretty, code), "вход верным кодом приглашения")

	if !body.IsNewUser {
		t.Error("первый вход по приглашению на новый номер должен помечаться is_new_user=true")
	}
	if !uuidPattern.MatchString(body.User.ID) {
		t.Errorf("идентификатор пользователя должен быть UUID, получен %q", body.User.ID)
	}
	if body.User.Phone != phoneStored {
		t.Errorf("ожидался нормализованный номер %q, получен %q", phoneStored, body.User.Phone)
	}

	if resp := currentSession(t, baseURL, body.Token); resp.StatusCode != http.StatusOK {
		t.Errorf("токен из входа по приглашению должен работать, «кто я» ответил %d", resp.StatusCode)
	}
}

// Вход по приглашению не требует предварительного запроса кода: код
// у человека уже есть, запрос кода ничего не выдаёт.
func TestSignInWithInviteCodeWorksWithoutRequestingCode(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	code := issueInvite(t, pool, phonePretty)

	requireSignedIn(t, createSession(t, baseURL, phonePretty, code), "вход по приглашению без запроса кода")
}

// Верный код сжигает приглашение: запрос кода после входа — 403 not_invited.
func TestSignInWithInviteCodeBurnsTheInvite(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	code := issueInvite(t, pool, phonePretty)

	requireSignedIn(t, createSession(t, baseURL, phonePretty, code), "вход верным кодом приглашения")

	requireInviteError(t, requestInviteCode(t, baseURL, phonePretty),
		http.StatusForbidden, "not_invited", "запрос кода после входа по приглашению")
}

// Тот же код второй раз — 401 invalid_code: приглашение сгорело.
func TestSignInWithInviteCodeUsedTwiceIsInvalidCode(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	code := issueInvite(t, pool, phonePretty)

	requireSignedIn(t, createSession(t, baseURL, phonePretty, code), "первый вход кодом приглашения")

	requireInviteError(t, createSession(t, baseURL, phonePretty, code),
		http.StatusUnauthorized, "invalid_code", "второй вход тем же кодом приглашения")
}

// Неверный код — 401 invalid_code, а приглашение остаётся действующим,
// пока попытки не кончились: запрос кода по-прежнему 202, верный код
// по-прежнему впускает.
func TestSignInWithWrongInviteCodeKeepsInviteWhileAttemptsLeft(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	code := issueInvite(t, pool, phonePretty)

	requireInviteError(t, createSession(t, baseURL, phonePretty, wrongCodeFor(code)),
		http.StatusUnauthorized, "invalid_code", "вход неверным кодом приглашения")

	requireInviteDelivery(t, requestInviteCode(t, baseURL, phonePretty))
	requireSignedIn(t, createSession(t, baseURL, phonePretty, code), "вход верным кодом после одной ошибки")
}

// Код подходит только к своему номеру: код из приглашения на один номер
// не впускает под другим.
func TestInviteCodeDoesNotWorkForAnotherPhone(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	code := issueInvite(t, pool, phonePretty)

	requireInviteError(t, createSession(t, baseURL, otherPhonePretty, code),
		http.StatusUnauthorized, "invalid_code", "вход чужим кодом приглашения")
	requireInviteError(t, requestInviteCode(t, baseURL, otherPhonePretty),
		http.StatusForbidden, "not_invited", "запрос кода на номер без своего приглашения")
}

// Неверный код пять раз, затем верный: шестая попытка — too_many_attempts
// (попытки проверяются раньше совпадения, ФТ-9), а запрос кода после
// этого — not_invited: исчерпанные попытки сжигают приглашение (ФТ-6).
func TestSixthAttemptOnInviteIsTooManyAttemptsAndBurnsIt(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	code := issueInvite(t, pool, phonePretty)
	wrong := wrongCodeFor(code)

	for attempt := 1; attempt <= 5; attempt++ {
		requireInviteError(t, createSession(t, baseURL, phonePretty, wrong),
			http.StatusUnauthorized, "invalid_code", "неверный код приглашения, попытка "+strconv.Itoa(attempt))
	}

	requireInviteError(t, createSession(t, baseURL, phonePretty, code),
		http.StatusUnauthorized, "too_many_attempts", "шестая попытка верным кодом")

	requireInviteError(t, requestInviteCode(t, baseURL, phonePretty),
		http.StatusForbidden, "not_invited", "запрос кода после исчерпанных попыток")
}

// У приглашения нет срока жизни, и code_expired в этом режиме не бывает:
// сервис с коротким CodeTTL всё равно впускает по старому приглашению (ФТ-5, ФТ-9).
func TestInviteDoesNotExpire(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{Invites: true, FixedCode: authCode, CodeTTL: 50 * time.Millisecond})
	pool := connect(t)
	code := issueInvite(t, pool, phonePretty)

	time.Sleep(100 * time.Millisecond)

	requireInviteDelivery(t, requestInviteCode(t, baseURL, phonePretty))
	requireSignedIn(t, createSession(t, baseURL, phonePretty, code), "вход по приглашению старше CodeTTL")
}

// --- Выдача заново и отзыв --------------------------------------------------

// Приглашение выдано заново: старый код — invalid_code, новый подходит (ФТ-4).
func TestReissuedInviteReplacesTheOldCode(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	oldCode := issueInvite(t, pool, phonePretty)
	newCode := issueInviteNotEqual(t, pool, phonePretty, oldCode)

	requireInviteError(t, createSession(t, baseURL, phonePretty, oldCode),
		http.StatusUnauthorized, "invalid_code", "вход старым кодом после новой выдачи")

	requireSignedIn(t, createSession(t, baseURL, phonePretty, newCode), "вход новым кодом")
}

// Новое приглашение возвращает попытки: после сгоревшего приглашения
// выдано новое, и по нему снова пять попыток — четыре ошибки подряд
// и верный код на пятой впускают.
func TestReissuedInviteStartsAttemptsOver(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	oldCode := issueInvite(t, pool, phonePretty)
	oldWrong := wrongCodeFor(oldCode)

	for attempt := 1; attempt <= 5; attempt++ {
		createSession(t, baseURL, phonePretty, oldWrong)
	}
	requireInviteError(t, requestInviteCode(t, baseURL, phonePretty),
		http.StatusForbidden, "not_invited", "запрос кода после исчерпанных попыток")

	newCode := issueInvite(t, pool, phonePretty)
	newWrong := wrongCodeFor(newCode)

	requireInviteDelivery(t, requestInviteCode(t, baseURL, phonePretty))

	for attempt := 1; attempt <= 4; attempt++ {
		requireInviteError(t, createSession(t, baseURL, phonePretty, newWrong),
			http.StatusUnauthorized, "invalid_code", "неверный код нового приглашения, попытка "+strconv.Itoa(attempt))
	}

	requireSignedIn(t, createSession(t, baseURL, phonePretty, newCode), "пятая попытка по новому приглашению верным кодом")
}

// Отозванное приглашение: запрос кода — not_invited, вход его кодом —
// invalid_code.
func TestRevokedInviteNoLongerWorks(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	code := issueInvite(t, pool, phonePretty)

	revokeInvite(t, pool, phoneSpaced)

	requireInviteError(t, requestInviteCode(t, baseURL, phonePretty),
		http.StatusForbidden, "not_invited", "запрос кода после отзыва приглашения")
	requireInviteError(t, createSession(t, baseURL, phonePretty, code),
		http.StatusUnauthorized, "invalid_code", "вход кодом отозванного приглашения")
}

// --- Уже зарегистрированный номер ---------------------------------------------

// Приглашение на уже зарегистрированный номер впускает в тот же аккаунт,
// и первым вход не помечается (ФТ-12): так человек входит заново после
// выхода или с нового телефона.
func TestInviteForRegisteredPhoneSignsIntoTheSameAccount(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)

	first := requireSignedIn(t, createSession(t, baseURL, phonePretty, issueInvite(t, pool, phonePretty)),
		"первый вход по приглашению")
	if resp := signOut(t, baseURL, first.Token); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("выход должен был пройти, получен статус %d", resp.StatusCode)
	}

	code := issueInvite(t, pool, phonePretty)
	requireInviteDelivery(t, requestInviteCode(t, baseURL, phonePretty))

	second := requireSignedIn(t, createSession(t, baseURL, phonePretty, code), "вход по новому приглашению на тот же номер")

	if second.IsNewUser {
		t.Error("вход по приглашению на зарегистрированный номер не должен помечаться is_new_user=true")
	}
	if second.User.ID != first.User.ID {
		t.Errorf("ожидался тот же пользователь %q, получен %q", first.User.ID, second.User.ID)
	}
}

// --- AUTH_FIXED_CODE ----------------------------------------------------------

// Режим включён и задан AUTH_FIXED_CODE: фиксированный код не подходит
// ни при приглашении, ни без него — действуют только приглашения (ФТ-2).
func TestFixedCodeDoesNotWorkInInvitesMode(t *testing.T) {
	baseURL, pool := startInvitesAPI(t)
	code := issueInviteNotEqual(t, pool, phonePretty, authCode)

	requireInviteError(t, createSession(t, baseURL, phonePretty, authCode),
		http.StatusUnauthorized, "invalid_code", "вход фиксированным кодом при выданном приглашении")

	requireInviteError(t, requestInviteCode(t, baseURL, otherPhonePretty),
		http.StatusForbidden, "not_invited", "запрос кода без приглашения при заданном AUTH_FIXED_CODE")
	requireInviteError(t, createSession(t, baseURL, otherPhonePretty, authCode),
		http.StatusUnauthorized, "invalid_code", "вход фиксированным кодом без приглашения")

	requireSignedIn(t, createSession(t, baseURL, phonePretty, code), "вход кодом приглашения после попытки фиксированным")
}
