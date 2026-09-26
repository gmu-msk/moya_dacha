-- Telegram-бот: specs/018-telegram-bot.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Куда бот пишет (требование 4): личка владельца и группа тестировщиков,
-- по одной строке на роль. Новая привязка роли заменяет старую.
CREATE TABLE telegram_chats (
    role     text        PRIMARY KEY CHECK (role IN ('owner', 'group')),
    chat_id  bigint      NOT NULL,
    bound_at timestamptz NOT NULL DEFAULT now()
);
