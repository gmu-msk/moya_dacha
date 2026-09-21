package tests

import (
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// Один и тот же номер в разных привычных видах: сервис хранит и сравнивает
// нормализованный вид (specs/001-auth.md, «Функциональные требования», п. 2).
const (
	phonePretty = "+7 (900) 123-45-67"
	phoneSpaced = "8 900 123 45 67"
	phoneDigits = "79001234567"
	phoneStored = "+79001234567"

	// Второй номер — для проверок, где нужны два разных пользователя.
	otherPhonePretty = "+7 (900) 555-77-88"
	otherPhoneStored = "+79005557788"
)

// Номера, которые в MVP не принимаются: не российские или не мобильные
// (specs/001-auth.md, «Функциональные требования», п. 3).
var notRussianMobilePhones = []struct {
	name  string
	phone string
}{
	{"городской", "+7 495 123-45-67"},
	{"третья цифра не девять", "+78001234567"},
	{"иностранный", "+1 202 555 0123"},
	{"мало цифр", "+7 900 123-45-6"},
	{"много цифр", "+7 900 123-45-678"},
	{"не номер вовсе", "телефон"},
}

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// requestCode запрашивает код на номер и требует, чтобы запрос был принят.
func requestCode(t *testing.T, baseURL, phone string) {
	t.Helper()

	resp := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phone})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("на запрос кода для %q ожидался статус 202, получен %d", phone, resp.StatusCode)
	}
}

// createSession пытается войти по номеру и коду и возвращает сырой ответ:
// проверяет его уже сам тест.
func createSession(t *testing.T, baseURL, phone, code string) *http.Response {
	t.Helper()
	return postJSON(t, baseURL+"/auth/session", map[string]any{"phone": phone, "code": code})
}

// signIn проходит вход целиком (код -> сессия) и возвращает токен
// и идентификатор пользователя.
func signIn(t *testing.T, baseURL, phone string) (token, userID string) {
	t.Helper()

	requestCode(t, baseURL, phone)

	resp := createSession(t, baseURL, phone, authCode)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на вход по номеру %q ожидался статус 200, получен %d", phone, resp.StatusCode)
	}

	var body struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	decode(t, resp, &body)

	if body.Token == "" {
		t.Fatalf("вход по номеру %q прошёл, но токен пустой", phone)
	}

	return body.Token, body.User.ID
}

// currentSession спрашивает «кто я» с этим токеном.
func currentSession(t *testing.T, baseURL, token string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, baseURL+"/auth/session", token, nil)
}

// signOut завершает сессию этого токена.
func signOut(t *testing.T, baseURL, token string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, baseURL+"/auth/session", token, nil)
}

// tooSoon требует, чтобы ответ был отказом «слишком рано», и возвращает
// остаток секунд из поля retry_after (схема AuthCodeTooSoon,
// specs/001-auth.md, «Функциональные требования», п. 5).
func tooSoon(t *testing.T, resp *http.Response) int {
	t.Helper()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("на повторный запрос кода ожидался статус 429, получен %d", resp.StatusCode)
	}

	var body struct {
		Code       string `json:"code"`
		Message    string `json:"message"`
		RetryAfter *int   `json:"retry_after"`
	}
	decode(t, resp, &body)

	if body.Code != "too_many_requests" {
		t.Fatalf("ожидалась ошибка too_many_requests, получена %q", body.Code)
	}
	if body.Message == "" {
		t.Errorf("в ошибке %q пустое сообщение для пользователя", body.Code)
	}
	if body.RetryAfter == nil {
		t.Fatalf("в отказе 429 нет поля retry_after: приложению не из чего считать счётчик")
	}
	if *body.RetryAfter < 1 {
		t.Fatalf("остаток retry_after=%d: меньше одной секунды он быть не может", *body.RetryAfter)
	}

	return *body.RetryAfter
}

// --- POST /api/auth/code: запрос кода ------------------------------------

