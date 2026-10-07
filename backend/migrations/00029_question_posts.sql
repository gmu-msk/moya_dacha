-- Пост-вопрос: specs/033-question-posts.md.
-- Только вперёд, down-миграции не пишутся (ADR-0005).

-- +goose Up

-- question задаётся при публикации и потом не меняется (требование 1).
-- Отметка решения — ссылка у поста, а не флаг у комментария: решение
-- у вопроса одно (ADR-0030). on delete set null — удалённый комментарий
-- отметку снимает (требование 11).
ALTER TABLE posts
    ADD COLUMN question          boolean NOT NULL DEFAULT false,
    ADD COLUMN solved            boolean NOT NULL DEFAULT false,
    ADD COLUMN answer_comment_id uuid REFERENCES comments (id) ON DELETE SET NULL;

-- Решение пропало — вопрос снова открыт: ручкой снятия отметки или
-- удалением комментария любым путём (требования 7 и 11). Одно правило
-- в базе вместо памяти о нём в каждом обработчике удаления.
-- +goose StatementBegin
CREATE FUNCTION posts_answer_cleared() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.answer_comment_id IS NOT NULL AND NEW.answer_comment_id IS NULL THEN
        NEW.solved := false;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER posts_answer_cleared
    BEFORE UPDATE OF answer_comment_id ON posts
    FOR EACH ROW EXECUTE FUNCTION posts_answer_cleared();
