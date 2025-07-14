package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/pkg/common"
	"net/http"

	"go.uber.org/zap/zapcore"
)

type AlertCore struct {
	cfg      *config.Config
	core     zapcore.Core
	minLevel zapcore.Level
}

func (a *AlertCore) Enabled(lvl zapcore.Level) bool {
	return a.core.Enabled(lvl)
}

func (a *AlertCore) With(fields []zapcore.Field) zapcore.Core {
	return &AlertCore{
		core:     a.core.With(fields),
		minLevel: a.minLevel,
	}
}

func (a *AlertCore) Check(entry zapcore.Entry, checkedEntry *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if a.Enabled(entry.Level) {
		return a.core.Check(entry, checkedEntry).AddCore(entry, a)
	}
	return checkedEntry
}

func (a *AlertCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	shouldSend := false
	for _, f := range fields {
		if f.Key == common.KEY_LOG_HOOK_SEND_ALERT && f.Type == zapcore.BoolType && f.Integer == 1 {
			shouldSend = true
			break
		}
	}
	if entry.Level >= a.minLevel && shouldSend {
		go a.sendTelegramAlert(entry, fields) // async biar tidak blocking
	}
	return a.core.Write(entry, fields)
}

func (a *AlertCore) Sync() error {
	return a.core.Sync()
}

func (a *AlertCore) sendTelegramAlert(entry zapcore.Entry, fields []zapcore.Field) {
	// Gunakan encoder untuk extract fields
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}

	// Format semua field
	fieldStr := ""
	for k, v := range enc.Fields {
		fieldStr += fmt.Sprintf("• %s: %v\n", k, v)
	}

	// Format waktu
	timestamp := entry.Time.Format("2006-01-02 15:04:05")

	// Final message
	message := fmt.Sprintf(
		"🚨 *%s Alert*\n\n*Message:* %s\n\n*Fields:*\n%s\n*Time:* %s",
		entry.Level.CapitalString(),
		entry.Message,
		fieldStr,
		timestamp,
	)

	// Kirim ke Telegram
	token := a.cfg.Telegram.BotToken
	chatID := a.cfg.Telegram.ChatID
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "Markdown",
	}

	jsonBody, _ := json.Marshal(payload)
	http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
}
