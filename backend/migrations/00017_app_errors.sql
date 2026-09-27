-- Отчёты об ошибках приложения: specs/021-app-errors.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Группа — одна и та же ошибка: отчёты с одинаковым отпечатком
-- (требование 5). notified_at ставит бот, когда написал о ней владельцу.
CREATE TABLE app_error_groups (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint text        NOT NULL UNIQUE,
    first_at    timestamptz NOT NULL DEFAULT now(),
    last_at     timestamptz NOT NULL DEFAULT now(),
    notified_at timestamptz
);

CREATE INDEX app_error_groups_last_idx ON app_error_groups (last_at);

-- Отчёт. Человек удалён — отчёт остаётся как отчёт без входа.
CREATE TABLE app_errors (
    id       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id uuid        NOT NULL REFERENCES app_error_groups (id) ON DELETE CASCADE,
    at       timestamptz NOT NULL DEFAULT now(),
    user_id  uuid        REFERENCES users (id) ON DELETE SET NULL,
    error    text        NOT NULL,
    stack    text        NOT NULL DEFAULT '',
    version  text        NOT NULL DEFAULT '',
    build    bigint      NOT NULL DEFAULT 0,
    screen   text        NOT NULL DEFAULT '',
    os       text        NOT NULL DEFAULT ''
);

CREATE INDEX app_errors_group_idx ON app_errors (group_id, at);
CREATE INDEX app_errors_at_idx ON app_errors (at);
CREATE INDEX app_errors_user_idx ON app_errors (user_id, at);
