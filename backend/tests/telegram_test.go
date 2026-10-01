package tests

// Тесты Telegram-бота (specs/018-telegram-bot.md).
//
// Бот — не HTTP-ручка сервиса, а клиент Telegram Bot API. Поэтому тест
// обращается к нему через Go-API из требований 20–22 (telegram.New, Run,
// CheckAlerts, SendBuild), а сам Telegram подменяет фейковым сервером
// на httptest: тот отдаёт обновления на getUpdates и записывает, что бот
// отправил через sendMessage и sendDocument. База — настоящая Postgres;
// привязки чатов, ошибки сервера и измерения тест кладёт прямо в таблицы
// telegram_chats, server_errors и server_samples из «Модели данных».

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/telegram"
)

// Участники переписки с ботом.
const (
	tgToken = "t"

	// tgOwnerNick — ник владельца в настройках (TELEGRAM_OWNER, ФТ-2).
	tgOwnerNick = "Owner_Nick"

	tgOwnerID    int64 = 1001       // владелец; его личка — чат с тем же номером
	tgStrangerID int64 = 2002       // тестировщик, не владелец
	tgImpostorID int64 = 3003       // занял ник владельца после привязки лички (ФТ-2а)
	tgGroupID    int64 = -100200300 // группа тестировщиков
	tgOtherGroup int64 = -100900900 // другая группа, для перепривязки
)

// Тексты из спецификации.
const (
	tgOwnerBound   = "Готово: сюда будут приходить тревоги и тестовые сборки."
	tgGroupBound   = "Готово: сюда будут приходить сборки приложения."
	tgStrangerText = "Здравствуйте! Это бот МоейДачи. Напишите сюда идею или что сломалось — я передам разработчику и пришлю номер задачи."
	tgAlertHeader  = "Тревога на проде:"
	tgAllClear     = "Всё в порядке: тревог больше нет."
)

// tgBuildInfo — сведения о сборке в форме specs/017-app-updates.md.
const tgBuildInfo = `{
  "version": "1.0.0",
  "build": 386900,
  "date": "2026-09-26T12:00:00Z",
  "commit": "aa652ef",
  "whatsNew": ["Лента друзей", "Починили лайки"]
}`

// --- Фейковый Telegram ----------------------------------------------------

// tgSent — одно сообщение, которое бот отправил в Telegram.
type tgSent struct {
	method   string // sendMessage или sendDocument
	chatID   int64
	text     string // text у sendMessage, caption у sendDocument
	filename string // имя файла document
	content  []byte // содержимое файла document
	threadID int64  // message_thread_id у sendMessage; 0 — не задан (019, ФТ-31)
	replyTo  int64  // reply_parameters.message_id у sendMessage; 0 — не задан
}

// fakeTelegram — Telegram Bot API в миниатюре.
type fakeTelegram struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	updates  []map[string]any // очередь обновлений; каждое отдаётся один раз
	nextID   int64
	sent     []tgSent
	failSend bool // отвечать {"ok": false} на отправку
	wrong    []string
	arrived  chan struct{}
	closing  chan struct{}

	// Файлы для getFile и скачивания <APIURL>/file/bot<Token>/<путь>
	// (specs/019-feedback.md, ФТ-21); заполняет feedback_test.go.
	files     map[string]tgFile // по file_id
	fileAsked []string          // file_id из запросов getFile

	// Ответы getChatMember (specs/019-feedback.md, ФТ-21а); заполняет
	// feedback_test.go. Кого нет в members — «member».
	members     map[int64]tgMember // по user_id
	failMembers bool               // отвечать {"ok": false} на getChatMember
	memberAsked []tgMemberCall

	// Вызовы setMyCommands (ФТ-23–25); заполняет serveSetMyCommands.
	commands     []tgCommands
	failCommands bool // отвечать {"ok": false} на setMyCommands
}

// tgCommand — одна команда меню бота.
type tgCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// tgCommands — один вызов setMyCommands.
type tgCommands struct {
	list        []tgCommand
	scopeType   string // scope.type; пусто — scope не задан
	scopeChatID int64  // scope.chat_id
}

func newFakeTelegram(t *testing.T) *fakeTelegram {
	t.Helper()

	f := &fakeTelegram{
		t:       t,
		nextID:  100,
		arrived: make(chan struct{}, 1),
		closing: make(chan struct{}),
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(func() {
		close(f.closing)
		f.srv.Close()
	})
	t.Cleanup(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, w := range f.wrong {
			t.Errorf("фейковый Telegram: %s", w)
		}
	})

	return f
}

func (f *fakeTelegram) URL() string { return f.srv.URL }

// setFailSend — Telegram начинает отвечать ошибкой на отправку.
func (f *fakeTelegram) setFailSend(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failSend = fail
}

// push кладёт в очередь сообщение от человека в чат.
func (f *fakeTelegram) push(fromID int64, username string, chatID int64, chatType, text string) {
	f.pushFields(fromID, username, chatID, chatType, map[string]any{"text": text})
}

// pushContact — человек присылает в чат контакт из телефонной книги:
// сообщение с contact.phone_number и без text (018, ФТ-27–28).
func (f *fakeTelegram) pushContact(fromID int64, username string, chatID int64, chatType, phone string) {
	f.pushFields(fromID, username, chatID, chatType, map[string]any{
		"contact": map[string]any{"phone_number": phone, "first_name": "Сосед"},
	})
}

// pushFields кладёт в очередь сообщение с этими полями (text, contact…).
func (f *fakeTelegram) pushFields(fromID int64, username string, chatID int64, chatType string, fields map[string]any) {
	f.mu.Lock()
	f.nextID++
	id := f.nextID

	from := map[string]any{"id": fromID, "is_bot": false, "first_name": "Человек"}
	if username != "" {
		from["username"] = username
	}
	chat := map[string]any{"id": chatID, "type": chatType}
	if chatType == "private" {
		chat["first_name"] = "Человек"
		if username != "" {
			chat["username"] = username
		}
	} else {
		chat["title"] = "Тестировщики МоейДачи"
	}

	msg := map[string]any{
		"message_id": id,
		"from":       from,
		"chat":       chat,
		"date":       time.Now().Unix(),
	}
	for k, v := range fields {
		msg[k] = v
	}
	f.updates = append(f.updates, map[string]any{"update_id": id, "message": msg})
	f.mu.Unlock()

	select {
	case f.arrived <- struct{}{}:
	default:
	}
}

// pushOwnerPrivate — владелец пишет боту в личку.
func (f *fakeTelegram) pushOwnerPrivate(text string) {
	f.push(tgOwnerID, tgOwnerNick, tgOwnerID, "private", text)
}

// sentTo — всё, что бот отправил в чат.
func (f *fakeTelegram) sentTo(chatID int64) []tgSent {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []tgSent
	for _, s := range f.sent {
		if s.chatID == chatID {
			out = append(out, s)
		}
	}
	return out
}

// allSent — всё, что бот отправил куда угодно.
func (f *fakeTelegram) allSent() []tgSent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]tgSent(nil), f.sent...)
}

