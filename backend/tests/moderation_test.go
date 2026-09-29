package tests

// Тесты модерации с дашборда владельца (specs/023-moderation.md).
//
// Как и сам дашборд (016), ручки модерации в контракт приложения не входят:
// адреса /dashboard/posts/… и /dashboard/comments/… висят рядом с /api,
// а форма поля `moderation` описана в спецификации, раздел «API / контракт
// данных». Посты, комментарии и жалобы тест создаёт так же, как приложение, —
// через /api; время появления и время жалобы через API не задать, поэтому
// там, где важен порядок, тест проставляет его прямо в таблицах posts,
// comments и reports.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// --- Форма ответа ---------------------------------------------------------

// modAuthor — автор элемента модерации: {id, nickname, name} (ФТ-4).
type modAuthor struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Name     string `json:"name"`
}

// modItem — пост или комментарий в списках reported и recent (ФТ-4).
type modItem struct {
	Kind         string    `json:"kind"`
	PostID       string    `json:"post_id"`
	CommentID    *string   `json:"comment_id"`
	Author       modAuthor `json:"author"`
	Text         string    `json:"text"`
	PhotoURL     *string   `json:"photo_url"`
	CreatedAt    string    `json:"created_at"`
	Reports      int       `json:"reports"`
	LastReportAt *string   `json:"last_report_at"`
	Reasons      []string  `json:"reasons"`
}

// modPayload — то из /dashboard/data, что нужно тестам модерации.
type modPayload struct {
	Totals     dashTotals `json:"totals"`
	Moderation struct {
		Reported []modItem `json:"reported"`
		Recent   []modItem `json:"recent"`
	} `json:"moderation"`
}

// --- Хелперы --------------------------------------------------------------

// modPhone — номер n-го участника теста модерации. Номера не пересекаются
// с номерами других тестов пакета.
func modPhone(n int) string {
	return fmt.Sprintf("+7 (900) 823-00-%02d", n)
}

// modUser — вошедший участник теста модерации.
type modUser struct {
	token    string
	id       string
	nickname string
	name     string
}

// newModUser входит по n-му номеру и выбирает никнейм и имя: автор
// в элементе модерации сверяется по ним (ФТ-4).
func newModUser(t *testing.T, baseURL string, n int, nickname, name string) modUser {
	t.Helper()

	token, id := signIn(t, baseURL, modPhone(n))

	if resp := setNickname(t, baseURL, token, map[string]any{"nickname": nickname}); resp.StatusCode != http.StatusOK {
		t.Fatalf("никнейм %q не выбрался: статус %d", nickname, resp.StatusCode)
	}
	if resp := do(t, http.MethodPut, baseURL+"/me", token, map[string]any{"name": name}); resp.StatusCode != http.StatusOK {
		t.Fatalf("имя %q не сохранилось: статус %d", name, resp.StatusCode)
	}

	return modUser{token: token, id: id, nickname: nickname, name: name}
}

// modRequest отправляет запрос на адрес дашборда. user и password — HTTP
// Basic (не ставится, если оба пусты), bearer — токен приложения
// (не ставится, если пуст).
func modRequest(t *testing.T, method, address, user, password, bearer string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(method, address, nil)
	if err != nil {
		t.Fatalf("не удалось собрать запрос %s %s: %v", method, address, err)
	}
	if user != "" || password != "" {
		req.SetBasicAuth(user, password)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("запрос %s %s не прошёл: %v", method, address, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// modDelete отправляет DELETE от владельца — с верным паролем дашборда.
// path — от корня сервиса, например "/dashboard/posts/…".
func modDelete(t *testing.T, root, path string) *http.Response {
	t.Helper()
	return modRequest(t, http.MethodDelete, root+path, "владелец", dashboardPassword, "")
}

// modRequireStatus требует статуса и заголовка Cache-Control: no-store
// (ФТ-1), а у 204 — ещё и пустого тела («API / контракт данных»).
func modRequireStatus(t *testing.T, resp *http.Response, want int, where string) {
	t.Helper()

	if resp.StatusCode != want {
		t.Fatalf("%s: ожидался статус %d, получен %d", where, want, resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("%s: ответ дашборда должен быть Cache-Control: no-store, получено %q", where, cc)
	}
	if want == http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("%s: не удалось прочитать тело ответа: %v", where, err)
		}
		if len(body) != 0 {
			t.Errorf("%s: у ответа 204 тело должно быть пустым, пришло %q", where, body)
		}
	}
}

// modDeleted удаляет от владельца и требует 204.
func modDeleted(t *testing.T, root, path, where string) {
	t.Helper()
	modRequireStatus(t, modDelete(t, root, path), http.StatusNoContent, where)
}

// modNotFound удаляет от владельца и требует 404.
func modNotFound(t *testing.T, root, path, where string) {
	t.Helper()
	modRequireStatus(t, modDelete(t, root, path), http.StatusNotFound, where)
}

// modData читает /dashboard/data и разбирает то, что нужно модерации.
func modData(t *testing.T, root string) modPayload {
	t.Helper()

	var body modPayload
	if err := json.Unmarshal(dashboardRaw(t, root), &body); err != nil {
		t.Fatalf("данные дашборда не разобрались как JSON: %v", err)
	}

	return body
}

// modRawModeration — поле moderation как есть, чтобы проверить не только
// значения, но и то, что пустое — это [] и null, а не пропущенное поле.
func modRawModeration(t *testing.T, root string) map[string]json.RawMessage {
	t.Helper()

	var top map[string]json.RawMessage
	if err := json.Unmarshal(dashboardRaw(t, root), &top); err != nil {
		t.Fatalf("данные дашборда не разобрались как JSON: %v", err)
	}
	raw, ok := top["moderation"]
	if !ok {
		t.Fatal("в /dashboard/data нет поля moderation (ФТ-3)")
	}

	var moderation map[string]json.RawMessage
	if err := json.Unmarshal(raw, &moderation); err != nil {
		t.Fatalf("поле moderation не объект: %s", raw)
	}

	return moderation
}

// modRawItems разбирает список элементов, не зная полей: для проверки,
// что поле есть и равно null, а не пропущено.
func modRawItems(t *testing.T, raw json.RawMessage, what string) []map[string]json.RawMessage {
	t.Helper()

	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("%s не разобрался как список объектов: %v (%s)", what, err, raw)
	}

	return items
}

// modKey — чем элемент отличается от других: вид и идентификатор.
func modKey(item modItem) string {
	if item.Kind == "comment" && item.CommentID != nil {
		return "comment:" + *item.CommentID
	}
	return item.Kind + ":" + item.PostID
}

func modPostKey(postID string) string       { return "post:" + postID }
func modCommentKey(commentID string) string { return "comment:" + commentID }

// modKeys — ключи элементов в том порядке, в котором они пришли.
func modKeys(items []modItem) []string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, modKey(item))
	}

	return keys
}

