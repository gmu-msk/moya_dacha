package tests

// Тесты по specs/034-photo-captions.md, требования 1–11: подпись автора
// к каждому фото поста — при создании, при правке, в ответах и в тэгах.
// Написаны по спецификации и контракту, без взгляда на реализацию
// (ADR-0002). Требования 12–15 — про приложение, здесь их нет.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"
)

// Подпись второго фото из пользовательского сценария.
const pcSortCaption = "Сорт Чудо, третий год"

// --- Представления из контракта -------------------------------------------

// pcMedia — фото поста (schema Media) с подписью. hasCaption — был ли
// в ответе ключ caption вообще: «нет подписи — поля нет» (требование 3),
// поэтому null и пустая строка считаются нарушением, а не отсутствием.
type pcMedia struct {
	ID         string
	Caption    *string
	hasCaption bool
}

func (m *pcMedia) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}

	*m = pcMedia{}
	if id, ok := fields["id"]; ok {
		if err := json.Unmarshal(id, &m.ID); err != nil {
			return err
		}
	}
	if caption, ok := fields["caption"]; ok {
		m.hasCaption = true
		if err := json.Unmarshal(caption, &m.Caption); err != nil {
			return err
		}
	}

	return nil
}

// pcPost — пост с полями, важными этой фиче (schema Post).
type pcPost struct {
	ID        string    `json:"id"`
	CreatedAt string    `json:"created_at"`
	Caption   string    `json:"caption"`
	EditedAt  *string   `json:"edited_at"`
	Tags      *[]string `json:"tags"`
	Media     []pcMedia `json:"media"`
}

// pcFeed — страница ленты, постов пользователя или группы (schema Feed).
type pcFeed struct {
	Items      []pcPost `json:"items"`
	NextCursor *string  `json:"next_cursor"`
}

// --- Хелперы --------------------------------------------------------------

// pcPhone — номер n-го участника теста; не пересекается с другими файлами.
func pcPhone(n int) string {
	return fmt.Sprintf("+7 (900) 734-00-%02d", n)
}

// newPCUser регистрирует n-го участника теста.
func newPCUser(t *testing.T, baseURL string, n int) dachnik {
	t.Helper()

	token, id := signIn(t, baseURL, pcPhone(n))

	return dachnik{token: token, id: id}
}

// pcPhotos загружает count фотографий автора.
func pcPhotos(t *testing.T, baseURL, token string, count int) []mediaPayload {
	t.Helper()

	photos := make([]mediaPayload, 0, count)
	for i := 0; i < count; i++ {
		photos = append(photos, photoOf(t, baseURL, token, 60, 40))
	}

	return photos
}

// pcIDs — идентификаторы фотографий по порядку.
func pcIDs(photos []mediaPayload) []string {
	ids := make([]string, 0, len(photos))
	for _, p := range photos {
		ids = append(ids, p.ID)
	}

	return ids
}

// pcDraft — тело POST /posts из этих фото и подписи; captions == nil —
// поля media_captions нет.
func pcDraft(photos []mediaPayload, caption string, captions map[string]string) map[string]any {
	body := map[string]any{"media_ids": pcIDs(photos), "caption": caption}
	if captions != nil {
		body["media_captions"] = captions
	}

	return body
}

// pcPostOK требует статус и возвращает пост.
func pcPostOK(t *testing.T, resp *http.Response, status int, where string) pcPost {
	t.Helper()

	if resp.StatusCode != status {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус %d, получен %d %s", where, status, resp.StatusCode, code)
	}

	var post pcPost
	decode(t, resp, &post)

	return post
}

// pcPublish публикует пост из этих фото с подписями и требует 201.
func pcPublish(t *testing.T, baseURL, token string, photos []mediaPayload, caption string, captions map[string]string) pcPost {
	t.Helper()
	return pcPostOK(t, createPost(t, baseURL, token, pcDraft(photos, caption, captions)),
		http.StatusCreated, "публикация поста")
}

// pcEdit правит подписи своего поста (body — как есть) и требует 200
// с тем же постом.
func pcEdit(t *testing.T, baseURL, token, postID string, body any, where string) pcPost {
	t.Helper()

	post := pcPostOK(t, editCaptionReq(t, baseURL, token, postID, body), http.StatusOK, where)
	if post.ID != postID {
		t.Fatalf("%s: правка поста %s вернула пост %s", where, postID, post.ID)
	}

	return post
}

// pcGet открывает пост по адресу и требует 200.
func pcGet(t *testing.T, baseURL, token, postID string) pcPost {
	t.Helper()
	return pcPostOK(t, fetchPost(t, baseURL, token, postID), http.StatusOK, "GET /posts/"+postID)
}

// pcPage требует 200 и возвращает страницу.
func pcPage(t *testing.T, resp *http.Response, where string) pcFeed {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		code := ""
		if resp.StatusCode >= 400 {
			code = errorCode(t, resp)
		}
		t.Fatalf("%s: ожидался статус 200, получен %d %s", where, resp.StatusCode, code)
	}

	var page pcFeed
	decode(t, resp, &page)

	return page
}

