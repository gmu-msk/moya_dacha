// Лента: specs/004-feed.md.
package api

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// DefaultFeedLimit и MaxFeedLimit — размер страницы ленты по умолчанию
// и предел, который клиент не перешагнёт. Полсотни постов за раз — это
// уже полсотни фотографий на дачной связи (specs/004-feed.md, требование 4).
const (
	DefaultFeedLimit = 20
	MaxFeedLimit     = 50
)

// GetFeed отдаёт страницу ленты, новые сверху: вкладку «Все» или
// «Подписки» (specs/012-follows.md, требования 15–18).
func (s *Server) GetFeed(ctx context.Context, request gen.GetFeedRequestObject) (gen.GetFeedResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetFeed401JSONResponse(errUnauthorized), nil
	}

	followingOnly := false
	if request.Params.Scope != nil {
		switch string(*request.Params.Scope) {
		case "all":
		case "following":
			followingOnly = true
		default:
			return gen.GetFeed400JSONResponse(errInvalidScope), nil
		}
	}

	limit := DefaultFeedLimit
	if request.Params.Limit != nil {
		limit = int(*request.Params.Limit)
	}
	if limit < 1 || limit > MaxFeedLimit {
		return gen.GetFeed400JSONResponse(errInvalidLimit), nil
	}

	var after *feedCursor
	if request.Params.Cursor != nil && *request.Params.Cursor != "" {
		parsed, err := parseFeedCursor(*request.Params.Cursor)
		if err != nil {
			return gen.GetFeed400JSONResponse(errInvalidCursor), nil
		}
		after = &parsed
	}

	// Лента одного тэга (specs/028-post-tags.md, требования 12–14).
	var tag string
	if request.Params.Tag != nil {
		normalized, err := normalizeTag(*request.Params.Tag)
		if err != nil {
			return gen.GetFeed400JSONResponse(errInvalidTag), nil
		}
		tag = normalized
	}

	page, err := s.feedPage(ctx, current.user.Id, feedFilter{tag: tag, followingOnly: followingOnly}, after, limit)
	if err != nil {
		return nil, err
	}
	return gen.GetFeed200JSONResponse(page), nil
}

// feedFilter сужает ленту. Пустой — лента «Все».
type feedFilter struct {
	// authorID — посты одного человека: так страницами отдаются посты в
	// профиле (specs/009-user-profile.md).
	authorID string
	// tag — посты с этим тэгом (specs/028-post-tags.md, требование 12).
	tag string
	// groupID — посты группы, её лента (specs/030-group-posts.md,
	// требование 9).
	groupID string
	// followingOnly — вкладка «Подписки» (specs/012-follows.md,
	// требование 17): свои посты, посты тех, на кого смотрящий подписан
	// (заявка — не подписка), и посты групп, где он участник
	// (specs/030-group-posts.md, требование 6).
	followingOnly bool
}

// feedPage читает страницу ленты и решает, есть ли продолжение.
// viewerID нужен, чтобы у каждого поста был признак «я отметил»
// (specs/005-likes.md, требование 4). Число комментариев приходит там
// же: в ленте видно, где разговор идёт (specs/006-comments.md).
//
// Каждый пост проходит проверку видимости (specs/013-post-visibility.md):
// закрытый профиль, «друзьям», «только мне». Группа поста её не
// расширяет (specs/030-group-posts.md, требование 3).
func (s *Server) feedPage(ctx context.Context, viewerID string, filter feedFilter, after *feedCursor, limit int) (gen.Feed, error) {
	var (
		afterTime *time.Time
		afterID   *string
	)
	if after != nil {
		afterTime, afterID = &after.createdAt, &after.id
	}
	optional := func(value string) *string {
		if value == "" {
			return nil
		}
		return &value
	}

	// Берём на пост больше, чем просили: лишний пост не отдаётся, он
	// только отвечает на вопрос «есть ли что-то дальше».
	rows, err := s.db.Query(ctx, `
		SELECT p.id, p.created_at, p.edited_at, p.caption, p.visibility,
			u.id, u.nickname, u.name, u.avatar_key,
			(SELECT count(*) FROM post_likes l WHERE l.post_id = p.id),
			EXISTS (
				SELECT 1 FROM post_likes l
				WHERE l.post_id = p.id AND l.user_id = $4::uuid
			),
			`+commentCount("$4")+`,
			`+postPlaceColumns("$4")+`
		FROM posts p JOIN users u ON u.id = p.author_id
		`+postPlaceJoin+`
		WHERE ($5::uuid IS NULL OR p.author_id = $5::uuid)
		  AND `+postVisibleTo("$4")+`
		  AND (NOT $6::boolean
		       OR p.author_id = $4::uuid
		       OR EXISTS (
		           SELECT 1 FROM follows f
		           WHERE f.follower_id = $4::uuid AND f.followee_id = p.author_id
		             AND f.accepted
		       )
		       OR EXISTS (
		           SELECT 1 FROM post_groups pg
		           JOIN group_members gm ON gm.group_id = pg.group_id
		           WHERE pg.post_id = p.id AND gm.user_id = $4::uuid
		             AND gm.state = 'member'
		       ))
		  AND ($7::text IS NULL OR EXISTS (
		       SELECT 1 FROM post_tags pt WHERE pt.post_id = p.id AND pt.tag = $7))
		  AND ($8::uuid IS NULL OR EXISTS (
		       SELECT 1 FROM post_groups pg WHERE pg.post_id = p.id AND pg.group_id = $8::uuid))
		  AND ($1::timestamptz IS NULL
		       OR (p.created_at, p.id) < ($1::timestamptz, $2::uuid))
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $3`, afterTime, afterID, limit+1, viewerID,
		optional(filter.authorID), filter.followingOnly, optional(filter.tag), optional(filter.groupID))
	if err != nil {
		return gen.Feed{}, err
	}
	defer rows.Close()

	feed := gen.Feed{Items: []gen.Post{}}
	for rows.Next() {
		var (
			post      gen.Post
			avatarKey *string
			place     postPlaceScan
		)
		if err := rows.Scan(append([]any{
			&post.Id, &post.CreatedAt, &post.EditedAt, &post.Caption, &post.Visibility,
			&post.Author.Id, &post.Author.Nickname, &post.Author.Name, &avatarKey,
			&post.Likes, &post.Liked, &post.Comments,
		}, place.targets()...)...); err != nil {
			return gen.Feed{}, err
		}
		place.apply(&post)
		if avatarKey != nil {
			url := s.cfg.Media.URL(*avatarKey)
			post.Author.AvatarUrl = &url
		}
		feed.Items = append(feed.Items, post)
	}
	if err := rows.Err(); err != nil {
		return gen.Feed{}, err
	}

	if len(feed.Items) > limit {
		feed.Items = feed.Items[:limit]
		last := feed.Items[limit-1]
		cursor := feedCursor{createdAt: last.CreatedAt, id: last.Id}.String()
		feed.NextCursor = &cursor
	}

	if err := s.attachMedia(ctx, feed.Items); err != nil {
		return gen.Feed{}, err
	}
	if err := s.attachTags(ctx, feed.Items); err != nil {
		return gen.Feed{}, err
	}
	if err := s.attachGroups(ctx, viewerID, feed.Items); err != nil {
		return gen.Feed{}, err
	}
	return feed, nil
}

