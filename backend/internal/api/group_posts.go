// Посты в группах и лента группы: specs/030-group-posts.md.
//
// Группа поста — только «где он покажется», а не «кто его увидит»:
// видимость поста (013) от групп не меняется (требование 3), поэтому
// здесь нет своей проверки видимости — лента группы идёт через тот же
// feedPage, что и лента.
package api

import (
	"context"
	"errors"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// errNotInGroup — автор выкладывает пост в группу, где он не участник,
// или в группу, которой нет (требование 2).
var errNotInGroup = errors.New("автор не участник группы")

// uniqueGroupIDs схлопывает повторы, сохраняя порядок (требование 1).
// Не UUID — сразу errNotInGroup: такой группы нет.
func uniqueGroupIDs(raw []string) ([]string, error) {
	seen := make(map[string]bool, len(raw))
	ids := make([]string, 0, len(raw))
	for _, id := range raw {
		if !isUUID(id) {
			return nil, errNotInGroup
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

// checkPostGroups — автор участник (member или хозяин: хозяин — тоже
// строка member) каждой группы и каждая ему видна (требование 2).
// Приглашённый — не участник.
func (s *Server) checkPostGroups(ctx context.Context, authorID string, groupIDs []string) error {
	if len(groupIDs) == 0 {
		return nil
	}
	var count int
	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM groups g
		JOIN group_members m ON m.group_id = g.id
		WHERE g.id = ANY($2::uuid[]) AND m.user_id = $1::uuid AND m.state = 'member'
		  AND `+groupVisible("$1::uuid"),
		authorID, groupIDs,
	).Scan(&count); err != nil {
		return err
	}
	if count != len(groupIDs) {
		return errNotInGroup
	}
	return nil
}

// writePostGroups связывает новый пост с группами.
func writePostGroups(ctx context.Context, db execer, postID string, groupIDs []string) error {
	if len(groupIDs) == 0 {
		return nil
	}
	_, err := db.Exec(ctx, `
		INSERT INTO post_groups (post_id, group_id)
		SELECT $1, unnest($2::uuid[])`, postID, groupIDs)
	return err
}

// attachGroups раскладывает группы по постам одним запросом на страницу,
// как attachTags: только видимые смотрящему, по названию без учёта
// регистра; нет групп — пустой список (требование 4).
func (s *Server) attachGroups(ctx context.Context, viewerID string, posts []gen.Post) error {
	if len(posts) == 0 {
		return nil
	}

	ids := make([]string, len(posts))
	for i, post := range posts {
		ids[i] = post.Id
	}

	rows, err := s.db.Query(ctx, `
		SELECT pg.post_id, g.id, g.name
		FROM post_groups pg JOIN groups g ON g.id = pg.group_id
		WHERE pg.post_id = ANY($1::uuid[]) AND `+groupVisible("$2::uuid")+`
		ORDER BY pg.post_id, lower(g.name), g.id`, ids, viewerID)
	if err != nil {
		return err
	}
	defer rows.Close()

	byPost := make(map[string][]gen.GroupBrief, len(posts))
	for rows.Next() {
		var (
			postID string
			group  gen.GroupBrief
		)
		if err := rows.Scan(&postID, &group.Id, &group.Name); err != nil {
			return err
		}
		byPost[postID] = append(byPost[postID], group)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range posts {
		groups := byPost[posts[i].Id]
		if groups == nil {
			groups = []gen.GroupBrief{}
		}
		posts[i].Groups = &groups
	}
	return nil
}

// GetGroupPosts — лента группы (требования 9–11): видна всем, кому видна
// группа, посты — только видимые смотрящему.
func (s *Server) GetGroupPosts(ctx context.Context, request gen.GetGroupPostsRequestObject) (gen.GetGroupPostsResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetGroupPosts401JSONResponse(errUnauthorized), nil
	}

	limit := DefaultFeedLimit
	if request.Params.Limit != nil {
		limit = int(*request.Params.Limit)
	}
	if limit < 1 || limit > MaxFeedLimit {
		return gen.GetGroupPosts400JSONResponse(errInvalidLimit), nil
	}

	var after *feedCursor
	if request.Params.Cursor != nil && *request.Params.Cursor != "" {
		parsed, err := parseFeedCursor(*request.Params.Cursor)
		if err != nil {
			return gen.GetGroupPosts400JSONResponse(errInvalidCursor), nil
		}
		after = &parsed
	}

	_, found, err := s.group(ctx, current.user.Id, request.GroupId)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.GetGroupPosts404JSONResponse(errGroupNotFound), nil
	}

	page, err := s.feedPage(ctx, current.user.Id, feedFilter{groupID: request.GroupId}, after, limit)
	if err != nil {
		return nil, err
	}
	return gen.GetGroupPosts200JSONResponse(page), nil
}

// errPostGroupNotMember — у поста чужая или несуществующая группа
// (specs/030-group-posts.md, «Тексты ошибок»).
var errPostGroupNotMember = gen.Error{
	Code:    "not_group_member",
	Message: "Выложить можно только в свою группу",
}
