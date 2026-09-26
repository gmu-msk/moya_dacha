// Package feedback — отзывы разработчику из приложения и Telegram
// (specs/019-feedback.md). Отзыв сначала ложится в базу, потом становится
// задачей GitHub «входящее»; автор из Telegram получает номер задачи,
// а потом — что её одобрили и что она вышла в сборке (ADR-0023).
package feedback

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/media"
)

// Откуда пришёл отзыв (требование 1).
const (
	SourceApp      = "app"
	SourceTelegram = "telegram"
)

// Статусы отзыва (требование 2).
const (
	StatusSent     = "sent"
	StatusAccepted = "accepted"
	StatusApproved = "approved"
	StatusDeclined = "declined"
	StatusDone     = "done"
	StatusReleased = "released"
)

const (
	DefaultRepo   = "gmu-msk/moya_dacha"
	DefaultAPIURL = "https://api.github.com"

	// ScreenshotDir — папка скриншотов в хранилище.
	ScreenshotDir = "feedback"

	// SyncEvery — как часто повторяются задачи и сверяются статусы
	// (требования 7–8).
	SyncEvery = 5 * time.Minute
)

// Метки задач (требование 6) и метки, означающие «одобрено» (требование 8).
const (
	labelInbox    = "входящее"
	labelApproved = "одобрено"
	labelApp      = "из приложения"
	labelTelegram = "из телеграма"
)

var approvedLabels = []string{labelApproved, "в работе", "на проверке"}

// Тексты автору (specs/019-feedback.md, «Тексты»).
const (
	TextThanks = "Спасибо! Передал разработчику."
	textTicket = "Спасибо! Записал как задачу #%d. Напишу, когда её одобрят и когда она выйдет в сборке."
	textOK     = "Задачу #%d одобрили: её возьмут в работу."
	textOut    = "Задача #%d вышла в сборке %d — обновите приложение."
)

type Config struct {
	// Token — токен GitHub с правом на задачи (FEEDBACK_GITHUB_TOKEN).
	// Пусто — задачи не заводятся, отзывы копятся в базе.
	Token string
	// Repo — «владелец/имя». Пусто — DefaultRepo.
	Repo string
	// APIURL — адрес REST API GitHub. Пусто — DefaultAPIURL.
	APIURL string
	// PublicURL — адрес сервиса снаружи (PUBLIC_URL): ссылки на скриншоты
	// в задаче должны открываться из GitHub.
	PublicURL string
	// Media — хранилище скриншотов. Пусто — временная папка.
	Media media.Storage
}

// Recipient — куда ответить автору в Telegram.
type Recipient struct {
	ChatID   int64
	ThreadID int64 // тема группы; 0 — без темы
	ReplyTo  int64 // сообщение автора; 0 — не отвечать на сообщение
}

// Notifier доставляет ответ автору. Его реализует Telegram-бот.
type Notifier interface {
	Notify(ctx context.Context, to Recipient, text string) error
}

// Item — новый отзыв.
type Item struct {
	Source string
	// UserID — автор из приложения; пусто у Telegram.
	UserID string
	Author string
	Text   string
	// Screenshot — картинка как пришла; её уменьшат и сохранят.
	Screenshot []byte
	AppVersion string
	Device     string
	// Telegram — куда отвечать; Private — это личка, а не группа.
	Telegram *Recipient
	Private  bool
}

// Entry — отзыв, как он лежит в базе.
type Entry struct {
	ID        int64
	Text      string
	Status    string
	Issue     *int32
	Build     *int64
	CreatedAt time.Time
}

type Service struct {
	db  *pgxpool.Pool
	cfg Config
	gh  *github
}

