package push

import (
	"context"
	"fmt"
	"strconv"
)

// Виды пушей: четыре события раздела (specs/014-notifications.md)
// и заявка на подписку.
const (
	KindFollow         = "follow"
	KindFollowAccepted = "follow_accepted"
	KindLike           = "like"
	KindComment        = "comment"
	KindFollowRequest  = "follow_request"
)

// item — созревшая строка очереди со всем, что нужно для текста.
type item struct {
	id       string
	userID   string
	kind     string
	actorID  string
	nickname string
	postID   *string
	comment  *string
	// others — непрочитанные отметки того же поста, кроме этой.
	others int64
	// cancelled — заявку уже отозвали, приняли или отклонили: пуш не нужен.
	cancelled bool
}

// due — строки, пролежавшие Delay, старые первыми. Отменённое событие
// раздела ушло из очереди каскадом, а заявку проверяем здесь
// (требование 9).
func (s *Sender) due(ctx context.Context) ([]item, error) {
	rows, err := s.db.Query(ctx, `
		SELECT q.id, q.user_id,
			coalesce(n.kind, 'follow_request'),
			a.id, a.nickname,
			n.post_id,
			left(c.text, `+strconv.Itoa(MaxComment)+`),
			CASE WHEN n.kind = 'like' THEN (
				SELECT count(*) FROM notifications o
				WHERE o.user_id = q.user_id AND o.kind = 'like' AND o.post_id = n.post_id
					AND o.id <> n.id AND o.created_at > me.notifications_seen_at
			) ELSE 0 END,
			q.requester_id IS NOT NULL AND NOT EXISTS (
				SELECT 1 FROM follows f
				WHERE f.follower_id = q.requester_id AND f.followee_id = q.user_id AND NOT f.accepted
			)
		FROM push_queue q
		JOIN users me ON me.id = q.user_id
		LEFT JOIN notifications n ON n.id = q.notification_id
		LEFT JOIN comments c ON c.id = n.comment_id
		JOIN users a ON a.id = coalesce(n.actor_id, q.requester_id)
		WHERE q.created_at <= now() - make_interval(secs => $1)
		ORDER BY q.created_at
		LIMIT 100`, s.cfg.Delay.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.userID, &it.kind, &it.actorID, &it.nickname,
			&it.postID, &it.comment, &it.others, &it.cancelled); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// body — текст пуша, как строка раздела без ника (требование 11): ник
// идёт заголовком.
func (it item) body() string {
	switch it.kind {
	case KindFollow:
		return "подписался на вас"
	case KindFollowAccepted:
		return "принял вашу заявку"
	case KindLike:
		if it.others > 0 {
			return fmt.Sprintf("и ещё %d отметили ваш пост", it.others)
		}
		return "отметил ваш пост"
	case KindComment:
		text := ""
		if it.comment != nil {
			text = *it.comment
		}
		return "ответил на ваш пост: «" + text + "»"
	case KindFollowRequest:
		return "хочет подписаться на вас"
	}
	return ""
}

// message — тело запроса FCM для одного телефона (раздел «Что уходит
// в FCM» спецификации).
func (it item) message(token string) fcmMessage {
	data := map[string]string{"kind": it.kind, "user_id": it.actorID}
	if it.postID != nil {
		data["post_id"] = *it.postID
	}
	m := fcmMessage{
		Token:        token,
		Notification: &fcmNotification{Title: it.nickname, Body: it.body()},
		Data:         data,
		Android:      &fcmAndroid{Priority: "high"},
	}
	// Отметки одного поста сменяют друг друга в шторке (требование 12).
	if it.kind == KindLike && it.postID != nil {
		m.Android.Notification = &fcmAndroidNotification{Tag: "like:" + *it.postID}
	}
	return m
}