// modFind ищет элемент по ключу; nil, если его нет.
func modFind(items []modItem, key string) *modItem {
	for i := range items {
		if modKey(items[i]) == key {
			return &items[i]
		}
	}

	return nil
}

// modMustFind требует, чтобы элемент был в списке.
func modMustFind(t *testing.T, items []modItem, key, list string) modItem {
	t.Helper()

	item := modFind(items, key)
	if item == nil {
		t.Fatalf("в %s нет элемента %s; есть %v", list, key, modKeys(items))
	}

	return *item
}

// modRequireAbsent требует, чтобы элемента в списке не было.
func modRequireAbsent(t *testing.T, items []modItem, key, list, why string) {
	t.Helper()

	if modFind(items, key) != nil {
		t.Errorf("в %s остался элемент %s — %s", list, key, why)
	}
}

// modRequireNoReports требует, что у элемента жалоб нет: 0, null и [].
func modRequireNoReports(t *testing.T, item modItem, where string) {
	t.Helper()

	if item.Reports != 0 {
		t.Errorf("%s: reports = %d, ожидалось 0", where, item.Reports)
	}
	if item.LastReportAt != nil {
		t.Errorf("%s: last_report_at = %q, ожидался null — жалоб нет", where, *item.LastReportAt)
	}
	if item.Reasons == nil || len(item.Reasons) != 0 {
		t.Errorf("%s: reasons = %v, ожидался пустой список []", where, item.Reasons)
	}
}

// modReportsAt ставит жалобе этого человека на эту цель заданное время.
// column — post_id или comment_id.
func modReportsAt(t *testing.T, column, targetID, reporterID string, at time.Time) {
	t.Helper()

	if n := execSQL(t, `UPDATE reports SET created_at = $1 WHERE `+column+` = $2 AND reporter_id = $3`,
		at, targetID, reporterID); n != 1 {
		t.Fatalf("время жалобы не проставилось: изменено строк %d", n)
	}
}

// modCreatedAt ставит посту или комментарию время появления. table —
// posts или comments.
func modCreatedAt(t *testing.T, table, id string, at time.Time) {
	t.Helper()

	if n := execSQL(t, `UPDATE `+table+` SET created_at = $1 WHERE id = $2`, at, id); n != 1 {
		t.Fatalf("время появления в %s не проставилось: изменено строк %d", table, n)
	}
}

// modRequireTime требует, чтобы время из ответа совпало с ожидаемым
// с точностью до секунды.
func modRequireTime(t *testing.T, got *string, want time.Time, what string) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s: время null, ожидалось %s", what, want.Format(time.RFC3339))
	}
	at := parseDashTime(t, *got, what)
	if d := at.Sub(want); d > time.Second || d < -time.Second {
		t.Errorf("%s: время %s, ожидалось %s", what, *got, want.UTC().Format(time.RFC3339))
	}
}

// modUnread — сколько непрочитанных уведомлений у человека.
func modUnread(t *testing.T, baseURL, token string) int {
	t.Helper()

	resp := do(t, http.MethodGet, baseURL+"/me/notifications/unread", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на чтение непрочитанных уведомлений ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body struct {
		Unread int `json:"unread"`
	}
	decode(t, resp, &body)

	return body.Unread
}

// modPostWithVisibility публикует пост с одной фотографией и заданной
// видимостью (013).
func modPostWithVisibility(t *testing.T, baseURL, token, caption, visibility string) postPayload {
	t.Helper()

	photo := photoOf(t, baseURL, token, 50, 30)

	return createdPost(t, createPost(t, baseURL, token, map[string]any{
		"media_ids":  []string{photo.ID},
		"caption":    caption,
		"visibility": visibility,
	}))
}

// modAllPaths — четыре новые ручки для поста и комментария.
func modAllPaths(postID, commentID string) []string {
	return []string{
		"/dashboard/posts/" + postID,
		"/dashboard/comments/" + commentID,
		"/dashboard/posts/" + postID + "/reports",
		"/dashboard/comments/" + commentID + "/reports",
	}
}

// --- Доступ ---------------------------------------------------------------

// Без DASHBOARD_PASSWORD новых ручек нет: 404 при любом пароле, и ничего
// не удаляется (ФТ-1; 016, ФТ-1).
func TestModerationIsAbsentWithoutPassword(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{})
	root := strings.TrimSuffix(baseURL, "/api")

	author, _ := signIn(t, baseURL, modPhone(1))
	reader, _ := signIn(t, baseURL, modPhone(2))
	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, reader, post.ID, commentText)
	requireAccepted(t, reportPostReason(t, baseURL, reader, post.ID, reportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, author, post.ID, comment.ID, commentReportReason))

	for _, path := range modAllPaths(post.ID, comment.ID) {
		for _, password := range []string{"", dashboardPassword} {
			resp := modRequest(t, http.MethodDelete, root+path, "владелец", password, "")
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("DELETE %s без DASHBOARD_PASSWORD (пароль %q): ожидался статус 404, получен %d", path, password, resp.StatusCode)
			}
		}
	}

	requirePostAlive(t, baseURL, author, post.ID, "после попыток удаления без дашборда")
	if got := commentIDs(commentsOf(t, baseURL, author, post.ID).Items); !reflect.DeepEqual(got, []string{comment.ID}) {
		t.Errorf("без дашборда комментарий трогать нельзя, а в посте комментарии %v", got)
	}
	if n := totalReports(t); n != 2 {
		t.Errorf("без дашборда жалобы трогать нельзя, а их осталось %d из 2", n)
	}
}

// Без пароля, с неверным паролем и с токеном приложения — 401 с вызовом
// Basic, и ничего не удаляется (ФТ-1; 016, ФТ-2, ФТ-3).
func TestModerationRejectsWithoutRightPassword(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author, _ := signIn(t, baseURL, modPhone(1))
	reader, _ := signIn(t, baseURL, modPhone(2))
	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, reader, post.ID, commentText)
	requireAccepted(t, reportPostReason(t, baseURL, reader, post.ID, reportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, author, post.ID, comment.ID, commentReportReason))

	cases := []struct {
		name           string
		user, password string
		bearer         string
	}{
		{name: "без заголовка"},
		{name: "неверный пароль", user: "владелец", password: "неверный-пароль"},
		{name: "пустой пароль", user: "владелец", password: ""},
		{name: "пароль с лишним символом", user: "владелец", password: dashboardPassword + "x"},
		{name: "токен приложения автора", bearer: author},
		{name: "пароль дашборда вместо токена", bearer: dashboardPassword},
	}

	for _, path := range modAllPaths(post.ID, comment.ID) {
		for _, c := range cases {
			t.Run(path+" "+c.name, func(t *testing.T) {
				resp := modRequest(t, http.MethodDelete, root+path, c.user, c.password, c.bearer)

				if resp.StatusCode != http.StatusUnauthorized {
					t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
				}
				if got := resp.Header.Get("WWW-Authenticate"); got != `Basic realm="moya-dacha"` {
					t.Fatalf(`ожидался заголовок WWW-Authenticate: Basic realm="moya-dacha", получен %q`, got)
				}
			})
		}
	}

	requirePostAlive(t, baseURL, author, post.ID, "после отвергнутых удалений")
	if got := commentIDs(commentsOf(t, baseURL, author, post.ID).Items); !reflect.DeepEqual(got, []string{comment.ID}) {
		t.Errorf("без пароля комментарий трогать нельзя, а в посте комментарии %v", got)
	}
	if n := totalReports(t); n != 2 {
		t.Errorf("без пароля жалобы трогать нельзя, а их осталось %d из 2", n)
	}
}

