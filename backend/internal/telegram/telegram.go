// Package telegram — служебный бот сервиса (specs/018-telegram-bot.md):
// тревоги и тестовые сборки владельцу в личку, сборки main — в группу,
// сводка по /status. Отдельного процесса нет, бот живёт в сервисе
// (ADR-0022).
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/feedback"
)

// DefaultAPIURL — адрес Bot API.
const DefaultAPIURL = "https://api.telegram.org"

// Роли чатов (требование 4). ideas — тема для отзывов
// (specs/019-feedback.md, требование 20).
const (
	RoleOwner = "owner"
	RoleGroup = "group"
	RoleIdeas = "ideas"
)

// ErrNotBound — чат роли ещё не привязан.
var ErrNotBound = errors.New("чат не привязан")

type Config struct {
	// Token — токен бота от @BotFather (TELEGRAM_BOT_TOKEN).
	Token string
	// APIURL — адрес Bot API. Пусто — DefaultAPIURL; тесты подставляют свой.
	APIURL string
	// Owner — ник владельца в Telegram без @ (TELEGRAM_OWNER).
	Owner string
	// Proxy — прокси до Bot API, например socks5://127.0.0.1:1080
	// (TELEGRAM_PROXY). Пусто — напрямую.
	Proxy string
	// DiskPath — где мерить диск для тревог и сводки.
	DiskPath string
	// PollTimeout — сколько Telegram держит getUpdates без обновлений.
	PollTimeout time.Duration
	// Feedback — отзывы и задачи GitHub (specs/019-feedback.md,
	// требование 34). nil — отзывы из Telegram не принимаются.
	Feedback *feedback.Service
}

type Bot struct {
	db   *pgxpool.Pool
	cfg  Config
	http *http.Client

	// sentAlerts — набор видов тревог, о котором владелец уже знает
	// (требование 13). Только в памяти.
	mu         sync.Mutex
	sentAlerts string
}

func New(db *pgxpool.Pool, cfg Config) *Bot {
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}
	if cfg.PollTimeout == 0 {
		cfg.PollTimeout = 50 * time.Second
	}
	if cfg.DiskPath == "" {
		cfg.DiskPath = "/"
	}
	cfg.APIURL = strings.TrimRight(cfg.APIURL, "/")
	cfg.Owner = strings.TrimPrefix(cfg.Owner, "@")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.Proxy != "" {
		if u, err := url.Parse(cfg.Proxy); err != nil {
			logError("TELEGRAM_PROXY не разобран, иду напрямую", err)
		} else {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &Bot{
		db:   db,
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.PollTimeout + 30*time.Second, Transport: transport},
	}
}

// chat — номер чата роли или ErrNotBound.
func (b *Bot) chat(ctx context.Context, role string) (int64, error) {
	var id int64
	err := b.db.QueryRow(ctx, `SELECT chat_id FROM telegram_chats WHERE role = $1`, role).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("%s: %w", role, ErrNotBound)
	}
	return id, err
}

// bind привязывает роль к чату; thread — тема группы или nil.
func (b *Bot) bind(ctx context.Context, role string, chatID int64, thread *int64) error {
	_, err := b.db.Exec(ctx, `
		INSERT INTO telegram_chats (role, chat_id, thread_id) VALUES ($1, $2, $3)
		ON CONFLICT (role) DO UPDATE
		SET chat_id = EXCLUDED.chat_id, thread_id = EXCLUDED.thread_id, bound_at = now()`,
		role, chatID, thread)
	return err
}

// ideas — чат и тема для отзывов; ok == false, если не привязаны.
func (b *Bot) ideas(ctx context.Context) (chat int64, thread *int64, ok bool, err error) {
	err = b.db.QueryRow(ctx, `SELECT chat_id, thread_id FROM telegram_chats WHERE role = $1`, RoleIdeas).
		Scan(&chat, &thread)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, false, nil
	}
	return chat, thread, err == nil, err
}

// apiResponse — общий вид ответа Bot API.
type apiResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

