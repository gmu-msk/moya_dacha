// Группы по интересам и по месту: specs/029-groups.md.
//
// Хозяин группы — groups.owner_id и одновременно строка group_members
// со state = member: «Мои группы» и число участников считаются одним
// запросом. Заявка и приглашение становятся участием той же строкой.
package api

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// Ограничения группы (требования 1 и 3).
const (
	MaxGroupName        = 60
	MaxGroupDescription = 500
	MaxGroupRadiusKm    = 100
	// MaxGroupList — сколько групп отдаёт список (требование 16).
	MaxGroupList = 100
)

// groupVisible — группа g видна смотрящему viewer: её хозяин не
// заблокировал смотрящего (требование 14), а группу по приглашению
// видят только те, у кого в ней есть строка — хозяин, участники и
// приглашённые (13). Заявки в такую группу не бывает.
func groupVisible(viewer string) string {
	return `NOT ` + blocks("g.owner_id", viewer) + ` AND (g.join_policy <> 'invite' OR EXISTS (
		SELECT 1 FROM group_members vm WHERE vm.group_id = g.id AND vm.user_id = ` + viewer + `))`
}

// groupQuery — видимые смотрящему ($1) группы в порядке groupScan,
// отобранные условием where над b.* и упорядоченные order. Расстояние
// до места — как у поста (027): -1 — тот же пункт, NULL — посчитать
// нельзя. near — смотрящий в пункте группы или в её радиусе (12).
func groupQuery(where, order string) string {
	return `
	SELECT b.id, b.name, b.description, b.kind, b.join_policy, b.created_at,
		b.owner_id, b.owner_nickname, b.owner_name, b.owner_avatar,
		b.members, b.membership, b.requests,
		b.place_id, b.place_name, b.place_area, b.radius_km, b.dist,
		coalesce(b.kind = 'place' AND (b.dist = -1 OR b.dist <= b.radius_km), false) AS near
	FROM (
		SELECT g.id, g.name, g.description, g.kind, g.join_policy, g.created_at,
			o.id AS owner_id, o.nickname AS owner_nickname, o.name AS owner_name,
			o.avatar_key AS owner_avatar,
			(SELECT count(*) FROM group_members m WHERE m.group_id = g.id AND m.state = 'member') AS members,
			CASE WHEN g.owner_id = $1::uuid THEN 'owner' ELSE coalesce((
				SELECT m.state FROM group_members m WHERE m.group_id = g.id AND m.user_id = $1::uuid
			), 'none') END AS membership,
			CASE WHEN g.owner_id = $1::uuid THEN (
				SELECT count(*) FROM group_members m
				WHERE m.group_id = g.id AND m.state = 'requested'
				  AND NOT ` + blockedBetween("$1::uuid", "m.user_id") + `
			) END AS requests,
			gp.id AS place_id, gp.name AS place_name, gp.area AS place_area, g.radius_km,
			CASE
				WHEN gp.id IS NULL OR vp.id IS NULL THEN NULL
				WHEN vp.id = gp.id THEN -1
				WHEN vp.lat IS NULL OR vp.lon IS NULL OR gp.lat IS NULL OR gp.lon IS NULL THEN NULL
				ELSE 2 * 6371 * asin(least(1, sqrt(
					power(sin(radians(gp.lat - vp.lat) / 2), 2)
					+ cos(radians(vp.lat)) * cos(radians(gp.lat))
					  * power(sin(radians(gp.lon - vp.lon) / 2), 2))))
			END AS dist
		FROM groups g
		JOIN users o ON o.id = g.owner_id
		LEFT JOIN places gp ON gp.id = g.place_id
		LEFT JOIN users vu ON vu.id = $1::uuid
		LEFT JOIN places vp ON vp.id = vu.place_id
		WHERE ` + groupVisible("$1::uuid") + `
	) b
	WHERE ` + where + `
	ORDER BY ` + order
}

