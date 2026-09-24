package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// Имя и «о себе» соседки, чей профиль открывают в большинстве проверок
// (specs/009-user-profile.md, «API / контракт данных»).
const (
	neighbourName  = "Валентина"
	neighbourAbout = "Теплица, три грядки клубники и кот Василий"
)

// --- Представления из контракта -------------------------------------------

// userProfilePayload — пользователь, каким его видят соседи (schema
// UserProfile). Поля phone нет и быть не может (ФТ-2): его отсутствие
// проверяется по сырому ответу, а не по этой структуре.
type userProfilePayload struct {
	ID        string  `json:"id"`
	Nickname  string  `json:"nickname"`
	Name      string  `json:"name"`
	About     string  `json:"about"`
	AvatarURL *string `json:"avatar_url"`
	CreatedAt string  `json:"created_at"`
	Posts     int     `json:"posts"`
}

// --- Хелперы --------------------------------------------------------------

// userAddress — адрес профиля по идентификатору пользователя. Идентификатор
// экранируется: в тестах он бывает и не UUID вовсе.
func userAddress(baseURL, userID string) string {
	return baseURL + "/users/" + url.PathEscape(userID)
}

// fetchUser открывает профиль пользователя.
func fetchUser(t *testing.T, baseURL, token, userID string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, userAddress(baseURL, userID), token, nil)
}

// userProfileOK требует, чтобы профиль открылся, и возвращает его.
func userProfileOK(t *testing.T, resp *http.Response) userProfilePayload {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на профиль пользователя ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body userProfilePayload
	decode(t, resp, &body)

	return body
}

// userProfile — самый частый случай: профиль, который должен открыться.
func userProfile(t *testing.T, baseURL, token, userID string) userProfilePayload {
	t.Helper()
	return userProfileOK(t, fetchUser(t, baseURL, token, userID))
}

// userProfileFields — профиль как словарь полей: тестам про то, какие поля
// есть в ответе, а каких нет, структура не годится.
func userProfileFields(t *testing.T, baseURL, token, userID string) map[string]any {
	t.Helper()

	resp := fetchUser(t, baseURL, token, userID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на профиль пользователя ожидался статус 200, получен %d", resp.StatusCode)
	}

	var fields map[string]any
	if err := json.Unmarshal(rawJSON(t, resp), &fields); err != nil {
		t.Fatalf("профиль пользователя не разобрался как JSON: %v", err)
	}

	return fields
}

// fetchUserPostsRaw запрашивает посты пользователя с готовой строкой
// запроса: она нужна там, где параметр нарочно неправильный (`limit=abc`),
// пустой (`cursor=`) или указан дважды.
func fetchUserPostsRaw(t *testing.T, baseURL, token, userID, query string) *http.Response {
	t.Helper()

	address := userAddress(baseURL, userID) + "/posts"
	if query != "" {
		address += "?" + query
	}

	return do(t, http.MethodGet, address, token, nil)
}

// fetchUserPosts запрашивает страницу постов пользователя с этими
// параметрами (feedParams: limit == 0 и пустой cursor — «параметра нет»).
func fetchUserPosts(t *testing.T, baseURL, token, userID string, params url.Values) *http.Response {
	t.Helper()

	query := ""
	if len(params) > 0 {
		query = params.Encode()
	}

	return fetchUserPostsRaw(t, baseURL, token, userID, query)
}

// userPostsOK требует, чтобы страница постов отдалась, и возвращает её:
// схема та же, что у ленты (schema Feed).
func userPostsOK(t *testing.T, resp *http.Response) feedPayload {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на страницу постов пользователя ожидался статус 200, получен %d", resp.StatusCode)
	}

	var page feedPayload
	decode(t, resp, &page)

	return page
}

// userPostsPage — страница постов пользователя, которая должна отдаться.
func userPostsPage(t *testing.T, baseURL, token, userID string, params url.Values) feedPayload {
	t.Helper()
	return userPostsOK(t, fetchUserPosts(t, baseURL, token, userID, params))
}

// walkUserPosts проходит посты пользователя страницами по limit — так
// сетка профиля подгружается при прокрутке — и возвращает идентификаторы
// подряд. Попутно требует, чтобы страница с курсором не была пустой
// и чтобы проход был конечен.
func walkUserPosts(t *testing.T, baseURL, token, userID string, limit int) []string {
	t.Helper()

	var ids []string
	cursor := ""

	for page := 1; ; page++ {
		if page > 200 {
			t.Fatalf("посты пользователя не кончаются: за %d страниц отдано %d постов", page-1, len(ids))
		}

		got := userPostsPage(t, baseURL, token, userID, feedParams(limit, cursor))
		if len(got.Items) == 0 && page > 1 {
			t.Fatalf("страница %d пуста, хотя курсор обещал продолжение", page)
		}
		ids = append(ids, postIDs(got.Items)...)

		if got.NextCursor == nil {
			break
		}
		cursor = cursorOf(t, got, fmt.Sprintf("страница %d", page))
	}

	return ids
}

