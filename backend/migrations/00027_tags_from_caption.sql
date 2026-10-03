-- Тэги поста — хэштеги в подписи: specs/028-post-tags.md, ADR-0029.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Тэги, поставленные отдельным полем до этой правки, дописываются в конец
-- подписи хэштегами: иначе первая правка подписи пересчитала бы тэги по
-- тексту и стёрла их. Уже стоящий в подписи хэштег не повторяется.
-- edited_at не трогается: человек подпись не правил.
UPDATE posts p
SET caption = btrim(p.caption || ' ' || missing.hashtags)
FROM (
    SELECT t.post_id, string_agg('#' || t.tag, ' ' ORDER BY t.position) AS hashtags
    FROM post_tags t
    JOIN posts q ON q.id = t.post_id
    WHERE lower(q.caption) !~ ('(^|[^[:alnum:]_-])#' || t.tag || '($|[^[:alnum:]_-])')
    GROUP BY t.post_id
) missing
WHERE p.id = missing.post_id;
