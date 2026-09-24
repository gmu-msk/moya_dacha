package tests

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"unicode/utf8"
)

// Тесты уведомлений (specs/014-notifications.md). Написаны по спецификации
// и контракту, не глядя в реализацию (ADR-0002).

// Виды событий из контракта (schema Notification, поле kind).
const (
	kindFollow         = "follow"
	kindFollowAccepted = "follow_accepted"
	kindLike           = "like"
	kindComment        = "comment"
)

// notificationCommentLimit — сколько символов комментария попадает
// в строку уведомления (ФТ-1, «API / контракт данных»).
const notificationCommentLimit = 100

// --- Представления из контракта -------------------------------------------

// notificationPostPayload — пост в строке уведомления (schema
// NotificationPost).
type notificationPostPayload struct {
	ID        string        `json:"id"`
	Thumbnail *mediaPayload `json:"thumbnail"`
}

// notificationPayload — строка раздела (schema Notification). Необязательные
// поля — указатели: их отсутствие отличимо от нулевого значения.
type notificationPayload struct {
	ID        string                   `json:"id"`
	Kind      string                   `json:"kind"`
	CreatedAt string                   `json:"created_at"`
	Actor     *followUserPayload       `json:"actor"`
	Others    *int                     `json:"others"`
	Post      *notificationPostPayload `json:"post"`
	Comment   *string                  `json:"comment"`
	Unread    *bool                    `json:"unread"`
}

// notificationListPayload — страница событий (schema NotificationList).
// Конец — отсутствие курсора.
type notificationListPayload struct {
	Items      []notificationPayload `json:"items"`
	NextCursor *string               `json:"next_cursor"`
}

// unreadPayload — счётчики непрочитанного (schema UnreadNotifications).
// Оба поля обязательны, поэтому указатели.
type unreadPayload struct {
	Unread   *int `json:"unread"`
	Requests *int `json:"requests"`
}

// --- Хелперы --------------------------------------------------------------

// fetchNotificationsRaw запрашивает события с готовой строкой запроса:
// тестам про мусорные параметры нужна именно строка.
func fetchNotificationsRaw(t *testing.T, baseURL, token, query string) *http.Response {
	t.Helper()

	address := baseURL + "/me/notifications"
	if query != "" {
		address += "?" + query
	}

	return do(t, http.MethodGet, address, token, nil)
}

// notificationsPage требует, чтобы страница событий отдалась, и разбирает её.
func notificationsPage(t *testing.T, baseURL, token string, params url.Values) notificationListPayload {
	t.Helper()

	resp := fetchNotificationsRaw(t, baseURL, token, params.Encode())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /me/notifications: ожидался статус 200, получен %d", resp.StatusCode)
	}

	var page notificationListPayload
	decode(t, resp, &page)

	if page.Items == nil {
		t.Fatalf("GET /me/notifications: поле items обязательно, даже пустое")
	}

	return page
}

// notificationsOf — все события пользователя одной страницей (limit 50):
// тестам про вид событий их меньше.
func notificationsOf(t *testing.T, baseURL string, who dachnik) []notificationPayload {
	t.Helper()

	page := notificationsPage(t, baseURL, who.token, feedParams(50, ""))
	if page.NextCursor != nil {
		t.Fatalf("событий больше 50, а тест ждал одну страницу")
	}

	return page.Items
}

// walkNotifications проходит события страницами по limit и возвращает
// идентификаторы строк подряд. Требует, чтобы проход был конечен
// и страницы не превышали limit.
func walkNotifications(t *testing.T, baseURL, token string, limit int) []string {
	t.Helper()

	var ids []string
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 100 {
			t.Fatalf("проход событий по %d не кончается", limit)
		}

		page := notificationsPage(t, baseURL, token, feedParams(limit, cursor))
		if len(page.Items) > limit {
			t.Fatalf("на странице %d событий при limit=%d", len(page.Items), limit)
		}
		for _, item := range page.Items {
			ids = append(ids, item.ID)
		}

		if page.NextCursor == nil {
			return ids
		}
		if *page.NextCursor == "" {
			t.Fatalf("курсор на продолжение пустой")
		}
		if len(page.Items) == 0 {
			t.Fatalf("пустая страница пришла с курсором")
		}
		cursor = *page.NextCursor
	}
}

// fetchUnread спрашивает счётчики непрочитанного.
func fetchUnread(t *testing.T, baseURL, token string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, baseURL+"/me/notifications/unread", token, nil)
}

