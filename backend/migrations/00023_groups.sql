-- Группы по интересам и по месту: specs/029-groups.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Хозяин удалил аккаунт — его группы уходят со всем составом. Место —
-- пункт ФИАС, как в профиле и у поста; у группы по интересам места и
-- радиуса нет (требования 3–4).
CREATE TABLE groups (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name        text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    description text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    kind        text        NOT NULL CHECK (kind IN ('interest', 'place')),
    join_policy text        NOT NULL CHECK (join_policy IN ('open', 'request', 'invite')),
    place_id    text        REFERENCES places (id),
    radius_km   integer     CHECK (radius_km BETWEEN 1 AND 100),
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind = 'place') = (place_id IS NOT NULL)),
    CHECK (kind = 'place' OR radius_km IS NULL)
);
CREATE INDEX groups_owner_idx ON groups (owner_id);

-- Одна строка на человека в группе: заявка при принятии и приглашение
-- при согласии становятся участием той же строкой, как заявка на
-- подписку (012). Хозяин — тоже строка member: так «Мои группы» и число
-- участников считаются одним запросом.
CREATE TABLE group_members (
    group_id   uuid        NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    state      text        NOT NULL CHECK (state IN ('member', 'requested', 'invited')),
    -- Когда вступил, попросился или приглашён.
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);
-- «Мои группы» и приглашения смотрящему.
CREATE INDEX group_members_user_idx ON group_members (user_id, state);

-- Пуш о заявке хозяину и о приглашении приглашённому (требование 28).
-- Строка ссылается на группу: удалённая группа уносит её каскадом, а
-- отвеченную заявку сервис проверяет перед отправкой, как заявку на
-- подписку (024). requester_id — кто просится или кто пригласил.
ALTER TABLE push_queue
    ADD COLUMN group_id uuid REFERENCES groups (id) ON DELETE CASCADE,
    ADD COLUMN group_kind text CHECK (group_kind IN ('group_request', 'group_invite')),
    ADD CHECK ((group_id IS NULL) = (group_kind IS NULL)),
    ADD CHECK (group_id IS NULL OR requester_id IS NOT NULL);

-- +goose StatementBegin
CREATE FUNCTION push_group_member() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    owner uuid;
BEGIN
    SELECT g.owner_id INTO owner FROM groups g WHERE g.id = NEW.group_id;
    IF NEW.state = 'requested' AND EXISTS (SELECT 1 FROM push_devices WHERE user_id = owner) THEN
        INSERT INTO push_queue (user_id, requester_id, group_id, group_kind)
        VALUES (owner, NEW.user_id, NEW.group_id, 'group_request');
    ELSIF NEW.state = 'invited' AND EXISTS (SELECT 1 FROM push_devices WHERE user_id = NEW.user_id) THEN
        INSERT INTO push_queue (user_id, requester_id, group_id, group_kind)
        VALUES (NEW.user_id, owner, NEW.group_id, 'group_invite');
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER group_members_push AFTER INSERT ON group_members
    FOR EACH ROW EXECUTE FUNCTION push_group_member();
