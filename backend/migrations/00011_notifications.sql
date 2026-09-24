-- Уведомления: specs/014-notifications.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Каждое действие — своя строка; лайки одного поста сливаются при
-- чтении. Отменённое действие удаляет строку, каскады убирают события
-- удалённых постов и комментариев (требование 4).
CREATE TABLE notifications (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- кому
    kind        text        NOT NULL CHECK (kind IN ('follow', 'follow_accepted', 'like', 'comment')),
    actor_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- кто
    post_id     uuid        REFERENCES posts (id) ON DELETE CASCADE,
    comment_id  uuid        REFERENCES comments (id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC, id DESC);
-- Отмена лайка и отписка ищут строку по тому, кто её вызвал.
CREATE INDEX notifications_actor_idx ON notifications (actor_id, kind);

-- Когда человек последний раз открывал раздел (требование 5). У тех,
-- кто уже есть, всё прежнее считается прочитанным.
ALTER TABLE users ADD COLUMN notifications_seen_at timestamptz NOT NULL DEFAULT now();

-- События пишет база, а не хендлеры: лайк, комментарий и подписка
-- появляются в нескольких местах (ручки, открытие профиля, демо-данные),
-- и триггер — единственное место, где событие не забудут записать или
-- убрать. Время события — время самого действия.

-- +goose StatementBegin
CREATE FUNCTION notify_like() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        -- Свой лайк своему посту событий не порождает (требование 3).
        INSERT INTO notifications (user_id, kind, actor_id, post_id, created_at)
        SELECT p.author_id, 'like', NEW.user_id, NEW.post_id, NEW.created_at
        FROM posts p WHERE p.id = NEW.post_id AND p.author_id <> NEW.user_id;
        RETURN NEW;
    END IF;
    DELETE FROM notifications
    WHERE kind = 'like' AND actor_id = OLD.user_id AND post_id = OLD.post_id;
    RETURN OLD;
END $$;
-- +goose StatementEnd

CREATE TRIGGER post_likes_notify AFTER INSERT OR DELETE ON post_likes
    FOR EACH ROW EXECUTE FUNCTION notify_like();

-- Удалённый комментарий уносит событие каскадом по comment_id.
-- +goose StatementBegin
CREATE FUNCTION notify_comment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO notifications (user_id, kind, actor_id, post_id, comment_id, created_at)
    SELECT p.author_id, 'comment', NEW.author_id, NEW.post_id, NEW.id, NEW.created_at
    FROM posts p WHERE p.id = NEW.post_id AND p.author_id <> NEW.author_id;
    RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER comments_notify AFTER INSERT ON comments
    FOR EACH ROW EXECUTE FUNCTION notify_comment();

-- Подписка на открытый профиль — follow хозяину. Принятая заявка —
-- follow_accepted заявителю, хозяину ничего. Отписка и отмена убирают
-- оба события пары.
-- +goose StatementBegin
CREATE FUNCTION notify_follow() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.accepted THEN
            INSERT INTO notifications (user_id, kind, actor_id, created_at)
            VALUES (NEW.followee_id, 'follow', NEW.follower_id, NEW.created_at);
        END IF;
        RETURN NEW;
    END IF;
    IF TG_OP = 'UPDATE' THEN
        IF NEW.accepted AND NOT OLD.accepted THEN
            INSERT INTO notifications (user_id, kind, actor_id, created_at)
            VALUES (NEW.follower_id, 'follow_accepted', NEW.followee_id, NEW.created_at);
        END IF;
        RETURN NEW;
    END IF;
    DELETE FROM notifications
    WHERE (kind = 'follow' AND user_id = OLD.followee_id AND actor_id = OLD.follower_id)
       OR (kind = 'follow_accepted' AND user_id = OLD.follower_id AND actor_id = OLD.followee_id);
    RETURN OLD;
END $$;
-- +goose StatementEnd

CREATE TRIGGER follows_notify AFTER INSERT OR UPDATE OF accepted OR DELETE ON follows
    FOR EACH ROW EXECUTE FUNCTION notify_follow();

-- Что уже было до этой миграции — тоже события, но прочитанные.
INSERT INTO notifications (user_id, kind, actor_id, created_at)
SELECT f.followee_id, 'follow', f.follower_id, f.created_at FROM follows f WHERE f.accepted;

INSERT INTO notifications (user_id, kind, actor_id, post_id, created_at)
SELECT p.author_id, 'like', l.user_id, l.post_id, l.created_at
FROM post_likes l JOIN posts p ON p.id = l.post_id WHERE p.author_id <> l.user_id;

INSERT INTO notifications (user_id, kind, actor_id, post_id, comment_id, created_at)
SELECT p.author_id, 'comment', c.author_id, c.post_id, c.id, c.created_at
FROM comments c JOIN posts p ON p.id = c.post_id WHERE p.author_id <> c.author_id;
