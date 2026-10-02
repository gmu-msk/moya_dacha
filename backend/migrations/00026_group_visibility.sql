-- «Кто увидит» — только участники группы: specs/031-group-visibility.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Видимость 'group' — пост видят автор и участники visibility_group_id
-- (требование 5). Удалили группу — ссылка обнуляется, и такой пост видит
-- только автор, а в ответе он «только мне» (требование 9).
ALTER TABLE posts
    ADD COLUMN visibility_group_id uuid REFERENCES groups (id) ON DELETE SET NULL,
    DROP CONSTRAINT posts_visibility_check,
    ADD CONSTRAINT posts_visibility_check
        CHECK (visibility IN ('all', 'friends', 'me', 'group')),
    ADD CONSTRAINT posts_visibility_group
        CHECK (visibility = 'group' OR visibility_group_id IS NULL);
