-- Комментарии: specs/006-comments.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Комментарии плоские: ответов на комментарий не существует, поэтому
-- ссылки на родительский комментарий в таблице нет и не будет
-- (CONTEXT.md).
CREATE TABLE comments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id    uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    author_id  uuid NOT NULL REFERENCES users (id),
    text       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Единственный порядок чтения: комментарии одного поста от старого
-- к новому. Идентификатор в индексе — чтобы одинаковое время давало
-- один и тот же порядок, а не разный при каждом чтении.
CREATE INDEX comments_post_id_idx ON comments (post_id, created_at, id);
