// Профиль пользователя: specs/002-profile.md.
package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
	"github.com/gmu-msk/moya_dacha/backend/internal/media"
	"github.com/gmu-msk/moya_dacha/backend/internal/profile"
)

// userColumns — поля, из которых собирается профиль. Порядок совпадает
// с scanUser: читать их из базы нужно везде одинаково.
const userColumns = `id, phone, created_at, nickname, nickname_chosen, name, about, avatar_key, closed`

// userColumnsPrefixed — те же поля, когда в запросе несколько таблиц
// и users названа u.
const userColumnsPrefixed = `u.id, u.phone, u.created_at, u.nickname, u.nickname_chosen, u.name, u.about, u.avatar_key, u.closed`

// GetMe отдаёт профиль владельца токена.
func (s *Server) GetMe(ctx context.Context, _ gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetMe401JSONResponse(errUnauthorized), nil
	}
	return gen.GetMe200JSONResponse(current.user), nil
}

// UpdateMe меняет полное имя и «о себе» — оба поля сразу.
func (s *Server) UpdateMe(ctx context.Context, request gen.UpdateMeRequestObject) (gen.UpdateMeResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.UpdateMe401JSONResponse(errUnauthorized), nil
	}
	if request.Body == nil {
		return gen.UpdateMe400JSONResponse(errEmptyRequest), nil
	}

	name, err := profile.NormalizeName(request.Body.Name)
	if err != nil {
		return gen.UpdateMe400JSONResponse(errInvalidName), nil
	}

	var rawAbout string
	if request.Body.About != nil {
		rawAbout = *request.Body.About
	}
	about, err := profile.NormalizeAbout(rawAbout)
	if err != nil {
		return gen.UpdateMe400JSONResponse(errInvalidAbout), nil
	}

	user, err := s.scanUser(s.db.QueryRow(ctx, `
		UPDATE users SET name = $2, about = $3
		WHERE id = $1
		RETURNING `+userColumns, current.user.Id, name, about))
	if err != nil {
		return nil, err
	}

	return gen.UpdateMe200JSONResponse(user), nil
}

// SetNickname выбирает или меняет никнейм (specs/010-nicknames.md).
// Занятость проверяет уникальный индекс по lower(nickname): проверка
// перед записью не спасла бы от двух одновременных запросов.
func (s *Server) SetNickname(ctx context.Context, request gen.SetNicknameRequestObject) (gen.SetNicknameResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.SetNickname401JSONResponse(errUnauthorized), nil
	}
	if request.Body == nil {
		return gen.SetNickname400JSONResponse(errEmptyRequest), nil
	}

	nickname, err := profile.NormalizeNickname(request.Body.Nickname)
	if err != nil {
		return gen.SetNickname400JSONResponse(errInvalidNickname), nil
	}

	user, err := s.scanUser(s.db.QueryRow(ctx, `
		UPDATE users SET nickname = $2, nickname_chosen = true
		WHERE id = $1
		RETURNING `+userColumns, current.user.Id, nickname))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "users_nickname_lower_idx" {
		return gen.SetNickname409JSONResponse(errNicknameTaken), nil
	}
	if err != nil {
		return nil, err
	}

	return gen.SetNickname200JSONResponse(user), nil
}

// SetAvatar принимает картинку, уменьшает её и делает аватаром.
func (s *Server) SetAvatar(ctx context.Context, request gen.SetAvatarRequestObject) (gen.SetAvatarResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.SetAvatar401JSONResponse(errUnauthorized), nil
	}
	if request.Body == nil {
		return gen.SetAvatar400JSONResponse(errEmptyRequest), nil
	}

	raw, err := readUpload(request.Body, "file", media.AvatarMaxBytes)
	switch {
	case errors.Is(err, errUploadTooLarge):
		return gen.SetAvatar413JSONResponse(errImageTooLarge), nil
	case err != nil:
		return gen.SetAvatar400JSONResponse(errEmptyRequest), nil
	}

	avatar, err := media.NormalizeAvatar(raw)
	if errors.Is(err, media.ErrNotAnImage) {
		return gen.SetAvatar400JSONResponse(errInvalidImage), nil
	}
	if err != nil {
		return nil, err
	}

	key, err := media.Key("avatars", media.AvatarExt)
	if err != nil {
		return nil, err
	}
	if err := s.cfg.Media.Put(ctx, key, avatar); err != nil {
		return nil, err
	}

	user, err := s.replaceAvatar(ctx, current.user.Id, &key)
	if err != nil {
		return nil, err
	}

	return gen.SetAvatar200JSONResponse(user), nil
}

