// Вход по коду приглашения: specs/015-invites.md.
package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
	"github.com/gmu-msk/moya_dacha/backend/internal/auth"
)

// requestInviteCode — запрос кода в режиме приглашений. Ничего не
// выдаётся и не отправляется: код у человека уже есть, сервису остаётся
// сказать, есть ли на номер действующее приглашение (требование 7).
func (s *Server) requestInviteCode(ctx context.Context, phone string) (gen.RequestAuthCodeResponseObject, error) {
	var invited bool
	if err := s.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM invites WHERE phone = $1 AND attempts_left > 0)`,
		phone,
	).Scan(&invited); err != nil {
		return nil, err
	}
	if !invited {
		return gen.RequestAuthCode403JSONResponse(errNotInvited), nil
	}
	return gen.RequestAuthCode202JSONResponse{Delivery: gen.Invite}, nil
}

// consumeInvite сверяет код с приглашением на номер. Срока у приглашения
// нет, поэтому проверок две: попытки, затем совпадение (требование 9).
// Верный код сжигает приглашение, неверный списывает попытку.
func (s *Server) consumeInvite(ctx context.Context, tx pgx.Tx, phone, code string) (*gen.Error, error) {
	var (
		codeHash     string
		attemptsLeft int
	)
	err := tx.QueryRow(ctx, `
		SELECT code_hash, attempts_left
		FROM invites
		WHERE phone = $1
		FOR UPDATE`, phone).Scan(&codeHash, &attemptsLeft)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return &errInvalidCode, nil
	case err != nil:
		return nil, err
	}

	switch {
	case attemptsLeft <= 0:
		return &errTooManyAttempts, nil
	case auth.Hash(code) != codeHash:
		if _, err := tx.Exec(ctx, `
			UPDATE invites SET attempts_left = attempts_left - 1
			WHERE phone = $1`, phone); err != nil {
			return nil, err
		}
		return &errInvalidCode, nil
	}

	if _, err := tx.Exec(ctx, `DELETE FROM invites WHERE phone = $1`, phone); err != nil {
		return nil, err
	}
	return nil, nil
}

var errNotInvited = gen.Error{
	Code:    "not_invited",
	Message: "Для входа нужен код приглашения. Попросите его у владельца МоейДачи",
}
