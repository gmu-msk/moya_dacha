package api

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// Подпись к каждому фото поста: specs/034-photo-captions.md.
// Подпись — колонка media.caption, а не комментарий (ADR-0031).

// errPhotoCaptionTooLong — подпись фото длиннее подписи поста.
var errPhotoCaptionTooLong = errors.New("подпись фото слишком длинная")

// photoCaptions обрезает края присланных подписей фото и проверяет длину
// (требования 1 и 6). Пустая после обрезки — подписи нет: в карте она
// остаётся пустой строкой, чтобы при правке снять прежнюю.
func photoCaptions(raw *gen.MediaCaptions) (map[string]string, error) {
	if raw == nil {
		return nil, nil
	}
	captions := make(map[string]string, len(*raw))
	for id, text := range *raw {
		text = strings.TrimSpace(text)
		if utf8.RuneCountInString(text) > MaxCaptionLength {
			return nil, errPhotoCaptionTooLong
		}
		captions[id] = text
	}
	return captions, nil
}

// writePhotoCaptions пишет подписи фото этого поста и говорит, поменялся
// ли хоть один текст. Ключ, который не фото поста, — errMediaUnusable
// (требования 5 и 9); тогда транзакция откатывается целиком.
func writePhotoCaptions(ctx context.Context, tx pgx.Tx, postID string, captions map[string]string) (bool, error) {
	if len(captions) == 0 {
		return false, nil
	}
	ids := make([]string, 0, len(captions))
	texts := make([]string, 0, len(captions))
	for id, text := range captions {
		if !isUUID(id) {
			return false, errMediaUnusable
		}
		ids = append(ids, id)
		texts = append(texts, text)
	}

	var found int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM media WHERE post_id = $1 AND id = ANY($2::uuid[])`,
		postID, ids,
	).Scan(&found); err != nil {
		return false, err
	}
	if found != len(ids) {
		return false, errMediaUnusable
	}

	// Пустая строка в базу не пишется: NULL — подписи нет.
	tag, err := tx.Exec(ctx, `
		UPDATE media m SET caption = NULLIF(c.text, '')
		FROM unnest($2::uuid[], $3::text[]) AS c(id, text)
		WHERE m.post_id = $1 AND m.id = c.id
		  AND m.caption IS DISTINCT FROM NULLIF(c.text, '')`,
		postID, ids, texts)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// refreshPostTags пересчитывает тэги поста из подписи поста и подписей
// его фото по порядку фото (требование 11). Перевод строки между ними —
// граница слова, так что хэштег одной подписи не склеится с другой.
func refreshPostTags(ctx context.Context, tx pgx.Tx, postID string) error {
	var text string
	if err := tx.QueryRow(ctx, `
		SELECT concat_ws(E'\n', p.caption,
		       (SELECT string_agg(m.caption, E'\n' ORDER BY m.position)
		        FROM media m WHERE m.post_id = p.id))
		FROM posts p WHERE p.id = $1`, postID,
	).Scan(&text); err != nil {
		return err
	}
	return writeTags(ctx, tx, postID, captionTags(text))
}
