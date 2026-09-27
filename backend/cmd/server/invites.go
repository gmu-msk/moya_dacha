package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/auth"
	"github.com/gmu-msk/moya_dacha/backend/internal/telegram"
)

const usage = `команды:
  invite <номер>     выдать приглашение и напечатать код
                     (с PUBLIC_OUTPUT=1 — код в Telegram владельцу)
  uninvite <номер>   отозвать приглашение
  invites            действующие приглашения
  alerts             текущие тревоги дашборда
  build-notify <owner|group> <сведения.json|-> [<apk>|<ссылка>]
                     отправить сборку в Telegram
  telegram-check     состояние Telegram-бота
  dashboard-password адрес и пароль дашборда владельцу в Telegram`

// runCommand выполняет команду владельца сервиса против той же базы,
// с которой работает сервис (specs/015-invites.md, требование 11).
func runCommand(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Отправка APK в Telegram идёт дольше 30 секунд на медленном канале.
	if args[0] == "telegram-check" && len(args) == 1 {
		return telegramCheck()
	}
	if args[0] == "dashboard-password" && len(args) == 1 {
		return dashboardPassword()
	}
	if args[0] == "build-notify" {
		if len(args) < 3 || len(args) > 4 {
			return errors.New(usage)
		}
		return buildNotify(args[1:])
	}

	var phone string
	switch {
	case (args[0] == "invites" || args[0] == "alerts") && len(args) == 1:
	case (args[0] == "invite" || args[0] == "uninvite") && len(args) == 2:
		phone = args[1]
	default:
		return errors.New(usage)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("переменная окружения DATABASE_URL не задана")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Вывод прогона GitHub Actions виден всем: кода в нём нет, номера
	// под маской (specs/015-invites.md, требование 18).
	public := publicOutput()
	show := func(phone string) string {
		if public {
			return maskPhone(phone)
		}
		return phone
	}

	switch args[0] {
	case "alerts":
		return printAlerts(ctx, pool)
	case "invite":
		if public {
			return inviteViaTelegram(ctx, pool, phone)
		}
		normalized, code, err := auth.IssueInvite(ctx, pool, phone)
		if err != nil {
			return phoneError(err)
		}
		fmt.Printf("%s\t%s\n", normalized, code)
	case "uninvite":
		if err := auth.RevokeInvite(ctx, pool, phone); err != nil {
			return phoneError(err)
		}
		if normalized, err := auth.NormalizePhone(phone); err == nil {
			fmt.Printf("%s\tприглашение отозвано\n", show(normalized))
		}
	case "invites":
		invites, err := auth.ListInvites(ctx, pool)
		if err != nil {
			return err
		}
		if len(invites) == 0 {
			fmt.Println("действующих приглашений нет")
		}
		for _, invite := range invites {
			fmt.Printf("%s\tвыдано %s\tпопыток осталось %d\n",
				show(invite.Phone), invite.CreatedAt.Local().Format("2006-01-02 15:04"), invite.AttemptsLeft)
		}
	}
	return nil
}

// inviteViaTelegram выдаёт приглашение и отправляет код владельцу в личку
// бота, ничего секретного не печатая. Некуда отправить — приглашение
// не выдаётся; не удалось отправить — выданное отзывается
// (specs/015-invites.md, требование 18).
func inviteViaTelegram(ctx context.Context, pool *pgxpool.Pool, phone string) error {
	if _, err := auth.NormalizePhone(phone); err != nil {
		return phoneError(err)
	}
	cfg, ok := telegramConfig()
	if !ok {
		return errors.New("бот не настроен: код некуда отправить, приглашение не выдано")
	}
	bot := telegram.New(pool, cfg)
	bound, err := bot.Bound(ctx, telegram.RoleOwner)
	if err != nil {
		return err
	}
	if !bound {
		return errors.New("личка владельца в боте не привязана (/start): код некуда отправить, приглашение не выдано")
	}

	normalized, code, err := auth.IssueInvite(ctx, pool, phone)
	if err != nil {
		return phoneError(err)
	}
	text := fmt.Sprintf("Приглашение в МоюДачу\nНомер: %s\nКод: %s", normalized, code)
	if err := bot.SendTo(ctx, telegram.RoleOwner, text); err != nil {
		if revokeErr := auth.RevokeInvite(ctx, pool, normalized); revokeErr != nil {
			return fmt.Errorf("код не отправлен (%v), и выданное приглашение не отозвалось: %w", err, revokeErr)
		}
		return fmt.Errorf("код не отправлен, приглашение отозвано: %w", err)
	}
	fmt.Printf("%s\tкод отправлен владельцу в Telegram\n", maskPhone(normalized))
	return nil
}

// publicOutput — вывод команды уйдёт в публичный лог прогона GitHub
// Actions (specs/015-invites.md, требование 18): переменная PUBLIC_OUTPUT
// с любым непустым значением.
func publicOutput() bool {
	return os.Getenv("PUBLIC_OUTPUT") != ""
}

// maskPhone прячет номер вида +79001234567 до +7 *** ***-45-67.
func maskPhone(phone string) string {
	if len(phone) != 12 {
		return "+7 *** ***-**-**"
	}
	return "+7 *** ***-" + phone[8:10] + "-" + phone[10:]
}

func phoneError(err error) error {
	if errors.Is(err, auth.ErrInvalidPhone) {
		return errors.New("это не российский мобильный номер: ожидается вида +7 900 123-45-67")
	}
	return err
}