// Запрос кода принимается, и ответ сообщает сроки по умолчанию из спеки:
// повтор через 60 секунд, код живёт 300 секунд.
func TestRequestCodeAcceptsPhoneAndReportsDefaultDeadlines(t *testing.T) {
	baseURL := startAPI(t)

	resp := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phonePretty})

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("ожидался статус 202, получен %d", resp.StatusCode)
	}

	var body struct {
		ResendAfter int `json:"resend_after"`
		CodeTTL     int `json:"code_ttl"`
	}
	decode(t, resp, &body)

	if body.ResendAfter != 60 {
		t.Errorf("ожидался resend_after=60, получен %d", body.ResendAfter)
	}
	if body.CodeTTL != 300 {
		t.Errorf("ожидался code_ttl=300, получен %d", body.CodeTTL)
	}
}

// Ответ сообщает те сроки, с которыми сервис работает на самом деле,
// а не константы из спеки (ФТ-15).
func TestRequestCodeReportsDeadlinesTheServiceActuallyWorksWith(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{
		CodeTTL:     90 * time.Second,
		ResendAfter: 30 * time.Second,
	})

	resp := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phonePretty})

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("ожидался статус 202, получен %d", resp.StatusCode)
	}

	var body struct {
		ResendAfter int `json:"resend_after"`
		CodeTTL     int `json:"code_ttl"`
	}
	decode(t, resp, &body)

	if body.ResendAfter != 30 {
		t.Errorf("сервис настроен на повтор через 30 секунд, а ответ сообщает resend_after=%d", body.ResendAfter)
	}
	if body.CodeTTL != 90 {
		t.Errorf("сервис настроен на код в 90 секунд, а ответ сообщает code_ttl=%d", body.CodeTTL)
	}
}

// Номер принимается в любом привычном виде: с пробелами, скобками,
// дефисами, с «+7», с «8» и вовсе без плюса.
func TestRequestCodeAcceptsPhoneWrittenInAnyUsualForm(t *testing.T) {
	forms := []string{phonePretty, phoneSpaced, phoneDigits, phoneStored, "+7 900 123-45-67"}

	for _, phone := range forms {
		t.Run(phone, func(t *testing.T) {
			baseURL := startAPI(t)

			resp := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phone})

			if resp.StatusCode != http.StatusAccepted {
				t.Fatalf("номер %q должен приниматься, ожидался статус 202, получен %d", phone, resp.StatusCode)
			}
		})
	}
}

// Не российский мобильный номер — 400 invalid_phone.
func TestRequestCodeRejectsPhoneThatIsNotARussianMobile(t *testing.T) {
	for _, testCase := range notRussianMobilePhones {
		t.Run(testCase.name, func(t *testing.T) {
			baseURL := startAPI(t)

			resp := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": testCase.phone})

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("на номер %q ожидался статус 400, получен %d", testCase.phone, resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "invalid_phone" {
				t.Fatalf("ожидалась ошибка invalid_phone, получена %q", code)
			}
		})
	}
}

// Пустой номер — это не «непонятный номер», а непригодное тело запроса:
// 400 invalid_request.
func TestRequestCodeRejectsEmptyPhone(t *testing.T) {
	baseURL := startAPI(t)

	resp := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": ""})

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "invalid_request" {
		t.Fatalf("ожидалась ошибка invalid_request, получена %q", code)
	}
}

// Тело запроса, которое не разбирается в объект с полем phone,
// — 400 invalid_request.
func TestRequestCodeRejectsMalformedBody(t *testing.T) {
	baseURL := startAPI(t)

	// Валидный JSON, но не объект запроса: поля phone в нём нет и быть не может.
	resp := postJSON(t, baseURL+"/auth/code", "это не объект запроса")

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "invalid_request" {
		t.Fatalf("ожидалась ошибка invalid_request, получена %q", code)
	}
}

// Второй запрос кода в пределах минуты — 429 too_many_requests,
// причём первый код остаётся живым. Номер во втором запросе записан
// иначе: ограничение считается по нормализованному номеру.
func TestRequestCodeTwiceWithinResendWindowIsRejected(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	resp := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phoneSpaced})

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("на повторный запрос кода ожидался статус 429, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "too_many_requests" {
		t.Fatalf("ожидалась ошибка too_many_requests, получена %q", code)
	}

	// Отказ во втором коде не должен гасить первый.
	signed := createSession(t, baseURL, phonePretty, authCode)
	if signed.StatusCode != http.StatusOK {
		t.Fatalf("первый код должен был остаться живым, но вход по нему дал %d", signed.StatusCode)
	}
}

