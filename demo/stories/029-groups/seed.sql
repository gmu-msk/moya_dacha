-- Данные сценария «Геогруппы и группы по интересам».
--
-- Основа — «Живая дача» ([000-dacha]). Два соседних СНТ: «Рассвет»
-- (председатель и его соседи) и «Ромашка» в трёх километрах (Галина,
-- Михалыч, Сергей). Геогруппы появляются сами — их делает триггер базы,
-- когда людям ставится пункт (specs/029-groups.md, требование 5).
-- Николай (01) пункт ещё не выбрал. Сверху две группы по интересам:
--   - «Розы и клематисы» Галины — Ирина из неё пригласила Николая;
--   - «Любители рыбалки» Сергея.
\i /stories/000-dacha/seed.sql

INSERT INTO places (id, name, area, lat, lon) VALUES
	('00000000-0000-4000-8000-000000000291', 'снт Рассвет', 'Раменский р-н, Московская обл', 55.5600, 38.2300),
	('00000000-0000-4000-8000-000000000292', 'снт Ромашка', 'Раменский р-н, Московская обл', 55.5800, 38.2650)
ON CONFLICT (id) DO NOTHING;

UPDATE users u SET place_id = v.place
FROM (VALUES
	( 2, '00000000-0000-4000-8000-000000000291'), ( 5, '00000000-0000-4000-8000-000000000291'),
	(10, '00000000-0000-4000-8000-000000000291'), (12, '00000000-0000-4000-8000-000000000291'),
	(13, '00000000-0000-4000-8000-000000000291'), (15, '00000000-0000-4000-8000-000000000291'),
	( 4, '00000000-0000-4000-8000-000000000292'), ( 7, '00000000-0000-4000-8000-000000000292'),
	( 9, '00000000-0000-4000-8000-000000000292')
) AS v(n, place)
JOIN dacha_users d ON d.n = v.n
WHERE u.id = d.id;

CREATE TEMP TABLE demo_groups (n int PRIMARY KEY, id uuid NOT NULL);
INSERT INTO demo_groups VALUES
	(1, '00000000-0000-4000-8000-000000002901'),
	(2, '00000000-0000-4000-8000-000000002902');

INSERT INTO groups (id, owner_id, name, description, kind, join_policy, created_at)
SELECT g.id, u.id, v.name, v.description, 'interest', 'open', now() - make_interval(days => v.ago)
FROM (VALUES
	(1, 4, 'Розы и клематисы', 'Обрезка, укрытие на зиму, обмен черенками', 40),
	(2, 9, 'Любители рыбалки', 'Где клюёт, на что и когда. Пруд за СНТ и Москва-река', 30)
) AS v(n, owner, name, description, ago)
JOIN demo_groups g ON g.n = v.n
JOIN dacha_users u ON u.n = v.owner;

-- Хозяин — тоже участник (specs/029-groups.md, «Модель данных»).
-- Приглашение Николаю — от Ирины, не от хозяйки: приглашает любой
-- участник (требование 23).
INSERT INTO group_members (group_id, user_id, state, invited_by, created_at)
SELECT g.id, u.id, v.state, b.id, now() - make_interval(hours => v.ago)
FROM (VALUES
	(1,  4, 'member', NULL::int, 960), (1,  8, 'member', NULL, 700), (1, 14, 'member', NULL, 500),
	(1,  1, 'invited', 8, 3),
	(2,  9, 'member', NULL, 720), (2,  5, 'member', NULL, 600), (2,  7, 'member', NULL, 400)
) AS v(grp, member, state, inviter, ago)
JOIN demo_groups g ON g.n = v.grp
JOIN dacha_users u ON u.n = v.member
LEFT JOIN dacha_users b ON b.n = v.inviter;
