package tests

// Тесты push-уведомлений (specs/024-push.md). Написаны по спецификации
// и контракту, не глядя в реализацию (ADR-0002).
//
// Ручка PUT /api/me/push-token проверяется по HTTP, как её зовёт телефон.
// Фоновая отправка — не ручка, а клиент FCM HTTP v1: тест запускает её
// через Go-API пакета push (New, Run) на той же базе, а Google подменяет
// фейковым сервером на httptest. Тот выдаёт OAuth-токен по token_uri из
// ключа сервисного аккаунта и записывает, что пришло на messages:send.

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
	"github.com/gmu-msk/moya_dacha/backend/internal/push"
)

const (
	// fcmProject — project_id в ключе сервисного аккаунта.
	fcmProject = "moya-dacha-test"
	// fcmAccessToken — OAuth-токен, который выдаёт фейковый token_uri.
	fcmAccessToken = "fake-access"
	// fcmScope — scope JWT сервисного аккаунта («Что уходит в FCM»).
	fcmScope = "https://www.googleapis.com/auth/firebase.messaging"
	// kindFollowRequest — вид пуша о заявке (требование 8).
	kindFollowRequest = "follow_request"
	// pushTokenLimit — длина токена телефона («API / контракт данных»).
	pushTokenLimit = 4096

	// pushDelay — PUSH_DELAY в тестах: успеть отменить действие
	// и не ждать долго.
	pushDelay = 500 * time.Millisecond
	// pushWait — сколько тест ждёт пуш, который должен прийти.
	pushWait = 5 * time.Second
)

// Ответы FCM на токен (требование 13).
const (
	fcmOK           = ""
	fcmUnregistered = "unregistered"
	fcmInvalid      = "invalid"
	fcmUnavailable  = "unavailable"
)

// fcmMessage — одно сообщение, принятое фейковым FCM.
type fcmMessage struct {
	Token    string
	Title    string
	Body     string
	Data     map[string]string
	Priority string
	Tag      string
	HasTag   bool
	Raw      string
}

// fakeFCM — фейковый Google: token_uri и FCM HTTP v1.
type fakeFCM struct {
	t   *testing.T
	srv *httptest.Server
	key *rsa.PrivateKey

	mu          sync.Mutex
	sent        []fcmMessage   // принятые с успехом
	attempts    map[string]int // запросы на messages:send по токену телефона
	modes       map[string]string
	tokenCalls  int
	problems    []string
	credentials []byte
}

func newFakeFCM(t *testing.T) *fakeFCM {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("не удалось создать RSA-ключ: %v", err)
	}

	f := &fakeFCM{
		t:        t,
		key:      key,
		attempts: map[string]int{},
		modes:    map[string]string{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", f.serveToken)
	mux.HandleFunc("/v1/", f.serveSend)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("не удалось сериализовать ключ: %v", err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	f.credentials, err = json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     fcmProject,
		"private_key_id": "test-key-id",
		"private_key":    string(pemKey),
		"client_email":   "push@" + fcmProject + ".iam.gserviceaccount.com",
		"client_id":      "1234567890",
		"token_uri":      f.tokenURI(),
	})
	if err != nil {
		t.Fatalf("не удалось собрать ключ сервисного аккаунта: %v", err)
	}

	// Проверка идёт после остановки отправки: t.Cleanup — в обратном порядке.
	t.Cleanup(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, p := range f.problems {
			t.Errorf("фейковый FCM: %s", p)
		}
	})

	return f
}

func (f *fakeFCM) URL() string      { return f.srv.URL }
func (f *fakeFCM) tokenURI() string { return f.srv.URL + "/token" }

func (f *fakeFCM) problem(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.problems = append(f.problems, fmt.Sprintf(format, args...))
}

// setMode задаёт, чем FCM отвечает на этот токен телефона.
func (f *fakeFCM) setMode(token, mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.modes[token] = mode
}

