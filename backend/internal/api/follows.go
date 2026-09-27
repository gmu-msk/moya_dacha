// Подписки и закрытые профили: specs/012-follows.md.
package api

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// relationColumns — отношение смотрящего ($2) к пользователю u: две
// колонки, «я → он» строкой Relation.following и «он → я» булевым.
// Одна и та же пара колонок нужна в профиле и в каждой строке списков.
const relationColumns = `
	COALESCE((
		SELECT CASE WHEN f.accepted THEN 'yes' ELSE 'requested' END
		FROM follows f WHERE f.follower_id = $2::uuid AND f.followee_id = u.id
	), 'none'),
	EXISTS (
		SELECT 1 FROM follows f
		WHERE f.follower_id = u.id AND f.followee_id = $2::uuid AND f.accepted
	)`

// FollowUser подписывает на открытый профиль сразу, а к закрытому подаёт
// заявку (требование 2). Уже подписанному или подавшему заявку ничего
// не меняет (требование 3).
func (s *Server) FollowUser(ctx context.Context, request gen.FollowUserRequestObject) (gen.FollowUserResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.FollowUser401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.UserId) {
		return gen.FollowUser404JSONResponse(errUserNotFound), nil
	}
	if request.UserId == current.user.Id {
		return gen.FollowUser400JSONResponse(errCannotFollowSelf), nil
	}

	// Того, кто заблокировал смотрящего, для него нет, а на того, кого
	// заблокировал он сам, сначала надо разблокировать
	// (specs/022-edit-block-delete.md, требования 14 и 15).
	var theyBlocked, iBlocked bool
	if err := s.db.QueryRow(ctx,
		`SELECT `+blocks("$2::uuid", "$1::uuid")+`, `+blocks("$1::uuid", "$2::uuid"),
		current.user.Id, request.UserId,
	).Scan(&theyBlocked, &iBlocked); err != nil {
		return nil, err
	}
	if theyBlocked {
		return gen.FollowUser404JSONResponse(errUserNotFound), nil
	}
	if iBlocked {
		return gen.FollowUser409JSONResponse(errUserBlocked), nil
	}

	// Закрыт ли профиль, решает база в той же вставке: между проверкой
	// и записью хозяин мог бы профиль открыть или закрыть.
	tag, err := s.db.Exec(ctx, `
		INSERT INTO follows (follower_id, followee_id, accepted)
		SELECT $1, u.id, NOT u.closed FROM users u WHERE u.id = $2
		ON CONFLICT (follower_id, followee_id) DO NOTHING`,
		current.user.Id, request.UserId)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		// Ничего не вставлено: связь уже была — или человека нет.
		relation, found, err := s.relation(ctx, current.user.Id, request.UserId)
		if err != nil {
			return nil, err
		}
		if !found {
			return gen.FollowUser404JSONResponse(errUserNotFound), nil
		}
		return gen.FollowUser200JSONResponse(relation), nil
	}

	relation, _, err := s.relation(ctx, current.user.Id, request.UserId)
	if err != nil {
		return nil, err
	}
	return gen.FollowUser200JSONResponse(relation), nil
}

// UnfollowUser отписывает или отменяет заявку — одна строка на пару,
// её и удаляем (требование 3). Отписка без подписки — не ошибка.
func (s *Server) UnfollowUser(ctx context.Context, request gen.UnfollowUserRequestObject) (gen.UnfollowUserResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.UnfollowUser401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.UserId) {
		return gen.UnfollowUser404JSONResponse(errUserNotFound), nil
	}

	if _, err := s.db.Exec(ctx,
		`DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`,
		current.user.Id, request.UserId,
	); err != nil {
		return nil, err
	}

	relation, found, err := s.relation(ctx, current.user.Id, request.UserId)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.UnfollowUser404JSONResponse(errUserNotFound), nil
	}
	return gen.UnfollowUser200JSONResponse(relation), nil
}