// attachMedia раскладывает медиа по постам страницы. Один запрос на всю
// страницу, а не по запросу на пост: двадцать постов — это два запроса
// к базе, а не двадцать один (specs/004-feed.md).
func (s *Server) attachMedia(ctx context.Context, posts []gen.Post) error {
	if len(posts) == 0 {
		return nil
	}

	ids := make([]string, len(posts))
	for i, post := range posts {
		ids[i] = post.Id
	}

	rows, err := s.db.Query(ctx, `
		SELECT post_id, id, kind, storage_key, width, height
		FROM media WHERE post_id = ANY($1::uuid[]) ORDER BY post_id, position`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	byPost := make(map[string][]gen.Media, len(posts))
	for rows.Next() {
		var (
			postID string
			item   gen.Media
			kind   string
			key    string
		)
		if err := rows.Scan(&postID, &item.Id, &kind, &key, &item.Width, &item.Height); err != nil {
			return err
		}
		item.Kind = gen.MediaKind(kind)
		item.Url = s.cfg.Media.URL(key)
		byPost[postID] = append(byPost[postID], item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range posts {
		posts[i].Media = byPost[posts[i].Id]
	}
	return nil
}

// feedCursor — место в ленте: время публикации и идентификатор
// последнего отданного поста.
//
// Пара строго убывает и уникальна, поэтому пост не пропускается и не
// повторяется, даже если в ту же миллисекунду опубликовано несколько
// постов. Смещением (OFFSET) так не выйдет: оно считается от начала
// ленты, и каждый новый пост сдвигает всё, что ниже
// (docs/adr/0015-feed-paginated-by-cursor.md).
type feedCursor struct {
	createdAt time.Time
	id        string
}

// cursorSeparator разделяет время и идентификатор. В самом времени
// и в UUID его быть не может.
const cursorSeparator = "|"

// String кодирует курсор в непрозрачную для клиента строку. Непрозрачную,
// но не секретную: она не заменяет токен и без него ничего не открывает.
func (c feedCursor) String() string {
	raw := c.createdAt.UTC().Format(time.RFC3339Nano) + cursorSeparator + c.id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// errBadCursor — курсор испорчен или пришёл из другой версии сервиса.
var errBadCursor = errors.New("курсор не разбирается")

func parseFeedCursor(value string) (feedCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return feedCursor{}, errBadCursor
	}

	moment, id, found := strings.Cut(string(raw), cursorSeparator)
	if !found || !isUUID(id) {
		return feedCursor{}, errBadCursor
	}

	createdAt, err := time.Parse(time.RFC3339Nano, moment)
	if err != nil {
		return feedCursor{}, errBadCursor
	}
	return feedCursor{createdAt: createdAt, id: id}, nil
}

// Ошибки ленты (specs/004-feed.md, «Ошибки»).
var (
	errInvalidLimit = gen.Error{
		Code:    "invalid_request",
		Message: "За раз отдаётся от 1 до 50 постов",
	}
	errInvalidScope = gen.Error{
		Code:    "invalid_request",
		Message: "Такой вкладки ленты нет",
	}
	errInvalidCursor = gen.Error{
		Code:    "invalid_cursor",
		Message: "Не получается продолжить ленту. Обновите её",
	}
)