// serveToken — OAuth: обмен JWT сервисного аккаунта на access_token.
func (f *fakeFCM) serveToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		f.problem("token_uri: метод %s, ожидался POST", r.Method)
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		f.problem("token_uri: форма не разобралась: %v", err)
		http.Error(w, "form", http.StatusBadRequest)
		return
	}
	if got := r.PostForm.Get("grant_type"); got != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
		f.problem("token_uri: grant_type = %q", got)
	}
	if err := f.checkJWT(r.PostForm.Get("assertion")); err != nil {
		f.problem("token_uri: assertion: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"bad assertion"}`))
		return
	}

	f.mu.Lock()
	f.tokenCalls++
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"access_token":"` + fcmAccessToken + `","expires_in":3600,"token_type":"Bearer"}`))
}

// checkJWT проверяет JWT сервисного аккаунта: RS256, подпись ключом
// аккаунта, scope FCM и aud = token_uri.
func (f *fakeFCM) checkJWT(assertion string) error {
	parts := strings.Split(assertion, ".")
	if len(parts) != 3 {
		return fmt.Errorf("не три части через точку: %q", assertion)
	}

	var header struct {
		Alg string `json:"alg"`
	}
	if err := decodeJWTPart(parts[0], &header); err != nil {
		return fmt.Errorf("заголовок: %v", err)
	}
	if header.Alg != "RS256" {
		return fmt.Errorf("alg = %q, ожидался RS256", header.Alg)
	}

	var claims struct {
		Iss   string `json:"iss"`
		Scope string `json:"scope"`
		Aud   string `json:"aud"`
		Iat   int64  `json:"iat"`
		Exp   int64  `json:"exp"`
	}
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return fmt.Errorf("payload: %v", err)
	}
	if !contains(strings.Fields(claims.Scope), fcmScope) {
		return fmt.Errorf("scope = %q, нет %s", claims.Scope, fcmScope)
	}
	if claims.Aud != f.tokenURI() {
		return fmt.Errorf("aud = %q, ожидался token_uri %q", claims.Aud, f.tokenURI())
	}
	if claims.Iss == "" {
		return fmt.Errorf("пустой iss")
	}
	if claims.Exp <= claims.Iat {
		return fmt.Errorf("exp %d не позже iat %d", claims.Exp, claims.Iat)
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("подпись не base64url: %v", err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&f.key.PublicKey, crypto.SHA256, digest[:], sig); err != nil {
		return fmt.Errorf("подпись не сходится с ключом аккаунта: %v", err)
	}

	return nil
}

func decodeJWTPart(part string, target any) error {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

// serveSend — POST /v1/projects/<project>/messages:send.
func (f *fakeFCM) serveSend(w http.ResponseWriter, r *http.Request) {
	want := "/v1/projects/" + fcmProject + "/messages:send"
	if r.Method != http.MethodPost || r.URL.Path != want {
		f.problem("запрос %s %s, ожидался POST %s", r.Method, r.URL.Path, want)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+fcmAccessToken {
		f.problem("Authorization = %q, ожидался Bearer %s", got, fcmAccessToken)
		fcmError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "")
		return
	}

	var body struct {
		Message struct {
			Token        string `json:"token"`
			Notification *struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"notification"`
			Data    map[string]any `json:"data"`
			Android *struct {
				Priority     string `json:"priority"`
				Notification *struct {
					Tag *string `json:"tag"`
				} `json:"notification"`
			} `json:"android"`
		} `json:"message"`
	}
	raw := new(strings.Builder)
	if err := json.NewDecoder(teeBody(r, raw)).Decode(&body); err != nil {
		f.problem("тело messages:send не JSON: %v", err)
		fcmError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "")
		return
	}

	m := fcmMessage{Token: body.Message.Token, Data: map[string]string{}, Raw: raw.String()}
	if n := body.Message.Notification; n != nil {
		m.Title, m.Body = n.Title, n.Body
	}
	for k, v := range body.Message.Data {
		s, ok := v.(string)
		if !ok {
			f.problem("data.%s = %v: значения data в FCM — только строки", k, v)
		}
		m.Data[k] = s
	}
	if a := body.Message.Android; a != nil {
		m.Priority = a.Priority
		if a.Notification != nil && a.Notification.Tag != nil {
			m.Tag, m.HasTag = *a.Notification.Tag, true
		}
	}

	f.mu.Lock()
	f.attempts[m.Token]++
	mode := f.modes[m.Token]
	if mode == fcmOK {
		f.sent = append(f.sent, m)
	}
	f.mu.Unlock()

	switch mode {
	case fcmUnregistered:
		fcmError(w, http.StatusNotFound, "NOT_FOUND", "UNREGISTERED")
	case fcmInvalid:
		fcmError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "INVALID_ARGUMENT")
	case fcmUnavailable:
		fcmError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "UNAVAILABLE")
	default:
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"projects/` + fcmProject + `/messages/1"}`))
	}
}

// teeBody читает тело запроса, попутно копируя его в raw.
func teeBody(r *http.Request, raw *strings.Builder) *strings.Reader {
	buf := new(strings.Builder)
	b := make([]byte, 4096)
	for {
		n, err := r.Body.Read(b)
		buf.Write(b[:n])
		if err != nil {
			break
		}
	}
	raw.WriteString(buf.String())
	return strings.NewReader(buf.String())
}

// fcmError — ответ FCM HTTP v1 с ошибкой.
func fcmError(w http.ResponseWriter, code int, status, errorCode string) {
	body := map[string]any{"code": code, "status": status, "message": "Requested entity was not found."}
	if errorCode != "" {
		body["details"] = []map[string]string{{
			"@type":     "type.googleapis.com/google.firebase.fcm.v1.FcmError",
			"errorCode": errorCode,
		}}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": body})
}

// sentTo — пуши, принятые FCM на токен телефона.
func (f *fakeFCM) sentTo(token string) []fcmMessage {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []fcmMessage
	for _, m := range f.sent {
		if m.Token == token {
			out = append(out, m)
		}
	}
	return out
}

func (f *fakeFCM) attemptsTo(token string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts[token]
}

func (f *fakeFCM) totalRequests() (sends, tokens int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, n := range f.attempts {
		sends += n
	}
	return sends, f.tokenCalls
}

// waitSent ждёт, пока на токен придёт n пушей, и возвращает их.
func (f *fakeFCM) waitSent(token string, n int, what string) []fcmMessage {
	f.t.Helper()

	deadline := time.Now().Add(pushWait)
	for {
		got := f.sentTo(token)
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			f.t.Fatalf("%s: за %v на токен %q пришло %d пушей из %d ожидаемых: %s",
				what, pushWait, token, len(got), n, describePushes(got))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitAttempts ждёт, пока на токен будет n запросов (с любым ответом).
func (f *fakeFCM) waitAttempts(token string, n int, what string) {
	f.t.Helper()

	deadline := time.Now().Add(pushWait)
	for f.attemptsTo(token) < n {
		if time.Now().After(deadline) {
			f.t.Fatalf("%s: за %v на токен %q было %d запросов из %d ожидаемых",
				what, pushWait, token, f.attemptsTo(token), n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func describePushes(list []fcmMessage) string {
	if len(list) == 0 {
		return "ничего"
	}
	var b strings.Builder
	for i, m := range list {
		fmt.Fprintf(&b, "\n  %d. %s — %s %v", i+1, m.Title, m.Body, m.Data)
	}
	return b.String()
}

// pushKinds — data.kind пушей по порядку.
func pushKinds(list []fcmMessage) []string {
	out := make([]string, 0, len(list))
	for _, m := range list {
		out = append(out, m.Data["kind"])
	}
	return out
}

// requirePushes требует, чтобы на токен пришли пуши ровно этих видов.
func requirePushes(t *testing.T, fake *fakeFCM, token string, kinds []string, where string) {
	t.Helper()

	got := fake.sentTo(token)
	if strings.Join(pushKinds(got), ",") != strings.Join(kinds, ",") {
		t.Errorf("%s: на токен %q пришли пуши %v, ожидались %v: %s",
			where, token, pushKinds(got), kinds, describePushes(got))
	}
}

// --- Запуск отправки ------------------------------------------------------

// pushConfig — настройки отправки в фейковый FCM.
func pushConfig(fake *fakeFCM, delay time.Duration) push.Config {
	return push.Config{
		Credentials: fake.credentials,
		APIURL:      fake.URL(),
		Delay:       delay,
		Interval:    20 * time.Millisecond,
	}
}

// runSender запускает Run в горутине; отмена — в t.Cleanup, до того как
// закроется фейковый FCM и пул.
func runSender(t *testing.T, pool *pgxpool.Pool, cfg push.Config) {
	t.Helper()

	sender, err := push.New(pool, cfg)
	if err != nil {
		t.Fatalf("push.New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		sender.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Run не вернулся за 5 секунд после отмены ctx")
		}
	})
}

// startPush поднимает сервис на чистой базе, фейковый FCM и отправку
// с задержкой delay.
func startPush(t *testing.T, delay time.Duration) (string, *fakeFCM) {
	t.Helper()
	return startPushWith(t, api.Config{}, delay)
}

func startPushWith(t *testing.T, cfg api.Config, delay time.Duration) (string, *fakeFCM) {
	t.Helper()

	fake := newFakeFCM(t)
	baseURL := startAPIWith(t, cfg)
	runSender(t, connect(t), pushConfig(fake, delay))

	return baseURL, fake
}

// putPushToken отдаёт токен телефона сервису; body — как есть.
func putPushToken(t *testing.T, baseURL, token string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/me/push-token", token, body)
}

// registerPhone запоминает токен телефона за сессией и требует 204.
func registerPhone(t *testing.T, baseURL, session, phoneToken string) {
	t.Helper()
	requireNoContent(t, putPushToken(t, baseURL, session, map[string]any{"token": phoneToken}),
		"PUT /me/push-token "+phoneToken)
}

// quiet — выждать заметно дольше PUSH_DELAY, чтобы пуш, если бы он был,
// успел уйти.
func quiet(delay time.Duration) {
	time.Sleep(delay + 600*time.Millisecond)
}

// pushQueueLen — строк в очереди пушей (push_queue, «Модель данных»).
func pushQueueLen(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM push_queue`).Scan(&n); err != nil {
		t.Fatalf("не удалось посчитать push_queue: %v", err)
	}
	return n
}

