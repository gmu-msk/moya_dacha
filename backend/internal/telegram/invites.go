package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/auth"
)

// Меню и приглашения из лички (specs/018-telegram-bot.md, требования 23–32).
const (
	textAskPhone     = "Пришлите номер телефона или контакт из телефонной книги."
	textBadPhone     = "Это не российский мобильный номер. Пришлите вида +7 900 123-45-67."
	textUninviteUse  = "Напишите номер: /uninvite +7 900 123-45-67"
	textInvitesNone  = "Действующих приглашений нет."
	textInvitesGroup = "Приглашения — только в личке бота."
)

// ownerCommands — меню лички владельца, по порядку (требование 24).
// Telegram принимает в меню только латиницу.
var ownerCommands = []map[string]string{
	{"command": "invite", "description": "Код входа по номеру телефона"},
	{"command": "invites", "description": "Действующие приглашения"},
	{"command": "uninvite", "description": "Отозвать приглашение"},
	{"command": "status", "description": "Сводка сервера"},
	{"command": "inbox", "description": "Входящие задачи"},
	{"command": "approve", "description": "Одобрить задачу"},
}

// moscow — время в списке приглашений. Летнего времени в Москве нет,
// а tzdata на сервере может и не быть.
var moscow = time.FixedZone("MSK", 3*60*60)

// syncMenu отправляет меню в личку владельца, если она привязана
// (требования 23, 25). Ошибка — только в лог.
func (b *Bot) syncMenu(ctx context.Context) {
	chat, err := b.chat(ctx, RoleOwner)
	if errors.Is(err, ErrNotBound) {
		return
	}
	if err == nil {
		err = b.callJSON(ctx, "setMyCommands", map[string]any{
			"commands": ownerCommands,
			"scope":    map[string]any{"type": "chat", "chat_id": chat},
		}, nil)
	}
	if err != nil {
		logError("не удалось отправить меню команд", err)
	}
}

func isInviteCommand(cmd string) bool {
	switch cmd {
	case "/invite", "/пригласить", "/uninvite", "/отозвать", "/invites", "/приглашения":
		return true
	}
	return false
}

// inviteCommand — ответ на команды приглашений в личке владельца
// (требования 26–27, 30–31).
func (b *Bot) inviteCommand(ctx context.Context, cmd, text string) (string, error) {
	_, arg, _ := strings.Cut(strings.TrimSpace(text), " ")
	arg = strings.TrimSpace(arg)
	switch cmd {
	case "/invite", "/пригласить":
		if arg == "" {
			b.setAwaitPhone(true)
			return textAskPhone, nil
		}
		return b.invite(ctx, arg)
	case "/uninvite", "/отозвать":
		if arg == "" {
			return textUninviteUse, nil
		}
		normalized, err := auth.NormalizePhone(arg)
		if err != nil {
			return textBadPhone, nil
		}
		if err := auth.RevokeInvite(ctx, b.db, normalized); err != nil {
			return "", err
		}
		return fmt.Sprintf("Приглашение на %s отозвано.", normalized), nil
	default:
		return b.invites(ctx)
	}
}

// invite выдаёт приглашение и возвращает ответ с кодом (требования 26, 29).
func (b *Bot) invite(ctx context.Context, phone string) (string, error) {
	normalized, code, err := auth.IssueInvite(ctx, b.db, phone)
	if errors.Is(err, auth.ErrInvalidPhone) {
		return textBadPhone, nil
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Приглашение в МоюДачу\nНомер: %s\nКод: %s", normalized, code), nil
}

// invites — ответ на /приглашения (требование 31).
func (b *Bot) invites(ctx context.Context) (string, error) {
	list, err := auth.ListInvites(ctx, b.db)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return textInvitesNone, nil
	}
	var s strings.Builder
	s.WriteString("Действующие приглашения:")
	for _, i := range list {
		fmt.Fprintf(&s, "\n%s — попыток %d, выдано %s",
			i.Phone, i.AttemptsLeft, i.CreatedAt.In(moscow).Format("02.01 15:04"))
	}
	return s.String(), nil
}

// phoneFromOwner — номер из контакта или из ответа на /invite без номера
// (требования 27–28). ok == false — сообщение не про приглашение.
func (b *Bot) phoneFromOwner(m *message) (string, bool) {
	awaited := b.setAwaitPhone(false)
	if m.Contact != nil && m.Contact.PhoneNumber != "" {
		return m.Contact.PhoneNumber, true
	}
	if awaited {
		return m.Text, true
	}
	return "", false
}

// setAwaitPhone ставит ожидание номера и возвращает прежнее.
func (b *Bot) setAwaitPhone(v bool) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	was := b.awaitPhone
	b.awaitPhone = v
	return was
}
