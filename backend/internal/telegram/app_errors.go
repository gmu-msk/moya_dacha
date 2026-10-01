package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/internal/monitor"
)

// newErrorsPerCheck — сколько новых ошибок приложения уходит за одну
// проверку (specs/021-app-errors.md, требование 8).
const newErrorsPerCheck = 5

type newAppError struct {
	group    string
	text     string
	stack    string
	version  string
	build    int64
	screen   string
	nickname *string
}

// CheckAppErrors — одна проверка новых ошибок приложения
// (specs/021-app-errors.md, требования 8–10): о каждой новой группе —
// сообщение владельцу, после отправки группа помечается в базе.
func (b *Bot) CheckAppErrors(ctx context.Context) error {
	if b.cfg.Token == "" {
		return nil
	}
	chat, err := b.chat(ctx, RoleOwner)
	if errors.Is(err, ErrNotBound) {
		return nil
	}
	if err != nil {
		return err
	}

	rows, err := b.db.Query(ctx, `
		SELECT g.id::text, e.error, e.stack, e.version, e.build, e.screen, u.nickname
		FROM app_error_groups g
		JOIN LATERAL (
			SELECT * FROM app_errors e WHERE e.group_id = g.id
			ORDER BY at, id LIMIT 1
		) e ON true
		LEFT JOIN users u ON u.id = e.user_id
		WHERE g.notified_at IS NULL AND g.first_at > now() - interval '24 hours'
		  AND e.user_id IS NOT NULL
		ORDER BY g.first_at, g.id
		LIMIT $1`, newErrorsPerCheck)
	if err != nil {
		return err
	}
	fresh, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (newAppError, error) {
		var e newAppError
		err := row.Scan(&e.group, &e.text, &e.stack, &e.version, &e.build, &e.screen, &e.nickname)
		return e, err
	})
	if err != nil {
		return err
	}

	for _, e := range fresh {
		if err := b.Send(ctx, chat, e.message()); err != nil {
			return err
		}
		if _, err := b.db.Exec(ctx, `UPDATE app_error_groups SET notified_at = now() WHERE id = $1`, e.group); err != nil {
			return err
		}
	}
	return b.checkAnonymousErrors(ctx, chat)
}

// checkAnonymousErrors — новые группы, чей первый отчёт пришёл без входа
// (требование 8а). Их текст мог прислать кто угодно, поэтому владельцу
// уходит только счётчик одним сообщением, а сами ошибки — на дашборде.
func (b *Bot) checkAnonymousErrors(ctx context.Context, chat int64) error {
	rows, err := b.db.Query(ctx, `
		SELECT g.id::text
		FROM app_error_groups g
		JOIN LATERAL (
			SELECT user_id FROM app_errors e WHERE e.group_id = g.id
			ORDER BY at, id LIMIT 1
		) e ON true
		WHERE g.notified_at IS NULL AND g.first_at > now() - interval '24 hours'
		  AND e.user_id IS NULL`)
	if err != nil {
		return err
	}
	groups, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(groups) == 0 {
		return err
	}
	text := fmt.Sprintf("Новые ошибки без входа: %d. Подробности — на дашборде.", len(groups))
	if err := b.Send(ctx, chat, text); err != nil {
		return err
	}
	_, err = b.db.Exec(ctx,
		`UPDATE app_error_groups SET notified_at = now() WHERE id = ANY($1::uuid[])`, groups)
	return err
}

// message — текст сообщения по требованию 8.
func (e newAppError) message() string {
	text := []rune(e.text)
	if len(text) > 500 {
		text = text[:500]
	}
	lines := []string{"Новая ошибка в приложении:", string(text)}
	if where := monitor.ErrorWhere(e.stack); where != "" {
		lines = append(lines, "Где: "+where)
	}
	build := "Сборка ?"
	if e.version != "" {
		build = fmt.Sprintf("Сборка %s (%d)", e.version, e.build)
	}
	if e.screen != "" {
		build += ", экран " + e.screen
	}
	lines = append(lines, build)
	if e.nickname != nil {
		lines = append(lines, "Кто: @"+*e.nickname)
	} else {
		lines = append(lines, "Кто: без входа")
	}
	return strings.Join(lines, "\n")
}