// requireUnread требует ровно таких счётчиков непрочитанного.
func requireUnread(t *testing.T, baseURL string, who dachnik, unread, requests int, where string) {
	t.Helper()

	resp := fetchUnread(t, baseURL, who.token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: GET /me/notifications/unread: ожидался статус 200, получен %d", where, resp.StatusCode)
	}

	var body unreadPayload
	decode(t, resp, &body)

	if body.Unread == nil || body.Requests == nil {
		t.Fatalf("%s: в ответе нет unread или requests: %+v", where, body)
	}
	if *body.Unread != unread || *body.Requests != requests {
		t.Errorf("%s: unread=%d requests=%d, ожидалось unread=%d requests=%d",
			where, *body.Unread, *body.Requests, unread, requests)
	}
}

// markSeen отмечает всё прочитанным.
func markSeen(t *testing.T, baseURL, token string) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/me/notifications/seen", token, nil)
}

// markSeenOK отмечает всё прочитанным и требует 204.
func markSeenOK(t *testing.T, baseURL string, who dachnik) {
	t.Helper()
	requireNoContent(t, markSeen(t, baseURL, who.token), "PUT /me/notifications/seen")
}

// requireKinds требует ровно такой последовательности видов событий.
func requireKinds(t *testing.T, items []notificationPayload, want []string, where string) {
	t.Helper()

	got := make([]string, 0, len(items))
	for _, item := range items {
		got = append(got, item.Kind)
	}
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: события %v, ожидались %v", where, got, want)
	}
}

// requireNoNotifications требует пустого раздела.
func requireNoNotifications(t *testing.T, baseURL string, who dachnik, where string) {
	t.Helper()
	requireKinds(t, notificationsOf(t, baseURL, who), nil, where)
}

// onlyNotification требует ровно одно событие этого вида и возвращает его.
func onlyNotification(t *testing.T, baseURL string, who dachnik, kind, where string) notificationPayload {
	t.Helper()

	items := notificationsOf(t, baseURL, who)
	requireKinds(t, items, []string{kind}, where)
	requireNotificationShape(t, items[0], where)

	return items[0]
}

// requireNotificationShape проверяет обязательные поля строки.
func requireNotificationShape(t *testing.T, n notificationPayload, where string) {
	t.Helper()

	if n.ID == "" {
		t.Errorf("%s: у строки нет id", where)
	}
	if n.CreatedAt == "" {
		t.Errorf("%s: у строки нет created_at", where)
	}
	if n.Actor == nil {
		t.Fatalf("%s: у строки нет actor", where)
	}
	if n.Actor.ID == "" || n.Actor.Nickname == "" {
		t.Errorf("%s: у actor нет id или ника: %+v", where, *n.Actor)
	}
	if n.Unread == nil {
		t.Errorf("%s: у строки нет unread", where)
	}
}

// requireActor требует, чтобы строку сделал именно этот человек.
func requireActor(t *testing.T, n notificationPayload, actorID, where string) {
	t.Helper()

	if n.Actor == nil {
		t.Fatalf("%s: у строки нет actor", where)
	}
	if n.Actor.ID != actorID {
		t.Errorf("%s: actor = %s, ожидался %s", where, n.Actor.ID, actorID)
	}
}

// othersOf — поле others строки лайков. Поле необязательное в контракте,
// поэтому его отсутствие читается как 0.
func othersOf(n notificationPayload) int {
	if n.Others == nil {
		return 0
	}
	return *n.Others
}

// requireLikeRow требует строку лайков этого поста с этим последним
// отметившим и таким числом остальных.
func requireLikeRow(t *testing.T, n notificationPayload, post postPayload, actorID string, others int, where string) {
	t.Helper()

	if n.Kind != kindLike {
		t.Fatalf("%s: kind = %q, ожидался like", where, n.Kind)
	}
	requireActor(t, n, actorID, where)
	if got := othersOf(n); got != others {
		t.Errorf("%s: others = %d, ожидалось %d", where, got, others)
	}
	requirePostThumbnail(t, n, post, where)
}

// requirePostThumbnail требует, чтобы в строке был этот пост с миниатюрой —
// первым его медиа.
func requirePostThumbnail(t *testing.T, n notificationPayload, post postPayload, where string) {
	t.Helper()

	if n.Post == nil {
		t.Fatalf("%s: у строки нет post", where)
	}
	if n.Post.ID != post.ID {
		t.Errorf("%s: post.id = %s, ожидался %s", where, n.Post.ID, post.ID)
	}
	if n.Post.Thumbnail == nil {
		t.Fatalf("%s: у поста строки нет thumbnail", where)
	}
	thumb := *n.Post.Thumbnail
	if thumb.URL == "" {
		t.Errorf("%s: у миниатюры пустой url", where)
	}
	if thumb.Width <= 0 || thumb.Height <= 0 {
		t.Errorf("%s: у миниатюры размеры %dx%d", where, thumb.Width, thumb.Height)
	}
	if len(post.Media) > 0 {
		cover := post.Media[0]
		if thumb.ID != cover.ID || thumb.URL != cover.URL || thumb.Width != cover.Width || thumb.Height != cover.Height {
			t.Errorf("%s: миниатюра %+v, ожидалось первое медиа поста %+v", where, thumb, cover)
		}
	}
}

