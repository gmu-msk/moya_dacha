package push

import (
	"context"
	"fmt"
	"strconv"
)

// Виды пушей: четыре события раздела (specs/014-notifications.md),
// заявка на подписку, заявка в группу и приглашение в неё
// (specs/029-groups.md, требование 29).
const (
	KindFollow         = "follow"
	KindFollowAccepted = "follow_accepted"
	KindLike           = "like"
	KindComment        = "comment"
	KindFollowRequest  = "follow_request"
	KindGroupRequest   = "group_request"
	KindGroupInvite    = "group_invite"
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
	// groupID и group — группа заявки или приглашения.
	groupID *string
	group   *string
	// others — непрочитанные отметки того же поста, кроме этой.
	others int64
	// cancelled — заявку (на подписку или в группу) или приглашение уже
	// отозвали, приняли или отклонили: пуш не нужен.
	cancelled bool
}

// due — строки, пролежавшие Delay, старые первыми. Отменённое событие
// раздела и удалённая группа ушли из очереди каскадом, а заявку и
// приглашение проверяем здесь (требование 9).
func (s *Sender) due(ctx context.Context) ([]item, error) {
	rows, err := s.db.Query(ctx, `
		SELECT q.id, q.user_id,
			coalesce(n.kind, q.group_kind, 'follow_request'),
			a.id, a.nickname,
			n.post_id,
			left(c.text, `+strconv.Itoa(MaxComment)+`),
			CASE WHEN n.kind = 'like' THEN (
				SELECT count(*) FROM notifications o
				WHERE o.user_id = q.user_id AND o.kind = 'like' AND o.post_id = n.post_id
					AND o.id <> n.id AND o.created_at > me.notifications_seen_at
			) ELSE 0 END,
			CASE q.group_kind
				WHEN 'group_request' THEN NOT EXISTS (
					SELECT 1 FROM group_members gm
					WHERE gm.group_id = q.group_id AND gm.user_id = q.requester_id AND gm.state = 'requested')
				WHEN 'group_invite' THEN NOT EXISTS (
					SELECT 1 FROM group_members gm
					WHERE gm.group_id = q.group_id AND gm.user_id = q.user_id AND gm.state = 'invited')
				ELSE q.requester_id IS NOT NULL AND NOT EXISTS (
					SELECT 1 FROM follows f
					WHERE f.follower_id = q.requester_id AND f.followee_id = q.user_id AND NOT f.accepted)
			END,
			q.group_id, g.name
		FROM push_queue q
		LEFT JOIN groups g ON g.id = q.group_id
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
			&it.postID, &it.comment, &it.others, &it.cancelled, &it.groupID, &it.group); err != nil {
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
	case KindGroupRequest:
		return "просится в группу «" + it.groupName() + "»"
	case KindGroupInvite:
		return "приглашает вас в группу «" + it.groupName() + "»"
	}
	return ""
}

func (it item) groupName() string {
	if it.group == nil {
		return ""
	}
	return *it.group
}

// message — тело запроса FCM для одного телефона (раздел «Что уходит
// в FCM» спецификации).
func (it item) message(token string) fcmMessage {
	data := map[string]string{"kind": it.kind, "user_id": it.actorID}
	if it.postID != nil {
		data["post_id"] = *it.postID
	}
	if it.groupID != nil {
		data["group_id"] = *it.groupID
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
