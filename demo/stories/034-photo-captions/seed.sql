-- Данные сценария «Подпись к каждому фото».
--
-- Основа — «Живая дача» ([000-dacha]). Пост 3 — три снимка томатов,
-- в общей подписи сорта перечислены словами; теперь у каждого фото своя
-- подпись. У первого её нарочно нет: место под карусель не занимается.
-- Николай (+79000000001), которым входят на показе, публикует свой.
\i /stories/000-dacha/seed.sql

UPDATE media m SET caption = c.caption
FROM (VALUES
	(2, '«Чёрный принц» — в миске. Сладкий, но трескается после дождя'),
	(3, 'А это «Хурма», в корзине. Самый поздний, дозрел только сейчас #томаты')
) AS c(pos, caption)
WHERE m.post_id = pg_temp.dacha_post(3) AND m.position = c.pos;

INSERT INTO post_tags (post_id, tag, position)
SELECT pg_temp.dacha_post(3), 'томаты', 1
WHERE NOT EXISTS (SELECT 1 FROM post_tags WHERE post_id = pg_temp.dacha_post(3) AND tag = 'томаты');
