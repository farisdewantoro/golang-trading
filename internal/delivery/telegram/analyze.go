package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleStartAnalyze(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID
	t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingAnalyzeSymbol, t.cfg.Cache.TelegramStateExpDuration)
	return c.Send("Silakan masukkan simbol saham yang ingin Anda analisis berserta dengan exchange code (contoh: IDX:BBCA, NASDAQ:TESLA).")
}

func (t *TelegramBotHandler) handleAnalyzeSymbol(ctx context.Context, c telebot.Context) error {
	defer t.ResetUserState(c.Sender().ID)

	stopChan := make(chan struct{})

	msg := t.showLoadingFlowAnalysis(c, stopChan, true)

	utils.GoSafe(func() {
		newCtx, cancel := context.WithTimeout(t.ctx, t.cfg.Telegram.TimeoutDuration)
		defer cancel()

		latestAnalyses, err := t.service.TelegramBotService.AnalyzeStock(newCtx, c)
		if err != nil {
			close(stopChan)
			t.log.ErrorContext(ctx, "Failed to analyze stock", logger.ErrorField(err))

			// Send error message
			_, err = t.telegram.Send(newCtx, c, fmt.Sprintf("❌ Failed to get stock analysis: %s", err.Error()))
			if err != nil {
				t.log.ErrorContext(newCtx, "Failed to send error message", logger.ErrorField(err))
			}
			return
		}

		close(stopChan)

		err = t.showAnalysis(newCtx, c, msg, latestAnalyses)

		if err != nil {
			t.log.ErrorContext(newCtx, "Failed to show analysis", logger.ErrorField(err))
			return
		}

	}).Run()

	return nil
}

func (t *TelegramBotHandler) showLoadingFlowAnalysis(c telebot.Context, stop <-chan struct{}, shouldSendNewMsg bool) *telebot.Message {

	msgRoot := c.Message()
	initial := "Sedang menganalisis saham kamu, mohon tunggu"

	var msg *telebot.Message
	var err error

	// Cek apakah pesan terakhir berasal dari bot
	if msgRoot == nil || msgRoot.Sender == nil || !msgRoot.Sender.IsBot || shouldSendNewMsg {
		msg, err = t.telegram.Send(t.ctx, c, initial, &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})
		if err != nil {
			t.log.ErrorContext(t.ctx, "Failed to send loading message", logger.ErrorField(err))
			return nil
		}
	} else {
		msg, err = t.telegram.Edit(t.ctx, c, msgRoot, initial)
		if err != nil {
			t.log.ErrorContext(t.ctx, "Failed to edit loading message", logger.ErrorField(err))
			return nil
		}
	}

	go func() {
		dots := []string{"⏳", "⏳⏳", "⏳⏳⏳"}
		i := 0
		ctx, cancel := context.WithTimeout(t.ctx, t.cfg.Telegram.TimeoutAsyncDuration)
		defer cancel()
		for {
			if utils.ShouldStopChan(stop, t.log) {
				return
			}
			if !utils.ShouldContinue(ctx, t.log) {
				return
			}
			_, err := t.telegram.Edit(ctx, c, msg, fmt.Sprintf("%s%s", initial, dots[i%len(dots)]))

			if err != nil {
				t.log.ErrorContext(ctx, "Failed to update loading animation", logger.ErrorField(err))
				return
			}
			i++
			time.Sleep(500 * time.Millisecond)

		}
	}()

	return msg
}

