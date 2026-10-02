-- Геогруппы и открытые группы по интересам: specs/029-groups.md, вторая
-- версия (#108). Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Группы по месту, созданные вручную, становятся группами по интересам:
-- место и радиус уходят, название и состав остаются. Все группы
-- открытые (требование 4); колонки join_policy и radius_km остаются для
-- старых сборок, но значение у них одно.
ALTER TABLE groups
    ALTER COLUMN owner_id DROP NOT NULL,
    DROP CONSTRAINT groups_check,
    DROP CONSTRAINT groups_check1,
    DROP CONSTRAINT groups_join_policy_check;

UPDATE groups SET kind = 'interest', place_id = NULL, radius_km = NULL WHERE kind = 'place';
UPDATE groups SET join_policy = 'open';

-- Геогруппа — без хозяина и с местом, одна на пункт (требование 2).
ALTER TABLE groups
    ADD CONSTRAINT groups_join_policy_check CHECK (join_policy = 'open'),
    ADD CONSTRAINT groups_radius_km_null CHECK (radius_km IS NULL),
    ADD CONSTRAINT groups_place_kind CHECK (
        (kind = 'place') = (place_id IS NOT NULL) AND (kind = 'place') = (owner_id IS NULL)),
    ADD CONSTRAINT groups_place_unique UNIQUE (place_id);

-- Заявок больше нет: ждущая заявка становится участием. Приглашение
-- помнит, кто пригласил (требование 23): до сих пор приглашал только
-- хозяин. Пригласивший удалил аккаунт — приглашение уходит (27).
ALTER TABLE group_members
    ADD COLUMN invited_by uuid REFERENCES users (id) ON DELETE CASCADE,
    DROP CONSTRAINT group_members_state_check;

UPDATE group_members SET state = 'member' WHERE state = 'requested';
UPDATE group_members gm SET invited_by = g.owner_id
FROM groups g
WHERE g.id = gm.group_id AND gm.state = 'invited';

ALTER TABLE group_members
    ADD CONSTRAINT group_members_state_check CHECK (state IN ('member', 'invited')),
    ADD CONSTRAINT group_members_invited_by CHECK ((state = 'invited') = (invited_by IS NOT NULL));

-- Пуш о приглашении — от пригласившего (требование 29). Ждущие пуши о
-- заявках сервис отменит сам: заявки больше не ждут ответа.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION push_group_member() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.state = 'invited' AND EXISTS (SELECT 1 FROM push_devices WHERE user_id = NEW.user_id) THEN
        INSERT INTO push_queue (user_id, requester_id, group_id, group_kind)
        VALUES (NEW.user_id, NEW.invited_by, NEW.group_id, 'group_invite');
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd

-- Геогруппа пункта и участие в ней (требования 5–8). Пункт меняется в
-- нескольких местах — ручка профиля, демо-данные, эта миграция, —
-- поэтому это триггер, как события уведомлений (014). Приглашённый в
-- геогруппу, выбрав её пункт, становится участником.
-- +goose StatementBegin
CREATE FUNCTION join_place_group(member uuid, place text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    gid uuid;
BEGIN
    INSERT INTO groups (name, description, kind, join_policy, place_id)
    SELECT left(coalesce(nullif(btrim(p.name), ''), p.id), 60), left(p.area, 500), 'place', 'open', p.id
    FROM places p WHERE p.id = place
    ON CONFLICT (place_id) DO NOTHING;

    SELECT g.id INTO gid FROM groups g WHERE g.place_id = place;
    IF gid IS NULL THEN
        RETURN;
    END IF;

    INSERT INTO group_members (group_id, user_id, state)
    VALUES (gid, member, 'member')
    ON CONFLICT (group_id, user_id) DO UPDATE
        SET state = 'member', invited_by = NULL, created_at = now()
        WHERE group_members.state = 'invited';
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION users_place_group() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM join_place_group(NEW.id, NEW.place_id);
    RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER users_place_group_insert AFTER INSERT ON users
    FOR EACH ROW WHEN (NEW.place_id IS NOT NULL)
    EXECUTE FUNCTION users_place_group();
-- Тот же пункт — ничего (требование 7), снятый — тоже (6).
CREATE TRIGGER users_place_group_update AFTER UPDATE OF place_id ON users
    FOR EACH ROW WHEN (NEW.place_id IS NOT NULL AND NEW.place_id IS DISTINCT FROM OLD.place_id)
    EXECUTE FUNCTION users_place_group();

-- Пункт выбран раньше геогрупп — человек в геогруппе своего пункта
-- (требование 9).
SELECT join_place_group(u.id, u.place_id)
FROM users u
WHERE u.place_id IS NOT NULL
ORDER BY u.created_at;
