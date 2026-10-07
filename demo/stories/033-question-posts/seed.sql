-- Данные сценария «Пост-вопрос».
--
-- Основа — «Живая дача» ([000-dacha]). Сверху — вопросы. Три поста,
-- которые и так спрашивают, стали вопросами: про тлю у Валентины (16)
-- решён — решением отмечен совет Сергея про муравьёв; про розы у Галины
-- Сергеевны (23) и про урожай томатов у Валентины (31) — ещё нет.
-- Николай (+79000000001), которым входят на показе, сегодня спросил про
-- огурцы, и ему уже ответили двое: отмечать решение — ему.
\i /stories/000-dacha/seed.sql

UPDATE posts SET question = true
WHERE id IN (pg_temp.dacha_post(16), pg_temp.dacha_post(23), pg_temp.dacha_post(31));

-- Решение у тли — «Ещё муравьёв надо гнать» Сергея (пользователь 12).
UPDATE posts p SET answer_comment_id = c.id, solved = true
FROM comments c
JOIN dacha_users u ON u.id = c.author_id AND u.n = 12
WHERE p.id = pg_temp.dacha_post(16) AND c.post_id = p.id;

-- Свежий вопрос Николая, пост 61 — первый в ленте.
INSERT INTO posts (id, author_id, caption, created_at, question)
SELECT pg_temp.dacha_post(61), u.id,
	'Огурцы в теплице: листья желтеют снизу и сохнут по краям. Что с ними?',
	now() - interval '40 minutes', true
FROM dacha_users u WHERE u.n = 1;

INSERT INTO media (author_id, post_id, position, kind, storage_key, width, height)
SELECT p.author_id, p.id, 1, 'photo', 'demo/cucumbers-1.jpg', 1200, 1600
FROM posts p WHERE p.id = pg_temp.dacha_post(61);

INSERT INTO comments (post_id, author_id, text, created_at)
SELECT pg_temp.dacha_post(61), u.id, c.text, now() - make_interval(mins => c.ago)
FROM (VALUES
	( 4, 30, 'Николай, похоже на нехватку азота. Подкормите мочевиной'),
	( 2, 20, 'А поливаете чем? Если холодной из скважины — от этого тоже желтеют'),
	(13, 10, 'Нижние листья желтеют к осени у всех, это нормально. Обрывайте их')
) AS c(author, ago, text)
JOIN dacha_users u ON u.n = c.author;