// requireUnreadFlag требует, чтобы у всех строк unread было таким.
func requireUnreadFlag(t *testing.T, items []notificationPayload, want bool, where string) {
	t.Helper()

	for i, item := range items {
		if item.Unread == nil {
			t.Errorf("%s: у строки %d нет unread", where, i)
		} else if *item.Unread != want {
			t.Errorf("%s: у строки %d (%s) unread = %v, ожидалось %v", where, i, item.Kind, *item.Unread, want)
		}
	}
}

// likeOK ставит лайк и требует успеха.
func likeOK(t *testing.T, baseURL string, who dachnik, postID string) {
	t.Helper()

	if resp := likePost(t, baseURL, who.token, postID); resp.StatusCode != http.StatusOK {
		t.Fatalf("лайк: ожидался статус 200, получен %d", resp.StatusCode)
	}
}

// unlikeOK снимает лайк и требует успеха.
func unlikeOK(t *testing.T, baseURL string, who dachnik, postID string) {
	t.Helper()

	if resp := unlikePost(t, baseURL, who.token, postID); resp.StatusCode != http.StatusOK {
		t.Fatalf("снятие лайка: ожидался статус 200, получен %d", resp.StatusCode)
	}
}

// ownPost публикует пост с одной фотографией от имени who.
func ownPost(t *testing.T, baseURL string, who dachnik) postPayload {
	t.Helper()
	return publishPost(t, baseURL, who.token, "Грядка с огурцами")
}

// --- Без токена -----------------------------------------------------------

// Все три ручки раздела — только для вошедшего.
func TestNotificationsRequireAuth(t *testing.T) {
	baseURL := startAPI(t)

	requireError(t, fetchNotificationsRaw(t, baseURL, "", ""), http.StatusUnauthorized, "unauthorized")
	requireError(t, fetchUnread(t, baseURL, ""), http.StatusUnauthorized, "unauthorized")
	requireError(t, markSeen(t, baseURL, ""), http.StatusUnauthorized, "unauthorized")
}

// --- Пустой раздел --------------------------------------------------------

// У новичка раздел пуст, непрочитанного нет («Событий нет»).
func TestNotificationsEmptyForNewcomer(t *testing.T) {
	baseURL := startAPI(t)
	a := newDachnik(t, baseURL, 1)

	page := notificationsPage(t, baseURL, a.token, nil)
	if len(page.Items) != 0 {
		t.Errorf("у новичка %d событий, ожидалось 0", len(page.Items))
	}
	if page.NextCursor != nil {
		t.Errorf("у пустого раздела курсор %q, ожидалось его отсутствие", *page.NextCursor)
	}
	requireUnread(t, baseURL, a, 0, 0, "новичок")
}

// --- follow ---------------------------------------------------------------

// Подписка на открытый профиль — событие follow у того, на кого
// подписались; actor несёт отношение смотрящего к подписавшемуся
// (ФТ-1: «Подписаться», если смотрящий ещё не подписан).
func TestNotificationFollowOnOpenProfile(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	followOK(t, baseURL, a, b.id)

	n := onlyNotification(t, baseURL, b, kindFollow, "B после подписки A")
	requireActor(t, n, a.id, "событие follow")
	requireRelation(t, n.Actor.Relation, followingNone, true, "actor события follow")
	if n.Post != nil {
		t.Errorf("у follow не должно быть post, пришёл %+v", *n.Post)
	}
	if n.Comment != nil {
		t.Errorf("у follow не должно быть comment, пришёл %q", *n.Comment)
	}

	requireNoNotifications(t, baseURL, a, "подписавшийся A")
}

// Встречная подписка меняет relation в строке: «Вы подписаны».
func TestNotificationFollowRelationAfterFollowBack(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	followOK(t, baseURL, a, b.id)
	followOK(t, baseURL, b, a.id)

	items := notificationsOf(t, baseURL, b)
	requireKinds(t, items, []string{kindFollow}, "B после взаимной подписки")
	requireRelation(t, items[0].Actor.Relation, followingYes, true, "actor события follow у B")
}