// pcFeedPage — первая страница ленты «Все» (limit 50).
func pcFeedPage(t *testing.T, baseURL, token string) pcFeed {
	t.Helper()
	return pcPage(t, fetchFeed(t, baseURL, token, feedParams(50, "")), "лента")
}

// pcTagFeed — первая страница ленты по тэгу (limit 50).
func pcTagFeed(t *testing.T, baseURL, token, tag string) pcFeed {
	t.Helper()

	params := url.Values{"limit": {"50"}, "tag": {tag}}

	return pcPage(t, fetchFeedRaw(t, baseURL, token, params.Encode()), "лента ?"+params.Encode())
}

// pcUserPage — первая страница постов пользователя (limit 50).
func pcUserPage(t *testing.T, baseURL, token, userID string) pcFeed {
	t.Helper()
	return pcPage(t, fetchUserPosts(t, baseURL, token, userID, feedParams(50, "")), "посты пользователя")
}

// pcFind — пост с этим id на странице или nil.
func pcFind(page pcFeed, postID string) *pcPost {
	for i := range page.Items {
		if page.Items[i].ID == postID {
			return &page.Items[i]
		}
	}

	return nil
}

// pcRequireFound требует пост на странице и возвращает его.
func pcRequireFound(t *testing.T, page pcFeed, postID, where string) pcPost {
	t.Helper()

	found := pcFind(page, postID)
	if found == nil {
		t.Fatalf("%s: поста %s нет", where, postID)
	}

	return *found
}

// pcRequireCaptions требует у поста ровно эти фото по порядку и у каждого
// эту подпись; пустая строка в want — у фото подписи нет, и поля caption
// в ответе нет вовсе (требование 3).
func pcRequireCaptions(t *testing.T, post pcPost, photos []mediaPayload, want []string, where string) {
	t.Helper()

	if len(post.Media) != len(photos) {
		t.Fatalf("%s: у поста %d фото, ожидалось %d", where, len(post.Media), len(photos))
	}

	for i, m := range post.Media {
		if m.ID != photos[i].ID {
			t.Errorf("%s: media[%d] — фото %s, ожидалось %s", where, i, m.ID, photos[i].ID)
		}

		switch {
		case want[i] == "" && m.hasCaption:
			got := "null"
			if m.Caption != nil {
				got = fmt.Sprintf("%q", *m.Caption)
			}
			t.Errorf("%s: у media[%d] подписи быть не должно, а поле caption есть: %s", where, i, got)
		case want[i] != "" && (m.Caption == nil):
			t.Errorf("%s: у media[%d] ожидалась подпись %q, а её нет", where, i, want[i])
		case want[i] != "" && *m.Caption != want[i]:
			t.Errorf("%s: у media[%d] подпись %q, ожидалась %q", where, i, *m.Caption, want[i])
		}
	}
}

// pcRequireTags требует у поста ровно эти тэги в этом порядке.
func pcRequireTags(t *testing.T, post pcPost, want []string, where string) {
	t.Helper()

	if post.Tags == nil {
		t.Errorf("%s: у поста %s нет поля tags, ожидалось %q", where, post.ID, want)
		return
	}
	if !slices.Equal(*post.Tags, want) && !(len(*post.Tags) == 0 && len(want) == 0) {
		t.Errorf("%s: tags = %q, ожидалось %q", where, *post.Tags, want)
	}
}

// pcRequireStays требует, чтобы после отказа у поста остались прежние
// подпись, подписи фото и edited_at.
func pcRequireStays(t *testing.T, baseURL, token string, before pcPost, photos []mediaPayload, want []string, where string) {
	t.Helper()

	got := pcGet(t, baseURL, token, before.ID)
	if got.Caption != before.Caption {
		t.Errorf("%s: подпись поста стала %q, а была %q", where, got.Caption, before.Caption)
	}
	pcRequireCaptions(t, got, photos, want, where)

	switch {
	case before.EditedAt == nil && got.EditedAt != nil:
		t.Errorf("%s: появилось edited_at = %q", where, *got.EditedAt)
	case before.EditedAt != nil && (got.EditedAt == nil || *got.EditedAt != *before.EditedAt):
		t.Errorf("%s: edited_at сдвинулось: было %q, стало %v", where, *before.EditedAt, got.EditedAt)
	}
}

// pcRequireMediaFree требует, что фотографии после отказа не опубликованы:
// из них можно собрать пост (003-posts: «ни одна не публикуется»).
func pcRequireMediaFree(t *testing.T, baseURL, token string, photos []mediaPayload, where string) {
	t.Helper()

	resp := createPost(t, baseURL, token, map[string]any{"media_ids": pcIDs(photos)})
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("%s: фото после отказа не публикуются снова: статус %d %s", where, resp.StatusCode, errorCode(t, resp))
	}
}

// --- Подпись фото при создании (требования 1, 3, 5, 6) ---------------------

