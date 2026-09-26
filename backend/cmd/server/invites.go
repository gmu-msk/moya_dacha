package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/auth"
)

const usage = `команды:
  invite <номер>     выдать приглашение и напечатать код
  uninvite <номер>   отозвать приглашение
  invites            действующие приглашения
  alerts             текущие тревоги дашборда
  build-notify <owner|group> <сведения.json|-> [<apk>|<ссылка>]
                     отправить сборку в Telegram
  telegram-check     состояние Telegram-бота`

// runCommand выполняет команду владельца сервиса против той же базы,
// с которой работает сервис (specs/015-invites.md, требование 11).
func runCommand(args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Отправка APK в Telegram идёт дольше 30 секунд на медленном канале.
	if args[0] == "telegram-check" && len(args) == 1 {
		return telegramCheck()
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

	switch args[0] {
	case "alerts":
		return printAlerts(ctx, pool)
	case "invite":
		normalized, code, err := auth.IssueInvite(ctx, pool, phone)
		if err != nil {
			return phoneError(err)
		}
		fmt.Printf("%s\t%s\n", normalized, code)
	case "uninvite":
		if err := auth.RevokeInvite(ctx, pool, phone); err != nil {
			return phoneError(err)
		}
		fmt.Println("приглашение отозвано")
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
				invite.Phone, invite.CreatedAt.Local().Format("2006-01-02 15:04"), invite.AttemptsLeft)
		}
	}
	return nil
}

func phoneError(err error) error {
	if errors.Is(err, auth.ErrInvalidPhone) {
		return errors.New("это не российский мобильный номер: ожидается вида +7 900 123-45-67")
	}
	return err
}