// requireAuthoredBy требует, чтобы все посты страницы были этого автора.
func requireAuthoredBy(t *testing.T, page feedPayload, userID, where string) {
	t.Helper()

	for _, item := range page.Items {
		if item.Author.ID != userID {
			t.Errorf("%s: пост %s автора %s попал в посты пользователя %s", where, item.ID, item.Author.ID, userID)
		}
	}
}

// requireNoPhone требует, чтобы в ответе не было ни поля phone, ни самого
// номера ни в каком привычном виде (ФТ-2, CONTEXT.md).
func requireNoPhone(t *testing.T, body string, phones []string, where string) {
	t.Helper()

	for _, forbidden := range append([]string{`"phone"`}, phones...) {
		if strings.Contains(body, forbidden) {
			t.Errorf("%s: номера телефона в ответе быть не должно, а он содержит %q", where, forbidden)
		}
	}
}

// requireSameRegistration требует, чтобы дата регистрации в профиле была
// той же, что человек видит у себя в GET /api/me. Сравнивается момент,
// а не строка: спеке важна дата, а не число знаков после секунд.
func requireSameRegistration(t *testing.T, got, want, where string) {
	t.Helper()

	gotAt, err := time.Parse(time.RFC3339Nano, got)
	if err != nil {
		t.Fatalf("%s: дата регистрации %q не разбирается как время: %v", where, got, err)
	}
	wantAt, err := time.Parse(time.RFC3339Nano, want)
	if err != nil {
		t.Fatalf("%s: дата регистрации в /me %q не разбирается как время: %v", where, want, err)
	}

	if !gotAt.Truncate(time.Second).Equal(wantAt.Truncate(time.Second)) {
		t.Errorf("%s: дата регистрации %s, а в /me — %s", where, got, want)
	}
}

// meetNeighbour регистрирует соседку с именем, «о себе» и аватаром — всем,
// что есть в профиле, — и возвращает её токен, идентификатор и ссылку на
// аватар.
func meetNeighbour(t *testing.T, baseURL string) (token, userID, avatar string) {
	t.Helper()

	token, userID = signIn(t, baseURL, otherPhonePretty)

	resp := updateProfile(t, baseURL, token, map[string]any{"name": neighbourName, "about": neighbourAbout})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("профиль соседки должен был сохраниться, получен статус %d", resp.StatusCode)
	}

	avatar = avatarOf(t, baseURL, token, imageBytes(t, "png", 300, 300))

	return token, userID, avatar
}

// --- GET /api/users/{userId}: профиль -------------------------------------

// Чужой профиль: имя, «о себе», аватар, дата регистрации и число постов;
// номера телефона нет ни в профиле, ни в постах (ФТ-1, ФТ-2, ФТ-6,
// «Профиль чужого пользователя»).
func TestUserProfileOfAnotherUserShowsWhoTheyAreWithoutPhone(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	neighbourToken, neighbourID, avatar := meetNeighbour(t, baseURL)
	publishPosts(t, baseURL, neighbourToken, 2)
	registered := profileOK(t, getProfile(t, baseURL, neighbourToken)).CreatedAt

	resp := fetchUser(t, baseURL, token, neighbourID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на чужой профиль ожидался статус 200, получен %d", resp.StatusCode)
	}
	raw := rawJSON(t, resp)

	var profile userProfilePayload
	if err := json.Unmarshal(raw, &profile); err != nil {
		t.Fatalf("профиль пользователя не разобрался как JSON: %v", err)
	}

	if profile.ID != neighbourID {
		t.Errorf("ожидался профиль %q, получен %q", neighbourID, profile.ID)
	}
	if profile.Name != neighbourName {
		t.Errorf("ожидалось имя %q, получено %q", neighbourName, profile.Name)
	}
	if profile.About != neighbourAbout {
		t.Errorf("ожидалось «о себе» %q, получено %q", neighbourAbout, profile.About)
	}
	if profile.AvatarURL == nil || *profile.AvatarURL != avatar {
		t.Errorf("ожидался аватар %q, получен %v", avatar, profile.AvatarURL)
	}
	requireSameRegistration(t, profile.CreatedAt, registered, "чужой профиль")
	if profile.Posts != 2 {
		t.Errorf("у соседки два поста, а в профиле %d", profile.Posts)
	}

	neighbourPhones := []string{otherPhoneStored, otherPhonePretty, "79005557788", "9005557788"}
	requireNoPhone(t, string(raw), neighbourPhones, "чужой профиль")

	posts := string(rawJSON(t, fetchUserPosts(t, baseURL, token, neighbourID, nil)))
	requireNoPhone(t, posts, neighbourPhones, "посты чужого пользователя")
}