// waitSent ждёт, пока в чат придёт n сообщений, и возвращает их.
func (f *fakeTelegram) waitSent(chatID int64, n int, what string) []tgSent {
	f.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		got := f.sentTo(chatID)
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			f.t.Fatalf("%s: за 5 секунд в чат %d пришло %d сообщений из %d ожидаемых: %s",
				what, chatID, len(got), n, describeSent(got))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (f *fakeTelegram) serve(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/file/bot"+tgToken+"/") {
		f.serveFile(w, r)
		return
	}
	prefix := "/bot" + tgToken + "/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		f.mu.Lock()
		f.wrong = append(f.wrong, fmt.Sprintf("запрос не по адресу <APIURL>/bot<Token>/<метод>: %s %s", r.Method, r.URL.Path))
		f.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	method := strings.TrimPrefix(r.URL.Path, prefix)

	switch method {
	case "getUpdates":
		f.serveUpdates(w, r)
	case "sendMessage":
		f.serveSendMessage(w, r)
	case "sendDocument":
		f.serveSendDocument(w, r)
	case "getFile":
		f.serveGetFile(w, r)
	case "setMyCommands":
		f.serveSetMyCommands(w, r)
	case "getChatMember":
		f.serveGetChatMember(w, r)
	case "getMe":
		tgReply(w, map[string]any{"id": 999, "is_bot": true, "first_name": "МояДача", "username": "moya_dacha_bot"})
	default:
		tgReply(w, true)
	}
}

func (f *fakeTelegram) serveUpdates(w http.ResponseWriter, r *http.Request) {
	var params struct {
		Offset  int64 `json:"offset"`
		Timeout int64 `json:"timeout"`
	}
	body, _ := io.ReadAll(r.Body)
	if len(body) > 0 && json.Unmarshal(body, &params) != nil {
		// Бот вправе прислать параметры формой — принимаем и так.
		_ = r.ParseForm()
		params.Offset, _ = strconv.ParseInt(r.Form.Get("offset"), 10, 64)
	}
	if q := r.URL.Query().Get("offset"); q != "" && params.Offset == 0 {
		params.Offset, _ = strconv.ParseInt(q, 10, 64)
	}

	// Long polling: ждём обновлений не дольше секунды — тесту больше
	// не нужно, а отмена бота не должна висеть на опросе.
	hold := time.NewTimer(time.Second)
	defer hold.Stop()
	for {
		if batch := f.take(params.Offset); len(batch) > 0 {
			tgReply(w, batch)
			return
		}
		select {
		case <-f.arrived:
		case <-hold.C:
			tgReply(w, []any{})
			return
		case <-r.Context().Done():
			return
		case <-f.closing:
			tgReply(w, []any{})
			return
		}
	}
}

// take забирает из очереди обновления с номером не меньше offset.
// Отданное обновление второй раз не отдаётся.
func (f *fakeTelegram) take(offset int64) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Отданное и подтверждённое (номер меньше offset) из очереди уходит.
	var batch []map[string]any
	for _, u := range f.updates {
		if u["update_id"].(int64) >= offset {
			batch = append(batch, u)
		}
	}
	f.updates = nil
	return batch
}

func (f *fakeTelegram) serveSendMessage(w http.ResponseWriter, r *http.Request) {
	var params struct {
		ChatID          json.RawMessage `json:"chat_id"`
		Text            string          `json:"text"`
		MessageThreadID int64           `json:"message_thread_id"`
		ReplyParameters *struct {
			MessageID int64 `json:"message_id"`
		} `json:"reply_parameters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		f.mu.Lock()
		f.wrong = append(f.wrong, fmt.Sprintf("sendMessage: тело не JSON (ФТ-22): %v", err))
		f.mu.Unlock()
		http.Error(w, `{"ok":false,"description":"bad json"}`, http.StatusBadRequest)
		return
	}

	s := tgSent{method: "sendMessage", chatID: f.chatID(string(params.ChatID)), text: params.Text, threadID: params.MessageThreadID}
	if params.ReplyParameters != nil {
		s.replyTo = params.ReplyParameters.MessageID
	}
	f.record(w, s)
}

func (f *fakeTelegram) serveSendDocument(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		f.mu.Lock()
		f.wrong = append(f.wrong, fmt.Sprintf("sendDocument: тело не multipart (ФТ-22): %v", err))
		f.mu.Unlock()
		http.Error(w, `{"ok":false,"description":"bad multipart"}`, http.StatusBadRequest)
		return
	}

	s := tgSent{
		method: "sendDocument",
		chatID: f.chatID(r.FormValue("chat_id")),
		text:   r.FormValue("caption"),
	}
	file, header, err := r.FormFile("document")
	if err != nil {
		f.mu.Lock()
		f.wrong = append(f.wrong, fmt.Sprintf("sendDocument без файла document: %v", err))
		f.mu.Unlock()
	} else {
		defer file.Close()
		s.filename = header.Filename
		s.content, _ = io.ReadAll(file)
	}

	f.record(w, s)
}

func (f *fakeTelegram) serveSetMyCommands(w http.ResponseWriter, r *http.Request) {
	var params struct {
		Commands []tgCommand `json:"commands"`
		Scope    *struct {
			Type   string          `json:"type"`
			ChatID json.RawMessage `json:"chat_id"`
		} `json:"scope"`
	}
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &params); err != nil {
		f.mu.Lock()
		f.wrong = append(f.wrong, fmt.Sprintf("setMyCommands: тело не JSON: %v", err))
		f.mu.Unlock()
		http.Error(w, `{"ok":false,"description":"bad json"}`, http.StatusBadRequest)
		return
	}

	c := tgCommands{list: params.Commands}
	if params.Scope != nil {
		c.scopeType = params.Scope.Type
		if len(params.Scope.ChatID) > 0 {
			c.scopeChatID = f.chatID(string(params.Scope.ChatID))
		}
	}

	f.mu.Lock()
	f.commands = append(f.commands, c)
	fail := f.failCommands
	f.mu.Unlock()

	if fail {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: BOT_COMMANDS_TOO_MUCH"}`))
		return
	}
	tgReply(w, true)
}

// setFailCommands — Telegram начинает отвечать ошибкой на setMyCommands.
func (f *fakeTelegram) setFailCommands(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failCommands = fail
}

// commandCalls — все вызовы setMyCommands по порядку.
func (f *fakeTelegram) commandCalls() []tgCommands {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]tgCommands(nil), f.commands...)
}

// waitCommands ждёт, пока setMyCommands вызовут n раз, и возвращает вызовы.
func (f *fakeTelegram) waitCommands(n int, what string) []tgCommands {
	f.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		got := f.commandCalls()
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			f.t.Fatalf("%s: за 5 секунд setMyCommands вызвали %d раз из %d ожидаемых", what, len(got), n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// chatID разбирает chat_id: число или строка с числом.
func (f *fakeTelegram) chatID(raw string) int64 {
	raw = strings.Trim(strings.TrimSpace(raw), `"`)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		f.mu.Lock()
		f.wrong = append(f.wrong, fmt.Sprintf("chat_id не число: %q", raw))
		f.mu.Unlock()
	}
	return id
}

func (f *fakeTelegram) record(w http.ResponseWriter, s tgSent) {
	f.mu.Lock()
	fail := f.failSend
	if !fail {
		f.sent = append(f.sent, s)
	}
	id := len(f.sent)
	f.mu.Unlock()

	if fail {
		// Ответ 200, но ok: false — ошибкой считается именно он (ФТ-22).
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot was kicked from the group chat"}`))
		return
	}
	tgReply(w, map[string]any{
		"message_id": id,
		"date":       time.Now().Unix(),
		"chat":       map[string]any{"id": s.chatID, "type": "private"},
		"text":       s.text,
	})
}

func tgReply(w http.ResponseWriter, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
}

func describeSent(list []tgSent) string {
	if len(list) == 0 {
		return "ничего"
	}
	parts := make([]string, 0, len(list))
	for _, s := range list {
		p := fmt.Sprintf("%s в %d: %q", s.method, s.chatID, s.text)
		if s.filename != "" {
			p += fmt.Sprintf(" (файл %s)", s.filename)
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, "; ")
}

// --- Хелперы --------------------------------------------------------------

// tgConfig — настройки бота, смотрящего в фейковый Telegram.
func tgConfig(t *testing.T, fake *fakeTelegram) telegram.Config {
	return telegram.Config{
		Token:       tgToken,
		APIURL:      fake.URL(),
		Owner:       tgOwnerNick,
		DiskPath:    t.TempDir(),
		PollTimeout: time.Second,
	}
}

// tgPool — пул к чистой базе.
func tgPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := connect(t)
	truncateAll(t, pool)
	return pool
}

// runBot запускает бота с Run в горутине; отмена — в t.Cleanup, до того
// как закроется фейковый Telegram.
func runBot(t *testing.T, pool *pgxpool.Pool, fake *fakeTelegram) *telegram.Bot {
	t.Helper()

	bot := telegram.New(pool, tgConfig(t, fake))

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
			t.Error("Run не вернулся за 5 секунд после отмены ctx (ФТ-21)")
		}
	})

	return bot
}

