-- Профиль пользователя: specs/002-profile.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Имя пустое, пока пользователь не знакомился: приложение показывает
-- ему экран знакомства. Имя неуникально — см. CONTEXT.md.
ALTER TABLE users ADD COLUMN name text NOT NULL DEFAULT '';

ALTER TABLE users ADD COLUMN about text NOT NULL DEFAULT '';

-- Ключ файла в хранилище (avatars/<случайное имя>.jpg), а не ссылка:
-- ссылку сервис строит из ключа сам, поэтому смена хранилища
-- не требует переписывать данные.
ALTER TABLE users ADD COLUMN avatar_key text;