// Свой профиль открывается той же ручкой по своему идентификатору
// и выглядит ровно так же, как его видит сосед; номера телефона нет
// и в своём (ФТ-2, ФТ-8, «Свой профиль по своему идентификатору»).
func TestUserProfileOfOneselfIsTheSameAsNeighboursSeeIt(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	resp := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("профиль должен был сохраниться, получен статус %d", resp.StatusCode)
	}
	avatar := avatarOf(t, baseURL, token, imageBytes(t, "png", 300, 300))
	publishPosts(t, baseURL, token, 1)

	otherToken, _ := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, otherToken, neighbourName)

	own := fetchUser(t, baseURL, token, userID)
	if own.StatusCode != http.StatusOK {
		t.Fatalf("на свой профиль ожидался статус 200, получен %d", own.StatusCode)
	}
	raw := rawJSON(t, own)

	requireNoPhone(t, string(raw), []string{phoneStored, phonePretty, phoneDigits, "9001234567"}, "свой профиль")

	var profile userProfilePayload
	if err := json.Unmarshal(raw, &profile); err != nil {
		t.Fatalf("свой профиль не разобрался как JSON: %v", err)
	}
	if profile.ID != userID || profile.Name != profileName || profile.About != profileAbout || profile.Posts != 1 {
		t.Errorf("свой профиль отдан неверно: %+v", profile)
	}
	if profile.AvatarURL == nil || *profile.AvatarURL != avatar {
		t.Errorf("ожидался аватар %q, получен %v", avatar, profile.AvatarURL)
	}

	var mine map[string]any
	if err := json.Unmarshal(raw, &mine); err != nil {
		t.Fatalf("свой профиль не разобрался как JSON: %v", err)
	}
	seenByNeighbour := userProfileFields(t, baseURL, otherToken, userID)
	// Отношение смотрящего к человеку в своём профиле не приходит —
	// кнопки подписки на себя нет (specs/012-follows.md, «Изменения
	// в существующих»). Всё остальное должно совпадать.
	delete(seenByNeighbour, "relation")

	if !reflect.DeepEqual(mine, seenByNeighbour) {
		t.Errorf("свой профиль отличается от того, что видит сосед:\nсвой: %v\nу соседа: %v", mine, seenByNeighbour)
	}
}

// Пользователь без аватара: ссылки на аватар нет или она null
// («Пользователь без аватара»).
func TestUserProfileWithoutAvatarHasNoAvatarLink(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, otherToken, neighbourName)

	requireNoAvatar(t, userProfile(t, baseURL, token, otherID).AvatarURL)

	fields := userProfileFields(t, baseURL, token, otherID)
	if link, present := fields["avatar_url"]; present && link != nil {
		t.Errorf("аватара нет, а в профиле avatar_url = %v", link)
	}
}

// Пользователь без «о себе»: поле есть и оно пустая строка, а не null
// и не пропущено («Пользователь без «о себе»», schema UserProfile).
func TestUserProfileWithoutAboutHasEmptyAbout(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, otherToken, neighbourName)

	fields := userProfileFields(t, baseURL, token, otherID)

	about, present := fields["about"]
	if !present {
		t.Fatal("поля about в профиле нет, а оно обязательное")
	}
	if about != "" {
		t.Errorf("«о себе» не задавалось, ожидалась пустая строка, получено %#v", about)
	}
	if fields["name"] != neighbourName {
		t.Errorf("ожидалось имя %q, получено %v", neighbourName, fields["name"])
	}
}

// Пользователь без постов: в профиле posts: 0, посты — пустой список
// без курсора («Пользователь без постов», «Если у человека нет постов»).
func TestUserWithoutPostsHasZeroPostsAndAnEmptyPage(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, otherToken, neighbourName)

	// Посты в сервисе есть, но не у неё.
	publishPosts(t, baseURL, token, 2)

	fields := userProfileFields(t, baseURL, token, otherID)
	posts, present := fields["posts"]
	if !present {
		t.Fatal("поля posts в профиле нет, а оно обязательное")
	}
	if posts != float64(0) {
		t.Errorf("постов у неё нет, ожидалось posts: 0, получено %#v", posts)
	}

	resp := fetchUserPosts(t, baseURL, token, otherID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на посты пользователя без постов ожидался статус 200, получен %d", resp.StatusCode)
	}

	var body map[string]any
	decode(t, resp, &body)

	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("в ответе ожидался список items, получено %#v", body["items"])
	}
	if len(items) != 0 {
		t.Errorf("постов у неё нет, а отдано %d", len(items))
	}
	if cursor, present := body["next_cursor"]; present && cursor != nil {
		t.Errorf("у пустого списка постов курсора быть не должно, получен %v", cursor)
	}
}

