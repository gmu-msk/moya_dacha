package monitor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Snapshot — ответ /dashboard/data (specs/016-dashboard.md, «API»).
type Snapshot struct {
	GeneratedAt time.Time `json:"generated_at"`
	Alerts      []Alert   `json:"alerts"`
	Totals      Totals    `json:"totals"`
	Activity    []Day     `json:"activity"`
	Server      Server    `json:"server"`
	Errors      Errors    `json:"errors"`
	Feedback    Feedback  `json:"feedback"`
	Usage       Usage     `json:"usage"`
}

// Feedback — отзывы разработчику (specs/019-feedback.md, требование 27).
type Feedback struct {
	Week   int64           `json:"week"`
	Recent []FeedbackEntry `json:"recent"`
}

type FeedbackEntry struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Source    string    `json:"source"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	Status    string    `json:"status"`
	Issue     *int32    `json:"issue"`
	IssueURL  *string   `json:"issue_url"`
	// Screenshot — ключ хранилища; ссылку из него делает дашборд.
	Screenshot    *string `json:"-"`
	ScreenshotURL *string `json:"screenshot_url"`
}

type Alert struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type Totals struct {
	Users    int64 `json:"users"`
	Posts    int64 `json:"posts"`
	Likes    int64 `json:"likes"`
	Comments int64 `json:"comments"`
	Reports  int64 `json:"reports"`
}

type Day struct {
	Date        string `json:"date"`
	NewUsers    int64  `json:"new_users"`
	ActiveUsers int64  `json:"active_users"`
	Posts       int64  `json:"posts"`
	Likes       int64  `json:"likes"`
	Comments    int64  `json:"comments"`
}

type Server struct {
	Now     *Now   `json:"now"`
	History []Step `json:"history"`
}

type Now struct {
	MeasuredAt       time.Time `json:"measured_at"`
	CPUPercent       float64   `json:"cpu_percent"`
	MemoryUsedBytes  int64     `json:"memory_used_bytes"`
	MemoryTotalBytes int64     `json:"memory_total_bytes"`
	SwapUsedBytes    int64     `json:"swap_used_bytes"`
	DiskUsedBytes    int64     `json:"disk_used_bytes"`
	DiskTotalBytes   int64     `json:"disk_total_bytes"`
	DBSizeBytes      int64     `json:"db_size_bytes"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
}

type Step struct {
	At              time.Time `json:"at"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryUsedBytes int64     `json:"memory_used_bytes"`
	DiskUsedBytes   int64     `json:"disk_used_bytes"`
	Requests        int64     `json:"requests"`
	Errors          int64     `json:"errors"`
}

type Errors struct {
	LastHour int64         `json:"last_hour"`
	LastDay  int64         `json:"last_day"`
	Recent   []ServerError `json:"recent"`
}

type ServerError struct {
	At      time.Time `json:"at"`
	Method  string    `json:"method"`
	Path    string    `json:"path"`
	Status  int       `json:"status"`
	Message string    `json:"message"`
}

// Collect собирает сводку для страницы целиком.
func (m *Monitor) Collect(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{GeneratedAt: time.Now().UTC()}
	steps := []func(context.Context, *Snapshot) error{
		m.totals, m.activity, m.server, m.errors, m.feedback, m.usage,
	}
	for _, step := range steps {
		if err := step(ctx, &snap); err != nil {
			return Snapshot{}, err
		}
	}
	// Пустые списки уходят в JSON пустыми, а не null.
	if snap.Server.History == nil {
		snap.Server.History = []Step{}
	}
	if snap.Errors.Recent == nil {
		snap.Errors.Recent = []ServerError{}
	}
	alerts, err := Alerts(ctx, m.db, m.diskPath)
	if err != nil {
		return Snapshot{}, err
	}
	snap.Alerts = alerts
	return snap, nil
}

// feedback — отзывы за неделю и последние десять (019, требование 27).
func (m *Monitor) feedback(ctx context.Context, s *Snapshot) error {
	if err := m.db.QueryRow(ctx, `
		SELECT count(*) FROM feedback WHERE created_at > now() - interval '7 days'`).
		Scan(&s.Feedback.Week); err != nil {
		return err
	}
	rows, err := m.db.Query(ctx, `
		SELECT id, created_at, source, author,
		       CASE WHEN char_length(text) > 140 THEN left(text, 140) || '…' ELSE text END,
		       status, issue, issue_url, screenshot
		FROM feedback ORDER BY created_at DESC, id DESC LIMIT 10`)
	if err != nil {
		return err
	}
	s.Feedback.Recent, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (FeedbackEntry, error) {
		var e FeedbackEntry
		err := row.Scan(&e.ID, &e.CreatedAt, &e.Source, &e.Author, &e.Text, &e.Status, &e.Issue, &e.IssueURL, &e.Screenshot)
		return e, err
	})
	if s.Feedback.Recent == nil {
		s.Feedback.Recent = []FeedbackEntry{}
	}
	return err
}

func (m *Monitor) totals(ctx context.Context, s *Snapshot) error {
	return m.db.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM users),
		       (SELECT count(*) FROM posts),
		       (SELECT count(*) FROM post_likes),
		       (SELECT count(*) FROM comments),
		       (SELECT count(*) FROM reports)`).
		Scan(&s.Totals.Users, &s.Totals.Posts, &s.Totals.Likes, &s.Totals.Comments, &s.Totals.Reports)
}

