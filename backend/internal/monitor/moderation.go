package monitor

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Moderation — раздел «Модерация» дашборда (specs/023-moderation.md,
// требования 3–6): на что жаловались и что появилось свежего; и
// геогруппы с участниками (specs/029-groups.md, требование 30).
type Moderation struct {
	Reported []ModerationItem `json:"reported"`
	Recent   []ModerationItem `json:"recent"`
	Groups   []PlaceGroup     `json:"groups"`
}

// PlaceGroup — геогруппа глазами владельца: убрать из неё может только
// он (specs/029-groups.md, требования 30–31).
type PlaceGroup struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Area    string         `json:"area"`
	Members []ContentOwner `json:"members"`
}

// ModerationItem — пост или комментарий глазами владельца (требование 4).
type ModerationItem struct {
	Kind      string       `json:"kind"`
	PostID    string       `json:"post_id"`
	CommentID *string      `json:"comment_id"`
	Author    ContentOwner `json:"author"`
	Text      string       `json:"text"`
	// Photo — ключ хранилища первого фото; ссылку из него делает дашборд.
	Photo        *string    `json:"-"`
	PhotoURL     *string    `json:"photo_url"`
	CreatedAt    time.Time  `json:"created_at"`
	Reports      int64      `json:"reports"`
	LastReportAt *time.Time `json:"last_report_at"`
	Reasons      []string   `json:"reasons"`
}

type ContentOwner struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Name     string `json:"name"`
}

// moderationItems — посты и комментарии вместе с жалобами на них.
// Жалоба на пост — строка с post_id и без comment_id, на комментарий —
// наоборот (ADR-0017), поэтому у поста в report_post стоит его id, а у
// комментария NULL, и сравнение идёт через IS NOT DISTINCT FROM.
// Видимость (013) и блокировки (022) не действуют: владелец видит всё.
const moderationItems = `
	WITH items AS (
		SELECT 'post' AS kind, p.id AS post_id, p.id AS report_post, NULL::uuid AS comment_id,
		       p.author_id, p.caption AS text, p.created_at
		FROM posts p
		UNION ALL
		SELECT 'comment', c.post_id, NULL::uuid, c.id,
		       c.author_id, c.text, c.created_at
		FROM comments c
	), rep AS (
		SELECT post_id, comment_id, count(*) AS n, max(created_at) AS last_at,
		       coalesce(array_agg(reason ORDER BY created_at DESC, id DESC)
		                FILTER (WHERE btrim(coalesce(reason, '')) <> ''), '{}') AS reasons
		FROM reports GROUP BY post_id, comment_id
	)
	SELECT i.kind, i.post_id::text, i.comment_id::text, u.id::text, u.nickname, u.name, i.text,
	       (SELECT m.storage_key FROM media m WHERE m.post_id = i.post_id ORDER BY m.position LIMIT 1),
	       i.created_at, coalesce(r.n, 0), r.last_at, coalesce(r.reasons, '{}')
	FROM items i
	JOIN users u ON u.id = i.author_id
	LEFT JOIN rep r ON r.post_id IS NOT DISTINCT FROM i.report_post
	               AND r.comment_id IS NOT DISTINCT FROM i.comment_id
`

// moderation собирает оба списка: жалобы — больше жалоб выше, при
// равенстве свежая жалоба выше, до 50 (требование 5); свежее — 30
// последних постов и комментариев (требование 6).
func (m *Monitor) moderation(ctx context.Context, s *Snapshot) error {
	var err error
	s.Moderation.Reported, err = m.moderationList(ctx, moderationItems+`
		WHERE r.n > 0
		ORDER BY r.n DESC, r.last_at DESC, i.created_at DESC
		LIMIT 50`)
	if err != nil {
		return err
	}
	s.Moderation.Recent, err = m.moderationList(ctx, moderationItems+`
		ORDER BY i.created_at DESC, i.kind DESC, coalesce(i.comment_id, i.post_id)
		LIMIT 30`)
	if err != nil {
		return err
	}
	s.Moderation.Groups, err = m.placeGroups(ctx)
	return err
}

// placeGroups — геогруппы, больше участников выше; участники — по нику.
// Ждущие приглашения — не участники.
func (m *Monitor) placeGroups(ctx context.Context) ([]PlaceGroup, error) {
	rows, err := m.db.Query(ctx, `
		SELECT g.id::text, g.name, g.description,
		       coalesce(json_agg(json_build_object('id', u.id::text, 'nickname', u.nickname, 'name', u.name)
		                ORDER BY lower(u.nickname), u.id) FILTER (WHERE u.id IS NOT NULL), '[]')
		FROM groups g
		LEFT JOIN group_members gm ON gm.group_id = g.id AND gm.state = 'member'
		LEFT JOIN users u ON u.id = gm.user_id
		WHERE g.kind = 'place'
		GROUP BY g.id
		ORDER BY count(u.id) DESC, lower(g.name), g.id`)
	if err != nil {
		return nil, err
	}
	groups, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (PlaceGroup, error) {
		var g PlaceGroup
		err := row.Scan(&g.ID, &g.Name, &g.Area, &g.Members)
		return g, err
	})
	if groups == nil {
		groups = []PlaceGroup{}
	}
	return groups, err
}

func (m *Monitor) moderationList(ctx context.Context, query string) ([]ModerationItem, error) {
	rows, err := m.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ModerationItem, error) {
		var it ModerationItem
		err := row.Scan(&it.Kind, &it.PostID, &it.CommentID, &it.Author.ID, &it.Author.Nickname,
			&it.Author.Name, &it.Text, &it.Photo, &it.CreatedAt, &it.Reports, &it.LastReportAt, &it.Reasons)
		return it, err
	})
	if items == nil {
		items = []ModerationItem{}
	}
	return items, err
}
