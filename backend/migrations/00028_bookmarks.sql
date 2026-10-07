-- Закладки: specs/032-bookmarks.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Ключ из пары — «не больше одной закладки на пост от человека».
-- Начинается с user_id: главный вопрос здесь — «мои закладки».
-- on delete cascade — закладка живёт вместе с постом и человеком
-- (требование 10).
CREATE TABLE post_bookmarks (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    post_id    uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, post_id)
);

-- Страницы «Сохранённых»: новые закладки сверху (требование 7).
CREATE INDEX post_bookmarks_user_created_idx
    ON post_bookmarks (user_id, created_at DESC, post_id DESC);

-- Число закладок у поста считается по post_id (требование 11).
CREATE INDEX post_bookmarks_post_idx ON post_bookmarks (post_id);