// scanGroup читает строку groupQuery.
func (s *Server) scanGroup(row pgx.Row) (gen.Group, error) {
	var (
		group      gen.Group
		kind       string
		policy     string
		avatarKey  *string
		members    int64
		membership string
		requests   *int64
		place      placeScan
		radius     *int32
		dist       *float64
	)
	if err := row.Scan(&group.Id, &group.Name, &group.Description, &kind, &policy, &group.CreatedAt,
		&group.Owner.Id, &group.Owner.Nickname, &group.Owner.Name, &avatarKey,
		&members, &membership, &requests,
		&place.id, &place.name, &place.area, &radius, &dist, &group.Near,
	); err != nil {
		return gen.Group{}, err
	}
	group.Kind = gen.GroupKind(kind)
	group.JoinPolicy = gen.GroupJoinPolicy(policy)
	if avatarKey != nil {
		url := s.cfg.Media.URL(*avatarKey)
		group.Owner.AvatarUrl = &url
	}
	group.Members = int32(members)
	group.Membership = gen.GroupMembership(membership)
	if requests != nil {
		n := int32(*requests)
		group.Requests = &n
	}
	group.Place = place.value()
	group.RadiusKm = radius
	if dist != nil {
		km := int32(0)
		if *dist != samePlace {
			km = roundDistance(*dist)
		}
		group.DistanceKm = &km
	}
	return group, nil
}

// group читает одну видимую смотрящему группу; found — видна ли она.
func (s *Server) group(ctx context.Context, viewerID, groupID string) (gen.Group, bool, error) {
	if !isUUID(groupID) {
		return gen.Group{}, false, nil
	}
	group, err := s.scanGroup(s.db.QueryRow(ctx, groupQuery("b.id = $2::uuid", "b.id"), viewerID, groupID))
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Group{}, false, nil
	}
	if err != nil {
		return gen.Group{}, false, err
	}
	return group, true, nil
}

