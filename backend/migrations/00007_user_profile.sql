-- Профиль пользователя для соседей: specs/009-user-profile.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Посты одного автора страницами, новые сверху. Без этого индекса каждая
-- страница профиля перебирала бы посты всех авторов по индексу ленты.
CREATE INDEX posts_author_created_at_idx
    ON posts (author_id, created_at DESC, id DESC);
