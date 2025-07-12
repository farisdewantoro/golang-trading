package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/internal/model"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"golang-trading/pkg/utils"
	"strconv"
	"strings"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleBtnToDetailStockPosition(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID
	id, err := strconv.Atoi(c.Data())

	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	stockPosition, err := t.service.TelegramBotService.GetDetailStockPosition(ctx, userID, uint(id))
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	if stockPosition == nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	return t.showMyPositionDetail(ctx, c, stockPosition)
}

func (t *TelegramBotHandler) showMyPositionDetail(ctx context.Context, c telebot.Context, stockPosition *model.StockPosition) error {
	sb := strings.Builder{}

	isHasMonitoring := len(stockPosition.StockPositionMonitorings) > 0

	stockCodeWithExchange := stockPosition.Exchange + ":" + stockPosition.StockCode
	marketPrice, _ := cache.GetFromCache[float64](fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange))

	if marketPrice == 0 && isHasMonitoring {
		marketPrice = stockPosition.StockPositionMonitorings[0].MarketPrice
	}

	sb.WriteString(fmt.Sprintf("<b>📌 Detail Posisi Saham %s</b>\n", stockCodeWithExchange))
	sb.WriteString("\n")
	sb.WriteString("<b>🧾 Informasi Posisi:</b>\n")
	sb.WriteString(fmt.Sprintf("  • Entry: %.2f \n", stockPosition.BuyPrice))
	sb.WriteString(fmt.Sprintf("  • Last Price: %.2f\n", marketPrice))
	sb.WriteString(fmt.Sprintf("  • PnL: %s\n", utils.FormatChange(stockPosition.BuyPrice, marketPrice)))
	sb.WriteString(fmt.Sprintf("  • TP: %.2f (%s)\n", stockPosition.TakeProfitPrice, utils.FormatChange(stockPosition.BuyPrice, stockPosition.TakeProfitPrice)))
	sb.WriteString(fmt.Sprintf("  • SL: %.2f (%s)\n", stockPosition.StopLossPrice, utils.FormatChange(stockPosition.BuyPrice, stockPosition.StopLossPrice)))

	menu := &telebot.ReplyMarkup{}

	btnBack := menu.Data(btnBackStockPosition.Text, btnBackStockPosition.Unique)
	btnExit := menu.Data("🚪 Exit dari Posisi", btnExitStockPosition.Unique, fmt.Sprintf("%s|%d", stockCodeWithExchange, stockPosition.ID))
	btnDelete := menu.Data("🗑 Hapus Posisi", btnDeleteStockPosition.Unique, fmt.Sprintf("%d", stockPosition.ID))

	menu.Inline(menu.Row(btnExit, btnDelete), menu.Row(btnBack))

	if !isHasMonitoring {
		sb.WriteString("\n\n<i>⚠️ Belum ada monitoring</i>")
		_, err := t.telegram.Send(ctx, c, sb.String(), menu, telebot.ModeHTML)
		return err
	}

	lastMonitoring := stockPosition.StockPositionMonitorings[len(stockPosition.StockPositionMonitorings)-1]

	var evalSummary model.EvaluationSummaryData
	err := json.Unmarshal(lastMonitoring.EvaluationSummary, &evalSummary)
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}
	sb.WriteString("\n")
	sb.WriteString("<b>📊 Evaluasi Terbaru:</b>\n")
	sb.WriteString(fmt.Sprintf("  • Skor Technical: %d\n", evalSummary.TechnicalScore))
	sb.WriteString(fmt.Sprintf("  • Status Technical: %s\n", evalSummary.TechnicalRecommendation))

	sb.WriteString("\n")
	sb.WriteString("<b>📜 Riwayat Evaluasi:</b>\n")
	for _, stockPositionMonitoring := range stockPosition.StockPositionMonitorings {
		var evalSummary model.EvaluationSummaryData
		err := json.Unmarshal(stockPositionMonitoring.EvaluationSummary, &evalSummary)
		if err != nil {
			continue
		}

		sb.WriteString(fmt.Sprintf("  <b>• %s</b>: %s\n",
			stockPositionMonitoring.Timestamp.Format("02 Jan 15:04"),
			evalSummary.TechnicalRecommendation))
		sb.WriteString(fmt.Sprintf("  ↳ Market Price: %.2f (%s)\n", stockPositionMonitoring.MarketPrice, utils.FormatChange(stockPosition.BuyPrice, float64(stockPositionMonitoring.MarketPrice))))
	}

	lastUpdate := lastMonitoring.Timestamp
	sb.WriteString(fmt.Sprintf("\n\n📅 Update Terakhir: %s", utils.PrettyDate(lastUpdate)))

	_, err = t.telegram.Send(ctx, c, sb.String(), menu, telebot.ModeHTML)
	return err
}
