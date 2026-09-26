package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// buildInfo — сведения о сборке (specs/017-app-updates.md).
type buildInfo struct {
	Version  string   `json:"version"`
	Build    int64    `json:"build"`
	WhatsNew []string `json:"whatsNew"`
}

// SendBuild отправляет сборку в чат роли: файлом, если задан apk, иначе
// текстом со ссылкой (требования 16–17). Не привязанный чат —
// ErrNotBound.
func (b *Bot) SendBuild(ctx context.Context, role string, info []byte, apk, link string) error {
	var bi buildInfo
	if err := json.Unmarshal(info, &bi); err != nil {
		return fmt.Errorf("сведения о сборке: %w", err)
	}
	chat, err := b.chat(ctx, role)
	if err != nil {
		return err
	}
	text := buildCaption(bi, link)
	if apk != "" {
		return b.sendDocument(ctx, chat, apk, fmt.Sprintf("moya-dacha-%d.apk", bi.Build), text)
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