// Пост из трёх фото, подпись у второго: 201, у media[1] — подпись,
// у остальных поля нет; общая подпись поста — как была. То же — в ответе
// на GET автору и постороннему, в ленте и в постах профиля (требования 3, 4).
func TestPhotoCaptionsOnSecondOfThree(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)
	reader := newPCUser(t, baseURL, 2)

	photos := pcPhotos(t, baseURL, author.token, 3)
	created := pcPublish(t, baseURL, author.token, photos, postCaption,
		map[string]string{photos[1].ID: pcSortCaption})

	want := []string{"", pcSortCaption, ""}
	pcRequireCaptions(t, created, photos, want, "ответ на создание")
	if created.Caption != postCaption {
		t.Errorf("ответ на создание: подпись поста %q, ожидалась %q", created.Caption, postCaption)
	}
	requireNotEdited(t, created.EditedAt, "ответ на создание")

	for _, view := range []struct {
		name string
		post pcPost
	}{
		{"GET /posts/{id} автором", pcGet(t, baseURL, author.token, created.ID)},
		{"GET /posts/{id} читателем", pcGet(t, baseURL, reader.token, created.ID)},
		{"лента читателя", pcRequireFound(t, pcFeedPage(t, baseURL, reader.token), created.ID, "лента читателя")},
		{"посты в профиле", pcRequireFound(t, pcUserPage(t, baseURL, reader.token, author.id), created.ID, "посты в профиле")},
	} {
		pcRequireCaptions(t, view.post, photos, want, view.name)
		if view.post.Caption != postCaption {
			t.Errorf("%s: подпись поста %q, ожидалась %q", view.name, view.post.Caption, postCaption)
		}
	}
}

// Подпись фото видна и в ленте группы, куда выложен пост (требование 3).
func TestPhotoCaptionsInGroupFeed(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)

	g := grInterestGroup(t, baseURL, author, "Сортоиспытатели")
	photos := pcPhotos(t, baseURL, author.token, 2)

	body := pcDraft(photos, postCaption, map[string]string{photos[0].ID: pcSortCaption})
	body["group_ids"] = []string{g.ID}
	created := pcPostOK(t, createPost(t, baseURL, author.token, body), http.StatusCreated, "публикация в группу")

	want := []string{pcSortCaption, ""}
	pcRequireCaptions(t, created, photos, want, "ответ на создание")

	page := pcPage(t, gpGroupPostsReq(t, baseURL, author.token, g.ID, "limit=50"), "лента группы")
	pcRequireCaptions(t, pcRequireFound(t, page, created.ID, "лента группы"), photos, want, "лента группы")
}

// Без media_captions, с null и с {} — все фото без подписей, как раньше,
// и поля caption у media нет (требования 3, 5).
func TestPhotoCaptionsAbsentAsBefore(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)

	cases := []struct {
		name  string
		extra map[string]any
	}{
		{"поля нет", map[string]any{}},
		{"null", map[string]any{"media_captions": nil}},
		{"пустой объект", map[string]any{"media_captions": map[string]string{}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			photos := pcPhotos(t, baseURL, author.token, 2)
			body := map[string]any{"media_ids": pcIDs(photos), "caption": postCaption}
			for k, v := range c.extra {
				body[k] = v
			}

			created := pcPostOK(t, createPost(t, baseURL, author.token, body), http.StatusCreated, c.name)
			pcRequireCaptions(t, created, photos, []string{"", ""}, c.name+": ответ на создание")
			pcRequireCaptions(t, pcGet(t, baseURL, author.token, created.ID), photos, []string{"", ""}, c.name+": GET")
		})
	}
}

// Подпись фото — как подпись поста: края обрезаются, переводы строк внутри
// остаются, одни пробелы — подписи нет, 1000 знаков после обрезки —
// можно (требование 1).
func TestPhotoCaptionsTrimmedLikePostCaption(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)

	exactly := repeatRunes("ё", captionLimit)
	multiline := "Сорт Чудо\nтретий год"

	cases := []struct {
		name string
		sent string
		want string
	}{
		{"края обрезаются", "  \n " + pcSortCaption + " \t\n", pcSortCaption},
		{"переводы строк внутри остаются", multiline, multiline},
		{"одни пробелы — подписи нет", "   \n\t  ", ""},
		{"пустая строка — подписи нет", "", ""},
		{"ровно 1000 знаков после обрезки", "   " + exactly + "   ", exactly},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			photos := pcPhotos(t, baseURL, author.token, 1)
			created := pcPublish(t, baseURL, author.token, photos, "", map[string]string{photos[0].ID: c.sent})

			pcRequireCaptions(t, created, photos, []string{c.want}, "ответ на создание")
			pcRequireCaptions(t, pcGet(t, baseURL, author.token, created.ID), photos, []string{c.want}, "GET")
		})
	}

	// В модели пустой строки не бывает: подписи нет — NULL (раздел
	// «Модель данных»).
	if n := countSQL(t, "SELECT count(*) FROM media WHERE caption IS NOT NULL AND btrim(caption) = ''"); n != 0 {
		t.Errorf("в media.caption лежат пустые подписи: %d", n)
	}
}

