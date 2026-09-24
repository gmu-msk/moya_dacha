-- Данные сценария «Видимость постов».
--
-- Основа — «Живая дача» ([000-dacha]) с подписками. Николай (номер 01)
-- подписан на Валентину, Галину Сергеевну, Михалыча, Людмилу и
-- председателя; взаимно — только с Михалычем (07): они друзья.
-- Николай подписан на Людмилу (12), а она на него нет, поэтому её пост
-- «друзьям» ему не виден.
\i /stories/000-dacha/seed.sql

-- Антоновка Михалыча — друзьям: Николай её видит, с отметкой.
UPDATE posts SET visibility = 'friends' WHERE id = pg_temp.dacha_post(9);
-- Семена «Бычьего сердца» у Людмилы — друзьям: Николаю не видны.
UPDATE posts SET visibility = 'friends' WHERE id = pg_temp.dacha_post(18);
-- Кабачки Николая — друзьям, подсолнухи — только ему.
UPDATE posts SET visibility = 'friends' WHERE id = pg_temp.dacha_post(17);
UPDATE posts SET visibility = 'me' WHERE id = pg_temp.dacha_post(47);
-- Тля Валентины — только ей.
UPDATE posts SET visibility = 'me' WHERE id = pg_temp.dacha_post(16);