func New(db *pgxpool.Pool, cfg Config) *Service {
	if cfg.Repo == "" {
		cfg.Repo = DefaultRepo
	}
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}
	if cfg.Media == nil {
		cfg.Media = media.NewDisk(filepath.Join(os.TempDir(), "moya-dacha-media"), "/media")
	}
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	return &Service{
		db:  db,
		cfg: cfg,
		gh: &github{
			token:  cfg.Token,
			repo:   cfg.Repo,
			apiURL: trimAPI(cfg.APIURL),
			http:   &http.Client{Timeout: 30 * time.Second},
		},
	}
}

// Enabled — задан ли токен GitHub.
func (s *Service) Enabled() bool { return s.cfg.Token != "" }

// Repo — репозиторий задач.
func (s *Service) Repo() string { return s.cfg.Repo }

// Add записывает отзыв в статусе sent. Скриншот не картинка —
// media.ErrNotAnImage.
func (s *Service) Add(ctx context.Context, it Item) (Entry, error) {
	var key *string
	if len(it.Screenshot) > 0 {
		photo, err := media.NormalizePhoto(it.Screenshot)
		if err != nil {
			return Entry{}, err
		}
		k, err := media.Key(ScreenshotDir, media.PhotoExt)
		if err != nil {
			return Entry{}, err
		}
		if err := s.cfg.Media.Put(ctx, k, photo.Content); err != nil {
			return Entry{}, err
		}
		key = &k
	}

	var chat, thread, message *int64
	var userID *string
	if it.UserID != "" {
		userID = &it.UserID
	}
	if r := it.Telegram; r != nil {
		chat, message = &r.ChatID, &r.ReplyTo
		if r.ThreadID != 0 {
			thread = &r.ThreadID
		}
	}

	e := Entry{Text: it.Text, Status: StatusSent}
	err := s.db.QueryRow(ctx, `
		INSERT INTO feedback (source, user_id, author, tg_chat_id, tg_thread_id,
		                      tg_message_id, tg_private, text, screenshot,
		                      app_version, device)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), NULLIF($11, ''))
		RETURNING id, created_at`,
		it.Source, userID, it.Author, chat, thread, message, it.Private, it.Text, key,
		it.AppVersion, it.Device,
	).Scan(&e.ID, &e.CreatedAt)
	return e, err
}

// Mine — отзывы автора из приложения, новые сверху, до 50 (требование 19).
func (s *Service) Mine(ctx context.Context, userID string) ([]Entry, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, text, status, issue, build, created_at
		FROM feedback WHERE user_id = $1
		ORDER BY created_at DESC, id DESC LIMIT 50`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Entry, error) {
		var e Entry
		err := row.Scan(&e.ID, &e.Text, &e.Status, &e.Issue, &e.Build, &e.CreatedAt)
		return e, err
	})
}

// Run проходит Sync раз в SyncEvery, пока не отменён ctx.
func (s *Service) Run(ctx context.Context, n Notifier) {
	ticker := time.NewTicker(SyncEvery)
	defer ticker.Stop()
	for {
		if err := s.Sync(ctx, n); err != nil && ctx.Err() == nil {
			slog.Error("feedback: сверка с GitHub", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Sync — один проход: завести задачи для отзывов sent и сверить статусы
// остальных (требования 7–10). Ошибка одного отзыва не останавливает
// остальные; вернётся первая.
func (s *Service) Sync(ctx context.Context, n Notifier) error {
	if !s.Enabled() {
		return nil
	}
	rows, err := s.db.Query(ctx, `SELECT id FROM feedback WHERE status = 'sent' ORDER BY id`)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	var first error
	for _, id := range ids {
		if err := s.Submit(ctx, id, n); err != nil && first == nil {
			first = err
		}
	}
	if err := s.refresh(ctx, n); err != nil && first == nil {
		first = err
	}
	return first
}

// Submit заводит задачу для отзыва id, если её ещё нет (требование 7),
// и отвечает автору из Telegram номером (требование 9). Два одновременных
// вызова для одного отзыва задачу не удвоят: строка держится под
// блокировкой, пока GitHub отвечает.
func (s *Service) Submit(ctx context.Context, id int64, n Notifier) error {
	if !s.Enabled() {
		return nil
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // после Commit — no-op

	var (
		source, author, text  string
		screenshot, ver, dev  *string
		private               bool
		chat, thread, message *int64
	)
	err = tx.QueryRow(ctx, `
		SELECT source, author, text, screenshot, app_version, device, tg_private,
		       tg_chat_id, tg_thread_id, tg_message_id
		FROM feedback WHERE id = $1 AND status = 'sent'
		FOR UPDATE SKIP LOCKED`, id).
		Scan(&source, &author, &text, &screenshot, &ver, &dev, &private, &chat, &thread, &message)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	labels := []string{labelInbox, labelApp}
	if source == SourceTelegram {
		labels = []string{labelInbox, labelTelegram}
	}
	issue, err := s.gh.create(ctx, Title(text), s.body(source, private, author, text, screenshot, ver, dev), labels)
	if err != nil {
		return fmt.Errorf("задача для отзыва %d: %w", id, err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE feedback SET status = 'accepted', issue = $2, issue_url = $3 WHERE id = $1`,
		id, issue.Number, issue.URL); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	notify(ctx, n, recipient(chat, thread, message), fmt.Sprintf(textTicket, issue.Number))
	return nil
}