// Число постов — сколько их сейчас: после публикации на один больше
// (ФТ-3, «Число постов после публикации нового»).
func TestUserPostCountGrowsAfterPublishing(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, otherToken, neighbourName)

	publishPosts(t, baseURL, otherToken, 2)
	if got := userProfile(t, baseURL, token, otherID).Posts; got != 2 {
		t.Fatalf("до публикации ожидалось два поста, получено %d", got)
	}

	publishPost(t, baseURL, otherToken, "Ещё одна грядка")

	if got := userProfile(t, baseURL, token, otherID).Posts; got != 3 {
		t.Errorf("после публикации ожидалось три поста, получено %d", got)
	}
	if got := userProfile(t, baseURL, otherToken, otherID).Posts; got != 3 {
		t.Errorf("в своём профиле после публикации ожидалось три поста, получено %d", got)
	}

	// Чужая публикация её число не трогает.
	publishPost(t, baseURL, token, "Не её пост")
	if got := userProfile(t, baseURL, token, otherID).Posts; got != 3 {
		t.Errorf("после чужой публикации её постов должно остаться три, получено %d", got)
	}
}

// Удалённый пост из числа уходит сразу, и из постов пользователя тоже
// (ФТ-3, «Число постов после удаления поста», 007-deletion).
func TestUserPostCountShrinksRightAfterDeletion(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, otherToken, neighbourName)

	published := newestFirst(publishPosts(t, baseURL, otherToken, 3))
	if got := userProfile(t, baseURL, token, otherID).Posts; got != 3 {
		t.Fatalf("до удаления ожидалось три поста, получено %d", got)
	}

	requireDeleted(t, deletePost(t, baseURL, otherToken, published[1]))

	if got := userProfile(t, baseURL, token, otherID).Posts; got != 2 {
		t.Errorf("после удаления ожидалось два поста, получено %d", got)
	}

	page := userPostsPage(t, baseURL, token, otherID, nil)
	requireFeedPosts(t, page, []string{published[0], published[2]}, "посты после удаления")
	requireFeedEnd(t, page, "посты после удаления")
}

// Профиль и посты открываются по тому самому author.id, что приходит
// с постом и комментарием (ФТ-6).
func TestUserProfileOpensByAuthorIDOfPostAndComment(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, otherToken, neighbourName)

	post := publishPost(t, baseURL, otherToken, postCaption)
	comment := createdComment(t, addCommentText(t, baseURL, token, post.ID, commentText))

	fromPost := userProfile(t, baseURL, token, post.Author.ID)
	if fromPost.ID != otherID || fromPost.Name != neighbourName {
		t.Errorf("по author.id поста ожидался профиль %s (%s), получен %s (%s)",
			otherID, neighbourName, fromPost.ID, fromPost.Name)
	}
	fromPostPosts := userPostsPage(t, baseURL, token, post.Author.ID, nil)
	requireFeedPosts(t, fromPostPosts, []string{post.ID}, "посты по author.id поста")

	fromComment := userProfile(t, baseURL, otherToken, comment.Author.ID)
	if fromComment.ID != userID || fromComment.Name != profileName {
		t.Errorf("по author.id комментария ожидался профиль %s (%s), получен %s (%s)",
			userID, profileName, fromComment.ID, fromComment.Name)
	}
	if fromComment.Posts != 0 {
		t.Errorf("у автора комментария постов нет, а в профиле %d", fromComment.Posts)
	}
}

// Пользователя нет или идентификатор не UUID — 404 user_not_found и для
// профиля, и для постов: для клиента это одно и то же (ФТ-7,
// «Неизвестный идентификатор», «Идентификатор не UUID»).
func TestUnknownUserIsNotFound(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	post := publishPost(t, baseURL, token, postCaption)

	cases := []struct {
		name   string
		userID string
	}{
		{"UUID, которого нет", unknownID},
		{"идентификатор поста вместо пользователя", post.ID},
		{"не UUID", "abc"},
		{"не UUID по-русски", notAnID},
	}

	for _, testCase := range cases {
		t.Run(testCase.name+": профиль", func(t *testing.T) {
			requireError(t, fetchUser(t, baseURL, token, testCase.userID), http.StatusNotFound, "user_not_found")
		})
		t.Run(testCase.name+": посты", func(t *testing.T) {
			requireError(t, fetchUserPosts(t, baseURL, token, testCase.userID, nil), http.StatusNotFound, "user_not_found")
		})
	}
}

// --- GET /api/users/{userId}/posts: посты пользователя --------------------

