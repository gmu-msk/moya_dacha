// Уведомления: specs/014-notifications.md.
//
// События пишут триггеры базы (backend/migrations/00011_notifications.sql):
// здесь их только читают, сливают лайки и отмечают прочитанным.
package api

import (
	"context"
	"strconv"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// NotificationsKeptFor — сколько хранятся события (требование 8).
const NotificationsKeptFor = 90 * 24 * time.Hour

// MaxNotificationComment — сколько символов комментария попадает в строку.
const MaxNotificationComment = 100

// notificationRows — строки раздела смотрящего ($1): каждое событие само
// по себе, а лайки одного поста — одной строкой с последним отметившим
// (требование 1). unread — новее последнего открытия раздела.
const notificationRows = `
	WITH grouped AS (
		SELECT n.*,
			row_number() OVER w AS rank,
			count(*) OVER (PARTITION BY n.kind, CASE WHEN n.kind = 'like' THEN n.post_id::text ELSE n.id::text END) - 1 AS others
		FROM notifications n
		WHERE n.user_id = $1
		WINDOW w AS (
			PARTITION BY n.kind, CASE WHEN n.kind = 'like' THEN n.post_id::text ELSE n.id::text END
			ORDER BY n.created_at DESC, n.id DESC
		)
	)
	SELECT g.id, g.kind, g.created_at, g.others, g.post_id, g.comment_id,
		g.created_at > (SELECT me.notifications_seen_at FROM users me WHERE me.id = $1) AS unread,
		g.actor_id
	FROM grouped g
	WHERE g.rank = 1`

// GetNotifications отдаёт строки раздела страницами, новые сверху.
// Заодно убирает события старше 90 дней: фонового уборщика нет.
func (s *Server) GetNotifications(ctx context.Context, request gen.GetNotificationsRequestObject) (gen.GetNotificationsResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetNotifications401JSONResponse(errUnauthorized), nil
	}
	after, limit, bad := pageParams(request.Params.Limit, request.Params.Cursor)
	if bad != nil {
		return gen.GetNotifications400JSONResponse(*bad), nil
	}

	if _, err := s.db.Exec(ctx,
		`DELETE FROM notifications WHERE user_id = $1 AND created_at < $2`,
		current.user.Id, time.Now().Add(-NotificationsKeptFor),
	); err != nil {
		return nil, err
	}

	var (
		afterTime *time.Time
		afterID   *string
	)
	if after != nil {
		afterTime, afterID = &after.createdAt, &after.id
	}

	// Человек, пост и комментарий строки читаются тем же запросом:
	// отношение — как в списках подписчиков (relationColumns ждёт
	// смотрящего вторым параметром), миниатюра — первое медиа поста.
	rows, err := s.db.Query(ctx, `
		SELECT r.id, r.kind, r.created_at, r.others, r.unread,
			u.id, u.nickname, u.name, u.avatar_key, `+relationColumns+`,
			r.post_id, m.id, m.kind, m.storage_key, m.width, m.height,
			left(c.text, `+strconv.Itoa(MaxNotificationComment)+`)
		FROM (`+notificationRows+`) r
		JOIN users u ON u.id = r.actor_id
		LEFT JOIN LATERAL (
			SELECT * FROM media WHERE post_id = r.post_id ORDER BY position LIMIT 1
		) m ON true
		LEFT JOIN comments c ON c.id = r.comment_id
		WHERE $3::timestamptz IS NULL OR (r.created_at, r.id) < ($3::timestamptz, $4::uuid)
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $5`, current.user.Id, current.user.Id, afterTime, afterID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := gen.NotificationList{Items: []gen.Notification{}}
	for rows.Next() {
		var (
			item       gen.Notification
			kind       string
			others     int32
			avatarKey  *string
			following  string
			relation   gen.Relation
			postID     *string
			mediaID    *string
			mediaKind  *string
			mediaKey   *string
			width      *int32
			height     *int32
			commentTxt *string
		)
		if err := rows.Scan(&item.Id, &kind, &item.CreatedAt, &others, &item.Unread,
			&item.Actor.Id, &item.Actor.Nickname, &item.Actor.Name, &avatarKey,
			&following, &relation.FollowedBy,
			&postID, &mediaID, &mediaKind, &mediaKey, &width, &height,
			&commentTxt,
		); err != nil {
			return nil, err
		}
		item.Kind = gen.NotificationKind(kind)
		if avatarKey != nil {
			url := s.cfg.Media.URL(*avatarKey)
			item.Actor.AvatarUrl = &url
		}
		relation.Following = gen.RelationFollowing(following)
		item.Actor.Relation = &relation
		if kind == "like" {
			item.Others = &others
		}
		if postID != nil && mediaID != nil {
			item.Post = &gen.NotificationPost{
				Id: *postID,
				Thumbnail: gen.Media{
					Id:     *mediaID,
					Kind:   gen.MediaKind(*mediaKind),
					Url:    s.cfg.Media.URL(*mediaKey),
					Width:  *width,
					Height: *height,
				},
			}
		}
		item.Comment = commentTxt
		list.Items = append(list.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(list.Items) > limit {
		list.Items = list.Items[:limit]
		last := list.Items[limit-1]
		cursor := feedCursor{createdAt: last.CreatedAt, id: last.Id}.String()
		list.NextCursor = &cursor
	}
	return gen.GetNotifications200JSONResponse(list), nil
}

// GetUnreadNotifications считает непрочитанные строки и ждущие заявки:
// по ним горит точка на колокольчике (требование 5).
func (s *Server) GetUnreadNotifications(ctx context.Context, _ gen.GetUnreadNotificationsRequestObject) (gen.GetUnreadNotificationsResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetUnreadNotifications401JSONResponse(errUnauthorized), nil
	}
	var counts gen.UnreadNotifications
	if err := s.db.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM (`+notificationRows+`) r
			 WHERE r.unread AND r.created_at >= $2),
			(SELECT count(*) FROM follows f WHERE f.followee_id = $1 AND NOT f.accepted)`,
		current.user.Id, time.Now().Add(-NotificationsKeptFor),
	).Scan(&counts.Unread, &counts.Requests); err != nil {
		return nil, err
	}
	return gen.GetUnreadNotifications200JSONResponse(counts), nil
}

// MarkNotificationsSeen отмечает прочитанным всё, что есть сейчас
// (требование 7). Заявки не трогает: они непрочитанные, пока ждут.
func (s *Server) MarkNotificationsSeen(ctx context.Context, _ gen.MarkNotificationsSeenRequestObject) (gen.MarkNotificationsSeenResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.MarkNotificationsSeen401JSONResponse(errUnauthorized), nil
	}
	if _, err := s.db.Exec(ctx,
		`UPDATE users SET notifications_seen_at = now() WHERE id = $1`, current.user.Id,
	); err != nil {
		return nil, err
	}
	return gen.MarkNotificationsSeen204Response{}, nil
}
