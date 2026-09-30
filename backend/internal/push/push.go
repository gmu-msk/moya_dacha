// Package push шлёт пуши о новых уведомлениях через Firebase Cloud
// Messaging HTTP v1 (specs/024-push.md, ADR-0027).
//
// Очередь в базе наполняют триггеры (backend/migrations/00019_push.sql):
// событие раздела «Уведомления» или заявка на подписку, если у получателя
// есть телефон. Здесь очередь разбирается: раз в Interval берётся то, что
// пролежало Delay, и уходит на все токены получателя.
package push

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Значения по умолчанию (требования 9 и 13).
const (
	DefaultAPIURL   = "https://fcm.googleapis.com"
	DefaultDelay    = 10 * time.Second
	DefaultInterval = time.Second
	DefaultMaxAge   = time.Hour
)

// MaxComment — сколько символов комментария попадает в пуш, как в строку
// раздела (specs/014-notifications.md).
const MaxComment = 100

// Config — настройки отправки.
type Config struct {
	// Credentials — ключ сервисного аккаунта Firebase, JSON целиком
	// (FCM_CREDENTIALS). Пусто — пушей нет.
	Credentials []byte
	// APIURL — адрес FCM. Пусто — DefaultAPIURL; тесты подставляют свой.
	APIURL string
	// Proxy — туннель до второго VPS (PUSH_PROXY), например
	// socks5://127.0.0.1:1080. Пусто — только напрямую.
	Proxy string
	// Delay — сколько событие лежит в очереди до отправки. 0 — DefaultDelay.
	Delay time.Duration
	// Interval — как часто разбирается очередь. 0 — DefaultInterval.
	Interval time.Duration
	// MaxAge — событие старше не отправляется. 0 — DefaultMaxAge.
	MaxAge time.Duration
}

// Sender разбирает очередь пушей.
type Sender struct {
	db   *pgxpool.Pool
	cfg  Config
	key  *serviceAccount // nil — ключа нет, пушей нет
	net  *network
	auth *tokenSource
}

// New разбирает ключ и готовит отправку. Ошибка — ключ задан, но не
// разобран: молча жить без пушей хуже, чем сказать об этом при запуске.
func New(db *pgxpool.Pool, cfg Config) (*Sender, error) {
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}
	cfg.APIURL = strings.TrimRight(cfg.APIURL, "/")
	if cfg.Delay == 0 {
		cfg.Delay = DefaultDelay
	}
	if cfg.Interval == 0 {
		cfg.Interval = DefaultInterval
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = DefaultMaxAge
	}
	s := &Sender{db: db, cfg: cfg, net: newNetwork(cfg.Proxy)}
	if len(strings.TrimSpace(string(cfg.Credentials))) == 0 {
		return s, nil
	}
	key, err := parseServiceAccount(cfg.Credentials)
	if err != nil {
		return nil, fmt.Errorf("ключ сервисного аккаунта Firebase: %w", err)
	}
	s.key = key
	s.auth = &tokenSource{key: key, net: s.net}
	return s, nil
}

// Enabled — задан ли ключ Firebase.
func (s *Sender) Enabled() bool { return s.key != nil }

// Check получает OAuth-токен: так видно, что Google принял ключ и до него
// есть дорога (требование 16). Возвращает проект Firebase.
func (s *Sender) Check(ctx context.Context) (project string, err error) {
	if s.key == nil {
		return "", ErrNotConfigured
	}
	if _, err := s.auth.token(ctx); err != nil {
		return "", err
	}
	return s.key.ProjectID, nil
}

// ViaProxy — ходит ли отправка уже через туннель.
func (s *Sender) ViaProxy() bool { return s.net.viaProxy.Load() }

// Run разбирает очередь, пока жив ctx.
func (s *Sender) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		if err := s.drain(ctx); err != nil && ctx.Err() == nil {
			slog.Error("пуши: очередь не разобрана", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// drain — один круг: выбросить устаревшее, отправить созревшее.
func (s *Sender) drain(ctx context.Context) error {
	if _, err := s.db.Exec(ctx,
		`DELETE FROM push_queue WHERE created_at < now() - make_interval(secs => $1)`,
		s.cfg.MaxAge.Seconds(),
	); err != nil {
		return err
	}
	if s.key == nil {
		// Без ключа очередь не копится (требование 14).
		_, err := s.db.Exec(ctx,
			`DELETE FROM push_queue WHERE created_at <= now() - make_interval(secs => $1)`,
			s.cfg.Delay.Seconds())
		return err
	}

	items, err := s.due(ctx)
	if err != nil {
		return err
	}
	for _, it := range items {
		if ctx.Err() != nil {
			return nil
		}
		if err := s.deliver(ctx, it); err != nil {
			// Временная ошибка: строка остаётся до следующего круга.
			slog.Warn("пуши: не отправлено, повтор на следующем круге", "err", err)
			continue
		}
		if _, err := s.db.Exec(ctx, `DELETE FROM push_queue WHERE id = $1`, it.id); err != nil {
			return err
		}
	}
	return nil
}

// deliver шлёт пуш на все токены получателя (требование 10). Токен,
// которого у FCM больше нет, забывается. Ошибка — временная: пуш будет
// отправлен ещё раз на все токены.
func (s *Sender) deliver(ctx context.Context, it item) error {
	if it.cancelled {
		return nil
	}
	rows, err := s.db.Query(ctx, `SELECT token FROM push_devices WHERE user_id = $1`, it.userID)
	if err != nil {
		return err
	}
	var tokens []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			return err
		}
		tokens = append(tokens, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var temporary error
	for _, t := range tokens {
		err := s.send(ctx, it.message(t))
		switch {
		case err == nil:
		case isGone(err):
			slog.Info("пуши: токена больше нет, забываю")
			if _, err := s.db.Exec(ctx, `DELETE FROM push_devices WHERE token = $1`, t); err != nil {
				return err
			}
		case isTemporary(err):
			temporary = err
		default:
			// FCM отверг само сообщение: повтор даст то же самое.
			slog.Error("пуши: FCM отверг сообщение", "err", err)
		}
	}
	return temporary
}
