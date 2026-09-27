package tests

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"
)

// Имя и «о себе», которыми пользователь подписан везде, где его показывают
// (specs/002-profile.md, «Функциональные требования», п. 1-3).
const (
	profileName  = "Николай"
	profileAbout = "Три сотки под картошку"
)

// getProfile спрашивает свой профиль.
func getProfile(t *testing.T, baseURL, token string) *http.Response {
	t.Helper()
	return do(t, http.MethodGet, baseURL+"/me", token, nil)
}

// updateProfile меняет имя и «о себе». body передаётся как есть: тестам
// про непригодное тело нужен не объект, а что угодно.
func updateProfile(t *testing.T, baseURL, token string, body any) *http.Response {
	t.Helper()
	return do(t, http.MethodPut, baseURL+"/me", token, body)
}

// putAvatar загружает картинку в поле file, как это делает приложение.
func putAvatar(t *testing.T, baseURL, token, filename string, content []byte) *http.Response {
	t.Helper()
	return upload(t, http.MethodPut, baseURL+"/me/avatar", token, "file", filename, content)
}

// deleteAvatar убирает аватар.
func deleteAvatar(t *testing.T, baseURL, token string) *http.Response {
	t.Helper()
	return do(t, http.MethodDelete, baseURL+"/me/avatar", token, nil)
}

// decodeProfile разбирает ответ как профиль целиком: все четыре операции
// возвращают одно и то же представление.
func decodeProfile(t *testing.T, resp *http.Response) struct {
	ID        string  `json:"id"`
	Phone     string  `json:"phone"`
	CreatedAt string  `json:"created_at"`
	Name      string  `json:"name"`
	About     string  `json:"about"`
	AvatarURL *string `json:"avatar_url"`
} {
	t.Helper()

	var body struct {
		ID        string  `json:"id"`
		Phone     string  `json:"phone"`
		CreatedAt string  `json:"created_at"`
		Name      string  `json:"name"`
		About     string  `json:"about"`
		AvatarURL *string `json:"avatar_url"`
	}
	decode(t, resp, &body)

	return body
}

// profileOK требует успешного ответа и возвращает разобранный профиль.
func profileOK(t *testing.T, resp *http.Response) struct {
	ID        string  `json:"id"`
	Phone     string  `json:"phone"`
	CreatedAt string  `json:"created_at"`
	Name      string  `json:"name"`
	About     string  `json:"about"`
	AvatarURL *string `json:"avatar_url"`
} {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ожидался статус 200, получен %d", resp.StatusCode)
	}

	return decodeProfile(t, resp)
}

// avatarLink достаёт ссылку на аватар и требует, чтобы она была.
func avatarLink(t *testing.T, link *string) string {
	t.Helper()

	if link == nil || *link == "" {
		t.Fatal("в профиле нет ссылки на аватар, хотя аватар поставлен")
	}

	return *link
}

// requireNoAvatar требует, чтобы аватара в профиле не было: ссылки нет
// или она null (specs/002-profile.md, «API / контракт данных»).
func requireNoAvatar(t *testing.T, link *string) {
	t.Helper()

	if link != nil && *link != "" {
		t.Errorf("аватара быть не должно, а в профиле ссылка %q", *link)
	}
}

// imageBytes рисует картинку заданного размера и кодирует её в нужный
// формат: "png", "jpeg" или "gif".
func imageBytes(t *testing.T, format string, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Градиент, а не сплошная заливка: у неё после уменьшения
			// нечего было бы разглядывать.
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 160, A: 255})
		}
	}

	var buf bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&buf, img)
	case "jpeg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	case "gif":
		err = gif.Encode(&buf, img, nil)
	default:
		t.Fatalf("тест просит неизвестный формат картинки %q", format)
	}
	if err != nil {
		t.Fatalf("не удалось закодировать картинку в %s: %v", format, err)
	}

	return buf.Bytes()
}

// bombPNG собирает PNG, заголовок которого объявляет картинку width×height,
// а данных в нём — одна строка нулей: файл в сотни байт, который при
// распаковке занял бы гигабайты. Сервер должен отказать по заголовку,
// не распаковывая (specs/002-profile.md, требование 5).
func bombPNG(t *testing.T, width, height uint32) []byte {
	t.Helper()

	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(kind string, data []byte) {
		var head [8]byte
		binary.BigEndian.PutUint32(head[:4], uint32(len(data)))
		copy(head[4:], kind)
		out.Write(head[:])
		out.Write(data)
		crc := crc32.NewIEEE()
		crc.Write([]byte(kind))
		crc.Write(data)
		var sum [4]byte
		binary.BigEndian.PutUint32(sum[:], crc.Sum32())
		out.Write(sum[:])
	}

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], width)
	binary.BigEndian.PutUint32(ihdr[4:8], height)
	ihdr[8] = 8 // бит на канал
	ihdr[9] = 2 // RGB
	chunk("IHDR", ihdr)

	var idat bytes.Buffer
	zw := zlib.NewWriter(&idat)
	if _, err := zw.Write(make([]byte, 1+3*int(width))); err != nil {
		t.Fatalf("не удалось сжать строку картинки: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("не удалось сжать строку картинки: %v", err)
	}
	chunk("IDAT", idat.Bytes())
	chunk("IEND", nil)

	return out.Bytes()
}

// bombCases — картинки больше 8192×8192 точек по произведению сторон:
// квадратная и узкая длинная, у которой каждая сторона меньше 65536.
func bombCases(t *testing.T) map[string][]byte {
	return map[string][]byte{
		"30000×30000": bombPNG(t, 30000, 30000),
		"65000×1100":  bombPNG(t, 65000, 1100),
	}
}

