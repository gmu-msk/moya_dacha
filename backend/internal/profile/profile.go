// Package profile — правила, которым подчиняются имя и «о себе»
// (specs/002-profile.md).
package profile

import (
	"errors"
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

var (
	// ErrInvalidName — имя пустое, слишком длинное или не одна строка.
	ErrInvalidName = errors.New("имя не подходит")
	// ErrInvalidAbout — «о себе» слишком длинное или не одна строка.
	ErrInvalidAbout = errors.New("«о себе» не подходит")
)

// NormalizeName проверяет имя и обрезает пробелы по краям.
// Имя обязательно: пользователь без имени не подписан нигде.
func NormalizeName(raw string) (string, error) {
	if hasControl(raw) {
		return "", ErrInvalidName
	}

	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLength {
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
