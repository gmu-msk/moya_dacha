-- Населённый пункт в профиле: specs/025-places.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Пункты, которые сервер отдавал в подсказках (требование 5). Ключ —
-- идентификатор ФИАС: два человека из одного СНТ попадают в одну запись.
CREATE TABLE places (
    id         text        PRIMARY KEY,
    name       text        NOT NULL,
    area       text        NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Пункт у человека один (требование 10). Удаление аккаунта уносит
-- привязку вместе со строкой users, запись в places остаётся (13).
ALTER TABLE users ADD COLUMN place_id text REFERENCES places (id);
CREATE INDEX users_place_idx ON users (place_id) WHERE place_id IS NOT NULL;
