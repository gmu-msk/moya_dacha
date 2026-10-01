package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/feedback"
	"github.com/gmu-msk/moya_dacha/backend/internal/monitor"
)

// Тексты ответов (specs/018-telegram-bot.md, «Тексты»).
const (
	textOwnerBound = "Готово: сюда будут приходить тревоги и тестовые сборки."
	textGroupBound = "Готово: сюда будут приходить сборки приложения."
	// Незнакомцу в личке — как написать отзыв (specs/019-feedback.md,
	// требование 23).
	textStranger = "Здравствуйте! Это бот МоейДачи. Напишите сюда идею или что сломалось — я передам разработчику и пришлю номер задачи."

	// Команды отзывов (specs/019-feedback.md, требования 20, 24–26).
	textIdeasBound = "Готово: сообщения отсюда станут задачами."
	textInboxEmpty = "Входящих нет."
	textApproveUse = "Напишите номер задачи: /одобрить 71"
	textNoGitHub   = "GitHub не настроен: нет FEEDBACK_GITHUB_TOKEN."
	textNoCaption  = "Скриншот без подписи"
	// textTestersOnly — ответ в личку не участнику группы (019, 21а).
	textTestersOnly = "Отзывы здесь принимаются только от тестировщиков МоейДачи."
)

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	MessageID       int64  `json:"message_id"`
	MessageThreadID int64  `json:"message_thread_id"`
	IsTopicMessage  bool   `json:"is_topic_message"`
	Text            string `json:"text"`
	Caption         string `json:"caption"`
	Photo           []struct {
		FileID string `json:"file_id"`
	} `json:"photo"`
	// Contact — карточка из телефонной книги (требование 28).
	Contact *struct {
		PhoneNumber string `json:"phone_number"`
	} `json:"contact"`
	From *struct {
		ID        int64  `json:"id"`
		IsBot     bool   `json:"is_bot"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"from"`
	Chat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
}

// Run читает обновления, пока не отменён ctx (требование 21).
func (b *Bot) Run(ctx context.Context) {
	b.syncMenu(ctx)
	var offset int64
	for ctx.Err() == nil {
		var updates []update
		err := b.callJSON(ctx, "getUpdates", map[string]any{
			"offset":          offset,
			"timeout":         int(b.cfg.PollTimeout.Seconds()),
			"allowed_updates": []string{"message"},
		}, &updates)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logError("не удалось получить обновления", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		for _, u := range updates {
			offset = max(offset, u.UpdateID+1)
			if u.Message != nil {
				if err := b.handle(ctx, u.Message); err != nil {
					logError("не удалось ответить на сообщение", err)
				}
			}
		}
	}
}

// command — первое слово без «@бот» в нижнем регистре; пусто, если
// сообщение не команда (требование 8).
func command(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return ""
	}
	cmd, _, _ := strings.Cut(fields[0], "@")
	return strings.ToLower(cmd)
}

// isOwner — от владельца ли сообщение (требование 2а): пока личка не
// привязана — по нику, после — только по id, равному номеру лички. Ник
// можно сменить и занять, а владелец выдаёт приглашения.
func (b *Bot) isOwner(ctx context.Context, m *message) (bool, error) {
	if b.cfg.Owner == "" || m.From == nil {
		return false, nil
	}
	chat, err := b.chat(ctx, RoleOwner)
	switch {
	case errors.Is(err, ErrNotBound):
		return strings.EqualFold(m.From.Username, b.cfg.Owner), nil
	case err != nil:
		return false, err
	}
	return m.From.ID == chat, nil
}

