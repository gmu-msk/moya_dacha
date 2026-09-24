-- Видимость постов: specs/013-post-visibility.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- all — все (у закрытого профиля — подписчики), friends — взаимные
-- подписки, me — только автор. Прежние посты были видны всем.
ALTER TABLE posts ADD COLUMN visibility text NOT NULL DEFAULT 'all'
    CHECK (visibility IN ('all', 'friends', 'me'));