// DeleteAvatar убирает аватар. Убрать несуществующий аватар — не ошибка.
func (s *Server) DeleteAvatar(ctx context.Context, _ gen.DeleteAvatarRequestObject) (gen.DeleteAvatarResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.DeleteAvatar401JSONResponse(errUnauthorized), nil
	}

	user, err := s.replaceAvatar(ctx, current.user.Id, nil)
	if err != nil {
		return nil, err
	}

	return gen.DeleteAvatar200JSONResponse(user), nil
}

// replaceAvatar ставит пользователю новый ключ аватара и убирает файл
// прежнего: два аватара одному человеку не нужны, а мусор в хранилище
// потом никто не найдёт.
func (s *Server) replaceAvatar(ctx context.Context, userID string, key *string) (gen.CurrentUser, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return gen.CurrentUser{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var previous *string
	if err := tx.QueryRow(ctx, `
		SELECT avatar_key FROM users WHERE id = $1 FOR UPDATE`, userID,
	).Scan(&previous); err != nil {
		return gen.CurrentUser{}, err
	}

	user, err := s.scanUser(tx.QueryRow(ctx, `
		UPDATE users SET avatar_key = $2
		WHERE id = $1
		RETURNING `+userColumns, userID, key))
	if err != nil {
		return gen.CurrentUser{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return gen.CurrentUser{}, err
	}

	if previous != nil && (key == nil || *previous != *key) {
		if err := s.cfg.Media.Delete(ctx, *previous); err != nil {
			// Ссылки на файл больше нет, так что для пользователя
			// всё в порядке; недоудалённый файл — забота владельца
			// сервиса, а не повод отвечать ошибкой.
			slog.Error("не удалось удалить прежний аватар", "key", *previous, "err", err)
		}
	}

	return user, nil
}

// scanUser читает профиль из строки, выбранной по userColumns.
func (s *Server) scanUser(row pgx.Row) (gen.CurrentUser, error) {
	var (
		user      gen.CurrentUser
		avatarKey *string
	)
	if err := row.Scan(
		&user.Id, &user.Phone, &user.CreatedAt,
		&user.Nickname, &user.NicknameChosen, &user.Name, &user.About, &avatarKey,
		&user.Closed,
	); err != nil {
		return gen.CurrentUser{}, err
	}

	if avatarKey != nil {
		url := s.cfg.Media.URL(*avatarKey)
		user.AvatarUrl = &url
	}
	return user, nil
}

// errUploadTooLarge — загружаемый файл больше разрешённого.
var errUploadTooLarge = errors.New("файл слишком большой")

// readUpload достаёт из multipart-запроса файл с этим именем поля.
//
// Читает не больше limit+1 байта: файл больше разрешённого не должен
// сначала попасть на сервис целиком, а потом быть отвергнутым.
func readUpload(form *multipart.Reader, field string, limit int) ([]byte, error) {
	for {
		part, err := form.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("в запросе нет файла")
		}
		if err != nil {
			return nil, err
		}

		// Нужен именно файл: текстовое поле с тем же именем — это не
		// картинка. Сгенерированный Dart-клиент кладёт рядом с файлом
		// одноимённое текстовое поле, и принимать его за картинку нельзя.
		if part.FormName() != field || part.FileName() == "" {
			continue
		}

		content, err := io.ReadAll(io.LimitReader(part, int64(limit)+1))
		if err != nil {
			return nil, err
		}
		if len(content) > limit {
			return nil, errUploadTooLarge
		}
		return content, nil
	}
}

// Ошибки профиля (specs/002-profile.md, «Ошибки»).
var (
	errInvalidName = gen.Error{
		Code:    "invalid_name",
		Message: "Имя — до 50 символов и в одну строку",
	}
	errInvalidNickname = gen.Error{
		Code:    "invalid_nickname",
		Message: "Никнейм — от 3 до 20 символов: латиница, цифры и _",
	}
	errNicknameTaken = gen.Error{
		Code:    "nickname_taken",
		Message: "Этот никнейм уже занят",
	}
	errInvalidAbout = gen.Error{
		Code:    "invalid_about",
		Message: "«О себе» — до 200 символов и в одну строку",
	}
	errInvalidImage = gen.Error{
		Code:    "invalid_image",
		Message: "Это не похоже на фотографию: нужен JPEG или PNG",
	}
	errImageTooLarge = gen.Error{
		Code:    "image_too_large",
		Message: "Фотография больше 5 МБ. Выберите поменьше",
	}
)