func (t *TelegramBotHandler) showAnalysis(ctx context.Context, c telebot.Context, loadingMsg *telebot.Message, latestAnalyses []model.StockAnalysis) error {

	sbHeader := strings.Builder{}

	sb := strings.Builder{}

	symbolWithExchange := latestAnalyses[0].Exchange + ":" + latestAnalyses[0].StockCode

	marketPrice, _ := cache.GetFromCache[int](fmt.Sprintf(common.KEY_LAST_PRICE, symbolWithExchange))
	if marketPrice == 0 {
		marketPrice = int(latestAnalyses[0].MarketPrice)
	}

	support := []float64{}
	resistance := []float64{}

	sb.WriteString("\n📊 <b><i>Rangkuman Analisis (Multi-Timeframe)</i></b>\n")

	analysisIDs := []string{}

	for _, analysis := range latestAnalyses {
		analysisIDs = append(analysisIDs, fmt.Sprintf("%d", analysis.ID))
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
		sb.WriteString(fmt.Sprintf("<b><i>%s</i></b>\n", valTimeframeSummary))
		sb.WriteString(fmt.Sprintf("- <b>Close</b>: %d (%s) | <b>Vol</b>: %s\n", int(ohclv[len(ohclv)-1].Close), utils.FormatChange(ohclv[len(ohclv)-1].Open, ohclv[len(ohclv)-1].Close), utils.FormatVolume(ohclv[len(ohclv)-1].Volume)))
		sb.WriteString(fmt.Sprintf("- <b>MACD</b>: %s | <b>RSI</b>: %d - %s\n", technicalData.GetTrendMACD(), int(technicalData.Value.Oscillators.RSI), dto.GetRSIText(int(technicalData.Recommend.Oscillators.RSI))))
		sb.WriteString(fmt.Sprintf("- <b>MA</b>: %s | <b>Osc</b>: %s \n", dto.GetSignalText(technicalData.Recommend.Global.MA), dto.GetSignalText(technicalData.Recommend.Global.Oscillators)))
		sb.WriteString(fmt.Sprintf("- <b>R1</b>: %d (%s) | <b>R2</b>: %d (%s)\n",
			int(technicalData.Value.Pivots.Classic.R1), utils.FormatChange(float64(marketPrice), technicalData.Value.Pivots.Classic.R1),
			int(technicalData.Value.Pivots.Classic.R2), utils.FormatChange(float64(marketPrice), technicalData.Value.Pivots.Classic.R2)))
		sb.WriteString(fmt.Sprintf("- <b>S1</b>: %d (%s) | <b>S2</b>: %d (%s)\n",
			int(technicalData.Value.Pivots.Classic.S1), utils.FormatChange(float64(marketPrice), technicalData.Value.Pivots.Classic.S1),
			int(technicalData.Value.Pivots.Classic.S2), utils.FormatChange(float64(marketPrice), technicalData.Value.Pivots.Classic.S2)))

		support = append(support,
			technicalData.Value.Pivots.Classic.S1,
			technicalData.Value.Pivots.Classic.S2,
			technicalData.Value.Pivots.Classic.S3,
			technicalData.Value.Pivots.Camarilla.S1,
			technicalData.Value.Pivots.Camarilla.S2,
			technicalData.Value.Pivots.Camarilla.S3,
			technicalData.Value.Pivots.Demark.S1,
			technicalData.Value.Pivots.Fibonacci.S1,
			technicalData.Value.Pivots.Fibonacci.S2,
			technicalData.Value.Pivots.Fibonacci.S3,
			technicalData.Value.Pivots.Woodie.S1,
			technicalData.Value.Pivots.Woodie.S2,
			technicalData.Value.Pivots.Woodie.S3,
		)
		resistance = append(resistance,
			technicalData.Value.Pivots.Classic.R1,
			technicalData.Value.Pivots.Classic.R2,
			technicalData.Value.Pivots.Classic.R3,
			technicalData.Value.Pivots.Camarilla.R1,
			technicalData.Value.Pivots.Camarilla.R2,
			technicalData.Value.Pivots.Camarilla.R3,
			technicalData.Value.Pivots.Demark.R1,
			technicalData.Value.Pivots.Fibonacci.R1,
			technicalData.Value.Pivots.Fibonacci.R2,
			technicalData.Value.Pivots.Fibonacci.R3,
			technicalData.Value.Pivots.Woodie.R1,
			technicalData.Value.Pivots.Woodie.R2,
			technicalData.Value.Pivots.Woodie.R3,
		)
	}

	evalSignal, err := t.service.TelegramBotService.EvaluateSignal(ctx, latestAnalyses)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to calculate weighted signal", logger.ErrorField(err))
		return err
	}

	iconSignal := "??"
	recommend := dto.SignalBuy

	if evalSignal == dto.SignalStrongBuy {
		iconSignal = "🟢"
		recommend = dto.SignalStrongBuy
	} else if evalSignal == dto.SignalBuy {
		iconSignal = "🟡"
		recommend = dto.SignalBuy
	} else {
		iconSignal = "🔴"
		recommend = dto.SignalHold
	}

	sbHeader.WriteString(fmt.Sprintf("<b>%s Signal %s - %s <i>(berdasarkan teknikal indikator utama)</i></b>", iconSignal, recommend, symbolWithExchange))
	sbHeader.WriteString("\n\n")
	sbHeader.WriteString(fmt.Sprintf("<b>💰 Harga: %d</b>\n", marketPrice))

	if evalSignal == dto.SignalStrongBuy || evalSignal == dto.SignalBuy {
		tradePlan := dto.CreateTradePlan(float64(marketPrice), support, resistance, float64(2.0))
		sbHeader.WriteString(fmt.Sprintf("🎯 <b>Take Profit</b>: %d (%s)\n", int(tradePlan.TakeProfit), utils.FormatChange(float64(marketPrice), tradePlan.TakeProfit)))
		sbHeader.WriteString(fmt.Sprintf("🛡️ <b>Stop Loss</b>: %d (%s)\n", int(tradePlan.StopLoss), utils.FormatChange(float64(marketPrice), tradePlan.StopLoss)))
		sbHeader.WriteString(fmt.Sprintf("🔁 <b>Risk Reward</b>: %.2f\n", tradePlan.RiskReward))
	}

	sbHeader.WriteString(fmt.Sprintf("<i><b>📅 Update: %s</b></i>", utils.PrettyDate(latestAnalyses[0].Timestamp)))
	sbHeader.WriteString("\n")

	menu := &telebot.ReplyMarkup{}

	btnAskAI := menu.Data(btnAskAIAnalyzer.Text, btnAskAIAnalyzer.Unique, fmt.Sprintf(btnAskAIAnalyzer.Data, symbolWithExchange))

	menu.Inline(menu.Row(btnAskAI))

	_, err = t.telegram.Edit(ctx, c, loadingMsg, sbHeader.String()+sb.String(), menu, telebot.ModeHTML)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to edit message", logger.ErrorField(err))
		return err
	}

	return nil
}
