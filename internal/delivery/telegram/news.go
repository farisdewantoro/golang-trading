package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/service"
	"golang-trading/internal/strategy"
	"golang-trading/pkg/common"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/telegram"
	"golang-trading/pkg/utils"
	"strings"

	"gopkg.in/telebot.v3"
)

const newsUsage = `<b>📰 Pantau Berita Saham</b>

Bot akan mengirim berita baru secara otomatis untuk saham di watchlist berita kamu <b>dan</b> saham yang ada di posisi aktif kamu (/myposition).

<b>Perintah:</b>
• <code>/news BBCA</code> - lihat berita terbaru BBCA
• <code>/watchnews BBCA TLKM</code> - pantau berita saham
• <code>/unwatchnews BBCA</code> - berhenti pantau berita`

// parseNewsArgs turns command args like "BBCA", "idx:tlkm" into IDX stock codes.
func parseNewsArgs(args []string) ([]string, []string) {
	var valid, invalid []string
	seen := map[string]bool{}
	for _, arg := range args {
		for _, raw := range strings.Split(arg, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			if idx := strings.Index(raw, ":"); idx >= 0 {
				if !strings.EqualFold(raw[:idx], common.EXCHANGE_IDX) {
					invalid = append(invalid, raw)
					continue
				}
				raw = raw[idx+1:]
			}
			code, err := service.NormalizeStockCode(raw)
			if err != nil {
				invalid = append(invalid, raw)
				continue
			}
			if !seen[code] {
				seen[code] = true
				valid = append(valid, code)
			}
		}
	}
	return valid, invalid
}

func (t *TelegramBotHandler) handleNews(ctx context.Context, c telebot.Context) error {
	codes, _ := parseNewsArgs(c.Args())
	if len(codes) == 0 {
		return t.sendNewsWatchlist(ctx, c)
	}
	return t.sendLatestNews(ctx, c, codes[0])
}

func (t *TelegramBotHandler) sendNewsWatchlist(ctx context.Context, c telebot.Context) error {
	watchlist, err := t.service.NewsService.GetWatchlist(ctx, c.Sender().ID)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to get news watchlist", logger.ErrorField(err))
		t.telegram.Send(ctx, c, commonErrorInternal)
		return err
	}

	sb := strings.Builder{}
	sb.WriteString(newsUsage)
	sb.WriteString("\n\n<b>📋 Watchlist berita kamu:</b>\n")

	menu := &telebot.ReplyMarkup{}
	rows := []telebot.Row{}
	if len(watchlist) == 0 {
		sb.WriteString("<i>Belum ada. Contoh: /watchnews BBCA</i>\n")
	}
	for i, w := range watchlist {
		sb.WriteString(fmt.Sprintf("%d. %s:%s\n", i+1, w.Exchange, w.StockCode))
		rows = append(rows, menu.Row(
			menu.Data(fmt.Sprintf("📰 %s", w.StockCode), btnShowNews.Unique, fmt.Sprintf("%s:%s", w.Exchange, w.StockCode)),
			menu.Data(fmt.Sprintf("🔕 Stop %s", w.StockCode), btnUnwatchNews.Unique, fmt.Sprintf("%s:%s", w.Exchange, w.StockCode)),
		))
	}
	rows = append(rows, menu.Row(btnDeleteMessage))
	menu.Inline(rows...)

	_, err = t.telegram.Send(ctx, c, sb.String(), menu, telebot.ModeHTML)
	return err
}

func (t *TelegramBotHandler) sendLatestNews(ctx context.Context, c telebot.Context, stockCode string) error {
	items, err := t.service.NewsService.GetLatestNews(ctx, stockCode)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to get latest news", logger.ErrorField(err), logger.StringField("stock_code", stockCode))
		t.telegram.Send(ctx, c, "⚠️ Gagal mengambil berita. Coba lagi nanti.")
		return err
	}

	menu := &telebot.ReplyMarkup{}
	menu.Inline(menu.Row(
		menu.Data("🔔 Pantau Berita", btnWatchNews.Unique, fmt.Sprintf("%s:%s", common.EXCHANGE_IDX, stockCode)),
		btnDeleteMessage,
	))

	if len(items) == 0 {
		_, err = t.telegram.Send(ctx, c,
			fmt.Sprintf("📭 Belum ada berita untuk <b>%s</b> dalam 7 hari terakhir.", utils.EscapeHTMLForTelegram(stockCode)),
			menu, telebot.ModeHTML)
		return err
	}

	message := telegram.FormatStockNewsForTelegram("📰 Berita", stockCode, strategy.ToNewsLines(items))
	_, err = t.telegram.Send(ctx, c, message, menu, telebot.ModeHTML, telebot.NoPreview)
	return err
}