// Ответы новых ручек не кэшируются — и 204, и 404 (ФТ-1; 016, ФТ-4).
func TestModerationAnswersAreNotCached(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author, _ := signIn(t, baseURL, modPhone(1))
	reader, _ := signIn(t, baseURL, modPhone(2))
	post := postToReport(t, baseURL, author)
	comment := commentOf(t, baseURL, reader, post.ID, commentText)

	unknown := "00000000-0000-4000-8000-000000000000"

	// «Оставить» сначала: удаление унесло бы пост и комментарий.
	modRequireStatus(t, modDelete(t, root, "/dashboard/posts/"+post.ID+"/reports"), http.StatusNoContent, "«Оставить» пост")
	modRequireStatus(t, modDelete(t, root, "/dashboard/comments/"+comment.ID+"/reports"), http.StatusNoContent, "«Оставить» комментарий")
	modRequireStatus(t, modDelete(t, root, "/dashboard/comments/"+comment.ID), http.StatusNoContent, "удаление комментария")
	modRequireStatus(t, modDelete(t, root, "/dashboard/posts/"+post.ID), http.StatusNoContent, "удаление поста")

	for _, path := range modAllPaths(unknown, unknown) {
		modRequireStatus(t, modDelete(t, root, path), http.StatusNotFound, "DELETE "+path+" на несуществующее")
	}
}