// downloadFile скачивает файл по ссылке из профиля.
func downloadFile(t *testing.T, baseURL, link string) []byte {
	t.Helper()

	resp := get(t, fileURL(t, baseURL, link))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("файл по ссылке %q должен отдаваться, получен статус %d", link, resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("не удалось прочитать файл по ссылке %q: %v", link, err)
	}

	return content
}

// requireFileGone требует, чтобы по ссылке больше ничего не отдавалось.
func requireFileGone(t *testing.T, baseURL, link string) {
	t.Helper()

	resp := get(t, fileURL(t, baseURL, link))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("по прежней ссылке %q файла быть не должно, получен статус %d", link, resp.StatusCode)
	}
}

// imageConfig разбирает скачанный файл как картинку: размеры и формат.
func imageConfig(t *testing.T, content []byte) (image.Config, string) {
	t.Helper()

	cfg, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("сохранённый аватар не читается как картинка: %v", err)
	}

	return cfg, format
}

// avatarOf ставит аватар и возвращает ссылку на него: подготовка для тестов,
// которые проверяют что-то другое.
func avatarOf(t *testing.T, baseURL, token string, content []byte) string {
	t.Helper()

	resp := putAvatar(t, baseURL, token, "avatar.png", content)
	profile := profileOK(t, resp)

	return avatarLink(t, profile.AvatarURL)
}

// --- GET /api/me: мой профиль --------------------------------------------

// У только что зарегистрировавшегося пользователя имя и «о себе» пустые,
// аватара нет (ФТ-1, «API / контракт данных»). Экран знакомства теперь
// показывается по nickname_chosen — это проверяет nicknames_test.go.
func TestProfileOfNewUserIsEmptyAndReadyForIntroduction(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)

	resp := getProfile(t, baseURL, token)
	profile := profileOK(t, resp)

	if profile.ID != userID {
		t.Errorf("ожидался пользователь %q, получен %q", userID, profile.ID)
	}
	if !uuidPattern.MatchString(profile.ID) {
		t.Errorf("идентификатор %q не похож на UUID", profile.ID)
	}
	if profile.Name != "" {
		t.Errorf("у нового пользователя имя должно быть пустым, получено %q", profile.Name)
	}
	if profile.About != "" {
		t.Errorf("у нового пользователя «о себе» должно быть пустым, получено %q", profile.About)
	}
	requireNoAvatar(t, profile.AvatarURL)
}

// Профиль показывает имя, «о себе», аватар, номер телефона и дату
// регистрации (ФТ-1).
func TestProfileShowsNameAboutAvatarPhoneAndRegistrationDate(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)

	updated := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("имя и «о себе» должны сохраняться, получен статус %d", updated.StatusCode)
	}
	uploadedLink := avatarOf(t, baseURL, token, imageBytes(t, "png", 200, 200))

	resp := getProfile(t, baseURL, token)
	profile := profileOK(t, resp)

	if profile.ID != userID {
		t.Errorf("ожидался пользователь %q, получен %q", userID, profile.ID)
	}
	if profile.Phone != phoneStored {
		t.Errorf("ожидался нормализованный номер %q, получен %q", phoneStored, profile.Phone)
	}
	if _, err := time.Parse(time.RFC3339, profile.CreatedAt); err != nil {
		t.Errorf("дата регистрации %q не разбирается как время: %v", profile.CreatedAt, err)
	}
	if profile.Name != profileName {
		t.Errorf("ожидалось имя %q, получено %q", profileName, profile.Name)
	}
	if profile.About != profileAbout {
		t.Errorf("ожидалось «о себе» %q, получено %q", profileAbout, profile.About)
	}
	if link := avatarLink(t, profile.AvatarURL); link != uploadedLink {
		t.Errorf("ожидалась ссылка на аватар %q, получена %q", uploadedLink, link)
	}
}

// Профиль виден только своему владельцу: сосед по токену получает свой
// профиль, а не чужой (ФТ-12).
func TestProfileIsReturnedOnlyToTheOwnerOfTheToken(t *testing.T) {
	baseURL := startAPI(t)

	ownerToken, ownerID := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)

	if ownerID == otherID {
		t.Fatalf("два номера должны дать двух разных пользователей, получен один %q", ownerID)
	}

	updated := updateProfile(t, baseURL, ownerToken, map[string]any{"name": profileName, "about": profileAbout})
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("имя и «о себе» должны сохраняться, получен статус %d", updated.StatusCode)
	}
	avatarOf(t, baseURL, ownerToken, imageBytes(t, "png", 200, 200))

	resp := getProfile(t, baseURL, otherToken)
	profile := profileOK(t, resp)

	if profile.ID != otherID {
		t.Errorf("ожидался пользователь %q, получен %q", otherID, profile.ID)
	}
	if profile.Phone != otherPhoneStored {
		t.Errorf("ожидался свой номер %q, получен %q", otherPhoneStored, profile.Phone)
	}
	if profile.Name != "" {
		t.Errorf("сосед своего имени не задавал, а в его профиле %q", profile.Name)
	}
	if profile.About != "" {
		t.Errorf("сосед «о себе» не задавал, а в его профиле %q", profile.About)
	}
	requireNoAvatar(t, profile.AvatarURL)
}

// --- PUT /api/me: имя и «о себе» -----------------------------------------

// Имя и «о себе» сохраняются, в ответ приходит профиль целиком, и он же
// виден при следующем запросе (ФТ-2, ФТ-3).
func TestUpdateProfileSetsNameAndAbout(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)

	resp := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
	profile := profileOK(t, resp)

	if profile.ID != userID {
		t.Errorf("ожидался пользователь %q, получен %q", userID, profile.ID)
	}
	if profile.Phone != phoneStored {
		t.Errorf("ожидался номер %q, получен %q", phoneStored, profile.Phone)
	}
	if profile.Name != profileName {
		t.Errorf("ожидалось имя %q, получено %q", profileName, profile.Name)
	}
	if profile.About != profileAbout {
		t.Errorf("ожидалось «о себе» %q, получено %q", profileAbout, profile.About)
	}

	stored := profileOK(t, getProfile(t, baseURL, token))
	if stored.Name != profileName {
		t.Errorf("после сохранения ожидалось имя %q, получено %q", profileName, stored.Name)
	}
	if stored.About != profileAbout {
		t.Errorf("после сохранения ожидалось «о себе» %q, получено %q", profileAbout, stored.About)
	}
}

