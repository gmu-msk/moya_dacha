package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gmu-msk/moya_dacha/backend/internal/push"
)

// pushConfig — настройки пушей из окружения (specs/024-push.md,
// требования 9, 14 и 15). Ключ в FCM_CREDENTIALS — JSON целиком или он
// же в base64: так его кладёт деплой, в файле окружения JSON в несколько
// строк не живёт.
func pushConfig() (push.Config, error) {
	cfg := push.Config{Proxy: strings.TrimSpace(os.Getenv("PUSH_PROXY"))}
	raw := strings.TrimSpace(os.Getenv("FCM_CREDENTIALS"))
	if raw != "" && !strings.HasPrefix(raw, "{") {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return cfg, fmt.Errorf("FCM_CREDENTIALS: ни JSON, ни base64: %w", err)
		}
		raw = string(decoded)
	}
	cfg.Credentials = []byte(raw)
	if d := strings.TrimSpace(os.Getenv("PUSH_DELAY")); d != "" {
		delay, err := time.ParseDuration(d)
		if err != nil {
			return cfg, fmt.Errorf("PUSH_DELAY: %w", err)
		}
		cfg.Delay = delay
	}
	return cfg, nil
}

// pushCheck — команда push-check: состояние пушей одной строкой для
// итога деплоя (требование 16). Проблема Firebase — не ошибка команды:
// деплой из-за неё не краснеет. Итог публичный, поэтому ни ключа, ни
// текста ошибки в выводе нет.
func pushCheck() error {
	cfg, err := pushConfig()
	if err != nil {
		fmt.Println("Пуши: FCM_CREDENTIALS не разобран")
		return nil
	}
	sender, err := push.New(nil, cfg)
	if err != nil {
		fmt.Println("Пуши: FCM_CREDENTIALS не разобран")
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	project, err := sender.Check(ctx)
	switch {
	case errors.Is(err, push.ErrNotConfigured):
		fmt.Println("Пуши: FCM_CREDENTIALS не задан")
	case err != nil:
		fmt.Println("Пуши: Firebase не принял ключ или недоступен")
	default:
		fmt.Printf("Пуши: Firebase принял ключ, проект %s\n", project)
	}
	return nil
}