// Только его посты, новые сверху, чужих в ответе нет — даже когда они
// выложены вперемешку (ФТ-4, «Посты пользователя»).
func TestUserPostsAreOnlyTheirsNewestFirst(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, otherToken, neighbourName)

	var mine, hers []string
	for i := 1; i <= 3; i++ {
		hers = append(hers, publishPost(t, baseURL, otherToken, fmt.Sprintf("Её грядка № %d", i)).ID)
		mine = append(mine, publishPost(t, baseURL, token, fmt.Sprintf("Моя грядка № %d", i)).ID)
	}

	herPage := userPostsPage(t, baseURL, token, otherID, nil)
	requireFeedPosts(t, herPage, newestFirst(hers), "её посты")
	requireAuthoredBy(t, herPage, otherID, "её посты")
	requireFeedEnd(t, herPage, "её посты")

	myPage := userPostsPage(t, baseURL, token, userID, nil)
	requireFeedPosts(t, myPage, newestFirst(mine), "мои посты")
	requireAuthoredBy(t, myPage, userID, "мои посты")
	requireFeedEnd(t, myPage, "мои посты")

	// Время публикации не растёт сверху вниз.
	for i := 1; i < len(herPage.Items); i++ {
		previous, current := herPage.Items[i-1].CreatedAt, herPage.Items[i].CreatedAt
		if previous < current {
			t.Errorf("пост %d опубликован в %s, а стоит выше поста от %s", i+1, current, previous)
		}
	}
}

// Одинаковое created_at: порядок по идентификатору, тоже по убыванию,
// и проход страницами ничего не теряет и не задваивает («Порядок постов»).
func TestUserPostsKeepStableOrderForTheSameCreatedAt(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	introduce(t, baseURL, otherToken, neighbourName)

	published := publishPosts(t, baseURL, otherToken, 4)
	setSameCreatedAt(t, published)

	want := append([]string(nil), published...)
	sort.Sort(sort.Reverse(sort.StringSlice(want)))

	page := userPostsPage(t, baseURL, token, otherID, nil)
	requireFeedPosts(t, page, want, "посты с одинаковым временем")

	for _, limit := range []int{1, 3} {
		walked := walkUserPosts(t, baseURL, token, otherID, limit)
		if !reflect.DeepEqual(walked, want) {
			t.Errorf("проход страницами по %d дал %v, ожидалось %v", limit, walked, want)
		}
	}
}

// Постов меньше, чем limit: все одной страницей и без курсора
// («Постов меньше, чем limit»).
func TestUserPostsFewerThanLimitComeInOnePage(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	published := newestFirst(publishPosts(t, baseURL, otherToken, 3))

	page := userPostsPage(t, baseURL, token, otherID, feedParams(10, ""))
	requireFeedPosts(t, page, published, "страница на десять постов")
	requireFeedEnd(t, page, "три поста при limit=10")
}

// Постов ровно limit: страница без курсора, хотя в сервисе есть и другие
// посты — чужие не считаются («Постов ровно limit»).
func TestUserPostsExactlyLimitHaveNoCursor(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)

	// Чужой пост старше её постов: будь он учтён, курсор бы появился.
	publishPost(t, baseURL, token, "Мой старый пост")
	published := newestFirst(publishPosts(t, baseURL, otherToken, 3))

	page := userPostsPage(t, baseURL, token, otherID, feedParams(3, ""))
	requireFeedPosts(t, page, published, "страница ровно на три поста")
	requireFeedEnd(t, page, "три поста при limit=3")
}

// Постов больше, чем limit: страница и курсор; курсор ведёт к постам
// этого же человека строго старше последнего отданного — чужие посты
// между ними не попадают (ФТ-4, «Постов больше, чем limit», «Курсор из
// предыдущего ответа»).
func TestUserPostsMoreThanLimitReturnCursorToOlderPostsOfTheSamePerson(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)

	var hers []string
	for i := 1; i <= 5; i++ {
		publishPost(t, baseURL, token, fmt.Sprintf("Мой пост № %d", i))
		hers = append(hers, publishPost(t, baseURL, otherToken, fmt.Sprintf(feedCaption, i)).ID)
	}
	hers = newestFirst(hers)

	first := userPostsPage(t, baseURL, token, otherID, feedParams(2, ""))
	requireFeedPosts(t, first, hers[:2], "первая страница")
	cursor := cursorOf(t, first, "первая страница")

	second := userPostsPage(t, baseURL, token, otherID, feedParams(2, cursor))
	requireFeedPosts(t, second, hers[2:4], "вторая страница")
	requireAuthoredBy(t, second, otherID, "вторая страница")
	cursor = cursorOf(t, second, "вторая страница")

	third := userPostsPage(t, baseURL, token, otherID, feedParams(2, cursor))
	requireFeedPosts(t, third, hers[4:], "третья страница")
	requireFeedEnd(t, third, "третья страница")

	walked := walkUserPosts(t, baseURL, token, otherID, 1)
	if !reflect.DeepEqual(walked, hers) {
		t.Errorf("проход по одному посту дал %v, ожидалось %v", walked, hers)
	}
}