// Подписался и сразу отписался — строки нет (ФТ-4).
func TestNotificationFollowGoneAfterUnfollow(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]

	followOK(t, baseURL, a, b.id)
	onlyNotification(t, baseURL, b, kindFollow, "B после подписки")

	unfollowOK(t, baseURL, a, b.id)
	requireNoNotifications(t, baseURL, b, "B после отписки A")
	requireUnread(t, baseURL, b, 0, 0, "B после отписки A")
}

// --- Заявки и follow_accepted ---------------------------------------------

// Заявка к закрытому профилю — не событие: она в requests (ФТ-1, ФТ-5).
func TestNotificationRequestIsNotEvent(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]
	setClosed(t, baseURL, b.token, true)

	followOK(t, baseURL, a, b.id)

	requireNoNotifications(t, baseURL, b, "B с заявкой")
	requireUnread(t, baseURL, b, 0, 1, "B с заявкой")
	requireNoNotifications(t, baseURL, a, "заявитель A")
}

// Заявка принята — у заявителя follow_accepted, у хозяина ничего.
func TestNotificationFollowAccepted(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]
	setClosed(t, baseURL, b.token, true)

	followOK(t, baseURL, a, b.id)
	requireNoContent(t, acceptRequest(t, baseURL, b.token, a.id), "принятие заявки")

	n := onlyNotification(t, baseURL, a, kindFollowAccepted, "заявитель A")
	requireActor(t, n, b.id, "событие follow_accepted")
	requireRelation(t, n.Actor.Relation, followingYes, false, "actor события follow_accepted")
	requireUnread(t, baseURL, a, 1, 0, "заявитель A")

	requireNoNotifications(t, baseURL, b, "хозяин B")
	requireUnread(t, baseURL, b, 0, 0, "хозяин B")
}

// Отклонённая заявка ничего не порождает.
func TestNotificationDeclinedRequestLeavesNothing(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	a, b := people[0], people[1]
	setClosed(t, baseURL, b.token, true)

	followOK(t, baseURL, a, b.id)
	requireNoContent(t, declineRequest(t, baseURL, b.token, a.id), "отклонение заявки")

	requireNoNotifications(t, baseURL, a, "заявитель A")
	requireNoNotifications(t, baseURL, b, "хозяин B")
	requireUnread(t, baseURL, b, 0, 0, "хозяин B")
}

// Открыл профиль — заявки приняты сами, у каждого заявителя follow_accepted.
func TestNotificationOpenProfileAcceptsRequests(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	owner, a, b := people[0], people[1], people[2]
	setClosed(t, baseURL, owner.token, true)

	followOK(t, baseURL, a, owner.id)
	followOK(t, baseURL, b, owner.id)
	requireUnread(t, baseURL, owner, 0, 2, "хозяин с двумя заявками")

	setClosed(t, baseURL, owner.token, false)

	for _, who := range []dachnik{a, b} {
		n := onlyNotification(t, baseURL, who, kindFollowAccepted, "заявитель после открытия профиля")
		requireActor(t, n, owner.id, "follow_accepted после открытия профиля")
	}
	requireUnread(t, baseURL, owner, 0, 0, "хозяин после открытия профиля")
}

// --- like -----------------------------------------------------------------

// Лайк чужого поста — строка like у автора с постом и миниатюрой.
func TestNotificationLike(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, a := people[0], people[1]
	post := ownPost(t, baseURL, author)

	likeOK(t, baseURL, a, post.ID)

	n := onlyNotification(t, baseURL, author, kindLike, "автор после лайка")
	requireLikeRow(t, n, post, a.id, 0, "строка лайка")
	requireRelation(t, n.Actor.Relation, followingNone, false, "actor строки лайка")
	if n.Comment != nil {
		t.Errorf("у like не должно быть comment, пришёл %q", *n.Comment)
	}

	requireNoNotifications(t, baseURL, a, "отметивший")
}

// Три лайка одного поста — одна строка: последний отметивший «и ещё 2».
func TestNotificationLikesMerge(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 4)
	author, likers := people[0], people[1:]
	post := ownPost(t, baseURL, author)

	for _, who := range likers {
		likeOK(t, baseURL, who, post.ID)
	}

	n := onlyNotification(t, baseURL, author, kindLike, "автор после трёх лайков")
	requireLikeRow(t, n, post, likers[2].id, 2, "слитая строка")
	requireUnread(t, baseURL, author, 1, 0, "слитая строка — одна непрочитанная")
}

// Лайки разных постов — разные строки.
func TestNotificationLikesOfDifferentPostsSeparate(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, a := people[0], people[1]
	first := ownPost(t, baseURL, author)
	second := ownPost(t, baseURL, author)

	likeOK(t, baseURL, a, first.ID)
	likeOK(t, baseURL, a, second.ID)

	items := notificationsOf(t, baseURL, author)
	requireKinds(t, items, []string{kindLike, kindLike}, "лайки двух постов")
	requireLikeRow(t, items[0], second, a.id, 0, "новая строка — второй пост")
	requireLikeRow(t, items[1], first, a.id, 0, "старая строка — первый пост")
}

