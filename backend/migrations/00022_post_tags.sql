-- Тэги у поста: specs/028-post-tags.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Тэг — нормализованная строка (требование 2), отдельной таблицы тэгов
-- нет: популярность считается прямо по этой (требование 16). Тэги уходят
-- вместе с постом (требование 11).
CREATE TABLE post_tags (
    post_id  uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    tag      text NOT NULL,
    position integer NOT NULL,
    PRIMARY KEY (post_id, tag)
);

-- Посты с тэгом (требование 12) и популярность тэгов.
CREATE INDEX post_tags_tag ON post_tags (tag, post_id);