// activity — 30 московских дней, последний — сегодня (требование 10).
// Дни без событий приходят нулями: их даёт generate_series.
func (m *Monitor) activity(ctx context.Context, s *Snapshot) error {
	rows, err := m.db.Query(ctx, `
		WITH days AS (
			SELECT (now() AT TIME ZONE 'Europe/Moscow')::date - i AS day
			FROM generate_series(0, 29) AS i
		),
		since AS (
			SELECT (min(day)::timestamp AT TIME ZONE 'Europe/Moscow') AS at FROM days
		),
		events AS (
			SELECT created_at, author_id AS user_id, 'post' AS kind FROM posts, since WHERE created_at >= since.at
			UNION ALL
			SELECT created_at, user_id, 'like' FROM post_likes, since WHERE created_at >= since.at
			UNION ALL
			SELECT created_at, author_id, 'comment' FROM comments, since WHERE created_at >= since.at
		),
		by_day AS (
			SELECT (created_at AT TIME ZONE 'Europe/Moscow')::date AS day,
			       count(*) FILTER (WHERE kind = 'post')    AS posts,
			       count(*) FILTER (WHERE kind = 'like')    AS likes,
			       count(*) FILTER (WHERE kind = 'comment') AS comments,
			       count(DISTINCT user_id)                  AS active_users
			FROM events GROUP BY 1
		),
		new_users AS (
			SELECT (created_at AT TIME ZONE 'Europe/Moscow')::date AS day, count(*) AS n
			FROM users, since WHERE created_at >= since.at GROUP BY 1
		)
		SELECT to_char(d.day, 'YYYY-MM-DD'),
		       coalesce(n.n, 0), coalesce(b.active_users, 0),
		       coalesce(b.posts, 0), coalesce(b.likes, 0), coalesce(b.comments, 0)
		FROM days d
		LEFT JOIN by_day b ON b.day = d.day
		LEFT JOIN new_users n ON n.day = d.day
		ORDER BY d.day`)
	if err != nil {
		return err
	}
	s.Activity, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Day, error) {
		var d Day
		err := row.Scan(&d.Date, &d.NewUsers, &d.ActiveUsers, &d.Posts, &d.Likes, &d.Comments)
		return d, err
	})
	return err
}

// server — последнее измерение и сутки шагами по 5 минут (требования 11–12).
func (m *Monitor) server(ctx context.Context, s *Snapshot) error {
	var now Now
	err := m.db.QueryRow(ctx, `
		SELECT measured_at, cpu_percent, memory_used_bytes, memory_total_bytes,
		       swap_used_bytes, disk_used_bytes, disk_total_bytes,
		       pg_database_size(current_database())
		FROM server_samples ORDER BY measured_at DESC LIMIT 1`).
		Scan(&now.MeasuredAt, &now.CPUPercent, &now.MemoryUsedBytes, &now.MemoryTotalBytes,
			&now.SwapUsedBytes, &now.DiskUsedBytes, &now.DiskTotalBytes, &now.DBSizeBytes)
	switch {
	case err == nil:
		now.UptimeSeconds = int64(time.Since(started).Seconds())
		s.Server.Now = &now
	case err != pgx.ErrNoRows:
		return err
	}

	rows, err := m.db.Query(ctx, `
		WITH steps AS (
			SELECT to_timestamp(floor(extract(epoch FROM measured_at) / 300) * 300) AS at,
			       avg(cpu_percent) AS cpu,
			       avg(memory_used_bytes)::bigint AS memory,
			       max(disk_used_bytes) AS disk,
			       sum(requests)::bigint AS requests
			FROM server_samples
			WHERE measured_at > now() - interval '24 hours'
			GROUP BY 1
		),
		errs AS (
			SELECT to_timestamp(floor(extract(epoch FROM at) / 300) * 300) AS at, count(*) AS n
			FROM server_errors
			WHERE at > now() - interval '24 hours' - interval '5 minutes'
			GROUP BY 1
		)
		SELECT s.at, s.cpu, s.memory, s.disk, s.requests, coalesce(e.n, 0)
		FROM steps s LEFT JOIN errs e ON e.at = s.at
		ORDER BY s.at`)
	if err != nil {
		return err
	}
	s.Server.History, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Step, error) {
		var st Step
		err := row.Scan(&st.At, &st.CPUPercent, &st.MemoryUsedBytes, &st.DiskUsedBytes, &st.Requests, &st.Errors)
		st.At = st.At.UTC()
		return st, err
	})
	return err
}