// Подпись фото длиннее 1000 знаков после обрезки — 400 invalid_caption,
// пост не создаётся, фото остаются свободными (требование 6).
func TestPhotoCaptionsTooLongIsRejected(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)

	photos := pcPhotos(t, baseURL, author.token, 2)
	resp := createPost(t, baseURL, author.token, pcDraft(photos, postCaption,
		map[string]string{photos[1].ID: repeatRunes("ы", captionLimit+1)}))
	requireCodeE(t, resp, http.StatusBadRequest, "invalid_caption", "подпись фото в 1001 знак")

	if page := pcUserPage(t, baseURL, author.token, author.id); len(page.Items) != 0 {
		t.Errorf("после отказа у автора появились посты: %d", len(page.Items))
	}
	pcRequireMediaFree(t, baseURL, author.token, photos, "после длинной подписи фото")
}

// Ключ media_captions не из media_ids — 400 invalid_media, пост не
// создаётся (требование 5).
func TestPhotoCaptionsKeyNotInMediaIDsIsRejected(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)
	other := newPCUser(t, baseURL, 2)

	otherPhoto := photoOf(t, baseURL, other.token, 60, 40)

	cases := []struct {
		name string
		key  func(t *testing.T) string
	}{
		{"своё загруженное фото, не из media_ids", func(t *testing.T) string {
			return photoOf(t, baseURL, author.token, 60, 40).ID
		}},
		{"чужое фото", func(t *testing.T) string { return otherPhoto.ID }},
		{"фото, которого нет", func(t *testing.T) string { return unknownID }},
		{"не UUID", func(t *testing.T) string { return "не-фото" }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			photos := pcPhotos(t, baseURL, author.token, 2)
			captions := map[string]string{photos[0].ID: pcSortCaption, c.key(t): "Лишняя подпись"}

			resp := createPost(t, baseURL, author.token, pcDraft(photos, postCaption, captions))
			requireCodeE(t, resp, http.StatusBadRequest, "invalid_media", c.name)
			pcRequireMediaFree(t, baseURL, author.token, photos, c.name)
		})
	}
}

// Порядок проверок при создании: число фото, подпись поста, подписи фото
// (длина), потом сами фото и ключи media_captions (требование 7).
func TestPhotoCaptionsCheckOrderOnCreate(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)

	tooLong := repeatRunes("ы", captionLimit+1)

	t.Run("нет фото и длинная подпись фото — no_media", func(t *testing.T) {
		resp := createPost(t, baseURL, author.token, map[string]any{
			"media_ids":      []string{},
			"media_captions": map[string]string{unknownID: tooLong},
		})
		requireCodeE(t, resp, http.StatusBadRequest, "no_media", "пустой media_ids")
	})

	t.Run("одиннадцать фото и длинная подпись фото — too_many_media", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 11)
		resp := createPost(t, baseURL, author.token, pcDraft(photos, "",
			map[string]string{photos[0].ID: tooLong}))
		requireCodeE(t, resp, http.StatusBadRequest, "too_many_media", "11 фото")
	})

	t.Run("длинная подпись поста и ключ не из media_ids — invalid_caption", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 1)
		resp := createPost(t, baseURL, author.token, pcDraft(photos, tooLong,
			map[string]string{unknownID: pcSortCaption}))
		requireCodeE(t, resp, http.StatusBadRequest, "invalid_caption", "длинная подпись поста")
	})

	t.Run("длинная подпись фото и несуществующее фото — invalid_caption", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 1)
		resp := createPost(t, baseURL, author.token, map[string]any{
			"media_ids":      []string{photos[0].ID, unknownID},
			"media_captions": map[string]string{photos[0].ID: tooLong},
		})
		requireCodeE(t, resp, http.StatusBadRequest, "invalid_caption", "длинная подпись фото при плохом media_ids")
	})

	t.Run("длинная подпись под ключом не из media_ids — invalid_caption", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 1)
		resp := createPost(t, baseURL, author.token, pcDraft(photos, "",
			map[string]string{unknownID: tooLong}))
		requireCodeE(t, resp, http.StatusBadRequest, "invalid_caption", "длинная подпись под чужим ключом")
	})

	t.Run("годные подписи и несуществующее фото — invalid_media", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 1)
		resp := createPost(t, baseURL, author.token, map[string]any{
			"media_ids":      []string{photos[0].ID, unknownID},
			"media_captions": map[string]string{photos[0].ID: pcSortCaption},
		})
		requireCodeE(t, resp, http.StatusBadRequest, "invalid_media", "несуществующее фото")
	})
}

// --- Правка (требования 8–10) ----------------------------------------------