// relation читает отношение смотрящего к пользователю. found — есть ли
// такой пользователь вообще.
func (s *Server) relation(ctx context.Context, viewerID, userID string) (gen.Relation, bool, error) {
	var (
		following string
		relation  gen.Relation
	)
	err := s.db.QueryRow(ctx, `SELECT `+relationColumns+` FROM users u
		WHERE u.id = $1 AND NOT `+blocks("u.id", "$2::uuid"),
		userID, viewerID,
	).Scan(&following, &relation.FollowedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Relation{}, false, nil
	}
	if err != nil {
		return gen.Relation{}, false, err
	}
	relation.Following = gen.RelationFollowing(following)
	return relation, true, nil
}

// GetFollowers отдаёт подписчиков страницами, новые связи сверху
// (требование 14).
func (s *Server) GetFollowers(ctx context.Context, request gen.GetFollowersRequestObject) (gen.GetFollowersResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetFollowers401JSONResponse(errUnauthorized), nil
	}
	after, limit, bad := pageParams(request.Params.Limit, request.Params.Cursor)
	if bad != nil {
		return gen.GetFollowers400JSONResponse(*bad), nil
	}
	list, access, err := s.followList(ctx, current.user.Id, request.UserId, true, after, limit)
	if err != nil {
		return nil, err
	}
	switch access {
	case profileMissing:
		return gen.GetFollowers404JSONResponse(errUserNotFound), nil
	case profileClosed:
		return gen.GetFollowers403JSONResponse(errProfileClosed), nil
	}
	return gen.GetFollowers200JSONResponse(list), nil
}

// GetFollowing отдаёт подписки страницами, новые связи сверху
// (требование 14).
func (s *Server) GetFollowing(ctx context.Context, request gen.GetFollowingRequestObject) (gen.GetFollowingResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetFollowing401JSONResponse(errUnauthorized), nil
	}
	after, limit, bad := pageParams(request.Params.Limit, request.Params.Cursor)
	if bad != nil {
		return gen.GetFollowing400JSONResponse(*bad), nil
	}
	list, access, err := s.followList(ctx, current.user.Id, request.UserId, false, after, limit)
	if err != nil {
		return nil, err
	}
	switch access {
	case profileMissing:
		return gen.GetFollowing404JSONResponse(errUserNotFound), nil
	case profileClosed:
		return gen.GetFollowing403JSONResponse(errProfileClosed), nil
	}
	return gen.GetFollowing200JSONResponse(list), nil
}

