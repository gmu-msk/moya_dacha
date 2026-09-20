-- Посты и их медиа: specs/003-posts.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

CREATE TABLE posts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id  uuid NOT NULL REFERENCES users (id),
    caption    text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Лента (004-feed) читает посты в обратном хронологическом порядке,
-- и это её единственный порядок.
CREATE INDEX posts_created_at_idx ON posts (created_at DESC, id DESC);

-- Медиа отдельной таблицей, а не массивом ключей в посте: у медиа есть
-- свои поля, оно живёт до поста, а видео, когда до него дойдёт, добавит
-- свои (ADR-0006).
CREATE TABLE media (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id   uuid NOT NULL REFERENCES users (id),
    -- NULL означает «загружено, но не опубликовано». Прикрепление
    -- одноразовое: поэтому одну фотографию нельзя положить в два поста.
    post_id     uuid REFERENCES posts (id) ON DELETE CASCADE,
    position    int,
    kind        text NOT NULL,
    -- Ключ файла в хранилище, а не ссылка (ADR-0011).
    storage_key text NOT NULL,
    width       int NOT NULL,
    height      int NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),

    UNIQUE (post_id, position),
    -- Либо медиа опубликовано и знает своё место, либо ещё нет.
    CONSTRAINT media_attached_together CHECK (
        (post_id IS NULL AND position IS NULL)
        OR (post_id IS NOT NULL AND position IS NOT NULL)
    )
);

-- Пост читает свои медиа по порядку; уборка неопубликованных ищет их
-- по автору.
CREATE INDEX media_post_id_idx ON media (post_id, position);
CREATE INDEX media_unattached_idx ON media (author_id, created_at) WHERE post_id IS NULL;
