// Отчёты об ошибках приложения: specs/021-app-errors.md.
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// reportsPerHour — сколько отчётов в час принимается от человека и от
// всех без входа вместе (требование 4).
const reportsPerHour = 100

// ReportAppError записывает отчёт и склеивает его с прошлыми отчётами
// той же ошибки (требования 1–6). Токен не обязателен: без него отчёт —
// без входа, и 401 ручка не отвечает (требование 3).
func (s *Server) ReportAppError(ctx context.Context, request gen.ReportAppErrorRequestObject) (gen.ReportAppErrorResponseObject, error) {
	body := request.Body
	if body == nil {
		return gen.ReportAppError400JSONResponse(errInvalidReport), nil
	}
	report := appErrorReport{
		error:   body.Error,
		stack:   deref(body.Stack),
		version: deref(body.Version),
		screen:  deref(body.Screen),
		os:      deref(body.Os),
	}
	if body.Build != nil {
		report.build = *body.Build
	}
	if !report.valid() {
		return gen.ReportAppError400JSONResponse(errInvalidReport), nil
	}

	var userID *string
	if current, ok := sessionFrom(ctx); ok && current.user.Id != "" {
		id := current.user.Id
		userID = &id
	}

	limited := false
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var recent int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM app_errors
			WHERE at > now() - interval '1 hour'
			  AND user_id IS NOT DISTINCT FROM $1`, userID).Scan(&recent); err != nil {
			return err
		}
		if recent >= reportsPerHour {
			limited = true
			return nil
		}

		var group string
		if err := tx.QueryRow(ctx, `
			INSERT INTO app_error_groups (fingerprint) VALUES ($1)
			ON CONFLICT (fingerprint) DO UPDATE SET last_at = now()
			RETURNING id::text`, fingerprint(report.error, report.stack)).Scan(&group); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO app_errors (group_id, user_id, error, stack, version, build, screen, os)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			group, userID, report.error, report.stack, report.version, report.build, report.screen, report.os)
		return err
	})
	if err != nil {
		return nil, err
	}
	if limited {
		return gen.ReportAppError429JSONResponse(errTooManyReports), nil
	}
	return gen.ReportAppError204Response{}, nil
}

type appErrorReport struct {
	error, stack, version, screen, os string
	build                             int64
}

// valid — требование 1.
func (r appErrorReport) valid() bool {
	n := utf8.RuneCountInString(r.error)
	return n >= 1 && n <= 2000 &&
		utf8.RuneCountInString(r.stack) <= 20000 &&
		utf8.RuneCountInString(r.version) <= 32 &&
		utf8.RuneCountInString(r.os) <= 100 &&
		r.build >= 0 &&
		(r.screen == "" || screenName.MatchString(r.screen))
}

// fingerprint — отпечаток ошибки: текст и первые 5 непустых строк стека
// без цифр и с пробелами, сжатыми до одного (требование 5).
func fingerprint(text, stack string) string {
	parts := []string{normalize(text)}
	for _, line := range strings.Split(stack, "\n") {
		if len(parts) == 6 {
			break
		}
		if line = normalize(line); line != "" {
			parts = append(parts, line)
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func normalize(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

var (
	errInvalidReport = gen.Error{
		Code:    "invalid_report",
		Message: "Отчёт не разобрался: текст ошибки от 1 до 2000 символов, стек до 20000, версия до 32, система до 100, сборка от 0, экран из a-z и _",
	}
	errTooManyReports = gen.Error{
		Code:    "too_many_reports",
		Message: "Слишком много отчётов за час",
	}
)
