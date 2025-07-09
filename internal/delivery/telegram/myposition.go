package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"golang-trading/pkg/utils"
	"strings"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleMyPosition(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID

	positions, err := t.service.TelegramBotService.GetStockPositions(ctx, dto.GetStockPositionsParam{
		TelegramID: &userID,
		IsActive:   utils.ToPointer(true),
	})
	if err != nil {
		t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	if len(positions) == 0 {
		t.telegram.Send(ctx, c, "❌ Tidak ada saham aktif yang kamu set position saat ini.")
		return nil
	}

	return t.showMyPosition(ctx, c, positions)
}

func (t *TelegramBotHandler) showMyPosition(ctx context.Context, c telebot.Context, positions []model.StockPosition) error {
	sb := strings.Builder{}
	header := `📊 Posisi Saham yang Kamu Pantau Saat ini:`
	sb.WriteString(header)
	sb.WriteString("\n")

	for _, position := range positions {
		sb.WriteString(position.StockCode)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("\n• %s", position.StockCode))
		sb.WriteString(fmt.Sprintf("\n 🎯 Buy: %d | TP: %d | SL: %d\n", int(position.BuyPrice), int(position.TakeProfitPrice), int(position.StopLossPrice)))

		stockCodeWithExchange := position.Exchange + ":" + position.StockCode
		marketPrice, _ := cache.GetFromCache[int](fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange))
		if marketPrice == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf(" 💰 Last Price: %d | 📈 PnL: (%s)\n", int(marketPrice), utils.FormatChange(float64(position.BuyPrice), float64(marketPrice))))
	}

	sb.WriteString("\n👉 Tekan tombol di bawah untuk melihat detail lengkap atau mengelola posisi.")
	menu := &telebot.ReplyMarkup{}
	rows := []telebot.Row{}
	var tempRow []telebot.Btn

	for _, position := range positions {
		btn := menu.Data(position.StockCode, btnToDetailStockPosition.Unique, fmt.Sprintf("%d", position.ID))
		tempRow = append(tempRow, btn)
		if len(tempRow) == 2 {
			rows = append(rows, menu.Row(tempRow...))
			tempRow = []telebot.Btn{}
		}
	}

	btnDelete := menu.Data("🗑 Hapus Pesan", btnDeleteMessage.Unique)

	if len(tempRow) > 0 {
		tempRow = append(tempRow, btnDelete)
		rows = append(rows, menu.Row(tempRow...))
	} else {
		rows = append(rows, menu.Row(btnDelete))
	}

	menu.Inline(rows...)
	_, err := t.telegram.Send(ctx, c, sb.String(), menu, telebot.ModeHTML)
	if err != nil {
		return err
	}
	return nil
}