// --- PUT /api/me/push-token -----------------------------------------------

// Без сессии токен не принимается.
func TestPushTokenRequiresAuth(t *testing.T) {
	baseURL := startAPI(t)

	requireError(t, putPushToken(t, baseURL, "", map[string]any{"token": "phone-1"}),
		http.StatusUnauthorized, "unauthorized")
	requireError(t, putPushToken(t, baseURL, "не-токен-сессии", map[string]any{"token": "phone-1"}),
		http.StatusUnauthorized, "unauthorized")
}

// Токен принимается, повторный PUT того же токена — тоже 204 («Повторный
// PUT с тем же токеном ничего не меняет, кроме времени»); токен в 4096
// символов — на границе, принимается.
func TestPushTokenAccepted(t *testing.T) {
	baseURL := startAPI(t)
	a := newDachnik(t, baseURL, 1)

	registerPhone(t, baseURL, a.token, "phone-1")
	registerPhone(t, baseURL, a.token, "phone-1")
	registerPhone(t, baseURL, a.token, strings.Repeat("x", pushTokenLimit))
}

// Нет токена, пустой, из пробелов, длиннее 4096 — 400 invalid_request.
func TestPushTokenValidation(t *testing.T) {
	baseURL := startAPI(t)
	a := newDachnik(t, baseURL, 1)

	cases := []struct {
		name string
		body any
	}{
		{"без тела", nil},
		{"без поля token", map[string]any{}},
		{"token null", map[string]any{"token": nil}},
		{"пустой", map[string]any{"token": ""}},
		{"из пробелов", map[string]any{"token": "   \t "}},
		{"длиннее 4096", map[string]any{"token": strings.Repeat("x", pushTokenLimit+1)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			requireError(t, putPushToken(t, baseURL, a.token, c.body), http.StatusBadRequest, "invalid_request")
		})
	}
}