// Отказ «слишком рано» сообщает остаток секунд: приложение рисует по нему
// живой счётчик, и считать ему больше не из чего (ФТ-5). Остаток — не меньше
// секунды и не больше окна повтора, настроенного в сервисе.
func TestRequestCodeTooSoonReportsSecondsLeft(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	// Номер записан иначе: остаток считается по нормализованному номеру.
	retryAfter := tooSoon(t, postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phoneSpaced}))

	if retryAfter > 60 {
		t.Errorf("окно повтора по умолчанию — 60 секунд, а ответ просит ждать %d", retryAfter)
	}
}

// Остаток считается по тому окну, с которым сервис работает на самом деле,
// и уменьшается с его ходом.
func TestRetryAfterCountsDownTheRealResendWindow(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 3 * time.Second})

	requestCode(t, baseURL, phonePretty)

	first := tooSoon(t, postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phonePretty}))
	if first > 3 {
		t.Fatalf("сервис настроен на окно в 3 секунды, а ответ просит ждать %d", first)
	}
	if first < 2 {
		t.Fatalf("окно из 3 секунд только началось, а ответ просит ждать всего %d", first)
	}

	time.Sleep(1200 * time.Millisecond)

	second := tooSoon(t, postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phonePretty}))
	if second >= first {
		t.Errorf("за секунду с лишним остаток должен был уменьшиться: было %d, стало %d", first, second)
	}
}

// Пока окно не закрылось, остаток не обнуляется: хвост меньше секунды
// сервис сообщает как одну секунду, иначе счётчик в приложении показывал бы
// «0» при живом ограничении.
func TestRetryAfterIsAtLeastOneSecondAtTheEndOfTheWindow(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 300 * time.Millisecond})

	requestCode(t, baseURL, phonePretty)

	retryAfter := tooSoon(t, postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phonePretty}))

	if retryAfter != 1 {
		t.Errorf("от окна в 300 миллисекунд остаётся меньше секунды, ожидался retry_after=1, получен %d", retryAfter)
	}
}

// После того как окно повтора прошло, код запрашивается снова.
func TestRequestCodeIsAllowedAgainAfterResendWindow(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 50 * time.Millisecond})

	requestCode(t, baseURL, phonePretty)
	time.Sleep(60 * time.Millisecond)

	resp := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": phonePretty})

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("после окна повтора ожидался статус 202, получен %d", resp.StatusCode)
	}
}

// Новый запрос кода отменяет предыдущий код: действует всегда только
// последний. Видно это по попыткам — сожжённый попытками код заменяется
// новым, по которому снова можно войти.
func TestNewCodeRequestReplacesThePreviousCode(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 50 * time.Millisecond})

	requestCode(t, baseURL, phonePretty)

	for attempt := 1; attempt <= 5; attempt++ {
		resp := createSession(t, baseURL, phonePretty, "0000")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("попытка %d с неверным кодом: ожидался статус 401, получен %d", attempt, resp.StatusCode)
		}
	}

	burned := createSession(t, baseURL, phonePretty, authCode)
	if code := errorCode(t, burned); code != "too_many_attempts" {
		t.Fatalf("первый код должен был сгореть по попыткам, получена ошибка %q", code)
	}

	time.Sleep(60 * time.Millisecond)
	requestCode(t, baseURL, phonePretty)

	resp := createSession(t, baseURL, phonePretty, authCode)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("по новому коду вход должен пройти, получен статус %d", resp.StatusCode)
	}
}

// Ограничение на повторный запрос кода считается по номеру: код на другой
// номер выдаётся сразу же.
func TestResendLimitIsCountedPerPhoneNumber(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	resp := postJSON(t, baseURL+"/auth/code", map[string]any{"phone": otherPhonePretty})

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("на другой номер код должен выдаваться сразу, ожидался статус 202, получен %d", resp.StatusCode)
	}
}

// --- POST /api/auth/session: вход по коду --------------------------------

