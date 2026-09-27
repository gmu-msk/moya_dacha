package monitor

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// appErrorsKeep — сколько хранятся отчёты об ошибках приложения
// (specs/021-app-errors.md, требование 7).
const appErrorsKeep = 30 * 24 * time.Hour

// AppErrors — ошибки приложения (specs/021-app-errors.md, требования 11–14).
type AppErrors struct {
	LastDay  int64           `json:"last_day"`
	LastWeek int64           `json:"last_week"`
	NewWeek  int64           `json:"new_week"`
	Groups   []AppErrorGroup `json:"groups"`
}

type AppErrorGroup struct {
	Error     string    `json:"error"`
	Where     string    `json:"where"`
	Stack     string    `json:"stack"`
	Reports   int64     `json:"reports"`
	Users     int64     `json:"users"`
	Anonymous int64     `json:"anonymous"`
	FirstAt   time.Time `json:"first_at"`
	LastAt    time.Time `json:"last_at"`
	Version   string    `json:"version"`
	Build     int64     `json:"build"`
	Screen    string    `json:"screen"`
	OS        string    `json:"os"`
}

// ErrorWhere — место ошибки в коде: первая строка стека из кода
// приложения, иначе первая непустая (требование 8).
func ErrorWhere(stack string) string {
	first := ""
	for _, line := range strings.Split(stack, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "package:moya_dacha/") {
			return line
		}
		if first == "" {
			first = line
		}
	}
	return first
}

// firstLines — первые n строк текста.
func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// appErrors — счётчики и до 20 групп с отчётами за 30 дней, поля — по
// последнему отчёту группы (требования 12–13).
func (m *Monitor) appErrors(ctx context.Context, s *Snapshot) error {
	if err := m.db.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM app_errors WHERE at > now() - interval '24 hours'),
		       (SELECT count(*) FROM app_errors WHERE at > now() - interval '7 days'),
		       (SELECT count(*) FROM app_error_groups WHERE first_at > now() - interval '7 days')`).
		Scan(&s.AppErrors.LastDay, &s.AppErrors.LastWeek, &s.AppErrors.NewWeek); err != nil {
		return err
	}
	rows, err := m.db.Query(ctx, `
		SELECT last.error, last.stack, c.reports, c.users, c.anonymous,
		       g.first_at, g.last_at, last.version, last.build, last.screen, last.os
		FROM app_error_groups g
		JOIN LATERAL (
			SELECT count(*) AS reports,
			       count(DISTINCT user_id) AS users,
			       count(*) FILTER (WHERE user_id IS NULL) AS anonymous
			FROM app_errors e
			WHERE e.group_id = g.id AND e.at > now() - interval '30 days'
		) c ON c.reports > 0
		JOIN LATERAL (
			SELECT error, stack, version, build, screen, os
			FROM app_errors e WHERE e.group_id = g.id
			ORDER BY at DESC, id DESC LIMIT 1
		) last ON true
		ORDER BY g.last_at DESC, g.id DESC
		LIMIT 20`)
	if err != nil {
		return err
	}
	s.AppErrors.Groups, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (AppErrorGroup, error) {
		var g AppErrorGroup
		err := row.Scan(&g.Error, &g.Stack, &g.Reports, &g.Users, &g.Anonymous,
			&g.FirstAt, &g.LastAt, &g.Version, &g.Build, &g.Screen, &g.OS)
		g.Where = ErrorWhere(g.Stack)
		g.Stack = firstLines(g.Stack, 30)
		g.FirstAt, g.LastAt = g.FirstAt.UTC(), g.LastAt.UTC()
		return g, err
	})
	if s.AppErrors.Groups == nil {
		s.AppErrors.Groups = []AppErrorGroup{}
	}
	return err
}

// forgetAppErrors стирает отчёты старше 30 дней и группы, у которых
// отчётов не осталось: та же ошибка потом — снова новая (требование 7).
func (m *Monitor) forgetAppErrors(ctx context.Context) error {
	if _, err := m.db.Exec(ctx, `DELETE FROM app_errors WHERE at < now() - make_interval(secs => $1)`,
		appErrorsKeep.Seconds()); err != nil {
		return err
	}
	_, err := m.db.Exec(ctx, `
		DELETE FROM app_error_groups g
		WHERE NOT EXISTS (SELECT 1 FROM app_errors e WHERE e.group_id = g.id)`)
	return err
}