// Имя меняется: второе сохранение заменяет первое (ФТ-2).
func TestUpdateProfileReplacesPreviousNameAndAbout(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	first := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
	if first.StatusCode != http.StatusOK {
		t.Fatalf("первое сохранение должно было пройти, получен статус %d", first.StatusCode)
	}

	resp := updateProfile(t, baseURL, token, map[string]any{"name": "Пётр", "about": "Теплица и две яблони"})
	profile := profileOK(t, resp)

	if profile.Name != "Пётр" {
		t.Errorf("ожидалось новое имя %q, получено %q", "Пётр", profile.Name)
	}
	if profile.About != "Теплица и две яблони" {
		t.Errorf("ожидалось новое «о себе» %q, получено %q", "Теплица и две яблони", profile.About)
	}
}

// Пробелы по краям имени обрезаются, сохраняется обрезанное
// («Ограничения и edge cases»).
func TestUpdateProfileTrimsSpacesAroundName(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	resp := updateProfile(t, baseURL, token, map[string]any{"name": "  " + profileName + "  ", "about": ""})
	profile := profileOK(t, resp)

	if profile.Name != profileName {
		t.Errorf("ожидалось обрезанное имя %q, получено %q", profileName, profile.Name)
	}
}

// С 010-nicknames имя необязательно: пустое или из одних пробелов — 200,
// и прежнее имя очищается («Ограничения и edge cases»).
func TestUpdateProfileClearsNameWhenItIsEmptyOrBlank(t *testing.T) {
	names := map[string]string{
		"пустое":       "",
		"один пробел":  " ",
		"одни пробелы": "      ",
	}

	for caseName, name := range names {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)
			introduce(t, baseURL, token, profileName)

			resp := updateProfile(t, baseURL, token, map[string]any{"name": name, "about": profileAbout})
			profile := profileOK(t, resp)

			if profile.Name != "" {
				t.Errorf("на имя %q ожидалось пустое имя, получено %q", name, profile.Name)
			}
			if profile.About != profileAbout {
				t.Errorf("ожидалось «о себе» %q, получено %q", profileAbout, profile.About)
			}

			stored := profileOK(t, getProfile(t, baseURL, token))
			if stored.Name != "" {
				t.Errorf("после сохранения ожидалось пустое имя, получено %q", stored.Name)
			}
		})
	}
}

// Пятьдесят кириллических символов — это пятьдесят символов, а не сто
// байтов: имя сохраняется («Ограничения и edge cases»).
func TestUpdateProfileAcceptsNameOfFiftyCyrillicCharacters(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	name := strings.Repeat("я", 50)

	resp := updateProfile(t, baseURL, token, map[string]any{"name": name, "about": ""})
	profile := profileOK(t, resp)

	if profile.Name != name {
		t.Errorf("имя из 50 символов должно сохраняться целиком, получено %q", profile.Name)
	}
}

// Пятьдесят первый символ — уже слишком длинное имя: 400 invalid_name.
func TestUpdateProfileRejectsNameLongerThanFiftyCharacters(t *testing.T) {
	names := map[string]string{
		"51 символ кириллицы": strings.Repeat("я", 51),
		"51 символ латиницы":  strings.Repeat("a", 51),
		"много символов":      strings.Repeat("я", 500),
		"51 после обрезки":    "  " + strings.Repeat("я", 51) + "  ",
	}

	for caseName, name := range names {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := updateProfile(t, baseURL, token, map[string]any{"name": name, "about": ""})

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "invalid_name" {
				t.Fatalf("ожидалась ошибка invalid_name, получена %q", code)
			}
		})
	}
}

// Имя — одна строка: перевод строки, возврат каретки и табуляция
// не принимаются (ФТ-4).
func TestUpdateProfileRejectsNameThatIsNotASingleLine(t *testing.T) {
	names := map[string]string{
		"перевод строки":  "Николай\nВторой",
		"возврат каретки": "Николай\rВторой",
		"табуляция":       "Николай\tВторой",
		"нулевой байт":    "Николай\u0000",
		"перевод в конце": profileName + "\n",
	}

	for caseName, name := range names {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := updateProfile(t, baseURL, token, map[string]any{"name": name, "about": ""})

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("на имя %q ожидался статус 400, получен %d", name, resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "invalid_name" {
				t.Fatalf("ожидалась ошибка invalid_name, получена %q", code)
			}
		})
	}
}

// «О себе» можно очистить: это замена обоих полей сразу, поэтому пустое
// и вовсе отсутствующее «о себе» очищают поле («Ограничения и edge cases»).
func TestUpdateProfileClearsAboutWhenItIsEmptyOrMissing(t *testing.T) {
	bodies := map[string]map[string]any{
		"пустое «о себе»":      {"name": profileName, "about": ""},
		"«о себе» не передано": {"name": profileName},
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			filled := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
			if filled.StatusCode != http.StatusOK {
				t.Fatalf("сначала «о себе» должно было сохраниться, получен статус %d", filled.StatusCode)
			}

			resp := updateProfile(t, baseURL, token, body)
			profile := profileOK(t, resp)

			if profile.About != "" {
				t.Errorf("«о себе» должно было очиститься, получено %q", profile.About)
			}
			if profile.Name != profileName {
				t.Errorf("имя должно было сохраниться как %q, получено %q", profileName, profile.Name)
			}

			stored := profileOK(t, getProfile(t, baseURL, token))
			if stored.About != "" {
				t.Errorf("после сохранения «о себе» должно быть пустым, получено %q", stored.About)
			}
		})
	}
}