// Верный код заводит пользователя, помечает вход как первый и возвращает
// токен сессии и самого пользователя с нормализованным номером.
func TestSignInWithValidCodeCreatesUserAndReturnsToken(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	resp := createSession(t, baseURL, phonePretty, authCode)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body struct {
		Token     string `json:"token"`
		IsNewUser bool   `json:"is_new_user"`
		User      struct {
			ID        string `json:"id"`
			Phone     string `json:"phone"`
			CreatedAt string `json:"created_at"`
		} `json:"user"`
	}
	decode(t, resp, &body)

	if body.Token == "" {
		t.Error("в ответе пустой токен сессии")
	}
	if !body.IsNewUser {
		t.Error("первый вход по новому номеру должен помечаться is_new_user=true")
	}
	if !uuidPattern.MatchString(body.User.ID) {
		t.Errorf("идентификатор пользователя должен быть UUID, получен %q", body.User.ID)
	}
	if body.User.Phone != phoneStored {
		t.Errorf("ожидался нормализованный номер %q, получен %q", phoneStored, body.User.Phone)
	}
	if _, err := time.Parse(time.RFC3339, body.User.CreatedAt); err != nil {
		t.Errorf("created_at %q не разбирается как время RFC3339: %v", body.User.CreatedAt, err)
	}
}

// Второй вход по известному номеру — это тот же пользователь,
// и первым он уже не помечается.
func TestSignInWithKnownPhoneIsNotMarkedAsNewUser(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 50 * time.Millisecond})

	_, firstUserID := signIn(t, baseURL, phonePretty)

	time.Sleep(60 * time.Millisecond)
	requestCode(t, baseURL, phonePretty)

	resp := createSession(t, baseURL, phonePretty, authCode)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body struct {
		IsNewUser bool `json:"is_new_user"`
		User      struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	decode(t, resp, &body)

	if body.IsNewUser {
		t.Error("повторный вход по тому же номеру не должен помечаться is_new_user=true")
	}
	if body.User.ID != firstUserID {
		t.Errorf("ожидался тот же пользователь %q, получен %q", firstUserID, body.User.ID)
	}
}

// Номер, начинающийся с «8», — тот же пользователь, что и с «+7».
func TestPhoneWithLeadingEightIsTheSameUserAsWithPlusSeven(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 50 * time.Millisecond})

	_, userIDViaEight := signIn(t, baseURL, phoneSpaced)

	time.Sleep(60 * time.Millisecond)
	requestCode(t, baseURL, phonePretty)

	resp := createSession(t, baseURL, phonePretty, authCode)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body struct {
		IsNewUser bool `json:"is_new_user"`
		User      struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	decode(t, resp, &body)

	if body.User.ID != userIDViaEight {
		t.Errorf("номер с «8» и с «+7» — один пользователь: ожидался %q, получен %q", userIDViaEight, body.User.ID)
	}
	if body.IsNewUser {
		t.Error("вход по тому же номеру в другой записи не должен заводить нового пользователя")
	}
}

// Код можно ввести, записав номер иначе, чем при запросе кода:
// сравнивается нормализованный вид.
func TestSignInAcceptsPhoneWrittenDifferentlyThanWhenCodeWasRequested(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	resp := createSession(t, baseURL, phoneDigits, authCode)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body struct {
		User struct {
			Phone string `json:"phone"`
		} `json:"user"`
	}
	decode(t, resp, &body)

	if body.User.Phone != phoneStored {
		t.Errorf("ожидался нормализованный номер %q, получен %q", phoneStored, body.User.Phone)
	}
}

// Неверный код — 401 invalid_code.
func TestSignInRejectsWrongCode(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	resp := createSession(t, baseURL, phonePretty, "0000")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "invalid_code" {
		t.Fatalf("ожидалась ошибка invalid_code, получена %q", code)
	}
}