// Новый пост человека между запросами страниц не появляется во второй
// странице и ничего не сдвигает; в первой странице заново он первый
// (ФТ-5, «Новый пост человека между запросами страниц»).
func TestUserPostsNewPostBetweenPagesDoesNotShiftTheSecondPage(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	published := newestFirst(publishPosts(t, baseURL, otherToken, 4))

	first := userPostsPage(t, baseURL, token, otherID, feedParams(2, ""))
	requireFeedPosts(t, first, published[:2], "первая страница")
	cursor := cursorOf(t, first, "первая страница")

	// Пока сосед смотрит сетку, она выкладывает новый пост.
	fresh := publishPost(t, baseURL, otherToken, "Свежий пост").ID

	second := userPostsPage(t, baseURL, token, otherID, feedParams(2, cursor))
	requireFeedPosts(t, second, published[2:], "вторая страница после новой публикации")
	requireFeedEnd(t, second, "вторая страница после новой публикации")

	refreshed := userPostsPage(t, baseURL, token, otherID, feedParams(2, ""))
	requireFeedPosts(t, refreshed, []string{fresh, published[0]}, "первая страница заново")
}

// Курсор — место в порядке постов, а не номер страницы и не ссылка на
// строку: если пост, на котором остановились, удалён, продолжение берётся
// от того же места (ФТ-5).
func TestUserPostsCursorWorksWhenItsPostIsDeleted(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	published := newestFirst(publishPosts(t, baseURL, otherToken, 5))

	first := userPostsPage(t, baseURL, token, otherID, feedParams(2, ""))
	requireFeedPosts(t, first, published[:2], "первая страница")
	cursor := cursorOf(t, first, "первая страница")

	// Удаляются и пост, на котором остановились, и самый новый: номер
	// страницы от этого бы съехал, место в порядке — нет.
	requireDeleted(t, deletePost(t, baseURL, otherToken, published[1]))
	requireDeleted(t, deletePost(t, baseURL, otherToken, published[0]))

	second := userPostsPage(t, baseURL, token, otherID, feedParams(2, cursor))
	requireFeedPosts(t, second, published[2:4], "вторая страница после удаления")
	cursorOf(t, second, "вторая страница после удаления")
}

// Курсор ленты и курсор постов пользователя устроены одинаково (ФТ-5):
// курсор из ленты — тоже место в общем порядке постов, и посты человека
// по нему продолжаются с того же места.
func TestUserPostsCursorIsTheSameKindAsTheFeedCursor(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)

	var hers []string
	for i := 1; i <= 3; i++ {
		hers = append(hers, publishPost(t, baseURL, otherToken, fmt.Sprintf(feedCaption, i)).ID)
		publishPost(t, baseURL, token, fmt.Sprintf("Мой пост № %d", i))
	}
	// Порядок публикации: её 1, мой 1, её 2, мой 2, её 3, мой 3.
	// Первые три поста ленты: мой 3, её 3, мой 2 — курсор встаёт на «мой 2».
	feed := feedPage(t, baseURL, token, feedParams(3, ""))
	cursor := cursorOf(t, feed, "первая страница ленты")

	page := userPostsPage(t, baseURL, token, otherID, feedParams(0, cursor))
	requireFeedPosts(t, page, []string{hers[1], hers[0]}, "её посты по курсору ленты")
	requireFeedEnd(t, page, "её посты по курсору ленты")
}

// Запрос без limit отдаёт 20 постов (ФТ-4, «Запрос без limit»).
func TestUserPostsWithoutLimitReturnTwenty(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	published := newestFirst(publishPosts(t, baseURL, otherToken, feedDefaultLimit+1))

	page := userPostsPage(t, baseURL, token, otherID, nil)
	if len(page.Items) != feedDefaultLimit {
		t.Fatalf("без limit ожидалось %d постов, получено %d", feedDefaultLimit, len(page.Items))
	}
	requireFeedPosts(t, page, published[:feedDefaultLimit], "страница по умолчанию")

	last := userPostsPage(t, baseURL, token, otherID, feedParams(0, cursorOf(t, page, "страница по умолчанию")))
	requireFeedPosts(t, last, published[feedDefaultLimit:], "вторая страница по умолчанию")
	requireFeedEnd(t, last, "вторая страница по умолчанию")
}

