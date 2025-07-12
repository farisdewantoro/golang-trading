package telegram

import (
	"context"
	"encoding/json"
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
		Monitoring: &dto.StockPositionMonitoringQueryParam{
			ShowNewest: utils.ToPointer(true),
		},
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
	sb.WriteString("\n\n")

	for idx, position := range positions {
		stockWithExchange := position.Exchange + ":" + position.StockCode
		sb.WriteString(fmt.Sprintf("<b>%d. %s</b>\n", idx+1, stockWithExchange))

		sb.WriteString(fmt.Sprintf("  • Entry: %.2f\n", position.BuyPrice))
		sb.WriteString(fmt.Sprintf("  • TP: %.2f (%s)\n", position.TakeProfitPrice, utils.FormatChange(position.BuyPrice, position.TakeProfitPrice)))
		sb.WriteString(fmt.Sprintf("  • SL: %.2f (%s)\n", position.StopLossPrice, utils.FormatChange(position.BuyPrice, position.StopLossPrice)))
		// sb.WriteString(fmt.Sprintf("\n 🎯 Buy: %d | TP: %d | SL: %d\n", int(position.BuyPrice), int(position.TakeProfitPrice), int(position.StopLossPrice)))

		var (
			marketPrice        float64
			techScore          string
			techRecommendation string
			pnl                string
		)

		isHasMonitoring := len(position.StockPositionMonitorings) > 0

		stockCodeWithExchange := position.Exchange + ":" + position.StockCode
		marketPrice, _ = cache.GetFromCache[float64](fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange))

		if marketPrice == 0 && isHasMonitoring {
			marketPrice = position.StockPositionMonitorings[0].MarketPrice
		}

		pnl = "N/A"
		if marketPrice > 0 {
			pnl = utils.FormatChange(position.BuyPrice, marketPrice)
		}
		sb.WriteString(fmt.Sprintf("  • Last Price: %.2f | PnL: (%s)\n", marketPrice, pnl))

		if !isHasMonitoring {
			techScore = "N/A"
			techRecommendation = "N/A"
		} else {

			var evalSummary model.EvaluationSummaryData

			err := json.Unmarshal(position.StockPositionMonitorings[0].EvaluationSummary, &evalSummary)
			if err != nil {
				continue
			}
			techScore = fmt.Sprintf("%d", evalSummary.TechnicalScore)
			techRecommendation = evalSummary.TechnicalRecommendation
		}

		sb.WriteString(fmt.Sprintf("  • Tech Status: %s\n", techRecommendation))
		sb.WriteString(fmt.Sprintf("  • Tech Score: %s\n", techScore))
		sb.WriteString("\n")
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

func (t *TelegramBotHandler) handleBtnBackStockPosition(ctx context.Context, c telebot.Context) error {
	return t.handleMyPosition(ctx, c)
}