// Двести символов «о себе» — предел, который принимается (ФТ-3).
func TestUpdateProfileAcceptsAboutOfTwoHundredCharacters(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	about := strings.Repeat("я", 200)

	resp := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": about})
	profile := profileOK(t, resp)

	if profile.About != about {
		t.Errorf("«о себе» из 200 символов должно сохраняться целиком, получено %d символов", len([]rune(profile.About)))
	}
}

// Двести первый символ — 400 invalid_about.
func TestUpdateProfileRejectsAboutLongerThanTwoHundredCharacters(t *testing.T) {
	abouts := map[string]string{
		"201 символ кириллицы": strings.Repeat("я", 201),
		"201 символ латиницы":  strings.Repeat("a", 201),
		"много символов":       strings.Repeat("я", 1000),
	}

	for caseName, about := range abouts {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": about})

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "invalid_about" {
				t.Fatalf("ожидалась ошибка invalid_about, получена %q", code)
			}
		})
	}
}

// «О себе» — тоже одна строка (ФТ-4).
func TestUpdateProfileRejectsAboutThatIsNotASingleLine(t *testing.T) {
	abouts := map[string]string{
		"перевод строки":  "Три сотки\nпод картошку",
		"возврат каретки": "Три сотки\rпод картошку",
		"табуляция":       "Три сотки\tпод картошку",
		"нулевой байт":    "Три сотки\u0000",
	}

	for caseName, about := range abouts {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": about})

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("на «о себе» %q ожидался статус 400, получен %d", about, resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "invalid_about" {
				t.Fatalf("ожидалась ошибка invalid_about, получена %q", code)
			}
		})
	}
}

// Тело запроса, которое не разбирается в объект с полем name,
// — 400 invalid_request.
func TestUpdateProfileRejectsMalformedBody(t *testing.T) {
	bodies := map[string]any{
		"не объект запроса":  "это не объект запроса",
		"имя не строка":      map[string]any{"name": 42},
		"«о себе» не строка": map[string]any{"name": profileName, "about": []string{"а", "б"}},
	}

	for caseName, body := range bodies {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := updateProfile(t, baseURL, token, body)

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "invalid_request" {
				t.Fatalf("ожидалась ошибка invalid_request, получена %q", code)
			}
		})
	}
}

// PUT /api/me меняет только имя и «о себе»: аватар, номер и дата
// регистрации остаются на месте, в ответе — профиль целиком.
func TestUpdateProfileKeepsAvatarAndRegistrationData(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)

	link := avatarOf(t, baseURL, token, imageBytes(t, "png", 300, 300))
	before := profileOK(t, getProfile(t, baseURL, token))

	resp := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
	profile := profileOK(t, resp)

	if profile.ID != userID {
		t.Errorf("ожидался пользователь %q, получен %q", userID, profile.ID)
	}
	if profile.CreatedAt != before.CreatedAt {
		t.Errorf("дата регистрации не должна меняться: была %q, стала %q", before.CreatedAt, profile.CreatedAt)
	}
	if got := avatarLink(t, profile.AvatarURL); got != link {
		t.Errorf("аватар не должен меняться: была ссылка %q, стала %q", link, got)
	}
	downloadFile(t, baseURL, link)
}

// --- PUT /api/me/avatar: поставить аватар --------------------------------

// PNG принимается, в ответ приходит профиль целиком со ссылкой на аватар,
// файл отдаётся сервисом и пересохранён в JPEG (ФТ-5, ФТ-7, ФТ-10, ФТ-11).
func TestSetAvatarFromPngReturnsProfileWithWorkingLink(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)

	named := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
	if named.StatusCode != http.StatusOK {
		t.Fatalf("имя должно было сохраниться, получен статус %d", named.StatusCode)
	}

	resp := putAvatar(t, baseURL, token, "avatar.png", imageBytes(t, "png", 300, 200))
	profile := profileOK(t, resp)

	if profile.ID != userID {
		t.Errorf("ожидался пользователь %q, получен %q", userID, profile.ID)
	}
	if profile.Name != profileName {
		t.Errorf("загрузка аватара не должна менять имя: ожидалось %q, получено %q", profileName, profile.Name)
	}
	if profile.About != profileAbout {
		t.Errorf("загрузка аватара не должна менять «о себе»: ожидалось %q, получено %q", profileAbout, profile.About)
	}

	link := avatarLink(t, profile.AvatarURL)
	cfg, format := imageConfig(t, downloadFile(t, baseURL, link))

	if format != "jpeg" {
		t.Errorf("аватар пересохраняется в JPEG, а сохранён как %q", format)
	}
	if cfg.Width != 300 || cfg.Height != 200 {
		t.Errorf("картинка меньше 512 должна сохраняться как есть, получено %dx%d", cfg.Width, cfg.Height)
	}

	stored := profileOK(t, getProfile(t, baseURL, token))
	if got := avatarLink(t, stored.AvatarURL); got != link {
		t.Errorf("ожидалась та же ссылка %q, получена %q", link, got)
	}
}

// JPEG принимается так же (ФТ-5).
func TestSetAvatarFromJpegIsAccepted(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	resp := putAvatar(t, baseURL, token, "avatar.jpg", imageBytes(t, "jpeg", 400, 400))
	profile := profileOK(t, resp)

	link := avatarLink(t, profile.AvatarURL)
	_, format := imageConfig(t, downloadFile(t, baseURL, link))

	if format != "jpeg" {
		t.Errorf("аватар должен храниться как JPEG, а сохранён как %q", format)
	}
}