// Пробелы по краям токена обрезаются: пуш уходит на обрезанный токен.
func TestPushTokenTrimmed(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 2)
	masha, kolya := people[0], people[1]

	registerPhone(t, baseURL, masha.token, "  phone-trim \n")
	followOK(t, baseURL, kolya, masha.id)

	fake.waitSent("phone-trim", 1, "пуш на обрезанный токен")
	sends, _ := fake.totalRequests()
	if sends != 1 {
		t.Errorf("запросов на messages:send %d, ожидался 1 — на обрезанный токен", sends)
	}
}

// --- Что уходит в FCM ------------------------------------------------------

// requirePush проверяет общий вид пуша (требование 11, «Что уходит в FCM»).
func requirePush(t *testing.T, m fcmMessage, kind, title, body, actorID, postID string) {
	t.Helper()

	where := "пуш " + kind
	if m.Title != title {
		t.Errorf("%s: title = %q, ожидался ник %q", where, m.Title, title)
	}
	if m.Body != body {
		t.Errorf("%s: body = %q, ожидался %q", where, m.Body, body)
	}
	if m.Data["kind"] != kind {
		t.Errorf("%s: data.kind = %q", where, m.Data["kind"])
	}
	if m.Data["user_id"] != actorID {
		t.Errorf("%s: data.user_id = %q, ожидался %q", where, m.Data["user_id"], actorID)
	}
	if postID != "" {
		if m.Data["post_id"] != postID {
			t.Errorf("%s: data.post_id = %q, ожидался %q", where, m.Data["post_id"], postID)
		}
	} else if v, ok := m.Data["post_id"]; ok {
		t.Errorf("%s: data.post_id = %q, а у этого вида его нет", where, v)
	}
	if m.Priority != "high" {
		t.Errorf("%s: android.priority = %q, ожидался high", where, m.Priority)
	}
	if kind == kindLike {
		if want := "like:" + postID; m.Tag != want {
			t.Errorf("%s: android.notification.tag = %q, ожидался %q (требование 12)", where, m.Tag, want)
		}
	} else if m.HasTag && m.Tag != "" {
		t.Errorf("%s: android.notification.tag = %q, а он только у like (требование 12)", where, m.Tag)
	}
	if t.Failed() {
		t.Logf("%s: тело запроса %s", where, m.Raw)
	}
}

// Отметка поста: заголовок — ник, текст «отметил ваш пост» (требование 11),
// tag like:<пост> (требование 12). Пуш уходит не раньше PUSH_DELAY
// (требование 9).
func TestPushLike(t *testing.T) {
	delay := 700 * time.Millisecond
	baseURL, fake := startPush(t, delay)
	people := newDachniks(t, baseURL, 1, 2)
	masha, kolya := people[0], people[1]
	chooseNickname(t, baseURL, kolya.token, "kolya_dacha")
	post := ownPost(t, baseURL, masha)
	registerPhone(t, baseURL, masha.token, "masha-phone")

	liked := time.Now()
	likeOK(t, baseURL, kolya, post.ID)

	got := fake.waitSent("masha-phone", 1, "пуш об отметке")
	if elapsed := time.Since(liked); elapsed < delay-50*time.Millisecond {
		t.Errorf("пуш ушёл через %v, а PUSH_DELAY = %v (требование 9)", elapsed, delay)
	}
	requirePush(t, got[0], kindLike, "kolya_dacha", "отметил ваш пост", kolya.id, post.ID)
}

// Комментарий: «ответил на ваш пост: «текст»» (требование 11).
func TestPushComment(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 2)
	masha, kolya := people[0], people[1]
	chooseNickname(t, baseURL, kolya.token, "kolya_dacha")
	post := ownPost(t, baseURL, masha)
	registerPhone(t, baseURL, masha.token, "masha-phone")

	commentOf(t, baseURL, kolya.token, post.ID, "Какие огурцы! Сорт какой?")

	got := fake.waitSent("masha-phone", 1, "пуш о комментарии")
	requirePush(t, got[0], kindComment, "kolya_dacha",
		"ответил на ваш пост: «Какие огурцы! Сорт какой?»", kolya.id, post.ID)
}

