// Жалобы на пост и на комментарий: specs/008-reports.md.
package api

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// MaxReasonLength — сколько символов помещается в причину жалобы. Та же
// мера, что у комментария и подписи: человеку не нужно помнить три
// предела (specs/008-reports.md, требование 2).
const MaxReasonLength = MaxCommentLength

// ReportPost принимает жалобу на чужой пост. Пост при этом не меняется
// и никуда не девается: жалоба — сигнал владельцу сервиса, а не
// действие над постом (ADR-0017).
func (s *Server) ReportPost(ctx context.Context, request gen.ReportPostRequestObject) (gen.ReportPostResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.ReportPost401JSONResponse(errUnauthorized), nil
	}

	// Порядок тот же, что при удалении: сначала есть ли пост, потом чей
	// он (specs/008-reports.md, требование 7).
	author, err := s.postAuthor(ctx, request.PostId, current.user.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.ReportPost404JSONResponse(errPostNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if author == current.user.Id {
		return gen.ReportPost403JSONResponse(errOwnPost), nil
	}

	reason, ok := reportReason(request.Body)
	if !ok {
		return gen.ReportPost400JSONResponse(errInvalidReason), nil
	}

	if _, err := s.db.Exec(ctx, `
		INSERT INTO reports (post_id, reporter_id, reason)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`,
		request.PostId, current.user.Id, reason,
	); err != nil {
		return nil, err
	}
	return gen.ReportPost202Response{}, nil
}

// ReportComment принимает жалобу на чужой комментарий. Комментарий
// остаётся на месте, и число комментариев у поста не меняется.
func (s *Server) ReportComment(ctx context.Context, request gen.ReportCommentRequestObject) (gen.ReportCommentResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.ReportComment401JSONResponse(errUnauthorized), nil
	}

	found, err := s.postExists(ctx, request.PostId, current.user.Id)
	if err != nil {
		return nil, err
	}
	if !found {
		return gen.ReportComment404JSONResponse(errPostNotFound), nil
	}

	// Пара «пост и комментарий» должна сойтись: существующий
	// комментарий под другим постом — это «такого комментария нет»
	// (specs/008-reports.md, требование 8).
	author, err := s.commentAuthor(ctx, request.PostId, request.CommentId)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.ReportComment404JSONResponse(errCommentNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if author == current.user.Id {
		return gen.ReportComment403JSONResponse(errOwnComment), nil
	}

	reason, ok := reportReason(request.Body)
	if !ok {
		return gen.ReportComment400JSONResponse(errInvalidReason), nil
	}

	if _, err := s.db.Exec(ctx, `
		INSERT INTO reports (comment_id, reporter_id, reason)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`,
		request.CommentId, current.user.Id, reason,
	); err != nil {
		return nil, err
	}
	return gen.ReportComment202Response{}, nil
}

// reportReason приводит присланную причину к тому, что ляжет в базу.
// Второе значение — прошла ли она по длине.
//
// Тела может не быть вовсе, причины в теле может не быть, и причина
// может оказаться из одних пробелов: всё это — жалоба без причины, а не
// ошибка (specs/008-reports.md, требование 2).
func reportReason(body *gen.ReportDraft) (*string, bool) {
	if body == nil || body.Reason == nil {
		return nil, true
	}

	reason := strings.TrimSpace(*body.Reason)
	if reason == "" {
		return nil, true
	}
	if utf8.RuneCountInString(reason) > MaxReasonLength {
		return nil, false
	}
	return &reason, true
}

// Ошибки жалоб (specs/008-reports.md, «Ошибки»).
var (
	errInvalidReason = gen.Error{
		Code:    "invalid_reason",
		Message: "Причина длиннее 1000 символов",
	}
	errOwnPost = gen.Error{
		Code:    "own_post",
		Message: "Это ваш пост. На свой не жалуются — его удаляют",
	}
	errOwnComment = gen.Error{
		Code:    "own_comment",
		Message: "Это ваш комментарий. На свой не жалуются — его удаляют",
	}
)