func (m *Monitor) errors(ctx context.Context, s *Snapshot) error {
	err := m.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE at > now() - interval '1 hour'),
		       count(*)
		FROM server_errors WHERE at > now() - interval '24 hours'`).
		Scan(&s.Errors.LastHour, &s.Errors.LastDay)
	if err != nil {
		return err
	}
	rows, err := m.db.Query(ctx, `
		SELECT at, method, path, status, message
		FROM server_errors ORDER BY at DESC, id DESC LIMIT 20`)
	if err != nil {
		return err
	}
	s.Errors.Recent, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ServerError, error) {
		var e ServerError
		err := row.Scan(&e.At, &e.Method, &e.Path, &e.Status, &e.Message)
		e.At = e.At.UTC()
		return e, err
	})
	return err
}

// Пороги тревог (требование 18).
const (
	diskFreeMinShare   = 0.10
	diskFreeMinBytes   = 2 << 30
	errorsPerHourAlarm = 3
	memoryAlarmShare   = 0.90
	memoryAlarmWindow  = 10 * time.Minute
)

// Alerts — текущие тревоги. Их считают в момент запроса и нигде не
// хранят (требование 19); этим же пользуется команда alerts.
func Alerts(ctx context.Context, db *pgxpool.Pool, diskPath string) ([]Alert, error) {
	alerts := []Alert{}

	used, total, err := DiskUsage(diskPath)
	if err != nil {
		return nil, fmt.Errorf("диск %s: %w", diskPath, err)
	}
	if free := total - used; total > 0 && (float64(free) < diskFreeMinShare*float64(total) || free < diskFreeMinBytes) {
		alerts = append(alerts, Alert{
			Kind:    "disk",
			Message: fmt.Sprintf("Мало места на диске: свободно %s из %s", Bytes(free), Bytes(total)),
		})
	}

	var lastHour int64
	if err := db.QueryRow(ctx, `SELECT count(*) FROM server_errors WHERE at > now() - interval '1 hour'`).
		Scan(&lastHour); err != nil {
		return nil, err
	}
	if lastHour >= errorsPerHourAlarm {
		alerts = append(alerts, Alert{
			Kind:    "errors",
			Message: fmt.Sprintf("Ошибок сервера за час: %d", lastHour),
		})
	}

	// «Десять минут подряд»: измерения за окно есть, и все выше порога.
	// Измерение раз в минуту, но пара пропусков тревогу не отменяет.
	var samples, high int64
	if err := db.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE memory_total_bytes > 0
		                          AND memory_used_bytes::float8 > $2::float8 * memory_total_bytes)
		FROM server_samples WHERE measured_at > now() - make_interval(secs => $1)`,
		memoryAlarmWindow.Seconds(), memoryAlarmShare).Scan(&samples, &high); err != nil {
		return nil, err
	}
	if samples >= 5 && high == samples {
		alerts = append(alerts, Alert{
			Kind:    "memory",
			Message: "Память занята больше 90% уже 10 минут",
		})
	}
	return alerts, nil
}

// Bytes — размер для человека: «1,4 ГБ», «512 МБ».
func Bytes(n int64) string {
	units := []string{"Б", "КБ", "МБ", "ГБ", "ТБ"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	var s string
	if v >= 10 || i == 0 {
		s = fmt.Sprintf("%.0f", v)
	} else {
		s = strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1)
	}
	return s + " " + units[i]
}