func (t *TelegramBotHandler) handleWatchNews(ctx context.Context, c telebot.Context) error {
	codes, invalid := parseNewsArgs(c.Args())
	if len(codes) == 0 {
		_, err := t.telegram.Send(ctx, c, "ℹ️ Format: <code>/watchnews BBCA</code> (bisa lebih dari satu, contoh: <code>/watchnews BBCA TLKM</code>)", telebot.ModeHTML)
		return err
	}

	userTelegram := dto.ToRequestUserTelegram(c.Sender())
	var ok []string
	for _, code := range codes {
		if err := t.service.NewsService.WatchNews(ctx, userTelegram, common.EXCHANGE_IDX, code); err != nil {
			t.log.ErrorContext(ctx, "Failed to watch news", logger.ErrorField(err), logger.StringField("stock_code", code))
			invalid = append(invalid, code)
			continue
		}
		ok = append(ok, code)
	}

	sb := strings.Builder{}
	if len(ok) > 0 {
		sb.WriteString(fmt.Sprintf("🔔 Berita untuk <b>%s</b> sekarang dipantau. Kamu akan dapat notifikasi saat ada berita baru.\n", strings.Join(ok, ", ")))
	}
	if len(invalid) > 0 {
		sb.WriteString(fmt.Sprintf("⚠️ Gagal / tidak valid: %s\n", utils.EscapeHTMLForTelegram(strings.Join(invalid, ", "))))
	}
	_, err := t.telegram.Send(ctx, c, sb.String(), telebot.ModeHTML)
	return err
}

func (t *TelegramBotHandler) handleUnwatchNews(ctx context.Context, c telebot.Context) error {
	codes, _ := parseNewsArgs(c.Args())
	if len(codes) == 0 {
		_, err := t.telegram.Send(ctx, c, "ℹ️ Format: <code>/unwatchnews BBCA</code>", telebot.ModeHTML)
		return err
	}

	for _, code := range codes {
		if err := t.service.NewsService.UnwatchNews(ctx, c.Sender().ID, common.EXCHANGE_IDX, code); err != nil {
			t.log.ErrorContext(ctx, "Failed to unwatch news", logger.ErrorField(err), logger.StringField("stock_code", code))
			t.telegram.Send(ctx, c, commonErrorInternal)
			return err
		}
	}

	_, err := t.telegram.Send(ctx, c, fmt.Sprintf("🔕 Berhenti memantau berita <b>%s</b>.\n<i>Catatan: berita saham di posisi aktif tetap dikirim selama posisinya masih terbuka.</i>", strings.Join(codes, ", ")), telebot.ModeHTML)
	return err
}

// splitNewsCallbackData parses "IDX:BBCA" from inline button data.
func splitNewsCallbackData(data string) (exchange, stockCode string, err error) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid news callback data: %q", data)
	}
	return parts[0], parts[1], nil
}

func (t *TelegramBotHandler) handleBtnWatchNews(ctx context.Context, c telebot.Context) error {
	exchange, code, err := splitNewsCallbackData(c.Data())
	if err != nil {
		return t.telegram.Respond(ctx, c, &telebot.CallbackResponse{Text: "Data tidak valid"})
	}
	if err := t.service.NewsService.WatchNews(ctx, dto.ToRequestUserTelegram(c.Sender()), exchange, code); err != nil {
		t.log.ErrorContext(ctx, "Failed to watch news", logger.ErrorField(err))
		return t.telegram.Respond(ctx, c, &telebot.CallbackResponse{Text: "⚠️ Gagal memantau berita"})
	}
	return t.telegram.Respond(ctx, c, &telebot.CallbackResponse{Text: fmt.Sprintf("🔔 Berita %s dipantau", code)})
}

func (t *TelegramBotHandler) handleBtnUnwatchNews(ctx context.Context, c telebot.Context) error {
	exchange, code, err := splitNewsCallbackData(c.Data())
	if err != nil {
		return t.telegram.Respond(ctx, c, &telebot.CallbackResponse{Text: "Data tidak valid"})
	}
	if err := t.service.NewsService.UnwatchNews(ctx, c.Sender().ID, exchange, code); err != nil {
		t.log.ErrorContext(ctx, "Failed to unwatch news", logger.ErrorField(err))
		return t.telegram.Respond(ctx, c, &telebot.CallbackResponse{Text: "⚠️ Gagal berhenti memantau"})
	}
	return t.telegram.Respond(ctx, c, &telebot.CallbackResponse{Text: fmt.Sprintf("🔕 Berhenti pantau berita %s", code)})
}

func (t *TelegramBotHandler) handleBtnShowNews(ctx context.Context, c telebot.Context) error {
	_, code, err := splitNewsCallbackData(c.Data())
	if err != nil {
		return t.telegram.Respond(ctx, c, &telebot.CallbackResponse{Text: "Data tidak valid"})
	}
	_ = t.telegram.Respond(ctx, c, &telebot.CallbackResponse{Text: "⏳ Mengambil berita..."})
	return t.sendLatestNews(ctx, c, code)
}
