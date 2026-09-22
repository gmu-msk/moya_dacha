-- Данные сценария «Профиль пользователя».
--
-- Основа — «Живая дача» ([000-dacha]): там у каждого по четыре поста,
-- и сетка профиля в них — один неполный ряд. Чтобы было видно сетку во
-- весь экран и подгрузку второй страницы, Валентине (номер 02) добавлены
-- тридцать два поста постарше: всего у неё тридцать шесть, а страница
-- профиля — тридцать. И нужен человек, у которого постов нет вовсе:
-- Зинаида только что пришла и пока лишь спросила под постом Валентины.
\i /stories/000-dacha/seed.sql

-- Подписи для старых постов Валентины, по кругу.
CREATE TEMP TABLE valentina_captions (k int PRIMARY KEY, caption text NOT NULL);
INSERT INTO valentina_captions VALUES
	(0, 'Огурцы пошли, собираю каждый день'),
	(1, 'Рассада в этом году крепкая, тьфу-тьфу'),
	(2, 'Перцы в теплице, пока зелёные'),
	(3, 'Закатала десять банок, это только начало'),
	(4, ''),
	(5, 'Помидоры подвязала, теперь только ждать'),
	(6, 'Клубника у соседки через забор. Завидую'),
	(7, 'Клумбу у теплицы наконец довела до ума');

-- Фотографии — те же, что уже есть в хранилище стенда, с их размерами.
CREATE TEMP TABLE valentina_photos AS
SELECT row_number() OVER (ORDER BY storage_key) - 1 AS k, storage_key, width, height
FROM (
	SELECT DISTINCT ON (storage_key) storage_key, width, height
	FROM media
	WHERE storage_key ~ '^demo/(greenhouse|cucumbers|tomatoes|peppers|seedlings|jars|strawberry|flowerbed)-'
	ORDER BY storage_key
) AS photos;

-- Посты 101…132 старше всех её постов из «Живой дачи»: от сорока дней
-- до полугода назад, раз в пять дней.
INSERT INTO posts (id, author_id, caption, created_at)
SELECT pg_temp.dacha_post(100 + g), u.id, c.caption, now() - make_interval(days => 35 + g * 5)
FROM generate_series(1, 32) AS g
JOIN dacha_users u ON u.n = 2
JOIN valentina_captions c ON c.k = g % 8;

-- У каждого третьего поста две фотографии: в сетке у таких значок в углу.
INSERT INTO media (author_id, post_id, position, kind, storage_key, width, height)
SELECT u.id, pg_temp.dacha_post(100 + g), 1, 'photo', f.storage_key, f.width, f.height
FROM generate_series(1, 32) AS g
JOIN dacha_users u ON u.n = 2
JOIN valentina_photos f ON f.k = g % (SELECT count(*) FROM valentina_photos)
UNION ALL
SELECT u.id, pg_temp.dacha_post(100 + g), 2, 'photo', f.storage_key, f.width, f.height
FROM generate_series(3, 32, 3) AS g
JOIN dacha_users u ON u.n = 2
JOIN valentina_photos f ON f.k = (g + 1) % (SELECT count(*) FROM valentina_photos);

-- Зинаида: пришла позавчера, без аватара и без постов. Её профиль
-- открывается по имени под комментарием.
INSERT INTO users (id, phone, created_at, name, about)
VALUES (
	'16161616-1616-4616-8616-161616161616',
	'+79000000016',
	now() - interval '2 days',
	'Зинаида',
	''
);

INSERT INTO comments (post_id, author_id, text, created_at)
VALUES (
	pg_temp.dacha_post(1),
	'16161616-1616-4616-8616-161616161616',
	'Валентина, а плёнку на зиму снимаете совсем? Я тут первый год',
	now() - interval '10 minutes'
);