// Лайк снят — человек уходит из строки (ФТ-4).
func TestNotificationUnlikeLeavesMergedRow(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 4)
	author, a, b, c := people[0], people[1], people[2], people[3]
	post := ownPost(t, baseURL, author)

	likeOK(t, baseURL, a, post.ID)
	likeOK(t, baseURL, b, post.ID)
	likeOK(t, baseURL, c, post.ID)

	unlikeOK(t, baseURL, c, post.ID)
	n := onlyNotification(t, baseURL, author, kindLike, "после снятия последнего лайка")
	requireLikeRow(t, n, post, b.id, 1, "последний снял — actor предыдущий")

	unlikeOK(t, baseURL, a, post.ID)
	n = onlyNotification(t, baseURL, author, kindLike, "после снятия первого лайка")
	requireLikeRow(t, n, post, b.id, 0, "остался один")
}

// Единственный лайк снят — строки нет.
func TestNotificationUnlikeOnlyLikeRemovesRow(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, a := people[0], people[1]
	post := ownPost(t, baseURL, author)

	likeOK(t, baseURL, a, post.ID)
	unlikeOK(t, baseURL, a, post.ID)

	requireNoNotifications(t, baseURL, author, "после снятия единственного лайка")
	requireUnread(t, baseURL, author, 0, 0, "после снятия единственного лайка")
}

// Свои действия уведомлений не порождают (ФТ-3).
func TestNotificationOwnActionsSilent(t *testing.T) {
	baseURL := startAPI(t)
	author := newDachnik(t, baseURL, 1)
	post := ownPost(t, baseURL, author)

	likeOK(t, baseURL, author, post.ID)
	commentOf(t, baseURL, author.token, post.ID, "Сам себе отвечу")

	requireNoNotifications(t, baseURL, author, "после своего лайка и комментария")
	requireUnread(t, baseURL, author, 0, 0, "после своего лайка и комментария")
}

// Свой лайк не входит в слитую строку чужих лайков.
func TestNotificationOwnLikeNotInMergedRow(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, a := people[0], people[1]
	post := ownPost(t, baseURL, author)

	likeOK(t, baseURL, a, post.ID)
	likeOK(t, baseURL, author, post.ID)

	n := onlyNotification(t, baseURL, author, kindLike, "чужой и свой лайк")
	requireLikeRow(t, n, post, a.id, 0, "в строке только чужой лайк")
}

// --- comment --------------------------------------------------------------

// Комментарий — строка comment с постом и текстом.
func TestNotificationComment(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, a := people[0], people[1]
	post := ownPost(t, baseURL, author)
	const text = "Какие огурцы! Сорт какой?"

	commentOf(t, baseURL, a.token, post.ID, text)

	n := onlyNotification(t, baseURL, author, kindComment, "автор после комментария")
	requireActor(t, n, a.id, "строка комментария")
	requirePostThumbnail(t, n, post, "строка комментария")
	if n.Comment == nil {
		t.Fatalf("у строки comment нет поля comment")
	}
	if *n.Comment != text {
		t.Errorf("comment = %q, ожидалось %q", *n.Comment, text)
	}
	if n.Others != nil && *n.Others != 0 {
		t.Errorf("у comment others = %d, поле только у like", *n.Others)
	}

	requireNoNotifications(t, baseURL, a, "комментатор")
}

// Длинный комментарий обрезается до 100 символов — символов, не байт.
func TestNotificationCommentTruncatedByRunes(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, a := people[0], people[1]
	post := ownPost(t, baseURL, author)
	text := repeatRunes("Щавель ёжик ", 150)

	commentOf(t, baseURL, a.token, post.ID, text)

	n := onlyNotification(t, baseURL, author, kindComment, "автор после длинного комментария")
	if n.Comment == nil {
		t.Fatalf("у строки comment нет поля comment")
	}
	got := *n.Comment
	if !utf8.ValidString(got) {
		t.Fatalf("comment — не UTF-8: обрезан посреди буквы? %q", got)
	}
	want := string([]rune(text)[:notificationCommentLimit])
	if got != want {
		t.Errorf("comment = %q (%d символов), ожидались первые %d символов %q",
			got, utf8.RuneCountInString(got), notificationCommentLimit, want)
	}
}

