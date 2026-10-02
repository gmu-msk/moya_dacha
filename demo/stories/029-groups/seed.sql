-- Данные сценария «Группы по интересам и по месту».
--
-- Основа — «Живая дача» ([000-dacha]). Все пятнадцать дачников — из
-- снт «Рассвет», в трёх километрах от него — снт «Ромашка». Сверху пять
-- групп, чтобы у Николая (01) было всё сразу:
--   - «Огурцы в теплице» — его группа по заявке, ждут две заявки;
--   - «Розы и клематисы» — Галина пригласила его в свою группу;
--   - «СНТ Рассвет» — группа его СНТ по заявке, «Рядом с вами»;
--   - «Соседи с Ромашки» — открытая, пункт рядом, радиус 5 км;
--   - «Любители рыбалки» — открытая, по интересам.
\i /stories/000-dacha/seed.sql

INSERT INTO places (id, name, area, lat, lon) VALUES
	('00000000-0000-4000-8000-000000000291', 'снт Рассвет', 'Раменский р-н, Московская обл', 55.5600, 38.2300),
	('00000000-0000-4000-8000-000000000292', 'снт Ромашка', 'Раменский р-н, Московская обл', 55.5800, 38.2650)
ON CONFLICT (id) DO NOTHING;

UPDATE users SET place_id = '00000000-0000-4000-8000-000000000291'
WHERE id IN (SELECT id FROM dacha_users);

CREATE TEMP TABLE demo_groups (n int PRIMARY KEY, id uuid NOT NULL);
INSERT INTO demo_groups VALUES
	(1, '00000000-0000-4000-8000-000000002901'),
	(2, '00000000-0000-4000-8000-000000002902'),
	(3, '00000000-0000-4000-8000-000000002903'),
	(4, '00000000-0000-4000-8000-000000002904'),
	(5, '00000000-0000-4000-8000-000000002905');

INSERT INTO groups (id, owner_id, name, description, kind, join_policy, place_id, radius_km, created_at)
SELECT g.id, u.id, v.name, v.description, v.kind, v.policy, v.place::text, v.radius, now() - make_interval(days => v.ago)
FROM (VALUES
	(1,  1, 'Огурцы в теплице', 'Сорта, подвязка, болезни — всё про тепличные огурцы', 'interest', 'request', NULL, NULL, 20),
	(2,  4, 'Розы и клематисы', 'Обрезка, укрытие на зиму, обмен черенками', 'interest', 'request', NULL, NULL, 40),
	(3, 15, 'СНТ Рассвет', 'Соседи по участкам: вода, дороги, взносы, общие собрания', 'place', 'request', '00000000-0000-4000-8000-000000000291', NULL, 90),
	(4,  7, 'Соседи с Ромашки', 'Ромашка и окрестности: попутки до станции, общий трактор', 'place', 'open', '00000000-0000-4000-8000-000000000292', 5, 60),
	(5,  9, 'Любители рыбалки', 'Где клюёт, на что и когда. Пруд за СНТ и Москва-река', 'interest', 'open', NULL, NULL, 30)
) AS v(n, owner, name, description, kind, policy, place, radius, ago)
JOIN demo_groups g ON g.n = v.n
JOIN dacha_users u ON u.n = v.owner;

-- Хозяин — тоже участник (specs/029-groups.md, «Модель данных»).
INSERT INTO group_members (group_id, user_id, state, created_at)
SELECT g.id, u.id, v.state, now() - make_interval(hours => v.ago)
FROM (VALUES
	(1,  1, 'member', 480), (1, 10, 'member', 300),
	(1,  6, 'requested', 5), (1, 11, 'requested', 2),
	(2,  4, 'member', 960), (2,  8, 'member', 700), (2, 14, 'member', 500),
	(2,  1, 'invited', 3),
	(3, 15, 'member', 2160), (3,  2, 'member', 2000), (3,  4, 'member', 1900),
	(3,  5, 'member', 1500), (3,  7, 'member', 1400), (3, 10, 'member', 1200),
	(3, 12, 'member', 900),  (3, 13, 'member', 600),
	(4,  7, 'member', 1440), (4,  3, 'member', 1000),
	(5,  9, 'member', 720),  (5,  5, 'member', 600), (5,  7, 'member', 400)
) AS v(grp, member, state, ago)
JOIN demo_groups g ON g.n = v.grp
JOIN dacha_users u ON u.n = v.member;