// Картинка больше 512 по стороне уменьшается до 512 с сохранением
// пропорций (ФТ-7).
func TestSetAvatarShrinksBigImageToFiveHundredTwelveKeepingProportions(t *testing.T) {
	cases := map[string]struct {
		width, height                 int
		expectedWidth, expectedHeight int
	}{
		"шире, чем выше": {width: 1200, height: 600, expectedWidth: 512, expectedHeight: 256},
		"выше, чем шире": {width: 600, height: 1200, expectedWidth: 256, expectedHeight: 512},
		"квадрат":        {width: 1024, height: 1024, expectedWidth: 512, expectedHeight: 512},
		"чуть больше":    {width: 513, height: 513, expectedWidth: 512, expectedHeight: 512},
	}

	for caseName, testCase := range cases {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := putAvatar(t, baseURL, token, "avatar.png", imageBytes(t, "png", testCase.width, testCase.height))
			profile := profileOK(t, resp)

			link := avatarLink(t, profile.AvatarURL)
			cfg, format := imageConfig(t, downloadFile(t, baseURL, link))

			if format != "jpeg" {
				t.Errorf("аватар должен храниться как JPEG, а сохранён как %q", format)
			}
			if cfg.Width > 512 || cfg.Height > 512 {
				t.Fatalf("аватар должен уменьшаться до 512, получено %dx%d", cfg.Width, cfg.Height)
			}
			// Допуск в один пиксель: округление при пересчёте стороны —
			// дело реализации, пропорции от этого не ломаются.
			if abs(cfg.Width-testCase.expectedWidth) > 1 || abs(cfg.Height-testCase.expectedHeight) > 1 {
				t.Errorf("из %dx%d ожидалось примерно %dx%d, получено %dx%d",
					testCase.width, testCase.height,
					testCase.expectedWidth, testCase.expectedHeight,
					cfg.Width, cfg.Height)
			}
		})
	}
}

// abs — модуль целого: нужен, чтобы сравнивать размеры с допуском.
func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// Картинка меньше 512×512 сохраняется как есть и не растягивается
// («Ограничения и edge cases»).
func TestSetAvatarKeepsImageSmallerThanFiveHundredTwelveAsItIs(t *testing.T) {
	cases := map[string]struct{ width, height int }{
		"совсем маленькая": {width: 64, height: 64},
		"не квадратная":    {width: 120, height: 80},
		"ровно 512":        {width: 512, height: 512},
	}

	for caseName, testCase := range cases {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := putAvatar(t, baseURL, token, "avatar.png", imageBytes(t, "png", testCase.width, testCase.height))
			profile := profileOK(t, resp)

			link := avatarLink(t, profile.AvatarURL)
			cfg, format := imageConfig(t, downloadFile(t, baseURL, link))

			if format != "jpeg" {
				t.Errorf("аватар должен храниться как JPEG, а сохранён как %q", format)
			}
			if cfg.Width != testCase.width || cfg.Height != testCase.height {
				t.Errorf("картинка %dx%d не должна меняться в размере, получено %dx%d",
					testCase.width, testCase.height, cfg.Width, cfg.Height)
			}
		})
	}
}

// Аватар пересохраняется, и из файла уходит всё, что не является
// картинкой: EXIF и любой другой довесок (ФТ-7).
func TestSetAvatarDropsEverythingThatIsNotThePictureItself(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	marker := []byte("EXIF-МЕТКА-КОТОРОЙ-В-АВАТАРЕ-БЫТЬ-НЕ-ДОЛЖНО")
	withMarker := append(imageBytes(t, "jpeg", 600, 600), marker...)

	resp := putAvatar(t, baseURL, token, "avatar.jpg", withMarker)
	profile := profileOK(t, resp)

	link := avatarLink(t, profile.AvatarURL)
	content := downloadFile(t, baseURL, link)

	if bytes.Contains(content, marker) {
		t.Error("в сохранённом аватаре осталось то, что не является картинкой")
	}
	if _, format := imageConfig(t, content); format != "jpeg" {
		t.Errorf("аватар должен храниться как JPEG, а сохранён как %q", format)
	}
}

// Новый аватар заменяет прежний: ссылка меняется, старый файл удаляется
// (ФТ-8, «Ограничения и edge cases»).
func TestSecondAvatarReplacesTheFirstOne(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	firstLink := avatarOf(t, baseURL, token, imageBytes(t, "png", 300, 300))
	downloadFile(t, baseURL, firstLink)

	resp := putAvatar(t, baseURL, token, "second.jpg", imageBytes(t, "jpeg", 400, 200))
	profile := profileOK(t, resp)

	secondLink := avatarLink(t, profile.AvatarURL)
	if secondLink == firstLink {
		t.Fatalf("ссылка на аватар должна была измениться, осталась %q", secondLink)
	}

	requireFileGone(t, baseURL, firstLink)

	cfg, format := imageConfig(t, downloadFile(t, baseURL, secondLink))
	if format != "jpeg" {
		t.Errorf("аватар должен храниться как JPEG, а сохранён как %q", format)
	}
	if cfg.Width != 400 || cfg.Height != 200 {
		t.Errorf("по новой ссылке ожидалась новая картинка 400x200, получено %dx%d", cfg.Width, cfg.Height)
	}

	stored := profileOK(t, getProfile(t, baseURL, token))
	if got := avatarLink(t, stored.AvatarURL); got != secondLink {
		t.Errorf("в профиле ожидалась новая ссылка %q, получена %q", secondLink, got)
	}
}

