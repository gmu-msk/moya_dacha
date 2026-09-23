-- Данные сценария «Никнейм».
--
-- Основа — «Живая дача» ([000-dacha]): у всех дачников никнеймы уже
-- выбраны. Чтобы показать, что видит человек, заведённый до никнеймов,
-- Николай (номер 01) носит временный никнейм и ещё не выбирал свой:
-- при входе его встретит экран знакомства, а до тех пор соседи видят
-- его постами dachnik_…. Денис (номер 11) полное имя не указывал —
-- его профиль показывает один никнейм.
\i /stories/000-dacha/seed.sql

UPDATE users SET nickname = 'dachnik_3f9a01bc27', nickname_chosen = false
WHERE id = (SELECT id FROM dacha_users WHERE n = 1);

UPDATE users SET name = ''
WHERE id = (SELECT id FROM dacha_users WHERE n = 11);
