-- Данные сценария «Подписки».
--
-- Основа — «Живая дача» ([000-dacha]) с подписками между дачниками:
-- Николай (номер 01) подписан на пятерых, его «Подписки» не пусты.
-- Катя (номер 14) закрыла профиль: Николай на неё не подписан и видит
-- вместо её постов замок, а в ленте «Все» её постов нет. К Кате ждут
-- ответа две заявки — от Петра (03) и Дениса (11): их видно, если войти
-- Катей.
\i /stories/000-dacha/seed.sql

UPDATE users SET closed = true
WHERE id = (SELECT id FROM dacha_users WHERE n = 14);

-- Николай к Кате не подписан, заявок от него нет: подать её — часть
-- показа. Пётр и Денис заявку уже подали.
DELETE FROM follows
WHERE followee_id = (SELECT id FROM dacha_users WHERE n = 14)
  AND follower_id IN (SELECT id FROM dacha_users WHERE n IN (1, 3, 11));

INSERT INTO follows (follower_id, followee_id, accepted, created_at)
SELECT a.id, b.id, false, now() - make_interval(hours => a.n)
FROM dacha_users a, dacha_users b
WHERE a.n IN (3, 11) AND b.n = 14;
