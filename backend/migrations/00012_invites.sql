-- Вход по коду приглашения: specs/015-invites.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Не более одного приглашения на номер: новое заменяет прежнее
-- (требование 4). Живёт на номер, а не на пользователя — приглашают
-- и тех, кого в users ещё нет. Сам код не хранится, только sha256.
CREATE TABLE invites (
    phone         text        PRIMARY KEY,
    code_hash     text        NOT NULL,
    attempts_left int         NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
