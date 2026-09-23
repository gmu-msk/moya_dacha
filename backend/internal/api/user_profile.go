// Профиль пользователя для соседей: specs/009-user-profile.md.
package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// GetUser отдаёт профиль любого пользователя любому вошедшему: кто это
// и сколько у него постов. Номера телефона здесь нет — ни в чужом
// профиле, ни в своём (CONTEXT.md).
func (s *Server) GetUser(ctx context.Context, request gen.GetUserRequestObject) (gen.GetUserResponseObject, error) {
	if _, ok := sessionFrom(ctx); !ok {
		return gen.GetUser401JSONResponse(errUnauthorized), nil
	}
	// Идентификатор не UUID — для клиента то же, что пользователя нет.
	if !isUUID(request.UserId) {
		return gen.GetUser404JSONResponse(errUserNotFound), nil
	}

	var (
		user      gen.UserProfile
		avatarKey *string
	)
	err := s.db.QueryRow(ctx, `
		SELECT u.id, u.nickname, u.name, u.about, u.avatar_key, u.created_at,
			(SELECT count(*) FROM posts p WHERE p.author_id = u.id)
		FROM users u WHERE u.id = $1`, request.UserId,
	).Scan(&user.Id, &user.Nickname, &user.Name, &user.About, &avatarKey, &user.CreatedAt, &user.Posts)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.GetUser404JSONResponse(errUserNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if avatarKey != nil {
		url := s.cfg.Media.URL(*avatarKey)
		user.AvatarUrl = &url
	}
	return gen.GetUser200JSONResponse(user), nil
}

// GetUserPosts отдаёт посты одного человека страницами — ту же ленту,
// суженную до автора, с тем же курсором (specs/009-user-profile.md,
// требования 4 и 5).
func (s *Server) GetUserPosts(ctx context.Context, request gen.GetUserPostsRequestObject) (gen.GetUserPostsResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetUserPosts401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.UserId) {
		return gen.GetUserPosts404JSONResponse(errUserNotFound), nil
	}

	limit := DefaultFeedLimit
	if request.Params.Limit != nil {
		limit = int(*request.Params.Limit)
	}
	if limit < 1 || limit > MaxFeedLimit {
		return gen.GetUserPosts400JSONResponse(errInvalidLimit), nil
	}

	var after *feedCursor
	if request.Params.Cursor != nil && *request.Params.Cursor != "" {
		parsed, err := parseFeedCursor(*request.Params.Cursor)
		if err != nil {
			return gen.GetUserPosts400JSONResponse(errInvalidCursor), nil
		}
		after = &parsed
	}

	// Пустая страница не отличает «постов нет» от «человека нет», а
	// клиенту это разные экраны: сначала проверяем, что человек есть.
	var exists bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, request.UserId,
	).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return gen.GetUserPosts404JSONResponse(errUserNotFound), nil
	}

	page, err := s.feedPage(ctx, current.user.Id, request.UserId, after, limit)
	if err != nil {
		return nil, err
	}
	return gen.GetUserPosts200JSONResponse(page), nil
}

// errUserNotFound — пользователя нет (specs/009-user-profile.md, «Ошибки»).
var errUserNotFound = gen.Error{
	Code:    "user_not_found",
	Message: "Такого пользователя нет",
}