// Комментарий ровно в 100 символов приходит целиком.
func TestNotificationCommentAtLimitWhole(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, a := people[0], people[1]
	post := ownPost(t, baseURL, author)
	text := repeatRunes("Укроп", notificationCommentLimit)

	commentOf(t, baseURL, a.token, post.ID, text)

	n := onlyNotification(t, baseURL, author, kindComment, "комментарий в 100 символов")
	if n.Comment == nil || *n.Comment != text {
		t.Errorf("comment = %v, ожидался текст целиком %q", n.Comment, text)
	}
}

// Комментарий удалён — строки нет (ФТ-4).
func TestNotificationCommentGoneAfterDelete(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, a := people[0], people[1]
	post := ownPost(t, baseURL, author)

	comment := commentOf(t, baseURL, a.token, post.ID, "Удалю через минуту")
	onlyNotification(t, baseURL, author, kindComment, "до удаления")

	requireDeleted(t, deleteComment(t, baseURL, a.token, post.ID, comment.ID))

	requireNoNotifications(t, baseURL, author, "после удаления комментария")
	requireUnread(t, baseURL, author, 0, 0, "после удаления комментария")
}

// Каждый комментарий — своя строка, в отличие от лайков.
func TestNotificationCommentsNotMerged(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, a := people[0], people[1]
	post := ownPost(t, baseURL, author)

	commentOf(t, baseURL, a.token, post.ID, "Первый")
	commentOf(t, baseURL, a.token, post.ID, "Второй")

	items := notificationsOf(t, baseURL, author)
	requireKinds(t, items, []string{kindComment, kindComment}, "два комментария")
	if items[0].Comment == nil || *items[0].Comment != "Второй" {
		t.Errorf("сверху ожидался второй комментарий, пришёл %v", items[0].Comment)
	}
	if items[1].Comment == nil || *items[1].Comment != "Первый" {
		t.Errorf("снизу ожидался первый комментарий, пришёл %v", items[1].Comment)
	}
}

// --- Удаление поста -------------------------------------------------------

// Пост удалён — все его события пропали, остальные на месте (ФТ-4).
func TestNotificationsGoneWithPost(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	author, a, b := people[0], people[1], people[2]
	doomed := ownPost(t, baseURL, author)
	kept := ownPost(t, baseURL, author)

	followOK(t, baseURL, a, author.id)
	likeOK(t, baseURL, a, doomed.ID)
	likeOK(t, baseURL, b, doomed.ID)
	commentOf(t, baseURL, b.token, doomed.ID, "Пропадёт вместе с постом")
	likeOK(t, baseURL, b, kept.ID)

	requireKinds(t, notificationsOf(t, baseURL, author),
		[]string{kindLike, kindComment, kindLike, kindFollow}, "до удаления поста")

	requireDeleted(t, deletePost(t, baseURL, author.token, doomed.ID))

	items := notificationsOf(t, baseURL, author)
	requireKinds(t, items, []string{kindLike, kindFollow}, "после удаления поста")
	requireLikeRow(t, items[0], kept, b.id, 0, "лайк оставшегося поста")
	requireUnread(t, baseURL, author, 2, 0, "после удаления поста")
}

// --- Порядок и страницы ---------------------------------------------------

// События разных видов — новые сверху; слитая строка лайков стоит по
// времени последнего лайка (ФТ-1, ФТ-8).
func TestNotificationsNewestFirst(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	author, a, b := people[0], people[1], people[2]
	post := ownPost(t, baseURL, author)

	likeOK(t, baseURL, a, post.ID)
	followOK(t, baseURL, a, author.id)
	commentOf(t, baseURL, a.token, post.ID, "Хороши!")

	requireKinds(t, notificationsOf(t, baseURL, author),
		[]string{kindComment, kindFollow, kindLike}, "лайк, подписка, комментарий")

	// Новый лайк того же поста поднимает слитую строку наверх.
	likeOK(t, baseURL, b, post.ID)

	items := notificationsOf(t, baseURL, author)
	requireKinds(t, items, []string{kindLike, kindComment, kindFollow}, "после второго лайка")
	requireLikeRow(t, items[0], post, b.id, 1, "поднятая строка лайков")
}

