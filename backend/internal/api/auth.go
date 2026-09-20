// Вход по номеру телефона: specs/001-auth.md.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
	"github.com/gmu-msk/moya_dacha/backend/internal/auth"
)

// RequestAuthCode выдаёт код подтверждения на номер телефона.
//
// Ответ одинаков для нового и для существующего номера: по нему нельзя
// узнать, зарегистрирован ли человек.
func (s *Server) RequestAuthCode(ctx context.Context, request gen.RequestAuthCodeRequestObject) (gen.RequestAuthCodeResponseObject, error) {
	if request.Body == nil || strings.TrimSpace(request.Body.Phone) == "" {
		return gen.RequestAuthCode400JSONResponse(errEmptyRequest), nil
	}

	phone, err := auth.NormalizePhone(request.Body.Phone)
	if err != nil {
		return gen.RequestAuthCode400JSONResponse(errInvalidPhone), nil
	}

	code := s.cfg.FixedCode
	if code == "" {
		code, err = auth.GenerateCode()
		if err != nil {
			return nil, err
		}
	}

	// Выдача кода и проверка «не чаще раза в минуту» — один запрос:
	// два параллельных запроса кода на один номер не должны разъехаться.
	// Если условие не выполнено, строка не обновляется и ничего не вернётся.
	const query = `
		INSERT INTO auth_codes (phone, code_hash, attempts_left, expires_at, created_at)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4::double precision), now())
		ON CONFLICT (phone) DO UPDATE
		SET code_hash     = excluded.code_hash,
		    attempts_left = excluded.attempts_left,
		    expires_at    = excluded.expires_at,
		    created_at    = excluded.created_at
		WHERE auth_codes.created_at <= now() - make_interval(secs => $5::double precision)
		RETURNING phone`

	var stored string
	err = s.db.QueryRow(ctx, query,
		phone, auth.Hash(code), auth.MaxAttempts,
		s.cfg.CodeTTL.Seconds(), s.cfg.ResendAfter.Seconds(),
	).Scan(&stored)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return gen.RequestAuthCode429JSONResponse(errTooManyRequests), nil
	case err != nil:
		return nil, err
	}

	if err := s.cfg.CodeSender.Send(ctx, phone, code); err != nil {
		// Код уже выдан: если доставка не удалась, честнее сказать об этом
		// сразу, чем оставить человека ждать несуществующую SMS.
		slog.Error("не удалось доставить код подтверждения", "phone", phone, "err", err)
		return nil, err
	}

	return gen.RequestAuthCode202JSONResponse{
		ResendAfter: int32(s.cfg.ResendAfter.Seconds()),
		CodeTtl:     int32(s.cfg.CodeTTL.Seconds()),
	}, nil
}

