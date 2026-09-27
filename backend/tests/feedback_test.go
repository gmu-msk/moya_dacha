package tests

// Тесты обратной связи (specs/019-feedback.md).
//
// Отзыв из приложения проверяется по HTTP, как его шлёт приложение:
// POST и GET /me/feedback. Задачи GitHub сервис заводит сам, поэтому тест
// подменяет GitHub фейковым REST API на httptest (ручки из ФТ-35): тот
// записывает заведённые задачи и метки и позволяет задать состояние задачи.
// Проход «завести задачи и сверить статусы», выход в сборке, бот
// и уведомления автору вызываются через Go-API из ФТ-30–34: feedback.New,
// Sync, Release, telegram.Config.Feedback, (*telegram.Bot).Notify.
// Telegram подменён фейком из telegram_test.go. База — настоящая Postgres;
// где сервис не даёт подготовить данные сам, тест пишет прямо в таблицы
// feedback и telegram_chats из «Модели данных».

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
	"github.com/gmu-msk/moya_dacha/backend/internal/feedback"
	"github.com/gmu-msk/moya_dacha/backend/internal/media"
	"github.com/gmu-msk/moya_dacha/backend/internal/telegram"
)

// *telegram.Bot — это Notifier (ФТ-31).
var _ feedback.Notifier = (*telegram.Bot)(nil)

// Настройки GitHub в тестах.
const (
	ghToken       = "gh-test-token"
	ghRepo        = "tester/dacha-feedback"
	ghDefaultRepo = "gmu-msk/moya_dacha" // FEEDBACK_GITHUB_REPO по умолчанию (ФТ-3)
	fbPublicURL   = "https://dacha.example"
	ghFirstIssue  = 71 // номер, который фейк выдаст первой заведённой задаче
)

// Метки и тексты из спецификации.
const (
	lblInbox    = "входящее"
	lblApp      = "из приложения"
	lblTelegram = "из телеграма"
	lblApproved = "одобрено"

	fbNoGitHubThanks = "Спасибо! Передал разработчику."
	fbIdeasBound     = "Готово: сообщения отсюда станут задачами."
	fbInboxHeader    = "Входящие:"
	fbInboxEmpty     = "Входящих нет."
	fbNeedNumber     = "Напишите номер задачи: /одобрить 71"
	fbNoGitHub       = "GitHub не настроен: нет FEEDBACK_GITHUB_TOKEN."
	fbNoCaption      = "Скриншот без подписи"

	fbBuild int64 = 386950
)

func fbAccepted(n int) string {
	return fmt.Sprintf("Спасибо! Записал как задачу #%d. Напишу, когда её одобрят и когда она выйдет в сборке.", n)
}

func fbApprovedNote(n int) string {
	return fmt.Sprintf("Задачу #%d одобрили: её возьмут в работу.", n)
}

func fbReleasedNote(n int, build int64) string {
	return fmt.Sprintf("Задача #%d вышла в сборке %d — обновите приложение.", n, build)
}

// --- Фейковый GitHub ------------------------------------------------------

// ghIssue — задача в фейковом GitHub.
type ghIssue struct {
	Number      int
	Title       string
	Body        string
	Labels      []string
	State       string // open | closed
	StateReason string // completed | not_planned | duplicate | ""
	PR          bool   // это pull request: в списке задач GitHub отдаёт и их
}

func (i ghIssue) hasLabel(name string) bool {
	for _, l := range i.Labels {
		if l == name {
			return true
		}
	}
	return false
}

// ghLabelCall — запрос POST /repos/{repo}/issues/{n}/labels.
type ghLabelCall struct {
	Number int
	Labels []string
}

// fakeGitHub — REST API задач GitHub в миниатюре (ФТ-35).
type fakeGitHub struct {
	t    *testing.T
	srv  *httptest.Server
	repo string

	mu          sync.Mutex
	issues      map[int]*ghIssue
	created     []int // номера задач, заведённых сервисом, по порядку
	next        int
	failCreate  bool
	createTries int
	failGet     map[int]bool
	labelCalls  []ghLabelCall
	listQueries []url.Values
	requests    int
	wrong       []string
}

func newFakeGitHub(t *testing.T, repo string) *fakeGitHub {
	t.Helper()

	g := &fakeGitHub{
		t:       t,
		repo:    repo,
		issues:  map[int]*ghIssue{},
		next:    ghFirstIssue,
		failGet: map[int]bool{},
	}
	g.srv = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.srv.Close)
	t.Cleanup(func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		for _, w := range g.wrong {
			t.Errorf("фейковый GitHub: %s", w)
		}
	})

	return g
}

func (g *fakeGitHub) URL() string { return g.srv.URL }

func (g *fakeGitHub) complain(format string, args ...any) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.wrong = append(g.wrong, fmt.Sprintf(format, args...))
}

func ghJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (g *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	g.requests++
	g.mu.Unlock()

	if got := r.Header.Get("Authorization"); got != "Bearer "+ghToken {
		g.complain("запрос %s %s с Authorization %q, ожидался «Bearer <токен>» (ФТ-35)", r.Method, r.URL.Path, got)
		ghJSON(w, http.StatusUnauthorized, map[string]any{"message": "Bad credentials"})
		return
	}
	if got := r.Header.Get("Accept"); !strings.Contains(got, "application/vnd.github+json") {
		g.complain("запрос %s %s с Accept %q, ожидался application/vnd.github+json (ФТ-35)", r.Method, r.URL.Path, got)
	}

	prefix := "/repos/" + g.repo + "/issues"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		g.complain("запрос не в репозиторий %s: %s %s", g.repo, r.Method, r.URL.Path)
		ghJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(rest, "/")

	switch {
	case rest == "" && r.Method == http.MethodPost:
		g.serveCreate(w, r)
	case rest == "" && r.Method == http.MethodGet:
		g.serveList(w, r)
	case len(parts) == 1 && r.Method == http.MethodGet:
		g.serveGet(w, parts[0])
	case len(parts) == 2 && parts[1] == "labels" && r.Method == http.MethodPost:
		g.serveLabels(w, r, parts[0])
	default:
		g.complain("неожиданный запрос %s %s", r.Method, r.URL.Path)
		ghJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
	}
}

func (g *fakeGitHub) serveCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		g.complain("POST issues: тело не JSON: %v", err)
		ghJSON(w, http.StatusBadRequest, map[string]any{"message": "Problems parsing JSON"})
		return
	}

	g.mu.Lock()
	g.createTries++
	if g.failCreate {
		g.mu.Unlock()
		ghJSON(w, http.StatusBadGateway, map[string]any{"message": "Server Error"})
		return
	}
	n := g.next
	g.next++
	g.issues[n] = &ghIssue{Number: n, Title: body.Title, Body: body.Body, Labels: body.Labels, State: "open"}
	g.created = append(g.created, n)
	g.mu.Unlock()

	ghJSON(w, http.StatusCreated, map[string]any{
		"number":   n,
		"html_url": g.htmlURL(n),
		"title":    body.Title,
		"state":    "open",
	})
}

func (g *fakeGitHub) htmlURL(n int) string {
	return fmt.Sprintf("https://github.com/%s/issues/%d", g.repo, n)
}

func issueJSON(i *ghIssue) map[string]any {
	labels := make([]map[string]any, 0, len(i.Labels))
	for _, l := range i.Labels {
		labels = append(labels, map[string]any{"name": l})
	}
	out := map[string]any{
		"number": i.Number,
		"title":  i.Title,
		"body":   i.Body,
		"state":  i.State,
		"labels": labels,
	}
	if i.StateReason != "" {
		out["state_reason"] = i.StateReason
	} else {
		out["state_reason"] = nil
	}
	if i.PR {
		out["pull_request"] = map[string]any{"url": "https://api.github.com/pulls/" + strconv.Itoa(i.Number)}
	}
	return out
}

func (g *fakeGitHub) serveGet(w http.ResponseWriter, raw string) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		g.complain("GET issues/%s: номер не число", raw)
		ghJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.failGet[n] {
		ghJSON(w, http.StatusInternalServerError, map[string]any{"message": "Server Error"})
		return
	}
	issue, ok := g.issues[n]
	if !ok {
		ghJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
		return
	}
	ghJSON(w, http.StatusOK, issueJSON(issue))
}

func (g *fakeGitHub) serveList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	g.mu.Lock()
	defer g.mu.Unlock()
	g.listQueries = append(g.listQueries, q)

	// Фильтры — как у настоящего GitHub: метки через запятую, все
	// обязательны; state по умолчанию open.
	var want []string
	if l := q.Get("labels"); l != "" {
		want = strings.Split(l, ",")
	}
	state := q.Get("state")
	if state == "" {
		state = "open"
	}
	perPage := 30
	if p, err := strconv.Atoi(q.Get("per_page")); err == nil && p > 0 {
		perPage = p
	}

	var list []*ghIssue
	for _, i := range g.issues {
		if state != "all" && i.State != state {
			continue
		}
		ok := true
		for _, l := range want {
			if !i.hasLabel(l) {
				ok = false
			}
		}
		if ok {
			list = append(list, i)
		}
	}
	// По умолчанию GitHub сортирует по созданию, новые сверху.
	sort.Slice(list, func(a, b int) bool { return list[a].Number > list[b].Number })
	if len(list) > perPage {
		list = list[:perPage]
	}

	out := make([]map[string]any, 0, len(list))
	for _, i := range list {
		out = append(out, issueJSON(i))
	}
	ghJSON(w, http.StatusOK, out)
}

func (g *fakeGitHub) serveLabels(w http.ResponseWriter, r *http.Request, raw string) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		ghJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
		return
	}
	var body struct {
		Labels []string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		g.complain("POST issues/%d/labels: тело не JSON: %v", n, err)
		ghJSON(w, http.StatusBadRequest, map[string]any{"message": "Problems parsing JSON"})
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	issue, ok := g.issues[n]
	if !ok {
		ghJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
		return
	}
	g.labelCalls = append(g.labelCalls, ghLabelCall{Number: n, Labels: body.Labels})
	for _, l := range body.Labels {
		if !issue.hasLabel(l) {
			issue.Labels = append(issue.Labels, l)
		}
	}
	labels := make([]map[string]any, 0, len(issue.Labels))
	for _, l := range issue.Labels {
		labels = append(labels, map[string]any{"name": l})
	}
	ghJSON(w, http.StatusOK, labels)
}

// seed заводит задачу в обход сервиса — как если бы её завёл человек.
func (g *fakeGitHub) seed(title, state string, pr bool, labels ...string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := g.next
	g.next++
	g.issues[n] = &ghIssue{Number: n, Title: title, Labels: labels, State: state, PR: pr}
	if state == "closed" {
		g.issues[n].StateReason = "completed"
	}
	return n
}

// closeIssue закрывает задачу с причиной (completed, not_planned, duplicate).
func (g *fakeGitHub) closeIssue(n int, reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.mustIssue(n).State = "closed"
	g.issues[n].StateReason = reason
}

// reopen открывает задачу заново.
func (g *fakeGitHub) reopen(n int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.mustIssue(n).State = "open"
	g.issues[n].StateReason = "reopened"
}

// label вешает метки на задачу, как это сделал бы владелец в GitHub.
func (g *fakeGitHub) label(n int, names ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	issue := g.mustIssue(n)
	for _, l := range names {
		if !issue.hasLabel(l) {
			issue.Labels = append(issue.Labels, l)
		}
	}
}

func (g *fakeGitHub) mustIssue(n int) *ghIssue {
	issue, ok := g.issues[n]
	if !ok {
		g.t.Fatalf("в фейковом GitHub нет задачи #%d", n)
	}
	return issue
}

func (g *fakeGitHub) setFailCreate(fail bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failCreate = fail
}

func (g *fakeGitHub) setFailGet(n int, fail bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failGet[n] = fail
}