// limit от 1 до 50 принимается, остальное — 400 invalid_request: ноль,
// больше пятидесяти, не число, пустой или указанный дважды (ФТ-4,
// «limit=0, limit=51, limit=abc, пустой или дважды», «Ошибки»).
func TestUserPostsLimitMustBeFromOneToFifty(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	published := newestFirst(publishPosts(t, baseURL, otherToken, 2))

	rejected := []struct {
		name  string
		query string
	}{
		{"ноль", "limit=0"},
		{"больше пятидесяти", "limit=51"},
		{"отрицательный", "limit=-1"},
		{"не число", "limit=abc"},
		{"пустой", "limit="},
		{"дважды", "limit=1&limit=2"},
	}

	for _, testCase := range rejected {
		t.Run(testCase.name, func(t *testing.T) {
			resp := fetchUserPostsRaw(t, baseURL, token, otherID, testCase.query)
			requireError(t, resp, http.StatusBadRequest, "invalid_request")
		})
	}

	t.Run("один", func(t *testing.T) {
		page := userPostsOK(t, fetchUserPostsRaw(t, baseURL, token, otherID, "limit=1"))
		requireFeedPosts(t, page, published[:1], "limit=1")
		cursorOf(t, page, "limit=1")
	})
	t.Run("пятьдесят", func(t *testing.T) {
		page := userPostsOK(t, fetchUserPostsRaw(t, baseURL, token, otherID, "limit=50"))
		requireFeedPosts(t, page, published, "limit=50")
		requireFeedEnd(t, page, "limit=50")
	})
}

// Испорченный курсор — 400 invalid_cursor («Испорченный курсор»).
func TestUserPostsRejectBrokenCursor(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	published := publishPosts(t, baseURL, otherToken, 3)

	issued := cursorOf(t, userPostsPage(t, baseURL, token, otherID, feedParams(1, "")), "первая страница")
	if len(issued) < 4 {
		t.Fatalf("курсор %q подозрительно короткий: тест собран неправильно", issued)
	}

	cases := []struct {
		name   string
		cursor string
	}{
		{"не курсор вовсе", "курсор"},
		{"мусор в base64", "0L3QtSDQutGD0YDRgdC-0YA="},
		{"обрезанный настоящий курсор", issued[:len(issued)-3]},
		{"идентификатор поста вместо курсора", published[0]},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			resp := fetchUserPostsRaw(t, baseURL, token, otherID, "cursor="+url.QueryEscape(testCase.cursor))
			requireError(t, resp, http.StatusBadRequest, "invalid_cursor")
		})
	}
}

// Пустой cursor= — как будто курсора нет: первая страница
// («Пустой cursor=»).
func TestUserPostsWithEmptyCursorReturnFirstPage(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)
	published := newestFirst(publishPosts(t, baseURL, otherToken, 3))

	page := userPostsOK(t, fetchUserPostsRaw(t, baseURL, token, otherID, "cursor="))
	requireFeedPosts(t, page, published, "посты с пустым курсором")
	requireFeedEnd(t, page, "посты с пустым курсором")

	withLimit := userPostsOK(t, fetchUserPostsRaw(t, baseURL, token, otherID, "limit=2&cursor="))
	requireFeedPosts(t, withLimit, published[:2], "первая страница с пустым курсором")
	cursorOf(t, withLimit, "первая страница с пустым курсором")
}

