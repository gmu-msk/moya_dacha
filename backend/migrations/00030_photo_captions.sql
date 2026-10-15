-- Подпись к каждому фото поста: specs/034-photo-captions.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- NULL — подписи нет; пустую строку сервер не пишет (требование 1).
-- Колонка у фото, а не таблица и не комментарий (ADR-0031).
ALTER TABLE media
    ADD COLUMN caption text CHECK (caption IS NULL OR caption <> '');
