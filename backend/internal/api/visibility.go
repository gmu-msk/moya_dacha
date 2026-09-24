// Видимость постов: specs/013-post-visibility.md.
package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// postVisibleTo — условие «пост p виден смотрящему» для запроса, где
// смотрящий передан параметром viewer (например, "$2"). Одно условие на
// все запросы постов и в SQL, а не в Go после выборки: иначе страница
// ленты пришла бы короче limit и курсор сбился бы (требование 4).
//
// Видимость считается в момент запроса (требование 5): подписки,
// закрытость профиля и видимость поста читаются здесь же.
func postVisibleTo(viewer string) string {
	follows := func(from, to string) string {
		return fmt.Sprintf(`EXISTS (
			SELECT 1 FROM follows vf
			WHERE vf.follower_id = %s AND vf.followee_id = %s AND vf.accepted
		)`, from, to)
	}
	v := viewer + "::uuid"
	return `(p.author_id = ` + v + `
		OR (p.visibility = 'all' AND (
			NOT (SELECT pa.closed FROM users pa WHERE pa.id = p.author_id)
			OR ` + follows(v, "p.author_id") + `))
		OR (p.visibility = 'friends'
			AND ` + follows(v, "p.author_id") + `
			AND ` + follows("p.author_id", v) + `))`
}

// validVisibility — одна из трёх видимостей контракта.
func validVisibility(value gen.PostVisibility) bool {
	switch value {
	case "all", "friends", "me":
		return true
	}
	return false
}

// SetPostVisibility меняет видимость своего поста (требование 6).
// Порядок проверок как у удаления: сначала виден ли пост, потом чей он,
// потом что прислали.
func (s *Server) SetPostVisibility(ctx context.Context, request gen.SetPostVisibilityRequestObject) (gen.SetPostVisibilityResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.SetPostVisibility401JSONResponse(errUnauthorized), nil
	}

	author, err := s.postAuthor(ctx, request.PostId, current.user.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.SetPostVisibility404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if author != current.user.Id {
		return gen.SetPostVisibility403JSONResponse(errNotYourPost), nil
	}
	if request.Body == nil || !validVisibility(request.Body.Visibility) {
		return gen.SetPostVisibility400JSONResponse(errInvalidVisibility), nil
	}

	if _, err := s.db.Exec(ctx,
		`UPDATE posts SET visibility = $2 WHERE id = $1`,
		request.PostId, string(request.Body.Visibility),
	); err != nil {
		return nil, err
	}

	post, err := s.post(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	return gen.SetPostVisibility200JSONResponse(post), nil
}

// errInvalidVisibility — видимость не из трёх известных.
var errInvalidVisibility = gen.Error{
	Code:    "invalid_request",
	Message: "Видимость поста — всем, друзьям или только мне",
}
