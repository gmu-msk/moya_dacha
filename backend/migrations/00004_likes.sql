-- Лайки: specs/005-likes.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Первичный ключ из пары — это и есть «не больше одного лайка на пост
-- от человека»: второй такой строки база не примет, и проверять это
-- в коде не нужно.
CREATE TABLE post_likes (
    post_id    uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id)
);

-- Отдельного индекса по user_id нет намеренно: «все посты, которые
-- я лайкнул» в MVP никто не спрашивает, а пост свои лайки считает
-- по первичному ключу.
