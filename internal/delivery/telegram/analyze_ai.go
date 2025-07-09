package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strings"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleAskAIAnalyzer(ctx context.Context, c telebot.Context) error {
	markup := &telebot.ReplyMarkup{}

	_, err := t.telegram.Edit(ctx, c, c.Message(), markup, telebot.ModeHTML)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to edit message", logger.ErrorField(err))
		return err
	}

	stopChan := make(chan struct{})

	msg := t.showLoadingFlowAnalysis(c, stopChan, true)

	utils.GoSafe(func() {

		newCtx, cancel := context.WithTimeout(t.ctx, t.cfg.Telegram.TimeoutAsyncDuration)
		defer cancel()

		analysis, err := t.service.TelegramBotService.AnalyzeStockAI(newCtx, c)
		if err != nil {
			close(stopChan)
			t.log.ErrorContext(ctx, "Failed to AI analyze stock", logger.ErrorField(err))

			// Send error message
			_, err = t.telegram.Edit(newCtx, c, msg, fmt.Sprintf("❌ Failed to AI analyze stock: %s", err.Error()))
			if err != nil {
				t.log.ErrorContext(newCtx, "Failed to send error message", logger.ErrorField(err))
			}
			return
		}

		close(stopChan)

		err = t.telegram.Delete(newCtx, c, msg)
		if err != nil {
			t.log.ErrorContext(newCtx, "Failed to delete loading message", logger.ErrorField(err))
			return
		}

		err = t.showAnalysisAI(newCtx, c, analysis)
		if err != nil {
			t.log.ErrorContext(newCtx, "Failed to show analysis AI", logger.ErrorField(err))
			return
		}
	}).OnPanic(func(err interface{}) {
		t.log.ErrorContext(ctx, "panic when AI analyze stock")
		close(stopChan)
	}).Run()

	return nil
}

func (t *TelegramBotHandler) showAnalysisAI(ctx context.Context, c telebot.Context, analysis *dto.AIAnalyzeStockResponse) error {
	sb := strings.Builder{}

	symbolWithExchange := analysis.Exchange + ":" + analysis.StockCode
	marketPrice, _ := cache.GetFromCache[int](fmt.Sprintf(common.KEY_LAST_PRICE, symbolWithExchange))
	if marketPrice == 0 {
		marketPrice = int(analysis.MarketPrice)
	}

	iconSignal := "??"
	if analysis.Signal == "BUY" {
		iconSignal = "🟢"
	} else if analysis.Signal == "HOLD" {
		iconSignal = "🟡"
	}

	sb.WriteString(fmt.Sprintf("<b>%s Signal %s - %s <i>(berdasarkan AI)</i></b>\n", iconSignal, analysis.Signal, symbolWithExchange))
	sb.WriteString(fmt.Sprintf("<i>⏰ %s</i>\n", utils.PrettyDate(analysis.Timestamp)))
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("<b>💰 Harga: %d</b>\n", int(marketPrice)))
	sb.WriteString(fmt.Sprintf("<b>🎯 TP:</b> %d (%s)\n", int(analysis.TargetPrice), utils.FormatChange(float64(marketPrice), float64(analysis.TargetPrice))))
	sb.WriteString(fmt.Sprintf("<b>🛡 SL:</b> %d (%s)\n", int(analysis.StopLoss), utils.FormatChange(float64(marketPrice), float64(analysis.StopLoss))))
	sb.WriteString(fmt.Sprintf("<b>📊 Score:</b> %d | <b>🤖 Confidence:</b> %d\n", int(analysis.TechnicalScore), int(analysis.Confidence)))

	_, stringRatio, _ := utils.CalculateRiskRewardRatio(float64(marketPrice), float64(analysis.TargetPrice), float64(analysis.StopLoss))
	sb.WriteString(fmt.Sprintf("<b>⚖️ RR:</b> %s | <b>⏳ ETA:</b> %s\n", stringRatio, analysis.EstimatedTimeToTP))

	sb.WriteString("\n")
	sb.WriteString("<b>📌 Key Insights:</b>\n")
	for k, insight := range analysis.KeyInsights {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", utils.PrettyKey(k), insight))
	}
	sb.WriteString("\n")
	sb.WriteString("<b>🧠 Alasan Pengambilan Keputusan</b>\n")
	sb.WriteString(analysis.Reason)
	sb.WriteString("\n")

	_, err := t.telegram.Send(ctx, c, sb.String(), telebot.ModeHTML)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to send message show analysis AI", logger.ErrorField(err))
		return err
	}

	return nil
}
