// Закладки: specs/032-bookmarks.md.
package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// bookmarkColumns — число закладок и признак «я сохранил» поста p для
// смотрящего viewer (например, "$2"). Счётчика в таблице постов нет, как
// и у лайков (требование 11).
func bookmarkColumns(viewer string) string {
	return `
	(SELECT count(*) FROM post_bookmarks b WHERE b.post_id = p.id),
	EXISTS (
		SELECT 1 FROM post_bookmarks b
		WHERE b.post_id = p.id AND b.user_id = ` + viewer + `::uuid
	)`
}

// BookmarkPost добавляет пост в закладки. Повтор ничего не меняет: время
// закладки остаётся первым (требование 3).
func (s *Server) BookmarkPost(ctx context.Context, request gen.BookmarkPostRequestObject) (gen.BookmarkPostResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.BookmarkPost401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.PostId) {
		return gen.BookmarkPost404JSONResponse(errPostNotFound), nil
	}

	// Как у лайка: SELECT вместо VALUES, и закладка невидимому или
	// несуществующему посту просто не вставится — «такого поста нет»
	// скажет чтение ниже (требование 9).
	if _, err := s.db.Exec(ctx, `
		INSERT INTO post_bookmarks (user_id, post_id)
		SELECT $2, p.id FROM posts p WHERE p.id = $1 AND `+postVisibleTo("$2")+`
		ON CONFLICT DO NOTHING`, request.PostId, current.user.Id,
	); err != nil {
		return nil, err
	}

	post, err := s.post(ctx, request.PostId, current.user.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.BookmarkPost404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.BookmarkPost200JSONResponse(post), nil
}

// UnbookmarkPost убирает пост из закладок. Убрать закладку, которой не
// было, — не ошибка; у невидимого поста закладка остаётся (требование 9).
func (s *Server) UnbookmarkPost(ctx context.Context, request gen.UnbookmarkPostRequestObject) (gen.UnbookmarkPostResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.UnbookmarkPost401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.PostId) {
		return gen.UnbookmarkPost404JSONResponse(errPostNotFound), nil
	}

	if _, err := s.db.Exec(ctx, `
		DELETE FROM post_bookmarks b
		WHERE b.user_id = $2 AND b.post_id = $1
		  AND EXISTS (
		      SELECT 1 FROM posts p WHERE p.id = b.post_id AND `+postVisibleTo("$2")+`
		  )`,
		request.PostId, current.user.Id,
	); err != nil {
		return nil, err
	}

	post, err := s.post(ctx, request.PostId, current.user.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.UnbookmarkPost404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.UnbookmarkPost200JSONResponse(post), nil
}

// GetBookmarks отдаёт «Сохранённые»: та же страница постов, что лента,
// только свои закладки и по времени закладки (требования 7 и 8).
func (s *Server) GetBookmarks(ctx context.Context, request gen.GetBookmarksRequestObject) (gen.GetBookmarksResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetBookmarks401JSONResponse(errUnauthorized), nil
	}
	after, limit, bad := pageParams(request.Params.Limit, request.Params.Cursor)
	if bad != nil {
		return gen.GetBookmarks400JSONResponse(*bad), nil
	}

	page, err := s.feedPage(ctx, current.user.Id, feedFilter{bookmarks: true}, after, limit)
	if err != nil {
		return nil, err
	}
	return gen.GetBookmarks200JSONResponse(page), nil
}