// Подписи перечисленных фото заменяются, пустая — снимает подпись,
// неперечисленные не меняются; edited_at появляется (требования 8, 10).
func TestPhotoCaptionsEditReplacesListedOnly(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)
	reader := newPCUser(t, baseURL, 2)

	photos := pcPhotos(t, baseURL, author.token, 3)
	post := pcPublish(t, baseURL, author.token, photos, postCaption,
		map[string]string{photos[0].ID: "Раз", photos[1].ID: "Два"})

	t.Run("пустая строка снимает подпись", func(t *testing.T) {
		edited := pcEdit(t, baseURL, author.token, post.ID, map[string]any{
			"caption":        postCaption,
			"media_captions": map[string]string{photos[0].ID: ""},
		}, "снятие подписи")
		want := []string{"", "Два", ""}
		pcRequireCaptions(t, edited, photos, want, "ответ на снятие")
		requireEditedAt(t, edited.EditedAt, edited.CreatedAt, "ответ на снятие")
		if edited.Caption != postCaption {
			t.Errorf("ответ на снятие: подпись поста %q, ожидалась %q", edited.Caption, postCaption)
		}

		pcRequireCaptions(t, pcGet(t, baseURL, reader.token, post.ID), photos, want, "GET читателем после снятия")
	})

	t.Run("новая подпись третьему фото, прочие прежние", func(t *testing.T) {
		edited := pcEdit(t, baseURL, author.token, post.ID, map[string]any{
			"caption":        postCaption,
			"media_captions": map[string]string{photos[2].ID: "  Три  "},
		}, "подпись третьему")
		want := []string{"", "Два", "Три"}
		pcRequireCaptions(t, edited, photos, want, "ответ на правку")

		pcRequireCaptions(t, pcRequireFound(t, pcFeedPage(t, baseURL, reader.token), post.ID, "лента"),
			photos, want, "лента после правки")
	})

	t.Run("общая подпись и подпись фото разом", func(t *testing.T) {
		edited := pcEdit(t, baseURL, author.token, post.ID, map[string]any{
			"caption":        fixedCaption,
			"media_captions": map[string]string{photos[1].ID: pcSortCaption, photos[0].ID: "Раз снова"},
		}, "правка всего")
		want := []string{"Раз снова", pcSortCaption, "Три"}
		pcRequireCaptions(t, edited, photos, want, "ответ на правку всего")
		if edited.Caption != fixedCaption {
			t.Errorf("ответ на правку всего: подпись поста %q, ожидалась %q", edited.Caption, fixedCaption)
		}
		pcRequireCaptions(t, pcGet(t, baseURL, author.token, post.ID), photos, want, "GET после правки всего")
	})

	t.Run("длина после обрезки: 1000 знаков можно", func(t *testing.T) {
		exactly := repeatRunes("ё", captionLimit)
		edited := pcEdit(t, baseURL, author.token, post.ID, map[string]any{
			"caption":        fixedCaption,
			"media_captions": map[string]string{photos[2].ID: " " + exactly + "\n"},
		}, "1000 знаков")
		pcRequireCaptions(t, edited, photos, []string{"Раз снова", pcSortCaption, exactly}, "ответ на 1000 знаков")
	})
}

// Правка без media_captions (так правит сборка без этой фичи), с null
// или с {} — подписи фото прежние (требование 8).
func TestPhotoCaptionsEditWithoutFieldKeepsThem(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)

	cases := []struct {
		name  string
		extra map[string]any
	}{
		{"поля нет", map[string]any{}},
		{"null", map[string]any{"media_captions": nil}},
		{"пустой объект", map[string]any{"media_captions": map[string]string{}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			photos := pcPhotos(t, baseURL, author.token, 2)
			post := pcPublish(t, baseURL, author.token, photos, postCaption,
				map[string]string{photos[1].ID: pcSortCaption})

			body := map[string]any{"caption": fixedCaption}
			for k, v := range c.extra {
				body[k] = v
			}

			edited := pcEdit(t, baseURL, author.token, post.ID, body, c.name)
			want := []string{"", pcSortCaption}
			pcRequireCaptions(t, edited, photos, want, c.name+": ответ на правку")
			if edited.Caption != fixedCaption {
				t.Errorf("%s: подпись поста %q, ожидалась %q", c.name, edited.Caption, fixedCaption)
			}
			requireEditedAt(t, edited.EditedAt, edited.CreatedAt, c.name+": общая подпись поменялась")
			pcRequireCaptions(t, pcGet(t, baseURL, author.token, post.ID), photos, want, c.name+": GET")
		})
	}
}