// Страницы по курсору: проход по limit даёт то же, что одна большая
// страница, последняя страница — без курсора.
func TestNotificationsPagination(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 6)
	owner, followers := people[0], people[1:]

	for _, who := range followers {
		followOK(t, baseURL, who, owner.id)
	}

	all := notificationsOf(t, baseURL, owner)
	if len(all) != len(followers) {
		t.Fatalf("событий %d, ожидалось %d", len(all), len(followers))
	}
	for i, item := range all {
		want := followers[len(followers)-1-i].id
		requireActor(t, item, want, fmt.Sprintf("строка %d: новые сверху", i))
	}

	wantIDs := make([]string, 0, len(all))
	for _, item := range all {
		wantIDs = append(wantIDs, item.ID)
	}

	for _, limit := range []int{1, 2, 5} {
		if got := walkNotifications(t, baseURL, owner.token, limit); !reflect.DeepEqual(got, wantIDs) {
			t.Errorf("проход по %d: %v, ожидалось %v", limit, got, wantIDs)
		}
	}

	first := notificationsPage(t, baseURL, owner.token, feedParams(2, ""))
	if len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("первая страница по 2: %d событий, курсор %v", len(first.Items), first.NextCursor)
	}
	// Событий ровно limit — страница целиком, и курсора нет: дальше пусто.
	exact := notificationsPage(t, baseURL, owner.token, feedParams(5, ""))
	if len(exact.Items) != 5 {
		t.Fatalf("страница по 5 из 5 событий: %d событий", len(exact.Items))
	}
	if exact.NextCursor != nil {
		t.Errorf("страница ровно на все события пришла с курсором %q", *exact.NextCursor)
	}

	second := notificationsPage(t, baseURL, owner.token, feedParams(3, cursorOfNotifications(t, notificationsPage(t, baseURL, owner.token, feedParams(3, "")))))
	if len(second.Items) != 2 {
		t.Errorf("вторая страница по 3 из 5: %d событий, ожидалось 2", len(second.Items))
	}
	if second.NextCursor != nil {
		t.Errorf("на последней странице курсор %q", *second.NextCursor)
	}
}

// cursorOfNotifications требует курсор на продолжение и возвращает его.
func cursorOfNotifications(t *testing.T, page notificationListPayload) string {
	t.Helper()

	if page.NextCursor == nil || *page.NextCursor == "" {
		t.Fatalf("ожидался курсор на продолжение, его нет")
	}

	return *page.NextCursor
}

// Размер страницы по умолчанию — 20.
func TestNotificationsDefaultLimit(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	author, a := people[0], people[1]
	post := ownPost(t, baseURL, author)

	for i := 0; i < 21; i++ {
		commentOf(t, baseURL, a.token, post.ID, fmt.Sprintf("Комментарий %d", i))
	}

	page := notificationsPage(t, baseURL, author.token, nil)
	if len(page.Items) != 20 {
		t.Errorf("страница без limit: %d событий, ожидалось 20", len(page.Items))
	}
	if page.NextCursor == nil {
		t.Errorf("после 20 из 21 события курсора нет")
	}
}

// limit вне 1..50 и не число — 400 invalid_request; мусорный курсор —
// 400 invalid_cursor.
func TestNotificationsBadParams(t *testing.T) {
	baseURL := startAPI(t)
	a := newDachnik(t, baseURL, 1)

	for _, query := range []string{"limit=0", "limit=51", "limit=-1", "limit=abc"} {
		t.Run(query, func(t *testing.T) {
			requireError(t, fetchNotificationsRaw(t, baseURL, a.token, query), http.StatusBadRequest, "invalid_request")
		})
	}

	for _, query := range []string{"limit=1", "limit=50"} {
		if resp := fetchNotificationsRaw(t, baseURL, a.token, query); resp.StatusCode != http.StatusOK {
			t.Errorf("%s: ожидался статус 200, получен %d", query, resp.StatusCode)
		}
	}

	requireError(t, fetchNotificationsRaw(t, baseURL, a.token, "cursor=%D0%BC%D1%83%D1%81%D0%BE%D1%80"),
		http.StatusBadRequest, "invalid_cursor")
	requireError(t, fetchNotificationsRaw(t, baseURL, a.token, "cursor=not-a-cursor"),
		http.StatusBadRequest, "invalid_cursor")
}

// --- Непрочитанное --------------------------------------------------------

// unread считает строки новее последнего открытия: слитая строка лайков —
// одна; чтение списка ничего не отмечает; PUT seen обнуляет счётчик
// и гасит unread у строк (ФТ-5, ФТ-7).
func TestNotificationsUnreadAndSeen(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 4)
	author, a, b, c := people[0], people[1], people[2], people[3]
	post := ownPost(t, baseURL, author)

	likeOK(t, baseURL, a, post.ID)
	likeOK(t, baseURL, b, post.ID)
	likeOK(t, baseURL, c, post.ID)
	followOK(t, baseURL, a, author.id)
	commentOf(t, baseURL, b.token, post.ID, "Отличный урожай")

	requireUnread(t, baseURL, author, 3, 0, "три строки: лайки, подписка, комментарий")
	items := notificationsOf(t, baseURL, author)
	requireUnreadFlag(t, items, true, "до открытия раздела")
	requireUnread(t, baseURL, author, 3, 0, "чтение списка ничего не отмечает")

	markSeenOK(t, baseURL, author)

	requireUnread(t, baseURL, author, 0, 0, "после PUT seen")
	items = notificationsOf(t, baseURL, author)
	if len(items) != 3 {
		t.Fatalf("после PUT seen строк %d, ожидалось 3: события не пропадают", len(items))
	}
	requireUnreadFlag(t, items, false, "после PUT seen")

	// Повторное открытие — тоже 204 и ничего не меняет.
	markSeenOK(t, baseURL, author)
	requireUnread(t, baseURL, author, 0, 0, "после второго PUT seen")
}