// createdIssues — задачи, заведённые сервисом, по порядку заведения.
func (g *fakeGitHub) createdIssues() []ghIssue {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]ghIssue, 0, len(g.created))
	for _, n := range g.created {
		i := *g.issues[n]
		i.Labels = append([]string(nil), i.Labels...)
		out = append(out, i)
	}
	return out
}

func (g *fakeGitHub) issue(n int) ghIssue {
	g.mu.Lock()
	defer g.mu.Unlock()
	i := *g.mustIssue(n)
	i.Labels = append([]string(nil), i.Labels...)
	return i
}

func (g *fakeGitHub) tries() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.createTries
}

func (g *fakeGitHub) requestCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.requests
}

func (g *fakeGitHub) labelRequests() []ghLabelCall {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]ghLabelCall(nil), g.labelCalls...)
}

func (g *fakeGitHub) lastListQuery() url.Values {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.listQueries) == 0 {
		return nil
	}
	return g.listQueries[len(g.listQueries)-1]
}

// waitCreated ждёт, пока сервис заведёт n задач, и возвращает их.
func (g *fakeGitHub) waitCreated(n int, what string) []ghIssue {
	g.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		got := g.createdIssues()
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			g.t.Fatalf("%s: за 5 секунд заведено задач %d из %d ожидаемых", what, len(got), n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// createdByTitle находит заведённую задачу по заголовку.
func (g *fakeGitHub) createdByTitle(t *testing.T, title string) ghIssue {
	t.Helper()
	var titles []string
	for _, i := range g.createdIssues() {
		if i.Title == title {
			return i
		}
		titles = append(titles, i.Title)
	}
	t.Fatalf("задачи с заголовком %q нет; заведены: %q", title, titles)
	return ghIssue{}
}

// --- Фейковый Telegram: файлы ----------------------------------------------

// tgFile — файл в фейковом Telegram. missing — getFile его знает, а
// скачивание отвечает 404.
type tgFile struct {
	path    string
	content []byte
	missing bool
}

func (f *fakeTelegram) addFile(fileID, path string, content []byte, missing bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.files == nil {
		f.files = map[string]tgFile{}
	}
	f.files[fileID] = tgFile{path: path, content: content, missing: missing}
}

func (f *fakeTelegram) askedFiles() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.fileAsked...)
}

func (f *fakeTelegram) serveGetFile(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var p struct {
		FileID string `json:"file_id"`
	}
	if json.Unmarshal(body, &p) != nil || p.FileID == "" {
		if vals, err := url.ParseQuery(string(body)); err == nil {
			p.FileID = vals.Get("file_id")
		}
	}
	if p.FileID == "" {
		p.FileID = r.URL.Query().Get("file_id")
	}

	f.mu.Lock()
	f.fileAsked = append(f.fileAsked, p.FileID)
	file, ok := f.files[p.FileID]
	f.mu.Unlock()

	if !ok {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: invalid file_id"}`))
		return
	}
	tgReply(w, map[string]any{
		"file_id":        p.FileID,
		"file_unique_id": "u" + p.FileID,
		"file_size":      len(file.content),
		"file_path":      file.path,
	})
}

func (f *fakeTelegram) serveFile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/file/bot"+tgToken+"/")

	f.mu.Lock()
	defer f.mu.Unlock()
	for _, file := range f.files {
		if file.path == path && !file.missing {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(file.content)
			return
		}
	}
	http.NotFound(w, r)
}

// tgMessage — входящее сообщение для фейкового Telegram.
type tgMessage struct {
	fromID    int64
	username  string
	firstName string
	lastName  string
	isBot     bool
	chatID    int64
	chatType  string // private | group | supergroup
	threadID  int64  // message_thread_id; 0 — не в теме
	text      string
	caption   string
	photo     []string // file_id размеров фото, от меньшего к большему
	sticker   bool
}

// pushMessage кладёт сообщение в очередь обновлений и возвращает его
// message_id.
func (f *fakeTelegram) pushMessage(m tgMessage) int64 {
	f.mu.Lock()
	f.nextID++
	id := f.nextID

	first := m.firstName
	if first == "" {
		first = "Человек"
	}
	from := map[string]any{"id": m.fromID, "is_bot": m.isBot, "first_name": first}
	if m.lastName != "" {
		from["last_name"] = m.lastName
	}
	if m.username != "" {
		from["username"] = m.username
	}
	chatType := m.chatType
	if chatType == "" {
		chatType = "private"
	}
	chat := map[string]any{"id": m.chatID, "type": chatType}
	if chatType == "private" {
		chat["first_name"] = first
		if m.username != "" {
			chat["username"] = m.username
		}
	} else {
		chat["title"] = "Тестировщики МоейДачи"
		if m.threadID != 0 {
			chat["is_forum"] = true
		}
	}

	msg := map[string]any{
		"message_id": id,
		"from":       from,
		"chat":       chat,
		"date":       time.Now().Unix(),
	}
	if m.threadID != 0 {
		msg["message_thread_id"] = m.threadID
		msg["is_topic_message"] = true
	}
	if m.text != "" {
		msg["text"] = m.text
	}
	if m.caption != "" {
		msg["caption"] = m.caption
	}
	if len(m.photo) > 0 {
		sizes := make([]map[string]any, 0, len(m.photo))
		for i, fileID := range m.photo {
			w := 90 * (i + 1) * (i + 1)
			sizes = append(sizes, map[string]any{
				"file_id":        fileID,
				"file_unique_id": "u" + fileID,
				"width":          w,
				"height":         w / 2,
				"file_size":      1000 * (i + 1) * (i + 1),
			})
		}
		msg["photo"] = sizes
	}
	if m.sticker {
		msg["sticker"] = map[string]any{
			"file_id": "sticker", "file_unique_id": "usticker",
			"type": "regular", "width": 512, "height": 512,
			"is_animated": false, "is_video": false, "emoji": "👍",
		}
	}

	f.updates = append(f.updates, map[string]any{"update_id": id, "message": msg})
	f.mu.Unlock()

	select {
	case f.arrived <- struct{}{}:
	default:
	}
	return id
}

// strangerPrivate — тестировщик пишет боту в личку.
func strangerPrivate(text string) tgMessage {
	return tgMessage{fromID: tgStrangerID, username: "tester_vasya", chatID: tgStrangerID, chatType: "private", text: text}
}

// strangerInGroup — тестировщик пишет в группу, в тему thread (0 — вне темы).
func strangerInGroup(chatID, thread int64, text string) tgMessage {
	return tgMessage{fromID: tgStrangerID, username: "tester_vasya", chatID: chatID, chatType: "supergroup", threadID: thread, text: text}
}

// --- Хранилище, которое помнит, что в него положили -----------------------

type storedFile struct {
	key  string
	data []byte
}

type recordingStorage struct {
	media.Storage

	mu   sync.Mutex
	puts []storedFile
}

func newRecordingStorage(t *testing.T) *recordingStorage {
	return &recordingStorage{Storage: media.NewDisk(t.TempDir(), "/media")}
}

func (s *recordingStorage) Put(ctx context.Context, key string, data []byte) error {
	s.mu.Lock()
	s.puts = append(s.puts, storedFile{key: key, data: append([]byte(nil), data...)})
	s.mu.Unlock()
	return s.Storage.Put(ctx, key, data)
}

func (s *recordingStorage) stored() []storedFile {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]storedFile(nil), s.puts...)
}

// --- Окружение ------------------------------------------------------------

// fbEnv — сервис с ручками, фейковый GitHub и сервис отзывов,
// смотрящий в него.
type fbEnv struct {
	baseURL string
	root    string
	pool    *pgxpool.Pool
	store   *recordingStorage
	gh      *fakeGitHub
	svc     *feedback.Service
}

// fbOptions — как собрать окружение.
type fbOptions struct {
	// background — ручки пишут через svc и сами в фоне заводят задачу
	// (api.Config.Feedback = svc). Иначе api.Config.Feedback = nil, и
	// задачи заводит только явный svc.Sync.
	background bool
	token      string // пусто — ghToken; "-" — без токена
	repo       string // пусто — ghRepo; "-" — не задавать (по умолчанию)
	publicURL  string // пусто — fbPublicURL; "-" — без PUBLIC_URL
}

func (o fbOptions) config(gh *fakeGitHub, store media.Storage) feedback.Config {
	cfg := feedback.Config{Token: ghToken, Repo: ghRepo, APIURL: gh.URL(), PublicURL: fbPublicURL, Media: store}
	switch o.token {
	case "":
	case "-":
		cfg.Token = ""
	default:
		cfg.Token = o.token
	}
	if o.repo == "-" {
		cfg.Repo = ""
	}
	if o.publicURL == "-" {
		cfg.PublicURL = ""
	}
	return cfg
}

func startFeedback(t *testing.T, o fbOptions) fbEnv {
	t.Helper()

	repo := ghRepo
	if o.repo == "-" {
		repo = ghDefaultRepo
	}
	gh := newFakeGitHub(t, repo)
	store := newRecordingStorage(t)
	pool := connect(t)
	svc := feedback.New(pool, o.config(gh, store))

	cfg := api.Config{Media: store, DashboardPassword: dashboardPassword}
	if o.background {
		cfg.Feedback = svc
	}
	baseURL := startAPIWith(t, cfg)

	return fbEnv{
		baseURL: baseURL,
		root:    strings.TrimSuffix(baseURL, "/api"),
		pool:    pool,
		store:   store,
		gh:      gh,
		svc:     svc,
	}
}

func fbCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// sync — один проход svc.Sync без уведомлений; ошибки не ждёт.
func (e fbEnv) sync(t *testing.T) {
	t.Helper()
	if err := e.svc.Sync(fbCtx(t), nil); err != nil {
		t.Fatalf("Sync вернул ошибку: %v", err)
	}
}

func feedbackPhone(n int) string {
	return fmt.Sprintf("+7 (900) 819-00-%02d", n)
}

// fbUser входит, задаёт никнейм и (если name не пусто) имя.
func fbUser(t *testing.T, baseURL string, n int, name, nick string) (token, userID string) {
	t.Helper()
	token, userID = signIn(t, baseURL, feedbackPhone(n))
	chooseNickname(t, baseURL, token, nick)
	if name != "" {
		introduce(t, baseURL, token, name)
	}
	return token, userID
}

// fbShot — скриншот в запросе.
type fbShot struct {
	filename string
	content  []byte
}