// edited_at ставится, если поменялся хоть один текст, и не трогается,
// если все тексты прежние (требование 10; 022, требование 4).
func TestPhotoCaptionsEditEditedAt(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)

	t.Run("поменялась только подпись фото", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 2)
		post := pcPublish(t, baseURL, author.token, photos, postCaption, nil)

		edited := pcEdit(t, baseURL, author.token, post.ID, map[string]any{
			"caption":        postCaption,
			"media_captions": map[string]string{photos[1].ID: pcSortCaption},
		}, "подпись фото")
		editedAt := requireEditedAt(t, edited.EditedAt, edited.CreatedAt, "ответ на правку подписи фото")

		got := pcGet(t, baseURL, author.token, post.ID)
		if got.EditedAt == nil || *got.EditedAt != editedAt {
			t.Errorf("GET: edited_at %v, ожидался %q", got.EditedAt, editedAt)
		}
	})

	t.Run("все тексты прежние, с учётом обрезки", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 2)
		post := pcPublish(t, baseURL, author.token, photos, postCaption,
			map[string]string{photos[0].ID: pcSortCaption})

		same := pcEdit(t, baseURL, author.token, post.ID, map[string]any{
			"caption":        " " + postCaption + " ",
			"media_captions": map[string]string{photos[0].ID: "  " + pcSortCaption + "\n"},
		}, "те же тексты")
		requireNotEdited(t, same.EditedAt, "ответ на правку теми же текстами")
		pcRequireCaptions(t, same, photos, []string{pcSortCaption, ""}, "ответ на правку теми же текстами")
		requireNotEdited(t, pcGet(t, baseURL, author.token, post.ID).EditedAt, "GET после правки теми же текстами")
	})

	t.Run("снятие подписи, которой не было", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 2)
		post := pcPublish(t, baseURL, author.token, photos, postCaption,
			map[string]string{photos[0].ID: pcSortCaption})

		same := pcEdit(t, baseURL, author.token, post.ID, map[string]any{
			"caption":        postCaption,
			"media_captions": map[string]string{photos[1].ID: "   "},
		}, "пустая подпись фото без подписи")
		requireNotEdited(t, same.EditedAt, "ответ: подписи не было и нет")
		pcRequireCaptions(t, same, photos, []string{pcSortCaption, ""}, "ответ: подписи не было и нет")
	})

	t.Run("повтор после правки не сдвигает edited_at", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 1)
		post := pcPublish(t, baseURL, author.token, photos, postCaption, nil)

		body := map[string]any{
			"caption":        postCaption,
			"media_captions": map[string]string{photos[0].ID: pcSortCaption},
		}
		first := pcEdit(t, baseURL, author.token, post.ID, body, "первая правка")
		editedAt := requireEditedAt(t, first.EditedAt, first.CreatedAt, "первая правка")

		again := pcEdit(t, baseURL, author.token, post.ID, body, "повтор")
		if again.EditedAt == nil || *again.EditedAt != editedAt {
			t.Errorf("повтор: edited_at %v, ожидался прежний %q", again.EditedAt, editedAt)
		}
	})
}

// Отказы правки: ключ — не фото этого поста, длинная подпись фото, порядок
// проверок; после отказа не меняется ничего, даже годная часть запроса
// (требование 9).
func TestPhotoCaptionsEditRejections(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)
	stranger := newPCUser(t, baseURL, 2)

	photos := pcPhotos(t, baseURL, author.token, 2)
	post := pcPublish(t, baseURL, author.token, photos, postCaption,
		map[string]string{photos[0].ID: pcSortCaption})
	want := []string{pcSortCaption, ""}

	otherPhotos := pcPhotos(t, baseURL, author.token, 1)
	otherPost := pcPublish(t, baseURL, author.token, otherPhotos, "", nil)
	unpublished := photoOf(t, baseURL, author.token, 60, 40)
	strangerPhoto := photoOf(t, baseURL, stranger.token, 60, 40)

	tooLong := repeatRunes("ы", captionLimit+1)

	cases := []struct {
		name   string
		token  string
		body   map[string]any
		status int
		code   string
	}{
		{
			name:  "ключ — фото другого поста",
			token: author.token,
			body: map[string]any{"caption": postCaption,
				"media_captions": map[string]string{otherPhotos[0].ID: "Чужая грядка"}},
			status: http.StatusBadRequest, code: "invalid_media",
		},
		{
			name:  "ключ — неопубликованное своё фото",
			token: author.token,
			body: map[string]any{"caption": postCaption,
				"media_captions": map[string]string{unpublished.ID: "Ещё не в посте"}},
			status: http.StatusBadRequest, code: "invalid_media",
		},
		{
			name:  "ключ — фото другого пользователя",
			token: author.token,
			body: map[string]any{"caption": postCaption,
				"media_captions": map[string]string{strangerPhoto.ID: "Не моё"}},
			status: http.StatusBadRequest, code: "invalid_media",
		},
		{
			name:  "ключ — фото, которого нет",
			token: author.token,
			body: map[string]any{"caption": postCaption,
				"media_captions": map[string]string{unknownID: "Нет такого"}},
			status: http.StatusBadRequest, code: "invalid_media",
		},
		{
			name:  "годная правка и плохой ключ — не меняется ничего",
			token: author.token,
			body: map[string]any{"caption": fixedCaption,
				"media_captions": map[string]string{photos[1].ID: "Годная", unknownID: "Нет такого"}},
			status: http.StatusBadRequest, code: "invalid_media",
		},
		{
			name:  "подпись фото в 1001 знак",
			token: author.token,
			body: map[string]any{"caption": postCaption,
				"media_captions": map[string]string{photos[1].ID: tooLong}},
			status: http.StatusBadRequest, code: "invalid_caption",
		},
		{
			name:  "длинная подпись поста и плохой ключ — invalid_caption",
			token: author.token,
			body: map[string]any{"caption": tooLong,
				"media_captions": map[string]string{unknownID: pcSortCaption}},
			status: http.StatusBadRequest, code: "invalid_caption",
		},
		{
			name:  "длинная подпись фото под плохим ключом — invalid_caption",
			token: author.token,
			body: map[string]any{"caption": postCaption,
				"media_captions": map[string]string{otherPhotos[0].ID: tooLong}},
			status: http.StatusBadRequest, code: "invalid_caption",
		},
		{
			name:  "чужой пост с длинной подписью фото — not_your_post",
			token: stranger.token,
			body: map[string]any{"caption": postCaption,
				"media_captions": map[string]string{photos[1].ID: tooLong}},
			status: http.StatusForbidden, code: "not_your_post",
		},
		{
			name:  "чужой пост с плохим ключом — not_your_post",
			token: stranger.token,
			body: map[string]any{"caption": postCaption,
				"media_captions": map[string]string{strangerPhoto.ID: "Моё"}},
			status: http.StatusForbidden, code: "not_your_post",
		},
		{
			name:  "чужой пост с годной подписью фото — not_your_post",
			token: stranger.token,
			body: map[string]any{"caption": postCaption,
				"media_captions": map[string]string{photos[1].ID: "Подпишу за автора"}},
			status: http.StatusForbidden, code: "not_your_post",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			requireCodeE(t, editCaptionReq(t, baseURL, c.token, post.ID, c.body), c.status, c.code, c.name)
			pcRequireStays(t, baseURL, author.token, post, photos, want, c.name)
		})
	}

	t.Run("поста нет — post_not_found раньше подписей", func(t *testing.T) {
		resp := editCaptionReq(t, baseURL, author.token, unknownID, map[string]any{
			"caption":        postCaption,
			"media_captions": map[string]string{photos[0].ID: tooLong},
		})
		requireCodeE(t, resp, http.StatusNotFound, "post_not_found", "несуществующий пост")
	})

	// Другой пост автора тоже не задет попыткой подписать его фото.
	pcRequireCaptions(t, pcGet(t, baseURL, author.token, otherPost.ID), otherPhotos, []string{""}, "другой пост автора")
}

