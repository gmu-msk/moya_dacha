// Комментарии: specs/006-comments.md.
package api

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// MaxCommentLength — сколько символов помещается в комментарий. Та же
// мера, что у подписи поста, нарочно: человеку не нужно помнить два
// предела (specs/006-comments.md, требование 4).
const MaxCommentLength = MaxCaptionLength

// commentCount — число комментариев под постом p, как его считает любой
// запрос поста. Счётчика в таблице постов нет по тем же причинам, что
// и у лайков (specs/006-comments.md, требование 8). Считается для
// смотрящего: комментарии тех, с кем у него блокировка, в число не входят
// (specs/022-edit-block-delete.md, требование 13).
func commentCount(viewer string) string {
	return `(SELECT count(*) FROM comments c WHERE c.post_id = p.id
		AND NOT ` + blockedBetween(viewer+"::uuid", "c.author_id") + `)`
}

// commentFields — комментарий c и его автор u в порядке scanComment.
const commentFields = `c.id, c.created_at, c.edited_at, c.text, u.id, u.nickname, u.name, u.avatar_key`

// GetComments отдаёт разговор под постом целиком, от старого к новому:
// страниц у него нет (specs/006-comments.md, требование 6).
func (s *Server) GetComments(ctx context.Context, request gen.GetCommentsRequestObject) (gen.GetCommentsResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetComments401JSONResponse(errUnauthorized), nil
	}

	found, err := s.postExists(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.GetComments404JSONResponse(errPostNotFound), nil
	}

	items, err := s.comments(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	return gen.GetComments200JSONResponse(gen.Comments{Items: items}), nil
}

// AddComment оставляет комментарий под постом — любым, включая свой.
func (s *Server) AddComment(ctx context.Context, request gen.AddCommentRequestObject) (gen.AddCommentResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.AddComment401JSONResponse(errUnauthorized), nil
	}

	// Пост проверяется раньше текста: сервис сначала отвечает, есть ли
	// куда писать (specs/006-comments.md, «API / контракт данных»).
	found, err := s.postExists(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.AddComment404JSONResponse(errPostNotFound), nil
	}

	if request.Body == nil {
		return gen.AddComment400JSONResponse(errEmptyRequest), nil
	}

	// Края обрезаются, и длина считается после обрезки — как у подписи
	// поста (specs/006-comments.md, требование 3).
	text := strings.TrimSpace(request.Body.Text)
	switch {
	case text == "":
		return gen.AddComment400JSONResponse(errEmptyComment), nil
	case utf8.RuneCountInString(text) > MaxCommentLength:
		return gen.AddComment400JSONResponse(errInvalidComment), nil
	}

	// Комментарий и его автор читаются тем же запросом, которым
	// вставляются: клиенту нужен не идентификатор, а то, что он сейчас
	// покажет под постом.
	row := s.db.QueryRow(ctx, `
		WITH added AS (
			INSERT INTO comments (post_id, author_id, text)
			VALUES ($1, $2, $3)
			RETURNING id, created_at, edited_at, author_id, text
		)
		SELECT `+commentFields+`
		FROM added c JOIN users u ON u.id = c.author_id`,
		request.PostId, current.user.Id, text)

	comment, err := s.scanComment(row)
	if err != nil {
		return nil, err
	}
	return gen.AddComment201JSONResponse(comment), nil
}

// postExists отвечает, есть ли такой пост для смотрящего: невидимого ему
// поста для него нет (specs/013-post-visibility.md, требование 4).
// Идентификатор, не похожий на UUID, — это «такого поста нет», а не
// ошибка разбора в базе.
func (s *Server) postExists(ctx context.Context, id, viewerID string) (bool, error) {
	if !isUUID(id) {
		return false, nil
	}

	var found bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM posts p WHERE p.id = $1 AND `+postVisibleTo("$2")+`)`,
		id, viewerID,
	).Scan(&found); err != nil {
		return false, err
	}
	return found, nil
}

// comments читает комментарии поста в их единственном порядке: от
// старого к новому, а при одинаковом времени — по идентификатору
// (specs/006-comments.md, требование 5). Комментарии тех, с кем у
// смотрящего блокировка, ему не видны (specs/022-edit-block-delete.md,
// требование 13).
func (s *Server) comments(ctx context.Context, postID, viewerID string) ([]gen.Comment, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+commentFields+`
		FROM comments c JOIN users u ON u.id = c.author_id
		WHERE c.post_id = $1 AND NOT `+blockedBetween("$2::uuid", "c.author_id")+`
		ORDER BY c.created_at, c.id`, postID, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []gen.Comment{}
	for rows.Next() {
		comment, err := s.scanComment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// scannable — строка результата, всё равно откуда: из одиночного
// запроса или из перебора.
type scannable interface {
	Scan(dest ...any) error
}

func (s *Server) scanComment(row scannable) (gen.Comment, error) {
	var (
		comment   gen.Comment
		avatarKey *string
	)
	if err := row.Scan(
		&comment.Id, &comment.CreatedAt, &comment.EditedAt, &comment.Text,
		&comment.Author.Id, &comment.Author.Nickname, &comment.Author.Name, &avatarKey,
	); err != nil {
		return gen.Comment{}, err
	}
	if avatarKey != nil {
		url := s.cfg.Media.URL(*avatarKey)
		comment.Author.AvatarUrl = &url
	}
	return comment, nil
}

// Ошибки комментариев (specs/006-comments.md, «Ошибки»).
var (
	errEmptyComment = gen.Error{
		Code:    "empty_comment",
		Message: "Комментарий пустой. Напишите что-нибудь",
	}
	errInvalidComment = gen.Error{
		Code:    "invalid_comment",
		Message: "Комментарий длиннее 1000 символов",
	}
)