// CreateGroup создаёт группу; создатель — хозяин и первый участник
// (требования 1–6).
func (s *Server) CreateGroup(ctx context.Context, request gen.CreateGroupRequestObject) (gen.CreateGroupResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.CreateGroup401JSONResponse(errUnauthorized), nil
	}
	body := request.Body
	if body == nil {
		return gen.CreateGroup400JSONResponse(errInvalidGroupRequest), nil
	}
	kind, policy := body.Kind, body.JoinPolicy
	if (kind != "interest" && kind != "place") ||
		(policy != "open" && policy != "request" && policy != "invite") {
		return gen.CreateGroup400JSONResponse(errInvalidGroupRequest), nil
	}

	name := strings.TrimSpace(body.Name)
	description := ""
	if body.Description != nil {
		description = strings.TrimSpace(*body.Description)
	}
	if name == "" || utf8.RuneCountInString(name) > MaxGroupName ||
		utf8.RuneCountInString(description) > MaxGroupDescription {
		return gen.CreateGroup400JSONResponse(errInvalidGroup), nil
	}

	var placeID *string
	if body.PlaceId != nil && strings.TrimSpace(*body.PlaceId) != "" {
		trimmed := strings.TrimSpace(*body.PlaceId)
		placeID = &trimmed
	}
	radius := body.RadiusKm
	if kind == "interest" {
		if placeID != nil || radius != nil {
			return gen.CreateGroup400JSONResponse(errInvalidGroup), nil
		}
	} else {
		if placeID == nil {
			return gen.CreateGroup400JSONResponse(errPlaceRequired), nil
		}
		if radius != nil && (*radius < 1 || *radius > MaxGroupRadiusKm) {
			return gen.CreateGroup400JSONResponse(errInvalidGroup), nil
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Место годится только то, что сервер уже отдавал в подсказках (025).
	var groupID string
	err = tx.QueryRow(ctx, `
		INSERT INTO groups (owner_id, name, description, kind, join_policy, place_id, radius_km)
		SELECT $1, $2, $3, $4, $5, $6, $7
		WHERE $6::text IS NULL OR EXISTS (SELECT 1 FROM places WHERE id = $6)
		RETURNING id`,
		current.user.Id, name, description, kind, policy, placeID, radius,
	).Scan(&groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.CreateGroup400JSONResponse(errUnknownPlace), nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id, state) VALUES ($1, $2, 'member')`,
		groupID, current.user.Id,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	group, _, err := s.group(ctx, current.user.Id, groupID)
	if err != nil {
		return nil, err
	}
	return gen.CreateGroup201JSONResponse(group), nil
}

// GetGroups отдаёт «Мои группы» или группы, куда можно вступить
// (требования 16–17).
func (s *Server) GetGroups(ctx context.Context, request gen.GetGroupsRequestObject) (gen.GetGroupsResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetGroups401JSONResponse(errUnauthorized), nil
	}
	params := request.Params

	where := []string{}
	order := ""
	scope := "mine"
	if params.Scope != nil {
		scope = *params.Scope
	}
	switch scope {
	case "mine":
		where = append(where, `b.membership IN ('owner', 'member')`)
		order = `lower(b.name), b.created_at, b.id`
	case "available":
		where = append(where, `b.membership IN ('none', 'requested', 'invited')`)
		order = `b.membership = 'invited' DESC, near DESC, b.members DESC, lower(b.name), b.created_at, b.id`
	default:
		return gen.GetGroups400JSONResponse(errInvalidGroupRequest), nil
	}

	var kind *string
	if params.Kind != nil {
		if *params.Kind != "interest" && *params.Kind != "place" {
			return gen.GetGroups400JSONResponse(errInvalidGroupRequest), nil
		}
		kind = params.Kind
	}
	where = append(where, `($2::text IS NULL OR b.kind = $2)`)

	var query *string
	if params.Q != nil && strings.TrimSpace(*params.Q) != "" {
		q := strings.ToLower(strings.TrimSpace(*params.Q))
		query = &q
	}
	where = append(where, `($3::text IS NULL OR strpos(lower(b.name), $3) > 0 OR strpos(lower(b.description), $3) > 0)`)

	rows, err := s.db.Query(ctx,
		groupQuery(strings.Join(where, " AND "), order)+` LIMIT `+strconv.Itoa(MaxGroupList),
		current.user.Id, kind, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := gen.GroupList{Items: []gen.Group{}}
	for rows.Next() {
		group, err := s.scanGroup(rows)
		if err != nil {
			return nil, err
		}
		list.Items = append(list.Items, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return gen.GetGroups200JSONResponse(list), nil
}

// GetGroup отдаёт одну группу (требование 15).
func (s *Server) GetGroup(ctx context.Context, request gen.GetGroupRequestObject) (gen.GetGroupResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetGroup401JSONResponse(errUnauthorized), nil
	}
	group, found, err := s.group(ctx, current.user.Id, request.GroupId)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.GetGroup404JSONResponse(errGroupNotFound), nil
	}
	return gen.GetGroup200JSONResponse(group), nil
}

// DeleteGroup удаляет свою группу; состав, заявки и приглашения уходят
// каскадом (требование 7).
func (s *Server) DeleteGroup(ctx context.Context, request gen.DeleteGroupRequestObject) (gen.DeleteGroupResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.DeleteGroup401JSONResponse(errUnauthorized), nil
	}
	group, found, err := s.group(ctx, current.user.Id, request.GroupId)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.DeleteGroup404JSONResponse(errGroupNotFound), nil
	}
	if group.Membership != gen.GroupMembershipOwner {
		return gen.DeleteGroup403JSONResponse(errNotGroupOwner), nil
	}
	if _, err := s.db.Exec(ctx,
		`DELETE FROM groups WHERE id = $1 AND owner_id = $2`, group.Id, current.user.Id,
	); err != nil {
		return nil, err
	}
	return gen.DeleteGroup204Response{}, nil
}

// JoinGroup вступает в группу, просится в неё или принимает приглашение
// (требование 18). Повтор ничего не меняет.
func (s *Server) JoinGroup(ctx context.Context, request gen.JoinGroupRequestObject) (gen.JoinGroupResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.JoinGroup401JSONResponse(errUnauthorized), nil
	}
	group, found, err := s.group(ctx, current.user.Id, request.GroupId)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.JoinGroup404JSONResponse(errGroupNotFound), nil
	}

	switch group.Membership {
	case gen.GroupMembershipInvited:
		// Приглашение — согласие хозяина: участник сразу, при любом правиле.
		_, err = s.db.Exec(ctx, `
			UPDATE group_members SET state = 'member', created_at = now()
			WHERE group_id = $1 AND user_id = $2 AND state = 'invited'`,
			group.Id, current.user.Id)
	case gen.GroupMembershipNone:
		// Без приглашения группа по приглашению не видна: сюда она не дойдёт.
		state := "member"
		if group.JoinPolicy == gen.GroupJoinPolicyRequest {
			state = "requested"
		}
		_, err = s.db.Exec(ctx, `
			INSERT INTO group_members (group_id, user_id, state) VALUES ($1, $2, $3)
			ON CONFLICT (group_id, user_id) DO NOTHING`,
			group.Id, current.user.Id, state)
	default:
		return gen.JoinGroup200JSONResponse(group), nil
	}
	if err != nil {
		return nil, err
	}

	group, found, err = s.group(ctx, current.user.Id, group.Id)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.JoinGroup404JSONResponse(errGroupNotFound), nil
	}
	return gen.JoinGroup200JSONResponse(group), nil
}

// LeaveGroup выходит из группы, отзывает заявку или отклоняет
// приглашение — строка смотрящего удаляется (требование 19).
func (s *Server) LeaveGroup(ctx context.Context, request gen.LeaveGroupRequestObject) (gen.LeaveGroupResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.LeaveGroup401JSONResponse(errUnauthorized), nil
	}
	group, found, err := s.group(ctx, current.user.Id, request.GroupId)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.LeaveGroup404JSONResponse(errGroupNotFound), nil
	}
	if group.Membership == gen.GroupMembershipOwner {
		return gen.LeaveGroup409JSONResponse(errOwnerCannotLeave), nil
	}
	if _, err := s.db.Exec(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, group.Id, current.user.Id,
	); err != nil {
		return nil, err
	}
	return gen.LeaveGroup204Response{}, nil
}

// GetGroupMembers отдаёт участников, а хозяину — ещё заявки и
// приглашения (требования 20–21).
func (s *Server) GetGroupMembers(ctx context.Context, request gen.GetGroupMembersRequestObject) (gen.GetGroupMembersResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetGroupMembers401JSONResponse(errUnauthorized), nil
	}
	state := "member"
	if request.Params.State != nil {
		state = *request.Params.State
	}
	if state != "member" && state != "requested" && state != "invited" {
		return gen.GetGroupMembers400JSONResponse(errInvalidGroupRequest), nil
	}
	group, found, err := s.group(ctx, current.user.Id, request.GroupId)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.GetGroupMembers404JSONResponse(errGroupNotFound), nil
	}
	if state != "member" && group.Membership != gen.GroupMembershipOwner {
		return gen.GetGroupMembers403JSONResponse(errNotGroupOwner), nil
	}

	// Участники — хозяин первым, дальше новые выше; заявки и приглашения
	// — новые сверху. Заблокировавших смотрящего нет (20), а заявок от
	// тех, с кем у хозяина блокировка, хозяин не видит, как и заявок на
	// подписку (012).
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.nickname, u.name, u.avatar_key, gm.state, gm.created_at, u.id = g.owner_id
		FROM group_members gm
		JOIN groups g ON g.id = gm.group_id
		JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = $1 AND gm.state = $2
		  AND NOT `+blocks("u.id", "$3::uuid")+`
		  AND ($2 = 'member' OR NOT `+blockedBetween("g.owner_id", "u.id")+`)
		ORDER BY u.id = g.owner_id DESC, gm.created_at DESC, u.id`,
		group.Id, state, current.user.Id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := gen.GroupMemberList{Items: []gen.GroupMember{}}
	for rows.Next() {
		member, err := s.scanGroupMember(rows)
		if err != nil {
			return nil, err
		}
		list.Items = append(list.Items, member)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return gen.GetGroupMembers200JSONResponse(list), nil
}

// scanGroupMember читает человека, его state, время и «хозяин ли».
func (s *Server) scanGroupMember(row pgx.Row) (gen.GroupMember, error) {
	var (
		member    gen.GroupMember
		avatarKey *string
		state     string
		owner     bool
	)
	if err := row.Scan(&member.User.Id, &member.User.Nickname, &member.User.Name, &avatarKey,
		&state, &member.CreatedAt, &owner); err != nil {
		return gen.GroupMember{}, err
	}
	if avatarKey != nil {
		url := s.cfg.Media.URL(*avatarKey)
		member.User.AvatarUrl = &url
	}
	member.State = gen.GroupMemberState(state)
	member.Role = gen.GroupMemberRoleMember
	if owner {
		member.Role = gen.GroupMemberRoleOwner
	}
	return member, nil
}

// AddGroupMember — хозяин принимает заявку или приглашает (требование 22).
func (s *Server) AddGroupMember(ctx context.Context, request gen.AddGroupMemberRequestObject) (gen.AddGroupMemberResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.AddGroupMember401JSONResponse(errUnauthorized), nil
	}
	group, found, err := s.group(ctx, current.user.Id, request.GroupId)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.AddGroupMember404JSONResponse(errGroupNotFound), nil
	}
	if group.Membership != gen.GroupMembershipOwner {
		return gen.AddGroupMember403JSONResponse(errNotGroupOwner), nil
	}
	if request.UserId == current.user.Id {
		return gen.AddGroupMember400JSONResponse(errCannotInviteSelf), nil
	}
	if !isUUID(request.UserId) {
		return gen.AddGroupMember404JSONResponse(errUserNotFound), nil
	}

	// Заблокировавшего хозяина для хозяина нет, а заблокированного им
	// сначала надо разблокировать — как с подпиской (022).
	var theyBlocked, iBlocked bool
	if err := s.db.QueryRow(ctx,
		`SELECT `+blocks("$2::uuid", "$1::uuid")+`, `+blocks("$1::uuid", "$2::uuid"),
		current.user.Id, request.UserId,
	).Scan(&theyBlocked, &iBlocked); err != nil {
		return nil, err
	}
	if theyBlocked {
		return gen.AddGroupMember404JSONResponse(errUserNotFound), nil
	}
	if iBlocked {
		return gen.AddGroupMember409JSONResponse(errUserBlocked), nil
	}

	// Нет строки — приглашение, заявка — участие, остальное как было.
	member, err := s.scanGroupMember(s.db.QueryRow(ctx, `
		WITH upsert AS (
			INSERT INTO group_members (group_id, user_id, state)
			SELECT $1, u.id, 'invited' FROM users u WHERE u.id = $2
			ON CONFLICT (group_id, user_id) DO UPDATE SET state = 'member', created_at = now()
			WHERE group_members.state = 'requested'
			RETURNING user_id, state, created_at
		), found AS (
			SELECT user_id, state, created_at FROM upsert
			UNION ALL
			SELECT user_id, state, created_at FROM group_members
			WHERE group_id = $1 AND user_id = $2 AND NOT EXISTS (SELECT 1 FROM upsert)
		)
		SELECT u.id, u.nickname, u.name, u.avatar_key, r.state, r.created_at, false
		FROM found r JOIN users u ON u.id = r.user_id`,
		group.Id, request.UserId))
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.AddGroupMember404JSONResponse(errUserNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.AddGroupMember200JSONResponse(member), nil
}

// RemoveGroupMember — хозяин отклоняет заявку, отзывает приглашение или
// убирает участника (требование 23).
func (s *Server) RemoveGroupMember(ctx context.Context, request gen.RemoveGroupMemberRequestObject) (gen.RemoveGroupMemberResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.RemoveGroupMember401JSONResponse(errUnauthorized), nil
	}
	group, found, err := s.group(ctx, current.user.Id, request.GroupId)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.RemoveGroupMember404JSONResponse(errGroupNotFound), nil
	}
	if group.Membership != gen.GroupMembershipOwner {
		return gen.RemoveGroupMember403JSONResponse(errNotGroupOwner), nil
	}
	if request.UserId == current.user.Id {
		return gen.RemoveGroupMember409JSONResponse(errOwnerCannotLeave), nil
	}
	if !isUUID(request.UserId) {
		return gen.RemoveGroupMember204Response{}, nil
	}
	if _, err := s.db.Exec(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, group.Id, request.UserId,
	); err != nil {
		return nil, err
	}
	return gen.RemoveGroupMember204Response{}, nil
}

