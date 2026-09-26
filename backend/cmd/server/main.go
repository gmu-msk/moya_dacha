// Команда server — единственный бинарник проекта.
// Миграции он не накатывает: это отдельный шаг деплоя (ADR-0005).
//
// Без аргументов бинарник запускает сервис. С аргументом — выполняет
// команду владельца и выходит: invite, uninvite, invites
// (specs/015-invites.md, требование 11), alerts (specs/016-dashboard.md,
// требование 20) и build-notify (specs/018-telegram-bot.md, требование 16).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/api"
	"github.com/gmu-msk/moya_dacha/backend/internal/feedback"
	"github.com/gmu-msk/moya_dacha/backend/internal/media"
	"github.com/gmu-msk/moya_dacha/backend/internal/telegram"
)

func main() {
	if len(os.Args) > 1 {
		if err := runCommand(os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("сервис остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("переменная окружения DATABASE_URL не задана")
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// AUTH_FIXED_CODE задан только на демо-стенде и в тестах: он делает код
	// подтверждения предсказуемым, чтобы фичу можно было показать без SMS
	// (specs/001-auth.md). В проде переменная пуста, и код случайный.
	cfg := api.Config{
		FixedCode: os.Getenv("AUTH_FIXED_CODE"),
		// AUTH_INVITES включает вход по коду приглашения: так идёт закрытый
		// тест, пока нет SMS-провайдера (specs/015-invites.md).
		Invites: os.Getenv("AUTH_INVITES") != "",
	}
	switch {
	case cfg.Invites:
		slog.Info("вход по кодам приглашения: коды выдаёт команда invite")
	case cfg.FixedCode != "":
		slog.Warn("включён фиксированный код подтверждения: войти может кто угодно, в проде так быть не должно")
	}

	// MEDIA_DIR — папка, в которой лежат файлы пользователей, MEDIA_BASE_URL —
	// префикс ссылок на них (specs/002-profile.md, ADR-0011). Без MEDIA_DIR
	// сервис возьмёт временную папку и скажет об этом в логе: так он
	// поднимается и без настройки, но переживёт перезапуск только с ней.
	if dir := os.Getenv("MEDIA_DIR"); dir != "" {
		baseURL := os.Getenv("MEDIA_BASE_URL")
		if baseURL == "" {
			baseURL = api.DefaultMediaBaseURL
		}
		cfg.Media = media.NewDisk(dir, baseURL)
		cfg.DiskPath = dir
	}

	// DASHBOARD_PASSWORD открывает дашборд владельца по /dashboard
	// (specs/016-dashboard.md). На VPS пароль заводит deploy/setup.sh.
	cfg.DashboardPassword = os.Getenv("DASHBOARD_PASSWORD")
	if cfg.DashboardPassword == "" {
		slog.Info("DASHBOARD_PASSWORD не задан: дашборда нет")
	}
	// Отзывы разработчику и задачи GitHub (specs/019-feedback.md).
	fb := feedback.New(pool, feedbackConfig(cfg.Media))
	cfg.Feedback = fb
	if !fb.Enabled() {
		slog.Info("FEEDBACK_GITHUB_TOKEN не задан: отзывы копятся в базе без задач")
	}

	service := api.New(pool, cfg)
	go service.RunMonitor(ctx)

	// Telegram-бот: тревоги и сводка владельцу (specs/018-telegram-bot.md),
	// отзывы из Telegram (specs/019-feedback.md).
	var notifier feedback.Notifier
	if tg, ok := telegramConfig(); ok {
		if tg.Owner == "" {
			slog.Warn("TELEGRAM_OWNER не задан: бот никого не признает владельцем")
		}
		tg.Feedback = fb
		bot := telegram.New(pool, tg)
		notifier = bot
		go bot.Run(ctx)
		go bot.RunAlerts(ctx)
		slog.Info("Telegram-бот запущен", "owner", tg.Owner)
	} else {
		slog.Info("TELEGRAM_BOT_TOKEN не задан: бота нет")
	}
	go fb.Run(ctx, notifier)

	srv := &http.Server{
		Addr:              addr,
		Handler:           service.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		slog.Info("сервис запущен", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		slog.Info("получен сигнал остановки, завершаемся")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
