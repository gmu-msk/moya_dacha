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
	// Блокировка в любую сторону прячет посты обоих друг от друга
	// (specs/022-edit-block-delete.md, требование 12).
	return `(p.author_id = ` + v + `
		OR (NOT ` + blockedBetween(v, "p.author_id") + ` AND (
			(p.visibility = 'all' AND (
				NOT (SELECT pa.closed FROM users pa WHERE pa.id = p.author_id)
				OR ` + follows(v, "p.author_id") + `))
			OR (p.visibility = 'friends'
				AND ` + follows(v, "p.author_id") + `
				AND ` + follows("p.author_id", v) + `)
			OR (p.visibility = 'group' AND ` + visibilityGroupMember(v) + `))))`
}

// visibilityGroupMember — смотрящий участник группы видимости поста p и
// группа ему видна (specs/031-group-visibility.md, требования 5 и 7).
// Хозяин — тоже строка member, приглашённый — нет. Группу удалили —
// ссылки нет, и пост видит только автор (требование 9).
func visibilityGroupMember(viewer string) string {
	return `EXISTS (
		SELECT 1 FROM group_members vm JOIN groups g ON g.id = vm.group_id
		WHERE vm.group_id = p.visibility_group_id AND vm.user_id = ` + viewer + `
		  AND vm.state = 'member' AND ` + groupVisible(viewer) + `)`
}

// postVisibilityColumns — видимость поста p для ответа и его группа
// видимости; к запросу добавляется postVisibilityJoin. Старое поле —
// по-прежнему одно из трёх, чтобы старые сборки разбирали ответ
// (specs/031-group-visibility.md, требования 10 и 11): у поста группы —
// friends, а у поста удалённой группы — me, как он и виден.
const postVisibilityColumns = `
	CASE p.visibility
		WHEN 'group' THEN CASE WHEN vg.id IS NULL THEN 'me' ELSE 'friends' END
		ELSE p.visibility
	END,
	vg.id, vg.name`

const postVisibilityJoin = `LEFT JOIN groups vg ON vg.id = p.visibility_group_id`

// visibilityGroupScan — куда читать группу видимости из postVisibilityColumns.
type visibilityGroupScan struct {
	id, name *string
}

func (v *visibilityGroupScan) targets(post *gen.Post) []any {
	return []any{&post.Visibility, &v.id, &v.name}
}

func (v *visibilityGroupScan) apply(post *gen.Post) {
	if v.id == nil || v.name == nil {
		return
	}
	post.VisibilityGroup = &gen.GroupBrief{Id: *v.id, Name: *v.name}
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

	// Видимость группы снимается; группа остаётся в группах поста
	// (specs/031-group-visibility.md, требование 12).
	if _, err := s.db.Exec(ctx,
		`UPDATE posts SET visibility = $2, visibility_group_id = NULL WHERE id = $1`,
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
