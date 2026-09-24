-- Подписки и закрытые профили: specs/012-follows.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Новый профиль открыт (требование 6).
ALTER TABLE users ADD COLUMN closed boolean NOT NULL DEFAULT false;

-- Одна строка на пару: заявка при принятии становится подпиской той же
-- строкой, отказ и отписка строку удаляют.
CREATE TABLE follows (
    follower_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    followee_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- false — заявка к закрытому профилю, true — действующая подписка.
    accepted    boolean     NOT NULL,
    -- Для заявки — когда подана, для подписки — когда принята: по этому
    -- времени новые связи идут в списках сверху.
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, followee_id),
    CHECK (follower_id <> followee_id)
);

-- Подписчики и заявки к человеку. Подписки человека читаются по
-- первичному ключу, он начинается с follower_id.
CREATE INDEX follows_followee_idx ON follows (followee_id, accepted, created_at DESC);
