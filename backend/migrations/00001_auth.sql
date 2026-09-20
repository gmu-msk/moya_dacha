-- Вход по номеру телефона: specs/001-auth.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Пользователь опознаётся номером телефона; имя и аватар появятся
-- отдельной миграцией вместе с 002-profile.
CREATE TABLE users (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    phone      text        NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Живой код подтверждения — не более одного на номер: новый запрос
-- затирает предыдущий. Строка живёт на номер, а не на пользователя,
-- потому что код запрашивают и те, кого в users ещё нет.
-- В открытом виде код не хранится, в базе лежит его sha256.
CREATE TABLE auth_codes (
    phone         text        PRIMARY KEY,
    code_hash     text        NOT NULL,
    attempts_left int         NOT NULL,
    expires_at    timestamptz NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- Сессия на устройство: выход на одном устройстве не трогает остальные.
-- Сам токен есть только у клиента, здесь лежит его sha256.
CREATE TABLE sessions (
    token_hash text        PRIMARY KEY,
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