// Неверный код объясняется пользователю словами: приложение показывает
// message как есть, поэтому текст — часть контракта этого экрана.
func TestSignInExplainsWrongCodeToTheUser(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	resp := createSession(t, baseURL, phonePretty, "0000")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
	}

	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decode(t, resp, &body)

	if body.Code != "invalid_code" {
		t.Fatalf("ожидалась ошибка invalid_code, получена %q", body.Code)
	}
	if body.Message != "Неверный код" {
		t.Errorf("пользователю показывается message: ожидалось «Неверный код», получено %q", body.Message)
	}
}

// Код, которого никто не запрашивал, — тоже invalid_code: по ответу нельзя
// узнать, есть ли по номеру живой код.
func TestSignInRejectsCodeThatWasNeverRequested(t *testing.T) {
	baseURL := startAPI(t)

	resp := createSession(t, baseURL, phonePretty, authCode)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "invalid_code" {
		t.Fatalf("ожидалась ошибка invalid_code, получена %q", code)
	}
}

// Код, введённый после срока жизни, — 401 code_expired.
func TestSignInRejectsExpiredCode(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{CodeTTL: 50 * time.Millisecond})

	requestCode(t, baseURL, phonePretty)
	time.Sleep(60 * time.Millisecond)

	resp := createSession(t, baseURL, phonePretty, authCode)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "code_expired" {
		t.Fatalf("ожидалась ошибка code_expired, получена %q", code)
	}
}

// На один код — пять попыток; шестая отвечает too_many_attempts, даже если
// код на этот раз верный: исчерпанные попытки сжигают код.
func TestSignInRejectsSixthAttemptWithTheSameCode(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	for attempt := 1; attempt <= 5; attempt++ {
		resp := createSession(t, baseURL, phonePretty, "0000")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("попытка %d: ожидался статус 401, получен %d", attempt, resp.StatusCode)
		}
		if code := errorCode(t, resp); code != "invalid_code" {
			t.Fatalf("попытка %d: ожидалась ошибка invalid_code, получена %q", attempt, code)
		}
	}

	resp := createSession(t, baseURL, phonePretty, authCode)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("на шестой попытке ожидался статус 401, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "too_many_attempts" {
		t.Fatalf("ожидалась ошибка too_many_attempts, получена %q", code)
	}
}

// Использованный код сгорает: второй раз тем же кодом войти нельзя.
func TestSignInRejectsCorrectCodeUsedTwice(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	first := createSession(t, baseURL, phonePretty, authCode)
	if first.StatusCode != http.StatusOK {
		t.Fatalf("первый вход должен был пройти, получен статус %d", first.StatusCode)
	}

	second := createSession(t, baseURL, phonePretty, authCode)

	if second.StatusCode != http.StatusUnauthorized {
		t.Fatalf("на повторный вход тем же кодом ожидался статус 401, получен %d", second.StatusCode)
	}
	if code := errorCode(t, second); code != "invalid_code" {
		t.Fatalf("ожидалась ошибка invalid_code, получена %q", code)
	}
}

// У ответа всегда одна причина: когда код и истёк, и исчерпал попытки,
// отвечается code_expired — срок жизни проверяется первым.
func TestExpiredCodeIsReportedBeforeExhaustedAttempts(t *testing.T) {
	// Срок жизни заведомо больше, чем занимают пять попыток подряд,
	// но достаточно короткий, чтобы дождаться его в тесте.
	baseURL := startAPIWith(t, api.Config{CodeTTL: 300 * time.Millisecond})

	requestCode(t, baseURL, phonePretty)

	for attempt := 1; attempt <= 5; attempt++ {
		resp := createSession(t, baseURL, phonePretty, "0000")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("попытка %d: ожидался статус 401, получен %d", attempt, resp.StatusCode)
		}
	}

	time.Sleep(350 * time.Millisecond)

	resp := createSession(t, baseURL, phonePretty, authCode)

	if code := errorCode(t, resp); code != "code_expired" {
		t.Fatalf("у истёкшего кода с исчерпанными попытками причина одна — code_expired, получена %q", code)
	}
}

// Пустой номер при входе — 400 invalid_request.
func TestSignInRejectsEmptyPhone(t *testing.T) {
	baseURL := startAPI(t)

	resp := createSession(t, baseURL, "", authCode)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "invalid_request" {
		t.Fatalf("ожидалась ошибка invalid_request, получена %q", code)
	}
}

