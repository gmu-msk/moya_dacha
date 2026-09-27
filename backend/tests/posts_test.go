package tests

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
)

// Подпись, с которой публикуется пост в большинстве проверок
// (specs/003-posts.md, «API / контракт данных»).
const postCaption = "Первая клубника в этом году"

// Идентификатор, которого сервис никогда не выдавал: UUID правильного вида,
// но такой фотографии (и такого поста) нет.
const unknownID = "2b7e1a90-5c3d-4f21-9a8b-6c5d4e3f2a10"

// --- Представления из контракта -------------------------------------------

// mediaPayload — медиа, как его отдаёт сервис (schema Media).
type mediaPayload struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// authorPayload — публичное представление пользователя (schema Author).
// Номера телефона в нём нет и быть не может (CONTEXT.md).
type authorPayload struct {
	ID        string  `json:"id"`
	Nickname  string  `json:"nickname"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

// postPayload — пост целиком (schema Post).
type postPayload struct {
	ID        string         `json:"id"`
	CreatedAt string         `json:"created_at"`
	Caption   string         `json:"caption"`
	Author    authorPayload  `json:"author"`
	Media     []mediaPayload `json:"media"`
}

// --- Хелперы --------------------------------------------------------------

// uploadPhoto загружает фотографию в поле file, как это делает приложение:
// каждая фотография идёт своим запросом (ФТ-2).
func uploadPhoto(t *testing.T, baseURL, token, filename string, content []byte) *http.Response {
	t.Helper()
	return upload(t, http.MethodPost, baseURL+"/media", token, "file", filename, content)
}

// createPost публикует пост. body передаётся как есть: тестам про
// непригодное тело нужен не объект, а что угодно.
func createPost(t *testing.T, baseURL, token string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, baseURL+"/posts", token, body)
}

// createPostOf публикует пост из перечисленных фотографий без подписи.
func createPostOf(t *testing.T, baseURL, token string, mediaIDs ...string) *http.Response {
	t.Helper()
	return createPost(t, baseURL, token, map[string]any{"media_ids": mediaIDs})
}

// fetchPost открывает пост по его адресу.
func fetchPost(t *testing.T, baseURL, token, postID string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, baseURL+"/posts/"+postID, token, nil)
}

// photoOK требует, чтобы фотография загрузилась, и возвращает её.
func photoOK(t *testing.T, resp *http.Response) mediaPayload {
	t.Helper()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("на загрузку фотографии ожидался статус 201, получен %d", resp.StatusCode)
	}

	var body mediaPayload
	decode(t, resp, &body)

	if !uuidPattern.MatchString(body.ID) {
		t.Errorf("идентификатор фотографии %q не похож на UUID", body.ID)
	}
	if body.Kind != "photo" {
		t.Errorf("в MVP медиа бывает только photo, получено %q", body.Kind)
	}
	if body.URL == "" {
		t.Error("у загруженной фотографии пустая ссылка")
	}

	return body
}

// photoOf загружает нарисованную PNG-картинку заданного размера и
// возвращает загруженную фотографию: подготовка для тестов про пост.
func photoOf(t *testing.T, baseURL, token string, width, height int) mediaPayload {
	t.Helper()
	return photoOK(t, uploadPhoto(t, baseURL, token, "photo.png", imageBytes(t, "png", width, height)))
}

// postOK требует ожидаемого статуса и возвращает разобранный пост.
func postOK(t *testing.T, resp *http.Response, wantStatus int) postPayload {
	t.Helper()

	if resp.StatusCode != wantStatus {
		t.Fatalf("ожидался статус %d, получен %d", wantStatus, resp.StatusCode)
	}

	var body postPayload
	decode(t, resp, &body)

	return body
}

// createdPost требует, чтобы пост был опубликован, и возвращает его.
func createdPost(t *testing.T, resp *http.Response) postPayload {
	t.Helper()
	return postOK(t, resp, http.StatusCreated)
}

// fetchedPost требует, чтобы пост открылся, и возвращает его.
func fetchedPost(t *testing.T, resp *http.Response) postPayload {
	t.Helper()
	return postOK(t, resp, http.StatusOK)
}

// requireError требует ошибку с ожидаемым статусом и машиночитаемым кодом.
func requireError(t *testing.T, resp *http.Response, wantStatus int, wantCode string) {
	t.Helper()

	if resp.StatusCode != wantStatus {
		t.Fatalf("ожидался статус %d, получен %d", wantStatus, resp.StatusCode)
	}
	if code := errorCode(t, resp); code != wantCode {
		t.Fatalf("ожидалась ошибка %s, получена %q", wantCode, code)
	}
}

// rawJSON читает тело ответа целиком, не разбирая: тесту про номер телефона
// мало разобранной структуры — важно, чего нет в самом ответе.
func rawJSON(t *testing.T, resp *http.Response) []byte {
	t.Helper()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("не удалось прочитать тело ответа: %v", err)
	}

	return raw
}

// photoConfig разбирает скачанный файл как картинку: размеры и формат.
func photoConfig(t *testing.T, content []byte) (image.Config, string) {
	t.Helper()

	cfg, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("сохранённая фотография не читается как картинка: %v", err)
	}

	return cfg, format
}

// requireStoredPhoto скачивает фотографию по ссылке из ответа и требует,
// чтобы она была JPEG нужного размера: сервис пересохраняет всё в JPEG
// и сообщает размеры сохранённого файла (ФТ-6, ФТ-8).
func requireStoredPhoto(t *testing.T, baseURL string, photo mediaPayload) {
	t.Helper()

	cfg, format := photoConfig(t, downloadFile(t, baseURL, photo.URL))

	if format != "jpeg" {
		t.Errorf("фотография пересохраняется в JPEG, а сохранена как %q", format)
	}
	if cfg.Width != photo.Width || cfg.Height != photo.Height {
		t.Errorf("в ответе размеры %dx%d, а в файле %dx%d",
			photo.Width, photo.Height, cfg.Width, cfg.Height)
	}
}

// requireMediaOrder требует, чтобы медиа поста шли ровно в том порядке,
// в котором автор перечислил их в media_ids (ФТ-3).
func requireMediaOrder(t *testing.T, post postPayload, want []mediaPayload) {
	t.Helper()

	if len(post.Media) != len(want) {
		t.Fatalf("в посте ожидалось %d фотографий, получено %d", len(want), len(post.Media))
	}
	for i, expected := range want {
		got := post.Media[i]
		if got.ID != expected.ID {
			t.Errorf("на месте %d ожидалась фотография %q, получена %q", i+1, expected.ID, got.ID)
		}
		if got.Width != expected.Width || got.Height != expected.Height {
			t.Errorf("на месте %d ожидалась картинка %dx%d, получена %dx%d",
				i+1, expected.Width, expected.Height, got.Width, got.Height)
		}
		if got.Kind != "photo" {
			t.Errorf("на месте %d ожидалось медиа вида photo, получено %q", i+1, got.Kind)
		}
	}
}

// introduce задаёт пользователю имя: автор в посте должен быть с именем
// (specs/002-profile.md, ФТ-2).
func introduce(t *testing.T, baseURL, token, name string) {
	t.Helper()

	resp := updateProfile(t, baseURL, token, map[string]any{"name": name, "about": ""})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("имя должно было сохраниться, получен статус %d", resp.StatusCode)
	}
}

// jpegWithExifOrientation дорисовывает к JPEG секцию EXIF с ориентацией:
// так телефон помечает снятое вертикально (ФТ-7). Ориентация 6 означает
// «повернуть на 90° по часовой при показе».
func jpegWithExifOrientation(t *testing.T, width, height int, orientation uint16) []byte {
	t.Helper()

	raw := imageBytes(t, "jpeg", width, height)
	if len(raw) < 2 || raw[0] != 0xFF || raw[1] != 0xD8 {
		t.Fatal("нарисованный JPEG не начинается с маркера SOI, тест собран неправильно")
	}

	// Минимальный TIFF: порядок байтов от старшего, один каталог,
	// одна запись — тег Orientation (0x0112) типа SHORT.
	tiff := []byte{
		'M', 'M', 0x00, 0x2A,
		0x00, 0x00, 0x00, 0x08,
		0x00, 0x01,
		0x01, 0x12,
		0x00, 0x03,
		0x00, 0x00, 0x00, 0x01,
		byte(orientation >> 8), byte(orientation), 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)

	length := len(payload) + 2
	segment := []byte{0xFF, 0xE1, byte(length >> 8), byte(length)}
	segment = append(segment, payload...)

	var out []byte
	out = append(out, raw[:2]...)
	out = append(out, segment...)
	out = append(out, raw[2:]...)

	return out
}

// webpBytes собирает файл-контейнер WebP: в MVP принимаются только JPEG
// и PNG, и тип определяется по содержимому («Ограничения и edge cases»).
func webpBytes() []byte {
	chunk := []byte{0x2F, 0x00, 0x00, 0x00, 0x10, 0x07, 0x10, 0x11, 0x11, 0x88, 0x88, 0xFE, 0x07, 0x00}

	out := []byte("RIFF")
	size := uint32(4 + 8 + len(chunk))
	out = append(out, byte(size), byte(size>>8), byte(size>>16), byte(size>>24))
	out = append(out, "WEBP"...)
	out = append(out, "VP8L"...)
	chunkSize := uint32(len(chunk))
	out = append(out, byte(chunkSize), byte(chunkSize>>8), byte(chunkSize>>16), byte(chunkSize>>24))
	out = append(out, chunk...)

	return out
}

// --- POST /api/media: загрузить фотографию --------------------------------

// JPEG принимается: в ответе — медиа с идентификатором, видом, ссылкой
// и размерами, и по ссылке сервис действительно отдаёт файл
// (ФТ-2, ФТ-6, ФТ-8, ФТ-10).
func TestUploadPhotoAcceptsJpegAndReturnsWorkingLink(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)

	resp := uploadPhoto(t, baseURL, token, "photo.jpg", imageBytes(t, "jpeg", 800, 600))
	photo := photoOK(t, resp)

	if photo.Width != 800 || photo.Height != 600 {
		t.Errorf("картинка меньше 1600 должна сохраняться как есть, получено %dx%d", photo.Width, photo.Height)
	}
	if strings.Contains(photo.URL, userID) {
		t.Errorf("ссылка на фотографию не должна содержать идентификатор автора: %q", photo.URL)
	}

	content := downloadFile(t, baseURL, photo.URL)
	cfg, format := photoConfig(t, content)

	if format != "jpeg" {
		t.Errorf("фотография должна храниться как JPEG, а сохранена как %q", format)
	}
	if cfg.Width != 800 || cfg.Height != 600 {
		t.Errorf("по ссылке ожидалась картинка 800x600, получено %dx%d", cfg.Width, cfg.Height)
	}
}

// PNG принимается и пересохраняется в JPEG («Ограничения и edge cases»).
func TestUploadPhotoAcceptsPngAndStoresItAsJpeg(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	photo := photoOK(t, uploadPhoto(t, baseURL, token, "photo.png", imageBytes(t, "png", 640, 480)))

	if photo.Width != 640 || photo.Height != 480 {
		t.Errorf("ожидались размеры 640x480, получено %dx%d", photo.Width, photo.Height)
	}
	requireStoredPhoto(t, baseURL, photo)
}

// Фотография больше 1600 по большей стороне уменьшается до 1600,
// пропорции сохраняются (ФТ-6).
func TestUploadPhotoShrinksBigPhotoToSixteenHundredKeepingProportions(t *testing.T) {
	cases := map[string]struct {
		width, height                 int
		expectedWidth, expectedHeight int
	}{
		"шире, чем выше": {width: 3200, height: 1600, expectedWidth: 1600, expectedHeight: 800},
		"выше, чем шире": {width: 1200, height: 2400, expectedWidth: 800, expectedHeight: 1600},
		"квадрат":        {width: 2000, height: 2000, expectedWidth: 1600, expectedHeight: 1600},
		"чуть больше":    {width: 1601, height: 1601, expectedWidth: 1600, expectedHeight: 1600},
	}

	for caseName, testCase := range cases {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			photo := photoOK(t, uploadPhoto(t, baseURL, token,
				"photo.jpg", imageBytes(t, "jpeg", testCase.width, testCase.height)))

			if photo.Width > 1600 || photo.Height > 1600 {
				t.Fatalf("фотография должна уменьшаться до 1600, получено %dx%d", photo.Width, photo.Height)
			}
			// Допуск в один пиксель: округление при пересчёте стороны —
			// дело реализации, пропорции от этого не ломаются.
			if abs(photo.Width-testCase.expectedWidth) > 1 || abs(photo.Height-testCase.expectedHeight) > 1 {
				t.Errorf("из %dx%d ожидалось примерно %dx%d, получено %dx%d",
					testCase.width, testCase.height,
					testCase.expectedWidth, testCase.expectedHeight,
					photo.Width, photo.Height)
			}

			// Размеры в ответе — это размеры сохранённого файла: лента
			// занимает по ним место до загрузки картинки (ФТ-8).
			requireStoredPhoto(t, baseURL, photo)
		})
	}
}

// Фотография меньше 1600 по большей стороне не увеличивается
// («Ограничения и edge cases»).
func TestUploadPhotoKeepsPhotoSmallerThanSixteenHundredAsItIs(t *testing.T) {
	cases := map[string]struct{ width, height int }{
		"совсем маленькая": {width: 100, height: 50},
		"не квадратная":    {width: 1200, height: 900},
		"ровно 1600":       {width: 1600, height: 1200},
	}

	for caseName, testCase := range cases {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			photo := photoOK(t, uploadPhoto(t, baseURL, token,
				"photo.png", imageBytes(t, "png", testCase.width, testCase.height)))

			if photo.Width != testCase.width || photo.Height != testCase.height {
				t.Errorf("картинка %dx%d не должна меняться в размере, получено %dx%d",
					testCase.width, testCase.height, photo.Width, photo.Height)
			}
			requireStoredPhoto(t, baseURL, photo)
		})
	}
}

// Снятое вертикально не ложится набок: фотография разворачивается по
// ориентации из EXIF, и уже развёрнутые размеры попадают в ответ (ФТ-7).
func TestUploadPhotoTurnsPhotoShotVerticallyByItsExifOrientation(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	// В файле пиксели лежат горизонтально, а EXIF говорит «повернуть
	// на 90°»: показывать такую фотографию нужно вертикально.
	content := jpegWithExifOrientation(t, 800, 400, 6)

	photo := photoOK(t, uploadPhoto(t, baseURL, token, "photo.jpg", content))

	if photo.Width != 400 || photo.Height != 800 {
		t.Errorf("снятая вертикально фотография должна отдаваться как 400x800, получено %dx%d",
			photo.Width, photo.Height)
	}
	requireStoredPhoto(t, baseURL, photo)
}

// Тип определяется по содержимому, а не по имени файла: текст с именем
// photo.jpg — 400 invalid_image (ФТ-5).
func TestUploadPhotoRejectsFileThatIsNotAnImage(t *testing.T) {
	files := map[string][]byte{
		"photo.jpg": []byte("это не картинка, а текстовый файл"),
		"photo.png": []byte("это не картинка, а текстовый файл"),
	}

	for filename, content := range files {
		t.Run(filename, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := uploadPhoto(t, baseURL, token, filename, content)

			requireError(t, resp, http.StatusBadRequest, "invalid_image")
		})
	}
}

// В MVP принимаются только JPEG и PNG: GIF и WebP с любым именем —
// 400 invalid_image («Ограничения и edge cases»).
func TestUploadPhotoRejectsGifAndWebp(t *testing.T) {
	cases := map[string]struct {
		filename string
		content  func(t *testing.T) []byte
	}{
		"gif с честным именем": {
			filename: "photo.gif",
			content:  func(t *testing.T) []byte { return imageBytes(t, "gif", 200, 200) },
		},
		"gif под видом jpeg": {
			filename: "photo.jpg",
			content:  func(t *testing.T) []byte { return imageBytes(t, "gif", 200, 200) },
		},
		"webp под видом jpeg": {
			filename: "photo.jpg",
			content:  func(t *testing.T) []byte { return webpBytes() },
		},
	}

	for caseName, testCase := range cases {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := uploadPhoto(t, baseURL, token, testCase.filename, testCase.content(t))

			requireError(t, resp, http.StatusBadRequest, "invalid_image")
		})
	}
}

// Фотография больше 10 МБ — 413 image_too_large, причём размер
// проверяется раньше содержимого («Ограничения и edge cases»).
func TestUploadPhotoRejectsPhotoLargerThanTenMegabytes(t *testing.T) {
	const tooLarge = 10*1024*1024 + 1

	cases := map[string][]byte{
		"с заголовком PNG": append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, tooLarge)...),
		"без заголовка":    make([]byte, tooLarge),
	}

	for caseName, content := range cases {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := uploadPhoto(t, baseURL, token, "photo.png", content)

			requireError(t, resp, http.StatusRequestEntityTooLarge, "image_too_large")
		})
	}
}

// Маленький PNG, объявляющий больше 8192×8192 точек, — 413 image_too_large:
// размеры проверяются по заголовку до распаковки (требование 5,
// «Ограничения и edge cases»).
func TestUploadPhotoRejectsImageDeclaringTooManyPixels(t *testing.T) {
	for caseName, content := range bombCases(t) {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := uploadPhoto(t, baseURL, token, "photo.png", content)

			requireError(t, resp, http.StatusRequestEntityTooLarge, "image_too_large")
		})
	}
}

// Загрузка без поля file — 400 invalid_request («Ограничения и edge cases»).
func TestUploadPhotoRejectsUploadWithoutFileField(t *testing.T) {
	fields := map[string]string{
		"поля нет вовсе":        "",
		"поле называется иначе": "photo",
	}

	for caseName, field := range fields {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := upload(t, http.MethodPost, baseURL+"/media", token, field,
				"photo.png", imageBytes(t, "png", 100, 100))

			requireError(t, resp, http.StatusBadRequest, "invalid_request")
		})
	}
}

// Имя файла случайное: две одинаковые картинки лежат по разным ссылкам,
// и обе отдаются (ФТ-10, «Модель данных»).
func TestUploadedPhotosLieUnderDifferentRandomLinks(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	sameImage := imageBytes(t, "png", 256, 256)

	first := photoOK(t, uploadPhoto(t, baseURL, token, "photo.png", sameImage))
	second := photoOK(t, uploadPhoto(t, baseURL, token, "photo.png", sameImage))

	if first.ID == second.ID {
		t.Fatalf("две загрузки должны дать две разные фотографии, обе — %q", first.ID)
	}
	if first.URL == second.URL {
		t.Fatalf("две загрузки должны лежать по разным ссылкам, обе — %q", first.URL)
	}

	downloadFile(t, baseURL, first.URL)
	downloadFile(t, baseURL, second.URL)
}

// --- POST /api/posts: опубликовать пост -----------------------------------

// Одна фотография без подписи — пост создаётся: в ответе он целиком,
// с автором, временем и своей единственной фотографией (ФТ-1).
func TestCreatePostWithOnePhotoAndWithoutCaption(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	photo := photoOf(t, baseURL, token, 800, 600)

	post := createdPost(t, createPostOf(t, baseURL, token, photo.ID))

	if !uuidPattern.MatchString(post.ID) {
		t.Errorf("идентификатор поста %q не похож на UUID", post.ID)
	}
	if _, err := time.Parse(time.RFC3339, post.CreatedAt); err != nil {
		t.Errorf("время публикации %q не разбирается как время: %v", post.CreatedAt, err)
	}
	if post.Caption != "" {
		t.Errorf("подписи не было, а в посте %q", post.Caption)
	}
	if post.Author.ID != userID {
		t.Errorf("автором ожидался %q, получен %q", userID, post.Author.ID)
	}
	if post.Author.Name != profileName {
		t.Errorf("ожидалось имя автора %q, получено %q", profileName, post.Author.Name)
	}

	requireMediaOrder(t, post, []mediaPayload{photo})
	requireStoredPhoto(t, baseURL, post.Media[0])
}

// Четыре фотографии — пост создаётся, и порядок в посте совпадает
// с порядком media_ids, а не с порядком загрузки (ФТ-1, ФТ-3).
func TestCreatePostWithFourPhotosKeepsTheOrderOfMediaIds(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)

	// Размеры у всех разные: по ним видно, что переставились именно
	// фотографии, а не только их идентификаторы.
	uploaded := []mediaPayload{
		photoOf(t, baseURL, token, 320, 240),
		photoOf(t, baseURL, token, 240, 320),
		photoOf(t, baseURL, token, 400, 100),
		photoOf(t, baseURL, token, 100, 400),
	}

	// Автор перечисляет их в обратном порядке загрузки.
	want := []mediaPayload{uploaded[3], uploaded[1], uploaded[0], uploaded[2]}
	ids := []string{want[0].ID, want[1].ID, want[2].ID, want[3].ID}

	post := createdPost(t, createPost(t, baseURL, token,
		map[string]any{"media_ids": ids, "caption": postCaption}))

	requireMediaOrder(t, post, want)
	for _, photo := range post.Media {
		requireStoredPhoto(t, baseURL, photo)
	}

	// Тот же порядок виден и при следующем открытии поста.
	stored := fetchedPost(t, fetchPost(t, baseURL, token, post.ID))
	requireMediaOrder(t, stored, want)
}

// Одиннадцать фотографий — 400 too_many_media («Ограничения и edge cases»).
func TestCreatePostRejectsMoreThanTenPhotos(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	var ids []string
	for i := 0; i < 11; i++ {
		ids = append(ids, photoOf(t, baseURL, token, 200, 200).ID)
	}

	resp := createPost(t, baseURL, token, map[string]any{"media_ids": ids, "caption": postCaption})

	requireError(t, resp, http.StatusBadRequest, "too_many_media")

	// Пост не создан, и ни одна фотография не прикрепилась: первые
	// десять по-прежнему можно опубликовать.
	retry := createdPost(t, createPostOf(t, baseURL, token, ids[:10]...))
	if len(retry.Media) != 10 {
		t.Errorf("после отказа десять фотографий должны публиковаться, в посте их %d", len(retry.Media))
	}
}

// Поста без фотографий не существует: пустой media_ids и «только подпись»
// — 400 no_media (ФТ-1, ADR-0006).
func TestCreatePostRejectsPostWithoutPhotos(t *testing.T) {
	bodies := map[string]any{
		"пустой media_ids":     map[string]any{"media_ids": []string{}},
		"пустой с подписью":    map[string]any{"media_ids": []string{}, "caption": postCaption},
		"только подпись":       map[string]any{"caption": postCaption},
		"media_ids равен null": map[string]any{"media_ids": nil, "caption": postCaption},
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := createPost(t, baseURL, token, body)

			requireError(t, resp, http.StatusBadRequest, "no_media")
		})
	}
}

// Один и тот же идентификатор дважды — 400 invalid_media: одна
// фотография — одно место в посте (ФТ-11, «Ограничения и edge cases»).
func TestCreatePostRejectsTheSamePhotoTwice(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	photo := photoOf(t, baseURL, token, 300, 300)

	resp := createPostOf(t, baseURL, token, photo.ID, photo.ID)

	requireError(t, resp, http.StatusBadRequest, "invalid_media")

	// Пост не создан, фотография осталась неприкреплённой.
	createdPost(t, createPostOf(t, baseURL, token, photo.ID))
}

// Чужая фотография в media_ids — 400 invalid_media: фотография
// принадлежит тому, кто её загрузил (ФТ-11).
func TestCreatePostRejectsPhotoOfAnotherUser(t *testing.T) {
	baseURL := startAPI(t)

	ownerToken, _ := signIn(t, baseURL, phonePretty)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)

	photo := photoOf(t, baseURL, ownerToken, 300, 200)

	resp := createPostOf(t, baseURL, otherToken, photo.ID)

	requireError(t, resp, http.StatusBadRequest, "invalid_media")

	// Сосед не смог ни присвоить фотографию, ни испортить её владельцу.
	post := createdPost(t, createPostOf(t, baseURL, ownerToken, photo.ID))
	requireMediaOrder(t, post, []mediaPayload{photo})
}

// Фотография, уже опубликованная в другом посте, — 400 invalid_media:
// прикрепление одноразовое (ФТ-11).
func TestCreatePostRejectsPhotoAlreadyPublished(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	photo := photoOf(t, baseURL, token, 300, 200)
	first := createdPost(t, createPostOf(t, baseURL, token, photo.ID))

	resp := createPostOf(t, baseURL, token, photo.ID)

	requireError(t, resp, http.StatusBadRequest, "invalid_media")

	// Первый пост от этой попытки не пострадал.
	stored := fetchedPost(t, fetchPost(t, baseURL, token, first.ID))
	requireMediaOrder(t, stored, []mediaPayload{photo})
}

// Несуществующий идентификатор фотографии — 400 invalid_media
// («Ограничения и edge cases»).
func TestCreatePostRejectsUnknownMediaID(t *testing.T) {
	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownID,
		"вовсе не UUID":                 "не-идентификатор",
		"пустая строка":                 "",
	}

	for caseName, id := range ids {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := createPostOf(t, baseURL, token, id)

			requireError(t, resp, http.StatusBadRequest, "invalid_media")
		})
	}
}

// Хотя бы один плохой идентификатор — и пост не создаётся целиком:
// хорошие фотографии остаются неприкреплёнными («Ограничения и edge cases»).
func TestCreatePostWithOneBadMediaIDCreatesNothing(t *testing.T) {
	baseURL := startAPI(t)

	ownerToken, _ := signIn(t, baseURL, phonePretty)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)

	first := photoOf(t, baseURL, ownerToken, 320, 240)
	second := photoOf(t, baseURL, ownerToken, 240, 320)
	foreign := photoOf(t, baseURL, otherToken, 100, 100)

	bad := map[string]string{
		"несуществующая фотография": unknownID,
		"чужая фотография":          foreign.ID,
	}

	for caseName, badID := range bad {
		t.Run(caseName, func(t *testing.T) {
			resp := createPost(t, baseURL, ownerToken, map[string]any{
				"media_ids": []string{first.ID, badID, second.ID},
				"caption":   postCaption,
			})

			requireError(t, resp, http.StatusBadRequest, "invalid_media")
		})
	}

	// Ни одной из хороших фотографий отказ не повредил: обе всё ещё
	// свободны и публикуются вместе одним постом.
	post := createdPost(t, createPostOf(t, baseURL, ownerToken, first.ID, second.ID))
	requireMediaOrder(t, post, []mediaPayload{first, second})

	// И чужая фотография тоже осталась у своего владельца.
	createdPost(t, createPostOf(t, baseURL, otherToken, foreign.ID))
}

// Тело запроса, которое не разбирается, — 400 invalid_request.
func TestCreatePostRejectsMalformedBody(t *testing.T) {
	bodies := map[string]any{
		"не объект запроса":         "это не объект запроса",
		"media_ids не массив":       map[string]any{"media_ids": 42},
		"media_ids не из строк":     map[string]any{"media_ids": []any{42}},
		"подпись не строка":         map[string]any{"media_ids": []string{unknownID}, "caption": []string{"а", "б"}},
		"media_ids вложен в объект": map[string]any{"media_ids": map[string]any{"id": unknownID}},
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := createPost(t, baseURL, token, body)

			requireError(t, resp, http.StatusBadRequest, "invalid_request")
		})
	}
}

// --- Подпись --------------------------------------------------------------

// Подпись необязательна: пустая и вовсе не переданная дают пустую
// подпись, а не ошибку (ФТ-4).
func TestCreatePostAcceptsEmptyCaption(t *testing.T) {
	captions := map[string]func(mediaID string) map[string]any{
		"пустая подпись": func(mediaID string) map[string]any {
			return map[string]any{"media_ids": []string{mediaID}, "caption": ""}
		},
		"подписи нет вовсе": func(mediaID string) map[string]any {
			return map[string]any{"media_ids": []string{mediaID}}
		},
	}

	for caseName, body := range captions {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			photo := photoOf(t, baseURL, token, 200, 200)

			post := createdPost(t, createPost(t, baseURL, token, body(photo.ID)))

			if post.Caption != "" {
				t.Errorf("подпись должна быть пустой, получена %q", post.Caption)
			}

			stored := fetchedPost(t, fetchPost(t, baseURL, token, post.ID))
			if stored.Caption != "" {
				t.Errorf("после публикации подпись должна остаться пустой, получена %q", stored.Caption)
			}
		})
	}
}

// Пробелы по краям подписи обрезаются: подпись из одних пробелов
// сохраняется пустой («Ограничения и edge cases»).
func TestCreatePostTrimsSpacesAroundCaption(t *testing.T) {
	cases := map[string]struct{ caption, expected string }{
		"одни пробелы":      {caption: "     ", expected: ""},
		"пробел и перевод":  {caption: " \n\t ", expected: ""},
		"пробелы по краям":  {caption: "   " + postCaption + "   ", expected: postCaption},
		"переводы по краям": {caption: "\n" + postCaption + "\n", expected: postCaption},
	}

	for caseName, testCase := range cases {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			photo := photoOf(t, baseURL, token, 200, 200)

			post := createdPost(t, createPost(t, baseURL, token,
				map[string]any{"media_ids": []string{photo.ID}, "caption": testCase.caption}))

			if post.Caption != testCase.expected {
				t.Errorf("из подписи %q ожидалось %q, получено %q", testCase.caption, testCase.expected, post.Caption)
			}

			stored := fetchedPost(t, fetchPost(t, baseURL, token, post.ID))
			if stored.Caption != testCase.expected {
				t.Errorf("после публикации ожидалась подпись %q, получена %q", testCase.expected, stored.Caption)
			}
		})
	}
}

// Тысяча кириллических символов — это тысяча символов, а не две тысячи
// байтов: подпись сохраняется («Ограничения и edge cases»).
func TestCreatePostAcceptsCaptionOfExactlyOneThousandCharacters(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	photo := photoOf(t, baseURL, token, 200, 200)
	caption := strings.Repeat("я", 1000)

	post := createdPost(t, createPost(t, baseURL, token,
		map[string]any{"media_ids": []string{photo.ID}, "caption": caption}))

	if post.Caption != caption {
		t.Errorf("подпись из 1000 символов должна сохраняться целиком, получено %d символов",
			len([]rune(post.Caption)))
	}

	stored := fetchedPost(t, fetchPost(t, baseURL, token, post.ID))
	if stored.Caption != caption {
		t.Errorf("после публикации ожидалось 1000 символов, получено %d", len([]rune(stored.Caption)))
	}
}

// Тысяча первый символ — 400 invalid_caption («Ограничения и edge cases»).
func TestCreatePostRejectsCaptionLongerThanOneThousandCharacters(t *testing.T) {
	captions := map[string]string{
		"1001 символ кириллицы": strings.Repeat("я", 1001),
		"1001 символ латиницы":  strings.Repeat("a", 1001),
		"очень длинная подпись": strings.Repeat("я", 5000),
	}

	for caseName, caption := range captions {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			photo := photoOf(t, baseURL, token, 200, 200)

			resp := createPost(t, baseURL, token,
				map[string]any{"media_ids": []string{photo.ID}, "caption": caption})

			requireError(t, resp, http.StatusBadRequest, "invalid_caption")

			// Пост не создан: фотография осталась свободной.
			createdPost(t, createPostOf(t, baseURL, token, photo.ID))
		})
	}
}

// Переводы строк внутри подписи сохраняются как есть: подпись — это
// текст, а не имя (ФТ-4, «Ограничения и edge cases»).
func TestCreatePostKeepsLineBreaksInCaption(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	photo := photoOf(t, baseURL, token, 200, 200)
	caption := "Первая клубника в этом году.\n\nСорт «Зенга»,\nвторой год под плёнкой."

	post := createdPost(t, createPost(t, baseURL, token,
		map[string]any{"media_ids": []string{photo.ID}, "caption": caption}))

	if post.Caption != caption {
		t.Errorf("подпись с переводами строк должна сохраняться как есть, получено %q", post.Caption)
	}

	stored := fetchedPost(t, fetchPost(t, baseURL, token, post.ID))
	if stored.Caption != caption {
		t.Errorf("после публикации ожидалась подпись %q, получена %q", caption, stored.Caption)
	}
}

// --- GET /api/posts/{id}: показать пост -----------------------------------

// Пост открывается по своему адресу и отдаётся ровно таким, каким был
// создан: контракт обещает у публикации тот же пост, что у чтения (ФТ-13).
func TestGetPostReturnsWhatWasCreated(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	avatarOf(t, baseURL, token, imageBytes(t, "png", 300, 300))

	first := photoOf(t, baseURL, token, 800, 600)
	second := photoOf(t, baseURL, token, 600, 800)

	created := createdPost(t, createPost(t, baseURL, token,
		map[string]any{"media_ids": []string{first.ID, second.ID}, "caption": postCaption}))

	stored := fetchedPost(t, fetchPost(t, baseURL, token, created.ID))

	if !reflect.DeepEqual(created, stored) {
		t.Errorf("пост при публикации и при чтении должен быть один и тот же:\nпри публикации %+v\nпри чтении    %+v",
			created, stored)
	}
	if stored.Caption != postCaption {
		t.Errorf("ожидалась подпись %q, получена %q", postCaption, stored.Caption)
	}
	requireMediaOrder(t, stored, []mediaPayload{first, second})
	for _, photo := range stored.Media {
		requireStoredPhoto(t, baseURL, photo)
	}
}

// Пост другого пользователя открывается: лента одна на всех
// («Ограничения и edge cases», ФТ-14).
func TestGetPostOfAnotherUserIsOpenToEveryone(t *testing.T) {
	baseURL := startAPI(t)

	ownerToken, ownerID := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)

	introduce(t, baseURL, ownerToken, profileName)
	introduce(t, baseURL, otherToken, "Пётр")

	photo := photoOf(t, baseURL, ownerToken, 400, 300)
	created := createdPost(t, createPost(t, baseURL, ownerToken,
		map[string]any{"media_ids": []string{photo.ID}, "caption": postCaption}))

	stored := fetchedPost(t, fetchPost(t, baseURL, otherToken, created.ID))

	if stored.ID != created.ID {
		t.Errorf("ожидался пост %q, получен %q", created.ID, stored.ID)
	}
	// Чужим именем пост не подписывается: автор остаётся автором (ФТ-14).
	if stored.Author.ID != ownerID {
		t.Errorf("автором ожидался %q, получен %q", ownerID, stored.Author.ID)
	}
	if stored.Author.ID == otherID {
		t.Error("пост подписан именем того, кто его открыл, а не автора")
	}
	if stored.Author.Name != profileName {
		t.Errorf("ожидалось имя автора %q, получено %q", profileName, stored.Author.Name)
	}
	requireMediaOrder(t, stored, []mediaPayload{photo})

	// И фотографии чужого поста тоже отдаются: пост видно целиком.
	requireStoredPhoto(t, baseURL, stored.Media[0])
}

// Несуществующий идентификатор поста — 404 post_not_found
// («Ограничения и edge cases»).
func TestGetPostWithUnknownIDIsNotFound(t *testing.T) {
	ids := map[string]string{
		"такого UUID сервис не выдавал": unknownID,
		"вовсе не UUID":                 "не-идентификатор",
	}

	for caseName, id := range ids {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := fetchPost(t, baseURL, token, id)

			requireError(t, resp, http.StatusNotFound, "post_not_found")
		})
	}
}

// Рядом с постом стоит публичное представление автора — имя и аватар,
// и никогда номер телефона. Проверяется по сырому ответу: важно не то,
// что разобралось, а то, чего в ответе нет (ФТ-13, CONTEXT.md).
func TestPostResponseShowsAuthorWithNameAndAvatarAndNeverThePhoneNumber(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)
	introduce(t, baseURL, token, profileName)
	avatarLinkOfAuthor := avatarOf(t, baseURL, token, imageBytes(t, "png", 300, 300))

	photo := photoOf(t, baseURL, token, 400, 300)

	created := createPost(t, baseURL, token,
		map[string]any{"media_ids": []string{photo.ID}, "caption": postCaption})
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("на публикацию ожидался статус 201, получен %d", created.StatusCode)
	}
	createdRaw := rawJSON(t, created)

	var post postPayload
	if err := json.Unmarshal(createdRaw, &post); err != nil {
		t.Fatalf("ответ на публикацию не разобрался как JSON: %v", err)
	}

	fetched := fetchPost(t, baseURL, token, post.ID)
	if fetched.StatusCode != http.StatusOK {
		t.Fatalf("на чтение поста ожидался статус 200, получен %d", fetched.StatusCode)
	}
	fetchedRaw := rawJSON(t, fetched)

	responses := map[string][]byte{
		"ответ на публикацию": createdRaw,
		"ответ на чтение":     fetchedRaw,
	}

	for name, raw := range responses {
		body := string(raw)

		// Ни номера в любом привычном виде, ни самого поля.
		for _, forbidden := range []string{phoneStored, phonePretty, phoneSpaced, phoneDigits, `"phone"`} {
			if strings.Contains(body, forbidden) {
				t.Errorf("в посте не должно быть номера телефона, а %s содержит %q", name, forbidden)
			}
		}

		// Автор при этом на месте: идентификатор, имя и аватар.
		var shape struct {
			Author map[string]any `json:"author"`
		}
		if err := json.Unmarshal(raw, &shape); err != nil {
			t.Fatalf("%s не разобрался как JSON: %v", name, err)
		}
		if shape.Author == nil {
			t.Fatalf("в посте нет автора: %s", name)
		}
		if _, ok := shape.Author["phone"]; ok {
			t.Errorf("у автора не может быть номера телефона, а в %s он есть", name)
		}
		if got, _ := shape.Author["id"].(string); got != userID {
			t.Errorf("в %s ожидался автор %q, получен %v", name, userID, shape.Author["id"])
		}
		if got, _ := shape.Author["name"].(string); got != profileName {
			t.Errorf("в %s ожидалось имя автора %q, получено %v", name, profileName, shape.Author["name"])
		}
		if got, _ := shape.Author["avatar_url"].(string); got != avatarLinkOfAuthor {
			t.Errorf("в %s ожидался аватар автора %q, получен %v", name, avatarLinkOfAuthor, shape.Author["avatar_url"])
		}
	}

	// Аватар автора действительно отдаётся сервисом.
	downloadFile(t, baseURL, avatarLinkOfAuthor)
	if post.Author.AvatarURL == nil || *post.Author.AvatarURL != avatarLinkOfAuthor {
		t.Errorf("ожидалась ссылка на аватар %q, получена %v", avatarLinkOfAuthor, post.Author.AvatarURL)
	}
}

// --- Доступ ---------------------------------------------------------------

// Любой из трёх запросов без токена — 401 unauthorized (ФТ-14).
func TestPostRequestsWithoutTokenAreUnauthorized(t *testing.T) {
	requests := map[string]func(t *testing.T, baseURL, token string) *http.Response{
		"POST /api/media": func(t *testing.T, baseURL, token string) *http.Response {
			return uploadPhoto(t, baseURL, token, "photo.png", imageBytes(t, "png", 200, 200))
		},
		"POST /api/posts": func(t *testing.T, baseURL, token string) *http.Response {
			return createPost(t, baseURL, token, map[string]any{"media_ids": []string{unknownID}})
		},
		"GET /api/posts/{id}": func(t *testing.T, baseURL, token string) *http.Response {
			return fetchPost(t, baseURL, token, unknownID)
		},
	}

	for name, request := range requests {
		t.Run(name, func(t *testing.T) {
			baseURL := startAPI(t)

			resp := request(t, baseURL, "")

			requireError(t, resp, http.StatusUnauthorized, "unauthorized")
		})
	}
}

// Токен, которого сервис не выдавал, — тоже 401 unauthorized (ФТ-14).
func TestPostRequestsWithUnknownTokenAreUnauthorized(t *testing.T) {
	requests := map[string]func(t *testing.T, baseURL, token string) *http.Response{
		"POST /api/media": func(t *testing.T, baseURL, token string) *http.Response {
			return uploadPhoto(t, baseURL, token, "photo.png", imageBytes(t, "png", 200, 200))
		},
		"POST /api/posts": func(t *testing.T, baseURL, token string) *http.Response {
			return createPost(t, baseURL, token, map[string]any{"media_ids": []string{unknownID}})
		},
		"GET /api/posts/{id}": func(t *testing.T, baseURL, token string) *http.Response {
			return fetchPost(t, baseURL, token, unknownID)
		},
	}

	for name, request := range requests {
		t.Run(name, func(t *testing.T) {
			baseURL := startAPI(t)

			resp := request(t, baseURL, "токен-которого-не-выдавали")

			requireError(t, resp, http.StatusUnauthorized, "unauthorized")
		})
	}
}

// Пост нельзя опубликовать от чужого имени: автором становится владелец
// токена, а не тот, кого назвали в теле запроса (ФТ-14).
func TestCreatePostIgnoresAuthorNamedInTheRequest(t *testing.T) {
	baseURL := startAPI(t)

	ownerToken, ownerID := signIn(t, baseURL, phonePretty)
	_, otherID := signIn(t, baseURL, otherPhonePretty)

	introduce(t, baseURL, ownerToken, profileName)
	photo := photoOf(t, baseURL, ownerToken, 300, 300)

	post := createdPost(t, createPost(t, baseURL, ownerToken, map[string]any{
		"media_ids": []string{photo.ID},
		"caption":   postCaption,
		"author_id": otherID,
		"author":    map[string]any{"id": otherID, "name": "Пётр"},
	}))

	if post.Author.ID != ownerID {
		t.Errorf("автором должен быть владелец токена %q, получен %q", ownerID, post.Author.ID)
	}
	if post.Author.Name != profileName {
		t.Errorf("ожидалось имя автора %q, получено %q", profileName, post.Author.Name)
	}
}

// --- Уборка неопубликованных фотографий -----------------------------------

// Отдельного расписания в проекте нет (ADR-0004): неприкреплённые
// фотографии старше часа убираются при следующей публикации их автора
// (ФТ-12). Час — это настройка сервиса, и тест задаёт её короткой, как
// auth-тесты задают CodeTTL и ResendAfter.
const unpublishedMediaTTL = 700 * time.Millisecond

// Неопубликованная фотография старше срока убирается при следующей
// публикации этого автора: файла больше нет, и прикрепить её нельзя
// («Ограничения и edge cases»).
func TestPublishingRemovesUnpublishedPhotosOlderThanAnHour(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{UnpublishedMediaTTL: unpublishedMediaTTL})

	token, _ := signIn(t, baseURL, phonePretty)

	stale := photoOf(t, baseURL, token, 320, 240)
	downloadFile(t, baseURL, stale.URL)

	time.Sleep(2 * unpublishedMediaTTL)

	// Фотография этого поста загружена только что и уборкой не
	// затрагивается: автор публикует её прямо сейчас.
	fresh := photoOf(t, baseURL, token, 240, 320)
	post := createdPost(t, createPostOf(t, baseURL, token, fresh.ID))

	requireFileGone(t, baseURL, stale.URL)

	resp := createPostOf(t, baseURL, token, stale.ID)
	requireError(t, resp, http.StatusBadRequest, "invalid_media")

	// Опубликованное уборка не трогает: пост остался целым.
	stored := fetchedPost(t, fetchPost(t, baseURL, token, post.ID))
	requireMediaOrder(t, stored, []mediaPayload{fresh})
	requireStoredPhoto(t, baseURL, stored.Media[0])
}

// Неопубликованная фотография моложе часа остаётся: автор ещё собирает
// пост («Ограничения и edge cases»).
func TestUnpublishedPhotoYoungerThanAnHourSurvivesPublishing(t *testing.T) {
	// Настройки по умолчанию: срок — час, и за время теста он не выйдет.
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	waiting := photoOf(t, baseURL, token, 320, 240)
	published := photoOf(t, baseURL, token, 240, 320)

	createdPost(t, createPostOf(t, baseURL, token, published.ID))

	// Фотография на месте — и файл отдаётся, и опубликовать её можно.
	requireStoredPhoto(t, baseURL, waiting)

	post := createdPost(t, createPostOf(t, baseURL, token, waiting.ID))
	requireMediaOrder(t, post, []mediaPayload{waiting})
}

// Уборка трогает только фотографии того, кто публикует: сосед в это
// время собирает свой пост (ФТ-12).
func TestPublishingRemovesOnlyUnpublishedPhotosOfTheSameAuthor(t *testing.T) {
	baseURL := startAPIWith(t, api.Config{UnpublishedMediaTTL: unpublishedMediaTTL})

	ownerToken, _ := signIn(t, baseURL, phonePretty)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)

	ownerStale := photoOf(t, baseURL, ownerToken, 320, 240)
	otherStale := photoOf(t, baseURL, otherToken, 240, 320)

	time.Sleep(2 * unpublishedMediaTTL)

	fresh := photoOf(t, baseURL, ownerToken, 200, 200)
	createdPost(t, createPostOf(t, baseURL, ownerToken, fresh.ID))

	requireFileGone(t, baseURL, ownerStale.URL)

	// Чужая неопубликованная фотография осталась на месте: её судьбу
	// решает публикация её собственного автора.
	requireStoredPhoto(t, baseURL, otherStale)
}