func (b *Bot) handle(ctx context.Context, m *message) error {
	cmd := command(m.Text)
	private := m.Chat.Type == "private"
	group := m.Chat.Type == "group" || m.Chat.Type == "supergroup"
	owner, err := b.isOwner(ctx, m)
	if err != nil {
		return err
	}

	// Номер для приглашения: контакт или ответ на /invite без номера
	// (требования 27–28). Команда снимает ожидание.
	if private && owner {
		if cmd != "" {
			b.setAwaitPhone(false)
		} else if phone, ok := b.phoneFromOwner(m); ok {
			text, err := b.invite(ctx, phone)
			if err != nil {
				return err
			}
			return b.Send(ctx, m.Chat.ID, text)
		}
	}
	if cmd == "" {
		return b.takeFeedback(ctx, m, owner)
	}

	if !owner {
		// В группе чужие команды молча пропускаются (требование 7).
		if private {
			return b.Send(ctx, m.Chat.ID, textStranger)
		}
		return nil
	}

	switch {
	case cmd == "/start" && private:
		if err := b.bind(ctx, RoleOwner, m.Chat.ID, nil); err != nil {
			return err
		}
		b.syncMenu(ctx)
		return b.Send(ctx, m.Chat.ID, textOwnerBound)
	case (cmd == "/group" || cmd == "/группа") && group:
		if err := b.bind(ctx, RoleGroup, m.Chat.ID, nil); err != nil {
			return err
		}
		return b.Send(ctx, m.Chat.ID, textGroupBound)
	case (cmd == "/ideas" || cmd == "/идеи") && group:
		var thread *int64
		if m.IsTopicMessage && m.MessageThreadID != 0 {
			thread = &m.MessageThreadID
		}
		if err := b.bind(ctx, RoleIdeas, m.Chat.ID, thread); err != nil {
			return err
		}
		return b.reply(ctx, m, textIdeasBound)
	case isInviteCommand(cmd) && private:
		text, err := b.inviteCommand(ctx, cmd, m.Text)
		if err != nil {
			return err
		}
		return b.Send(ctx, m.Chat.ID, text)
	case isInviteCommand(cmd):
		// Код не должен попасть в группу (требование 32).
		return b.reply(ctx, m, textInvitesGroup)
	case cmd == "/inbox" || cmd == "/входящие":
		return b.reply(ctx, m, b.inbox(ctx))
	case cmd == "/approve" || cmd == "/одобрить":
		return b.reply(ctx, m, b.approve(ctx, m.Text))
	case cmd == "/status" || cmd == "/статус":
		text, err := b.Status(ctx)
		if err != nil {
			return err
		}
		return b.Send(ctx, m.Chat.ID, text)
	}
	return nil
}

// Status — сводка здоровья для /status (требование 9).
func (b *Bot) Status(ctx context.Context) (string, error) {
	snap, err := monitor.New(b.db, b.cfg.DiskPath).Collect(ctx)
	if err != nil {
		return "", err
	}

	var s strings.Builder
	if len(snap.Alerts) == 0 {
		s.WriteString("Всё в порядке\n")
	} else {
		s.WriteString("Тревоги:\n")
		for _, a := range snap.Alerts {
			fmt.Fprintf(&s, "• %s\n", a.Message)
		}
	}
	fmt.Fprintf(&s, "Людей: %d, постов: %d\n", snap.Totals.Users, snap.Totals.Posts)
	fmt.Fprintf(&s, "Ошибок за сутки: %d\n", snap.Errors.LastDay)
	if now := snap.Server.Now; now != nil {
		fmt.Fprintf(&s, "Процессор %.0f%%, память %s из %s, диск %s из %s\n",
			now.CPUPercent,
			monitor.Bytes(now.MemoryUsedBytes), monitor.Bytes(now.MemoryTotalBytes),
			monitor.Bytes(now.DiskUsedBytes), monitor.Bytes(now.DiskTotalBytes))
	}
	fmt.Fprintf(&s, "Работает: %s", uptime(monitor.Uptime()))
	return s.String(), nil
}

// uptime — «3 д 4 ч», «5 ч 12 мин», «7 мин».
func uptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d д %d ч", days, hours)
	case hours > 0:
		return fmt.Sprintf("%d ч %d мин", hours, minutes)
	default:
		return fmt.Sprintf("%d мин", minutes)
	}
}

// reply отвечает в том же чате и той же теме.
func (b *Bot) reply(ctx context.Context, m *message, text string) error {
	to := feedback.Recipient{ChatID: m.Chat.ID}
	if m.IsTopicMessage {
		to.ThreadID = m.MessageThreadID
	}
	return b.Notify(ctx, to, text)
}

