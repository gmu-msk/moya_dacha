-- Данные сценария «Уведомления».
--
-- Основа — «Живая дача» ([000-dacha]): события к постам Николая (номер 01)
-- уже есть, их база записала сама, а раздел он открывал полдня назад.
-- Здесь к ним добавлено свежее, чтобы были видны все четыре вида и заявка.
\i /stories/000-dacha/seed.sql

-- Лайки малины Николая: последним отметил Михалыч час назад — строка
-- «Mikhalych и ещё N отметили ваш пост».
DELETE FROM post_likes
WHERE post_id = pg_temp.dacha_post(4)
  AND user_id = (SELECT id FROM dacha_users WHERE n = 7);
INSERT INTO post_likes (post_id, user_id, created_at)
SELECT pg_temp.dacha_post(4), id, now() - interval '1 hour'
FROM dacha_users WHERE n = 7;

-- Людмила спрашивает про сорт четверть часа назад.
INSERT INTO comments (post_id, author_id, text, created_at)
SELECT pg_temp.dacha_post(4), id, 'Какая крупная! Сорт какой? Хочу себе пару кустов весной', now() - interval '15 minutes'
FROM dacha_users WHERE n = 12;

-- Алексей-пчеловод подписался на Николая три часа назад, а Николай на него
-- нет: в строке кнопка «Подписаться».
DELETE FROM follows
WHERE follower_id = (SELECT id FROM dacha_users WHERE n = 13)
  AND followee_id = (SELECT id FROM dacha_users WHERE n = 1);
INSERT INTO follows (follower_id, followee_id, accepted, created_at)
SELECT a.id, b.id, true, now() - interval '3 hours'
FROM dacha_users a, dacha_users b WHERE a.n = 13 AND b.n = 1;

-- Катя закрыла профиль; Николай подал ей заявку, вчера она её приняла —
-- «katya_rassada принял вашу заявку» в «Раньше».
UPDATE users SET closed = true
WHERE id = (SELECT id FROM dacha_users WHERE n = 14);
DELETE FROM follows
WHERE follower_id = (SELECT id FROM dacha_users WHERE n = 1)
  AND followee_id = (SELECT id FROM dacha_users WHERE n = 14);
INSERT INTO follows (follower_id, followee_id, accepted, created_at)
SELECT a.id, b.id, false, now() - interval '30 hours'
FROM dacha_users a, dacha_users b WHERE a.n = 1 AND b.n = 14;
UPDATE follows SET accepted = true, created_at = now() - interval '26 hours'
WHERE follower_id = (SELECT id FROM dacha_users WHERE n = 1)
  AND followee_id = (SELECT id FROM dacha_users WHERE n = 14);

-- Николай закрыл профиль, и десять минут назад к нему пришла заявка от
-- Дениса — она сверху раздела с «Принять» и «Отклонить».
UPDATE users SET closed = true
WHERE id = (SELECT id FROM dacha_users WHERE n = 1);
DELETE FROM follows
WHERE follower_id = (SELECT id FROM dacha_users WHERE n = 11)
  AND followee_id = (SELECT id FROM dacha_users WHERE n = 1);
INSERT INTO follows (follower_id, followee_id, accepted, created_at)
SELECT a.id, b.id, false, now() - interval '10 minutes'
FROM dacha_users a, dacha_users b WHERE a.n = 11 AND b.n = 1;