// Пустой код при входе — 400 invalid_request.
func TestSignInRejectsEmptyCode(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	resp := createSession(t, baseURL, phonePretty, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "invalid_request" {
		t.Fatalf("ожидалась ошибка invalid_request, получена %q", code)
	}
}

// Вход по не российскому мобильному номеру — 400 invalid_phone.
func TestSignInRejectsPhoneThatIsNotARussianMobile(t *testing.T) {
	for _, testCase := range notRussianMobilePhones {
		t.Run(testCase.name, func(t *testing.T) {
			baseURL := startAPI(t)

			resp := createSession(t, baseURL, testCase.phone, authCode)

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("на номер %q ожидался статус 400, получен %d", testCase.phone, resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "invalid_phone" {
				t.Fatalf("ожидалась ошибка invalid_phone, получена %q", code)
			}
		})
	}
}

// Тело запроса на вход, которое не разбирается, — 400 invalid_request.
func TestSignInRejectsMalformedBody(t *testing.T) {
	baseURL := startAPI(t)

	resp := postJSON(t, baseURL+"/auth/session", "это не объект запроса")

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "invalid_request" {
		t.Fatalf("ожидалась ошибка invalid_request, получена %q", code)
	}
}

// --- GET /api/auth/session: кто я ----------------------------------------

// По токену видно, кто его владелец и когда он зарегистрировался.
func TestCurrentSessionReturnsOwnerOfTheToken(t *testing.T) {
	baseURL := startAPI(t)

	requestCode(t, baseURL, phonePretty)

	created := createSession(t, baseURL, phonePretty, authCode)
	if created.StatusCode != http.StatusOK {
		t.Fatalf("вход должен был пройти, получен статус %d", created.StatusCode)
	}

	var signedIn struct {
		Token string `json:"token"`
		User  struct {
			ID        string `json:"id"`
			Phone     string `json:"phone"`
			CreatedAt string `json:"created_at"`
		} `json:"user"`
	}
	decode(t, created, &signedIn)

	resp := currentSession(t, baseURL, signedIn.Token)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body struct {
		User struct {
			ID        string `json:"id"`
			Phone     string `json:"phone"`
			CreatedAt string `json:"created_at"`
		} `json:"user"`
	}
	decode(t, resp, &body)

	if body.User.ID != signedIn.User.ID {
		t.Errorf("ожидался пользователь %q, получен %q", signedIn.User.ID, body.User.ID)
	}
	if body.User.Phone != phoneStored {
		t.Errorf("ожидался нормализованный номер %q, получен %q", phoneStored, body.User.Phone)
	}
	if body.User.CreatedAt != signedIn.User.CreatedAt {
		t.Errorf("ожидалось created_at %q, получено %q", signedIn.User.CreatedAt, body.User.CreatedAt)
	}
}

// Запрос без заголовка Authorization — 401 unauthorized.
func TestCurrentSessionRejectsRequestWithoutAuthorization(t *testing.T) {
	baseURL := startAPI(t)

	resp := currentSession(t, baseURL, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "unauthorized" {
		t.Fatalf("ожидалась ошибка unauthorized, получена %q", code)
	}
}

// Чужой (несуществующий) токен — 401 unauthorized.
func TestCurrentSessionRejectsUnknownToken(t *testing.T) {
	baseURL := startAPI(t)

	resp := currentSession(t, baseURL, "токен-которого-не-выдавали")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "unauthorized" {
		t.Fatalf("ожидалась ошибка unauthorized, получена %q", code)
	}
}

// Заголовок Authorization без схемы Bearer — 401 unauthorized,
// даже если сам токен настоящий.
func TestCurrentSessionRejectsAuthorizationWithoutBearerScheme(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	req, err := http.NewRequest(http.MethodGet, baseURL+"/auth/session", nil)
	if err != nil {
		t.Fatalf("не удалось собрать запрос: %v", err)
	}
	req.Header.Set("Authorization", token) // без «Bearer »

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("запрос не прошёл: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "unauthorized" {
		t.Fatalf("ожидалась ошибка unauthorized, получена %q", code)
	}
}

// Номер телефона возвращается только владельцу сессии: по чужому токену
// приходит чужой (свой) номер, а не номер соседа.
func TestPhoneIsReturnedOnlyToTheOwnerOfTheSession(t *testing.T) {
	baseURL := startAPI(t)

	_, ownerID := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)

	if ownerID == otherID {
		t.Fatalf("два разных номера должны дать двух разных пользователей, получен один %q", ownerID)
	}

	resp := currentSession(t, baseURL, otherToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body struct {
		User struct {
			ID    string `json:"id"`
			Phone string `json:"phone"`
		} `json:"user"`
	}
	decode(t, resp, &body)

	if body.User.ID != otherID {
		t.Errorf("ожидался пользователь %q, получен %q", otherID, body.User.ID)
	}
	if body.User.Phone != otherPhoneStored {
		t.Errorf("ожидался свой номер %q, получен %q", otherPhoneStored, body.User.Phone)
	}
}

// Вход с двух устройств даёт две сессии: токены разные, работают оба,
// пользователь один и тот же.
func TestTwoDevicesGetIndependentWorkingTokens(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 50 * time.Millisecond})

	firstToken, userID := signIn(t, baseURL, phonePretty)

	time.Sleep(60 * time.Millisecond)
	secondToken, sameUserID := signIn(t, baseURL, phonePretty)

	if firstToken == secondToken {
		t.Fatal("вход с двух устройств должен выдавать разные токены")
	}
	if sameUserID != userID {
		t.Fatalf("оба входа — один пользователь: ожидался %q, получен %q", userID, sameUserID)
	}

	for name, token := range map[string]string{"первое устройство": firstToken, "второе устройство": secondToken} {
		resp := currentSession(t, baseURL, token)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: ожидался статус 200, получен %d", name, resp.StatusCode)
		}
	}
}