// boundChat — номер чата роли из telegram_chats; ok == false, если строки нет.
func boundChat(t *testing.T, pool *pgxpool.Pool, role string) (int64, bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int64
	err := pool.QueryRow(ctx, `SELECT chat_id FROM telegram_chats WHERE role = $1`, role).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		t.Fatalf("не удалось прочитать telegram_chats: %v", err)
	}
	return id, true
}

// countChats — сколько строк в telegram_chats.
func countChats(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM telegram_chats`).Scan(&n); err != nil {
		t.Fatalf("не удалось прочитать telegram_chats: %v", err)
	}
	return n
}

// waitBound ждёт, пока роль окажется привязана к чату.
func waitBound(t *testing.T, pool *pgxpool.Pool, role string, chatID int64) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		got, ok := boundChat(t, pool, role)
		if ok && got == chatID {
			return
		}
		if time.Now().After(deadline) {
			if ok {
				t.Fatalf("роль %s привязана к чату %d, а ожидался %d", role, got, chatID)
			}
			t.Fatalf("роль %s за 5 секунд так и не привязалась к чату %d", role, chatID)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// bindChat привязывает роль к чату прямо в базе — для проверок, где
// привязка командой не главное.
func bindChat(t *testing.T, role string, chatID int64) {
	t.Helper()
	execSQL(t, `
		INSERT INTO telegram_chats (role, chat_id) VALUES ($1, $2)
		ON CONFLICT (role) DO UPDATE SET chat_id = EXCLUDED.chat_id`, role, chatID)
}

// settle — бот обрабатывает обновления по порядку; чтобы убедиться, что
// на обновление он промолчал, тест шлёт за ним метку — /status владельца
// в личку — и ждёт ответа на неё.
func settle(t *testing.T, fake *fakeTelegram) {
	t.Helper()

	before := len(fake.sentTo(tgOwnerID))
	fake.pushOwnerPrivate("/status")
	fake.waitSent(tgOwnerID, before+1, "ответ на /status-метку")
	time.Sleep(200 * time.Millisecond)
}

// lines режет текст на строки без пробелов по краям.
func lines(text string) []string {
	raw := strings.Split(strings.TrimSpace(text), "\n")
	out := make([]string, len(raw))
	for i, l := range raw {
		out[i] = strings.TrimSpace(l)
	}
	return out
}

// indexOfLine — номер первой строки с этим началом, или -1.
func indexOfLine(ls []string, prefix string) int {
	for i, l := range ls {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}

// requireCalmDisk пропускает тест, если на машине, где он идёт, правда
// мало места: тогда тревога disk честная, и «всё в порядке» не проверить.
func requireCalmDisk(t *testing.T, path string) {
	t.Helper()

	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return
	}
	total := uint64(st.Blocks) * uint64(st.Bsize)
	free := uint64(st.Bavail) * uint64(st.Bsize)
	if total == 0 {
		return
	}
	if free < 2_000_000_000 || free*10 < total {
		t.Skipf("на машине теста мало места на диске (%d из %d байт), тревога disk честная", free, total)
	}
}

// hotMemorySamples кладёт измерения последних 10 минут, в каждом память
// занята больше 90%, — это тревога memory (specs/016-dashboard.md, ФТ-18).
func hotMemorySamples(t *testing.T) {
	t.Helper()
	now := time.Now()
	for i := 0; i < 6; i++ {
		s := calmSample(now.Add(-time.Duration(i)*time.Minute - 30*time.Second))
		s.memUsed = 1_950_000_000
		insertSample(t, s)
	}
}

// threeErrors кладёт три ошибки сервера за последний час — тревога errors.
func threeErrors(t *testing.T) {
	t.Helper()
	now := time.Now()
	for i := 0; i < 3; i++ {
		insertServerError(t, now.Add(-time.Duration(5+10*i)*time.Minute), "POST", "/api/posts", 500, "сбой")
	}
}

// telegramPhone — номер n-го участника тестов бота; не пересекается
// с номерами других тестов пакета.
func telegramPhone(n int) string {
	return fmt.Sprintf("+7 (900) 818-00-%02d", n)
}

// --- Привязка чатов -------------------------------------------------------

// /start владельца в личке привязывает личку и отвечает текстом из
// спецификации (ФТ-4, ФТ-5).
func TestTelegramStartBindsOwnerChat(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	fake.pushOwnerPrivate("/start")

	got := fake.waitSent(tgOwnerID, 1, "ответ на /start")
	if got[0].method != "sendMessage" || strings.TrimSpace(got[0].text) != tgOwnerBound {
		t.Fatalf("на /start владельца ожидался ответ %q, получено: %s", tgOwnerBound, describeSent(got))
	}
	waitBound(t, pool, "owner", tgOwnerID)
	if n := countChats(t, pool); n != 1 {
		t.Fatalf("после /start в telegram_chats ожидалась одна строка, их %d", n)
	}
}

// Ник владельца сверяется без учёта регистра (ФТ-2).
func TestTelegramOwnerNickIgnoresCase(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	fake.push(tgOwnerID, strings.ToLower(tgOwnerNick), tgOwnerID, "private", "/start")

	got := fake.waitSent(tgOwnerID, 1, "ответ на /start")
	if strings.TrimSpace(got[0].text) != tgOwnerBound {
		t.Fatalf("ник %q — это владелец %q в другом регистре; ожидался ответ %q, получено: %s",
			strings.ToLower(tgOwnerNick), tgOwnerNick, tgOwnerBound, describeSent(got))
	}
	waitBound(t, pool, "owner", tgOwnerID)
}

// Команда в личке от чужого получает приветствие из specs/019-feedback.md, ФТ-23
// (раньше — «Это служебный бот МоейДачи.», 018 ФТ-7), и ничего не привязывает.
func TestTelegramStrangerInPrivateGetsServiceReply(t *testing.T) {
	cases := []struct {
		name     string
		username string
		text     string
	}{
		{name: "start от другого ника", username: "tester_vasya", text: "/start"},
		{name: "start без ника", username: "", text: "/start"},
		{name: "status от другого ника", username: "tester_vasya", text: "/status"},
		{name: "ник, похожий на владельца", username: tgOwnerNick + "_fan", text: "/start"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pool := tgPool(t)
			fake := newFakeTelegram(t)
			runBot(t, pool, fake)

			fake.push(tgStrangerID, c.username, tgStrangerID, "private", c.text)

			got := fake.waitSent(tgStrangerID, 1, "ответ чужому")
			if strings.TrimSpace(got[0].text) != tgStrangerText {
				t.Fatalf("чужому на %q ожидался ответ %q, получено: %s", c.text, tgStrangerText, describeSent(got))
			}

			settle(t, fake)
			if n := countChats(t, pool); n != 0 {
				t.Fatalf("команда чужого ничего не должна привязывать, а в telegram_chats %d строк", n)
			}
			if extra := fake.sentTo(tgStrangerID); len(extra) != 1 {
				t.Fatalf("чужому ожидался ровно один ответ, получено: %s", describeSent(extra))
			}
		})
	}
}

// /group и /группа владельца в группе или супергруппе привязывают группу
// (ФТ-6); часть после @ и регистр не важны (ФТ-8).
func TestTelegramGroupCommandBindsGroup(t *testing.T) {
	cases := []struct {
		chatType string
		text     string
	}{
		{chatType: "group", text: "/group"},
		{chatType: "supergroup", text: "/group"},
		{chatType: "group", text: "/группа"},
		{chatType: "supergroup", text: "/группа"},
		{chatType: "supergroup", text: "/group@moya_dacha_bot"},
		{chatType: "group", text: "/GROUP"},
	}

	for _, c := range cases {
		t.Run(c.chatType+" "+c.text, func(t *testing.T) {
			pool := tgPool(t)
			fake := newFakeTelegram(t)
			runBot(t, pool, fake)

			fake.push(tgOwnerID, tgOwnerNick, tgGroupID, c.chatType, c.text)

			got := fake.waitSent(tgGroupID, 1, "ответ на "+c.text)
			if strings.TrimSpace(got[0].text) != tgGroupBound {
				t.Fatalf("на %s владельца в %s ожидался ответ %q, получено: %s", c.text, c.chatType, tgGroupBound, describeSent(got))
			}
			waitBound(t, pool, "group", tgGroupID)
			if _, ok := boundChat(t, pool, "owner"); ok {
				t.Fatal("/group не должна привязывать личку владельца")
			}
		})
	}
}

// Команды не от владельца в группе бот молча пропускает: без ответа
// и без привязки (ФТ-7).
func TestTelegramStrangerInGroupIsIgnored(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	fake.push(tgStrangerID, "tester_vasya", tgGroupID, "supergroup", "/group")
	fake.push(tgStrangerID, "tester_vasya", tgGroupID, "supergroup", "/status")
	fake.push(tgStrangerID, "tester_vasya", tgGroupID, "supergroup", "/start")
	settle(t, fake)

	if got := fake.sentTo(tgGroupID); len(got) != 0 {
		t.Fatalf("на команды тестировщика в группе бот должен молчать, а отправил: %s", describeSent(got))
	}
	if got := fake.sentTo(tgStrangerID); len(got) != 0 {
		t.Fatalf("на команды тестировщика в группе бот не должен писать ему в личку, а отправил: %s", describeSent(got))
	}
	if _, ok := boundChat(t, pool, "group"); ok {
		t.Fatal("/group тестировщика не должна привязывать группу")
	}
}

// Новая привязка роли заменяет старую: строка одна на роль (ФТ-4).
func TestTelegramRebindReplacesChat(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	fake.push(tgOwnerID, tgOwnerNick, tgGroupID, "group", "/group")
	fake.waitSent(tgGroupID, 1, "ответ на первую /group")
	waitBound(t, pool, "group", tgGroupID)

	fake.push(tgOwnerID, tgOwnerNick, tgOtherGroup, "supergroup", "/group")
	fake.waitSent(tgOtherGroup, 1, "ответ на вторую /group")
	waitBound(t, pool, "group", tgOtherGroup)

	fake.pushOwnerPrivate("/start")
	fake.waitSent(tgOwnerID, 1, "ответ на /start")
	waitBound(t, pool, "owner", tgOwnerID)

	// Та же личка ещё раз — строка по-прежнему одна.
	fake.pushOwnerPrivate("/start")
	fake.waitSent(tgOwnerID, 2, "ответ на повторный /start")

	if n := countChats(t, pool); n != 2 {
		t.Fatalf("в telegram_chats по строке на роль — ожидалось 2, получено %d", n)
	}
	if id, _ := boundChat(t, pool, "group"); id != tgOtherGroup {
		t.Fatalf("группа должна смотреть в новый чат %d, а смотрит в %d", tgOtherGroup, id)
	}
}

// requireStatusReply — ответ в чат — сводка /status (ФТ-9), а не
// приветствие чужому.
func requireStatusReply(t *testing.T, s tgSent, what string) {
	t.Helper()
	if s.method != "sendMessage" || indexOfLine(lines(s.text), "Работает:") < 0 {
		t.Fatalf("%s: ожидалась сводка со строкой «Работает: …», получено: %s", what, describeSent([]tgSent{s}))
	}
}

// После привязки лички владелец опознаётся по from.id, равному номеру
// лички: сменив ник, он остаётся владельцем — в личке и в группе
// (ФТ-2а, ФТ-5).
func TestTelegramOwnerKnownByIDAfterStart(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	fake.pushOwnerPrivate("/start")
	fake.waitSent(tgOwnerID, 1, "ответ на /start")
	waitBound(t, pool, "owner", tgOwnerID)

	const renamed = "renamed_owner"
	fake.push(tgOwnerID, renamed, tgOwnerID, "private", "/status")
	requireStatusReply(t, fake.waitSent(tgOwnerID, 2, "/status владельца с новым ником")[1], "/status владельца с новым ником в личке")

	fake.push(tgOwnerID, "", tgOwnerID, "private", "/start")
	got := fake.waitSent(tgOwnerID, 3, "/start владельца без ника")
	if strings.TrimSpace(got[2].text) != tgOwnerBound {
		t.Fatalf("владелец без ника после привязки — всё ещё владелец; ожидался ответ %q, получено: %s", tgOwnerBound, describeSent(got[2:]))
	}

	fake.push(tgOwnerID, renamed, tgGroupID, "supergroup", "/group")
	gotGroup := fake.waitSent(tgGroupID, 1, "/group владельца с новым ником")
	if strings.TrimSpace(gotGroup[0].text) != tgGroupBound {
		t.Fatalf("на /group владельца с новым ником ожидался ответ %q, получено: %s", tgGroupBound, describeSent(gotGroup))
	}
	waitBound(t, pool, "group", tgGroupID)

	fake.push(tgOwnerID, renamed, tgGroupID, "supergroup", "/status")
	requireStatusReply(t, fake.waitSent(tgGroupID, 2, "/status в группе")[1], "/status владельца с новым ником в группе")

	if id, _ := boundChat(t, pool, "owner"); id != tgOwnerID {
		t.Fatalf("личка владельца должна остаться %d, а она %d", tgOwnerID, id)
	}
}

// Ник владельца у человека с другим id, когда личка уже привязана, —
// не владелец: /start не перепривязывает и получает ответ из ФТ-7, /status
// — тоже; в группе его /group и /status бот молча пропускает (ФТ-2а, ФТ-5).
func TestTelegramOwnerNickTakenAfterStart(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	fake.pushOwnerPrivate("/start")
	fake.waitSent(tgOwnerID, 1, "ответ на /start")
	waitBound(t, pool, "owner", tgOwnerID)

	for i, cmd := range []string{"/start", "/status", "/START"} {
		fake.push(tgImpostorID, tgOwnerNick, tgImpostorID, "private", cmd)
		got := fake.waitSent(tgImpostorID, i+1, "ответ занявшему ник на "+cmd)
		if strings.TrimSpace(got[i].text) != tgStrangerText {
			t.Fatalf("занявшему ник владельца на %s ожидался ответ %q, получено: %s", cmd, tgStrangerText, describeSent(got[i:]))
		}
	}
	// Ник в другом регистре — тоже не владелец.
	fake.push(tgImpostorID, strings.ToLower(tgOwnerNick), tgImpostorID, "private", "/start")
	fake.waitSent(tgImpostorID, 4, "ответ занявшему ник в другом регистре")

	fake.push(tgImpostorID, tgOwnerNick, tgGroupID, "supergroup", "/group")
	fake.push(tgImpostorID, tgOwnerNick, tgGroupID, "supergroup", "/status")
	settle(t, fake)

	if id, _ := boundChat(t, pool, "owner"); id != tgOwnerID {
		t.Fatalf("/start занявшего ник не должен перепривязывать личку: она %d, а была %d", id, tgOwnerID)
	}
	if _, ok := boundChat(t, pool, "group"); ok {
		t.Fatal("/group занявшего ник не должна привязывать группу")
	}
	if got := fake.sentTo(tgGroupID); len(got) != 0 {
		t.Fatalf("на команды занявшего ник в группе бот молчит, а отправил: %s", describeSent(got))
	}
	got := fake.sentTo(tgImpostorID)
	if len(got) != 4 {
		t.Fatalf("занявшему ник ожидалось ровно 4 ответа — на команды в личке, получено: %s", describeSent(got))
	}
	for _, s := range got {
		if strings.TrimSpace(s.text) != tgStrangerText {
			t.Fatalf("занявшему ник уходит только ответ из ФТ-7, а ушло: %s", describeSent([]tgSent{s}))
		}
	}
	if n := countChats(t, pool); n != 1 {
		t.Fatalf("в telegram_chats ожидалась одна строка — личка владельца, их %d", n)
	}
}

// Сообщение без «/» в начале бот пропускает, даже от владельца (ФТ-8).
func TestTelegramIgnoresNonCommands(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	runBot(t, pool, fake)

	fake.pushOwnerPrivate("привет")
	fake.pushOwnerPrivate("start")
	fake.pushOwnerPrivate("как дела /status")
	fake.push(tgStrangerID, "tester_vasya", tgStrangerID, "private", "здравствуйте")
	fake.push(tgOwnerID, tgOwnerNick, tgGroupID, "group", "group")
	settle(t, fake)

	if got := fake.sentTo(tgOwnerID); len(got) != 1 {
		t.Fatalf("владельцу ожидался один ответ — на метку /status, получено: %s", describeSent(got))
	}
	if got := fake.sentTo(tgStrangerID); len(got) != 0 {
		t.Fatalf("на сообщение без команды чужому бот должен молчать, а отправил: %s", describeSent(got))
	}
	if got := fake.sentTo(tgGroupID); len(got) != 0 {
		t.Fatalf("на сообщение без команды в группе бот должен молчать, а отправил: %s", describeSent(got))
	}
	if n := countChats(t, pool); n != 0 {
		t.Fatalf("сообщения без команд ничего не привязывают, а в telegram_chats %d строк", n)
	}
}

// --- /статус --------------------------------------------------------------

// /status и /статус владельца в личке или группе отвечают сводкой:
// «Всё в порядке», «Людей: N, постов: M», «Ошибок за сутки: K»,
// «Работает: …» (ФТ-9). Измерений машины нет — строки «Процессор» нет.
func TestTelegramStatusSummary(t *testing.T) {
	cases := []struct {
		name     string
		chatID   int64
		chatType string
		text     string
	}{
		{name: "status в личке", chatID: tgOwnerID, chatType: "private", text: "/status"},
		{name: "статус в личке", chatID: tgOwnerID, chatType: "private", text: "/статус"},
		{name: "СТАТУС в личке", chatID: tgOwnerID, chatType: "private", text: "/СТАТУС"},
		{name: "status с именем бота в группе", chatID: tgGroupID, chatType: "supergroup", text: "/status@moya_dacha_bot"},
		{name: "статус в группе", chatID: tgGroupID, chatType: "group", text: "/статус"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			baseURL := startAPI(t) // чистит базу
			pool := connect(t)

			anna, _ := signIn(t, baseURL, telegramPhone(1))
			boris, _ := signIn(t, baseURL, telegramPhone(2))
			publishPost(t, baseURL, anna, "Первые огурцы")
			publishPost(t, baseURL, anna, "Кабачки пошли")
			publishPost(t, baseURL, boris, "Теплица готова")

			// Одна ошибка за сутки (тревогой не считается) и одна старше суток.
			insertServerError(t, time.Now().Add(-2*time.Hour), "GET", "/api/feed", 500, "сбой")
			insertServerError(t, time.Now().Add(-30*time.Hour), "GET", "/api/feed", 500, "давний сбой")

			fake := newFakeTelegram(t)
			cfg := tgConfig(t, fake)
			requireCalmDisk(t, cfg.DiskPath)
			runBot(t, pool, fake)

			fake.push(tgOwnerID, tgOwnerNick, c.chatID, c.chatType, c.text)
			got := fake.waitSent(c.chatID, 1, "сводка на "+c.text)
			if got[0].method != "sendMessage" {
				t.Fatalf("сводка должна уйти текстом, получено: %s", describeSent(got))
			}

			ls := lines(got[0].text)
			if ls[0] != "Всё в порядке" {
				t.Fatalf("первая строка сводки без тревог — «Всё в порядке», получено %q\nсводка:\n%s", ls[0], got[0].text)
			}
			people := indexOfLine(ls, "Людей:")
			if people < 0 || ls[people] != "Людей: 2, постов: 3" {
				t.Fatalf("в сводке ожидалась строка «Людей: 2, постов: 3»\nсводка:\n%s", got[0].text)
			}
			errs := indexOfLine(ls, "Ошибок за сутки:")
			if errs < 0 || ls[errs] != "Ошибок за сутки: 1" {
				t.Fatalf("в сводке ожидалась строка «Ошибок за сутки: 1» (ошибка старше суток не в счёт)\nсводка:\n%s", got[0].text)
			}
			up := indexOfLine(ls, "Работает:")
			if up < 0 {
				t.Fatalf("в сводке нет строки «Работает: …»\nсводка:\n%s", got[0].text)
			}
			if !(people < errs && errs < up) {
				t.Fatalf("строки сводки не по порядку: «Людей» — %d, «Ошибок» — %d, «Работает» — %d\nсводка:\n%s", people, errs, up, got[0].text)
			}
			if indexOfLine(ls, "Процессор") >= 0 {
				t.Fatalf("измерений машины нет — строки «Процессор …» быть не должно\nсводка:\n%s", got[0].text)
			}
		})
	}
}

// При тревогах сводка начинается с «Тревоги:» и строк «• <текст>», а при
// измерениях машины в ней есть строка «Процессор …» перед «Работает» (ФТ-9).
func TestTelegramStatusWithAlertsAndMachine(t *testing.T) {
	pool := tgPool(t)
	threeErrors(t)
	insertSample(t, calmSample(time.Now().Add(-30*time.Second)))

	fake := newFakeTelegram(t)
	requireCalmDisk(t, t.TempDir())
	runBot(t, pool, fake)

	fake.pushOwnerPrivate("/status")
	got := fake.waitSent(tgOwnerID, 1, "сводка на /status")

	ls := lines(got[0].text)
	if ls[0] != "Тревоги:" {
		t.Fatalf("при тревогах первая строка сводки — «Тревоги:», получено %q\nсводка:\n%s", ls[0], got[0].text)
	}
	if len(ls) < 2 || ls[1] != "• Ошибок сервера за час: 3" {
		t.Fatalf("под «Тревоги:» ожидалась строка «• Ошибок сервера за час: 3»\nсводка:\n%s", got[0].text)
	}
	if indexOfLine(ls, "Людей: 0, постов: 0") < 0 {
		t.Fatalf("в сводке ожидалась строка «Людей: 0, постов: 0»\nсводка:\n%s", got[0].text)
	}
	if indexOfLine(ls, "Ошибок за сутки: 3") < 0 {
		t.Fatalf("в сводке ожидалась строка «Ошибок за сутки: 3»\nсводка:\n%s", got[0].text)
	}
	cpu := indexOfLine(ls, "Процессор ")
	up := indexOfLine(ls, "Работает:")
	if cpu < 0 {
		t.Fatalf("есть измерение машины — в сводке ожидалась строка «Процессор X%%, память A из B, диск C из D»\nсводка:\n%s", got[0].text)
	}
	for _, part := range []string{"%", "память ", "диск "} {
		if !strings.Contains(ls[cpu], part) {
			t.Fatalf("в строке машины нет %q: %q", part, ls[cpu])
		}
	}
	if !strings.Contains(ls[cpu], "ГБ") {
		t.Fatalf("размеры в строке машины — как на дашборде («1,4 ГБ»), получено %q", ls[cpu])
	}
	if up < 0 || cpu > up {
		t.Fatalf("строка «Процессор …» должна идти перед «Работает: …»\nсводка:\n%s", got[0].text)
	}
}

// --- Тревоги --------------------------------------------------------------

// Личка не привязана — ничего не уходит, и набор отправленным не
// считается: после привязки тревога приходит на следующей проверке (ФТ-14).
func TestTelegramAlertsWaitForOwnerChat(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	cfg := tgConfig(t, fake)
	requireCalmDisk(t, cfg.DiskPath)
	bot := telegram.New(pool, cfg)
	ctx := context.Background()

	threeErrors(t)
	bindChat(t, "group", tgGroupID) // группа тревог не получает

	if err := bot.CheckAlerts(ctx); err != nil {
		t.Fatalf("CheckAlerts без привязанной лички не должен падать: %v", err)
	}
	if got := fake.allSent(); len(got) != 0 {
		t.Fatalf("личка не привязана — ничего не должно уходить, а ушло: %s", describeSent(got))
	}

	bindChat(t, "owner", tgOwnerID)
	if err := bot.CheckAlerts(ctx); err != nil {
		t.Fatalf("CheckAlerts: %v", err)
	}
	got := fake.sentTo(tgOwnerID)
	if len(got) != 1 {
		t.Fatalf("после привязки лички тревога должна прийти на следующей проверке, получено: %s", describeSent(fake.allSent()))
	}
	requireAlertMessage(t, got[0], "• Ошибок сервера за час: 3")
	if g := fake.sentTo(tgGroupID); len(g) != 0 {
		t.Fatalf("тревоги уходят только в личку владельца, а в группу ушло: %s", describeSent(g))
	}
}

// requireAlertMessage — сообщение «Тревога на проде:» со списком ровно
// из этих строк.
func requireAlertMessage(t *testing.T, s tgSent, bullets ...string) {
	t.Helper()

	ls := lines(s.text)
	if s.method != "sendMessage" || ls[0] != tgAlertHeader {
		t.Fatalf("ожидалось сообщение, начинающееся «%s», получено: %s", tgAlertHeader, describeSent([]tgSent{s}))
	}
	var got []string
	for _, l := range ls[1:] {
		if l != "" {
			got = append(got, l)
		}
	}
	if len(got) != len(bullets) {
		t.Fatalf("в тревоге ожидались строки %q, получены %q", bullets, got)
	}
	for _, want := range bullets {
		found := false
		for _, l := range got {
			if l == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("в тревоге нет строки %q; строки: %q", want, got)
		}
	}
}

// Полный цикл тревог (ФТ-10–12): появилась — одно сообщение; тот же
// набор, даже с другим текстом, — тишина; набор изменился — снова весь
// список; тревоги прошли — «Всё в порядке: тревог больше нет.»; и снова
// тишина, пока всё спокойно.
func TestTelegramAlertsLifecycle(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	cfg := tgConfig(t, fake)
	requireCalmDisk(t, cfg.DiskPath)
	bot := telegram.New(pool, cfg)
	ctx := context.Background()
	bindChat(t, "owner", tgOwnerID)

	check := func(step string, want int) []tgSent {
		t.Helper()
		if err := bot.CheckAlerts(ctx); err != nil {
			t.Fatalf("%s: CheckAlerts: %v", step, err)
		}
		got := fake.sentTo(tgOwnerID)
		if len(got) != want {
			t.Fatalf("%s: владельцу ожидалось %d сообщений всего, получено: %s", step, want, describeSent(got))
		}
		return got
	}

	check("тревог нет", 0)

	threeErrors(t)
	got := check("появились ошибки", 1)
	requireAlertMessage(t, got[0], "• Ошибок сервера за час: 3")

	check("та же тревога", 1)

	// Текст тревоги изменился, а набор видов — нет: пуш не нужен (ФТ-12).
	insertServerError(t, time.Now().Add(-time.Minute), "GET", "/api/feed", 502, "ещё сбой")
	check("ошибок стало четыре", 1)

	// Добавилась тревога memory — уходит весь список, не только новая (ФТ-10).
	hotMemorySamples(t)
	got = check("добавилась память", 2)
	requireAlertMessage(t, got[1], "• Ошибок сервера за час: 4", "• Память занята больше 90% уже 10 минут")

	// Ошибки ушли, память осталась — набор изменился и не пуст.
	execSQL(t, `DELETE FROM server_errors`)
	got = check("остались только память", 3)
	requireAlertMessage(t, got[2], "• Память занята больше 90% уже 10 минут")

	execSQL(t, `DELETE FROM server_samples`)
	got = check("тревоги прошли", 4)
	if got[3].method != "sendMessage" || strings.TrimSpace(got[3].text) != tgAllClear {
		t.Fatalf("когда тревоги прошли, ожидалось %q, получено: %s", tgAllClear, describeSent(got[3:]))
	}

	check("всё спокойно дальше", 4)
}

// Если тревог не было с самого начала — ничего не уходит, никакого
// «всё в порядке» (ФТ-11).
func TestTelegramNoAlertsNoMessages(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	cfg := tgConfig(t, fake)
	requireCalmDisk(t, cfg.DiskPath)
	bot := telegram.New(pool, cfg)
	bindChat(t, "owner", tgOwnerID)
	insertSample(t, calmSample(time.Now().Add(-time.Minute)))
	insertServerError(t, time.Now().Add(-time.Minute), "GET", "/api/feed", 500, "один сбой")

	for i := 0; i < 3; i++ {
		if err := bot.CheckAlerts(context.Background()); err != nil {
			t.Fatalf("CheckAlerts: %v", err)
		}
	}
	if got := fake.allSent(); len(got) != 0 {
		t.Fatalf("тревог не было — ничего не должно уходить, а ушло: %s", describeSent(got))
	}
}

// Бот помнит отправленный набор только в памяти: после перезапуска
// текущие тревоги приходят ещё раз (ФТ-13).
func TestTelegramAlertsRepeatAfterRestart(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	cfg := tgConfig(t, fake)
	requireCalmDisk(t, cfg.DiskPath)
	ctx := context.Background()
	bindChat(t, "owner", tgOwnerID)
	threeErrors(t)

	first := telegram.New(pool, cfg)
	if err := first.CheckAlerts(ctx); err != nil {
		t.Fatalf("CheckAlerts: %v", err)
	}
	if err := first.CheckAlerts(ctx); err != nil {
		t.Fatalf("CheckAlerts: %v", err)
	}
	if got := fake.sentTo(tgOwnerID); len(got) != 1 {
		t.Fatalf("до перезапуска ожидалась одна тревога, получено: %s", describeSent(got))
	}

	restarted := telegram.New(pool, cfg)
	if err := restarted.CheckAlerts(ctx); err != nil {
		t.Fatalf("CheckAlerts после перезапуска: %v", err)
	}
	got := fake.sentTo(tgOwnerID)
	if len(got) != 2 {
		t.Fatalf("после перезапуска текущая тревога должна прийти ещё раз, получено: %s", describeSent(got))
	}
	requireAlertMessage(t, got[1], "• Ошибок сервера за час: 3")
}

// --- Сборки ---------------------------------------------------------------

// Чат роли не привязан — SendBuild возвращает telegram.ErrNotBound
// и ничего не отправляет (ФТ-18, ФТ-21).
func TestTelegramSendBuildNotBound(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	bot := telegram.New(pool, tgConfig(t, fake))
	ctx := context.Background()

	for _, role := range []string{"group", "owner"} {
		err := bot.SendBuild(ctx, role, []byte(tgBuildInfo), "", "https://example.com/app.apk")
		if !errors.Is(err, telegram.ErrNotBound) {
			t.Fatalf("SendBuild в непривязанный чат %s: ожидалась telegram.ErrNotBound, получено %v", role, err)
		}
	}

	// Привязанная личка не делает привязанной группу.
	bindChat(t, "owner", tgOwnerID)
	err := bot.SendBuild(ctx, "group", []byte(tgBuildInfo), "", "")
	if !errors.Is(err, telegram.ErrNotBound) {
		t.Fatalf("SendBuild в группу при привязанной только личке: ожидалась telegram.ErrNotBound, получено %v", err)
	}

	if got := fake.allSent(); len(got) != 0 {
		t.Fatalf("в непривязанный чат ничего не должно уходить, а ушло: %s", describeSent(got))
	}
}

// requireBuildCaption — подпись сборки из ФТ-17.
func requireBuildCaption(t *testing.T, text string) []string {
	t.Helper()

	ls := lines(text)
	want := []string{"МояДача 1.0.0, сборка 386900", "Что нового:", "• Лента друзей", "• Починили лайки"}
	if len(ls) < len(want) {
		t.Fatalf("подпись сборки короче ожидаемой:\n%s", text)
	}
	for i, w := range want {
		if ls[i] != w {
			t.Fatalf("строка %d подписи: ожидалось %q, получено %q\nподпись:\n%s", i+1, w, ls[i], text)
		}
	}
	return ls
}

// APK уходит в группу файлом moya-dacha-<сборка>.apk с подписью (ФТ-16, ФТ-17).
func TestTelegramSendBuildFileToGroup(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	bot := telegram.New(pool, tgConfig(t, fake))
	bindChat(t, "owner", tgOwnerID)
	bindChat(t, "group", tgGroupID)

	apk := filepath.Join(t.TempDir(), "app-release.apk")
	content := []byte("PK\x03\x04 будто бы APK")
	if err := os.WriteFile(apk, content, 0o644); err != nil {
		t.Fatalf("не удалось записать APK: %v", err)
	}

	if err := bot.SendBuild(context.Background(), "group", []byte(tgBuildInfo), apk, ""); err != nil {
		t.Fatalf("SendBuild: %v", err)
	}

	got := fake.sentTo(tgGroupID)
	if len(got) != 1 || got[0].method != "sendDocument" {
		t.Fatalf("в группу ожидался один sendDocument, получено: %s", describeSent(fake.allSent()))
	}
	if got[0].filename != "moya-dacha-386900.apk" {
		t.Fatalf("файл должен называться moya-dacha-386900.apk, а называется %q", got[0].filename)
	}
	if string(got[0].content) != string(content) {
		t.Fatalf("в Telegram ушло не то содержимое APK: %d байт вместо %d", len(got[0].content), len(content))
	}
	ls := requireBuildCaption(t, got[0].text)
	if indexOfLine(ls, "Скачать:") >= 0 {
		t.Fatalf("без ссылки строки «Скачать:» быть не должно:\n%s", got[0].text)
	}
	if o := fake.sentTo(tgOwnerID); len(o) != 0 {
		t.Fatalf("сборка для группы не должна уходить владельцу, а ушло: %s", describeSent(o))
	}
}

// Со ссылкой сборка уходит текстом, «Скачать: <ссылка>» — ниже через
// пустую строку (ФТ-16, ФТ-17).
func TestTelegramSendBuildLinkToOwner(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	bot := telegram.New(pool, tgConfig(t, fake))
	bindChat(t, "owner", tgOwnerID)
	bindChat(t, "group", tgGroupID)

	const link = "https://github.com/gmu-msk/moya_dacha/releases/download/pr-42/app-debug.apk"
	if err := bot.SendBuild(context.Background(), "owner", []byte(tgBuildInfo), "", link); err != nil {
		t.Fatalf("SendBuild: %v", err)
	}

	got := fake.sentTo(tgOwnerID)
	if len(got) != 1 || got[0].method != "sendMessage" {
		t.Fatalf("владельцу ожидался один sendMessage, получено: %s", describeSent(fake.allSent()))
	}
	requireBuildCaption(t, got[0].text)
	if !strings.HasSuffix(strings.TrimSpace(got[0].text), "\n\nСкачать: "+link) {
		t.Fatalf("текст должен кончаться строкой «Скачать: %s» после пустой строки:\n%s", link, got[0].text)
	}
	if g := fake.sentTo(tgGroupID); len(g) != 0 {
		t.Fatalf("сборка для владельца не должна уходить в группу, а ушло: %s", describeSent(g))
	}
}

// Без файла и ссылки — только текст подписи (ФТ-16).
func TestTelegramSendBuildTextOnly(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	bot := telegram.New(pool, tgConfig(t, fake))
	bindChat(t, "group", tgGroupID)

	if err := bot.SendBuild(context.Background(), "group", []byte(tgBuildInfo), "", ""); err != nil {
		t.Fatalf("SendBuild: %v", err)
	}

	got := fake.sentTo(tgGroupID)
	if len(got) != 1 || got[0].method != "sendMessage" {
		t.Fatalf("в группу ожидался один sendMessage, получено: %s", describeSent(fake.allSent()))
	}
	ls := requireBuildCaption(t, got[0].text)
	if indexOfLine(ls, "Скачать:") >= 0 {
		t.Fatalf("без ссылки строки «Скачать:» быть не должно:\n%s", got[0].text)
	}
}

// Telegram ответил {"ok": false} — SendBuild возвращает ошибку, и это
// не ErrNotBound (ФТ-18, ФТ-22).
func TestTelegramSendBuildTelegramError(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	bot := telegram.New(pool, tgConfig(t, fake))
	bindChat(t, "owner", tgOwnerID)
	bindChat(t, "group", tgGroupID)
	fake.setFailSend(true)

	apk := filepath.Join(t.TempDir(), "app-release.apk")
	if err := os.WriteFile(apk, []byte("PK\x03\x04"), 0o644); err != nil {
		t.Fatalf("не удалось записать APK: %v", err)
	}

	cases := []struct {
		name string
		role string
		apk  string
		link string
	}{
		{name: "файлом", role: "group", apk: apk},
		{name: "ссылкой", role: "owner", link: "https://example.com/app.apk"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := bot.SendBuild(context.Background(), c.role, []byte(tgBuildInfo), c.apk, c.link)
			if err == nil {
				t.Fatal(`Telegram ответил {"ok": false}, а SendBuild вернул nil`)
			}
			if errors.Is(err, telegram.ErrNotBound) {
				t.Fatalf("чат привязан — ошибка Telegram не должна быть ErrNotBound: %v", err)
			}
		})
	}
}

// tgFileLimit — больше этого Telegram не принимает файл от бота (ФТ-16а).
const tgFileLimit = 52428800

// sparseAPK создаёт APK нужного размера, не занимая места на диске:
// файл разреженный, внутри нули.
func sparseAPK(t *testing.T, size int64) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app-release.apk")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("не удалось создать APK: %v", err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Fatalf("не удалось растянуть APK до %d байт: %v", size, err)
	}
	return path
}

// APK больше 50 МБ и есть ссылка — сборка уходит текстом со ссылкой,
// файл не отправляется (ФТ-16а, ФТ-17).
func TestTelegramSendBuildTooBigFileGoesAsLink(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	bot := telegram.New(pool, tgConfig(t, fake))
	bindChat(t, "group", tgGroupID)

	apk := sparseAPK(t, tgFileLimit+1)
	const link = "https://moya-dacha.example/app.apk"
	if err := bot.SendBuild(context.Background(), "group", []byte(tgBuildInfo), apk, link); err != nil {
		t.Fatalf("SendBuild с APK больше лимита и ссылкой: %v", err)
	}

	all := fake.allSent()
	got := fake.sentTo(tgGroupID)
	if len(all) != 1 || len(got) != 1 || got[0].method != "sendMessage" {
		t.Fatalf("APK больше 50 МБ со ссылкой: в группу ожидался один sendMessage и никакого sendDocument, получено: %s",
			describeSent(all))
	}
	requireBuildCaption(t, got[0].text)
	if !strings.HasSuffix(strings.TrimSpace(got[0].text), "\n\nСкачать: "+link) {
		t.Fatalf("текст должен кончаться строкой «Скачать: %s» после пустой строки:\n%s", link, got[0].text)
	}
}

// APK больше 50 МБ, а ссылки нет — ошибка, в чат ничего не уходит (ФТ-16а).
func TestTelegramSendBuildTooBigFileWithoutLinkFails(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	bot := telegram.New(pool, tgConfig(t, fake))
	bindChat(t, "group", tgGroupID)

	apk := sparseAPK(t, tgFileLimit+1)
	err := bot.SendBuild(context.Background(), "group", []byte(tgBuildInfo), apk, "")
	if err == nil {
		t.Fatal("APK больше 50 МБ без ссылки: SendBuild должен вернуть ошибку, а вернул nil")
	}
	if errors.Is(err, telegram.ErrNotBound) {
		t.Fatalf("чат привязан — ошибка из-за размера APK не должна быть ErrNotBound: %v", err)
	}
	if got := fake.allSent(); len(got) != 0 {
		t.Fatalf("APK больше 50 МБ без ссылки: в чат ничего не должно уходить, а ушло: %s", describeSent(got))
	}
}

// APK в лимите уходит файлом, даже если ссылка задана, и строки
// «Скачать:» в подписи нет (ФТ-16а). Ровно 52 428 800 байт — ещё в лимите.
func TestTelegramSendBuildFileWithinLimitIgnoresLink(t *testing.T) {
	small := filepath.Join(t.TempDir(), "small.apk")
	if err := os.WriteFile(small, []byte("PK\x03\x04 будто бы APK"), 0o644); err != nil {
		t.Fatalf("не удалось записать APK: %v", err)
	}

	cases := []struct {
		name string
		apk  func(t *testing.T) string
		size int
	}{
		{name: "маленький", apk: func(*testing.T) string { return small }, size: len("PK\x03\x04 будто бы APK")},
		{name: "ровно 50 МБ", apk: func(t *testing.T) string { return sparseAPK(t, tgFileLimit) }, size: tgFileLimit},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pool := tgPool(t)
			fake := newFakeTelegram(t)
			bot := telegram.New(pool, tgConfig(t, fake))
			bindChat(t, "group", tgGroupID)

			const link = "https://moya-dacha.example/app.apk"
			if err := bot.SendBuild(context.Background(), "group", []byte(tgBuildInfo), c.apk(t), link); err != nil {
				t.Fatalf("SendBuild: %v", err)
			}

			all := fake.allSent()
			got := fake.sentTo(tgGroupID)
			if len(all) != 1 || len(got) != 1 || got[0].method != "sendDocument" {
				t.Fatalf("APK в лимите со ссылкой: в группу ожидался один sendDocument, получено: %s", describeSent(all))
			}
			if got[0].filename != "moya-dacha-386900.apk" {
				t.Fatalf("файл должен называться moya-dacha-386900.apk, а называется %q", got[0].filename)
			}
			if len(got[0].content) != c.size {
				t.Fatalf("в Telegram ушло %d байт APK вместо %d", len(got[0].content), c.size)
			}
			ls := requireBuildCaption(t, got[0].text)
			if indexOfLine(ls, "Скачать:") >= 0 {
				t.Fatalf("APK ушёл файлом — строки «Скачать:» в подписи быть не должно:\n%s", got[0].text)
			}
		})
	}
}

// Подпись длиннее 1024 символов: файл уходит с первой строкой подписи,
// полный текст — следом отдельным сообщением (ФТ-16б).
func TestTelegramSendBuildLongCaptionFollowsFile(t *testing.T) {
	pool := tgPool(t)
	fake := newFakeTelegram(t)
	bot := telegram.New(pool, tgConfig(t, fake))
	bindChat(t, "group", tgGroupID)

	// Первые две строки — как в tgBuildInfo, чтобы подошёл requireBuildCaption.
	whatsNew := []string{"Лента друзей", "Починили лайки"}
	for i := 1; i <= 20; i++ {
		whatsNew = append(whatsNew, fmt.Sprintf(
			"Пункт %d: длинное описание изменения, чтобы подпись к файлу вышла за предел Telegram", i))
	}
	info, err := json.Marshal(map[string]any{
		"version":  "1.0.0",
		"build":    386900,
		"date":     "2026-09-26T12:00:00Z",
		"commit":   "aa652ef",
		"whatsNew": whatsNew,
	})
	if err != nil {
		t.Fatalf("не удалось собрать сведения о сборке: %v", err)
	}

	apk := filepath.Join(t.TempDir(), "app-release.apk")
	content := []byte("PK\x03\x04 будто бы APK")
	if err := os.WriteFile(apk, content, 0o644); err != nil {
		t.Fatalf("не удалось записать APK: %v", err)
	}

	if err := bot.SendBuild(context.Background(), "group", info, apk, ""); err != nil {
		t.Fatalf("SendBuild с длинной подписью: %v", err)
	}

	all := fake.allSent()
	got := fake.sentTo(tgGroupID)
	if len(all) != 2 || len(got) != 2 || got[0].method != "sendDocument" || got[1].method != "sendMessage" {
		t.Fatalf("длинная подпись: в группу ожидались sendDocument, затем sendMessage, получено: %s", describeSent(all))
	}

	doc := got[0]
	if doc.filename != "moya-dacha-386900.apk" || string(doc.content) != string(content) {
		t.Fatalf("ушёл не тот файл: %q, %d байт", doc.filename, len(doc.content))
	}
	if strings.TrimSpace(doc.text) != "МояДача 1.0.0, сборка 386900" {
		t.Fatalf("подпись к файлу должна быть первой строкой «МояДача 1.0.0, сборка 386900», а она:\n%s", doc.text)
	}

	full := got[1].text
	if n := len([]rune(full)); n <= 1024 {
		t.Fatalf("тест неисправен: полная подпись должна быть длиннее 1024 символов, а в ней %d", n)
	}
	ls := requireBuildCaption(t, full)
	for _, w := range whatsNew {
		if indexOfLine(ls, "• "+w) < 0 {
			t.Fatalf("в полном тексте подписи нет строки «• %s»:\n%s", w, full)
		}
	}
	if indexOfLine(ls, "Скачать:") >= 0 {
		t.Fatalf("без ссылки строки «Скачать:» быть не должно:\n%s", full)
	}
}