// takeFeedback превращает сообщение в отзыв, если оно в личке или в теме
// для идей (specs/019-feedback.md, требования 21–22, 9).
func (b *Bot) takeFeedback(ctx context.Context, m *message, owner bool) error {
	fb := b.cfg.Feedback
	if fb == nil || m.From == nil || m.From.IsBot {
		return nil
	}
	private := m.Chat.Type == "private"
	if private && !owner && !b.isTester(ctx, m.From.ID) {
		// Бота находит поиском кто угодно, а отзыв — задача в публичном
		// репозитории и файл на диске (specs/019-feedback.md, 21а).
		return b.Send(ctx, m.Chat.ID, textTestersOnly)
	}
	if !private {
		chat, thread, ok, err := b.ideas(ctx)
		if err != nil || !ok || chat != m.Chat.ID {
			return err
		}
		if thread != nil && !(m.IsTopicMessage && m.MessageThreadID == *thread) {
			return nil
		}
	}

	text := strings.TrimSpace(m.Text)
	if text == "" {
		text = strings.TrimSpace(m.Caption)
	}
	var shot []byte
	if len(m.Photo) > 0 {
		// Последний размер — самый большой.
		raw, err := b.download(ctx, m.Photo[len(m.Photo)-1].FileID)
		if err != nil {
			logError("не удалось скачать фото отзыва", err)
		} else {
			shot = raw
		}
		if text == "" {
			text = textNoCaption
		}
	}
	if text == "" {
		return nil
	}

	to := &feedback.Recipient{ChatID: m.Chat.ID, ReplyTo: m.MessageID}
	if m.IsTopicMessage {
		to.ThreadID = m.MessageThreadID
	}
	item := feedback.Item{
		Source:     feedback.SourceTelegram,
		Author:     author(m),
		Text:       text,
		Screenshot: shot,
		Telegram:   to,
		Private:    private,
	}
	entry, err := fb.Add(ctx, item)
	if err != nil && len(shot) > 0 {
		// Картинка не разобралась — отзыв важнее скриншота.
		logError("фото отзыва не картинка", err)
		item.Screenshot = nil
		entry, err = fb.Add(ctx, item)
	}
	if err != nil {
		return err
	}
	if !fb.Enabled() {
		return b.Notify(ctx, *to, feedback.TextThanks)
	}
	// Не вышло — задачу заведёт и ответит следующая сверка.
	if err := fb.Submit(ctx, entry.ID, b); err != nil {
		logError("задача GitHub не заведена, повторю", err)
	}
	return nil
}

// author — «@ник» или имя и фамилия (требование 22).
func author(m *message) string {
	if m.From.Username != "" {
		return "@" + m.From.Username
	}
	return strings.TrimSpace(m.From.FirstName + " " + m.From.LastName)
}

// inbox — ответ на /входящие (требование 24).
func (b *Bot) inbox(ctx context.Context) string {
	if b.cfg.Feedback == nil {
		return textNoGitHub
	}
	issues, err := b.cfg.Feedback.Inbox(ctx)
	switch {
	case errors.Is(err, feedback.ErrNotConfigured):
		return textNoGitHub
	case err != nil:
		logError("не удалось прочитать входящие", err)
		return "Не получилось спросить GitHub: " + err.Error()
	case len(issues) == 0:
		return textInboxEmpty
	}
	var s strings.Builder
	s.WriteString("Входящие:")
	for _, i := range issues {
		fmt.Fprintf(&s, "\n#%d %s", i.Number, i.Title)
	}
	return s.String()
}

// approve — ответ на /одобрить N (требования 25–26).
func (b *Bot) approve(ctx context.Context, text string) string {
	if b.cfg.Feedback == nil {
		return textNoGitHub
	}
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return textApproveUse
	}
	n, err := strconv.Atoi(strings.TrimPrefix(fields[1], "#"))
	if err != nil || n <= 0 {
		return textApproveUse
	}
	err = b.cfg.Feedback.Approve(ctx, n, b)
	switch {
	case errors.Is(err, feedback.ErrNotConfigured):
		return textNoGitHub
	case errors.Is(err, feedback.ErrNoIssue):
		return fmt.Sprintf("Задачи #%d нет.", n)
	case err != nil:
		logError("не удалось одобрить задачу", err)
		return "Не получилось: " + err.Error()
	}
	return fmt.Sprintf("#%d одобрена.", n)
}

// isTester — состоит ли человек в привязанной группе тестировщиков
// (specs/019-feedback.md, требование 21а). Группа не привязана или
// Telegram ответил ошибкой — нет: без ответа лучше не принять отзыв, чем
// принять от постороннего.
func (b *Bot) isTester(ctx context.Context, userID int64) bool {
	group, err := b.chat(ctx, RoleGroup)
	if err != nil {
		if !errors.Is(err, ErrNotBound) {
			logError("не удалось прочитать группу тестировщиков", err)
		}
		return false
	}
	var member struct {
		Status   string `json:"status"`
		IsMember bool   `json:"is_member"`
	}
	if err := b.callJSON(ctx, "getChatMember",
		map[string]any{"chat_id": group, "user_id": userID}, &member); err != nil {
		logError("не удалось проверить участника группы", err)
		return false
	}
	switch member.Status {
	case "creator", "administrator", "member":
		return true
	case "restricted":
		return member.IsMember
	}
	return false
}
