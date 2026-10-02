-- Данные сценария «Кто увидит — только участники группы».
--
-- Основа — сценарий постов в группах ([030-group-posts]). Сверху:
--   - Николай (01) — на двух дачах: кроме «снт Ромашка» вступил в
--     геогруппу «снт Рассвет»; в «Розы и клематисы» он только приглашён;
--   - три поста групп становятся «только участникам»: 22 — «снт
--     Ромашка», 36 — «снт Рассвет», 38 — «Розы и клематисы». Последний
--     Николай не видит, пока не примет приглашение
--     (specs/031-group-visibility.md, требования 5 и 7).
\i /stories/030-group-posts/seed.sql

INSERT INTO group_members (group_id, user_id, state, created_at)
SELECT g.id, u.id, 'member', now() - interval '1 day'
FROM groups g, dacha_users u
WHERE g.place_id = '00000000-0000-4000-8000-000000000291' AND u.n = 1
ON CONFLICT (group_id, user_id) DO UPDATE SET state = 'member';

UPDATE posts p SET visibility = 'group', visibility_group_id = coalesce(
	(SELECT id FROM groups WHERE place_id = CASE v.grp
		WHEN 'romashka' THEN '00000000-0000-4000-8000-000000000292'
		WHEN 'rassvet' THEN '00000000-0000-4000-8000-000000000291' END),
	(SELECT id FROM demo_groups WHERE n::text = v.grp))
FROM (VALUES (22, 'romashka'), (36, 'rassvet'), (38, '1')) AS v(post, grp)
WHERE p.id = pg_temp.dacha_post(v.post);