// groupRequestRows — ждущие заявки в группы смотрящего ($1) и
// приглашения ему (требования 25–26): вид, группа, человек, время.
var groupRequestRows = `
	SELECT 'request' AS kind, g.id AS group_id, g.name AS group_name,
		u.id AS user_id, u.nickname, u.name, u.avatar_key, gm.created_at
	FROM group_members gm
	JOIN groups g ON g.id = gm.group_id
	JOIN users u ON u.id = gm.user_id
	WHERE g.owner_id = $1::uuid AND gm.state = 'requested'
	  AND NOT ` + blockedBetween("$1::uuid", "u.id") + `
	UNION ALL
	SELECT 'invite', g.id, g.name, o.id, o.nickname, o.name, o.avatar_key, gm.created_at
	FROM group_members gm
	JOIN groups g ON g.id = gm.group_id
	JOIN users o ON o.id = g.owner_id
	WHERE gm.user_id = $1::uuid AND gm.state = 'invited'
	  AND NOT ` + blocks("o.id", "$1::uuid")

// GetGroupRequests отдаёт заявки и приглашения для раздела
// «Уведомления», новые сверху (требование 25).
func (s *Server) GetGroupRequests(ctx context.Context, _ gen.GetGroupRequestsRequestObject) (gen.GetGroupRequestsResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetGroupRequests401JSONResponse(errUnauthorized), nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT * FROM (`+groupRequestRows+`) r ORDER BY r.created_at DESC, r.group_id, r.user_id`,
		current.user.Id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := gen.GroupRequestList{Items: []gen.GroupRequest{}}
	for rows.Next() {
		var (
			item      gen.GroupRequest
			kind      string
			avatarKey *string
			at        time.Time
		)
		if err := rows.Scan(&kind, &item.Group.Id, &item.Group.Name,
			&item.User.Id, &item.User.Nickname, &item.User.Name, &avatarKey, &at); err != nil {
			return nil, err
		}
		item.Kind = gen.GroupRequestKind(kind)
		item.CreatedAt = at
		if avatarKey != nil {
			url := s.cfg.Media.URL(*avatarKey)
			item.User.AvatarUrl = &url
		}
		list.Items = append(list.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return gen.GetGroupRequests200JSONResponse(list), nil
}

var (
	errInvalidGroupRequest = gen.Error{
		Code:    "invalid_request",
		Message: "Запрос не разобран",
	}
	errInvalidGroup = gen.Error{
		Code:    "invalid_group",
		Message: "Название — от 1 до 60 знаков, описание — до 500, радиус — от 1 до 100 км",
	}
	errPlaceRequired = gen.Error{
		Code:    "place_required",
		Message: "Выберите место группы",
	}
	errCannotInviteSelf = gen.Error{
		Code:    "cannot_invite_self",
		Message: "Вы уже в группе",
	}
	errNotGroupOwner = gen.Error{
		Code:    "not_group_owner",
		Message: "Это может только создатель группы",
	}
	errGroupNotFound = gen.Error{
		Code:    "group_not_found",
		Message: "Группы больше нет",
	}
	errOwnerCannotLeave = gen.Error{
		Code:    "owner_cannot_leave",
		Message: "Создатель не может выйти из своей группы — только удалить её",
	}
)
