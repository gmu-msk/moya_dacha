package telegram

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/monitor"
)

// RunAlerts проверяет тревоги раз в минуту, пока не отменён ctx
// (требование 10).
func (b *Bot) RunAlerts(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if err := b.CheckAlerts(ctx); err != nil {
			logError("не удалось проверить тревоги", err)
		}
	}
}

// CheckAlerts — одна проверка тревог (требования 10–14). Пишет владельцу,
// только когда изменился набор видов тревог.
func (b *Bot) CheckAlerts(ctx context.Context) error {
	alerts, err := monitor.Alerts(ctx, b.db, b.cfg.DiskPath)
	if err != nil {
		return err
	}
	kinds := make([]string, 0, len(alerts))
	for _, a := range alerts {
		kinds = append(kinds, a.Kind)
	}
	slices.Sort(kinds)
	set := strings.Join(kinds, ",")

	b.mu.Lock()
	defer b.mu.Unlock()
	if set == b.sentAlerts {
		return nil
	}

	var text string
	if len(alerts) == 0 {
		text = "Всё в порядке: тревог больше нет."
	} else {
		var s strings.Builder
		s.WriteString("Тревога на проде:")
		for _, a := range alerts {
			s.WriteString("\n• " + a.Message)
		}
		text = s.String()
	}

	chat, err := b.chat(ctx, RoleOwner)
	if errors.Is(err, ErrNotBound) {
		// Набор не считается отправленным: после привязки тревога придёт
		// на следующей проверке (требование 14).
		return nil
	}
	if err != nil {
		return err
	}
	if err := b.Send(ctx, chat, text); err != nil {
		return err
	}
	b.sentAlerts = set
	return nil
}