// Сервис не разрешает чужим страницам слать DELETE: на предварительный
// запрос CORS нет разрешения ни для какого источника (ФТ-2).
func TestModerationGivesNoCORSPermission(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author, _ := signIn(t, baseURL, modPhone(1))
	post := postToReport(t, baseURL, author)

	req, err := http.NewRequest(http.MethodOptions, root+"/dashboard/posts/"+post.ID, nil)
	if err != nil {
		t.Fatalf("не удалось собрать предварительный запрос: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	req.Header.Set("Access-Control-Request-Headers", "authorization")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("предварительный запрос не прошёл: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("сервис не должен разрешать чужим страницам запросы к дашборду, а отдал Access-Control-Allow-Origin: %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); strings.Contains(strings.ToUpper(got), "DELETE") {
		t.Errorf("сервис не должен разрешать DELETE чужим страницам, а отдал Access-Control-Allow-Methods: %q", got)
	}

	requirePostAlive(t, baseURL, author, post.ID, "после предварительного запроса")
}

// --- Данные раздела -------------------------------------------------------

// На пустой базе оба списка — [], а не null и не пропущены (ФТ-3).
func TestModerationOnEmptyBase(t *testing.T) {
	_, root := startDashboardAPI(t, api.Config{})

	moderation := modRawModeration(t, root)
	for _, list := range []string{"reported", "recent"} {
		raw, ok := moderation[list]
		if !ok {
			t.Errorf("в moderation нет поля %s", list)
			continue
		}
		if got := strings.TrimSpace(string(raw)); got != "[]" {
			t.Errorf("moderation.%s на пустой базе должен быть [], получено %s", list, got)
		}
	}
}

// Форма элементов: пост и комментарий с жалобами и без — все поля
// на месте, у поста comment_id — null, у комментария фото — первое фото
// его поста, время последней жалобы и причины — как записаны (ФТ-4,
// пользовательский сценарий).
func TestModerationItemShape(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	petr := newModUser(t, baseURL, 1, "petr", "Пётр")
	valya := newModUser(t, baseURL, 2, "valya", "Валентина")
	anna := newModUser(t, baseURL, 3, "anna", "")

	caption := "Продаю перегной, машина 5 тонн"
	first := photoOf(t, baseURL, petr.token, 60, 40)
	second := photoOf(t, baseURL, petr.token, 61, 41)
	post := createdPost(t, createPost(t, baseURL, petr.token,
		map[string]any{"media_ids": []string{first.ID, second.ID}, "caption": caption}))

	commentBody := "Звоните соседу: +7 900 000-00-00"
	comment := commentOf(t, baseURL, valya.token, post.ID, commentBody)
	quiet := commentOf(t, baseURL, anna.token, post.ID, "Хороший перегной")

	requireAccepted(t, reportPostReason(t, baseURL, valya.token, post.ID, "Реклама"))
	requireAccepted(t, reportPostReason(t, baseURL, anna.token, post.ID, "Это реклама, а не дача"))
	requireAccepted(t, reportCommentReason(t, baseURL, petr.token, post.ID, comment.ID, "Чужой телефон"))

	now := time.Now().UTC().Truncate(time.Second)
	modReportsAt(t, "post_id", post.ID, valya.id, now.Add(-2*time.Hour))
	annaAt := now.Add(-time.Hour)
	modReportsAt(t, "post_id", post.ID, anna.id, annaAt)
	commentAt := now.Add(-30 * time.Minute)
	modReportsAt(t, "comment_id", comment.ID, petr.id, commentAt)

	data := modData(t, root)

	// Пост.
	p := modMustFind(t, data.Moderation.Reported, modPostKey(post.ID), "reported")
	if p.Kind != "post" {
		t.Errorf("пост: kind = %q, ожидалось post", p.Kind)
	}
	if p.PostID != post.ID {
		t.Errorf("пост: post_id = %q, ожидалось %q", p.PostID, post.ID)
	}
	if p.CommentID != nil {
		t.Errorf("пост: comment_id = %q, ожидался null", *p.CommentID)
	}
	if want := (modAuthor{ID: petr.id, Nickname: "petr", Name: "Пётр"}); p.Author != want {
		t.Errorf("пост: author = %+v, ожидалось %+v", p.Author, want)
	}
	if p.Text != caption {
		t.Errorf("пост: text = %q, ожидалась подпись %q", p.Text, caption)
	}
	if p.PhotoURL == nil || *p.PhotoURL != first.URL {
		t.Errorf("пост: photo_url = %v, ожидалась ссылка на первое фото %q", p.PhotoURL, first.URL)
	}
	postCreated := parseDashTime(t, post.CreatedAt, "created_at поста из /api")
	if p.CreatedAt == "" {
		t.Error("пост: пустой created_at")
	} else if got := parseDashTime(t, p.CreatedAt, "created_at поста"); got.Sub(postCreated).Abs() > time.Second {
		t.Errorf("пост: created_at = %s, а пост опубликован в %s", p.CreatedAt, post.CreatedAt)
	}
	if p.Reports != 2 {
		t.Errorf("пост: reports = %d, ожидалось 2", p.Reports)
	}
	modRequireTime(t, p.LastReportAt, annaAt, "пост: last_report_at")
	if want := []string{"Это реклама, а не дача", "Реклама"}; !reflect.DeepEqual(p.Reasons, want) {
		t.Errorf("пост: reasons = %q, ожидалось %q (от новых к старым)", p.Reasons, want)
	}

	// Комментарий с жалобой.
	c := modMustFind(t, data.Moderation.Reported, modCommentKey(comment.ID), "reported")
	if c.Kind != "comment" {
		t.Errorf("комментарий: kind = %q, ожидалось comment", c.Kind)
	}
	if c.PostID != post.ID {
		t.Errorf("комментарий: post_id = %q, ожидался пост, под которым он, %q", c.PostID, post.ID)
	}
	if c.CommentID == nil || *c.CommentID != comment.ID {
		t.Errorf("комментарий: comment_id = %v, ожидалось %q", c.CommentID, comment.ID)
	}
	if want := (modAuthor{ID: valya.id, Nickname: "valya", Name: "Валентина"}); c.Author != want {
		t.Errorf("комментарий: author = %+v, ожидалось %+v", c.Author, want)
	}
	if c.Text != commentBody {
		t.Errorf("комментарий: text = %q, ожидалось %q", c.Text, commentBody)
	}
	if c.PhotoURL == nil || *c.PhotoURL != first.URL {
		t.Errorf("комментарий: photo_url = %v, ожидалось первое фото его поста %q", c.PhotoURL, first.URL)
	}
	if c.CreatedAt == "" {
		t.Error("комментарий: пустой created_at")
	}
	if c.Reports != 1 {
		t.Errorf("комментарий: reports = %d, ожидалось 1", c.Reports)
	}
	modRequireTime(t, c.LastReportAt, commentAt, "комментарий: last_report_at")
	if want := []string{"Чужой телефон"}; !reflect.DeepEqual(c.Reasons, want) {
		t.Errorf("комментарий: reasons = %q, ожидалось %q", c.Reasons, want)
	}

	// Комментарий без жалоб: в recent, не в reported, жалоб ноль; пустое
	// имя автора — пустая строка.
	modRequireAbsent(t, data.Moderation.Reported, modCommentKey(quiet.ID), "reported", "на него не жаловались")
	q := modMustFind(t, data.Moderation.Recent, modCommentKey(quiet.ID), "recent")
	modRequireNoReports(t, q, "комментарий без жалоб")
	if want := (modAuthor{ID: anna.id, Nickname: "anna", Name: ""}); q.Author != want {
		t.Errorf("комментарий без жалоб: author = %+v, ожидалось %+v", q.Author, want)
	}

	// В recent у поста с жалобами — те же числа, что в reported.
	pr := modMustFind(t, data.Moderation.Recent, modPostKey(post.ID), "recent")
	if pr.Reports != 2 || !reflect.DeepEqual(pr.Reasons, p.Reasons) {
		t.Errorf("пост в recent: reports = %d, reasons = %q — должны совпадать с reported (%d, %q)",
			pr.Reports, pr.Reasons, p.Reports, p.Reasons)
	}

	// comment_id у поста — именно null, а не пропущенное поле; reasons
	// у элемента без жалоб — [], а не null; last_report_at — null.
	moderation := modRawModeration(t, root)
	for _, item := range modRawItems(t, moderation["recent"], "moderation.recent") {
		var kind string
		_ = json.Unmarshal(item["kind"], &kind)
		for _, field := range []string{"kind", "post_id", "comment_id", "author", "text", "photo_url",
			"created_at", "reports", "last_report_at", "reasons"} {
			if _, ok := item[field]; !ok {
				t.Errorf("в элементе recent (%s) нет поля %s", kind, field)
			}
		}
		if kind == "post" {
			if got := strings.TrimSpace(string(item["comment_id"])); got != "null" {
				t.Errorf("у поста comment_id должен быть null, получено %s", got)
			}
		}
		var reports int
		_ = json.Unmarshal(item["reports"], &reports)
		if reports == 0 {
			if got := strings.TrimSpace(string(item["reasons"])); got != "[]" {
				t.Errorf("у элемента без жалоб reasons должен быть [], получено %s", got)
			}
			if got := strings.TrimSpace(string(item["last_report_at"])); got != "null" {
				t.Errorf("у элемента без жалоб last_report_at должен быть null, получено %s", got)
			}
		}
	}
}

// Пост с пустой подписью: text — пустая строка, не null (ФТ-4).
func TestModerationPostWithoutCaptionHasEmptyText(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	photo := photoOf(t, baseURL, author.token, 50, 30)
	post := createdPost(t, createPostOf(t, baseURL, author.token, photo.ID))

	moderation := modRawModeration(t, root)
	for _, item := range modRawItems(t, moderation["recent"], "moderation.recent") {
		var id string
		_ = json.Unmarshal(item["post_id"], &id)
		if id != post.ID {
			continue
		}
		if got := strings.TrimSpace(string(item["text"])); got != `""` {
			t.Errorf("у поста без подписи text должен быть пустой строкой, получено %s", got)
		}
		return
	}
	t.Fatalf("поста %s нет в recent", post.ID)
}

// Причины: от новых к старым, только непустые; жалобы без причины
// считаются в reports, но в reasons их нет (ФТ-4).
func TestModerationReasonsNewestFirstWithoutEmpty(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	post := postToReport(t, baseURL, author.token)

	reporters := make([]modUser, 0, 4)
	for i := 0; i < 4; i++ {
		reporters = append(reporters, newModUser(t, baseURL, 10+i, fmt.Sprintf("reporter_%d", i), ""))
	}

	requireAccepted(t, reportPostReason(t, baseURL, reporters[0].token, post.ID, "Реклама"))
	requireAccepted(t, reportPost(t, baseURL, reporters[1].token, post.ID, nil))
	requireAccepted(t, reportPostReason(t, baseURL, reporters[2].token, post.ID, "   "))
	requireAccepted(t, reportPostReason(t, baseURL, reporters[3].token, post.ID, "Это реклама"))

	now := time.Now().UTC().Truncate(time.Second)
	modReportsAt(t, "post_id", post.ID, reporters[0].id, now.Add(-4*time.Hour))
	modReportsAt(t, "post_id", post.ID, reporters[1].id, now.Add(-3*time.Hour))
	modReportsAt(t, "post_id", post.ID, reporters[3].id, now.Add(-2*time.Hour))
	// Последняя по времени — жалоба без причины: last_report_at — её время.
	last := now.Add(-time.Hour)
	modReportsAt(t, "post_id", post.ID, reporters[2].id, last)

	item := modMustFind(t, modData(t, root).Moderation.Reported, modPostKey(post.ID), "reported")
	if item.Reports != 4 {
		t.Errorf("reports = %d, ожидалось 4 — жалобы без причины тоже считаются", item.Reports)
	}
	if want := []string{"Это реклама", "Реклама"}; !reflect.DeepEqual(item.Reasons, want) {
		t.Errorf("reasons = %q, ожидалось %q — от новых к старым и без пустых", item.Reasons, want)
	}
	modRequireTime(t, item.LastReportAt, last, "last_report_at")
}

// Жалобы только без причин: reasons — [], а не null (ФТ-4).
func TestModerationReasonsEmptyListWhenNoReasons(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	reporter := newModUser(t, baseURL, 2, "valya", "Валентина")
	post := postToReport(t, baseURL, author.token)
	requireAccepted(t, reportPost(t, baseURL, reporter.token, post.ID, nil))

	for _, item := range modRawItems(t, modRawModeration(t, root)["reported"], "moderation.reported") {
		if got := strings.TrimSpace(string(item["reasons"])); got != "[]" {
			t.Errorf("у жалобы без причины reasons должен быть [], получено %s", got)
		}
		if got := strings.TrimSpace(string(item["reports"])); got != "1" {
			t.Errorf("reports = %s, ожидалось 1", got)
		}
	}
}

// Порядок reported: больше жалоб — выше, при равенстве выше то, на что
// пожаловались позже; посты и комментарии в одном списке; то, на что
// не жаловались, туда не попадает (ФТ-5).
func TestModerationReportedOrder(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	r1 := newModUser(t, baseURL, 2, "valya", "Валентина")
	r2 := newModUser(t, baseURL, 3, "anna", "Анна")

	a := postToReport(t, baseURL, author.token)
	b := postToReport(t, baseURL, author.token)
	c := postToReport(t, baseURL, author.token)
	clean := postToReport(t, baseURL, author.token)
	d := commentOf(t, baseURL, author.token, clean.ID, commentText)
	e := commentOf(t, baseURL, author.token, clean.ID, commentText)

	now := time.Now().UTC().Truncate(time.Second)

	// a: одна жалоба, самая свежая из всех.
	requireAccepted(t, reportPostReason(t, baseURL, r1.token, a.ID, reportReason))
	modReportsAt(t, "post_id", a.ID, r1.id, now.Add(-1*time.Minute))

	// b: две жалобы, последняя час назад.
	requireAccepted(t, reportPostReason(t, baseURL, r1.token, b.ID, reportReason))
	requireAccepted(t, reportPostReason(t, baseURL, r2.token, b.ID, reportReason))
	modReportsAt(t, "post_id", b.ID, r1.id, now.Add(-5*time.Hour))
	modReportsAt(t, "post_id", b.ID, r2.id, now.Add(-1*time.Hour))

	// c: две жалобы, последняя полчаса назад — выше b.
	requireAccepted(t, reportPostReason(t, baseURL, r1.token, c.ID, reportReason))
	requireAccepted(t, reportPostReason(t, baseURL, r2.token, c.ID, reportReason))
	modReportsAt(t, "post_id", c.ID, r1.id, now.Add(-6*time.Hour))
	modReportsAt(t, "post_id", c.ID, r2.id, now.Add(-30*time.Minute))

	// d: комментарий, одна жалоба три часа назад — ниже a.
	requireAccepted(t, reportCommentReason(t, baseURL, r1.token, clean.ID, d.ID, commentReportReason))
	modReportsAt(t, "comment_id", d.ID, r1.id, now.Add(-3*time.Hour))

	// e: комментарий, две жалобы, последняя два часа назад — между c/b
	// и a: по числу жалоб наравне с b и c, но жалоба старше их.
	requireAccepted(t, reportCommentReason(t, baseURL, r1.token, clean.ID, e.ID, commentReportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, r2.token, clean.ID, e.ID, commentReportReason))
	modReportsAt(t, "comment_id", e.ID, r1.id, now.Add(-7*time.Hour))
	modReportsAt(t, "comment_id", e.ID, r2.id, now.Add(-2*time.Hour))

	want := []string{
		modPostKey(c.ID),
		modPostKey(b.ID),
		modCommentKey(e.ID),
		modPostKey(a.ID),
		modCommentKey(d.ID),
	}
	if got := modKeys(modData(t, root).Moderation.Reported); !reflect.DeepEqual(got, want) {
		t.Fatalf("порядок reported:\nполучен  %v\nожидался %v\n(больше жалоб выше, при равенстве — свежее последней жалобы выше)", got, want)
	}
}

// reported — не больше 50 элементов (ФТ-5).
func TestModerationReportedIsCappedAtFifty(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	reporter := newModUser(t, baseURL, 2, "valya", "Валентина")
	post := postToReport(t, baseURL, author.token)

	for i := 0; i < 51; i++ {
		comment := commentOf(t, baseURL, author.token, post.ID, fmt.Sprintf("Комментарий %d", i))
		requireAccepted(t, reportCommentReason(t, baseURL, reporter.token, post.ID, comment.ID, commentReportReason))
	}

	if got := len(modData(t, root).Moderation.Reported); got != 50 {
		t.Fatalf("в reported %d элементов, ожидалось не больше 50 (жалоб на 51 комментарий)", got)
	}
}

// recent — последние 30 постов и комментариев вместе, от новых к старым;
// пост «только я» и пост «для друзей» в нём тоже есть (ФТ-6).
func TestModerationRecentIsLastThirtyOfPostsAndComments(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	reader := newModUser(t, baseURL, 2, "valya", "Валентина")

	type entry struct {
		table, id, key string
	}
	var entries []entry

	public := postToReport(t, baseURL, author.token)
	entries = append(entries, entry{"posts", public.ID, modPostKey(public.ID)})

	addComments := func(n int) {
		for i := 0; i < n; i++ {
			comment := commentOf(t, baseURL, reader.token, public.ID, fmt.Sprintf("Комментарий %d", len(entries)))
			entries = append(entries, entry{"comments", comment.ID, modCommentKey(comment.ID)})
		}
	}

	addComments(15)
	private := modPostWithVisibility(t, baseURL, author.token, "Только для себя", visibilityMe)
	entries = append(entries, entry{"posts", private.ID, modPostKey(private.ID)})
	addComments(10)
	friends := modPostWithVisibility(t, baseURL, author.token, "Для друзей", visibilityFriends)
	entries = append(entries, entry{"posts", friends.ID, modPostKey(friends.ID)})
	addComments(8)
	// Всего 1 + 15 + 1 + 10 + 1 + 8 = 36 — больше 30.

	// Время появления — строго по порядку создания, с шагом в минуту.
	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	for i, e := range entries {
		modCreatedAt(t, e.table, e.id, base.Add(time.Duration(i)*time.Minute))
	}

	want := make([]string, 0, 30)
	for i := len(entries) - 1; i >= 0 && len(want) < 30; i-- {
		want = append(want, entries[i].key)
	}

	recent := modData(t, root).Moderation.Recent
	if got := modKeys(recent); !reflect.DeepEqual(got, want) {
		t.Fatalf("recent:\nполучен  %v\nожидался %v\n(30 последних постов и комментариев вместе, от новых к старым)", got, want)
	}

	p := modMustFind(t, recent, modPostKey(private.ID), "recent")
	if p.Kind != "post" || p.Text != "Только для себя" {
		t.Errorf("пост «только я» в recent: kind = %q, text = %q", p.Kind, p.Text)
	}
	modRequireNoReports(t, p, "пост «только я»")
	modMustFind(t, recent, modPostKey(friends.ID), "recent")

	// Комментарий несёт первое фото своего поста.
	last := modMustFind(t, recent, entries[len(entries)-1].key, "recent")
	if last.PostID != public.ID {
		t.Errorf("комментарий в recent: post_id = %q, ожидалось %q", last.PostID, public.ID)
	}
	if last.PhotoURL == nil || *last.PhotoURL != public.Media[0].URL {
		t.Errorf("комментарий в recent: photo_url = %v, ожидалось первое фото поста %q", last.PhotoURL, public.Media[0].URL)
	}
}

// Пост «только я» с жалобой (пожаловались, пока он был виден всем) тоже
// попадает в reported: видимость на модерацию не действует (ФТ-6).
func TestModerationReportedIncludesHiddenPost(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	reporter := newModUser(t, baseURL, 2, "valya", "Валентина")
	post := postToReport(t, baseURL, author.token)
	requireAccepted(t, reportPostReason(t, baseURL, reporter.token, post.ID, reportReason))

	if resp := setVisibility(t, baseURL, author.token, post.ID, map[string]any{"visibility": visibilityMe}); resp.StatusCode != http.StatusOK {
		t.Fatalf("видимость не сменилась: статус %d", resp.StatusCode)
	}

	item := modMustFind(t, modData(t, root).Moderation.Reported, modPostKey(post.ID), "reported")
	if item.Reports != 1 {
		t.Errorf("reports = %d, ожидалось 1", item.Reports)
	}
}

// --- Удаление поста -------------------------------------------------------

// Владелец удаляет чужой пост: 204, пост пропал отовсюду вместе с фото,
// комментариями и жалобами — и на него, и на его комментарии; соседний
// пост на месте; автора не извещают (ФТ-7, ФТ-10, сценарий шаги 3-4).
func TestModerationDeletePost(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	petr := newModUser(t, baseURL, 1, "petr", "Пётр")
	valya := newModUser(t, baseURL, 2, "valya", "Валентина")
	anna := newModUser(t, baseURL, 3, "anna", "Анна")

	post := postWithPhotos(t, baseURL, petr.token, 2)
	other := postToReport(t, baseURL, petr.token)
	comment := commentOf(t, baseURL, valya.token, post.ID, commentText)
	if resp := do(t, http.MethodPut, baseURL+"/posts/"+post.ID+"/like", anna.token, nil); resp.StatusCode >= 300 {
		t.Fatalf("лайк не поставился: статус %d", resp.StatusCode)
	}

	requireAccepted(t, reportPostReason(t, baseURL, valya.token, post.ID, "Реклама"))
	requireAccepted(t, reportPostReason(t, baseURL, anna.token, post.ID, "Это реклама, а не дача"))
	requireAccepted(t, reportCommentReason(t, baseURL, anna.token, post.ID, comment.ID, commentReportReason))
	requireAccepted(t, reportPostReason(t, baseURL, valya.token, other.ID, reportReason))

	before := modData(t, root)
	if before.Totals.Reports != 4 {
		t.Fatalf("перед удалением totals.reports = %d, ожидалось 4", before.Totals.Reports)
	}
	unreadBefore := modUnread(t, baseURL, petr.token)

	modDeleted(t, root, "/dashboard/posts/"+post.ID, "удаление поста владельцем")

	requirePostGone(t, baseURL, petr.token, post.ID, "автор после удаления владельцем")
	requirePostGone(t, baseURL, valya.token, post.ID, "читатель после удаления владельцем")
	for _, photo := range post.Media {
		requireDeletedPhotoGone(t, baseURL, photo)
	}
	requirePostAlive(t, baseURL, petr.token, other.ID, "соседний пост")

	if n := countSQL(t, "SELECT count(*) FROM comments WHERE post_id = $1", post.ID); n != 0 {
		t.Errorf("пост удалён, а комментариев под ним в базе осталось %d", n)
	}
	if n := countSQL(t, "SELECT count(*) FROM post_likes WHERE post_id = $1", post.ID); n != 0 {
		t.Errorf("пост удалён, а лайков на нём в базе осталось %d", n)
	}

	after := modData(t, root)
	if after.Totals.Reports != 1 {
		t.Errorf("после удаления totals.reports = %d, ожидалась 1 — жалоба на соседний пост", after.Totals.Reports)
	}
	for _, list := range []struct {
		name  string
		items []modItem
	}{{"reported", after.Moderation.Reported}, {"recent", after.Moderation.Recent}} {
		modRequireAbsent(t, list.items, modPostKey(post.ID), list.name, "пост удалён")
		modRequireAbsent(t, list.items, modCommentKey(comment.ID), list.name, "комментарий удалён вместе с постом")
	}
	modMustFind(t, after.Moderation.Reported, modPostKey(other.ID), "reported")

	if got := modUnread(t, baseURL, petr.token); got > unreadBefore {
		t.Errorf("автора не извещают об удалении, а непрочитанных уведомлений стало %d (было %d)", got, unreadBefore)
	}

	// Повтор — 404: удалять уже нечего (ФТ-9).
	modNotFound(t, root, "/dashboard/posts/"+post.ID, "повторное удаление поста")
	// «Оставить» на удалённый пост — тоже 404 (ФТ-11).
	modNotFound(t, root, "/dashboard/posts/"+post.ID+"/reports", "«Оставить» на удалённый пост")
	// Автор правит удалённое — 404, как после удаления им самим.
	if resp := do(t, http.MethodPut, baseURL+"/posts/"+post.ID+"/caption", petr.token,
		map[string]any{"caption": "Новая подпись"}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("правка подписи удалённого поста: ожидался статус 404, получен %d", resp.StatusCode)
	}
}

// Пост с видимостью «только я» владелец удаляет так же (ФТ-6, ФТ-7).
func TestModerationDeleteHiddenPost(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	post := modPostWithVisibility(t, baseURL, author.token, "Только для себя", visibilityMe)

	modDeleted(t, root, "/dashboard/posts/"+post.ID, "удаление поста «только я»")
	requirePostGone(t, baseURL, author.token, post.ID, "автор после удаления владельцем")
}

// Нет такого поста или id не похож на идентификатор — 404; id комментария
// по адресу поста — тоже 404, и ничего не удалено (ФТ-9).
func TestModerationDeleteUnknownPostIsNotFound(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	post := postToReport(t, baseURL, author.token)
	comment := commentOf(t, baseURL, author.token, post.ID, commentText)

	for _, id := range []string{
		"00000000-0000-4000-8000-000000000000",
		"not-a-uuid",
		"123",
		comment.ID,
	} {
		modNotFound(t, root, "/dashboard/posts/"+id, "DELETE /dashboard/posts/"+id)
		modNotFound(t, root, "/dashboard/posts/"+id+"/reports", "DELETE /dashboard/posts/"+id+"/reports")
	}

	requirePostAlive(t, baseURL, author.token, post.ID, "после 404")
	if got := commentIDs(commentsOf(t, baseURL, author.token, post.ID).Items); !reflect.DeepEqual(got, []string{comment.ID}) {
		t.Errorf("после 404 комментарии поста %v, ожидался %v", got, []string{comment.ID})
	}
}

// --- Удаление комментария -------------------------------------------------

// Владелец удаляет чужой комментарий без жалобы: 204, комментария нет
// в посте, пост и другие комментарии на месте, жалобы на комментарий
// ушли, жалоба на пост осталась, уведомление о комментарии пропало,
// автора не извещают (ФТ-8, ФТ-10, сценарий шаг 5).
func TestModerationDeleteComment(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	petr := newModUser(t, baseURL, 1, "petr", "Пётр")
	valya := newModUser(t, baseURL, 2, "valya", "Валентина")
	anna := newModUser(t, baseURL, 3, "anna", "Анна")

	post := postToReport(t, baseURL, petr.token)
	unreadBefore := modUnread(t, baseURL, petr.token)

	bad := commentOf(t, baseURL, valya.token, post.ID, "Звоните соседу: +7 900 000-00-00")
	good := commentOf(t, baseURL, anna.token, post.ID, commentText)

	requireAccepted(t, reportCommentReason(t, baseURL, anna.token, post.ID, bad.ID, commentReportReason))
	requireAccepted(t, reportPostReason(t, baseURL, anna.token, post.ID, reportReason))

	valyaUnread := modUnread(t, baseURL, valya.token)

	modDeleted(t, root, "/dashboard/comments/"+bad.ID, "удаление комментария владельцем")

	if got := commentIDs(commentsOf(t, baseURL, petr.token, post.ID).Items); !reflect.DeepEqual(got, []string{good.ID}) {
		t.Errorf("после удаления в посте комментарии %v, ожидался только %v", got, []string{good.ID})
	}
	requirePostAlive(t, baseURL, petr.token, post.ID, "пост под удалённым комментарием")

	if n := len(commentReports(t, bad.ID, "после удаления комментария")); n != 0 {
		t.Errorf("комментарий удалён, а жалоб на него в базе осталось %d", n)
	}
	if n := len(postReports(t, post.ID, "после удаления комментария")); n != 1 {
		t.Errorf("жалоба на пост должна остаться, а их %d", n)
	}

	data := modData(t, root)
	if data.Totals.Reports != 1 {
		t.Errorf("после удаления комментария totals.reports = %d, ожидалась 1", data.Totals.Reports)
	}
	modRequireAbsent(t, data.Moderation.Reported, modCommentKey(bad.ID), "reported", "комментарий удалён")
	modRequireAbsent(t, data.Moderation.Recent, modCommentKey(bad.ID), "recent", "комментарий удалён")
	modMustFind(t, data.Moderation.Recent, modCommentKey(good.ID), "recent")
	modMustFind(t, data.Moderation.Reported, modPostKey(post.ID), "reported")

	// Уведомление автору поста о комментарии Валентины ушло вместе
	// с комментарием: осталось только про комментарий Анны.
	if got := modUnread(t, baseURL, petr.token); got > unreadBefore+1 {
		t.Errorf("уведомление об удалённом комментарии должно уйти: непрочитанных %d, до комментариев было %d, остался один комментарий", got, unreadBefore)
	}
	if got := modUnread(t, baseURL, valya.token); got > valyaUnread {
		t.Errorf("автора комментария не извещают об удалении, а непрочитанных стало %d (было %d)", got, valyaUnread)
	}

	// Повтор — 404 (ФТ-9).
	modNotFound(t, root, "/dashboard/comments/"+bad.ID, "повторное удаление комментария")
	modNotFound(t, root, "/dashboard/comments/"+bad.ID+"/reports", "«Оставить» на удалённый комментарий")
	// Автор правит удалённый комментарий — 404.
	if resp := do(t, http.MethodPut, baseURL+"/posts/"+post.ID+"/comments/"+bad.ID, valya.token,
		map[string]any{"text": "Исправила"}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("правка удалённого комментария: ожидался статус 404, получен %d", resp.StatusCode)
	}
}

// Нет такого комментария или id не похож на идентификатор — 404; id поста
// по адресу комментария — тоже 404, и пост не удалён (ФТ-9).
func TestModerationDeleteUnknownCommentIsNotFound(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	post := postToReport(t, baseURL, author.token)

	for _, id := range []string{
		"00000000-0000-4000-8000-000000000000",
		"not-a-uuid",
		"123",
		post.ID,
	} {
		modNotFound(t, root, "/dashboard/comments/"+id, "DELETE /dashboard/comments/"+id)
		modNotFound(t, root, "/dashboard/comments/"+id+"/reports", "DELETE /dashboard/comments/"+id+"/reports")
	}

	requirePostAlive(t, baseURL, author.token, post.ID, "после 404 по адресу комментария")
}

// --- «Оставить» -----------------------------------------------------------

// «Оставить» пост: 204, пост на месте, жалоб на него нет, из reported
// ушёл, в recent reports = 0, totals.reports уменьшился; жалобы на
// комментарий под ним не тронуты; новая жалоба возвращает пост в список
// (ФТ-11, ФТ-12, сценарий шаг 6).
func TestModerationDismissPostReports(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	petr := newModUser(t, baseURL, 1, "petr", "Пётр")
	valya := newModUser(t, baseURL, 2, "valya", "Валентина")
	anna := newModUser(t, baseURL, 3, "anna", "Анна")

	post := postToReport(t, baseURL, petr.token)
	comment := commentOf(t, baseURL, petr.token, post.ID, commentText)

	requireAccepted(t, reportPostReason(t, baseURL, valya.token, post.ID, "Реклама"))
	requireAccepted(t, reportPostReason(t, baseURL, anna.token, post.ID, reportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, anna.token, post.ID, comment.ID, commentReportReason))

	if got := modData(t, root).Totals.Reports; got != 3 {
		t.Fatalf("перед «Оставить» totals.reports = %d, ожидалось 3", got)
	}

	modDeleted(t, root, "/dashboard/posts/"+post.ID+"/reports", "«Оставить» пост")

	requirePostAlive(t, baseURL, petr.token, post.ID, "после «Оставить»")
	if got := commentIDs(commentsOf(t, baseURL, petr.token, post.ID).Items); !reflect.DeepEqual(got, []string{comment.ID}) {
		t.Errorf("после «Оставить» комментарии поста %v, ожидался %v", got, []string{comment.ID})
	}
	if n := len(postReports(t, post.ID, "после «Оставить»")); n != 0 {
		t.Errorf("после «Оставить» жалоб на пост в базе %d, ожидалось 0", n)
	}
	if n := len(commentReports(t, comment.ID, "после «Оставить» пост")); n != 1 {
		t.Errorf("«Оставить» пост не трогает жалобы на комментарии под ним, а их %d из 1", n)
	}

	data := modData(t, root)
	if data.Totals.Reports != 1 {
		t.Errorf("после «Оставить» totals.reports = %d, ожидалась 1", data.Totals.Reports)
	}
	modRequireAbsent(t, data.Moderation.Reported, modPostKey(post.ID), "reported", "жалобы на пост стёрты «Оставить»")
	c := modMustFind(t, data.Moderation.Reported, modCommentKey(comment.ID), "reported")
	if c.Reports != 1 {
		t.Errorf("комментарий под оставленным постом: reports = %d, ожидалось 1", c.Reports)
	}
	p := modMustFind(t, data.Moderation.Recent, modPostKey(post.ID), "recent")
	modRequireNoReports(t, p, "пост после «Оставить»")

	// Повтор — 204: жалоб нет, но пост есть.
	modDeleted(t, root, "/dashboard/posts/"+post.ID+"/reports", "повторное «Оставить» пост")

	// Новая жалоба возвращает пост в reported — даже от того же человека.
	requireAccepted(t, reportPostReason(t, baseURL, valya.token, post.ID, "Снова реклама"))
	back := modMustFind(t, modData(t, root).Moderation.Reported, modPostKey(post.ID), "reported после новой жалобы")
	if back.Reports != 1 {
		t.Errorf("после новой жалобы reports = %d, ожидалось 1", back.Reports)
	}
	if want := []string{"Снова реклама"}; !reflect.DeepEqual(back.Reasons, want) {
		t.Errorf("после новой жалобы reasons = %q, ожидалось %q", back.Reasons, want)
	}
}

// «Оставить» комментарий: 204, комментарий на месте, из reported ушёл,
// жалоба на пост не тронута (ФТ-11, ФТ-12).
func TestModerationDismissCommentReports(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	petr := newModUser(t, baseURL, 1, "petr", "Пётр")
	valya := newModUser(t, baseURL, 2, "valya", "Валентина")
	anna := newModUser(t, baseURL, 3, "anna", "Анна")

	post := postToReport(t, baseURL, petr.token)
	comment := commentOf(t, baseURL, valya.token, post.ID, commentText)

	requireAccepted(t, reportCommentReason(t, baseURL, petr.token, post.ID, comment.ID, commentReportReason))
	requireAccepted(t, reportCommentReason(t, baseURL, anna.token, post.ID, comment.ID, "Грубость какая"))
	requireAccepted(t, reportPostReason(t, baseURL, anna.token, post.ID, reportReason))

	modDeleted(t, root, "/dashboard/comments/"+comment.ID+"/reports", "«Оставить» комментарий")

	if got := commentIDs(commentsOf(t, baseURL, petr.token, post.ID).Items); !reflect.DeepEqual(got, []string{comment.ID}) {
		t.Errorf("после «Оставить» комментарии поста %v, ожидался %v", got, []string{comment.ID})
	}
	if n := len(commentReports(t, comment.ID, "после «Оставить» комментарий")); n != 0 {
		t.Errorf("после «Оставить» жалоб на комментарий в базе %d, ожидалось 0", n)
	}
	if n := len(postReports(t, post.ID, "после «Оставить» комментарий")); n != 1 {
		t.Errorf("жалоба на пост не должна пострадать, а их %d из 1", n)
	}

	data := modData(t, root)
	if data.Totals.Reports != 1 {
		t.Errorf("после «Оставить» комментарий totals.reports = %d, ожидалась 1", data.Totals.Reports)
	}
	modRequireAbsent(t, data.Moderation.Reported, modCommentKey(comment.ID), "reported", "жалобы на комментарий стёрты")
	modMustFind(t, data.Moderation.Reported, modPostKey(post.ID), "reported")
	c := modMustFind(t, data.Moderation.Recent, modCommentKey(comment.ID), "recent")
	modRequireNoReports(t, c, "комментарий после «Оставить»")

	modDeleted(t, root, "/dashboard/comments/"+comment.ID+"/reports", "повторное «Оставить» комментарий")
}

// «Оставить» то, на что не жаловались, — всё равно 204 (ФТ-11).
func TestModerationDismissWithoutReportsIsNoContent(t *testing.T) {
	baseURL, root := startDashboardAPI(t, api.Config{})

	author := newModUser(t, baseURL, 1, "petr", "Пётр")
	post := postToReport(t, baseURL, author.token)
	comment := commentOf(t, baseURL, author.token, post.ID, commentText)

	modDeleted(t, root, "/dashboard/posts/"+post.ID+"/reports", "«Оставить» пост без жалоб")
	modDeleted(t, root, "/dashboard/comments/"+comment.ID+"/reports", "«Оставить» комментарий без жалоб")

	requirePostAlive(t, baseURL, author.token, post.ID, "после «Оставить» без жалоб")
	if got := commentIDs(commentsOf(t, baseURL, author.token, post.ID).Items); !reflect.DeepEqual(got, []string{comment.ID}) {
		t.Errorf("после «Оставить» без жалоб комментарии поста %v, ожидался %v", got, []string{comment.ID})
	}
}
