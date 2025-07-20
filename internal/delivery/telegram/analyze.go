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

func (t *TelegramBotHandler) handleBtnGeneralAnalysis(ctx context.Context, c telebot.Context) error {
	return t.showAnalysisWithLoading(ctx, c, false)
}

func (t *TelegramBotHandler) handleAnalyzeSymbol(ctx context.Context, c telebot.Context) error {
	defer t.ResetUserState(c.Sender().ID)

	return t.showAnalysisWithLoading(ctx, c, true)
}

func (t *TelegramBotHandler) showAnalysisWithLoading(ctx context.Context, c telebot.Context, shouldSendMsg bool) error {

	stopChan := make(chan struct{})

	msg := t.showLoadingFlowAnalysis(c, stopChan, shouldSendMsg)

	symbol := c.Data()

	if symbol == "" {
		symbol = c.Text()
	}

	utils.GoSafe(func() {
		newCtx, cancel := context.WithTimeout(t.ctx, t.cfg.Telegram.TimeoutDuration)
		defer cancel()

		latestAnalyses, err := t.service.TelegramBotService.AnalyzeStock(newCtx, c, symbol)
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

	var (
		analysisIDs []string
		sbHeader    strings.Builder
		sb          strings.Builder
		exchange    string
	)

	if len(latestAnalyses) == 0 {
		_, err := t.telegram.Send(ctx, c, "❌ Tidak ada analisis")
		return err
	}

	exchange = latestAnalyses[0].Exchange

	symbolWithExchange := latestAnalyses[0].Exchange + ":" + latestAnalyses[0].StockCode

	marketPrice, _ := cache.GetFromCache[float64](fmt.Sprintf(common.KEY_LAST_PRICE, symbolWithExchange))
	if marketPrice == 0 {
		marketPrice = latestAnalyses[0].MarketPrice
	}

	sb.WriteString("\n📊 <b><i>Rangkuman Analisis (Multi-Timeframe)</i></b>\n")

	tradePlanResult, err := t.service.TradingService.CreateTradePlan(ctx, latestAnalyses)
	if err != nil {
		return err
	}

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
		sb.WriteString(fmt.Sprintf("- <b>Close</b>: %s (%s) | <b>Vol</b>: %s\n", utils.FormatPrice(ohclv[len(ohclv)-1].Close, exchange), utils.FormatChange(ohclv[len(ohclv)-1].Open, ohclv[len(ohclv)-1].Close), utils.FormatVolume(ohclv[len(ohclv)-1].Volume)))
		sb.WriteString(fmt.Sprintf("- <b>MACD</b>: %s | <b>RSI</b>: %d - %s\n", technicalData.GetTrendMACD(), int(technicalData.Value.Oscillators.RSI), dto.GetRSIStatus(int(technicalData.Value.Oscillators.RSI))))
		sb.WriteString(fmt.Sprintf("- <b>MA</b>: %s | <b>Osc</b>: %s \n", dto.GetSignalText(technicalData.Recommend.Global.MA), dto.GetSignalText(technicalData.Recommend.Global.Oscillators)))

		resultPivots, err := t.service.TradingService.BuildTimeframePivots(&analysis)
		if err != nil {
			t.log.ErrorContext(ctx, "Failed to build pivots", logger.ErrorField(err))
			return err
		}

		for _, val := range resultPivots {
			for _, pivot := range val.PivotData {
				sb.WriteString(fmt.Sprintf("- <b>%s S/R:</b>\n", pivot.Type))
				sb.WriteString("<b> - R: </b>")
				for idx, level := range pivot.Resistance {
					sb.WriteString(fmt.Sprintf("%s (%dx)", utils.FormatPrice(level.Price, exchange), level.Touches))
					if idx < len(pivot.Resistance)-1 {
						sb.WriteString(" | ")
					}
				}
				sb.WriteString("\n")
				sb.WriteString("<b> - S: </b>")
				for idx, level := range pivot.Support {
					sb.WriteString(fmt.Sprintf("%s (%dx)", utils.FormatPrice(level.Price, exchange), level.Touches))
					if idx < len(pivot.Support)-1 {
						sb.WriteString(" | ")
					}
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString("<b>🧠 Insight</b>\n")
	for _, insight := range tradePlanResult.Insights {
		sb.WriteString(fmt.Sprintf("- %s\n", utils.EscapeHTMLForTelegram(insight)))
	}

	iconSignal := "??"
	recommend := dto.SignalBuy

	if tradePlanResult.TechnicalSignal == dto.SignalStrongBuy {
		iconSignal = "🟢"
		recommend = dto.SignalStrongBuy
	} else if tradePlanResult.TechnicalSignal == dto.SignalBuy {
		iconSignal = "🟡"
		recommend = dto.SignalBuy
	} else {
		iconSignal = "🔴"
		recommend = dto.SignalHold
	}

	sbHeader.WriteString(fmt.Sprintf("<b>%s Signal %s - %s <i>(berdasarkan teknikal indikator utama)</i></b>", iconSignal, recommend, symbolWithExchange))
	sbHeader.WriteString("\n")
	sbHeader.WriteString(fmt.Sprintf("<i><b>📅 Update: </b>%s</i>", utils.PrettyDate(latestAnalyses[0].Timestamp)))
	sbHeader.WriteString("\n\n")

	sbHeader.WriteString(fmt.Sprintf("<b>💰 Harga: %s</b>\n", utils.FormatPrice(marketPrice, exchange)))

	menu := &telebot.ReplyMarkup{}
	row := []telebot.Row{}
	btnAskAI := menu.Data(btnAskAIAnalyzer.Text, btnAskAIAnalyzer.Unique, fmt.Sprintf(btnAskAIAnalyzer.Data, symbolWithExchange))
	row = append(row, menu.Row(btnAskAI))

	if tradePlanResult.TechnicalSignal == dto.SignalStrongBuy || tradePlanResult.TechnicalSignal == dto.SignalBuy {
		sbHeader.WriteString(fmt.Sprintf("🎯 <b>Take Profit</b>: %s (%s)\n", utils.FormatPrice(tradePlanResult.TakeProfit, exchange), utils.FormatChange(marketPrice, tradePlanResult.TakeProfit)))
		sbHeader.WriteString(fmt.Sprintf("🛡️ <b>Stop Loss</b>: %s (%s)\n", utils.FormatPrice(tradePlanResult.StopLoss, exchange), utils.FormatChange(marketPrice, tradePlanResult.StopLoss)))
		sbHeader.WriteString(fmt.Sprintf("🔁 <b>Risk Reward</b>: %.2f\n", tradePlanResult.RiskReward))
		sbHeader.WriteString(fmt.Sprintf("🪧 <b>Plan: </b>%s\n", tradePlanResult.PlanType.String()))
		sbHeader.WriteString("\n<b>📝 Penjelasan SL & TP</b>\n")
		sbHeader.WriteString(fmt.Sprintf("<i>🛡️ <b>Stop Loss</b> ditentukan berdasarkan %s</i>\n", tradePlanResult.SLReason))
		sbHeader.WriteString(fmt.Sprintf("<i>🎯 <b>Take Profit</b> berasal dari %s</i>\n", tradePlanResult.TPReason))

		btnSetPosition := menu.Data(btnSetPositionTechnical.Text, btnSetPositionTechnical.Unique, symbolWithExchange)
		row = append(row, menu.Row(btnSetPosition))
	}

	row = append(row, menu.Row(btnDeleteMessage))
	menu.Inline(row...)

	_, err = t.telegram.Edit(ctx, c, loadingMsg, sbHeader.String()+sb.String(), menu, telebot.ModeHTML)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to edit message", logger.ErrorField(err))
		return err
	}

	return nil
}
