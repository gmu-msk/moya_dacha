package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"math/big"
	"strings"
	"time"
)

// Сроки и лимиты из specs/001-auth.md. Сервис умеет работать и с другими
// (тестам нужны короткие), но по умолчанию действуют эти.
const (
	// DefaultCodeTTL — сколько живёт выданный код подтверждения.
	DefaultCodeTTL = 5 * time.Minute
	// DefaultResendAfter — как часто можно просить код на один номер.
	DefaultResendAfter = time.Minute
	// MaxAttempts — сколько раз можно ошибиться, вводя один код.
	MaxAttempts = 5
	// CodeLength — длина кода подтверждения в цифрах.
	CodeLength = 4
)

// Sender доставляет код подтверждения до владельца номера.
//
// Отдельный интерфейс существует ровно затем, чтобы выбор SMS-провайдера
// не трогал ни контракт, ни хендлеры: подключение провайдера — это новая
// реализация Sender и строчка в main, больше ничего
// (specs/001-auth.md, требование 13).
type Sender interface {
	Send(ctx context.Context, phone, code string) error
}

// LogSender пишет код в лог сервиса — так код доставляется, пока
// SMS-провайдер не выбран: владелец сервиса читает лог и передаёт код
// тестировщику (specs/000-overview.md, «Внешние зависимости»).
type LogSender struct{}

func (LogSender) Send(_ context.Context, phone, code string) error {
	slog.Info("код подтверждения", "phone", phone, "code", code)
	return nil
}

// GenerateCode возвращает случайный код подтверждения из CodeLength цифр.
func GenerateCode() (string, error) {
	var code strings.Builder
	for range CodeLength {
		digit, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code.WriteString(digit.String())
	}
	return code.String(), nil
}

// GenerateToken возвращает токен сессии: 32 случайных байта, которые
// нельзя угадать и не нужно читать глазами.
func GenerateToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Hash — то, что хранится в базе вместо самого кода или токена
// (specs/001-auth.md, «Модель данных»).
func Hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
