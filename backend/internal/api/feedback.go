package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
	"github.com/gmu-msk/moya_dacha/backend/internal/feedback"
	"github.com/gmu-msk/moya_dacha/backend/internal/media"
)

// Отзывы разработчику (specs/019-feedback.md, требования 18–19).

const (
	feedbackMaxText  = 4000
	feedbackMaxField = 200
)

// SendFeedback записывает отзыв и в фоне заводит задачу GitHub.
func (s *Server) SendFeedback(ctx context.Context, request gen.SendFeedbackRequestObject) (gen.SendFeedbackResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.SendFeedback401JSONResponse(errUnauthorized), nil
	}
	if request.Body == nil {
		return gen.SendFeedback400JSONResponse(errFeedbackText), nil
	}

	var text, version, device string
	var shot []byte
	for {
		part, err := request.Body.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return gen.SendFeedback400JSONResponse(errEmptyRequest), nil
		}
		switch name := part.FormName(); {
		case name == "screenshot" && part.FileName() != "":
			raw, err := io.ReadAll(io.LimitReader(part, media.PhotoMaxBytes+1))
			if err != nil {
				return gen.SendFeedback400JSONResponse(errEmptyRequest), nil
			}
			if len(raw) > media.PhotoMaxBytes {
				return gen.SendFeedback413JSONResponse(errPhotoTooLarge), nil
			}
			shot = raw
		case part.FileName() != "":
			// Файл под чужим именем — не наше дело.
		case name == "text" || name == "app_version" || name == "device":
			// Текст длиннее 4000 символов — это до 16000 байт в UTF-8.
			raw, err := io.ReadAll(io.LimitReader(part, 4*feedbackMaxText+1))
			if err != nil {
				return gen.SendFeedback400JSONResponse(errEmptyRequest), nil
			}
			switch name {
			case "text":
				if len(raw) > 4*feedbackMaxText {
					return gen.SendFeedback400JSONResponse(errFeedbackText), nil
				}
				text = strings.TrimSpace(string(raw))
			case "app_version":
				version = clip(string(raw), feedbackMaxField)
			case "device":
				device = clip(string(raw), feedbackMaxField)
			}
		}
	}
	if n := utf8.RuneCountInString(text); n == 0 || n > feedbackMaxText {
		return gen.SendFeedback400JSONResponse(errFeedbackText), nil
	}

	var name, nickname string
	if err := s.db.QueryRow(ctx, `SELECT name, nickname FROM users WHERE id = $1`, current.user.Id).
		Scan(&name, &nickname); err != nil {
		return nil, err
	}
	author := "@" + nickname
	if name = strings.TrimSpace(name); name != "" {
		author = name + " (@" + nickname + ")"
	}

	entry, err := s.cfg.Feedback.Add(ctx, feedback.Item{
		Source:     feedback.SourceApp,
		UserID:     current.user.Id,
		Author:     author,
		Text:       text,
		Screenshot: shot,
		AppVersion: version,
		Device:     device,
	})
	if errors.Is(err, media.ErrNotAnImage) {
		return gen.SendFeedback400JSONResponse(errInvalidImage), nil
	}
	if err != nil {
		return nil, err
	}

	// Задача заводится в фоне: человек не ждёт GitHub. Не вышло —
	// повторит сверка раз в 5 минут (требование 7).
	if s.cfg.Feedback.Enabled() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := s.cfg.Feedback.Submit(ctx, entry.ID, nil); err != nil {
				slog.Error("задача GitHub для отзыва не заведена, повторю", "id", entry.ID, "err", err)
			}
		}()
	}
	return gen.SendFeedback201JSONResponse(feedbackOut(entry)), nil
}

// GetMyFeedback — свои отзывы, новые сверху.
func (s *Server) GetMyFeedback(ctx context.Context, _ gen.GetMyFeedbackRequestObject) (gen.GetMyFeedbackResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.GetMyFeedback401JSONResponse(errUnauthorized), nil
	}
	list, err := s.cfg.Feedback.Mine(ctx, current.user.Id)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Feedback, 0, len(list))
	for _, e := range list {
		items = append(items, feedbackOut(e))
	}
	return gen.GetMyFeedback200JSONResponse{Items: items}, nil
}

func feedbackOut(e feedback.Entry) gen.Feedback {
	return gen.Feedback{
		Id:        e.ID,
		Text:      e.Text,
		Status:    gen.FeedbackStatus(e.Status),
		Issue:     e.Issue,
		Build:     e.Build,
		CreatedAt: e.CreatedAt,
	}
}

// clip обрезает пробелы по краям и оставляет не больше max символов.
func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > max {
		s = strings.TrimSpace(string([]rune(s)[:max]))
	}
	return s
}

var errFeedbackText = gen.Error{
	Code:    "invalid_text",
	Message: "Напишите отзыв: от 1 до 4000 символов",
}