// postFeedback шлёт отзыв multipart-формой. Поле text уходит, только если
// оно есть в fields.
func postFeedback(t *testing.T, baseURL, token string, fields map[string]string, shot *fbShot) *http.Response {
	t.Helper()

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := form.WriteField(k, fields[k]); err != nil {
			t.Fatalf("не удалось собрать multipart-запрос: %v", err)
		}
	}
	if shot != nil {
		part, err := form.CreateFormFile("screenshot", shot.filename)
		if err != nil {
			t.Fatalf("не удалось собрать multipart-запрос: %v", err)
		}
		if _, err := part.Write(shot.content); err != nil {
			t.Fatalf("не удалось записать скриншот в запрос: %v", err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatalf("не удалось закрыть multipart-запрос: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/me/feedback", &body)
	if err != nil {
		t.Fatalf("не удалось собрать запрос: %v", err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /me/feedback не прошёл: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

type feedbackPayload struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	Status    string `json:"status"`
	Issue     *int   `json:"issue"`
	Build     *int64 `json:"build"`
	CreatedAt string `json:"created_at"`
}

// sendFeedback шлёт отзыв и требует 201.
func sendFeedback(t *testing.T, baseURL, token string, fields map[string]string, shot *fbShot) feedbackPayload {
	t.Helper()
	resp := postFeedback(t, baseURL, token, fields, shot)
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("на отзыв %q ожидался статус 201, получен %d: %s", fields["text"], resp.StatusCode, raw)
	}
	var fb feedbackPayload
	decode(t, resp, &fb)
	return fb
}

func sendText(t *testing.T, baseURL, token, text string) feedbackPayload {
	t.Helper()
	return sendFeedback(t, baseURL, token, map[string]string{"text": text}, nil)
}

// myFeedback — GET /me/feedback с требованием 200.
func myFeedback(t *testing.T, baseURL, token string) []feedbackPayload {
	t.Helper()
	resp := do(t, http.MethodGet, baseURL+"/me/feedback", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на GET /me/feedback ожидался статус 200, получен %d", resp.StatusCode)
	}
	var body struct {
		Items []feedbackPayload `json:"items"`
	}
	decode(t, resp, &body)
	return body.Items
}

func findFeedback(items []feedbackPayload, id int64) *feedbackPayload {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

// waitMyStatus опрашивает GET /me/feedback, пока отзыв не окажется в статусе.
func waitMyStatus(t *testing.T, baseURL, token string, id int64, status string) feedbackPayload {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		fb := findFeedback(myFeedback(t, baseURL, token), id)
		if fb == nil {
			t.Fatalf("отзыва %d нет в GET /me/feedback", id)
		}
		if fb.Status == status {
			return *fb
		}
		if time.Now().After(deadline) {
			t.Fatalf("отзыв %d за 5 секунд не перешёл в %s, он в %s", id, status, fb.Status)
		}
		time.Sleep(30 * time.Millisecond)
	}
}

// fbRow — строка feedback из базы.
type fbRow struct {
	id         int64
	source     string
	author     string
	status     string
	issue      *int
	issueURL   *string
	build      *int64
	appVersion *string
	device     *string
	screenshot *string
}

func feedbackRows(t *testing.T, where string, args ...any) []fbRow {
	t.Helper()

	pool := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		SELECT id, source, author, status, issue, issue_url, build, app_version, device, screenshot
		FROM feedback WHERE `+where+` ORDER BY id`, args...)
	if err != nil {
		t.Fatalf("не удалось прочитать feedback: %v", err)
	}
	defer rows.Close()

	var out []fbRow
	for rows.Next() {
		var r fbRow
		if err := rows.Scan(&r.id, &r.source, &r.author, &r.status, &r.issue, &r.issueURL, &r.build, &r.appVersion, &r.device, &r.screenshot); err != nil {
			t.Fatalf("не удалось прочитать строку feedback: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("не удалось прочитать feedback: %v", err)
	}
	return out
}

// statusOfIssue — статус отзыва, за которым задача n.
func statusOfIssue(t *testing.T, n int) string {
	t.Helper()
	rows := feedbackRows(t, "issue = $1", n)
	if len(rows) != 1 {
		t.Fatalf("задаче #%d ожидался один отзыв, их %d", n, len(rows))
	}
	return rows[0].status
}

// waitIssueStatus ждёт, пока отзыв задачи n окажется в статусе.
func waitIssueStatus(t *testing.T, n int, status string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		rows := feedbackRows(t, "issue = $1", n)
		if len(rows) == 1 && rows[0].status == status {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("отзыв задачи #%d за 5 секунд не перешёл в %s: %+v", n, status, rows)
		}
		time.Sleep(30 * time.Millisecond)
	}
}

// requireLabels требует ровно такой набор меток.
func requireLabels(t *testing.T, issue ghIssue, want ...string) {
	t.Helper()
	got := append([]string(nil), issue.Labels...)
	sort.Strings(got)
	w := append([]string(nil), want...)
	sort.Strings(w)
	if strings.Join(got, "|") != strings.Join(w, "|") {
		t.Fatalf("у задачи #%d метки %q, ожидались %q (ФТ-6)", issue.Number, issue.Labels, want)
	}
}

// fbRefLine — строка тела задачи со ссылкой на отзыв на дашборде (ФТ-5):
// «Отзыв: №<id>[, есть скриншот] — автор и скриншот на дашборде
// <PUBLIC_URL>/dashboard». publicURL пустой — без PUBLIC_URL.
func fbRefLine(id int64, withShot bool, publicURL string) string {
	shot := ""
	if withShot {
		shot = ", есть скриншот"
	}
	return fmt.Sprintf("Отзыв: №%d%s — автор и скриншот на дашборде %s/dashboard", id, shot, publicURL)
}

// requireIssueBody сверяет тело задачи с шаблоном ФТ-5: текст, пустая
// строка, «---» и строки meta. Репозиторий задач публичный, поэтому ни
// автора, ни ссылки на скриншот в теле быть не должно.
func requireIssueBody(t *testing.T, body, text string, meta ...string) {
	t.Helper()

	body = strings.ReplaceAll(body, "\r\n", "\n")
	if !strings.HasPrefix(body, text+"\n\n") {
		t.Fatalf("тело задачи должно начинаться текстом отзыва и пустой строкой (ФТ-5):\n%s", body)
	}
	for _, banned := range []string{"Автор:", "![скриншот]", "/media/"} {
		if strings.Contains(body, banned) {
			t.Fatalf("в теле задачи не должно быть %q: автор и скриншот только на дашборде (ФТ-5):\n%s", banned, body)
		}
	}
	rest := strings.TrimPrefix(body, text+"\n\n")

	want := "---\n" + strings.Join(meta, "\n")
	got := strings.TrimRight(rest, "\n ")
	if got != want {
		t.Fatalf("хвост тела задачи (ФТ-5):\n%s\nожидался:\n%s", got, want)
	}
}

// dashboardShot — screenshot_url отзыва id из /dashboard/data (ФТ-27):
// автор и скриншот теперь только там.
func dashboardShot(t *testing.T, env fbEnv, id int64) string {
	t.Helper()
	var body struct {
		Feedback struct {
			Recent []struct {
				ID            int64   `json:"id"`
				ScreenshotURL *string `json:"screenshot_url"`
			} `json:"recent"`
		} `json:"feedback"`
	}
	if err := json.Unmarshal(dashboardRaw(t, env.root), &body); err != nil {
		t.Fatalf("данные дашборда не разобрались: %v", err)
	}
	for _, item := range body.Feedback.Recent {
		if item.ID != id {
			continue
		}
		if item.ScreenshotURL == nil || !strings.HasPrefix(*item.ScreenshotURL, "/media/") {
			t.Fatalf("у отзыва %d со скриншотом screenshot_url — ссылка хранилища «/media/…», а он %v", id, item.ScreenshotURL)
		}
		return *item.ScreenshotURL
	}
	t.Fatalf("отзыва %d нет в feedback.recent дашборда", id)
	return ""
}

// onlyFeedbackID — номер единственного отзыва в базе.
func onlyFeedbackID(t *testing.T) int64 {
	t.Helper()
	rows := feedbackRows(t, "TRUE")
	if len(rows) != 1 {
		t.Fatalf("в feedback ожидалась одна строка, их %d", len(rows))
	}
	return rows[0].id
}

// photoContent — картинка, которую «прислал» Telegram.
func photoContent(t *testing.T) []byte {
	return imageBytes(t, "jpeg", 400, 200)
}

// runFeedbackBot запускает бота с сервисом отзывов (ФТ-34).
func runFeedbackBot(t *testing.T, pool *pgxpool.Pool, fake *fakeTelegram, svc *feedback.Service) *telegram.Bot {
	t.Helper()

	cfg := tgConfig(t, fake)
	cfg.Feedback = svc
	bot := telegram.New(pool, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		bot.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Run не вернулся за 5 секунд после отмены ctx")
		}
	})
	return bot
}

// tgEnv — бот с сервисом отзывов, фейковые Telegram и GitHub.
type tgEnv struct {
	pool  *pgxpool.Pool
	fake  *fakeTelegram
	gh    *fakeGitHub
	store *recordingStorage
	svc   *feedback.Service
	bot   *telegram.Bot
}

// startFeedbackBot поднимает бота; withToken == false — без токена GitHub.
func startFeedbackBot(t *testing.T, withToken bool) tgEnv {
	t.Helper()

	pool := tgPool(t)
	fake := newFakeTelegram(t)
	gh := newFakeGitHub(t, ghRepo)
	store := newRecordingStorage(t)
	o := fbOptions{}
	if !withToken {
		o.token = "-"
	}
	svc := feedback.New(pool, o.config(gh, store))
	bot := runFeedbackBot(t, pool, fake, svc)
	return tgEnv{pool: pool, fake: fake, gh: gh, store: store, svc: svc, bot: bot}
}

// requireReply требует, чтобы сообщение было sendMessage с текстом want
// ответом на replyTo в теме thread.
func requireReply(t *testing.T, s tgSent, want string, replyTo, thread int64) {
	t.Helper()
	if s.method != "sendMessage" || strings.TrimSpace(s.text) != want {
		t.Fatalf("ожидалось сообщение %q, получено: %s", want, describeSent([]tgSent{s}))
	}
	if replyTo != 0 && s.replyTo != replyTo {
		t.Fatalf("%q должно быть ответом на сообщение %d (reply_parameters.message_id), а ответ на %d", want, replyTo, s.replyTo)
	}
	if s.threadID != thread {
		t.Fatalf("%q должно уйти в тему %d (message_thread_id), а ушло в %d", want, thread, s.threadID)
	}
}

// --- POST /me/feedback ----------------------------------------------------

// Отзыв записывается и возвращается в статусе sent: 201, текст без
// пробелов по краям, задачи и сборки ещё нет (ФТ-1, ФТ-2, ФТ-18).
func TestFeedbackPostReturnsSent(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	token, userID := fbUser(t, env.baseURL, 1, "Анна", "anna_dacha")

	before := time.Now().Add(-time.Minute)
	fb := sendFeedback(t, env.baseURL, token, map[string]string{
		"text":        "  Лента не обновляется после поста \n ",
		"app_version": "1.0.0 (386900)",
		"device":      "Google Pixel 7, Android 14",
	}, nil)

	if fb.ID <= 0 {
		t.Fatalf("у отзыва должен быть номер, получено %d", fb.ID)
	}
	if fb.Status != "sent" {
		t.Fatalf("новый отзыв должен быть в статусе sent, он в %q", fb.Status)
	}
	if fb.Text != "Лента не обновляется после поста" {
		t.Fatalf("текст отзыва после обрезки пробелов — %q", fb.Text)
	}
	if fb.Issue != nil || fb.Build != nil {
		t.Fatalf("у нового отзыва не должно быть issue и build: %+v", fb)
	}
	created, err := time.Parse(time.RFC3339, fb.CreatedAt)
	if err != nil || created.Before(before) || created.After(time.Now().Add(time.Minute)) {
		t.Fatalf("created_at %q должен быть временем отправки", fb.CreatedAt)
	}

	rows := feedbackRows(t, "user_id = $1", userID)
	if len(rows) != 1 {
		t.Fatalf("в feedback ожидалась одна строка автора, их %d", len(rows))
	}
	r := rows[0]
	if r.source != "app" || r.status != "sent" || r.author != "Анна (@anna_dacha)" {
		t.Fatalf("строка отзыва: source %q, status %q, author %q; ожидались app, sent, «Анна (@anna_dacha)» (ФТ-18)", r.source, r.status, r.author)
	}
	if r.appVersion == nil || *r.appVersion != "1.0.0 (386900)" || r.device == nil || *r.device != "Google Pixel 7, Android 14" {
		t.Fatalf("версия и телефон должны сохраниться как присланы: %v, %v", r.appVersion, r.device)
	}
}

// Автор — «<имя> (@<ник>)», а без имени — «@<ник>» (ФТ-18).
func TestFeedbackAuthorFromProfile(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	anna, annaID := fbUser(t, env.baseURL, 1, "Анна Петрова", "anna_dacha")
	petr, petrID := fbUser(t, env.baseURL, 2, "", "petr_dacha")

	sendText(t, env.baseURL, anna, "от Анны")
	sendText(t, env.baseURL, petr, "от Петра")

	if rows := feedbackRows(t, "user_id = $1", annaID); len(rows) != 1 || rows[0].author != "Анна Петрова (@anna_dacha)" {
		t.Fatalf("автор с именем должен быть «Анна Петрова (@anna_dacha)», строки: %+v", rows)
	}
	if rows := feedbackRows(t, "user_id = $1", petrID); len(rows) != 1 || rows[0].author != "@petr_dacha" {
		t.Fatalf("автор без имени должен быть «@petr_dacha», строки: %+v", rows)
	}
}

// Пустой текст и текст длиннее 4000 символов — 400 invalid_text; ровно
// 4000 после обрезки пробелов принимается (ФТ-18).
func TestFeedbackTextValidation(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	token, _ := fbUser(t, env.baseURL, 1, "", "tester_one")

	bad := map[string]string{
		"пустой":             "",
		"одни пробелы":       "   \n\t  ",
		"4001 символ":        strings.Repeat("ж", 4001),
		"4001 после обрезки": "  " + strings.Repeat("ж", 4001) + "  ",
	}
	for name, text := range bad {
		t.Run(name, func(t *testing.T) {
			resp := postFeedback(t, env.baseURL, token, map[string]string{"text": text}, nil)
			requireError(t, resp, http.StatusBadRequest, "invalid_text")
		})
	}
	if n := countSQL(t, `SELECT count(*) FROM feedback`); n != 0 {
		t.Fatalf("отвергнутый отзыв не должен записываться, а в feedback %d строк", n)
	}

	ok := sendText(t, env.baseURL, token, "  "+strings.Repeat("ж", 4000)+"\n")
	if len([]rune(ok.Text)) != 4000 {
		t.Fatalf("текст ровно в 4000 символов должен сохраниться целиком, сохранено %d", len([]rune(ok.Text)))
	}
}

// Скриншот не картинка (или не JPEG/PNG) — 400 invalid_image; больше
// 10 МБ — 413; маленький PNG, объявляющий больше 8192×8192 точек, — 413
// image_too_large (ФТ-18). Отзыв при этом не записывается.
func TestFeedbackScreenshotValidation(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	token, _ := fbUser(t, env.baseURL, 1, "", "tester_one")

	bad := map[string]fbShot{
		"текст под видом png": {filename: "shot.png", content: []byte("это не картинка, а текст")},
		"gif":                 {filename: "shot.gif", content: imageBytes(t, "gif", 100, 100)},
	}
	for name, shot := range bad {
		t.Run(name, func(t *testing.T) {
			shot := shot
			resp := postFeedback(t, env.baseURL, token, map[string]string{"text": "скриншот"}, &shot)
			requireError(t, resp, http.StatusBadRequest, "invalid_image")
		})
	}

	t.Run("больше 10 МБ", func(t *testing.T) {
		big := imageBytes(t, "jpeg", 100, 100)
		big = append(big, make([]byte, 10<<20+1-len(big))...)
		resp := postFeedback(t, env.baseURL, token, map[string]string{"text": "скриншот"}, &fbShot{filename: "shot.jpg", content: big})
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("на скриншот больше 10 МБ ожидался статус 413, получен %d", resp.StatusCode)
		}
	})

	for name, content := range bombCases(t) {
		t.Run("объявляет "+name, func(t *testing.T) {
			resp := postFeedback(t, env.baseURL, token, map[string]string{"text": "скриншот"}, &fbShot{filename: "shot.png", content: content})
			requireError(t, resp, http.StatusRequestEntityTooLarge, "image_too_large")
		})
	}

	if n := countSQL(t, `SELECT count(*) FROM feedback`); n != 0 {
		t.Fatalf("отвергнутый отзыв не должен записываться, а в feedback %d строк", n)
	}
}

// Без токена или с чужим токеном ручки отвечают 401.
func TestFeedbackRequiresSession(t *testing.T) {
	env := startFeedback(t, fbOptions{})

	for _, token := range []string{"", "не-токен"} {
		if resp := postFeedback(t, env.baseURL, token, map[string]string{"text": "привет"}, nil); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("POST /me/feedback с токеном %q: ожидался 401, получен %d", token, resp.StatusCode)
		}
		if resp := do(t, http.MethodGet, env.baseURL+"/me/feedback", token, nil); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("GET /me/feedback с токеном %q: ожидался 401, получен %d", token, resp.StatusCode)
		}
	}
	if n := countSQL(t, `SELECT count(*) FROM feedback`); n != 0 {
		t.Fatalf("без входа отзыв не записывается, а в feedback %d строк", n)
	}
}

// app_version и device длиннее 200 символов обрезаются до 200 (ФТ-18).
func TestFeedbackMetaTruncated(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	token, userID := fbUser(t, env.baseURL, 1, "", "tester_one")

	version := strings.Repeat("в", 250)
	device := strings.Repeat("т", 199) + "Ё" + strings.Repeat("т", 50)
	sendFeedback(t, env.baseURL, token, map[string]string{"text": "длинные поля", "app_version": version, "device": device}, nil)

	rows := feedbackRows(t, "user_id = $1", userID)
	if len(rows) != 1 || rows[0].appVersion == nil || rows[0].device == nil {
		t.Fatalf("ожидалась одна строка с версией и телефоном: %+v", rows)
	}
	if got := *rows[0].appVersion; got != string([]rune(version)[:200]) {
		t.Fatalf("версия должна обрезаться до 200 символов, сохранено %d", len([]rune(got)))
	}
	if got := *rows[0].device; got != string([]rune(device)[:200]) {
		t.Fatalf("телефон должен обрезаться до 200 символов, сохранено %d: …%q", len([]rune(got)), string([]rune(got)[190:]))
	}
}

// GET /me/feedback — свои отзывы, новые сверху, не больше 50; чужих не
// видно (ФТ-19).
func TestFeedbackListOwnNewestFirst(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	anna, annaID := fbUser(t, env.baseURL, 1, "", "anna_dacha")
	petr, _ := fbUser(t, env.baseURL, 2, "", "petr_dacha")

	if items := myFeedback(t, env.baseURL, anna); len(items) != 0 {
		t.Fatalf("без отзывов список должен быть пуст, а в нём %d", len(items))
	}

	first := sendText(t, env.baseURL, anna, "первый")
	petrs := sendText(t, env.baseURL, petr, "от Петра")
	second := sendText(t, env.baseURL, anna, "второй")
	third := sendText(t, env.baseURL, anna, "третий")

	items := myFeedback(t, env.baseURL, anna)
	var ids []int64
	for _, i := range items {
		ids = append(ids, i.ID)
	}
	if fmt.Sprint(ids) != fmt.Sprint([]int64{third.ID, second.ID, first.ID}) {
		t.Fatalf("Анне ожидались её отзывы новые сверху %v, получено %v", []int64{third.ID, second.ID, first.ID}, ids)
	}
	if got := myFeedback(t, env.baseURL, petr); len(got) != 1 || got[0].ID != petrs.ID || got[0].Text != "от Петра" {
		t.Fatalf("Петру ожидался только свой отзыв, получено %+v", got)
	}

	// Ещё 55 старых отзывов Анны — в ответе не больше 50, новые сверху.
	execSQL(t, `
		INSERT INTO feedback (source, user_id, author, text, created_at)
		SELECT 'app', $1, '@anna_dacha', 'старый ' || g, now() - interval '1 day' - g * interval '1 minute'
		FROM generate_series(1, 55) g`, annaID)

	items = myFeedback(t, env.baseURL, anna)
	if len(items) != 50 {
		t.Fatalf("отзывов больше 50 — в ответе ожидалось 50, получено %d", len(items))
	}
	if items[0].ID != third.ID || items[1].ID != second.ID || items[2].ID != first.ID || items[3].Text != "старый 1" {
		t.Fatalf("новые должны идти сверху: %q, %q, %q, %q…", items[0].Text, items[1].Text, items[2].Text, items[3].Text)
	}
	if items[49].Text != "старый 47" {
		t.Fatalf("последним из 50 должен быть «старый 47», а он %q", items[49].Text)
	}
}

// --- Задача GitHub --------------------------------------------------------

// Отзыв из приложения со скриншотом сразу после приёма становится задачей:
// заголовок, тело по шаблону (номер отзыва, «есть скриншот» и ссылка на
// дашборд вместо автора и скриншота), метки; скриншот — JPEG до 1600 по
// большей стороне в хранилище. В приложении — accepted с номером
// (ФТ-4–7, ФТ-18, ФТ-33).
func TestFeedbackAppIssueInBackground(t *testing.T) {
	env := startFeedback(t, fbOptions{background: true})
	token, _ := fbUser(t, env.baseURL, 1, "Анна", "anna_dacha")

	const text = "Лента не обновляется после поста"
	fb := sendFeedback(t, env.baseURL, token, map[string]string{
		"text":        text,
		"app_version": "1.0.0 (386900)",
		"device":      "Google Pixel 7, Android 14",
	}, &fbShot{filename: "screen.png", content: imageBytes(t, "png", 3200, 1200)})
	if fb.Status != "sent" {
		t.Fatalf("ответ на POST — отзыв в статусе sent, а он в %q", fb.Status)
	}

	issues := env.gh.waitCreated(1, "задача по отзыву из приложения")
	issue := issues[0]
	if issue.Title != "Отзыв: "+text {
		t.Fatalf("заголовок задачи %q, ожидался %q (ФТ-4)", issue.Title, "Отзыв: "+text)
	}
	requireLabels(t, issue, lblInbox, lblApp)
	requireIssueBody(t, issue.Body, text,
		"Откуда: приложение",
		fbRefLine(fb.ID, true, fbPublicURL),
		"Версия: 1.0.0 (386900)",
		"Телефон: Google Pixel 7, Android 14",
	)

	// Скриншот открывается без входа и пересохранён как фото поста.
	content := downloadFile(t, env.baseURL, dashboardShot(t, env, fb.ID))
	cfg, format := photoConfig(t, content)
	if format != "jpeg" || cfg.Width != 1600 || cfg.Height != 600 {
		t.Fatalf("скриншот должен храниться JPEG 1600×600, а он %s %d×%d (ФТ-18)", format, cfg.Width, cfg.Height)
	}

	got := waitMyStatus(t, env.baseURL, token, fb.ID, "accepted")
	if got.Issue == nil || *got.Issue != issue.Number {
		t.Fatalf("в приложении у отзыва должна быть задача #%d, получено %v", issue.Number, got.Issue)
	}
	if got.Build != nil {
		t.Fatalf("build бывает только у released, а у accepted он %d", *got.Build)
	}
	rows := feedbackRows(t, "issue = $1", issue.Number)
	if len(rows) != 1 || rows[0].issueURL == nil || *rows[0].issueURL != env.gh.htmlURL(issue.Number) {
		t.Fatalf("у отзыва должна запомниться ссылка задачи %q: %+v", env.gh.htmlURL(issue.Number), rows)
	}

	// Повторный проход второй задачи не заводит (ФТ-7).
	env.sync(t)
	env.sync(t)
	if n := len(env.gh.createdIssues()); n != 1 {
		t.Fatalf("повтор не должен заводить вторую задачу, а их %d", n)
	}
}

// Задачу заводит и проход Sync; без PUBLIC_URL ссылка на дашборд —
// «/dashboard»; без версии и телефона нет их строк; без Repo — репозиторий
// по умолчанию (ФТ-3, ФТ-5, ФТ-30, ФТ-32).
func TestFeedbackIssueViaSyncDefaults(t *testing.T) {
	env := startFeedback(t, fbOptions{repo: "-", publicURL: "-"})
	token, _ := fbUser(t, env.baseURL, 1, "", "petr_dacha")

	fb := sendFeedback(t, env.baseURL, token, map[string]string{"text": "Кнопка «Опубликовать» уехала"},
		&fbShot{filename: "screen.jpg", content: imageBytes(t, "jpeg", 800, 600)})

	time.Sleep(300 * time.Millisecond)
	if n := env.gh.requestCount(); n != 0 {
		t.Fatalf("без api.Config.Feedback ручка не ходит в GitHub, а запросов %d", n)
	}

	env.sync(t)
	issues := env.gh.createdIssues()
	if len(issues) != 1 {
		t.Fatalf("Sync должен завести одну задачу, заведено %d", len(issues))
	}
	requireLabels(t, issues[0], lblInbox, lblApp)
	requireIssueBody(t, issues[0].Body, "Кнопка «Опубликовать» уехала",
		"Откуда: приложение",
		fbRefLine(fb.ID, true, ""),
	)
	downloadFile(t, env.baseURL, dashboardShot(t, env, fb.ID))

	got := waitMyStatus(t, env.baseURL, token, fb.ID, "accepted")
	if got.Issue == nil || *got.Issue != ghFirstIssue {
		t.Fatalf("у отзыва должна быть задача #%d, получено %v", ghFirstIssue, got.Issue)
	}

	env.sync(t)
	if n := len(env.gh.createdIssues()); n != 1 {
		t.Fatalf("повторный Sync не заводит вторую задачу, а их %d (ФТ-7)", n)
	}
}

// Заголовок — «Отзыв: » и первая непустая строка; длиннее 60 символов —
// первые 60 и «…» (ФТ-4).
func TestFeedbackIssueTitle(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	token, _ := fbUser(t, env.baseURL, 1, "", "tester_one")

	sixty := strings.Repeat("я", 60)
	cases := map[string]string{
		"Хочу сортировать посты по культурам":   "Отзыв: Хочу сортировать посты по культурам",
		"\n\n   Первая строка  \nвторая строка": "Отзыв: Первая строка",
		sixty:                   "Отзыв: " + sixty,
		sixty + "ю и ещё хвост": "Отзыв: " + sixty + "…",
	}
	for text := range cases {
		sendText(t, env.baseURL, token, text)
	}
	env.sync(t)

	if n := len(env.gh.createdIssues()); n != len(cases) {
		t.Fatalf("ожидалось %d задач, заведено %d", len(cases), n)
	}
	for text, title := range cases {
		issue := env.gh.createdByTitle(t, title)
		trimmed := strings.TrimSpace(text)
		if !strings.HasPrefix(issue.Body, trimmed+"\n\n") {
			t.Fatalf("задача %q: тело должно начинаться полным текстом %q:\n%s", title, trimmed, issue.Body)
		}
	}
}

// GitHub ответил ошибкой — отзыв остаётся sent; следующий проход заводит
// задачу, и в приложении видно accepted с номером (ФТ-1, ФТ-7).
func TestFeedbackGitHubFailureRetried(t *testing.T) {
	env := startFeedback(t, fbOptions{background: true})
	token, _ := fbUser(t, env.baseURL, 1, "", "tester_one")

	env.gh.setFailCreate(true)
	fb := sendText(t, env.baseURL, token, "GitHub лежит")

	deadline := time.Now().Add(5 * time.Second)
	for env.gh.tries() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("после приёма отзыва сервис за 5 секунд не попытался завести задачу (ФТ-7)")
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)

	got := findFeedback(myFeedback(t, env.baseURL, token), fb.ID)
	if got == nil || got.Status != "sent" || got.Issue != nil {
		t.Fatalf("GitHub ошибся — отзыв должен остаться sent без задачи: %+v", got)
	}

	_ = env.svc.Sync(fbCtx(t), nil) // GitHub всё ещё лежит
	if n := len(env.gh.createdIssues()); n != 0 {
		t.Fatalf("GitHub отвечает ошибкой — задач быть не должно, а их %d", n)
	}

	env.gh.setFailCreate(false)
	env.sync(t)
	issues := env.gh.createdIssues()
	if len(issues) != 1 {
		t.Fatalf("после починки GitHub Sync должен завести одну задачу, заведено %d", len(issues))
	}
	got2 := waitMyStatus(t, env.baseURL, token, fb.ID, "accepted")
	if got2.Issue == nil || *got2.Issue != issues[0].Number {
		t.Fatalf("у отзыва должна быть задача #%d, получено %v", issues[0].Number, got2.Issue)
	}

	env.sync(t)
	if n := len(env.gh.createdIssues()); n != 1 {
		t.Fatalf("повторный Sync не заводит вторую задачу, а их %d", n)
	}
}

// Без токена GitHub отзывы только копятся в базе: в GitHub никто не ходит,
// статус sent (ФТ-3).
func TestFeedbackWithoutToken(t *testing.T) {
	env := startFeedback(t, fbOptions{background: true, token: "-"})
	token, _ := fbUser(t, env.baseURL, 1, "", "tester_one")

	fb := sendText(t, env.baseURL, token, "без токена")
	if err := env.svc.Sync(fbCtx(t), nil); err != nil {
		t.Logf("Sync без токена вернул %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	if n := env.gh.requestCount(); n != 0 {
		t.Fatalf("без токена сервис не должен ходить в GitHub, а запросов %d", n)
	}
	if got := findFeedback(myFeedback(t, env.baseURL, token), fb.ID); got == nil || got.Status != "sent" {
		t.Fatalf("без токена отзыв остаётся sent: %+v", got)
	}
}

// --- Сверка статусов ------------------------------------------------------

// Sync сверяет задачи accepted, approved, done: закрыта not_planned или
// duplicate — declined; закрыта иначе — done; открыта с «одобрено»,
// «в работе» или «на проверке» при accepted — approved; иначе без
// изменений. Ошибка GitHub на одной задаче не мешает остальным (ФТ-8).
func TestFeedbackStatusSync(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	token, _ := fbUser(t, env.baseURL, 1, "", "tester_one")

	names := []string{"not_planned", "duplicate", "completed", "одобрено", "в работе", "на проверке", "входящее", "ошибка"}
	ids := map[string]int64{}
	for _, n := range names {
		ids[n] = sendText(t, env.baseURL, token, "отзыв "+n).ID
	}
	env.sync(t)

	num := map[string]int{}
	for _, n := range names {
		num[n] = env.gh.createdByTitle(t, "Отзыв: отзыв "+n).Number
	}

	env.gh.closeIssue(num["not_planned"], "not_planned")
	env.gh.closeIssue(num["duplicate"], "duplicate")
	env.gh.closeIssue(num["completed"], "completed")
	env.gh.label(num["одобрено"], "одобрено")
	env.gh.label(num["в работе"], "в работе")
	env.gh.label(num["на проверке"], "на проверке")
	env.gh.label(num["ошибка"], "одобрено")
	env.gh.setFailGet(num["ошибка"], true)

	_ = env.svc.Sync(fbCtx(t), nil) // ошибка на одной задаче — вправе вернуть ошибку

	want := map[string]string{
		"not_planned": "declined",
		"duplicate":   "declined",
		"completed":   "done",
		"одобрено":    "approved",
		"в работе":    "approved",
		"на проверке": "approved",
		"входящее":    "accepted",
		"ошибка":      "accepted",
	}
	requireStatuses := func(stage string, want map[string]string) {
		t.Helper()
		items := myFeedback(t, env.baseURL, token)
		for n, status := range want {
			fb := findFeedback(items, ids[n])
			if fb == nil {
				t.Fatalf("%s: отзыва %q нет в списке", stage, n)
			}
			if fb.Status != status {
				t.Errorf("%s: отзыв с задачей «%s» в статусе %s, ожидался %s", stage, n, fb.Status, status)
			}
			if fb.Issue == nil || *fb.Issue != num[n] {
				t.Errorf("%s: у отзыва «%s» должна остаться задача #%d, получено %v", stage, n, num[n], fb.Issue)
			}
			if fb.Build != nil {
				t.Errorf("%s: build бывает только у released, у «%s» он %d", stage, n, *fb.Build)
			}
		}
	}
	requireStatuses("первая сверка", want)

	// Вторая сверка: одобренную закрыли сделанной, отклонённую открыли
	// с «одобрено» (declined больше не сверяется), сделанную открыли
	// с «одобрено» (done → approved не бывает), GitHub починился.
	env.gh.closeIssue(num["одобрено"], "completed")
	env.gh.closeIssue(num["в работе"], "not_planned")
	env.gh.reopen(num["not_planned"])
	env.gh.label(num["not_planned"], "одобрено")
	env.gh.reopen(num["completed"])
	env.gh.label(num["completed"], "одобрено")
	env.gh.setFailGet(num["ошибка"], false)

	env.sync(t)

	want["одобрено"] = "done"
	want["в работе"] = "declined"
	want["ошибка"] = "approved"
	requireStatuses("вторая сверка", want)
}

// Release: сначала сверка, потом каждый done получает released с номером
// сборки; прочие статусы не трогаются; вышедшее в прошлой сборке номер
// не меняет (ФТ-11, ФТ-32).
func TestFeedbackRelease(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	token, _ := fbUser(t, env.baseURL, 1, "", "tester_one")

	ids := map[string]int64{}
	for _, n := range []string{"сделано", "закрыто без сверки", "одобрено", "отклонено", "ждёт"} {
		ids[n] = sendText(t, env.baseURL, token, n).ID
	}
	env.sync(t)
	num := func(n string) int { return env.gh.createdByTitle(t, "Отзыв: "+n).Number }

	env.gh.closeIssue(num("сделано"), "completed")
	env.gh.label(num("одобрено"), "одобрено")
	env.gh.closeIssue(num("отклонено"), "not_planned")
	env.sync(t)
	// Эту закрыли уже после сверки — Release сверит её сам.
	env.gh.closeIssue(num("закрыто без сверки"), "completed")

	if err := env.svc.Release(fbCtx(t), fbBuild, nil); err != nil {
		t.Fatalf("Release вернул ошибку: %v", err)
	}

	items := myFeedback(t, env.baseURL, token)
	check := func(n, status string, build int64) {
		t.Helper()
		fb := findFeedback(items, ids[n])
		if fb == nil {
			t.Fatalf("отзыва %q нет в списке", n)
		}
		if fb.Status != status {
			t.Errorf("отзыв «%s» в статусе %s, ожидался %s", n, fb.Status, status)
		}
		switch {
		case build == 0 && fb.Build != nil:
			t.Errorf("у «%s» (%s) не должно быть build, а он %d", n, status, *fb.Build)
		case build != 0 && (fb.Build == nil || *fb.Build != build):
			t.Errorf("у «%s» build должен быть %d, получено %v", n, build, fb.Build)
		}
	}
	check("сделано", "released", fbBuild)
	check("закрыто без сверки", "released", fbBuild)
	check("одобрено", "approved", 0)
	check("отклонено", "declined", 0)
	check("ждёт", "accepted", 0)

	// Следующая сборка: вышедшее остаётся с прежним номером, новое сделанное
	// получает новый.
	env.gh.closeIssue(num("одобрено"), "completed")
	if err := env.svc.Release(fbCtx(t), fbBuild+100, nil); err != nil {
		t.Fatalf("Release вернул ошибку: %v", err)
	}
	items = myFeedback(t, env.baseURL, token)
	check("сделано", "released", fbBuild)
	check("одобрено", "released", fbBuild+100)
	check("ждёт", "accepted", 0)
}

// --- Telegram: отзывы -----------------------------------------------------

// Notify пишет sendMessage в чат с message_thread_id (если тема задана)
// и reply_parameters.message_id (ФТ-31).
func TestFeedbackBotNotify(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	bot := telegram.New(pool, tgConfig(t, fake))

	if err := bot.Notify(fbCtx(t), feedback.Recipient{ChatID: tgGroupID, ThreadID: 7, ReplyTo: 42}, "в теме"); err != nil {
		t.Fatalf("Notify вернул ошибку: %v", err)
	}
	if err := bot.Notify(fbCtx(t), feedback.Recipient{ChatID: tgStrangerID, ReplyTo: 43}, "в личке"); err != nil {
		t.Fatalf("Notify вернул ошибку: %v", err)
	}

	requireReply(t, fake.waitSent(tgGroupID, 1, "Notify в тему")[0], "в теме", 42, 7)
	requireReply(t, fake.waitSent(tgStrangerID, 1, "Notify в личку")[0], "в личке", 43, 0)

	fake.setFailSend(true)
	if err := bot.Notify(fbCtx(t), feedback.Recipient{ChatID: tgStrangerID}, "не уйдёт"); err == nil {
		t.Fatal("Telegram ответил ok: false — Notify должен вернуть ошибку")
	}
}

// Сообщение в личку от кого угодно, владельца тоже, — отзыв «из телеграма»;
// автор получает номер задачи ответом на своё сообщение (ФТ-5, ФТ-6, ФТ-9,
// ФТ-21, ФТ-22).
func TestFeedbackTelegramPrivate(t *testing.T) {
	cases := []struct {
		name   string
		msg    tgMessage
		author string
	}{
		{name: "тестировщик с ником", msg: strangerPrivate("Хочу сортировать посты по культурам"), author: "@tester_vasya"},
		{name: "тестировщик без ника", msg: tgMessage{fromID: tgStrangerID, firstName: "Вася", lastName: "Пупкин", chatID: tgStrangerID, text: "Хочу сортировать посты по культурам"}, author: "Вася Пупкин"},
		{name: "владелец", msg: tgMessage{fromID: tgOwnerID, username: tgOwnerNick, chatID: tgOwnerID, text: "Хочу сортировать посты по культурам"}, author: "@" + tgOwnerNick},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := startFeedbackBot(t, true)
			msgID := env.fake.pushMessage(c.msg)

			issue := env.gh.waitCreated(1, "задача из лички")[0]
			if issue.Title != "Отзыв: Хочу сортировать посты по культурам" {
				t.Fatalf("заголовок задачи %q", issue.Title)
			}
			requireLabels(t, issue, lblInbox, lblTelegram)
			requireIssueBody(t, issue.Body, "Хочу сортировать посты по культурам",
				"Откуда: Telegram, личка",
				fbRefLine(onlyFeedbackID(t), false, fbPublicURL),
			)

			got := env.fake.waitSent(c.msg.chatID, 1, "ответ автору")
			requireReply(t, got[0], fbAccepted(issue.Number), msgID, 0)
			waitIssueStatus(t, issue.Number, "accepted")

			rows := feedbackRows(t, "issue = $1", issue.Number)
			if rows[0].source != "telegram" || rows[0].author != c.author {
				t.Fatalf("строка отзыва: source %q, author %q; ожидались telegram и %q", rows[0].source, rows[0].author, c.author)
			}
			settle(t, env.fake)
			if n := len(env.fake.sentTo(c.msg.chatID)); c.msg.chatID != tgOwnerID && n != 1 {
				t.Fatalf("автору ожидался один ответ, получено %d", n)
			}
		})
	}
}

// /ideas и /идеи владельца в группе привязывают тему для идей: роль ideas,
// чат и тема (ФТ-20). Чужая команда в группе ничего не привязывает.
func TestFeedbackTelegramIdeasBinding(t *testing.T) {
	env := startFeedbackBot(t, true)

	env.fake.pushMessage(tgMessage{fromID: tgStrangerID, username: "tester_vasya", chatID: tgGroupID, chatType: "supergroup", threadID: 7, text: "/ideas"})
	settle(t, env.fake)
	if n := countSQL(t, `SELECT count(*) FROM telegram_chats WHERE role = 'ideas'`); n != 0 {
		t.Fatalf("команда не от владельца ничего не привязывает, а строк ideas %d", n)
	}
	if got := env.fake.sentTo(tgGroupID); len(got) != 0 {
		t.Fatalf("на чужую команду в группе бот молчит, а отправил: %s", describeSent(got))
	}

	env.fake.pushMessage(tgMessage{fromID: tgOwnerID, username: tgOwnerNick, chatID: tgGroupID, chatType: "supergroup", threadID: 7, text: "/ideas@moya_dacha_bot"})
	got := env.fake.waitSent(tgGroupID, 1, "ответ на /ideas")
	if strings.TrimSpace(got[0].text) != fbIdeasBound {
		t.Fatalf("на /ideas ожидался ответ %q, получено: %s", fbIdeasBound, describeSent(got))
	}
	waitBound(t, env.pool, "ideas", tgGroupID)
	if n := countSQL(t, `SELECT count(*) FROM telegram_chats WHERE role = 'ideas' AND thread_id = 7`); n != 1 {
		t.Fatal("привязка ideas должна запомнить тему 7")
	}

	// /идеи вне темы в другой группе заменяет привязку, тема пустая.
	env.fake.pushMessage(tgMessage{fromID: tgOwnerID, username: tgOwnerNick, chatID: tgOtherGroup, chatType: "group", text: "/идеи"})
	got = env.fake.waitSent(tgOtherGroup, 1, "ответ на /идеи")
	if strings.TrimSpace(got[0].text) != fbIdeasBound {
		t.Fatalf("на /идеи ожидался ответ %q, получено: %s", fbIdeasBound, describeSent(got))
	}
	waitBound(t, env.pool, "ideas", tgOtherGroup)
	if n := countSQL(t, `SELECT count(*) FROM telegram_chats WHERE role = 'ideas' AND thread_id IS NULL`); n != 1 {
		t.Fatal("привязка без темы должна оставить thread_id пустым")
	}
	if n := countSQL(t, `SELECT count(*) FROM feedback`); n != 0 {
		t.Fatalf("команды отзывами не становятся, а в feedback %d строк", n)
	}
}

// bindIdeas привязывает тему для идей прямо в базе; thread == 0 — без темы.
func bindIdeas(t *testing.T, chatID, thread int64) {
	t.Helper()
	var th any
	if thread != 0 {
		th = thread
	}
	execSQL(t, `
		INSERT INTO telegram_chats (role, chat_id, thread_id) VALUES ('ideas', $1, $2)
		ON CONFLICT (role) DO UPDATE SET chat_id = EXCLUDED.chat_id, thread_id = EXCLUDED.thread_id`, chatID, th)
}

// В привязанной группе отзывом становится только сообщение в привязанной
// теме; другие темы, сообщения вне темы, другие группы, боты, команды
// и стикеры бот молча пропускает. Ответ — в ту же тему (ФТ-9, ФТ-21).
func TestFeedbackTelegramTopic(t *testing.T) {
	env := startFeedbackBot(t, true)
	bindIdeas(t, tgGroupID, 7)
	bindChat(t, "group", tgOtherGroup)

	env.fake.pushMessage(strangerInGroup(tgGroupID, 8, "в другой теме"))
	env.fake.pushMessage(strangerInGroup(tgGroupID, 0, "вне темы"))
	env.fake.pushMessage(strangerInGroup(tgOtherGroup, 7, "в группе сборок"))
	env.fake.pushMessage(tgMessage{fromID: 777, username: "other_bot", isBot: true, chatID: tgGroupID, chatType: "supergroup", threadID: 7, text: "я бот"})
	env.fake.pushMessage(strangerInGroup(tgGroupID, 7, "/status"))
	env.fake.pushMessage(tgMessage{fromID: tgStrangerID, username: "tester_vasya", chatID: tgGroupID, chatType: "supergroup", threadID: 7, sticker: true})
	msgID := env.fake.pushMessage(strangerInGroup(tgGroupID, 7, "Хочу сортировать посты по культурам"))

	issue := env.gh.waitCreated(1, "задача из темы")[0]
	requireLabels(t, issue, lblInbox, lblTelegram)
	requireIssueBody(t, issue.Body, "Хочу сортировать посты по культурам",
		"Откуда: Telegram, тема группы",
		fbRefLine(onlyFeedbackID(t), false, fbPublicURL),
	)
	got := env.fake.waitSent(tgGroupID, 1, "ответ в теме")
	requireReply(t, got[0], fbAccepted(issue.Number), msgID, 7)

	settle(t, env.fake)
	if n := countSQL(t, `SELECT count(*) FROM feedback`); n != 1 {
		t.Fatalf("отзывом должно стать одно сообщение, а в feedback %d строк", n)
	}
	if n := len(env.gh.createdIssues()); n != 1 {
		t.Fatalf("задача должна быть одна, а их %d", n)
	}
	if got := env.fake.sentTo(tgGroupID); len(got) != 1 {
		t.Fatalf("в группу ожидался только ответ на отзыв, отправлено: %s", describeSent(got))
	}
	if got := env.fake.sentTo(tgOtherGroup); len(got) != 0 {
		t.Fatalf("в группу сборок бот не отвечает на сообщения, отправлено: %s", describeSent(got))
	}
}

// Привязка без темы — отзывом становится любое сообщение группы (ФТ-20).
func TestFeedbackTelegramGroupWithoutTopic(t *testing.T) {
	env := startFeedbackBot(t, true)
	bindIdeas(t, tgGroupID, 0)

	first := env.fake.pushMessage(strangerInGroup(tgGroupID, 0, "первая идея"))
	env.gh.waitCreated(1, "первая задача")
	second := env.fake.pushMessage(strangerInGroup(tgGroupID, 5, "вторая идея"))
	env.gh.waitCreated(2, "вторая задача")

	got := env.fake.waitSent(tgGroupID, 2, "ответы в группе")
	a := env.gh.createdByTitle(t, "Отзыв: первая идея").Number
	b := env.gh.createdByTitle(t, "Отзыв: вторая идея").Number
	for _, s := range got {
		switch s.replyTo {
		case first:
			requireReply(t, s, fbAccepted(a), first, 0)
		case second:
			requireReply(t, s, fbAccepted(b), second, 5)
		default:
			t.Fatalf("ответ не на сообщение автора: %s", describeSent([]tgSent{s}))
		}
	}
}

// Фото: текст — подпись, без подписи — «Скриншот без подписи»; берётся
// самый большой размер через getFile и <APIURL>/file/bot<Token>/<путь>;
// не скачалось — отзыв без скриншота (ФТ-5, ФТ-21, ФТ-30).
func TestFeedbackTelegramPhoto(t *testing.T) {
	cases := []struct {
		name    string
		caption string
		text    string
		missing bool
	}{
		{name: "с подписью", caption: "Кнопка уехала за край", text: "Кнопка уехала за край"},
		{name: "без подписи", text: fbNoCaption},
		{name: "не скачалось", caption: "Фото не скачать", text: "Фото не скачать", missing: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := startFeedbackBot(t, true)
			content := photoContent(t)
			env.fake.addFile("small", "photos/small.jpg", imageBytes(t, "jpeg", 90, 45), false)
			env.fake.addFile("big", "photos/file_7.jpg", content, c.missing)

			msgID := env.fake.pushMessage(tgMessage{
				fromID: tgStrangerID, username: "tester_vasya", chatID: tgStrangerID, chatType: "private",
				caption: c.caption, photo: []string{"small", "big"},
			})

			issue := env.gh.waitCreated(1, "задача по фото")[0]
			if issue.Title != "Отзыв: "+c.text {
				t.Fatalf("заголовок задачи %q, ожидался %q", issue.Title, "Отзыв: "+c.text)
			}
			requireIssueBody(t, issue.Body, c.text, "Откуда: Telegram, личка",
				fbRefLine(onlyFeedbackID(t), !c.missing, fbPublicURL))
			requireReply(t, env.fake.waitSent(tgStrangerID, 1, "ответ автору")[0], fbAccepted(issue.Number), msgID, 0)

			asked := env.fake.askedFiles()
			if len(asked) == 0 || asked[len(asked)-1] != "big" {
				t.Fatalf("getFile должен запрашивать самый большой размер «big», запрошено %q", asked)
			}
			for _, a := range asked {
				if a == "small" {
					t.Fatalf("маленький размер фото скачивать не нужно, запрошено %q", asked)
				}
			}

			rows := feedbackRows(t, "issue = $1", issue.Number)
			stored := env.store.stored()
			if c.missing {
				if rows[0].screenshot != nil {
					t.Fatalf("фото не скачалось — скриншота у отзыва быть не должно, а он %q", *rows[0].screenshot)
				}
				return
			}
			if rows[0].screenshot == nil || len(stored) == 0 {
				t.Fatalf("скриншот должен сохраниться в хранилище: строка %v, положено файлов %d", rows[0].screenshot, len(stored))
			}
			if _, _, err := image.DecodeConfig(bytes.NewReader(stored[len(stored)-1].data)); err != nil {
				t.Fatalf("в хранилище должна лечь картинка: %v", err)
			}
		})
	}
}

// Без токена GitHub отзыв из Telegram копится в базе, а автор сразу
// получает «Спасибо! Передал разработчику.» (ФТ-3, ФТ-9).
func TestFeedbackTelegramWithoutGitHub(t *testing.T) {
	env := startFeedbackBot(t, false)

	msgID := env.fake.pushMessage(strangerPrivate("Лента пустая"))
	got := env.fake.waitSent(tgStrangerID, 1, "ответ без GitHub")
	requireReply(t, got[0], fbNoGitHubThanks, msgID, 0)

	rows := feedbackRows(t, "source = 'telegram'")
	if len(rows) != 1 || rows[0].status != "sent" || rows[0].issue != nil || rows[0].author != "@tester_vasya" {
		t.Fatalf("отзыв должен лежать в базе в статусе sent без задачи: %+v", rows)
	}
	if n := env.gh.requestCount(); n != 0 {
		t.Fatalf("без токена в GitHub не ходят, а запросов %d", n)
	}
}

// Сообщение без текста и без фото (стикер) — не отзыв (ФТ-21).
func TestFeedbackTelegramStickerIgnored(t *testing.T) {
	env := startFeedbackBot(t, true)

	env.fake.pushMessage(tgMessage{fromID: tgStrangerID, username: "tester_vasya", chatID: tgStrangerID, sticker: true})
	settle(t, env.fake)

	if n := countSQL(t, `SELECT count(*) FROM feedback`); n != 0 {
		t.Fatalf("стикер отзывом не становится, а в feedback %d строк", n)
	}
	if got := env.fake.sentTo(tgStrangerID); len(got) != 0 {
		t.Fatalf("на стикер бот молчит, а отправил: %s", describeSent(got))
	}
}

// --- Telegram: /inbox и /approve ------------------------------------------

// /inbox и /входящие — открытые задачи с «входящее», новые сверху, без PR,
// до 20; нет таких — «Входящих нет.» (ФТ-24).
func TestFeedbackTelegramInbox(t *testing.T) {
	env := startFeedbackBot(t, true)

	env.fake.pushOwnerPrivate("/inbox")
	got := env.fake.waitSent(tgOwnerID, 1, "ответ на пустой /inbox")
	if strings.TrimSpace(got[0].text) != fbInboxEmpty {
		t.Fatalf("без задач ожидался ответ %q, получено: %s", fbInboxEmpty, describeSent(got))
	}

	a := env.gh.seed("Отзыв: Лента не обновляется после поста", "open", false, lblInbox, lblApp)
	b := env.gh.seed("Отзыв: Хочу сортировать посты по культурам", "open", false, lblInbox, lblTelegram)
	env.gh.seed("Отзыв: закрытая", "closed", false, lblInbox)
	env.gh.seed("Отзыв: уже одобрена", "open", false, lblApproved)
	env.gh.seed("PR: сортировка", "open", true, lblInbox)

	for i, cmd := range []string{"/inbox", "/входящие"} {
		env.fake.pushOwnerPrivate(cmd)
		got = env.fake.waitSent(tgOwnerID, i+2, "ответ на "+cmd)
		want := strings.Join([]string{
			fbInboxHeader,
			fmt.Sprintf("#%d Отзыв: Хочу сортировать посты по культурам", b),
			fmt.Sprintf("#%d Отзыв: Лента не обновляется после поста", a),
		}, "\n")
		if strings.Join(lines(got[i+1].text), "\n") != want {
			t.Fatalf("на %s ожидалось:\n%s\nполучено:\n%s", cmd, want, got[i+1].text)
		}
	}
	q := env.gh.lastListQuery()
	if q.Get("labels") != lblInbox || q.Get("state") != "open" {
		t.Fatalf("список задач запрашивается с labels=входящее&state=open, а запрошен %v (ФТ-35)", q)
	}

	// Больше 20 — только 20 новых.
	var last int
	for i := 0; i < 22; i++ {
		last = env.gh.seed(fmt.Sprintf("Отзыв: много %d", i), "open", false, lblInbox)
	}
	env.fake.pushOwnerPrivate("/inbox")
	got = env.fake.waitSent(tgOwnerID, 4, "ответ на длинный /inbox")
	ls := lines(got[3].text)
	if len(ls) != 21 || ls[0] != fbInboxHeader {
		t.Fatalf("ожидался заголовок и 20 задач, получено %d строк:\n%s", len(ls), got[3].text)
	}
	if !strings.HasPrefix(ls[1], fmt.Sprintf("#%d ", last)) {
		t.Fatalf("первой должна идти самая новая задача #%d, а строка %q", last, ls[1])
	}
}

// /approve N и /одобрить N вешают «одобрено», отвечают «#N одобрена.»;
// отзыв этой задачи в accepted сразу становится approved, а автору
// приходит «Задачу #N одобрили…» (ФТ-10, ФТ-25).
func TestFeedbackTelegramApprove(t *testing.T) {
	env := startFeedbackBot(t, true)

	msgID := env.fake.pushMessage(strangerPrivate("Хочу сортировать посты по культурам"))
	n := env.gh.waitCreated(1, "задача из лички")[0].Number
	requireReply(t, env.fake.waitSent(tgStrangerID, 1, "номер задачи")[0], fbAccepted(n), msgID, 0)
	waitIssueStatus(t, n, "accepted")

	env.fake.pushOwnerPrivate(fmt.Sprintf("/approve %d", n))
	got := env.fake.waitSent(tgOwnerID, 1, "ответ на /approve")
	if strings.TrimSpace(got[0].text) != fmt.Sprintf("#%d одобрена.", n) {
		t.Fatalf("на /approve ожидался ответ %q, получено: %s", fmt.Sprintf("#%d одобрена.", n), describeSent(got))
	}
	calls := env.gh.labelRequests()
	if len(calls) != 1 || calls[0].Number != n || fmt.Sprint(calls[0].Labels) != fmt.Sprint([]string{lblApproved}) {
		t.Fatalf("ожидался один запрос меток {\"labels\": [\"одобрено\"]} на #%d, были %+v", n, calls)
	}
	if env.gh.issue(n).hasLabel(lblInbox) != true {
		t.Fatal("метку «входящее» бот не снимает (ФТ-25)")
	}
	waitIssueStatus(t, n, "approved")
	author := env.fake.waitSent(tgStrangerID, 2, "сообщение автору об одобрении")
	if strings.TrimSpace(author[1].text) != fbApprovedNote(n) {
		t.Fatalf("автору ожидалось %q, получено: %s", fbApprovedNote(n), describeSent(author))
	}

	// Задача без отзыва: метка и ответ есть, писать некому.
	other := env.gh.seed("Отзыв: от владельца", "open", false, lblInbox)
	env.fake.pushOwnerPrivate(fmt.Sprintf("/одобрить %d", other))
	got = env.fake.waitSent(tgOwnerID, 2, "ответ на /одобрить")
	if strings.TrimSpace(got[1].text) != fmt.Sprintf("#%d одобрена.", other) {
		t.Fatalf("на /одобрить ожидался ответ %q, получено: %s", fmt.Sprintf("#%d одобрена.", other), describeSent(got))
	}
	if !env.gh.issue(other).hasLabel(lblApproved) {
		t.Fatalf("на #%d должна появиться метка «одобрено»", other)
	}

	// Повторный Sync второй раз автору не пишет.
	if err := env.svc.Sync(fbCtx(t), env.bot); err != nil {
		t.Fatalf("Sync вернул ошибку: %v", err)
	}
	settle(t, env.fake)
	if got := env.fake.sentTo(tgStrangerID); len(got) != 2 {
		t.Fatalf("автору ожидалось два сообщения — номер и одобрение, получено: %s", describeSent(got))
	}
}

// Ошибки /approve: нет числа, задачи нет в GitHub, команда не от владельца
// (ФТ-23, ФТ-25).
func TestFeedbackTelegramApproveErrors(t *testing.T) {
	env := startFeedbackBot(t, true)
	existing := env.gh.seed("Отзыв: есть", "open", false, lblInbox)

	replies := []struct{ cmd, want string }{
		{"/approve", fbNeedNumber},
		{"/одобрить", fbNeedNumber},
		{"/approve abc", fbNeedNumber},
		{"/approve 999", "Задачи #999 нет."},
	}
	for i, r := range replies {
		env.fake.pushOwnerPrivate(r.cmd)
		got := env.fake.waitSent(tgOwnerID, i+1, "ответ на "+r.cmd)
		if strings.TrimSpace(got[i].text) != r.want {
			t.Fatalf("на %q ожидался ответ %q, получено %q", r.cmd, r.want, got[i].text)
		}
	}

	env.fake.pushMessage(strangerPrivate(fmt.Sprintf("/approve %d", existing)))
	got := env.fake.waitSent(tgStrangerID, 1, "ответ чужому на /approve")
	if strings.TrimSpace(got[0].text) != tgStrangerText {
		t.Fatalf("чужому на /approve ожидался ответ %q, получено %q", tgStrangerText, got[0].text)
	}
	settle(t, env.fake)
	if calls := env.gh.labelRequests(); len(calls) != 0 {
		t.Fatalf("метки ставятся только по команде владельца, а запросы были: %+v", calls)
	}
	if env.gh.issue(existing).hasLabel(lblApproved) {
		t.Fatal("чужая команда не одобряет задачу")
	}
	if n := countSQL(t, `SELECT count(*) FROM feedback`); n != 0 {
		t.Fatalf("команды отзывами не становятся, а в feedback %d строк", n)
	}
}

// Без токена GitHub и без сервиса отзывов /inbox и /approve отвечают
// «GitHub не настроен…»; без сервиса сообщения отзывами не становятся
// (ФТ-26, ФТ-34).
func TestFeedbackTelegramNotConfigured(t *testing.T) {
	t.Run("без токена", func(t *testing.T) {
		env := startFeedbackBot(t, false)
		for i, cmd := range []string{"/inbox", "/входящие", "/approve 71", "/одобрить 71"} {
			env.fake.pushOwnerPrivate(cmd)
			got := env.fake.waitSent(tgOwnerID, i+1, "ответ на "+cmd)
			if strings.TrimSpace(got[i].text) != fbNoGitHub {
				t.Fatalf("на %s без токена ожидался ответ %q, получено %q", cmd, fbNoGitHub, got[i].text)
			}
		}
		if n := env.gh.requestCount(); n != 0 {
			t.Fatalf("без токена в GitHub не ходят, а запросов %d", n)
		}
	})

	t.Run("без сервиса отзывов", func(t *testing.T) {
		pool := tgPool(t)
		fake := newFakeTelegram(t)
		runBot(t, pool, fake)

		for i, cmd := range []string{"/inbox", "/approve 71"} {
			fake.pushOwnerPrivate(cmd)
			got := fake.waitSent(tgOwnerID, i+1, "ответ на "+cmd)
			if strings.TrimSpace(got[i].text) != fbNoGitHub {
				t.Fatalf("на %s без сервиса ожидался ответ %q, получено %q", cmd, fbNoGitHub, got[i].text)
			}
		}
		fake.pushMessage(strangerPrivate("идея без сервиса"))
		settle(t, fake)
		if n := countSQL(t, `SELECT count(*) FROM feedback`); n != 0 {
			t.Fatalf("без сервиса отзывы из Telegram не принимаются, а в feedback %d строк", n)
		}
		if got := fake.sentTo(tgStrangerID); len(got) != 0 {
			t.Fatalf("без сервиса бот молчит на сообщение, а отправил: %s", describeSent(got))
		}
	})
}

// --- Уведомления автору ---------------------------------------------------

// Sync с ботом: accepted → approved — автору «Задачу #N одобрили…» в ту же
// тему; declined и done — молча; Release — «Задача #N вышла в сборке B…».
// Автору из приложения бот не пишет (ФТ-10–12, ФТ-32).
func TestFeedbackTelegramNotifications(t *testing.T) {
	env := startFeedbackBot(t, true)
	bindIdeas(t, tgGroupID, 7)

	env.fake.pushMessage(strangerPrivate("идея из лички"))
	env.gh.waitCreated(1, "задача из лички")
	env.fake.pushMessage(strangerInGroup(tgGroupID, 7, "идея из темы"))
	env.gh.waitCreated(2, "задача из темы")
	env.fake.pushMessage(strangerPrivate("ненужная идея"))
	env.gh.waitCreated(3, "третья задача")

	priv := env.gh.createdByTitle(t, "Отзыв: идея из лички").Number
	topic := env.gh.createdByTitle(t, "Отзыв: идея из темы").Number
	decl := env.gh.createdByTitle(t, "Отзыв: ненужная идея").Number
	env.fake.waitSent(tgStrangerID, 2, "номера задач в личке")
	env.fake.waitSent(tgGroupID, 1, "номер задачи в теме")
	waitIssueStatus(t, priv, "accepted")
	waitIssueStatus(t, topic, "accepted")
	waitIssueStatus(t, decl, "accepted")

	// Отзыв из приложения — прямо в базе: бот его автору не пишет.
	app := env.gh.seed("Отзыв: из приложения", "open", false, lblInbox, lblApp)
	execSQL(t, `INSERT INTO feedback (source, author, text, status, issue, issue_url)
		VALUES ('app', '@anna_dacha', 'из приложения', 'accepted', $1, $2)`, app, env.gh.htmlURL(app))

	env.gh.label(priv, lblApproved)
	env.gh.label(topic, "в работе")
	env.gh.label(app, lblApproved)
	env.gh.closeIssue(decl, "not_planned")
	before := len(env.fake.allSent())

	if err := env.svc.Sync(fbCtx(t), env.bot); err != nil {
		t.Fatalf("Sync вернул ошибку: %v", err)
	}
	priv2 := env.fake.waitSent(tgStrangerID, 3, "одобрение в личку")
	requireReply(t, priv2[2], fbApprovedNote(priv), 0, 0)
	topic2 := env.fake.waitSent(tgGroupID, 2, "одобрение в тему")
	requireReply(t, topic2[1], fbApprovedNote(topic), 0, 7)
	if n := len(env.fake.allSent()) - before; n != 2 {
		t.Fatalf("после сверки ожидалось два сообщения — одобрения, отправлено %d: %s", n, describeSent(env.fake.allSent()[before:]))
	}
	if s := statusOfIssue(t, decl); s != "declined" {
		t.Fatalf("отклонённый отзыв должен стать declined, он %s", s)
	}
	if s := statusOfIssue(t, app); s != "approved" {
		t.Fatalf("отзыв из приложения должен стать approved, он %s", s)
	}

	// Сделано: done молча, потом Release пишет о выходе.
	env.gh.closeIssue(priv, "completed")
	env.gh.closeIssue(app, "completed")
	env.gh.closeIssue(topic, "completed")
	before = len(env.fake.allSent())
	if err := env.svc.Sync(fbCtx(t), env.bot); err != nil {
		t.Fatalf("Sync вернул ошибку: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if n := len(env.fake.allSent()) - before; n != 0 {
		t.Fatalf("переход в done автору ничего не шлёт, а отправлено: %s", describeSent(env.fake.allSent()[before:]))
	}
	if s := statusOfIssue(t, priv); s != "done" {
		t.Fatalf("закрытая сделанной задача — done, а отзыв %s", s)
	}

	if err := env.svc.Release(fbCtx(t), fbBuild, env.bot); err != nil {
		t.Fatalf("Release вернул ошибку: %v", err)
	}
	priv3 := env.fake.waitSent(tgStrangerID, 4, "выход в сборке в личку")
	requireReply(t, priv3[3], fbReleasedNote(priv, fbBuild), 0, 0)
	topic3 := env.fake.waitSent(tgGroupID, 3, "выход в сборке в тему")
	requireReply(t, topic3[2], fbReleasedNote(topic, fbBuild), 0, 7)
	if n := len(env.fake.allSent()) - before; n != 2 {
		t.Fatalf("после Release ожидалось два сообщения, отправлено: %s", describeSent(env.fake.allSent()[before:]))
	}
	for _, n := range []int{priv, topic, app} {
		if s := statusOfIssue(t, n); s != "released" {
			t.Fatalf("отзыв задачи #%d после Release должен быть released, он %s", n, s)
		}
	}
	if rows := feedbackRows(t, "issue = $1", app); rows[0].build == nil || *rows[0].build != fbBuild {
		t.Fatalf("у вышедшего отзыва должна запомниться сборка %d: %+v", fbBuild, rows[0])
	}
	if s := statusOfIssue(t, decl); s != "declined" {
		t.Fatalf("отклонённый после Release остаётся declined, он %s", s)
	}

	// Повтор Release ничего не шлёт.
	before = len(env.fake.allSent())
	if err := env.svc.Release(fbCtx(t), fbBuild+1, env.bot); err != nil {
		t.Fatalf("Release вернул ошибку: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if n := len(env.fake.allSent()) - before; n != 0 {
		t.Fatalf("вышедшее второй раз не объявляется, а отправлено: %s", describeSent(env.fake.allSent()[before:]))
	}
}

// Не ушёл ответ автору — статус всё равно меняется, повторной отправки нет;
// без бота (n == nil) — то же (ФТ-13, ФТ-32).
func TestFeedbackTelegramNotifyFailureNoRetry(t *testing.T) {
	env := startFeedbackBot(t, true)

	env.fake.pushMessage(strangerPrivate("первая"))
	a := env.gh.waitCreated(1, "первая задача")[0].Number
	env.fake.pushMessage(strangerPrivate("вторая"))
	b := env.gh.waitCreated(2, "вторая задача")[1].Number
	env.fake.waitSent(tgStrangerID, 2, "номера задач")
	waitIssueStatus(t, a, "accepted")
	waitIssueStatus(t, b, "accepted")

	env.gh.label(a, lblApproved)
	env.fake.setFailSend(true)
	_ = env.svc.Sync(fbCtx(t), env.bot)
	env.fake.setFailSend(false)
	if s := statusOfIssue(t, a); s != "approved" {
		t.Fatalf("ответ не ушёл, но статус должен смениться на approved, он %s", s)
	}

	env.gh.label(b, lblApproved)
	if err := env.svc.Sync(fbCtx(t), nil); err != nil {
		t.Fatalf("Sync без бота вернул ошибку: %v", err)
	}
	if s := statusOfIssue(t, b); s != "approved" {
		t.Fatalf("без бота статус всё равно меняется на approved, он %s", s)
	}

	if err := env.svc.Sync(fbCtx(t), env.bot); err != nil {
		t.Fatalf("Sync вернул ошибку: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if got := env.fake.sentTo(tgStrangerID); len(got) != 2 {
		t.Fatalf("повторной отправки нет: автору ожидались только два номера задач, получено: %s", describeSent(got))
	}
}

// --- Дашборд --------------------------------------------------------------

// В /dashboard/data поле feedback: сколько за 7 дней и последние 10 —
// номер, время, откуда, автор, текст до 140 символов, статус, задача,
// ссылка на скриншот (ФТ-27).
func TestFeedbackDashboard(t *testing.T) {
	env := startFeedback(t, fbOptions{})
	token, _ := fbUser(t, env.baseURL, 1, "Анна", "anna_dacha")

	// Два старых — за пределами недели.
	execSQL(t, `
		INSERT INTO feedback (source, author, text, created_at)
		SELECT 'telegram', '@old', 'старый ' || g, now() - interval '8 days' - g * interval '1 hour'
		FROM generate_series(1, 2) g`)
	// Двенадцать за неделю: i часов назад.
	execSQL(t, `
		INSERT INTO feedback (source, author, text, created_at)
		SELECT CASE WHEN g % 2 = 0 THEN 'app' ELSE 'telegram' END, '@tester_' || g, 'недавний ' || g,
		       now() - g * interval '1 hour'
		FROM generate_series(1, 12) g`)
	execSQL(t, `UPDATE feedback SET status = 'accepted', issue = 71, issue_url = 'https://github.com/tester/dacha-feedback/issues/71'
		WHERE text = 'недавний 1'`)

	long := strings.Repeat("д", 300)
	fresh := sendFeedback(t, env.baseURL, token, map[string]string{"text": long},
		&fbShot{filename: "screen.png", content: imageBytes(t, "png", 400, 300)})

	var body struct {
		Feedback *struct {
			Week   *int             `json:"week"`
			Recent []map[string]any `json:"recent"`
		} `json:"feedback"`
	}
	if err := json.Unmarshal(dashboardRaw(t, env.root), &body); err != nil {
		t.Fatalf("данные дашборда не разобрались: %v", err)
	}
	if body.Feedback == nil || body.Feedback.Week == nil {
		t.Fatal("в /dashboard/data должно быть поле feedback с week")
	}
	if *body.Feedback.Week != 13 {
		t.Fatalf("за 7 дней пришло 13 отзывов, а week = %d", *body.Feedback.Week)
	}
	recent := body.Feedback.Recent
	if len(recent) != 10 {
		t.Fatalf("recent — последние 10, а их %d", len(recent))
	}

	for i, item := range recent {
		for _, key := range []string{"id", "created_at", "source", "author", "text", "status", "issue", "issue_url", "screenshot_url"} {
			if _, ok := item[key]; !ok {
				t.Fatalf("в recent[%d] нет поля %q: %v", i, key, item)
			}
		}
		if id, ok := item["id"].(float64); !ok || id <= 0 {
			t.Fatalf("recent[%d].id должен быть номером отзыва, а он %v", i, item["id"])
		}
		if _, err := time.Parse(time.RFC3339, fmt.Sprint(item["created_at"])); err != nil {
			t.Fatalf("recent[%d].created_at %q не время RFC 3339", i, item["created_at"])
		}
	}

	first := recent[0]
	text := fmt.Sprint(first["text"])
	if n := len([]rune(text)); n > 141 || n < 139 || !strings.HasPrefix(long, strings.TrimSuffix(text, "…")) {
		t.Fatalf("текст на дашборде обрезается до 140 символов, а он %d: %q", n, text)
	}
	if first["source"] != "app" || first["author"] != "Анна (@anna_dacha)" || first["status"] != "sent" {
		t.Fatalf("первым должен идти свежий отзыв из приложения: %v", first)
	}
	if first["issue"] != nil || first["issue_url"] != nil {
		t.Fatalf("пока задачи нет, issue и issue_url — null: %v", first)
	}
	if first["id"] != float64(fresh.ID) {
		t.Fatalf("id на дашборде — номер отзыва %d, а он %v", fresh.ID, first["id"])
	}
	shotURL, ok := first["screenshot_url"].(string)
	if !ok || !strings.HasPrefix(shotURL, "/media/") {
		t.Fatalf("screenshot_url у отзыва со скриншотом — ссылка хранилища «/media/…», а он %v", first["screenshot_url"])
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(downloadFile(t, env.baseURL, shotURL))); err != nil {
		t.Fatalf("по screenshot_url должна отдаваться картинка: %v", err)
	}

	withIssue := recent[1]
	if withIssue["text"] != "недавний 1" || withIssue["source"] != "telegram" || withIssue["author"] != "@tester_1" || withIssue["status"] != "accepted" {
		t.Fatalf("вторым должен идти «недавний 1»: %v", withIssue)
	}
	if withIssue["issue"] != float64(71) || withIssue["issue_url"] != "https://github.com/tester/dacha-feedback/issues/71" {
		t.Fatalf("у отзыва с задачей — issue 71 и ссылка: %v", withIssue)
	}
	if withIssue["screenshot_url"] != nil {
		t.Fatalf("без скриншота screenshot_url — null: %v", withIssue)
	}
	for i := 2; i < 10; i++ {
		if want := fmt.Sprintf("недавний %d", i); recent[i]["text"] != want {
			t.Fatalf("recent[%d] должен быть %q (новые сверху), а он %v", i, want, recent[i]["text"])
		}
	}
}