// --- DELETE /api/auth/session: выход -------------------------------------

// Выход прекращает действие токена: дальше по нему ничего не узнать.
func TestSignOutEndsTheSession(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	resp := signOut(t, baseURL, token)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("на выход ожидался статус 204, получен %d", resp.StatusCode)
	}

	after := currentSession(t, baseURL, token)
	if after.StatusCode != http.StatusUnauthorized {
		t.Fatalf("после выхода токен не должен работать, получен статус %d", after.StatusCode)
	}
	if code := errorCode(t, after); code != "unauthorized" {
		t.Fatalf("ожидалась ошибка unauthorized, получена %q", code)
	}
}

// Выход на одном устройстве не трогает сессию на другом.
func TestSignOutKeepsSessionsOnOtherDevicesAlive(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{ResendAfter: 50 * time.Millisecond})

	firstToken, _ := signIn(t, baseURL, phonePretty)

	time.Sleep(60 * time.Millisecond)
	secondToken, _ := signIn(t, baseURL, phonePretty)

	if resp := signOut(t, baseURL, firstToken); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("на выход ожидался статус 204, получен %d", resp.StatusCode)
	}

	if resp := currentSession(t, baseURL, firstToken); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("токен первого устройства должен был перестать действовать, получен статус %d", resp.StatusCode)
	}
	if resp := currentSession(t, baseURL, secondToken); resp.StatusCode != http.StatusOK {
		t.Errorf("второе устройство должно продолжать работать, получен статус %d", resp.StatusCode)
	}
}

// Повторный выход с тем же токеном — 401 unauthorized: токена уже нет.
func TestRepeatedSignOutWithTheSameTokenIsUnauthorized(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	if resp := signOut(t, baseURL, token); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("на первый выход ожидался статус 204, получен %d", resp.StatusCode)
	}

	resp := signOut(t, baseURL, token)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("на повторный выход ожидался статус 401, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "unauthorized" {
		t.Fatalf("ожидалась ошибка unauthorized, получена %q", code)
	}
}

// Выход без токена — 401 unauthorized.
func TestSignOutRejectsRequestWithoutToken(t *testing.T) {
	baseURL := startAPI(t)

	resp := signOut(t, baseURL, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
	}
	if code := errorCode(t, resp); code != "unauthorized" {
		t.Fatalf("ожидалась ошибка unauthorized, получена %q", code)
	}
}
