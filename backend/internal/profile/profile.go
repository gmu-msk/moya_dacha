// Package profile — правила, которым подчиняются никнейм, имя и «о себе»
// (specs/002-profile.md, specs/010-nicknames.md).
package profile

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxNameLength — сколько символов помещается в имя.
	MaxNameLength = 50
	// MaxAboutLength — сколько символов помещается в «о себе».
	MaxAboutLength = 200
)

// nicknamePattern — никнейм целиком: от 3 до 20 латинских букв, цифр
// и подчёркиваний.
var nicknamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)

var (
	// ErrInvalidNickname — никнейм не подходит под правила.
	ErrInvalidNickname = errors.New("никнейм не подходит")
	// ErrInvalidName — имя слишком длинное или не одна строка.
	ErrInvalidName = errors.New("имя не подходит")
	// ErrInvalidAbout — «о себе» слишком длинное или не одна строка.
	ErrInvalidAbout = errors.New("«о себе» не подходит")
)

// NormalizeNickname проверяет никнейм и обрезает пробелы по краям.
// Регистр не трогает: никнейм хранится так, как его ввели, а уникален
// без учёта регистра — это забота базы.
func NormalizeNickname(raw string) (string, error) {
	nickname := strings.TrimSpace(raw)
	if !nicknamePattern.MatchString(nickname) {
		return "", ErrInvalidNickname
	}
	return nickname, nil
}

// NormalizeName проверяет полное имя и обрезает пробелы по краям.
// Имя необязательно: пользователя подписывает никнейм.
func NormalizeName(raw string) (string, error) {
	if hasControl(raw) {
		return "", ErrInvalidName
	}

	name := strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) > MaxNameLength {
		return "", ErrInvalidName
	}
	return name, nil
}

// NormalizeAbout проверяет «о себе» и обрезает пробелы по краям.
// Пустое «о себе» — это нормально: поле необязательное.
func NormalizeAbout(raw string) (string, error) {
	if hasControl(raw) {
		return "", ErrInvalidAbout
	}

	about := strings.TrimSpace(raw)
	if utf8.RuneCountInString(about) > MaxAboutLength {
		return "", ErrInvalidAbout
	}
	return about, nil
}

// hasControl отвечает, есть ли в строке управляющие символы: перевод
// строки, табуляция и прочее, из-за чего имя перестаёт быть одной
// строкой. Длина считается в символах, а не в байтах: имя из
// пятидесяти кириллических букв — это пятьдесят символов.
func hasControl(text string) bool {
	return strings.ContainsFunc(text, unicode.IsControl)
}
