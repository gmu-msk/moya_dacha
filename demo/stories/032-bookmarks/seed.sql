-- Данные сценария «Закладки».
--
-- Основа — «Живая дача» ([000-dacha]). Сверху — закладки. Николай
-- (+79000000001), которым входят на показе, сохранил пять постов, и
-- сохранял их не по порядку ленты: последним — пост 25, поэтому он
-- первый в «Сохранённых», хотя в ленте он ниже остальных. Другие
-- дачники сохраняли полезное — сорта томатов (3), отключение воды (6),
-- картошку (12): у этих постов под закладкой число, у большинства
-- остальных числа нет.
\i /stories/000-dacha/seed.sql

-- Закладки Николая: номер поста и сколько часов назад сохранил.
INSERT INTO post_bookmarks (user_id, post_id, created_at)
SELECT u.id, pg_temp.dacha_post(v.post), now() - make_interval(hours => v.hours)
FROM (VALUES (25, 1), (3, 5), (17, 20), (6, 30), (12, 50)) AS v(post, hours)
JOIN dacha_users u ON u.n = 1;

-- Закладки остальных: номер поста и сколько дачников его сохранили.
INSERT INTO post_bookmarks (user_id, post_id, created_at)
SELECT u.id, p.id, p.created_at + make_interval(mins => u.n * 7)
FROM (VALUES (3, 6), (6, 4), (12, 3), (1, 1), (14, 2)) AS v(post, savers)
JOIN posts p ON p.id = pg_temp.dacha_post(v.post)
JOIN dacha_users u ON u.n BETWEEN 2 AND 1 + v.savers AND u.id <> p.author_id
WHERE p.created_at + make_interval(mins => u.n * 7) < now();