// Пост в ответе — целиком, как в ленте и по своему адресу: автор, все
// фотографии по порядку, лайки, «я отметил» того, кто спрашивает, и число
// комментариев (ФТ-4, «Пост в ответе»).
func TestUserPostsItemIsTheWholePostJustLikeInTheFeed(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	otherToken, otherID, avatar := meetNeighbour(t, baseURL)

	photos := []mediaPayload{
		photoOf(t, baseURL, otherToken, 400, 300),
		photoOf(t, baseURL, otherToken, 300, 400),
		photoOf(t, baseURL, otherToken, 200, 200),
		photoOf(t, baseURL, otherToken, 640, 480),
	}
	mediaIDs := make([]string, 0, len(photos))
	for _, photo := range photos {
		mediaIDs = append(mediaIDs, photo.ID)
	}
	created := createdPost(t, createPost(t, baseURL, otherToken,
		map[string]any{"media_ids": mediaIDs, "caption": postCaption}))

	// Смотрящий отметил пост и прокомментировал его.
	likedOK(t, likePost(t, baseURL, token, created.ID), http.StatusOK)
	createdComment(t, addCommentText(t, baseURL, token, created.ID, commentText))

	resp := fetchUserPosts(t, baseURL, token, otherID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("на посты пользователя ожидался статус 200, получен %d", resp.StatusCode)
	}
	var raw feedRawPayload
	decode(t, resp, &raw)
	if len(raw.Items) != 1 {
		t.Fatalf("у неё один пост, а отдано %d", len(raw.Items))
	}

	var fromProfile map[string]any
	if err := json.Unmarshal(raw.Items[0], &fromProfile); err != nil {
		t.Fatalf("пост из профиля не разобрался как JSON: %v", err)
	}

	var fromPage map[string]any
	if err := json.Unmarshal(rawJSON(t, fetchPost(t, baseURL, token, created.ID)), &fromPage); err != nil {
		t.Fatalf("пост по своему адресу не разобрался как JSON: %v", err)
	}
	if !reflect.DeepEqual(fromProfile, fromPage) {
		t.Errorf("пост в профиле отличается от поста по своему адресу:\nв профиле: %v\nпо адресу: %v", fromProfile, fromPage)
	}

	feedResp := fetchFeed(t, baseURL, token, nil)
	if feedResp.StatusCode != http.StatusOK {
		t.Fatalf("на страницу ленты ожидался статус 200, получен %d", feedResp.StatusCode)
	}
	var feed feedRawPayload
	decode(t, feedResp, &feed)
	if len(feed.Items) != 1 {
		t.Fatalf("в ленте ожидался один пост, получено %d", len(feed.Items))
	}
	var fromFeed map[string]any
	if err := json.Unmarshal(feed.Items[0], &fromFeed); err != nil {
		t.Fatalf("пост из ленты не разобрался как JSON: %v", err)
	}
	if !reflect.DeepEqual(fromProfile, fromFeed) {
		t.Errorf("пост в профиле отличается от поста в ленте:\nв профиле: %v\nв ленте: %v", fromProfile, fromFeed)
	}

	// И сами поля — на случай, если все три места ошибаются одинаково.
	var item struct {
		postPayload
		Likes    int  `json:"likes"`
		Liked    bool `json:"liked"`
		Comments int  `json:"comments"`
	}
	if err := json.Unmarshal(raw.Items[0], &item); err != nil {
		t.Fatalf("пост из профиля не разобрался как пост: %v", err)
	}

	requireMediaOrder(t, item.postPayload, photos)
	if item.Caption != postCaption {
		t.Errorf("ожидалась подпись %q, получена %q", postCaption, item.Caption)
	}
	if item.Author.ID != otherID || item.Author.Name != neighbourName {
		t.Errorf("ожидался автор %s (%s), получен %s (%s)", otherID, neighbourName, item.Author.ID, item.Author.Name)
	}
	if item.Author.AvatarURL == nil || *item.Author.AvatarURL != avatar {
		t.Errorf("ожидался аватар автора %q, получен %v", avatar, item.Author.AvatarURL)
	}
	if item.Likes != 1 || !item.Liked {
		t.Errorf("ожидался один лайк и liked=true, получено %d и %t", item.Likes, item.Liked)
	}
	if item.Comments != 1 {
		t.Errorf("ожидался один комментарий, получено %d", item.Comments)
	}

	// «Я отметил» — для того, кто спрашивает: сама она пост не отмечала.
	var ownRaw feedRawPayload
	decode(t, fetchUserPosts(t, baseURL, otherToken, otherID, nil), &ownRaw)
	if len(ownRaw.Items) != 1 {
		t.Fatalf("в своих постах ожидался один пост, получено %d", len(ownRaw.Items))
	}
	var ownItem struct {
		Likes int  `json:"likes"`
		Liked bool `json:"liked"`
	}
	if err := json.Unmarshal(ownRaw.Items[0], &ownItem); err != nil {
		t.Fatalf("пост из своего профиля не разобрался: %v", err)
	}
	if ownItem.Likes != 1 || ownItem.Liked {
		t.Errorf("автору ожидался один лайк и liked=false, получено %d и %t", ownItem.Likes, ownItem.Liked)
	}
}

// --- Доступ -----------------------------------------------------------------

// Без токена или с неизвестным токеном — 401 unauthorized и для профиля,
// и для постов, в том числе для пользователя, которого нет: сервис закрыт
// целиком (ФТ-1, «Любой запрос без токена или с неизвестным токеном»).
func TestUserProfileAndPostsRequireValidToken(t *testing.T) {
	baseURL := startAPI(t)

	_, userID := signIn(t, baseURL, phonePretty)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)
	publishPosts(t, baseURL, otherToken, 1)

	tokens := []struct {
		name  string
		token string
	}{
		{"без токена", ""},
		{"неизвестный токен", "этого-токена-сервис-не-выдавал"},
	}
	users := []struct {
		name   string
		userID string
	}{
		{"существующий пользователь", userID},
		{"неизвестный пользователь", unknownID},
		{"не UUID", "abc"},
	}

	for _, token := range tokens {
		for _, user := range users {
			t.Run(token.name+", "+user.name+": профиль", func(t *testing.T) {
				requireError(t, fetchUser(t, baseURL, token.token, user.userID), http.StatusUnauthorized, "unauthorized")
			})
			t.Run(token.name+", "+user.name+": посты", func(t *testing.T) {
				resp := fetchUserPosts(t, baseURL, token.token, user.userID, feedParams(10, ""))
				requireError(t, resp, http.StatusUnauthorized, "unauthorized")
			})
		}
	}
}
