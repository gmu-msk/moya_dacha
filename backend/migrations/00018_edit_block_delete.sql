-- Правка, блокировка и удаление аккаунта: specs/022-edit-block-delete.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Когда подпись или комментарий последний раз меняли; NULL — не меняли.
ALTER TABLE posts ADD COLUMN edited_at timestamptz;
ALTER TABLE comments ADD COLUMN edited_at timestamptz;

-- Блокировка — односторонняя запись, а действует в обе стороны
-- (требования 12–14). Уходит вместе с любым из двоих.
CREATE TABLE blocks (
    blocker_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    blocked_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);

CREATE INDEX blocks_blocked_idx ON blocks (blocked_id);

-- Пользователь удаляется одной строкой (требование 21, ADR-0025): всё,
-- что он выложил, уходит каскадом.
ALTER TABLE posts DROP CONSTRAINT posts_author_id_fkey,
    ADD CONSTRAINT posts_author_id_fkey
    FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE CASCADE;
ALTER TABLE media DROP CONSTRAINT media_author_id_fkey,
    ADD CONSTRAINT media_author_id_fkey
    FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE CASCADE;
ALTER TABLE comments DROP CONSTRAINT comments_author_id_fkey,
    ADD CONSTRAINT comments_author_id_fkey
    FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE CASCADE;

-- Жалоба остаётся без автора (требование 23): она про чужое содержимое.
ALTER TABLE reports ALTER COLUMN reporter_id DROP NOT NULL,
    DROP CONSTRAINT reports_reporter_id_fkey,
    ADD CONSTRAINT reports_reporter_id_fkey
    FOREIGN KEY (reporter_id) REFERENCES users (id) ON DELETE SET NULL;

-- Комментарии автора — для каскада при удалении аккаунта.
CREATE INDEX comments_author_idx ON comments (author_id);
