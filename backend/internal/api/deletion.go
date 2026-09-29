// Удаление своего поста и своего комментария: specs/007-deletion.md.
package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// DeletePost удаляет пост автора вместе с его фотографиями, лайками и
// комментариями. Удаление жёсткое, флага «удалён» в модели нет
// (ADR-0007).
func (s *Server) DeletePost(ctx context.Context, request gen.DeletePostRequestObject) (gen.DeletePostResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.DeletePost401JSONResponse(errUnauthorized), nil
	}

	// Порядок проверок: сначала есть ли пост, потом чей он. Иначе
	// «такого поста нет» и «пост чужой» поменялись бы местами
	// (specs/007-deletion.md, требование 8).
	author, err := s.postAuthor(ctx, request.PostId, current.user.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.DeletePost404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if author != current.user.Id {
		return gen.DeletePost403JSONResponse(errNotYourPost), nil
	}

	keys, err := s.removePost(ctx, request.PostId)
	if err != nil {
		return nil, err
	}

	s.deleteFiles(ctx, keys)
	return gen.DeletePost204Response{}, nil
}

// deleteFiles убирает файлы удалённого поста. Зовётся после транзакции:
// база — источник истины, и она не должна ждать хранилище. Ошибка здесь
// не отменяет удаления — человеку уже сказано «удалено», — но
// осиротевший файл попадает в лог (specs/007-deletion.md, требование 5).
func (s *Server) deleteFiles(ctx context.Context, keys []string) {
	for _, key := range keys {
		if err := s.cfg.Media.Delete(ctx, key); err != nil {
			slog.Error("не удалось удалить файл фотографии удалённого поста", "key", key, "err", err)
		}
	}
}

// DeleteComment удаляет свой комментарий. Чужой не удаляется даже
// автором поста: чужое убирают жалобой (specs/007-deletion.md,
// требование 3).
func (s *Server) DeleteComment(ctx context.Context, request gen.DeleteCommentRequestObject) (gen.DeleteCommentResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.DeleteComment401JSONResponse(errUnauthorized), nil
	}

	found, err := s.postExists(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.DeleteComment404JSONResponse(errPostNotFound), nil
	}

	// Пара «пост и комментарий» должна сойтись: существующий комментарий
	// под другим постом — это «такого комментария нет», а не удаление
	// (specs/007-deletion.md, требование 10).
	author, err := s.commentAuthor(ctx, request.PostId, request.CommentId)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.DeleteComment404JSONResponse(errCommentNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if author != current.user.Id {
		return gen.DeleteComment403JSONResponse(errNotYourComment), nil
	}

	if _, err := s.db.Exec(ctx,
		`DELETE FROM comments WHERE id = $1`, request.CommentId,
	); err != nil {
		return nil, err
	}
	return gen.DeleteComment204Response{}, nil
}

// postAuthor отвечает, кто выложил пост, видимый смотрящему. Невидимый —
// то же, что несуществующий (specs/013-post-visibility.md, требование 4).
// Идентификатор, не похожий на UUID, — это «такого поста нет», а не
// ошибка разбора в базе.
func (s *Server) postAuthor(ctx context.Context, id, viewerID string) (string, error) {
	if !isUUID(id) {
		return "", pgx.ErrNoRows
	}

	var author string
	if err := s.db.QueryRow(ctx,
		`SELECT p.author_id FROM posts p WHERE p.id = $1 AND `+postVisibleTo("$2"),
		id, viewerID,
	).Scan(&author); err != nil {
		return "", err
	}
	return author, nil
}

// commentAuthor отвечает, кто написал комментарий под этим постом.
func (s *Server) commentAuthor(ctx context.Context, postID, commentID string) (string, error) {
	if !isUUID(commentID) {
		return "", pgx.ErrNoRows
	}

	var author string
	if err := s.db.QueryRow(ctx,
		`SELECT author_id FROM comments WHERE id = $1 AND post_id = $2`,
		commentID, postID,
	).Scan(&author); err != nil {
		return "", err
	}
	return author, nil
}

// removePost убирает пост из базы и возвращает ключи его файлов.
//
// Фотографии удаляются явно, хотя `on delete cascade` убрал бы их и сам:
// иначе ключи файлов исчезнут вместе со строками, и убирать из хранилища
// станет нечего. Лайки и комментарии уходят каскадом (ADR-0007).
func (s *Server) removePost(ctx context.Context, id string) ([]string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx,
		`DELETE FROM media WHERE post_id = $1 RETURNING storage_key`, id)
	if err != nil {
		return nil, err
	}

	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM posts WHERE id = $1`, id); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return keys, nil
}

// Ошибки удаления (specs/007-deletion.md, «Ошибки»).
var (
	errNotYourPost = gen.Error{
		Code:    "not_your_post",
		Message: "Это чужой пост. Удалить можно только свой",
	}
	errNotYourComment = gen.Error{
		Code:    "not_your_comment",
		Message: "Это чужой комментарий. Удалить можно только свой",
	}
	errCommentNotFound = gen.Error{
		Code:    "comment_not_found",
		Message: "Такого комментария нет",
	}
)
