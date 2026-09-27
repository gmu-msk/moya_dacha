// Правка подписи и комментария, блокировка и удаление аккаунта:
// specs/022-edit-block-delete.md.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// blockedBetween — условие «между a и b есть блокировка в любую сторону».
// Блокировка записана одной строкой, а действует на обоих (требования
// 12–14), поэтому проверяются оба направления.
func blockedBetween(a, b string) string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1 FROM blocks bl
		WHERE (bl.blocker_id = %[1]s AND bl.blocked_id = %[2]s)
		   OR (bl.blocker_id = %[2]s AND bl.blocked_id = %[1]s)
	)`, a, b)
}

// blocks — «who заблокировал whom».
func blocks(who, whom string) string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1 FROM blocks bl WHERE bl.blocker_id = %s AND bl.blocked_id = %s
	)`, who, whom)
}

// EditCaption меняет подпись своего поста. Порядок проверок как у
// удаления: пост, «своё ли», потом подпись (требование 6).
func (s *Server) EditCaption(ctx context.Context, request gen.EditCaptionRequestObject) (gen.EditCaptionResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.EditCaption401JSONResponse(errUnauthorized), nil
	}

	author, err := s.postAuthor(ctx, request.PostId, current.user.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.EditCaption404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if author != current.user.Id {
		return gen.EditCaption403JSONResponse(errNotYourPostEdit), nil
	}
	if request.Body == nil {
		return gen.EditCaption400JSONResponse(errInvalidEdit), nil
	}

	caption := strings.TrimSpace(request.Body.Caption)
	if utf8.RuneCountInString(caption) > MaxCaptionLength {
		return gen.EditCaption400JSONResponse(errInvalidCaption), nil
	}

	// Та же подпись — не правка: edited_at не появляется и не сдвигается
	// (требование 4).
	if _, err := s.db.Exec(ctx, `
		UPDATE posts SET caption = $2, edited_at = now()
		WHERE id = $1 AND caption <> $2`, request.PostId, caption,
	); err != nil {
		return nil, err
	}

	post, err := s.post(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	return gen.EditCaption200JSONResponse(post), nil
}

// EditComment меняет текст своего комментария. Место в разговоре не
// меняется: порядок — по времени, когда комментарий оставили
// (требование 3).
func (s *Server) EditComment(ctx context.Context, request gen.EditCommentRequestObject) (gen.EditCommentResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.EditComment401JSONResponse(errUnauthorized), nil
	}

	found, err := s.postExists(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.EditComment404JSONResponse(errPostNotFound), nil
	}

	author, err := s.commentAuthor(ctx, request.PostId, request.CommentId)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.EditComment404JSONResponse(errCommentNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if author != current.user.Id {
		return gen.EditComment403JSONResponse(errNotYourCommentEdit), nil
	}
	if request.Body == nil {
		return gen.EditComment400JSONResponse(errInvalidEdit), nil
	}

	text := strings.TrimSpace(request.Body.Text)
	switch {
	case text == "":
		return gen.EditComment400JSONResponse(errEmptyComment), nil
	case utf8.RuneCountInString(text) > MaxCommentLength:
		return gen.EditComment400JSONResponse(errInvalidComment), nil
	}

	if _, err := s.db.Exec(ctx, `
		UPDATE comments SET text = $2, edited_at = now()
		WHERE id = $1 AND text <> $2`, request.CommentId, text,
	); err != nil {
		return nil, err
	}

	row := s.db.QueryRow(ctx, `
		SELECT `+commentFields+`
		FROM comments c JOIN users u ON u.id = c.author_id
		WHERE c.id = $1`, request.CommentId)
	comment, err := s.scanComment(row)
	if err != nil {
		return nil, err
	}
	return gen.EditComment200JSONResponse(comment), nil
}

// BlockUser блокирует пользователя и разрывает связи между двумя сразу:
// подписки, заявки и уведомления в обе стороны (требование 11). Всё в
// одной транзакции, чтобы между блокировкой и разрывом не проскочила
// новая подписка.
func (s *Server) BlockUser(ctx context.Context, request gen.BlockUserRequestObject) (gen.BlockUserResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.BlockUser401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.UserId) {
		return gen.BlockUser404JSONResponse(errUserNotFound), nil
	}
	if request.UserId == current.user.Id {
		return gen.BlockUser400JSONResponse(errCannotBlockSelf), nil
	}

	me, them := current.user.Id, request.UserId
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	visible, err := userVisible(ctx, tx, them, me)
	if err != nil {
		return nil, err
	}
	if !visible {
		return gen.BlockUser404JSONResponse(errUserNotFound), nil
	}

	for _, query := range []string{
		`INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		`DELETE FROM follows
		 WHERE (follower_id = $1 AND followee_id = $2)
		    OR (follower_id = $2 AND followee_id = $1)`,
		`DELETE FROM notifications
		 WHERE (user_id = $1 AND actor_id = $2)
		    OR (user_id = $2 AND actor_id = $1)`,
	} {
		if _, err := tx.Exec(ctx, query, me, them); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return gen.BlockUser204Response{}, nil
}

// UnblockUser снимает только блокировку: подписки сами не возвращаются
// (требование 17).
func (s *Server) UnblockUser(ctx context.Context, request gen.UnblockUserRequestObject) (gen.UnblockUserResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.UnblockUser401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.UserId) {
		return gen.UnblockUser404JSONResponse(errUserNotFound), nil
	}

	visible, err := userVisible(ctx, s.db, request.UserId, current.user.Id)
	if err != nil {
		return nil, err
	}
	if !visible {
		return gen.UnblockUser404JSONResponse(errUserNotFound), nil
	}

	if _, err := s.db.Exec(ctx,
		`DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`,
		current.user.Id, request.UserId,
	); err != nil {
		return nil, err
	}
	return gen.UnblockUser204Response{}, nil
}

// GetBlocked отдаёт свои блокировки целиком, последние сверху
// (требование 18).
func (s *Server) GetBlocked(ctx context.Context, _ gen.GetBlockedRequestObject) (gen.GetBlockedResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetBlocked401JSONResponse(errUnauthorized), nil
	}

	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.nickname, u.name, u.avatar_key
		FROM blocks b JOIN users u ON u.id = b.blocked_id
		WHERE b.blocker_id = $1
		ORDER BY b.created_at DESC, u.id DESC`, current.user.Id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := gen.AuthorList{Items: []gen.Author{}}
	for rows.Next() {
		var (
			item      gen.Author
			avatarKey *string
		)
		if err := rows.Scan(&item.Id, &item.Nickname, &item.Name, &avatarKey); err != nil {
			return nil, err
		}
		if avatarKey != nil {
			url := s.cfg.Media.URL(*avatarKey)
			item.AvatarUrl = &url
		}
		list.Items = append(list.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return gen.GetBlocked200JSONResponse(list), nil
}

// DeleteMe удаляет аккаунт целиком и сразу (требования 20–25,
// ADR-0025). Строки уходят каскадом от users, ключи файлов читаются
// до этого в той же транзакции, а сами файлы убираются после неё: база
// не ждёт хранилище (specs/007-deletion.md, требование 5).
func (s *Server) DeleteMe(ctx context.Context, _ gen.DeleteMeRequestObject) (gen.DeleteMeResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.DeleteMe401JSONResponse(errUnauthorized), nil
	}
	id := current.user.Id

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Все фотографии автора — и опубликованные, и нет — и его аватар.
	rows, err := tx.Query(ctx, `
		SELECT storage_key FROM media WHERE author_id = $1
		UNION ALL
		SELECT avatar_key FROM users WHERE id = $1 AND avatar_key IS NOT NULL`, id)
	if err != nil {
		return nil, err
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}

	// Отзыв остаётся владельцу, но без имени (требование 23).
	if _, err := tx.Exec(ctx,
		`UPDATE feedback SET author = $2 WHERE user_id = $1`, id, deletedAuthor,
	); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	for _, key := range keys {
		if err := s.cfg.Media.Delete(ctx, key); err != nil {
			slog.Error("не удалось удалить файл удалённого аккаунта", "key", key, "err", err)
		}
	}
	return gen.DeleteMe204Response{}, nil
}

// deletedAuthor — чем подписан отзыв удалённого пользователя.
const deletedAuthor = "удалённый пользователь"

// querier — пул или транзакция: проверка нужна и там, и там.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// userVisible отвечает, есть ли пользователь для смотрящего: того, кто
// заблокировал смотрящего, для него нет (требование 14).
func userVisible(ctx context.Context, db querier, userID, viewerID string) (bool, error) {
	var found bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM users u
			WHERE u.id = $1 AND NOT `+blocks("u.id", "$2::uuid")+`
		)`, userID, viewerID,
	).Scan(&found)
	return found, err
}

// Ошибки правки и блокировки (specs/022-edit-block-delete.md, «Ошибки»).
var (
	errNotYourPostEdit = gen.Error{
		Code:    "not_your_post",
		Message: "Это чужой пост. Изменить можно только свой",
	}
	errNotYourCommentEdit = gen.Error{
		Code:    "not_your_comment",
		Message: "Это чужой комментарий. Изменить можно только свой",
	}
	errInvalidEdit = gen.Error{
		Code:    "invalid_request",
		Message: "Пришёл пустой запрос. Напишите текст и сохраните ещё раз",
	}
	errCannotBlockSelf = gen.Error{
		Code:    "cannot_block_self",
		Message: "Себя заблокировать нельзя",
	}
	errUserBlocked = gen.Error{
		Code:    "user_blocked",
		Message: "Вы заблокировали этого человека. Сначала разблокируйте",
	}
)