// После открытия раздела новое событие снова непрочитанное — и только оно.
func TestNotificationNewEventAfterSeenIsUnread(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	author, a, b := people[0], people[1], people[2]
	post := ownPost(t, baseURL, author)

	followOK(t, baseURL, a, author.id)
	markSeenOK(t, baseURL, author)

	commentOf(t, baseURL, b.token, post.ID, "Новенькое")

	requireUnread(t, baseURL, author, 1, 0, "комментарий после открытия")
	items := notificationsOf(t, baseURL, author)
	requireKinds(t, items, []string{kindComment, kindFollow}, "после открытия и комментария")
	requireUnreadFlag(t, items[:1], true, "новый комментарий")
	requireUnreadFlag(t, items[1:], false, "старая подписка")
}

// Новый лайк к уже прочитанной слитой строке делает её снова непрочитанной.
func TestNotificationMergedLikeUnreadAgain(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	author, a, b := people[0], people[1], people[2]
	post := ownPost(t, baseURL, author)

	likeOK(t, baseURL, a, post.ID)
	markSeenOK(t, baseURL, author)
	requireUnread(t, baseURL, author, 0, 0, "после открытия")

	likeOK(t, baseURL, b, post.ID)

	requireUnread(t, baseURL, author, 1, 0, "новый лайк к прочитанной строке")
	n := onlyNotification(t, baseURL, author, kindLike, "слитая строка")
	requireLikeRow(t, n, post, b.id, 1, "слитая строка после нового лайка")
	requireUnreadFlag(t, []notificationPayload{n}, true, "слитая строка после нового лайка")
}

// Ждущие заявки остаются непрочитанными и после открытия раздела; ответ
// на заявку убирает её из счётчика.
func TestNotificationRequestsStayAfterSeen(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	owner, a, b := people[0], people[1], people[2]
	setClosed(t, baseURL, owner.token, true)

	followOK(t, baseURL, a, owner.id)
	followOK(t, baseURL, b, owner.id)
	requireUnread(t, baseURL, owner, 0, 2, "две заявки")

	markSeenOK(t, baseURL, owner)
	requireUnread(t, baseURL, owner, 0, 2, "две заявки после открытия")

	requireNoContent(t, acceptRequest(t, baseURL, owner.token, a.id), "принятие заявки A")
	requireUnread(t, baseURL, owner, 0, 1, "после принятия одной")

	requireNoContent(t, declineRequest(t, baseURL, owner.token, b.id), "отклонение заявки B")
	requireUnread(t, baseURL, owner, 0, 0, "после ответа на обе")
}

// Заявитель отменил заявку — она ушла из счётчика.
func TestNotificationCancelledRequestLeavesCounter(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 2)
	owner, a := people[0], people[1]
	setClosed(t, baseURL, owner.token, true)

	followOK(t, baseURL, a, owner.id)
	requireUnread(t, baseURL, owner, 0, 1, "заявка")

	unfollowOK(t, baseURL, a, owner.id)
	requireUnread(t, baseURL, owner, 0, 0, "после отмены заявки")
}

// --- Срок хранения --------------------------------------------------------

// Событие старше 90 дней не отдаётся и не считается (ФТ-8). Сервис сам
// такого не создаст, поэтому событие старится прямо в таблице из «Модели
// данных».
func TestNotificationsOlderThan90DaysGone(t *testing.T) {
	baseURL := startAPI(t)
	people := newDachniks(t, baseURL, 1, 3)
	owner, a, b := people[0], people[1], people[2]

	followOK(t, baseURL, a, owner.id)
	if affected := execSQL(t,
		`UPDATE notifications SET created_at = now() - interval '91 days' WHERE user_id = $1 AND actor_id = $2`,
		owner.id, a.id); affected != 1 {
		t.Fatalf("событие не состарилось: изменено строк %d", affected)
	}
	followOK(t, baseURL, b, owner.id)

	n := onlyNotification(t, baseURL, owner, kindFollow, "после старения события")
	requireActor(t, n, b.id, "осталось только свежее событие")
	requireUnread(t, baseURL, owner, 1, 0, "старое событие не считается")
}
