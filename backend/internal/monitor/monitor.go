// Package monitor — то, из чего собран дашборд владельца
// (specs/016-dashboard.md): измерения машины, учёт ошибок сервера,
// сводка для страницы и тревоги. Отдельных программ мониторинга на VPS
// нет, всё живёт в самом сервисе и его базе (ADR-0019).
package monitor

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Сколько хранятся измерения и ошибки (требования 13 и 15).
const (
	samplesKeep = 14 * 24 * time.Hour
	errorsKeep  = 30 * 24 * time.Hour
)

// started — когда запустился процесс: отсюда uptime_seconds.
var started = time.Now()

// Monitor считает запросы между измерениями, раз в минуту меряет машину
// и записывает ошибки сервера. Один на процесс сервиса.
type Monitor struct {
	db       *pgxpool.Pool
	diskPath string
	requests atomic.Int64
}

// New — монитор, который меряет диск там, где лежит diskPath.
func New(db *pgxpool.Pool, diskPath string) *Monitor {
	return &Monitor{db: db, diskPath: diskPath}
}

// DiskPath — где монитор меряет диск.
func (m *Monitor) DiskPath() string { return m.diskPath }

// CountRequest учитывает один обслуженный запрос.
func (m *Monitor) CountRequest() { m.requests.Add(1) }

// Run меряет машину раз в минуту, пока не отменён ctx. Первое измерение —
// через минуту после запуска: загрузку процессора считать не из чего,
// пока нет предыдущего снимка.
func (m *Monitor) Run(ctx context.Context) {
	prev, _ := readCPU()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		cur, ok := readCPU()
		var cpu float64
		if ok {
			cpu = cpuPercent(prev, cur)
			prev = cur
		}
		m.sample(ctx, cpu)
	}
}

func (m *Monitor) sample(ctx context.Context, cpu float64) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	used, total, swap := readMemory()
	diskUsed, diskTotal, err := DiskUsage(m.diskPath)
	if err != nil {
		slog.Warn("не удалось измерить диск", "path", m.diskPath, "err", err)
	}
	requests := m.requests.Swap(0)

	_, err = m.db.Exec(ctx, `
		INSERT INTO server_samples (cpu_percent, memory_used_bytes, memory_total_bytes,
			swap_used_bytes, disk_used_bytes, disk_total_bytes, requests)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		cpu, used, total, swap, diskUsed, diskTotal, requests)
	if err != nil {
		// Запросы не пропадают: доедут со следующим измерением.
		m.requests.Add(requests)
		slog.Error("не удалось записать измерение", "err", err)
		return
	}

	_, err = m.db.Exec(ctx, `DELETE FROM server_samples WHERE measured_at < now() - make_interval(secs => $1)`,
		samplesKeep.Seconds())
	if err == nil {
		_, err = m.db.Exec(ctx, `DELETE FROM server_errors WHERE at < now() - make_interval(secs => $1)`,
			errorsKeep.Seconds())
	}
	if err == nil {
		err = m.forgetAppErrors(ctx)
	}
	if err != nil {
		slog.Error("не удалось стереть старые измерения", "err", err)
	}
}

// RecordError записывает ошибку сервера. Запись идёт в фоне: ответ
// клиенту она не задерживает и не меняет, а сама неудача уходит в лог
// (specs/016-dashboard.md, «Ограничения»).
func (m *Monitor) RecordError(method, path string, status int, message string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := m.db.Exec(ctx, `
			INSERT INTO server_errors (method, path, status, message)
			VALUES ($1, $2, $3, $4)`, method, path, status, message)
		if err != nil {
			slog.Error("не удалось записать ошибку сервера", "path", path, "err", err)
		}
	}()
}

// Uptime — сколько работает процесс сервиса.
func Uptime() time.Duration { return time.Since(started) }
