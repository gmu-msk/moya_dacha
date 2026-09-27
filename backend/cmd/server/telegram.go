package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/feedback"
	"github.com/gmu-msk/moya_dacha/backend/internal/media"
	"github.com/gmu-msk/moya_dacha/backend/internal/telegram"
)

// feedbackConfig — настройки отзывов из окружения (specs/019-feedback.md,
// требования 3 и 28). store — хранилище файлов сервиса, nil — из MEDIA_DIR.
func feedbackConfig(store media.Storage) feedback.Config {
	if store == nil {
		if dir := os.Getenv("MEDIA_DIR"); dir != "" {
			baseURL := os.Getenv("MEDIA_BASE_URL")
			if baseURL == "" {
				baseURL = "/media"
			}
			store = media.NewDisk(dir, baseURL)
		}
	}
	return feedback.Config{
		Token:     strings.TrimSpace(os.Getenv("FEEDBACK_GITHUB_TOKEN")),
		Repo:      strings.TrimSpace(os.Getenv("FEEDBACK_GITHUB_REPO")),
		PublicURL: strings.TrimSpace(os.Getenv("PUBLIC_URL")),
		Media:     store,
	}
}

// telegramConfig — настройки бота из окружения (specs/018-telegram-bot.md,
// требования 1–2). Без токена бота нет: ok == false.
func telegramConfig() (cfg telegram.Config, ok bool) {
	// Пробелы и перевод строки по краям приходят из вставки с телефона.
	cfg = telegram.Config{
		Token:    strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		APIURL:   strings.TrimSpace(os.Getenv("TELEGRAM_API_URL")),
		Owner:    strings.TrimSpace(os.Getenv("TELEGRAM_OWNER")),
		Proxy:    strings.TrimSpace(os.Getenv("TELEGRAM_PROXY")),
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
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "https://") {
			link = a
		} else {
			apk = a
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

	bot := telegram.New(pool, cfg)
	err = bot.SendBuild(ctx, role, info, apk, link)
	if errors.Is(err, telegram.ErrNotBound) {
		fmt.Printf("Чат %s не привязан: сборка не отправлена\n", role)
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("Сборка отправлена: %s\n", role)

	// Сборка main в группе — всё сделанное из отзывов вышло в ней
	// (specs/019-feedback.md, требование 11). Неудача сборку не отменяет.
	if role == telegram.RoleGroup {
		var bi struct {
			Build int64 `json:"build"`
		}
		if err := json.Unmarshal(info, &bi); err == nil && bi.Build > 0 {
			if err := feedback.New(pool, feedbackConfig(nil)).Release(ctx, bi.Build, bot); err != nil {
				fmt.Printf("Отзывы: не удалось отметить вышедшее: %v\n", err)
			}
		}
	}
	return nil
}

// telegramCheck — команда telegram-check: состояние бота одним текстом
// для итога деплоя. Проблема бота — не ошибка команды, деплой из-за неё
// не краснеет. С PUBLIC_OUTPUT в выводе нет ника владельца, пути до
// Telegram и текстов ошибок (specs/018-telegram-bot.md, требование 3а).
func telegramCheck() error {
	public := publicOutput()
	cfg, ok := telegramConfig()
	if !ok {
		fmt.Println("Бот: TELEGRAM_BOT_TOKEN не задан")
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	bot := telegram.New(pool, cfg)

	name, err := bot.Me(ctx)
	switch {
	case err != nil && public:
		fmt.Println("Бот: Telegram не принял токен или недоступен")
		return nil
	case err != nil:
		fmt.Printf("Бот: Telegram не принял токен или недоступен: %v\n", err)
		return nil
	case public:
		fmt.Printf("Бот: @%s\n", name)
	case cfg.Proxy != "":
		fmt.Printf("Бот: @%s, через туннель до второго VPS\n", name)
	case cfg.APIURL != "":
		fmt.Printf("Бот: @%s, через %s\n", name, cfg.APIURL)
	default:
		fmt.Printf("Бот: @%s\n", name)
	}
	switch {
	case cfg.Owner == "":
		fmt.Println("Владелец: TELEGRAM_OWNER не задан, /start никого не привяжет")
	case public:
		fmt.Println("Владелец: задан")
	default:
		fmt.Printf("Владелец: @%s\n", strings.TrimPrefix(cfg.Owner, "@"))
	}
	for _, role := range []string{telegram.RoleOwner, telegram.RoleGroup} {
		bound, err := bot.Bound(ctx, role)
		if err != nil {
			return err
		}
		state := "не привязан"
		if bound {
			state = "привязан"
		}
		fmt.Printf("Чат %s: %s\n", role, state)
	}
	printGitHub(ctx, pool, public)
	return nil
}

// printGitHub — строка про задачи GitHub (specs/019-feedback.md,
// требование 29).
func printGitHub(ctx context.Context, pool *pgxpool.Pool, public bool) {
	fb := feedback.New(pool, feedbackConfig(nil))
	err := fb.Check(ctx)
	switch {
	case errors.Is(err, feedback.ErrNotConfigured):
		fmt.Println("GitHub: FEEDBACK_GITHUB_TOKEN не задан — отзывы копятся в базе")
	case err != nil && public:
		fmt.Println("GitHub: не принял токен")
	case err != nil:
		fmt.Printf("GitHub: не принял токен: %v\n", err)
	default:
		fmt.Printf("GitHub: задачи в %s\n", fb.Repo())
	}
}

// dashboardPassword — команда dashboard-password: адрес и пароль
// дашборда владельцу в личку бота (specs/016-dashboard.md, требование 5).
// Сама команда ничего секретного не печатает: её вывод — публичный лог
// прогона «Дашборд».
func dashboardPassword() error {
	password := os.Getenv("DASHBOARD_PASSWORD")
	if password == "" {
		return errors.New("DASHBOARD_PASSWORD не задан: пароль появится после деплоя из main")
	}
	cfg, ok := telegramConfig()
	if !ok {
		return errors.New("бот не настроен: пароль некуда отправить")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	text := "Дашборд МоейДачи\n"
	if url := strings.TrimSpace(os.Getenv("PUBLIC_URL")); url != "" {
		text += url + "/dashboard\n"
	}
	text += "Имя любое, пароль:\n" + password
	err = telegram.New(pool, cfg).SendTo(ctx, telegram.RoleOwner, text)
	if errors.Is(err, telegram.ErrNotBound) {
		return errors.New("личка владельца в боте не привязана (/start): пароль некуда отправить")
	}
	if err != nil {
		// Текст ошибки не печатается: в нём может оказаться адрес
		// Bot API с токеном, а вывод виден всем.
		return errors.New("пароль не отправлен: Telegram недоступен")
	}
	fmt.Println("Адрес и пароль дашборда отправлены владельцу в Telegram")
	return nil
}