// Title — «Отзыв: » и первая непустая строка, до 60 символов
// (требование 4).
func Title(text string) string {
	line := ""
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			line = l
			break
		}
	}
	if utf8.RuneCountInString(line) > 60 {
		line = string([]rune(line)[:60]) + "…"
	}
	return "Отзыв: " + line
}

// body — тело задачи (требование 5).
func (s *Service) body(source string, private bool, author, text string, screenshot, ver, dev *string) string {
	var b strings.Builder
	b.WriteString(text)
	b.WriteString("\n\n")
	if screenshot != nil {
		link := s.cfg.Media.URL(*screenshot)
		if strings.HasPrefix(link, "/") {
			link = s.cfg.PublicURL + link
		}
		fmt.Fprintf(&b, "![скриншот](%s)\n\n", link)
	}
	b.WriteString("---\n")
	from := "приложение"
	if source == SourceTelegram {
		from = "Telegram, тема группы"
		if private {
			from = "Telegram, личка"
		}
	}
	fmt.Fprintf(&b, "Откуда: %s\nАвтор: %s", from, author)
	if ver != nil {
		fmt.Fprintf(&b, "\nВерсия: %s", *ver)
	}
	if dev != nil {
		fmt.Fprintf(&b, "\nТелефон: %s", *dev)
	}
	return b.String()
}

// tracked — отзыв, чья задача сверяется с GitHub.
type tracked struct {
	id                    int64
	issue                 int
	status                string
	chat, thread, message *int64
}

// refresh сверяет статусы с задачами (требование 8).
func (s *Service) refresh(ctx context.Context, n Notifier) error {
	rows, err := s.db.Query(ctx, `
		SELECT id, issue, status, tg_chat_id, tg_thread_id, tg_message_id
		FROM feedback
		WHERE status IN ('accepted', 'approved', 'done') AND issue IS NOT NULL
		ORDER BY id`)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (tracked, error) {
		var t tracked
		err := row.Scan(&t.id, &t.issue, &t.status, &t.chat, &t.thread, &t.message)
		return t, err
	})
	if err != nil {
		return err
	}

	// Одна задача GitHub читается один раз за проход.
	issues := map[int]Issue{}
	var first error
	for _, t := range list {
		issue, ok := issues[t.issue]
		if !ok {
			issue, err = s.gh.issue(ctx, t.issue)
			if err != nil {
				if first == nil {
					first = fmt.Errorf("задача #%d: %w", t.issue, err)
				}
				continue
			}
			issues[t.issue] = issue
		}
		next := nextStatus(t.status, issue)
		if next == t.status {
			continue
		}
		tag, err := s.db.Exec(ctx, `UPDATE feedback SET status = $3 WHERE id = $1 AND status = $2`,
			t.id, t.status, next)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 && next == StatusApproved {
			notify(ctx, n, recipient(t.chat, t.thread, t.message), fmt.Sprintf(textOK, t.issue))
		}
	}
	return first
}

