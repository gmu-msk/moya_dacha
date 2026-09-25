package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gmu-msk/moya_dacha/backend/internal/monitor"
)

// printAlerts печатает текущие тревоги дашборда по одной на строке или
// «Всё в порядке» (specs/016-dashboard.md, требование 20). Тревога —
// не ошибка команды: ошибкой команда выходит, только если проверить
// не вышло. Её зовёт прогон «Дашборд» в GitHub Actions.
func printAlerts(ctx context.Context, pool *pgxpool.Pool) error {
	// Диск меряется там же, где его меряет сервис: в папке файлов.
	diskPath := os.Getenv("MEDIA_DIR")
	if diskPath == "" {
		diskPath = "/"
	}
	alerts, err := monitor.Alerts(ctx, pool, diskPath)
	if err != nil {
		return err
	}
	if len(alerts) == 0 {
		fmt.Println("Всё в порядке")
		return nil
	}
	for _, a := range alerts {
		fmt.Println(a.Message)
	}
	return nil
}