// followList читает страницу подписчиков (followers) или подписок
// человека. Курсор — время связи и идентификатор человека в строке,
// как у ленты (docs/adr/0015-feed-paginated-by-cursor.md).
func (s *Server) followList(ctx context.Context, viewerID, userID string, followers bool, after *feedCursor, limit int) (gen.FollowList, profileAccess, error) {
	access, err := s.profileAccess(ctx, viewerID, userID)
	if err != nil || access != profileOpen {
		return gen.FollowList{}, access, err
	}

	// Человек в строке — подписчик для списка подписчиков и тот, на кого
	// подписан, для списка подписок.
	person, owner := "f.follower_id", "f.followee_id"
	if !followers {
		person, owner = owner, person
	}

	var (
		afterTime *time.Time
		afterID   *string
	)
	if after != nil {
		afterTime, afterID = &after.createdAt, &after.id
	}

	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.nickname, u.name, u.avatar_key, f.created_at, `+relationColumns+`
		FROM follows f JOIN users u ON u.id = `+person+`
		WHERE `+owner+` = $1 AND f.accepted
		  AND NOT `+blockedBetween("$2::uuid", "u.id")+`
		  AND ($3::timestamptz IS NULL OR (f.created_at, u.id) < ($3::timestamptz, $4::uuid))
		ORDER BY f.created_at DESC, u.id DESC
		LIMIT $5`, userID, viewerID, afterTime, afterID, limit+1)
	if err != nil {
		return gen.FollowList{}, access, err
	}
	defer rows.Close()

	list := gen.FollowList{Items: []gen.FollowUser{}}
	var times []time.Time
	for rows.Next() {
		var (
			item      gen.FollowUser
			avatarKey *string
			since     time.Time
			following string
			relation  gen.Relation
		)
		if err := rows.Scan(&item.Id, &item.Nickname, &item.Name, &avatarKey, &since,
			&following, &relation.FollowedBy); err != nil {
			return gen.FollowList{}, access, err
		}
		if avatarKey != nil {
			url := s.cfg.Media.URL(*avatarKey)
			item.AvatarUrl = &url
		}
		// У самого смотрящего кнопки нет (требование 14).
		if item.Id != viewerID {
			relation.Following = gen.RelationFollowing(following)
			item.Relation = &relation
		}
		list.Items = append(list.Items, item)
		times = append(times, since)
	}
	if err := rows.Err(); err != nil {
		return gen.FollowList{}, access, err
	}

	if len(list.Items) > limit {
		list.Items = list.Items[:limit]
		cursor := feedCursor{createdAt: times[limit-1], id: list.Items[limit-1].Id}.String()
		list.NextCursor = &cursor
	}
	return list, access, nil
}

// profileAccess — открыт ли смотрящему закрытый профиль целиком.
type profileAccess int

const (
	profileOpen    profileAccess = iota // профиль открыт, свой или смотрящий подписан
	profileClosed                       // закрыт, а смотрящий не подписан
	profileMissing                      // пользователя нет
)

// profileAccess решает, видны ли смотрящему посты и списки человека
// (требование 7). Заявка — не подписка.
func (s *Server) profileAccess(ctx context.Context, viewerID, userID string) (profileAccess, error) {
	if !isUUID(userID) {
		return profileMissing, nil
	}
	var open bool
	err := s.db.QueryRow(ctx, `
		SELECT NOT u.closed OR u.id = $2 OR EXISTS (
			SELECT 1 FROM follows f
			WHERE f.follower_id = $2 AND f.followee_id = u.id AND f.accepted
		)
		FROM users u WHERE u.id = $1 AND NOT `+blocks("u.id", "$2::uuid"), userID, viewerID,
	).Scan(&open)
	if errors.Is(err, pgx.ErrNoRows) {
		return profileMissing, nil
	}
	if err != nil {
		return profileMissing, err
	}
	if !open {
		return profileClosed, nil
	}
	return profileOpen, nil
}

// SetPrivacy закрывает или открывает свой профиль. Закрытие подписчиков
// не трогает (требование 8), открытие принимает все ждущие заявки
// (требование 9) — в одной транзакции, чтобы между ними не проскочила
// новая заявка к уже открытому профилю.
func (s *Server) SetPrivacy(ctx context.Context, request gen.SetPrivacyRequestObject) (gen.SetPrivacyResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.SetPrivacy401JSONResponse(errUnauthorized), nil
	}
	if request.Body == nil || request.Body.Closed == nil {
		return gen.SetPrivacy400JSONResponse(errInvalidPrivacy), nil
	}
	closed := *request.Body.Closed

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // после Commit откат ничего не делает

	user, err := s.scanUser(tx.QueryRow(ctx, `
		UPDATE users SET closed = $2 WHERE id = $1
		RETURNING `+userColumns, current.user.Id, closed))
	if err != nil {
		return nil, err
	}
	if !closed {
		// Подписка считается с момента принятия: время связи — сейчас.
		if _, err := tx.Exec(ctx, `
			UPDATE follows SET accepted = true, created_at = now()
			WHERE followee_id = $1 AND NOT accepted`, current.user.Id,
		); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return gen.SetPrivacy200JSONResponse(user), nil
}

// GetFollowRequests отдаёт ждущие заявки ко мне, новые сверху.
func (s *Server) GetFollowRequests(ctx context.Context, request gen.GetFollowRequestsRequestObject) (gen.GetFollowRequestsResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetFollowRequests401JSONResponse(errUnauthorized), nil
	}
	after, limit, bad := pageParams(request.Params.Limit, request.Params.Cursor)
	if bad != nil {
		return gen.GetFollowRequests400JSONResponse(*bad), nil
	}

	var (
		afterTime *time.Time
		afterID   *string
	)
	if after != nil {
		afterTime, afterID = &after.createdAt, &after.id
	}

	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.nickname, u.name, u.avatar_key, f.created_at
		FROM follows f JOIN users u ON u.id = f.follower_id
		WHERE f.followee_id = $1 AND NOT f.accepted
		  AND NOT `+blockedBetween("$1::uuid", "u.id")+`
		  AND ($2::timestamptz IS NULL OR (f.created_at, u.id) < ($2::timestamptz, $3::uuid))
		ORDER BY f.created_at DESC, u.id DESC
		LIMIT $4`, current.user.Id, afterTime, afterID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := gen.AuthorList{Items: []gen.Author{}}
	var times []time.Time
	for rows.Next() {
		var (
			item      gen.Author
			avatarKey *string
			since     time.Time
		)
		if err := rows.Scan(&item.Id, &item.Nickname, &item.Name, &avatarKey, &since); err != nil {
			return nil, err
		}
		if avatarKey != nil {
			url := s.cfg.Media.URL(*avatarKey)
			item.AvatarUrl = &url
		}
		list.Items = append(list.Items, item)
		times = append(times, since)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(list.Items) > limit {
		list.Items = list.Items[:limit]
		cursor := feedCursor{createdAt: times[limit-1], id: list.Items[limit-1].Id}.String()
		list.NextCursor = &cursor
	}
	return gen.GetFollowRequests200JSONResponse(list), nil
}

// AcceptFollowRequest делает заявку подпиской той же строкой.
func (s *Server) AcceptFollowRequest(ctx context.Context, request gen.AcceptFollowRequestRequestObject) (gen.AcceptFollowRequestResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.AcceptFollowRequest401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.UserId) {
		return gen.AcceptFollowRequest404JSONResponse(errRequestNotFound), nil
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE follows SET accepted = true, created_at = now()
		WHERE follower_id = $1 AND followee_id = $2 AND NOT accepted`,
		request.UserId, current.user.Id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return gen.AcceptFollowRequest404JSONResponse(errRequestNotFound), nil
	}
	return gen.AcceptFollowRequest204Response{}, nil
}

// DeclineFollowRequest удаляет заявку молча: заявитель может подать
// новую (требование 10).
func (s *Server) DeclineFollowRequest(ctx context.Context, request gen.DeclineFollowRequestRequestObject) (gen.DeclineFollowRequestResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.DeclineFollowRequest401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.UserId) {
		return gen.DeclineFollowRequest404JSONResponse(errRequestNotFound), nil
	}
	tag, err := s.db.Exec(ctx, `
		DELETE FROM follows
		WHERE follower_id = $1 AND followee_id = $2 AND NOT accepted`,
		request.UserId, current.user.Id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return gen.DeclineFollowRequest404JSONResponse(errRequestNotFound), nil
	}
	return gen.DeclineFollowRequest204Response{}, nil
}

// pageParams разбирает размер страницы и курсор списка — так же, как
// у ленты. bad — готовая ошибка для ответа 400.
func pageParams[L ~int32](rawLimit *L, rawCursor *string) (*feedCursor, int, *gen.Error) {
	limit := DefaultFeedLimit
	if rawLimit != nil {
		limit = int(*rawLimit)
	}
	if limit < 1 || limit > MaxFeedLimit {
		return nil, 0, &errInvalidLimit
	}
	if rawCursor == nil || *rawCursor == "" {
		return nil, limit, nil
	}
	parsed, err := parseFeedCursor(*rawCursor)
	if err != nil {
		return nil, 0, &errInvalidCursor
	}
	return &parsed, limit, nil
}

// Ошибки подписок (specs/012-follows.md, «Ошибки»).
var (
	errCannotFollowSelf = gen.Error{
		Code:    "cannot_follow_self",
		Message: "На себя подписаться нельзя",
	}
	errProfileClosed = gen.Error{
		Code:    "profile_closed",
		Message: "Это закрытый профиль: его видят только подписчики",
	}
	errRequestNotFound = gen.Error{
		Code:    "request_not_found",
		Message: "Такой заявки нет",
	}
	errInvalidPrivacy = gen.Error{
		Code:    "invalid_request",
		Message: "Не сказано, закрыть профиль или открыть",
	}
)