// CreateSession обменивает код подтверждения на токен сессии. Если номера
// ещё нет, пользователь заводится здесь же: регистрация и вход — одно
// действие.
func (s *Server) CreateSession(ctx context.Context, request gen.CreateSessionRequestObject) (gen.CreateSessionResponseObject, error) {
	if request.Body == nil ||
		strings.TrimSpace(request.Body.Phone) == "" ||
		strings.TrimSpace(request.Body.Code) == "" {
		return gen.CreateSession400JSONResponse(errEmptyRequest), nil
	}

	phone, err := auth.NormalizePhone(request.Body.Phone)
	if err != nil {
		return gen.CreateSession400JSONResponse(errInvalidPhone), nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		codeHash     string
		attemptsLeft int
		expired      bool
	)
	err = tx.QueryRow(ctx, `
		SELECT code_hash, attempts_left, expires_at <= now()
		FROM auth_codes
		WHERE phone = $1
		FOR UPDATE`, phone).Scan(&codeHash, &attemptsLeft, &expired)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Кода по этому номеру не запрашивали. Говорить об этом отдельно
		// незачем: ответ тот же, что и на неверный код.
		return gen.CreateSession401JSONResponse(errInvalidCode), nil
	case err != nil:
		return nil, err
	}

	// Порядок проверок зафиксирован спекой: срок, попытки, совпадение.
	switch {
	case expired:
		return gen.CreateSession401JSONResponse(errCodeExpired), nil
	case attemptsLeft <= 0:
		return gen.CreateSession401JSONResponse(errTooManyAttempts), nil
	case auth.Hash(request.Body.Code) != codeHash:
		if _, err := tx.Exec(ctx, `
			UPDATE auth_codes SET attempts_left = attempts_left - 1
			WHERE phone = $1`, phone); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return gen.CreateSession401JSONResponse(errInvalidCode), nil
	}

	// Код подошёл и на этом сгорает: второй раз им не войти.
	if _, err := tx.Exec(ctx, `DELETE FROM auth_codes WHERE phone = $1`, phone); err != nil {
		return nil, err
	}

	user, isNew, err := upsertUser(ctx, tx, phone)
	if err != nil {
		return nil, err
	}

	token, err := auth.GenerateToken()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id) VALUES ($1, $2)`,
		auth.Hash(token), user.Id,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return gen.CreateSession200JSONResponse{
		Token:     token,
		IsNewUser: isNew,
		User:      user,
	}, nil
}

// GetSession отвечает, чья это сессия.
func (s *Server) GetSession(ctx context.Context, _ gen.GetSessionRequestObject) (gen.GetSessionResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetSession401JSONResponse(errUnauthorized), nil
	}
	return gen.GetSession200JSONResponse{User: current.user}, nil
}

// DeleteSession прекращает действие этого токена. Сессии на других
// устройствах остаются живыми.
func (s *Server) DeleteSession(ctx context.Context, _ gen.DeleteSessionRequestObject) (gen.DeleteSessionResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.DeleteSession401JSONResponse(errUnauthorized), nil
	}

	if _, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, current.tokenHash); err != nil {
		return nil, err
	}
	return gen.DeleteSession204Response{}, nil
}

// upsertUser возвращает пользователя с этим номером, заводя его, если
// номер появился впервые.
func upsertUser(ctx context.Context, tx pgx.Tx, phone string) (gen.CurrentUser, bool, error) {
	var user gen.CurrentUser

	err := tx.QueryRow(ctx, `
		INSERT INTO users (phone) VALUES ($1)
		ON CONFLICT (phone) DO NOTHING
		RETURNING id, phone, created_at`, phone,
	).Scan(&user.Id, &user.Phone, &user.CreatedAt)
	if err == nil {
		return user, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return gen.CurrentUser{}, false, err
	}

	err = tx.QueryRow(ctx, `
		SELECT id, phone, created_at FROM users WHERE phone = $1`, phone,
	).Scan(&user.Id, &user.Phone, &user.CreatedAt)
	if err != nil {
		return gen.CurrentUser{}, false, err
	}
	return user, false, nil
}

// session — то, что сервис знает о том, кто пришёл с токеном.
type session struct {
	tokenHash string
	user      gen.CurrentUser
}

type contextKey struct{}

// withSession опознаёт пришедшего по заголовку Authorization и кладёт
// его в контекст. Проверять, нужна ли операции авторизация, не её дело:
// хендлеры, которым нужен пользователь, спрашивают его сами и сами
// отвечают 401 — так список защищённых операций не разъезжается
// с контрактом.
func (s *Server) withSession(next gen.StrictHandlerFunc, _ string) gen.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			return next(ctx, w, r, request)
		}

		var current session
		current.tokenHash = auth.Hash(token)
		err := s.db.QueryRow(ctx, `
			SELECT u.id, u.phone, u.created_at
			FROM sessions s JOIN users u ON u.id = s.user_id
			WHERE s.token_hash = $1`, current.tokenHash,
		).Scan(&current.user.Id, &current.user.Phone, &current.user.CreatedAt)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Токен недействителен — пришедший остаётся неопознанным.
			return next(ctx, w, r, request)
		case err != nil:
			return nil, err
		}

		return next(context.WithValue(ctx, contextKey{}, current), w, r, request)
	}
}

func sessionFrom(ctx context.Context) (session, bool) {
	current, ok := ctx.Value(contextKey{}).(session)
	return current, ok
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

// Ошибки контракта (specs/001-auth.md, «Ошибки»). Сообщения показываются
// пользователю, поэтому написаны для него, а не для лога.
var (
	errEmptyRequest = gen.Error{
		Code:    "invalid_request",
		Message: "Заполните номер телефона и код",
	}
	errInvalidPhone = gen.Error{
		Code:    "invalid_phone",
		Message: "Это не похоже на номер мобильного телефона",
	}
	errTooManyRequests = gen.Error{
		Code:    "too_many_requests",
		Message: "Код уже отправлен. Подождите минуту и попробуйте снова",
	}
	errInvalidCode = gen.Error{
		Code:    "invalid_code",
		Message: "Код не подошёл",
	}
	errCodeExpired = gen.Error{
		Code:    "code_expired",
		Message: "Код устарел. Запросите новый",
	}
	errTooManyAttempts = gen.Error{
		Code:    "too_many_attempts",
		Message: "Слишком много попыток. Запросите новый код",
	}
	errUnauthorized = gen.Error{
		Code:    "unauthorized",
		Message: "Нужно войти заново",
	}
)
