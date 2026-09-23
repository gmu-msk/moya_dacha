-- Никнейм: specs/010-nicknames.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Никнейм есть у каждого пользователя всегда. Пока человек не выбрал
-- его сам, он временный: значение по умолчанию изменчивое, поэтому база
-- считает его заново для каждой строки — и для тех, что уже есть,
-- и для каждой новой.
ALTER TABLE users ADD COLUMN nickname text NOT NULL
    DEFAULT ('dachnik_' || substr(md5(gen_random_uuid()::text), 1, 10));

-- Пока никнейм не выбран самим человеком, приложение показывает ему
-- экран знакомства.
ALTER TABLE users ADD COLUMN nickname_chosen boolean NOT NULL DEFAULT false;

-- Уникальность без учёта регистра: Valya и valya — один никнейм.
-- Хранится он так, как его ввели.
CREATE UNIQUE INDEX users_nickname_lower_idx ON users (lower(nickname));
