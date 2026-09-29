-- Данные сценария «Модерация» (specs/023-moderation.md): «Живая дача»,
-- жалобы на объявление и на реплику под ним, одна жалоба зря и свежий
-- комментарий с чужим номером, на который никто не пожаловался.
\i /stories/000-dacha/seed.sql

-- Объявление Петра о перегное (пост 2): двое сочли его рекламой.
-- Антоновка Михалыча (пост 9): жалоба без причины, по ошибке.
INSERT INTO reports (post_id, reporter_id, reason, created_at)
SELECT pg_temp.dacha_post(r.post), u.id, r.reason, now() - make_interval(mins => r.ago)
FROM (VALUES
	(2,  2, 'Это реклама, а не дача', 120),
	(2,  9, 'Реклама',                 40),
	(9, 11, NULL,                      15)
) AS r(post, who, reason, ago)
JOIN dacha_users u ON u.n = r.who;

-- Реплика Валентины под объявлением: Пётр счёл её грубой.
INSERT INTO reports (comment_id, reporter_id, reason, created_at)
SELECT c.id, '33333333-3333-4333-8333-333333333333'::uuid, 'Грубо', now() - interval '30 minutes'
FROM comments c
WHERE c.post_id = pg_temp.dacha_post(2) AND c.text LIKE 'Опять реклама%';

-- Свежий комментарий Сергея с номером соседа — жалобы нет, его владелец
-- находит сам в «Свежем». Номер выдуманный.
INSERT INTO comments (post_id, author_id, text, created_at)
VALUES (pg_temp.dacha_post(1), '99999999-9999-4999-8999-999999999999'::uuid,
	'Кому плёнку снимать — зовите Иваныча с 12-го участка, вот его номер: +7 900 000-99-99, звоните хоть ночью',
	now() - interval '5 minutes');
