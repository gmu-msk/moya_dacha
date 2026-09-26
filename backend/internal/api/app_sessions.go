// Заходы и время в приложении: specs/020-app-sessions.md.
package api

import (
	"context"
	"regexp"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// StartAppSession заводит сессию со временем начала по часам сервера
// (требования 3–4).
func (s *Server) StartAppSession(ctx context.Context, _ gen.StartAppSessionRequestObject) (gen.StartAppSessionResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.StartAppSession401JSONResponse(errUnauthorized), nil
	}
	var id string
	if err := s.db.QueryRow(ctx, `
		INSERT INTO app_sessions (user_id) VALUES ($1) RETURNING id::text`,
		current.user.Id,
	).Scan(&id); err != nil {
		return nil, err
	}
	return gen.StartAppSession201JSONResponse(gen.AppSessionStarted{Id: id}), nil
}

// EndAppSession ставит конец «сейчас» и заменяет экраны присланными:
// счётчик у приложения накопительный, поэтому повторная отметка ничего
// не удваивает (требования 5, 8).
func (s *Server) EndAppSession(ctx context.Context, request gen.EndAppSessionRequestObject) (gen.EndAppSessionResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.EndAppSession401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.SessionId) {
		return gen.EndAppSession404JSONResponse(errAppSessionNotFound), nil
	}
	var screens map[string]int32
	if request.Body != nil && request.Body.Screens != nil {
		screens = *request.Body.Screens
	}
	if !validScreens(screens) {
		return gen.EndAppSession400JSONResponse(errInvalidScreens), nil
	}

	found := false
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE app_sessions SET ended_at = now()
			WHERE id = $1 AND user_id = $2`,
			request.SessionId, current.user.Id)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		found = true
		if _, err := tx.Exec(ctx, `DELETE FROM app_session_screens WHERE session_id = $1`, request.SessionId); err != nil {
			return err
		}
		for screen, opens := range screens {
			if _, err := tx.Exec(ctx, `
				INSERT INTO app_session_screens (session_id, screen, opens) VALUES ($1, $2, $3)`,
				request.SessionId, screen, opens); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.EndAppSession404JSONResponse(errAppSessionNotFound), nil
	}
	return gen.EndAppSession204Response{}, nil
}

var screenName = regexp.MustCompile(`^[a-z_]{1,32}$`)

// validScreens — требование 9: имя из a-z и _, до 32 символов, число
// от 1 до 10000, экранов не больше 50.
func validScreens(screens map[string]int32) bool {
	if len(screens) > 50 {
		return false
	}
	for name, opens := range screens {
		if !screenName.MatchString(name) || opens < 1 || opens > 10000 {
			return false
		}
	}
	return true
}

var (
	errAppSessionNotFound = gen.Error{
		Code:    "not_found",
		Message: "Такой сессии нет",
	}
	errInvalidScreens = gen.Error{
		Code:    "invalid_screens",
		Message: "Экраны не разобрались: имя из a-z и _, до 32 символов, число от 1 до 10000, не больше 50 экранов",
	}
)
