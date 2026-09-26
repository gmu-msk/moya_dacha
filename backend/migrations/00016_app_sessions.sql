-- Заходы и время в приложении: specs/020-app-sessions.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Сессия — время, пока приложение открыто на экране у вошедшего
-- человека. Начало и конец ставит сервер по своим часам (требование 3);
-- конца нет, пока приложение его не отметило.
CREATE TABLE app_sessions (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at   timestamptz
);

CREATE INDEX app_sessions_started_idx ON app_sessions (started_at);

-- Сколько раз за сессию открывался экран: счётчик накопительный,
-- повторная отметка конца его заменяет (требование 8).
CREATE TABLE app_session_screens (
    session_id uuid NOT NULL REFERENCES app_sessions (id) ON DELETE CASCADE,
    screen     text NOT NULL,
    opens      int  NOT NULL CHECK (opens > 0),
    PRIMARY KEY (session_id, screen)
);
