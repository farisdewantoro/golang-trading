package telegram

import (
	"context"
	"fmt"
	"golang-trading/pkg/logger"
	"strconv"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleAlertSignal(ctx context.Context, c telebot.Context) error {
	telegramID := c.Sender().ID

	alertSignals, err := t.service.TelegramBotService.GetAlertSignal(ctx, telegramID)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to get alert signal", logger.ErrorField(err))
		t.telegram.Send(ctx, c, commonErrorInternal)
		return err
	}

	sb := strings.Builder{}
	sb.WriteString("<b>📡 Pengaturan Alert Signal Trading:</b>\n")
	sb.WriteString("\n")

	sb.WriteString("Jika <b>ON</b>, kamu akan menerima notifikasi sinyal BUY terbaik yang dikirim otomatis sesuai jadwal oleh sistem (berdasarkan analisa terkuat).\n")
	sb.WriteString("\n")

	for idx, alertSignal := range alertSignals {
		textIsActive := "✅ ON"
		if alertSignal.IsActive != nil && !*alertSignal.IsActive {
			textIsActive = "❌ OFF"
		}
		sb.WriteString(fmt.Sprintf("%d. <b>%s - %s</b>\n", idx+1, alertSignal.Exchange, textIsActive))
	}

	sb.WriteString("\n")
	sb.WriteString("👉 Klik tombol dibawah ini untuk mengaktifkan atau menonaktifkan alert:\n")
	sb.WriteString("\n")

	menu := &telebot.ReplyMarkup{}
	rows := []telebot.Row{}
	var tempRow []telebot.Btn

	for _, alertSignal := range alertSignals {
		textBtn := alertSignal.Exchange
		if alertSignal.IsActive != nil && !*alertSignal.IsActive {
			textBtn = fmt.Sprintf("%s - %s", alertSignal.Exchange, "✅ ON")
		} else {
			textBtn = fmt.Sprintf("%s - %s", alertSignal.Exchange, "❌ OFF")
		}
		tempRow = append(tempRow, menu.Data(textBtn, btnAlertSignal.Unique, fmt.Sprintf("%s:%t", alertSignal.Exchange, !*alertSignal.IsActive)))
		if len(tempRow) == 2 {
			rows = append(rows, menu.Row(tempRow...))
			tempRow = []telebot.Btn{}
		}
	}

	if len(tempRow) > 0 {
		tempRow = append(tempRow, btnDeleteMessage)
		rows = append(rows, menu.Row(tempRow...))
	} else {
		rows = append(rows, menu.Row(btnDeleteMessage))
	}

	menu.Inline(rows...)

	_, err = t.telegram.Send(ctx, c, sb.String(), menu, telebot.ModeHTML)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to send alert signal", logger.ErrorField(err))
		t.telegram.Send(ctx, c, commonErrorInternal)
		return err
	}

	return nil
}

func (t *TelegramBotHandler) handleBtnAlertSignal(ctx context.Context, c telebot.Context) error {
	data := strings.Split(c.Data(), ":")
	exchange := data[0]
	isActive, err := strconv.ParseBool(data[1])
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to parse alert signal", logger.ErrorField(err))
		t.telegram.Send(ctx, c, commonErrorInternal)
		return err
	}

	telegramID := c.Sender().ID

	err = t.service.TelegramBotService.SetAlertSignal(ctx, telegramID, exchange, isActive)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to set alert signal", logger.ErrorField(err))
		t.telegram.Send(ctx, c, commonErrorInternal)
		return err
	}

	text := fmt.Sprintf("✅ Alert signal untuk %s telah", exchange)
	if !isActive {
		text += " <b>dinonaktifkan</b>"
	} else {
		text += " <b>diaktifkan</b>"
	}

	t.telegram.Edit(ctx, c, c.Message(), text, telebot.ModeHTML)
	time.Sleep(300 * time.Millisecond)
	t.telegram.Delete(ctx, c, c.Message())
	return t.handleAlertSignal(ctx, c)
}
