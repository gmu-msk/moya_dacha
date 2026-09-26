package monitor

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Usage — заходы и время в приложении (specs/020-app-sessions.md,
// требования 11–15).
type Usage struct {
	WeekUsers int64         `json:"week_users"`
	Days      []UsageDay    `json:"days"`
	Screens   []UsageScreen `json:"screens"`
}

type UsageDay struct {
	Date     string `json:"date"`
	Users    int64  `json:"users"`
	Sessions int64  `json:"sessions"`
	Minutes  int64  `json:"minutes"`
}

type UsageScreen struct {
	Screen string `json:"screen"`
	Opens  int64  `json:"opens"`
	Users  int64  `json:"users"`
}

// usage — 30 московских дней по началу сессии, как activity, и экраны
// за неделю. Сессия без конца — ноль минут, длиннее трёх часов —
// три часа (требование 12).
func (m *Monitor) usage(ctx context.Context, s *Snapshot) error {
	rows, err := m.db.Query(ctx, `
		WITH days AS (
			SELECT (now() AT TIME ZONE 'Europe/Moscow')::date - i AS day
			FROM generate_series(0, 29) AS i
		),
		since AS (
			SELECT (min(day)::timestamp AT TIME ZONE 'Europe/Moscow') AS at FROM days
		),
		by_day AS (
			SELECT (started_at AT TIME ZONE 'Europe/Moscow')::date AS day,
			       count(DISTINCT user_id) AS users,
			       count(*)                AS sessions,
			       coalesce(sum(extract(epoch FROM least(
			           coalesce(ended_at, started_at) - started_at,
			           interval '3 hours'))), 0) AS seconds
			FROM app_sessions, since
			WHERE started_at >= since.at
			GROUP BY 1
		)
		SELECT to_char(d.day, 'YYYY-MM-DD'),
		       coalesce(b.users, 0), coalesce(b.sessions, 0),
		       floor(coalesce(b.seconds, 0) / 60)::bigint
		FROM days d
		LEFT JOIN by_day b ON b.day = d.day
		ORDER BY d.day`)
	if err != nil {
		return err
	}
	s.Usage.Days, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (UsageDay, error) {
		var d UsageDay
		err := row.Scan(&d.Date, &d.Users, &d.Sessions, &d.Minutes)
		return d, err
	})
	if err != nil {
		return err
	}

	if err := m.db.QueryRow(ctx, `
		SELECT count(DISTINCT user_id) FROM app_sessions
		WHERE started_at > now() - interval '7 days'`).
		Scan(&s.Usage.WeekUsers); err != nil {
		return err
	}

	rows, err = m.db.Query(ctx, `
		SELECT sc.screen, sum(sc.opens)::bigint, count(DISTINCT se.user_id)
		FROM app_session_screens sc
		JOIN app_sessions se ON se.id = sc.session_id
		WHERE se.started_at > now() - interval '7 days'
		GROUP BY sc.screen
		ORDER BY 2 DESC, 1`)
	if err != nil {
		return err
	}
	s.Usage.Screens, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (UsageScreen, error) {
		var sc UsageScreen
		err := row.Scan(&sc.Screen, &sc.Opens, &sc.Users)
		return sc, err
	})
	if s.Usage.Screens == nil {
		s.Usage.Screens = []UsageScreen{}
	}
	return err
}
