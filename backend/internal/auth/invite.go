package auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB — то, что нужно приглашениям от базы: хватает и пула, и транзакции.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Invite — действующее приглашение, как его видит владелец сервиса.
// Кода здесь нет: он не хранится (specs/015-invites.md, требование 11).
type Invite struct {
	Phone        string
	CreatedAt    time.Time
	AttemptsLeft int
}

// IssueInvite выдаёт приглашение на номер и возвращает нормализованный
// номер и код. Прежнее приглашение на этот номер заменяется, попытки
// начинаются заново (specs/015-invites.md, требование 4).
func IssueInvite(ctx context.Context, db DB, phone string) (string, string, error) {
	normalized, err := NormalizePhone(phone)
	if err != nil {
		return "", "", err
	}
	code, err := GenerateCode()
	if err != nil {
		return "", "", err
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO invites (phone, code_hash, attempts_left, created_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (phone) DO UPDATE
		SET code_hash     = excluded.code_hash,
		    attempts_left = excluded.attempts_left,
		    created_at    = excluded.created_at`,
		normalized, Hash(code), MaxAttempts,
	); err != nil {
		return "", "", err
	}
	return normalized, code, nil
}

// RevokeInvite отзывает приглашение на номер. Отзыв несуществующего
// приглашения — не ошибка: результат тот же, приглашения нет.
func RevokeInvite(ctx context.Context, db DB, phone string) error {
	normalized, err := NormalizePhone(phone)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `DELETE FROM invites WHERE phone = $1`, normalized)
	return err
}

// ListInvites возвращает действующие приглашения, свежие сверху.
// Приглашения с исчерпанными попытками уже не действуют и в список
// не попадают.
func ListInvites(ctx context.Context, db DB) ([]Invite, error) {
	rows, err := db.Query(ctx, `
		SELECT phone, created_at, attempts_left
		FROM invites
		WHERE attempts_left > 0
		ORDER BY created_at DESC, phone`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Invite, error) {
		var invite Invite
		err := row.Scan(&invite.Phone, &invite.CreatedAt, &invite.AttemptsLeft)
		return invite, err
	})
}
