package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/telegram"
)

// telegramConfig — настройки бота из окружения (specs/018-telegram-bot.md,
// требования 1–2). Без токена бота нет: ok == false.
func telegramConfig() (cfg telegram.Config, ok bool) {
	cfg = telegram.Config{
		Token:    os.Getenv("TELEGRAM_BOT_TOKEN"),
		APIURL:   os.Getenv("TELEGRAM_API_URL"),
		Owner:    os.Getenv("TELEGRAM_OWNER"),
		DiskPath: os.Getenv("MEDIA_DIR"),
	}
	return cfg, cfg.Token != ""
}

// buildNotify — команда build-notify (требования 16–18): отправить
// сборку в чат роли. Не настроенный бот и не привязанный чат — не
// ошибка: деплой из-за них не краснеет.
func buildNotify(args []string) error {
	role := args[0]
	if role != telegram.RoleOwner && role != telegram.RoleGroup {
		return fmt.Errorf("роль — owner или group, а не %q", role)
	}

	var info []byte
	var err error
	if args[1] == "-" {
		info, err = io.ReadAll(os.Stdin)
	} else {
		info, err = os.ReadFile(args[1])
	}
	if err != nil {
		return err
	}

	var apk, link string
	if len(args) == 3 {
		if strings.HasPrefix(args[2], "https://") {
			link = args[2]
		} else {
			apk = args[2]
		}
	}

	cfg, ok := telegramConfig()
	if !ok {
		fmt.Println("Бот не настроен: сборка не отправлена")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("переменная окружения DATABASE_URL не задана")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	err = telegram.New(pool, cfg).SendBuild(ctx, role, info, apk, link)
	if errors.Is(err, telegram.ErrNotBound) {
		fmt.Printf("Чат %s не привязан: сборка не отправлена\n", role)
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("Сборка отправлена: %s\n", role)
	return nil
}