// Тип картинки определяется по содержимому, а не по имени файла: текст
// с именем photo.jpg — 400 invalid_image (ФТ-6).
func TestSetAvatarRejectsFileThatIsNotAnImage(t *testing.T) {
	files := map[string][]byte{
		"photo.jpg": []byte("это не картинка"),
		"photo.png": []byte("это не картинка"),
	}

	for filename, content := range files {
		t.Run(filename, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := putAvatar(t, baseURL, token, filename, content)

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "invalid_image" {
				t.Fatalf("ожидалась ошибка invalid_image, получена %q", code)
			}

			profile := profileOK(t, getProfile(t, baseURL, token))
			requireNoAvatar(t, profile.AvatarURL)
		})
	}
}

// В MVP принимаются только JPEG и PNG: настоящая GIF-картинка с любым
// именем — 400 invalid_image («Ограничения и edge cases»).
func TestSetAvatarRejectsGif(t *testing.T) {
	filenames := []string{"avatar.gif", "avatar.jpg"}

	for _, filename := range filenames {
		t.Run(filename, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := putAvatar(t, baseURL, token, filename, imageBytes(t, "gif", 200, 200))

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "invalid_image" {
				t.Fatalf("ожидалась ошибка invalid_image, получена %q", code)
			}
		})
	}
}

// Картинка больше 5 МБ — 413 image_too_large, причём размер проверяется
// раньше, чем содержимое: файл без заголовка картинки отвергается так же.
func TestSetAvatarRejectsImageLargerThanFiveMegabytes(t *testing.T) {
	const tooLarge = 5*1024*1024 + 1

	cases := map[string][]byte{
		"с заголовком PNG": append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, tooLarge)...),
		"без заголовка":    make([]byte, tooLarge),
	}

	for caseName, content := range cases {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := putAvatar(t, baseURL, token, "avatar.png", content)

			if resp.StatusCode != http.StatusRequestEntityTooLarge {
				t.Fatalf("ожидался статус 413, получен %d", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "image_too_large" {
				t.Fatalf("ожидалась ошибка image_too_large, получена %q", code)
			}

			profile := profileOK(t, getProfile(t, baseURL, token))
			requireNoAvatar(t, profile.AvatarURL)
		})
	}
}

// Маленький PNG, объявляющий картинку больше 8192×8192 точек, — 413
// image_too_large: размеры проверяются по заголовку до распаковки
// (требование 5, «Ограничения и edge cases»).
func TestSetAvatarRejectsImageDeclaringTooManyPixels(t *testing.T) {
	for caseName, content := range bombCases(t) {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := putAvatar(t, baseURL, token, "avatar.png", content)

			if resp.StatusCode != http.StatusRequestEntityTooLarge {
				t.Fatalf("на PNG в %d байт, объявляющий %s, ожидался статус 413, получен %d", len(content), caseName, resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "image_too_large" {
				t.Fatalf("ожидалась ошибка image_too_large, получена %q", code)
			}

			profile := profileOK(t, getProfile(t, baseURL, token))
			requireNoAvatar(t, profile.AvatarURL)
		})
	}
}

// Загрузка без поля file — 400 invalid_request
// («Ограничения и edge cases»).
func TestSetAvatarRejectsUploadWithoutFileField(t *testing.T) {
	fields := map[string]string{
		"поля нет вовсе":        "",
		"поле называется иначе": "avatar",
	}

	for caseName, field := range fields {
		t.Run(caseName, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			resp := upload(t, http.MethodPut, baseURL+"/me/avatar", token, field, "avatar.png", imageBytes(t, "png", 100, 100))

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("ожидался статус 400, получен %d", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "invalid_request" {
				t.Fatalf("ожидалась ошибка invalid_request, получена %q", code)
			}
		})
	}
}

// Имя файла случайное: ссылку на чужой аватар не угадать по профилю
// соседа, и две одинаковые картинки лежат по разным ссылкам
// («Ограничения и edge cases», ФТ-10).
func TestAvatarLinksAreRandomAndDifferForEveryUser(t *testing.T) {
	baseURL := startAPI(t)

	ownerToken, ownerID := signIn(t, baseURL, phonePretty)
	otherToken, otherID := signIn(t, baseURL, otherPhonePretty)

	sameImage := imageBytes(t, "png", 256, 256)

	ownerLink := avatarOf(t, baseURL, ownerToken, sameImage)
	otherLink := avatarOf(t, baseURL, otherToken, sameImage)

	if ownerLink == otherLink {
		t.Fatalf("у разных пользователей должны быть разные ссылки, обе — %q", ownerLink)
	}
	for _, guessable := range []string{ownerID, otherID, phoneStored, otherPhoneStored} {
		if strings.Contains(ownerLink, guessable) || strings.Contains(otherLink, guessable) {
			t.Errorf("ссылка на аватар не должна содержать %q: имя файла случайное", guessable)
		}
	}

	// Каждая ссылка ведёт на свой файл, и оба файла отдаются.
	downloadFile(t, baseURL, ownerLink)
	downloadFile(t, baseURL, otherLink)
}

// --- DELETE /api/me/avatar: убрать аватар --------------------------------

// Аватар убирается: ссылки в профиле больше нет, файл не отдаётся (ФТ-9).
func TestDeleteAvatarRemovesLinkAndFile(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	named := updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
	if named.StatusCode != http.StatusOK {
		t.Fatalf("имя должно было сохраниться, получен статус %d", named.StatusCode)
	}
	link := avatarOf(t, baseURL, token, imageBytes(t, "png", 300, 300))

	resp := deleteAvatar(t, baseURL, token)
	profile := profileOK(t, resp)

	requireNoAvatar(t, profile.AvatarURL)
	if profile.Name != profileName {
		t.Errorf("удаление аватара не должно менять имя: ожидалось %q, получено %q", profileName, profile.Name)
	}
	if profile.About != profileAbout {
		t.Errorf("удаление аватара не должно менять «о себе»: ожидалось %q, получено %q", profileAbout, profile.About)
	}

	requireFileGone(t, baseURL, link)

	stored := profileOK(t, getProfile(t, baseURL, token))
	requireNoAvatar(t, stored.AvatarURL)
}

