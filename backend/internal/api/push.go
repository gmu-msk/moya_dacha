// Токен телефона для пушей: specs/024-push.md.
//
// Сами пуши шлёт пакет push из очереди, которую наполняют триггеры базы
// (backend/migrations/00019_push.sql); здесь телефон только называет себя.
package api

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// MaxPushToken — длиннее токенов FCM не бывает с запасом.
const MaxPushToken = 4096

// SetPushToken запоминает токен FCM за сессией (требование 7): у сессии
// один токен, а тот же токен с другой сессии переезжает к этой.
func (s *Server) SetPushToken(ctx context.Context, request gen.SetPushTokenRequestObject) (gen.SetPushTokenResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.SetPushToken401JSONResponse(errUnauthorized), nil
	}
	if request.Body == nil {
		return gen.SetPushToken400JSONResponse(errInvalidPushToken), nil
	}
	token := strings.TrimSpace(request.Body.Token)
	if token == "" || utf8.RuneCountInString(token) > MaxPushToken {
		return gen.SetPushToken400JSONResponse(errInvalidPushToken), nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx,
		`DELETE FROM push_devices WHERE session_hash = $1 AND token <> $2`,
		current.tokenHash, token,
	); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO push_devices (token, session_hash, user_id) VALUES ($1, $2, $3)
		ON CONFLICT (token) DO UPDATE
		SET session_hash = EXCLUDED.session_hash, user_id = EXCLUDED.user_id, updated_at = now()`,
		token, current.tokenHash, current.user.Id,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return gen.SetPushToken204Response{}, nil
}

var errInvalidPushToken = gen.Error{
	Code:    "invalid_request",
	Message: "Токен уведомлений не подходит",
}
