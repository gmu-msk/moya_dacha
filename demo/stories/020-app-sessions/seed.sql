-- Заходы и время в приложении (specs/020-app-sessions.md): «Живая дача»
-- и месяц сессий, чтобы раздел дашборда не стоял пустым. Случайность
-- детерминированная — из md5, — поэтому картинка одна и та же при
-- каждом сбросе стенда.
\i /stories/000-dacha/seed.sql

-- Кто и когда заходил: чем ближе к сегодняшнему дню, тем больше людей
-- (тест расходится), днём чаще, чем ночью; сессия от 1 до 25 минут.
-- Последняя сессия Валентины — без конца, как приложение, убитое системой.
WITH people AS (
    SELECT id, row_number() OVER (ORDER BY id) AS n FROM users
),
draws AS (
    SELECT p.id AS user_id, d, k,
           ('x' || substr(md5(p.id::text || d || '-' || k), 1, 8))::bit(32)::bigint AS r
    FROM people p, generate_series(0, 29) AS d, generate_series(1, 3) AS k
),
picked AS (
    SELECT user_id, d, k, r,
           date_trunc('day', now() AT TIME ZONE 'Europe/Moscow') - make_interval(days => d)
               + make_interval(hours => 8 + (r % 13)::int, mins => (r / 13 % 60)::int) AS local_start
    FROM draws
    WHERE r % 100 < 70 - 2 * d
)
INSERT INTO app_sessions (id, user_id, started_at, ended_at)
SELECT md5(user_id::text || d || '-' || k)::uuid,
       user_id,
       local_start AT TIME ZONE 'Europe/Moscow',
       (local_start AT TIME ZONE 'Europe/Moscow') + make_interval(mins => 1 + (r / 7 % 25)::int, secs => (r % 60)::int)
FROM picked
WHERE local_start AT TIME ZONE 'Europe/Moscow' < now();

UPDATE app_sessions SET ended_at = NULL
WHERE id = (SELECT id FROM app_sessions
            WHERE user_id = '22222222-2222-4222-8222-222222222222'
            ORDER BY started_at DESC LIMIT 1);

-- Экраны: лента есть почти в каждой сессии, дальше — реже.
INSERT INTO app_session_screens (session_id, screen, opens)
SELECT s.id, sc.screen,
       1 + (('x' || substr(md5(s.id::text || sc.screen), 1, 8))::bit(32)::bigint % sc.most)::int
FROM app_sessions s,
     (VALUES ('feed', 100, 4), ('post', 70, 6), ('user', 40, 3), ('notifications', 45, 2),
             ('profile', 35, 2), ('new_post', 15, 1), ('follows', 10, 2), ('profile_edit', 6, 1),
             ('user_posts', 8, 2), ('feedback', 3, 1), ('about', 2, 1)) AS sc(screen, chance, most)
WHERE ('x' || substr(md5(s.id::text || sc.screen || 'p'), 1, 8))::bit(32)::bigint % 100 < sc.chance;