// Убрать аватар, которого нет, — не ошибка: 200 и профиль без аватара
// (ФТ-9, «Ограничения и edge cases»).
func TestDeleteAvatarWithoutAvatarIsNotAnError(t *testing.T) {
	baseURL := startAPI(t)

	token, userID := signIn(t, baseURL, phonePretty)

	first := deleteAvatar(t, baseURL, token)
	profile := profileOK(t, first)

	if profile.ID != userID {
		t.Errorf("ожидался пользователь %q, получен %q", userID, profile.ID)
	}
	requireNoAvatar(t, profile.AvatarURL)

	// Второе удаление подряд — тоже не ошибка.
	second := deleteAvatar(t, baseURL, token)
	repeated := profileOK(t, second)
	requireNoAvatar(t, repeated.AvatarURL)
}

// После удаления аватар можно поставить снова.
func TestAvatarCanBeSetAgainAfterDeletion(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)

	firstLink := avatarOf(t, baseURL, token, imageBytes(t, "png", 300, 300))

	removed := deleteAvatar(t, baseURL, token)
	requireNoAvatar(t, profileOK(t, removed).AvatarURL)

	secondLink := avatarOf(t, baseURL, token, imageBytes(t, "png", 300, 300))
	if secondLink == firstLink {
		t.Errorf("новый аватар должен лежать по новой ссылке, получена прежняя %q", secondLink)
	}
	downloadFile(t, baseURL, secondLink)
}

// --- Доступ: профиль принадлежит владельцу токена ------------------------

// Любой из четырёх запросов без токена — 401 unauthorized (ФТ-12).
func TestProfileRequestsWithoutTokenAreUnauthorized(t *testing.T) {
	requests := map[string]func(t *testing.T, baseURL, token string) *http.Response{
		"GET /api/me": func(t *testing.T, baseURL, token string) *http.Response {
			return getProfile(t, baseURL, token)
		},
		"PUT /api/me": func(t *testing.T, baseURL, token string) *http.Response {
			return updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
		},
		"PUT /api/me/avatar": func(t *testing.T, baseURL, token string) *http.Response {
			return putAvatar(t, baseURL, token, "avatar.png", imageBytes(t, "png", 100, 100))
		},
		"DELETE /api/me/avatar": func(t *testing.T, baseURL, token string) *http.Response {
			return deleteAvatar(t, baseURL, token)
		},
	}

	for name, request := range requests {
		t.Run(name, func(t *testing.T) {
			baseURL := startAPI(t)

			resp := request(t, baseURL, "")

			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "unauthorized" {
				t.Fatalf("ожидалась ошибка unauthorized, получена %q", code)
			}
		})
	}
}

// Токен, которого сервис не выдавал, — тоже 401 unauthorized (ФТ-12).
func TestProfileRequestsWithUnknownTokenAreUnauthorized(t *testing.T) {
	requests := map[string]func(t *testing.T, baseURL, token string) *http.Response{
		"GET /api/me": func(t *testing.T, baseURL, token string) *http.Response {
			return getProfile(t, baseURL, token)
		},
		"PUT /api/me": func(t *testing.T, baseURL, token string) *http.Response {
			return updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
		},
		"PUT /api/me/avatar": func(t *testing.T, baseURL, token string) *http.Response {
			return putAvatar(t, baseURL, token, "avatar.png", imageBytes(t, "png", 100, 100))
		},
		"DELETE /api/me/avatar": func(t *testing.T, baseURL, token string) *http.Response {
			return deleteAvatar(t, baseURL, token)
		},
	}

	for name, request := range requests {
		t.Run(name, func(t *testing.T) {
			baseURL := startAPI(t)

			resp := request(t, baseURL, "токен-которого-не-выдавали")

			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "unauthorized" {
				t.Fatalf("ожидалась ошибка unauthorized, получена %q", code)
			}
		})
	}
}

// После выхода тот же токен к профилю уже не пускает (ФТ-12).
func TestProfileIsUnavailableAfterSignOut(t *testing.T) {
	requests := map[string]func(t *testing.T, baseURL, token string) *http.Response{
		"GET /api/me": func(t *testing.T, baseURL, token string) *http.Response {
			return getProfile(t, baseURL, token)
		},
		"PUT /api/me": func(t *testing.T, baseURL, token string) *http.Response {
			return updateProfile(t, baseURL, token, map[string]any{"name": profileName, "about": profileAbout})
		},
		"PUT /api/me/avatar": func(t *testing.T, baseURL, token string) *http.Response {
			return putAvatar(t, baseURL, token, "avatar.png", imageBytes(t, "png", 100, 100))
		},
		"DELETE /api/me/avatar": func(t *testing.T, baseURL, token string) *http.Response {
			return deleteAvatar(t, baseURL, token)
		},
	}

	for name, request := range requests {
		t.Run(name, func(t *testing.T) {
			baseURL := startAPI(t)

			token, _ := signIn(t, baseURL, phonePretty)

			if out := signOut(t, baseURL, token); out.StatusCode != http.StatusNoContent {
				t.Fatalf("выход должен был пройти, получен статус %d", out.StatusCode)
			}

			resp := request(t, baseURL, token)

			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("ожидался статус 401, получен %d", resp.StatusCode)
			}
			if code := errorCode(t, resp); code != "unauthorized" {
				t.Fatalf("ожидалась ошибка unauthorized, получена %q", code)
			}
		})
	}
}