// Длинный комментарий в пуше — первые 100 символов, как в разделе
// (требование 11).
func TestPushCommentCut(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 2)
	masha, kolya := people[0], people[1]
	post := ownPost(t, baseURL, masha)
	registerPhone(t, baseURL, masha.token, "masha-phone")
	nick := me(t, baseURL, kolya.token).Nickname

	text := repeatRunes("Укроп и петрушка ", 250)
	commentOf(t, baseURL, kolya.token, post.ID, text)

	got := fake.waitSent("masha-phone", 1, "пуш о длинном комментарии")
	want := "ответил на ваш пост: «" + string([]rune(text)[:notificationCommentLimit]) + "»"
	requirePush(t, got[0], kindComment, nick, want, kolya.id, post.ID)
}

// Подписка на открытый профиль: «подписался на вас», data.user_id — кто
// подписался (требование 11).
func TestPushFollow(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 2)
	masha, kolya := people[0], people[1]
	chooseNickname(t, baseURL, kolya.token, "kolya_dacha")
	registerPhone(t, baseURL, masha.token, "masha-phone")

	followOK(t, baseURL, kolya, masha.id)

	got := fake.waitSent("masha-phone", 1, "пуш о подписке")
	requirePush(t, got[0], kindFollow, "kolya_dacha", "подписался на вас", kolya.id, "")
}

// Заявка на закрытый профиль — хозяину «хочет подписаться на вас»
// (требование 8); принятая заявка — заявителю «принял вашу заявку».
func TestPushFollowRequestAndAccepted(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 2)
	masha, kolya := people[0], people[1]
	chooseNickname(t, baseURL, masha.token, "masha_dacha")
	chooseNickname(t, baseURL, kolya.token, "kolya_dacha")
	setClosed(t, baseURL, masha.token, true)
	registerPhone(t, baseURL, masha.token, "masha-phone")
	registerPhone(t, baseURL, kolya.token, "kolya-phone")

	if rel := followOK(t, baseURL, kolya, masha.id); rel.Following != followingRequested {
		t.Fatalf("подписка на закрытый профиль: following = %q, ожидалась заявка", rel.Following)
	}

	got := fake.waitSent("masha-phone", 1, "пуш о заявке")
	requirePush(t, got[0], kindFollowRequest, "kolya_dacha", "хочет подписаться на вас", kolya.id, "")

	requireNoContent(t, acceptRequest(t, baseURL, masha.token, kolya.id), "принять заявку")

	got = fake.waitSent("kolya-phone", 1, "пуш о принятой заявке")
	requirePush(t, got[0], kindFollowAccepted, "masha_dacha", "принял вашу заявку", masha.id, "")

	// У хозяина от принятия ничего нового (014, «Заявка принята»).
	quiet(100 * time.Millisecond)
	requirePushes(t, fake, "masha-phone", []string{kindFollowRequest}, "после принятия заявки")
	requirePushes(t, fake, "kolya-phone", []string{kindFollowAccepted}, "после принятия заявки")
}

// --- Слитые отметки -------------------------------------------------------

// Отметки одного поста: вторая и третья — «и ещё N отметили ваш пост»,
// N — непрочитанные отметки поста кроме этой (требование 11); у всех один
// tag (требование 12). После открытия раздела счёт заново. OAuth-токен
// сервис берёт один раз и держит до истечения («Что уходит в FCM»).
func TestPushLikesMerge(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 5)
	masha, kolya, petya, vasya, fedya := people[0], people[1], people[2], people[3], people[4]
	chooseNickname(t, baseURL, petya.token, "petya_dacha")
	post := ownPost(t, baseURL, masha)
	registerPhone(t, baseURL, masha.token, "masha-phone")

	likeOK(t, baseURL, kolya, post.ID)
	fake.waitSent("masha-phone", 1, "первая отметка")
	likeOK(t, baseURL, vasya, post.ID)
	fake.waitSent("masha-phone", 2, "вторая отметка")
	likeOK(t, baseURL, petya, post.ID)
	got := fake.waitSent("masha-phone", 3, "третья отметка")

	if got[0].Body != "отметил ваш пост" {
		t.Errorf("первая отметка: body = %q, ожидался «отметил ваш пост»", got[0].Body)
	}
	if got[1].Body != "и ещё 1 отметили ваш пост" {
		t.Errorf("вторая отметка: body = %q, ожидался «и ещё 1 отметили ваш пост»", got[1].Body)
	}
	requirePush(t, got[2], kindLike, "petya_dacha", "и ещё 2 отметили ваш пост", petya.id, post.ID)
	for i, m := range got {
		if m.Tag != "like:"+post.ID {
			t.Errorf("отметка %d: tag = %q, у всех отметок поста он один: like:%s", i+1, m.Tag, post.ID)
		}
	}

	// Раздел открыт — прежние отметки прочитаны, счёт с нуля.
	markSeenOK(t, baseURL, masha)
	likeOK(t, baseURL, fedya, post.ID)
	got = fake.waitSent("masha-phone", 4, "отметка после открытия раздела")
	if got[3].Body != "отметил ваш пост" {
		t.Errorf("отметка после открытия раздела: body = %q, ожидался «отметил ваш пост»", got[3].Body)
	}

	if _, tokens := fake.totalRequests(); tokens != 1 {
		t.Errorf("OAuth-токен запрошен %d раз за 4 пуша, ожидался 1: сервис держит его до истечения", tokens)
	}
}

