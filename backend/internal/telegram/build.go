package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
)

// buildInfo — сведения о сборке (specs/017-app-updates.md).
type buildInfo struct {
	Version  string   `json:"version"`
	Build    int64    `json:"build"`
	WhatsNew []string `json:"whatsNew"`
}

// Лимиты Bot API для sendDocument (требования 16а, 16б).
const (
	MaxDocumentSize = 50 << 20 // 52 428 800 байт
	maxCaption      = 1024
)

// SendBuild отправляет сборку в чат роли: файлом, если задан apk и он
// пролезает в лимит Telegram, иначе текстом со ссылкой (требования
// 16–17). Не привязанный чат — ErrNotBound.
func (b *Bot) SendBuild(ctx context.Context, role string, info []byte, apk, link string) error {
	var bi buildInfo
	if err := json.Unmarshal(info, &bi); err != nil {
		return fmt.Errorf("сведения о сборке: %w", err)
	}
	chat, err := b.chat(ctx, role)
	if err != nil {
		return err
	}
	if apk != "" {
		st, err := os.Stat(apk)
		if err != nil {
			return err
		}
		if st.Size() <= MaxDocumentSize {
			return b.sendBuildFile(ctx, chat, apk, bi)
		}
		if link == "" {
			return fmt.Errorf("APK %d МБ больше лимита Telegram 50 МБ, а ссылки нет", st.Size()>>20)
		}
	}
	return b.Send(ctx, chat, buildCaption(bi, link))
}

// sendBuildFile — APK файлом; длинная подпись — следом сообщением
// (требование 16б).
func (b *Bot) sendBuildFile(ctx context.Context, chat int64, apk string, bi buildInfo) error {
	name := fmt.Sprintf("moya-dacha-%d.apk", bi.Build)
	text := buildCaption(bi, "")
	if len(utf16.Encode([]rune(text))) <= maxCaption {
		return b.sendDocument(ctx, chat, apk, name, text)
	}
	first, _, _ := strings.Cut(text, "\n")
	if err := b.sendDocument(ctx, chat, apk, name, first); err != nil {
		return err
	}
	return b.Send(ctx, chat, text)
}

func buildCaption(bi buildInfo, link string) string {
	var s strings.Builder
	fmt.Fprintf(&s, "МояДача %s, сборка %d", bi.Version, bi.Build)
	if len(bi.WhatsNew) > 0 {
		s.WriteString("\nЧто нового:")
		for _, line := range bi.WhatsNew {
			s.WriteString("\n• " + line)
		}
	}
	if link != "" {
		s.WriteString("\n\nСкачать: " + link)
	}
	return s.String()
}
