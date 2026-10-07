// Пост-вопрос: specs/033-question-posts.md.
package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// questionColumns — вопрос ли пост p, решён ли он и какой комментарий —
// решение, в порядке questionScan.targets.
const questionColumns = `p.question, p.solved, p.answer_comment_id`

// questionScan принимает questionColumns. Поля в Post — указатели (их
// нет на сервере с main), поэтому читаем в значения и кладём адреса.
type questionScan struct {
	question, solved bool
	answer           *string
}

func (q *questionScan) targets() []any {
	return []any{&q.question, &q.solved, &q.answer}
}

func (q *questionScan) apply(post *gen.Post) {
	post.Question, post.Solved, post.AnswerCommentId = &q.question, &q.solved, q.answer
}

// errQuestionNoPost, errQuestionNotYours, errQuestionNotQuestion —
// общие для трёх ручек ответы проверки поста (требование 10).
var (
	errQuestionNoPost      = errors.New("поста нет")
	errQuestionNotYours    = errors.New("пост чужой")
	errQuestionNotQuestion = errors.New("пост не вопрос")
)

// ownQuestion проверяет по порядку: виден ли пост, свой ли он, вопрос
// ли он (требование 10).
func (s *Server) ownQuestion(ctx context.Context, postID, viewerID string) error {
	if !isUUID(postID) {
		return errQuestionNoPost
	}
	var (
		author   string
		question bool
	)
	err := s.db.QueryRow(ctx,
		`SELECT p.author_id, p.question FROM posts p WHERE p.id = $1 AND `+postVisibleTo("$2"),
		postID, viewerID,
	).Scan(&author, &question)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return errQuestionNoPost
	case err != nil:
		return err
	case author != viewerID:
		return errQuestionNotYours
	case !question:
		return errQuestionNotQuestion
	}
	return nil
}

// questionError переводит ошибку ownQuestion в ответ: 404, 403 или 400.
// ok=false — ошибка не из этих трёх, её нужно вернуть как есть.
func questionError(err error) (status int, body gen.Error, ok bool) {
	switch {
	case errors.Is(err, errQuestionNoPost):
		return 404, errPostNotFound, true
	case errors.Is(err, errQuestionNotYours):
		return 403, errNotYourQuestion, true
	case errors.Is(err, errQuestionNotQuestion):
		return 400, errNotAQuestion, true
	}
	return 0, gen.Error{}, false
}

// SetQuestionSolved отмечает свой вопрос решённым или нерешённым.
// «Не решён» снимает и отметку решения (требование 5).
func (s *Server) SetQuestionSolved(ctx context.Context, request gen.SetQuestionSolvedRequestObject) (gen.SetQuestionSolvedResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.SetQuestionSolved401JSONResponse(errUnauthorized), nil
	}
	if err := s.ownQuestion(ctx, request.PostId, current.user.Id); err != nil {
		status, body, known := questionError(err)
		switch {
		case !known:
			return nil, err
		case status == 404:
			return gen.SetQuestionSolved404JSONResponse(body), nil
		case status == 403:
			return gen.SetQuestionSolved403JSONResponse(body), nil
		default:
			return gen.SetQuestionSolved400JSONResponse(body), nil
		}
	}
	// Без поля — ошибка, а не «Не решён»: иначе ошибка клиента молча
	// сняла бы отметку решения.
	if request.Body == nil || request.Body.Solved == nil {
		return gen.SetQuestionSolved400JSONResponse(errEmptyRequest), nil
	}

	solved := *request.Body.Solved
	if _, err := s.db.Exec(ctx, `
		UPDATE posts SET solved = $2,
			answer_comment_id = CASE WHEN $2 THEN answer_comment_id END
		WHERE id = $1`, request.PostId, solved,
	); err != nil {
		return nil, err
	}

	post, err := s.post(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	return gen.SetQuestionSolved200JSONResponse(post), nil
}

// MarkAnswer отмечает комментарий решением; прежняя отметка снимается
// сама — ссылка у поста одна (требования 6 и 8).
func (s *Server) MarkAnswer(ctx context.Context, request gen.MarkAnswerRequestObject) (gen.MarkAnswerResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.MarkAnswer401JSONResponse(errUnauthorized), nil
	}
	if err := s.ownQuestion(ctx, request.PostId, current.user.Id); err != nil {
		status, body, known := questionError(err)
		switch {
		case !known:
			return nil, err
		case status == 404:
			return gen.MarkAnswer404JSONResponse(body), nil
		case status == 403:
			return gen.MarkAnswer403JSONResponse(body), nil
		default:
			return gen.MarkAnswer400JSONResponse(body), nil
		}
	}
	if request.Body == nil || request.Body.CommentId == "" {
		return gen.MarkAnswer400JSONResponse(errEmptyRequest), nil
	}
	commentID := request.Body.CommentId
	if !isUUID(commentID) {
		return gen.MarkAnswer404JSONResponse(errCommentNotFound), nil
	}

	// Комментарий — под этим постом и виден автору: скрытый блокировкой
	// для него не существует (требование 8).
	tag, err := s.db.Exec(ctx, `
		UPDATE posts p SET answer_comment_id = c.id, solved = true
		FROM comments c
		WHERE p.id = $1 AND c.id = $2 AND c.post_id = p.id
		  AND NOT `+blockedBetween("$3::uuid", "c.author_id"),
		request.PostId, commentID, current.user.Id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return gen.MarkAnswer404JSONResponse(errCommentNotFound), nil
	}

	post, err := s.post(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	return gen.MarkAnswer200JSONResponse(post), nil
}

// UnmarkAnswer снимает отметку решения и возвращает вопрос в «Не решён»
// (требование 7). Отметки не было — не ошибка.
func (s *Server) UnmarkAnswer(ctx context.Context, request gen.UnmarkAnswerRequestObject) (gen.UnmarkAnswerResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.UnmarkAnswer401JSONResponse(errUnauthorized), nil
	}
	if err := s.ownQuestion(ctx, request.PostId, current.user.Id); err != nil {
		status, body, known := questionError(err)
		switch {
		case !known:
			return nil, err
		case status == 404:
			return gen.UnmarkAnswer404JSONResponse(body), nil
		case status == 403:
			return gen.UnmarkAnswer403JSONResponse(body), nil
		default:
			return gen.UnmarkAnswer400JSONResponse(body), nil
		}
	}

	if _, err := s.db.Exec(ctx,
		`UPDATE posts SET answer_comment_id = NULL, solved = false WHERE id = $1`,
		request.PostId,
	); err != nil {
		return nil, err
	}

	post, err := s.post(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	return gen.UnmarkAnswer200JSONResponse(post), nil
}

// Ошибки вопроса (specs/033-question-posts.md, «Ошибки»).
var (
	errNotYourQuestion = gen.Error{
		Code:    "not_your_post",
		Message: "Это чужой вопрос. Отметить решение может только автор",
	}
	errNotAQuestion = gen.Error{
		Code:    "not_a_question",
		Message: "Это не вопрос: решения у такого поста нет",
	}
)