// Отметки разных постов друг друга не считают: N — отметки этого поста.
func TestPushLikesOfDifferentPosts(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 3)
	masha, kolya, petya := people[0], people[1], people[2]
	first := ownPost(t, baseURL, masha)
	second := ownPost(t, baseURL, masha)
	registerPhone(t, baseURL, masha.token, "masha-phone")

	likeOK(t, baseURL, kolya, first.ID)
	fake.waitSent("masha-phone", 1, "отметка первого поста")
	likeOK(t, baseURL, petya, second.ID)
	got := fake.waitSent("masha-phone", 2, "отметка второго поста")

	requirePush(t, got[1], kindLike, me(t, baseURL, petya.token).Nickname, "отметил ваш пост", petya.id, second.ID)
}

// --- Отменённое событие и свои действия -----------------------------------

// Действие отменено за время PUSH_DELAY — пуша нет (требование 9).
// Следом идёт подписка другого человека: её пуш — метка того, что
// отправка работает и время отменённого пуша прошло.
func TestPushCancelledWithinDelay(t *testing.T) {
	cases := []struct {
		name   string
		act    func(t *testing.T, baseURL string, masha, kolya dachnik, post postPayload)
		closed bool
	}{
		{"лайк снят", func(t *testing.T, baseURL string, masha, kolya dachnik, post postPayload) {
			likeOK(t, baseURL, kolya, post.ID)
			unlikeOK(t, baseURL, kolya, post.ID)
		}, false},
		{"комментарий удалён", func(t *testing.T, baseURL string, masha, kolya dachnik, post postPayload) {
			c := commentOf(t, baseURL, kolya.token, post.ID, "Скоро удалю")
			if resp := deleteComment(t, baseURL, kolya.token, post.ID, c.ID); resp.StatusCode/100 != 2 {
				t.Fatalf("удаление комментария: статус %d", resp.StatusCode)
			}
		}, false},
		{"пост удалён", func(t *testing.T, baseURL string, masha, kolya dachnik, post postPayload) {
			likeOK(t, baseURL, kolya, post.ID)
			commentOf(t, baseURL, kolya.token, post.ID, "Пост сейчас пропадёт")
			if resp := deletePost(t, baseURL, masha.token, post.ID); resp.StatusCode/100 != 2 {
				t.Fatalf("удаление поста: статус %d", resp.StatusCode)
			}
		}, false},
		{"отписка", func(t *testing.T, baseURL string, masha, kolya dachnik, post postPayload) {
			followOK(t, baseURL, kolya, masha.id)
			unfollowOK(t, baseURL, kolya, masha.id)
		}, false},
		{"заявка отозвана", func(t *testing.T, baseURL string, masha, kolya dachnik, post postPayload) {
			followOK(t, baseURL, kolya, masha.id)
			unfollowOK(t, baseURL, kolya, masha.id)
		}, true},
		{"заявка отклонена", func(t *testing.T, baseURL string, masha, kolya dachnik, post postPayload) {
			followOK(t, baseURL, kolya, masha.id)
			requireNoContent(t, declineRequest(t, baseURL, masha.token, kolya.id), "отклонить заявку")
		}, true},
		{"заявка принята", func(t *testing.T, baseURL string, masha, kolya dachnik, post postPayload) {
			followOK(t, baseURL, kolya, masha.id)
			requireNoContent(t, acceptRequest(t, baseURL, masha.token, kolya.id), "принять заявку")
		}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			baseURL, fake := startPush(t, pushDelay)
			people := newDachniks(t, baseURL, 1, 3)
			masha, kolya, petya := people[0], people[1], people[2]
			post := ownPost(t, baseURL, masha)
			if c.closed {
				setClosed(t, baseURL, masha.token, true)
			}
			registerPhone(t, baseURL, masha.token, "masha-phone")

			c.act(t, baseURL, masha, kolya, post)

			// Метка: подписка (или заявка) Пети.
			followOK(t, baseURL, petya, masha.id)
			got := fake.waitSent("masha-phone", 1, "пуш-метка от Пети")
			quiet(100 * time.Millisecond)
			got = fake.sentTo("masha-phone")

			if len(got) != 1 || got[0].Data["user_id"] != petya.id {
				t.Errorf("пришли пуши %s, ожидался только пуш-метка от Пети", describePushes(got))
			}
		})
	}
}

// Свои действия пушей не дают: лайк и комментарий своему посту
// (требование 8, 014 требование 3).
func TestPushNotForOwnActions(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 2)
	masha, kolya := people[0], people[1]
	post := ownPost(t, baseURL, masha)
	registerPhone(t, baseURL, masha.token, "masha-phone")

	likeOK(t, baseURL, masha, post.ID)
	commentOf(t, baseURL, masha.token, post.ID, "Сама себе")

	followOK(t, baseURL, kolya, masha.id)
	fake.waitSent("masha-phone", 1, "пуш-метка от Коли")
	quiet(100 * time.Millisecond)
	requirePushes(t, fake, "masha-phone", []string{kindFollow}, "после своих действий")
}

// --- Токены и сессии -------------------------------------------------------

