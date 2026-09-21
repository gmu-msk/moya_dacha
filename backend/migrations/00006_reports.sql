-- Жалобы: specs/008-reports.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- Одна таблица на оба вида жалоб: владельцу сервиса нужен один список
-- «что разобрать», а не два (ADR-0017). Заполнена ровно одна ссылка —
-- либо на пост, либо на комментарий.
--
-- Состояния у жалобы нет: разбирается она вне сервиса, и отмечать
-- «рассмотрено» некому.
CREATE TABLE reports (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id     uuid REFERENCES posts (id) ON DELETE CASCADE,
    comment_id  uuid REFERENCES comments (id) ON DELETE CASCADE,
    reporter_id uuid NOT NULL REFERENCES users (id),
    reason      text,
    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT reports_one_target
        CHECK ((post_id IS NULL) <> (comment_id IS NULL))
);

-- Вторая жалоба того же человека на то же самое не появляется: сигнал
-- у владельца сервиса уже есть, а первая причина не переписывается —
-- он мог её уже прочитать (specs/008-reports.md, требование 4).
CREATE UNIQUE INDEX reports_post_reporter_idx
    ON reports (post_id, reporter_id) WHERE post_id IS NOT NULL;
CREATE UNIQUE INDEX reports_comment_reporter_idx
    ON reports (comment_id, reporter_id) WHERE comment_id IS NOT NULL;
