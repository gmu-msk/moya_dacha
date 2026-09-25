-- Дашборд владельца: specs/016-dashboard.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Измерения машины раз в минуту (требование 13). Хранятся 14 дней,
-- старые стирает сам сервис. requests — сколько запросов обслужено
-- с прошлого измерения.
CREATE TABLE server_samples (
    measured_at        timestamptz      NOT NULL DEFAULT now(),
    cpu_percent        double precision NOT NULL DEFAULT 0,
    memory_used_bytes  bigint           NOT NULL DEFAULT 0,
    memory_total_bytes bigint           NOT NULL DEFAULT 0,
    swap_used_bytes    bigint           NOT NULL DEFAULT 0,
    disk_used_bytes    bigint           NOT NULL DEFAULT 0,
    disk_total_bytes   bigint           NOT NULL DEFAULT 0,
    requests           bigint           NOT NULL DEFAULT 0
);
CREATE INDEX server_samples_measured_at_idx ON server_samples (measured_at DESC);

-- Ошибки сервера — ответы 5xx и паники (требования 14–15). Хранятся
-- 30 дней. Путь — как пришёл, с идентификаторами: по нему видно, на
-- каком посте сломалось.
CREATE TABLE server_errors (
    id      bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at      timestamptz NOT NULL DEFAULT now(),
    method  text        NOT NULL DEFAULT '',
    path    text        NOT NULL DEFAULT '',
    status  int         NOT NULL DEFAULT 500,
    message text        NOT NULL DEFAULT ''
);
CREATE INDEX server_errors_at_idx ON server_errors (at DESC);