// У получателя нет токена — пуша нет и очередь не растёт; токен, отданный
// позже, старых событий не получает («Ограничения и edge cases»).
func TestPushNoTokenNoPush(t *testing.T) {
	baseURL, fake := startPush(t, pushDelay)
	pool := connect(t)
	people := newDachniks(t, baseURL, 1, 3)
	masha, kolya, petya := people[0], people[1], people[2]
	post := ownPost(t, baseURL, masha)

	likeOK(t, baseURL, kolya, post.ID)
	if n := pushQueueLen(t, pool); n != 0 {
		t.Errorf("у получателя нет телефона, а в push_queue %d строк", n)
	}

	// Токен приходит, пока лайк ещё «лежал бы» в очереди.
	registerPhone(t, baseURL, masha.token, "masha-phone")
	followOK(t, baseURL, petya, masha.id)

	fake.waitSent("masha-phone", 1, "пуш о подписке после токена")
	quiet(100 * time.Millisecond)
	requirePushes(t, fake, "masha-phone", []string{kindFollow}, "старая отметка без токена")
}

// Два телефона — две сессии одного человека, у каждой свой токен: пуш
// на оба (требование 10).
func TestPushToEveryPhone(t *testing.T) {
	baseURL, fake := startPushWith(t, api.Config{ResendAfter: 50 * time.Millisecond}, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 2)
	masha, kolya := people[0], people[1]
	post := ownPost(t, baseURL, masha)

	time.Sleep(60 * time.Millisecond)
	secondSession, secondID := signIn(t, baseURL, dachnikPhone(1))
	if secondID != masha.id {
		t.Fatalf("второй вход по тому же номеру дал другого пользователя: %s и %s", masha.id, secondID)
	}
	registerPhone(t, baseURL, masha.token, "masha-phone-1")
	registerPhone(t, baseURL, secondSession, "masha-phone-2")

	likeOK(t, baseURL, kolya, post.ID)

	fake.waitSent("masha-phone-1", 1, "первый телефон")
	fake.waitSent("masha-phone-2", 1, "второй телефон")
	quiet(100 * time.Millisecond)
	requirePushes(t, fake, "masha-phone-1", []string{kindLike}, "первый телефон")
	requirePushes(t, fake, "masha-phone-2", []string{kindLike}, "второй телефон")
}

// У сессии один токен: новый заменяет старый (требование 7).
func TestPushNewTokenReplacesOld(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 2)
	masha, kolya := people[0], people[1]

	registerPhone(t, baseURL, masha.token, "old-token")
	registerPhone(t, baseURL, masha.token, "new-token")

	followOK(t, baseURL, kolya, masha.id)

	fake.waitSent("new-token", 1, "пуш на новый токен")
	quiet(100 * time.Millisecond)
	if n := fake.attemptsTo("old-token"); n != 0 {
		t.Errorf("на заменённый токен ушло %d запросов, ожидалось 0", n)
	}
}

// Тот же токен с сессии другого человека переезжает к ней: пуши первого на
// него больше не идут, второго — идут (требование 7).
func TestPushTokenMovesToOtherSession(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 4)
	masha, kolya, petya, vasya := people[0], people[1], people[2], people[3]

	registerPhone(t, baseURL, masha.token, "shared-phone")
	registerPhone(t, baseURL, kolya.token, "shared-phone")

	followOK(t, baseURL, petya, masha.id) // Маше — телефон уже не её
	followOK(t, baseURL, vasya, kolya.id) // Коле — на этот телефон

	got := fake.waitSent("shared-phone", 1, "пуш Коле")
	quiet(100 * time.Millisecond)
	got = fake.sentTo("shared-phone")
	if len(got) != 1 || got[0].Data["user_id"] != vasya.id {
		t.Errorf("на телефон пришли пуши %s, ожидался только пуш Коле о подписке Васи", describePushes(got))
	}
}

// Выход забывает токен сессии: пушей на него больше нет (требование 7).
func TestPushForgottenOnSignOut(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 3)
	masha, kolya, petya := people[0], people[1], people[2]

	registerPhone(t, baseURL, masha.token, "masha-phone")
	registerPhone(t, baseURL, petya.token, "petya-phone")
	if resp := signOut(t, baseURL, masha.token); resp.StatusCode/100 != 2 {
		t.Fatalf("выход: статус %d", resp.StatusCode)
	}

	followOK(t, baseURL, kolya, masha.id)
	followOK(t, baseURL, kolya, petya.id) // метка

	fake.waitSent("petya-phone", 1, "пуш-метка Пете")
	quiet(100 * time.Millisecond)
	if n := fake.attemptsTo("masha-phone"); n != 0 {
		t.Errorf("после выхода на телефон Маши ушло %d запросов, ожидалось 0", n)
	}
}

// --- Ответы FCM ------------------------------------------------------------

