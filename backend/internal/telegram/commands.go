package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/monitor"
)

// Тексты ответов (specs/018-telegram-bot.md, «Тексты»).
const (
	textOwnerBound = "Готово: сюда будут приходить тревоги и тестовые сборки."
	textGroupBound = "Готово: сюда будут приходить сборки приложения."
	textStranger   = "Это служебный бот МоейДачи."
)

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	Text string `json:"text"`
	From *struct {
		Username string `json:"username"`
	} `json:"from"`
	Chat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
}

// Run читает обновления, пока не отменён ctx (требование 21).
func (b *Bot) Run(ctx context.Context) {
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

func (b *Bot) isOwner(m *message) bool {
	return b.cfg.Owner != "" && m.From != nil && strings.EqualFold(m.From.Username, b.cfg.Owner)
}

func (b *Bot) handle(ctx context.Context, m *message) error {
	cmd := command(m.Text)
	if cmd == "" {
		return nil
	}
	private := m.Chat.Type == "private"
	group := m.Chat.Type == "group" || m.Chat.Type == "supergroup"

	if !b.isOwner(m) {
		// В группе чужие команды молча пропускаются (требование 7).
		if private {
			return b.Send(ctx, m.Chat.ID, textStranger)
		}
		return nil
	}

	switch {
	case cmd == "/start" && private:
		if err := b.bind(ctx, RoleOwner, m.Chat.ID); err != nil {
			return err
		}
		return b.Send(ctx, m.Chat.ID, textOwnerBound)
	case (cmd == "/group" || cmd == "/группа") && group:
		if err := b.bind(ctx, RoleGroup, m.Chat.ID); err != nil {
			return err
		}
		return b.Send(ctx, m.Chat.ID, textGroupBound)
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
