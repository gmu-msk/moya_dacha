-- Геометка поста и расстояние до него: specs/027-post-place.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Координаты центра пункта (требования 10–11): из подсказок DaData, могут
-- быть неизвестны. Наружу не отдаются, нужны только для расстояния.
ALTER TABLE places ADD COLUMN lat double precision, ADD COLUMN lon double precision;

-- Место поста — ссылка на пункт, а не текст: переименование в
-- справочнике видно и у постов (требование 6).
ALTER TABLE posts ADD COLUMN place_id text REFERENCES places (id);