func (b *Bot) call(ctx context.Context, method, contentType string, body io.Reader, result any) error {
	url := b.cfg.APIURL + "/bot" + b.cfg.Token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := b.http.Do(req)
	if err != nil {
		// В ошибке net/http есть адрес, а в адресе — токен.
		return fmt.Errorf("telegram %s: %w", method, redact(err, b.cfg.Token))
	}
	defer resp.Body.Close()

	var r apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("telegram %s: HTTP %d: %w", method, resp.StatusCode, err)
	}
	if !r.OK {
		return fmt.Errorf("telegram %s: %s", method, r.Description)
	}
	if result != nil {
		return json.Unmarshal(r.Result, result)
	}
	return nil
}

func redact(err error, token string) error {
	if token == "" {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), token, "***"))
}

func (b *Bot) callJSON(ctx context.Context, method string, payload, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return b.call(ctx, method, "application/json", bytes.NewReader(body), result)
}

// Send пишет текст в чат.
func (b *Bot) Send(ctx context.Context, chatID int64, text string) error {
	return b.callJSON(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, nil)
}

// Notify отвечает автору отзыва: в его теме и на его сообщение
// (specs/019-feedback.md, требование 31).
func (b *Bot) Notify(ctx context.Context, to feedback.Recipient, text string) error {
	payload := map[string]any{"chat_id": to.ChatID, "text": text}
	if to.ThreadID != 0 {
		payload["message_thread_id"] = to.ThreadID
	}
	if to.ReplyTo != 0 {
		// Сообщение автора могли удалить — ответ всё равно нужен.
		payload["reply_parameters"] = map[string]any{
			"message_id":                  to.ReplyTo,
			"allow_sending_without_reply": true,
		}
	}
	return b.callJSON(ctx, "sendMessage", payload, nil)
}

// download скачивает файл Telegram по его file_id (getFile).
func (b *Bot) download(ctx context.Context, fileID string) ([]byte, error) {
	var f struct {
		FilePath string `json:"file_path"`
	}
	if err := b.callJSON(ctx, "getFile", map[string]any{"file_id": fileID}, &f); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		b.cfg.APIURL+"/file/bot"+b.cfg.Token+"/"+f.FilePath, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram файл: %w", redact(err, b.cfg.Token))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram файл: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 20<<20))
}

// sendDocument отправляет файл с подписью под именем name.
func (b *Bot) sendDocument(ctx context.Context, chatID int64, path, name, caption string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Файл идёт потоком: APK в десятки мегабайт держать в памяти незачем.
	pr, pw := io.Pipe()
	form := multipart.NewWriter(pw)
	go func() {
		err := func() error {
			if err := form.WriteField("chat_id", fmt.Sprint(chatID)); err != nil {
				return err
			}
			if err := form.WriteField("caption", caption); err != nil {
				return err
			}
			part, err := form.CreateFormFile("document", name)
			if err != nil {
				return err
			}
			if _, err := io.Copy(part, f); err != nil {
				return err
			}
			return form.Close()
		}()
		pw.CloseWithError(err)
	}()
	return b.call(ctx, "sendDocument", form.FormDataContentType(), pr, nil)
}

// logError — неудача бота видна в логе сервиса, но сервис из-за неё не
// падает (specs/018-telegram-bot.md, «Ограничения»).
func logError(what string, err error) {
	slog.Error("telegram: "+what, "err", err)
}

// Me — ник бота по токену (getMe): проверка, что токен рабочий и
// Telegram с этой машины доступен.
func (b *Bot) Me(ctx context.Context) (string, error) {
	var me struct {
		Username string `json:"username"`
	}
	if err := b.callJSON(ctx, "getMe", map[string]any{}, &me); err != nil {
		return "", err
	}
	return me.Username, nil
}

// Bound — привязан ли чат роли.
func (b *Bot) Bound(ctx context.Context, role string) (bool, error) {
	_, err := b.chat(ctx, role)
	if errors.Is(err, ErrNotBound) {
		return false, nil
	}
	return err == nil, err
}
