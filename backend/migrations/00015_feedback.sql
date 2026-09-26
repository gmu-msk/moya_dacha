-- Обратная связь: specs/019-feedback.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Отзыв разработчику из приложения или Telegram. Сначала строка здесь,
-- потом задача GitHub (требование 1): GitHub недоступен — отзыв не теряется.
CREATE TABLE feedback (
    id            bigserial   PRIMARY KEY,
    source        text        NOT NULL CHECK (source IN ('app', 'telegram')),
    user_id       uuid        REFERENCES users (id) ON DELETE SET NULL,
    author        text        NOT NULL,
    -- Куда отвечать автору из Telegram: чат, тема и его сообщение.
    tg_chat_id    bigint,
    tg_thread_id  bigint,
    tg_message_id bigint,
    tg_private    boolean     NOT NULL DEFAULT false,
    text          text        NOT NULL,
    screenshot    text,
    app_version   text,
    device        text,
    status        text        NOT NULL DEFAULT 'sent'
                  CHECK (status IN ('sent', 'accepted', 'approved', 'declined', 'done', 'released')),
    issue         int,
    issue_url     text,
    build         bigint,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX feedback_user_idx ON feedback (user_id, created_at DESC);
CREATE INDEX feedback_status_idx ON feedback (status) WHERE status IN ('sent', 'accepted', 'approved', 'done');

-- Тема «Идеи и баги» (требование 20): роль ideas и номер темы.
ALTER TABLE telegram_chats DROP CONSTRAINT telegram_chats_role_check;
ALTER TABLE telegram_chats ADD CONSTRAINT telegram_chats_role_check
    CHECK (role IN ('owner', 'group', 'ideas'));
ALTER TABLE telegram_chats ADD COLUMN thread_id bigint;