// --- Тэги из подписей фото (требование 11) ---------------------------------

// Хэштег в подписи фото — тэг поста: в поле tags и в ленте по тэгу.
func TestPhotoCaptionsHashtagMakesPostTag(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)
	reader := newPCUser(t, baseURL, 2)

	photos := pcPhotos(t, baseURL, author.token, 2)
	post := pcPublish(t, baseURL, author.token, photos, "Урожай",
		map[string]string{photos[1].ID: "Это #груша, сорт Чудо"})

	pcRequireTags(t, post, []string{"груша"}, "ответ на создание")
	pcRequireTags(t, pcGet(t, baseURL, reader.token, post.ID), []string{"груша"}, "GET читателем")

	byTag := pcRequireFound(t, pcTagFeed(t, baseURL, reader.token, "груша"), post.ID, "лента ?tag=груша")
	pcRequireCaptions(t, byTag, photos, []string{"", "Это #груша, сорт Чудо"}, "лента ?tag=груша")
	pcRequireTags(t, byTag, []string{"груша"}, "лента ?tag=груша")

	if post.Caption != "Урожай" {
		t.Errorf("подпись поста %q: хэштеги фото в неё не переносятся", post.Caption)
	}
}

// Тэги собираются из подписи поста, затем из подписей фото по порядку
// фото — не по порядку ключей в объекте; повторы схлопываются, берутся
// первые 10 разных (требование 11; 028, требования 3–5).
func TestPhotoCaptionsTagsOrderDedupAndLimit(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)

	t.Run("подпись поста, потом фото по порядку", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 3)
		post := pcPublish(t, baseURL, author.token, photos, "Грядка #яблоня #Груша", map[string]string{
			photos[2].ID: "#альфа",
			photos[0].ID: "#вишня #груша",
			photos[1].ID: "#бета #ВИШНЯ",
		})
		pcRequireTags(t, post, []string{"яблоня", "груша", "вишня", "бета", "альфа"}, "ответ на создание")
	})

	t.Run("первые десять разных", func(t *testing.T) {
		photos := pcPhotos(t, baseURL, author.token, 2)
		post := pcPublish(t, baseURL, author.token, photos, "#т01 #т02 #т03 #т04 #т05 #т06", map[string]string{
			photos[0].ID: "#т03 #т07 #т08",
			photos[1].ID: "#т09 #т10 #т11 #т12",
		})
		pcRequireTags(t, post,
			[]string{"т01", "т02", "т03", "т04", "т05", "т06", "т07", "т08", "т09", "т10"}, "ответ на создание")
		pcRequireTags(t, pcGet(t, baseURL, author.token, post.ID),
			[]string{"т01", "т02", "т03", "т04", "т05", "т06", "т07", "т08", "т09", "т10"}, "GET")

		if pcFind(pcTagFeed(t, baseURL, author.token, "т11"), post.ID) != nil {
			t.Error("одиннадцатый тэг не входит в тэги поста, а пост нашёлся по нему")
		}
	})
}

