// Лайки: specs/005-likes.md.
package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// likeColumns — число лайков и признак «я отметил», как их считает
// любой запрос поста. Счётчика в таблице постов нет намеренно: держать
// его согласованным дороже, чем посчитать по первичному ключу
// (specs/005-likes.md, требование 7).
//
// Ждёт, что идентификатор спрашивающего — второй параметр запроса.
const likeColumns = `
	(SELECT count(*) FROM post_likes l WHERE l.post_id = p.id) AS likes,
	EXISTS (
		SELECT 1 FROM post_likes l
		WHERE l.post_id = p.id AND l.user_id = $2::uuid
	) AS liked`

// LikePost отмечает пост. Повторный лайк ничего не меняет: важно не то,
// что изменилось, а то, что стало (specs/005-likes.md, требование 3).
func (s *Server) LikePost(ctx context.Context, request gen.LikePostRequestObject) (gen.LikePostResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.LikePost401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.PostId) {
		return gen.LikePost404JSONResponse(errPostNotFound), nil
	}

	// ON CONFLICT DO NOTHING — это и есть идемпотентность: второй лайк
	// того же человека база не примет, и это не ошибка. А SELECT вместо
	// VALUES означает, что лайк несуществующему посту просто не вставится
	// и на этом всё: «такого поста нет» скажет чтение ниже.
	if _, err := s.db.Exec(ctx, `
		INSERT INTO post_likes (post_id, user_id)
		SELECT p.id, $2 FROM posts p WHERE p.id = $1 AND `+postVisibleTo("$2")+`
		ON CONFLICT DO NOTHING`, request.PostId, current.user.Id,
	); err != nil {
		return nil, err
	}

	post, err := s.post(ctx, request.PostId, current.user.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.LikePost404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.LikePost200JSONResponse(post), nil
}

// UnlikePost снимает лайк. Снять лайк, которого не было, — не ошибка.
func (s *Server) UnlikePost(ctx context.Context, request gen.UnlikePostRequestObject) (gen.UnlikePostResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.UnlikePost401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.PostId) {
		return gen.UnlikePost404JSONResponse(errPostNotFound), nil
	}

	if _, err := s.db.Exec(ctx, `
		DELETE FROM post_likes l
		WHERE l.post_id = $1 AND l.user_id = $2
		  AND EXISTS (
		      SELECT 1 FROM posts p WHERE p.id = l.post_id AND `+postVisibleTo("$2")+`
		  )`,
		request.PostId, current.user.Id,
	); err != nil {
		return nil, err
	}

	// Невидимому посту лайк не снимается: для смотрящего такого поста
	// нет (specs/013-post-visibility.md, требования 4 и 6), и его прежний
	// лайк остаётся на месте. Пост читается после удаления, и он же отвечает на вопрос, есть ли
	// такой пост вообще: снять лайк у несуществующего — это 404, а не
	// «ничего не произошло».
	post, err := s.post(ctx, request.PostId, current.user.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.UnlikePost404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.UnlikePost200JSONResponse(post), nil
}
