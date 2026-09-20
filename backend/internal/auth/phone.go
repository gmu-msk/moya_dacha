// Package auth — предметная часть входа по номеру телефона: разбор номера,
// выдача кода подтверждения и его доставка. Хендлеры, которые всем этим
// пользуются, лежат в internal/api (specs/001-auth.md).
package auth

import "errors"

// ErrInvalidPhone — номер не разбирается или не российский мобильный.
var ErrInvalidPhone = errors.New("номер не является российским мобильным")

// NormalizePhone приводит номер к единому виду +79XXXXXXXXX.
//
// `+7 (900) 123-45-67`, `8 900 123 45 67` и `79001234567` — один и тот же
// номер: всё, что не цифра, отбрасывается, а ведущая восьмёрка заменяется
// семёркой. В MVP принимаются только российские мобильные номера
// (specs/001-auth.md, требование 3).
func NormalizePhone(raw string) (string, error) {
	digits := make([]byte, 0, len(raw))
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits = append(digits, byte(r))
		}
	}

	if len(digits) != 11 {
		return "", ErrInvalidPhone
	}
	if digits[0] != '7' && digits[0] != '8' {
		return "", ErrInvalidPhone
	}
	if digits[1] != '9' {
		return "", ErrInvalidPhone
	}

	digits[0] = '7'
	return "+" + string(digits), nil
}