// Правка пересчитывает тэги целиком — из всех подписей, в том числе
// тех, что правка не трогала (требование 11; 028, требование 6).
func TestPhotoCaptionsEditRecomputesTags(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)

	photos := pcPhotos(t, baseURL, author.token, 2)
	post := pcPublish(t, baseURL, author.token, photos, "Грядка #сорт", map[string]string{
		photos[0].ID: "#груша",
		photos[1].ID: "#чудо",
	})
	pcRequireTags(t, post, []string{"сорт", "груша", "чудо"}, "после создания")

	t.Run("правка только общей подписи оставляет тэги фото", func(t *testing.T) {
		edited := pcEdit(t, baseURL, author.token, post.ID, map[string]any{"caption": "Грядка #урожай"}, "общая подпись")
		pcRequireTags(t, edited, []string{"урожай", "груша", "чудо"}, "ответ на правку общей подписи")
		if pcFind(pcTagFeed(t, baseURL, author.token, "груша"), post.ID) == nil {
			t.Error("после правки общей подписи пост пропал из ленты по тэгу фото «груша»")
		}
		if pcFind(pcTagFeed(t, baseURL, author.token, "сорт"), post.ID) != nil {
			t.Error("тэг «сорт» убран из общей подписи, а пост по нему находится")
		}
	})

	t.Run("снятие подписи фото убирает её тэг", func(t *testing.T) {
		edited := pcEdit(t, baseURL, author.token, post.ID, map[string]any{
			"caption":        "Грядка #урожай",
			"media_captions": map[string]string{photos[0].ID: ""},
		}, "снятие подписи с хэштегом")
		pcRequireTags(t, edited, []string{"урожай", "чудо"}, "ответ на снятие")
		if pcFind(pcTagFeed(t, baseURL, author.token, "груша"), post.ID) != nil {
			t.Error("подпись с #груша снята, а пост по тэгу находится")
		}
	})

	t.Run("новый хэштег в подписи фото добавляет тэг", func(t *testing.T) {
		edited := pcEdit(t, baseURL, author.token, post.ID, map[string]any{
			"caption":        "Грядка #урожай",
			"media_captions": map[string]string{photos[0].ID: "Теперь #малина"},
		}, "новый хэштег")
		pcRequireTags(t, edited, []string{"урожай", "малина", "чудо"}, "ответ на новый хэштег")
		pcRequireTags(t, pcGet(t, baseURL, author.token, post.ID), []string{"урожай", "малина", "чудо"}, "GET")
		if pcFind(pcTagFeed(t, baseURL, author.token, "малина"), post.ID) == nil {
			t.Error("пост не нашёлся по новому тэгу из подписи фото")
		}
	})

	t.Run("отказ правки не трогает тэги", func(t *testing.T) {
		resp := editCaptionReq(t, baseURL, author.token, post.ID, map[string]any{
			"caption":        "Грядка",
			"media_captions": map[string]string{photos[1].ID: "#новое", unknownID: "#лишнее"},
		})
		requireCodeE(t, resp, http.StatusBadRequest, "invalid_media", "плохой ключ")
		pcRequireTags(t, pcGet(t, baseURL, author.token, post.ID), []string{"урожай", "малина", "чудо"}, "после отказа")
	})
}

// --- Видимость и удаление (требования 2, 4) --------------------------------

// Подпись фото видна тем, кому виден пост, и только им: пост «только я»
// посторонний не находит ни по адресу, ни по тэгу из подписи фото
// (требование 4).
func TestPhotoCaptionsFollowPostVisibility(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)
	stranger := newPCUser(t, baseURL, 2)

	photos := pcPhotos(t, baseURL, author.token, 1)
	body := pcDraft(photos, "Только себе", map[string]string{photos[0].ID: "Секрет #тайник"})
	body["visibility"] = visibilityMe
	post := pcPostOK(t, createPost(t, baseURL, author.token, body), http.StatusCreated, "пост «только я»")

	pcRequireCaptions(t, pcGet(t, baseURL, author.token, post.ID), photos, []string{"Секрет #тайник"}, "GET автором")

	requireCodeE(t, fetchPost(t, baseURL, stranger.token, post.ID), http.StatusNotFound, "post_not_found", "GET посторонним")
	if pcFind(pcTagFeed(t, baseURL, stranger.token, "тайник"), post.ID) != nil {
		t.Error("посторонний нашёл пост «только я» по тэгу из подписи фото")
	}
}

// Удаление поста уносит его фото вместе с подписями (требование 2).
func TestPhotoCaptionsGoAwayWithPost(t *testing.T) {
	baseURL := startAPI(t)
	author := newPCUser(t, baseURL, 1)

	photos := pcPhotos(t, baseURL, author.token, 2)
	post := pcPublish(t, baseURL, author.token, photos, "", map[string]string{
		photos[0].ID: pcSortCaption,
		photos[1].ID: "Та же грядка через месяц #урожай",
	})
	kept := pcPhotos(t, baseURL, author.token, 1)
	other := pcPublish(t, baseURL, author.token, kept, "", map[string]string{kept[0].ID: "Остаётся"})

	requireDeleted(t, deletePost(t, baseURL, author.token, post.ID))

	requireNoRows(t, "media", "post_id", post.ID, "фото удалённого поста")
	for _, p := range photos {
		requireNoRows(t, "media", "id", p.ID, "фото "+p.ID)
	}
	if pcFind(pcTagFeed(t, baseURL, author.token, "урожай"), post.ID) != nil {
		t.Error("удалённый пост находится по тэгу из подписи фото")
	}

	pcRequireCaptions(t, pcGet(t, baseURL, author.token, other.ID), kept, []string{"Остаётся"}, "другой пост после удаления")
}