// FCM ответил, что токена больше нет (404 UNREGISTERED, 400
// INVALID_ARGUMENT), — токен забыт: следующие события на него не шлются
// (требование 13).
func TestPushDeadTokenForgotten(t *testing.T) {
	for _, mode := range []string{fcmUnregistered, fcmInvalid} {
		t.Run(mode, func(t *testing.T) {
			baseURL, fake := startPush(t, 100*time.Millisecond)
			people := newDachniks(t, baseURL, 1, 4)
			masha, kolya, petya, vasya := people[0], people[1], people[2], people[3]
			fake.setMode("dead-phone", mode)
			registerPhone(t, baseURL, masha.token, "dead-phone")
			registerPhone(t, baseURL, vasya.token, "vasya-phone")

			followOK(t, baseURL, kolya, masha.id)
			fake.waitAttempts("dead-phone", 1, "первый пуш на мёртвый токен")

			followOK(t, baseURL, petya, masha.id)
			followOK(t, baseURL, petya, vasya.id) // метка
			fake.waitSent("vasya-phone", 1, "пуш-метка Васе")
			quiet(100 * time.Millisecond)

			if n := fake.attemptsTo("dead-phone"); n != 1 {
				t.Errorf("на забытый токен было %d запросов, ожидался 1: токен должен быть забыт после ответа FCM", n)
			}
		})
	}
}

// Временная ошибка FCM (503) — повтор на следующем круге; когда FCM
// ожил, пуш доходит, и ровно один (требование 13).
func TestPushRetriedAfterTemporaryError(t *testing.T) {
	baseURL, fake := startPush(t, 100*time.Millisecond)
	people := newDachniks(t, baseURL, 1, 2)
	masha, kolya := people[0], people[1]
	fake.setMode("masha-phone", fcmUnavailable)
	registerPhone(t, baseURL, masha.token, "masha-phone")

	followOK(t, baseURL, kolya, masha.id)
	fake.waitAttempts("masha-phone", 2, "повтор после 503")

	fake.setMode("masha-phone", fcmOK)
	fake.waitSent("masha-phone", 1, "пуш после того, как FCM ожил")
	quiet(100 * time.Millisecond)
	requirePushes(t, fake, "masha-phone", []string{kindFollow}, "после повторов")
}

// Событие старше MaxAge (на проде час) уже не шлётся, даже когда FCM ожил
// (требование 13).
func TestPushTooOldDropped(t *testing.T) {
	fake := newFakeFCM(t)
	baseURL := startAPI(t)
	cfg := pushConfig(fake, 100*time.Millisecond)
	cfg.MaxAge = 700 * time.Millisecond
	runSender(t, connect(t), cfg)

	people := newDachniks(t, baseURL, 1, 3)
	masha, kolya, petya := people[0], people[1], people[2]
	post := ownPost(t, baseURL, masha)
	fake.setMode("masha-phone", fcmUnavailable)
	registerPhone(t, baseURL, masha.token, "masha-phone")

	followOK(t, baseURL, kolya, masha.id)
	fake.waitAttempts("masha-phone", 1, "первая попытка")
	time.Sleep(cfg.MaxAge + 300*time.Millisecond)

	fake.setMode("masha-phone", fcmOK)
	likeOK(t, baseURL, petya, post.ID) // свежее событие — метка
	fake.waitSent("masha-phone", 1, "свежий пуш")
	quiet(100 * time.Millisecond)
	requirePushes(t, fake, "masha-phone", []string{kindLike}, "старое событие брошено")
}

// --- Без ключа Firebase ----------------------------------------------------

// Без ключа Firebase New не ошибается, Run ничего не шлёт и очередь не
// копит; токены сервис принимает, и после настройки пуши идут на уже
// известные телефоны (требование 14).
func TestPushWithoutCredentials(t *testing.T) {
	fake := newFakeFCM(t)
	baseURL := startAPI(t)
	pool := connect(t)
	people := newDachniks(t, baseURL, 1, 3)
	masha, kolya, petya := people[0], people[1], people[2]

	func() {
		sender, err := push.New(pool, push.Config{
			APIURL:   fake.URL(),
			Delay:    100 * time.Millisecond,
			Interval: 20 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("push.New без ключа: %v, ожидался успех", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			sender.Run(ctx)
		}()

		registerPhone(t, baseURL, masha.token, "masha-phone")
		followOK(t, baseURL, kolya, masha.id)

		// Очередь не копится: строка либо не легла, либо убрана.
		deadline := time.Now().Add(2 * time.Second)
		for pushQueueLen(t, pool) != 0 && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		if n := pushQueueLen(t, pool); n != 0 {
			t.Errorf("без ключа в push_queue %d строк, очередь не должна копиться", n)
		}
		quiet(100 * time.Millisecond)

		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Run без ключа не вернулся за 5 секунд после отмены ctx")
		}
	}()

	if sends, tokens := fake.totalRequests(); sends != 0 || tokens != 0 {
		t.Errorf("без ключа в Google ушло запросов: messages:send %d, token %d; ожидалось 0", sends, tokens)
	}

	// Ключ появился — пуши идут на уже известный телефон, старое не догоняет.
	runSender(t, pool, pushConfig(fake, 100*time.Millisecond))
	followOK(t, baseURL, petya, masha.id)

	got := fake.waitSent("masha-phone", 1, "пуш после настройки ключа")
	quiet(100 * time.Millisecond)
	got = fake.sentTo("masha-phone")
	if len(got) != 1 || got[0].Data["user_id"] != petya.id {
		t.Errorf("после настройки ключа пришли пуши %s, ожидался только пуш о подписке Пети", describePushes(got))
	}
}
