-- Посты в группах: specs/030-group-posts.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Связь поста с группой. Уходит вместе с постом (требование 15) и вместе
-- с группой — пост при этом остаётся у автора (требование 14). Выход
-- автора из группы связь не трогает (требование 13).
CREATE TABLE post_groups (
    post_id  uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    group_id uuid NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, group_id)
);

-- Лента группы (требование 9) и посты групп в «Подписках» (требование 6).
CREATE INDEX post_groups_group ON post_groups (group_id, post_id);