func nextStatus(current string, issue Issue) string {
	if issue.State == "closed" {
		if issue.StateReason == "not_planned" || issue.StateReason == "duplicate" {
			return StatusDeclined
		}
		return StatusDone
	}
	if current == StatusAccepted && issue.hasLabel(approvedLabels...) {
		return StatusApproved
	}
	return current
}

// Release — сборка build ушла в группу (требование 11): сначала сверка,
// потом всё сделанное становится вышедшим в этой сборке.
func (s *Service) Release(ctx context.Context, build int64, n Notifier) error {
	if err := s.Sync(ctx, n); err != nil {
		// Сверка неполная — выпускается то, что уже известно сделанным.
		slog.Error("feedback: сверка перед выпуском", "err", err)
	}
	rows, err := s.db.Query(ctx, `
		UPDATE feedback SET status = 'released', build = $1
		WHERE status = 'done'
		RETURNING issue, tg_chat_id, tg_thread_id, tg_message_id`, build)
	if err != nil {
		return err
	}
	type out struct {
		issue                 int
		chat, thread, message *int64
	}
	done, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (out, error) {
		var o out
		err := row.Scan(&o.issue, &o.chat, &o.thread, &o.message)
		return o, err
	})
	if err != nil {
		return err
	}
	for _, o := range done {
		notify(ctx, n, recipient(o.chat, o.thread, o.message), fmt.Sprintf(textOut, o.issue, build))
	}
	return nil
}

// Approve ставит задаче «одобрено» и сразу сообщает авторам её отзывов
// (требование 25).
func (s *Service) Approve(ctx context.Context, issue int, n Notifier) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}
	if err := s.gh.addLabel(ctx, issue, labelApproved); err != nil {
		return err
	}
	rows, err := s.db.Query(ctx, `
		UPDATE feedback SET status = 'approved'
		WHERE issue = $1 AND status = 'accepted'
		RETURNING tg_chat_id, tg_thread_id, tg_message_id`, issue)
	if err != nil {
		return err
	}
	type out struct{ chat, thread, message *int64 }
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (out, error) {
		var o out
		err := row.Scan(&o.chat, &o.thread, &o.message)
		return o, err
	})
	if err != nil {
		return err
	}
	for _, o := range list {
		notify(ctx, n, recipient(o.chat, o.thread, o.message), fmt.Sprintf(textOK, issue))
	}
	return nil
}

// Inbox — открытые задачи «входящее», новые сверху, до 20 (требование 24).
func (s *Service) Inbox(ctx context.Context) ([]Issue, error) {
	if !s.Enabled() {
		return nil, ErrNotConfigured
	}
	return s.gh.open(ctx, labelInbox, 20)
}

// Check — принимает ли GitHub токен и видит ли репозиторий
// (требование 29).
func (s *Service) Check(ctx context.Context) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}
	return s.gh.check(ctx)
}

func recipient(chat, thread, message *int64) *Recipient {
	if chat == nil {
		return nil
	}
	r := &Recipient{ChatID: *chat}
	if thread != nil {
		r.ThreadID = *thread
	}
	if message != nil {
		r.ReplyTo = *message
	}
	return r
}

// notify отвечает автору, если он из Telegram и есть кому доставить.
// Неудача не отменяет смену статуса (требование 13).
func notify(ctx context.Context, n Notifier, to *Recipient, text string) {
	if n == nil || to == nil {
		return
	}
	if err := n.Notify(ctx, *to, text); err != nil {
		slog.Error("feedback: ответ автору не ушёл", "chat", to.ChatID, "err", err)
	}
}
