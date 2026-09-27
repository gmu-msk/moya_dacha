// Посты: specs/003-posts.md.
package api

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
	"github.com/gmu-msk/moya_dacha/backend/internal/media"
)

// MaxCaptionLength — сколько символов помещается в подпись.
const MaxCaptionLength = 1000

// MaxPostMedia — сколько фотографий помещается в пост. Нижняя разумная
// граница взята сознательно: поднять лимит потом ничего не сломает,
// опустить уже не выйдет (ADR-0006).
const MaxPostMedia = 10

// UploadMedia принимает фотографию, уменьшает её и запоминает.
// Опубликованной она станет, когда автор соберёт из неё пост.
func (s *Server) UploadMedia(ctx context.Context, request gen.UploadMediaRequestObject) (gen.UploadMediaResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.UploadMedia401JSONResponse(errUnauthorized), nil
	}
	if request.Body == nil {
		return gen.UploadMedia400JSONResponse(errEmptyRequest), nil
	}

	raw, err := readUpload(request.Body, "file", media.PhotoMaxBytes)
	switch {
	case errors.Is(err, errUploadTooLarge):
		return gen.UploadMedia413JSONResponse(errPhotoTooLarge), nil
	case err != nil:
		return gen.UploadMedia400JSONResponse(errEmptyRequest), nil
	}

	photo, err := media.NormalizePhoto(raw)
	if errors.Is(err, media.ErrNotAnImage) {
		return gen.UploadMedia400JSONResponse(errInvalidImage), nil
	}
	if errors.Is(err, media.ErrTooManyPixels) {
		return gen.UploadMedia413JSONResponse(errTooManyPixels), nil
	}
	if err != nil {
		return nil, err
	}

	key, err := media.Key(media.PhotoDir, media.PhotoExt)
	if err != nil {
		return nil, err
	}
	if err := s.cfg.Media.Put(ctx, key, photo.Content); err != nil {
		return nil, err
	}

	var id string
	if err := s.db.QueryRow(ctx, `
		INSERT INTO media (author_id, kind, storage_key, width, height)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		current.user.Id, string(gen.Photo), key, photo.Width, photo.Height,
	).Scan(&id); err != nil {
		return nil, err
	}

	return gen.UploadMedia201JSONResponse{
		Id:     id,
		Kind:   gen.Photo,
		Url:    s.cfg.Media.URL(key),
		Width:  int32(photo.Width),
		Height: int32(photo.Height),
	}, nil
}

// CreatePost собирает пост из уже загруженных фотографий.
func (s *Server) CreatePost(ctx context.Context, request gen.CreatePostRequestObject) (gen.CreatePostResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.CreatePost401JSONResponse(errUnauthorized), nil
	}
	if request.Body == nil {
		return gen.CreatePost400JSONResponse(errEmptyRequest), nil
	}

	switch {
	case len(request.Body.MediaIds) == 0:
		return gen.CreatePost400JSONResponse(errNoMedia), nil
	case len(request.Body.MediaIds) > MaxPostMedia:
		return gen.CreatePost400JSONResponse(errTooManyMedia), nil
	}

	var rawCaption string
	if request.Body.Caption != nil {
		rawCaption = *request.Body.Caption
	}
	caption := strings.TrimSpace(rawCaption)
	if utf8.RuneCountInString(caption) > MaxCaptionLength {
		return gen.CreatePost400JSONResponse(errInvalidCaption), nil
	}

	// Без видимости пост виден всем (specs/013-post-visibility.md,
	// требование 1).
	visibility := gen.PostVisibility("all")
	if request.Body.Visibility != nil {
		visibility = *request.Body.Visibility
	}
	if !validVisibility(visibility) {
		return gen.CreatePost400JSONResponse(errInvalidVisibility), nil
	}

	id, err := s.insertPost(ctx, current.user.Id, caption, visibility, request.Body.MediaIds)
	if errors.Is(err, errMediaUnusable) {
		return gen.CreatePost400JSONResponse(errInvalidMedia), nil
	}
	if err != nil {
		return nil, err
	}

	s.forgetUnpublishedMedia(ctx, current.user.Id)

	post, err := s.post(ctx, id, current.user.Id)
	if err != nil {
		return nil, err
	}
	return gen.CreatePost201JSONResponse(post), nil
}

// GetPost отдаёт пост тому, кому он виден (specs/013-post-visibility.md).
// Невидимый — 404, как несуществующий: не выдаём, что пост есть.
func (s *Server) GetPost(ctx context.Context, request gen.GetPostRequestObject) (gen.GetPostResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetPost401JSONResponse(errUnauthorized), nil
	}
	if !isUUID(request.PostId) {
		return gen.GetPost404JSONResponse(errPostNotFound), nil
	}

	post, err := s.post(ctx, request.PostId, current.user.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.GetPost404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.GetPost200JSONResponse(post), nil
}

// errMediaUnusable — фотографии нет, она чужая или уже в другом посте.
var errMediaUnusable = errors.New("фотография не годится для поста")

// insertPost заводит пост и прикрепляет к нему фотографии.
//
// Пост создаётся целиком или не создаётся вовсе: прикрепление каждой
// фотографии — это перевод строки из «загружено» в «опубликовано», и
// если хоть один перевод не удался, транзакция откатывается целиком
// (specs/003-posts.md).
func (s *Server) insertPost(ctx context.Context, authorID, caption string, visibility gen.PostVisibility, mediaIDs []string) (string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO posts (author_id, caption, visibility) VALUES ($1, $2, $3)
		RETURNING id`, authorID, caption, string(visibility),
	).Scan(&id); err != nil {
		return "", err
	}

	for position, mediaID := range mediaIDs {
		if !isUUID(mediaID) {
			return "", errMediaUnusable
		}

		// Условие `post_id IS NULL` делает прикрепление одноразовым:
		// повторно та же фотография не прикрепится ни в этот пост,
		// ни в чужой.
		tag, err := tx.Exec(ctx, `
			UPDATE media SET post_id = $1, position = $2
			WHERE id = $3 AND author_id = $4 AND post_id IS NULL`,
			id, position+1, mediaID, authorID)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() != 1 {
			return "", errMediaUnusable
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// forgetUnpublishedMedia убирает фотографии, которые автор загрузил, но
// так и не опубликовал. Отдельного расписания в проекте нет (ADR-0004),
// и заводить его ради этого не нужно: мусор убирается тогда же, когда
// появляется новый пост того же автора.
//
// Ошибки только логируются: пост уже создан, и для автора всё в порядке.
func (s *Server) forgetUnpublishedMedia(ctx context.Context, authorID string) {
	rows, err := s.db.Query(ctx, `
		DELETE FROM media
		WHERE author_id = $1 AND post_id IS NULL AND created_at < now() - $2::interval
		RETURNING storage_key`, authorID, s.cfg.UnpublishedMediaTTL.String())
	if err != nil {
		slog.Error("не удалось убрать неопубликованные фотографии", "err", err)
		return
	}

	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		slog.Error("не удалось убрать неопубликованные фотографии", "err", err)
		return
	}

	for _, key := range keys {
		if err := s.cfg.Media.Delete(ctx, key); err != nil {
			slog.Error("не удалось удалить файл неопубликованной фотографии", "key", key, "err", err)
		}
	}
}

// post собирает пост целиком: сам пост, его автора, его медиа, лайки и
// число комментариев. Признак «я отметил» считается для того, кто
// спрашивает, поэтому viewerID — часть запроса, а не поста
// (specs/005-likes.md).
func (s *Server) post(ctx context.Context, id, viewerID string) (gen.Post, error) {
	var (
		post      gen.Post
		avatarKey *string
	)
	if err := s.db.QueryRow(ctx, `
		SELECT p.id, p.created_at, p.edited_at, p.caption, p.visibility,
			u.id, u.nickname, u.name, u.avatar_key,
			`+likeColumns+`,
			`+commentCount("$2")+`
		FROM posts p JOIN users u ON u.id = p.author_id
		WHERE p.id = $1 AND `+postVisibleTo("$2"), id, viewerID,
	).Scan(
		&post.Id, &post.CreatedAt, &post.EditedAt, &post.Caption, &post.Visibility,
		&post.Author.Id, &post.Author.Nickname, &post.Author.Name, &avatarKey,
		&post.Likes, &post.Liked, &post.Comments,
	); err != nil {
		return gen.Post{}, err
	}
	if avatarKey != nil {
		url := s.cfg.Media.URL(*avatarKey)
		post.Author.AvatarUrl = &url
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, kind, storage_key, width, height
		FROM media WHERE post_id = $1 ORDER BY position`, id)
	if err != nil {
		return gen.Post{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			item gen.Media
			kind string
			key  string
		)
		if err := rows.Scan(&item.Id, &kind, &key, &item.Width, &item.Height); err != nil {
			return gen.Post{}, err
		}
		item.Kind = gen.MediaKind(kind)
		item.Url = s.cfg.Media.URL(key)
		post.Media = append(post.Media, item)
	}
	if err := rows.Err(); err != nil {
		return gen.Post{}, err
	}

	return post, nil
}

// isUUID отвечает, похож ли идентификатор на UUID. Нужен, чтобы
// опечатка в адресе становилась честным «такого поста нет», а не
// ошибкой разбора в базе.
func isUUID(value string) bool {
	const length = 36
	if len(value) != length {
		return false
	}

	for i, symbol := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if symbol != '-' {
				return false
			}
			continue
		}
		switch {
		case symbol >= '0' && symbol <= '9',
			symbol >= 'a' && symbol <= 'f',
			symbol >= 'A' && symbol <= 'F':
		default:
			return false
		}
	}
	return true
}

// DefaultUnpublishedMediaTTL — сколько живёт загруженная, но не
// опубликованная фотография (specs/003-posts.md, требование 12).
const DefaultUnpublishedMediaTTL = time.Hour

// Ошибки постов (specs/003-posts.md, «Ошибки»).
var (
	errNoMedia = gen.Error{
		Code:    "no_media",
		Message: "Пост без фотографии не бывает. Добавьте хотя бы одну",
	}
	errTooManyMedia = gen.Error{
		Code:    "too_many_media",
		Message: "В пост помещается не больше четырёх фотографий",
	}
	errInvalidMedia = gen.Error{
		Code:    "invalid_media",
		Message: "Одна из фотографий недоступна. Загрузите её заново",
	}
	errInvalidCaption = gen.Error{
		Code:    "invalid_caption",
		Message: "Подпись длиннее 1000 символов",
	}
	errPhotoTooLarge = gen.Error{
		Code:    "image_too_large",
		Message: "Фотография больше 10 МБ. Выберите поменьше",
	}
	errTooManyPixels = gen.Error{
		Code:    "image_too_large",
		Message: "Картинка слишком большая: больше 8192×8192 точек. Выберите поменьше",
	}
	errPostNotFound = gen.Error{
		Code:    "post_not_found",
		Message: "Такого поста нет",
	}
)
