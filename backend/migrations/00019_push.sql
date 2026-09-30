-- Push-уведомления: specs/024-push.md, ADR-0027.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Токен FCM телефона. Одна сессия — один токен (требование 7): выход
-- и удаление аккаунта уносят его каскадом.
CREATE TABLE push_devices (
    token        text        PRIMARY KEY,
    session_hash text        NOT NULL UNIQUE REFERENCES sessions (token_hash) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX push_devices_user_idx ON push_devices (user_id);

-- Очередь пушей. Отменённое событие уносит свою строку каскадом по
-- notification_id (требование 9); отозванную заявку сервис проверяет
-- перед отправкой.
CREATE TABLE push_queue (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- кому
    notification_id uuid        REFERENCES notifications (id) ON DELETE CASCADE,
    requester_id    uuid        REFERENCES users (id) ON DELETE CASCADE, -- у follow_request
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK ((notification_id IS NULL) <> (requester_id IS NULL))
);
CREATE INDEX push_queue_created_idx ON push_queue (created_at);

-- Строку кладёт база, как и сами события (014): у получателя без
-- телефона очередь не растёт. Время — время постановки, а не события:
-- задержка считается от него.

-- +goose StatementBegin
CREATE FUNCTION push_notification() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM push_devices WHERE user_id = NEW.user_id) THEN
        INSERT INTO push_queue (user_id, notification_id) VALUES (NEW.user_id, NEW.id);
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER notifications_push AFTER INSERT ON notifications
    FOR EACH ROW EXECUTE FUNCTION push_notification();

-- Заявка к закрытому профилю — не событие раздела, но пуш о ней есть
-- (требование 8).
-- +goose StatementBegin
CREATE FUNCTION push_follow_request() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT NEW.accepted AND EXISTS (SELECT 1 FROM push_devices WHERE user_id = NEW.followee_id) THEN
        INSERT INTO push_queue (user_id, requester_id) VALUES (NEW.followee_id, NEW.follower_id);
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER follows_push AFTER INSERT ON follows
    FOR EACH ROW EXECUTE FUNCTION push_follow_request();
