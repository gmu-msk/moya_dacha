// Профиль пользователя для соседей: specs/009-user-profile.md.
package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// GetUser отдаёт профиль любого пользователя любому вошедшему: кто это,
// сколько у него постов, подписчиков и подписок и как к нему относится
// смотрящий. Номера телефона здесь нет — ни в чужом профиле, ни в своём
// (CONTEXT.md). Шапку закрытого профиля видят все (specs/012-follows.md,
// требование 7).
func (s *Server) GetUser(ctx context.Context, request gen.GetUserRequestObject) (gen.GetUserResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetUser401JSONResponse(errUnauthorized), nil
	}
	// Идентификатор не UUID — для клиента то же, что пользователя нет.
	if !isUUID(request.UserId) {
		return gen.GetUser404JSONResponse(errUserNotFound), nil
	}

	var (
		user      gen.UserProfile
		avatarKey *string
		following string
		relation  gen.Relation
		place     placeScan
	)
	// Заявки в числа не входят (требование 12). Посты — только видимые
	// смотрящему, а у закрытого профиля без подписки — те, что он увидит,
	// когда подпишется: «Подписчикам» (требование 13). При блокировке
	// постов нет вовсе, а того, кто заблокировал смотрящего, нет для него
	// самого (specs/022-edit-block-delete.md, требования 14 и 15).
	err := s.db.QueryRow(ctx, `
		SELECT u.id, u.nickname, u.name, u.about, u.avatar_key, u.created_at, u.closed,
			`+placeColumns("u")+`,
			(SELECT count(*) FROM posts p WHERE p.author_id = u.id
				AND NOT `+blockedBetween("$2::uuid", "u.id")+`
				AND (p.visibility = 'all' OR `+postVisibleTo("$2")+`)),
			(SELECT count(*) FROM follows f WHERE f.followee_id = u.id AND f.accepted),
			(SELECT count(*) FROM follows f WHERE f.follower_id = u.id AND f.accepted),
			`+blocks("$2::uuid", "u.id")+`,
			`+relationColumns+`
		FROM users u WHERE u.id = $1 AND NOT `+blocks("u.id", "$2::uuid"), request.UserId, current.user.Id,
	).Scan(&user.Id, &user.Nickname, &user.Name, &user.About, &avatarKey, &user.CreatedAt,
		&user.Closed, &place.id, &place.name, &place.area, &user.Posts, &user.Followers, &user.Following, &user.Blocked,
		&following, &relation.FollowedBy)
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
	user.Place = place.value()
	// К себе отношения нет: в своём профиле нет и кнопки.
	if user.Id != current.user.Id {
		relation.Following = gen.RelationFollowing(following)
		user.Relation = &relation
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

	// Пустая страница не отличает «постов нет» от «человека нет» и от
	// «профиль закрыт», а клиенту это разные экраны: сначала проверяем,
	// что человек есть и его посты смотрящему открыты.
	access, err := s.profileAccess(ctx, current.user.Id, request.UserId)
	if err != nil {
		return nil, err
	}
	switch access {
	case profileMissing:
		return gen.GetUserPosts404JSONResponse(errUserNotFound), nil
	case profileClosed:
		return gen.GetUserPosts403JSONResponse(errProfileClosed), nil
	}

	page, err := s.feedPage(ctx, current.user.Id, request.UserId, false, after, limit)
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
