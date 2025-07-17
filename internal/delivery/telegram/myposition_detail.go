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
	msgRoot := c.Message()
	shouldSendNewMessage := msgRoot == nil || msgRoot.Sender == nil || !msgRoot.Sender.IsBot

	isHasMonitoring := len(stockPosition.StockPositionMonitorings) > 0

	stockCodeWithExchange := stockPosition.Exchange + ":" + stockPosition.StockCode
	marketPrice, _ := cache.GetFromCache[float64](fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange))

	if marketPrice == 0 && isHasMonitoring {
		marketPrice = stockPosition.StockPositionMonitorings[0].MarketPrice
	}

	ageDays := utils.DaysSince(stockPosition.BuyDate)

	sb.WriteString(fmt.Sprintf("<b>📌 Detail Posisi Saham %s</b>\n", stockCodeWithExchange))
	sb.WriteString("\n")
	sb.WriteString("<b>🧾 Informasi Posisi:</b>\n")
	sb.WriteString(fmt.Sprintf("  • Buy: %s (%d Hari)\n", stockPosition.BuyDate.Format("02/01 15:04"), ageDays))
	sb.WriteString(fmt.Sprintf("  • Entry: %.2f \n", stockPosition.BuyPrice))
	sb.WriteString(fmt.Sprintf("  • Last Price: %.2f\n", marketPrice))
	sb.WriteString(fmt.Sprintf("  • PnL: %s\n", utils.FormatChange(stockPosition.BuyPrice, marketPrice)))
	if stockPosition.TrailingProfitPrice > 0 {
		sb.WriteString(fmt.Sprintf("  • TP: %.2f ➡️ %.2f (%s)\n", stockPosition.TakeProfitPrice, stockPosition.TrailingProfitPrice, utils.FormatChange(stockPosition.BuyPrice, stockPosition.TrailingProfitPrice)))
	} else {
		sb.WriteString(fmt.Sprintf("  • TP: %.2f (%s)\n", stockPosition.TakeProfitPrice, utils.FormatChange(stockPosition.BuyPrice, stockPosition.TakeProfitPrice)))
	}
	if stockPosition.TrailingStopPrice > 0 {
		sb.WriteString(fmt.Sprintf("  • SL: %.2f ➡️ %.2f (%s)\n", stockPosition.StopLossPrice, stockPosition.TrailingStopPrice, utils.FormatChange(stockPosition.BuyPrice, stockPosition.TrailingStopPrice)))
	} else {
		sb.WriteString(fmt.Sprintf("  • SL: %.2f (%s)\n", stockPosition.StopLossPrice, utils.FormatChange(stockPosition.BuyPrice, stockPosition.StopLossPrice)))
	}

	menu := &telebot.ReplyMarkup{}

	btnBack := menu.Data(btnBackStockPosition.Text, btnBackStockPosition.Unique)
	btnExit := menu.Data("📤 Keluar dari Posisi", btnExitStockPosition.Unique, fmt.Sprintf("%s|%d", stockCodeWithExchange, stockPosition.ID))
	btnDelete := menu.Data("🗑 Hapus Posisi", btnDeleteStockPosition.Unique, fmt.Sprintf("%d", stockPosition.ID))

	menu.Inline(menu.Row(btnExit, btnDelete), menu.Row(btnBack))

	if !isHasMonitoring {
		sb.WriteString("\n\n<i>⚠️ Belum ada monitoring</i>")
		if shouldSendNewMessage {
			_, err := t.telegram.Send(ctx, c, sb.String(), menu, telebot.ModeHTML)
			return err
		} else {
			_, err := t.telegram.Edit(ctx, c, msgRoot, sb.String(), menu, telebot.ModeHTML)
			return err
		}
	}

	lastMonitoring := stockPosition.StockPositionMonitorings[0]

	var evalSummary model.PositionAnalysisSummary
	err := json.Unmarshal(lastMonitoring.EvaluationSummary, &evalSummary)
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}
	sb.WriteString("\n")
	sb.WriteString("<b>📊 Evaluasi Terbaru</b>\n")
	sb.WriteString(fmt.Sprintf("  • Score: %.2f (TA)\n", evalSummary.TechnicalAnalysis.Score))
	sb.WriteString(fmt.Sprintf("  • Signal: %s (TA)\n", dto.Signal(evalSummary.PositionSignal).String()))
	sb.WriteString(fmt.Sprintf("  • Status: %s (TA)\n", dto.PositionStatus(evalSummary.TechnicalAnalysis.Status).String()))
	sb.WriteString(fmt.Sprintf("  • TA Signal: %s\n", evalSummary.TechnicalAnalysis.Signal))

	sb.WriteString("\n")
	sb.WriteString("<b>🧠 Insight</b>\n")
	for _, insight := range evalSummary.TechnicalAnalysis.Insight {
		sb.WriteString(fmt.Sprintf("- %s\n", utils.EscapeHTMLForTelegram(insight)))
	}

	if len(stockPosition.StockPositionMonitorings) > 0 && len(stockPosition.StockPositionMonitorings[0].StockPositionMonitoringAnalysisRefs) > 0 {
		sb.WriteString("\n")
		sb.WriteString("📊 <b><i>Rangkuman Analisis (Multi-Timeframe)</i></b>\n")

		latestAnalyses := stockPosition.StockPositionMonitorings[0].StockPositionMonitoringAnalysisRefs
		for _, refAnalysis := range latestAnalyses {
			analysis := refAnalysis.StockAnalysis
			var (
				technicalData dto.TradingViewScanner
				ohclv         []dto.StockOHLCV
			)
			if err := json.Unmarshal([]byte(analysis.TechnicalData), &technicalData); err != nil {
				return err
			}

			if err := json.Unmarshal([]byte(analysis.OHLCV), &ohclv); err != nil {
				return err
			}

			if len(ohclv) == 0 {
				_, err := t.telegram.Send(ctx, c, "❌ Tidak ada data harga")
				return err
			}
			valTimeframeSummary := "??"
			switch technicalData.Recommend.Global.Summary {
			case dto.TradingViewSignalStrongBuy:
				valTimeframeSummary = fmt.Sprintf("🟢 Timeframe %s - Strong Buy", analysis.Timeframe)
			case dto.TradingViewSignalBuy:
				valTimeframeSummary = fmt.Sprintf("🟢 Timeframe %s - Buy", analysis.Timeframe)
			case dto.TradingViewSignalNeutral:
				valTimeframeSummary = fmt.Sprintf("🟡 Timeframe %s - Neutral", analysis.Timeframe)
			case dto.TradingViewSignalSell:
				valTimeframeSummary = fmt.Sprintf("🔴 Timeframe %s - Sell", analysis.Timeframe)
			case dto.TradingViewSignalStrongSell:
				valTimeframeSummary = fmt.Sprintf("🔴 Timeframe %s - Strong Sell", analysis.Timeframe)
			}

			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf("<b>%s</b>\n", valTimeframeSummary))
			sb.WriteString(fmt.Sprintf("- <b>Close</b>: %.2f (%s) | <b>Vol</b>: %s\n", ohclv[len(ohclv)-1].Close, utils.FormatChange(ohclv[len(ohclv)-1].Open, ohclv[len(ohclv)-1].Close), utils.FormatVolume(ohclv[len(ohclv)-1].Volume)))
			sb.WriteString(fmt.Sprintf("- <b>MACD</b>: %s | <b>RSI</b>: %d - %s\n", technicalData.GetTrendMACD(), int(technicalData.Value.Oscillators.RSI), dto.GetRSIStatus(int(technicalData.Value.Oscillators.RSI))))
			sb.WriteString(fmt.Sprintf("- <b>MA</b>: %s | <b>Osc</b>: %s \n", dto.GetSignalText(technicalData.Recommend.Global.MA), dto.GetSignalText(technicalData.Recommend.Global.Oscillators)))

		}
	}

	sb.WriteString("\n")
	sb.WriteString("<b>📜 Riwayat Evaluasi</b>\n")
	for _, stockPositionMonitoring := range stockPosition.StockPositionMonitorings {
		var evalSummary model.PositionAnalysisSummary
		err := json.Unmarshal(stockPositionMonitoring.EvaluationSummary, &evalSummary)
		if err != nil {
			continue
		}

		sb.WriteString(fmt.Sprintf("\n<b>(%s) - %.2f %s</b>\n",
			stockPositionMonitoring.Timestamp.Format("02/01 15:04"),
			stockPositionMonitoring.MarketPrice,
			utils.FormatChangeWithIcon(stockPosition.BuyPrice, float64(stockPositionMonitoring.MarketPrice)),
		))
		sb.WriteString(fmt.Sprintf("Signal: %s\n", dto.Signal(evalSummary.PositionSignal)))

		if evalSummary.PositionSignal == string(dto.TrailingStop) {
			sb.WriteString(fmt.Sprintf("Chg: %.2f ➡️ %.2f\n", stockPosition.StopLossPrice, stockPosition.TrailingStopPrice))

		} else if evalSummary.PositionSignal == string(dto.TrailingProfit) {
			sb.WriteString(fmt.Sprintf("Chg: %.2f ➡️ %.2f\n", stockPosition.TakeProfitPrice, stockPosition.TrailingProfitPrice))
		}
		sb.WriteString(fmt.Sprintf("Status: %s\n", dto.PositionStatus(evalSummary.TechnicalAnalysis.Status)))

		sb.WriteString(fmt.Sprintf("Osc: %s | RSI: %s\n", evalSummary.TechnicalAnalysis.IndicatorSummary.Osc, evalSummary.TechnicalAnalysis.IndicatorSummary.RSI))
		sb.WriteString(fmt.Sprintf("MA: %s | MACD: %s\n", evalSummary.TechnicalAnalysis.IndicatorSummary.MA, evalSummary.TechnicalAnalysis.IndicatorSummary.MACD))
		sb.WriteString(fmt.Sprintf("Vol: %s | Score: %.2f (%s)\n", evalSummary.TechnicalAnalysis.IndicatorSummary.Volume, evalSummary.TechnicalAnalysis.Score, evalSummary.TechnicalAnalysis.Signal))

	}
	lastUpdate := lastMonitoring.Timestamp
	sb.WriteString(fmt.Sprintf("\n\n📅 Update Terakhir: %s", utils.PrettyDate(lastUpdate)))

	if shouldSendNewMessage {
		_, err = t.telegram.Send(ctx, c, sb.String(), menu, telebot.ModeHTML)
		return err
	} else {
		_, err = t.telegram.Edit(ctx, c, msgRoot, sb.String(), menu, telebot.ModeHTML)
		return err
	}
}
