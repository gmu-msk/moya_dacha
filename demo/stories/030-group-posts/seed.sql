-- Данные сценария «Посты в группах и лента группы».
--
-- Основа — сценарий групп ([029-groups]): «снт Рассвет», «снт Ромашка»,
-- «Розы и клематисы», «Любители рыбалки». Сверху:
--   - Николай (01) выбрал «снт Ромашка» — триггер сам сделал его
--     участником геогруппы (specs/029-groups.md, требование 5) — и
--     вступил в «Любителей рыбалки»;
--   - часть старых постов «Живой дачи» выложена в группах. Сергей (09)
--     у Николая не в подписках, но его посты из общих групп приходят в
--     «Подписки» (specs/030-group-posts.md, требование 6).
\i /stories/029-groups/seed.sql

UPDATE users SET place_id = '00000000-0000-4000-8000-000000000292'
WHERE id = (SELECT id FROM dacha_users WHERE n = 1);

INSERT INTO group_members (group_id, user_id, state, created_at)
SELECT g.id, u.id, 'member', now() - interval '2 days'
FROM demo_groups g, dacha_users u
WHERE g.n = 2 AND u.n = 1
ON CONFLICT (group_id, user_id) DO UPDATE SET state = 'member';

-- Пост → группа: «romashka», «rassvet» — геогруппы по пункту, число —
-- группа по интересам из demo_groups. Пост Галины про розы — сразу в
-- двух группах.
INSERT INTO post_groups (post_id, group_id)
SELECT pg_temp.dacha_post(v.post), coalesce(
	(SELECT id FROM groups WHERE place_id = CASE v.grp
		WHEN 'romashka' THEN '00000000-0000-4000-8000-000000000292'
		WHEN 'rassvet' THEN '00000000-0000-4000-8000-000000000291' END),
	(SELECT id FROM demo_groups WHERE n::text = v.grp))
FROM (VALUES
	(11, 'romashka'), (22, 'romashka'), (23, 'romashka'), (25, 'romashka'),
	(51, 'romashka'),
	( 6, 'rassvet'), (20, 'rassvet'), (36, 'rassvet'), (55, 'rassvet'),
	(23, '1'), (38, '1'), (52, '1'),
	(10, '2'), (51, '2')
) AS v(post, grp);