// Имя и аватар соседа меняются независимо: каждый правит только свой
// профиль (ФТ-12).
func TestProfileChangesOfOneUserDoNotTouchAnother(t *testing.T) {
	baseURL := startAPI(t)

	ownerToken, _ := signIn(t, baseURL, phonePretty)
	otherToken, _ := signIn(t, baseURL, otherPhonePretty)

	ownerNamed := updateProfile(t, baseURL, ownerToken, map[string]any{"name": profileName, "about": profileAbout})
	if ownerNamed.StatusCode != http.StatusOK {
		t.Fatalf("имя владельца должно было сохраниться, получен статус %d", ownerNamed.StatusCode)
	}
	ownerLink := avatarOf(t, baseURL, ownerToken, imageBytes(t, "png", 300, 300))

	otherNamed := updateProfile(t, baseURL, otherToken, map[string]any{"name": "Пётр", "about": ""})
	if otherNamed.StatusCode != http.StatusOK {
		t.Fatalf("имя соседа должно было сохраниться, получен статус %d", otherNamed.StatusCode)
	}
	if removed := deleteAvatar(t, baseURL, otherToken); removed.StatusCode != http.StatusOK {
		t.Fatalf("удаление несуществующего аватара — не ошибка, получен статус %d", removed.StatusCode)
	}

	owner := profileOK(t, getProfile(t, baseURL, ownerToken))
	if owner.Name != profileName {
		t.Errorf("имя владельца должно было остаться %q, получено %q", profileName, owner.Name)
	}
	if owner.About != profileAbout {
		t.Errorf("«о себе» владельца должно было остаться %q, получено %q", profileAbout, owner.About)
	}
	if got := avatarLink(t, owner.AvatarURL); got != ownerLink {
		t.Errorf("аватар владельца должен был остаться %q, получен %q", ownerLink, got)
	}
	downloadFile(t, baseURL, ownerLink)

	other := profileOK(t, getProfile(t, baseURL, otherToken))
	if other.Name != "Пётр" {
		t.Errorf("имя соседа должно было остаться %q, получено %q", "Пётр", other.Name)
	}
	requireNoAvatar(t, other.AvatarURL)
}

// --- /media/: раздаются файлы, а не папки -----------------------------------

// mediaFolders — адреса папок хранилища, которые ФТ-13 называет прямо:
// со слешем на конце и без.
var mediaFolders = []string{
	"/media", "/media/",
	"/media/avatars", "/media/avatars/",
	"/media/posts", "/media/posts/",
}

// foldersOf добавляет к списку папок все папки, в которых лежат файлы по
// ссылкам: если хранилище раскладывает файлы глубже, чем /media/avatars/,
// промежуточные папки тоже не должны отдавать список.
func foldersOf(t *testing.T, baseURL string, links ...string) []string {
	t.Helper()

	folders := append([]string(nil), mediaFolders...)
	seen := map[string]bool{}
	for _, folder := range folders {
		seen[folder] = true
	}
	add := func(folder string) {
		if !seen[folder] {
			seen[folder] = true
			folders = append(folders, folder)
		}
	}
	for _, link := range links {
		parsed, err := url.Parse(fileURL(t, baseURL, link))
		if err != nil {
			t.Fatalf("ссылка %q не разбирается: %v", link, err)
		}
		for dir := path.Dir(parsed.Path); dir != "/" && dir != "."; dir = path.Dir(dir) {
			add(dir)
			add(dir + "/")
		}
	}

	return folders
}

// requireNoListing требует, чтобы по адресу папки отдавался 404 и в ответе
// не было имён сохранённых файлов. Клиент ходит по редиректам (например,
// /media/avatars → /media/avatars/), поэтому проверяется итоговый ответ.
func requireNoListing(t *testing.T, baseURL, folder string, links []string) {
	t.Helper()

	resp := get(t, fileURL(t, baseURL, folder))
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("не удалось прочитать ответ на %q: %v", folder, err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("на адрес папки %q ожидался статус 404, получен %d (итоговый адрес %s)",
			folder, resp.StatusCode, resp.Request.URL)
	}
	for _, link := range links {
		name := path.Base(link)
		stem := strings.TrimSuffix(name, path.Ext(name))
		if strings.Contains(string(body), stem) {
			t.Errorf("в ответе на адрес папки %q видно имя сохранённого файла %q", folder, name)
		}
	}
}

// Список файлов хранилища не отдаётся никому: на адреса папок со слешем
// и без — 404, имён файлов в ответе нет, а сам аватар по своей ссылке
// скачивается (ФТ-13, ФТ-10).
func TestMediaFoldersAreNotListedButAvatarIsServed(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	link := avatarOf(t, baseURL, token, imageBytes(t, "png", 200, 200))

	for _, folder := range foldersOf(t, baseURL, link) {
		t.Run(folder, func(t *testing.T) {
			requireNoListing(t, baseURL, folder, []string{link})
		})
	}

	downloadFile(t, baseURL, link)
}

// То же, когда в хранилище лежат и аватар, и фотография поста: папка
// с фотографиями не выдаёт их списком — иначе закрытые видимостью фото
// ушли бы к кому угодно (ФТ-13; specs/013-post-visibility.md).
func TestMediaFoldersDoNotListAvatarsOrPostPhotos(t *testing.T) {
	baseURL := startAPI(t)

	token, _ := signIn(t, baseURL, phonePretty)
	avatar := avatarOf(t, baseURL, token, imageBytes(t, "png", 200, 200))
	photo := photoOf(t, baseURL, token, 400, 300)
	createdPost(t, createPostOf(t, baseURL, token, photo.ID))

	links := []string{avatar, photo.URL}
	for _, folder := range foldersOf(t, baseURL, links...) {
		t.Run(folder, func(t *testing.T) {
			requireNoListing(t, baseURL, folder, links)
		})
	}

	downloadFile(t, baseURL, avatar)
	downloadFile(t, baseURL, photo.URL)
}
