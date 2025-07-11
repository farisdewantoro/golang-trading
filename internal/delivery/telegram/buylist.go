package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/model"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strings"
	"sync"
	"time"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleBuyList(ctx context.Context, c telebot.Context) error {
	latestAnalyses, err := t.service.TelegramBotService.GetAllLatestAnalyses(ctx)
	if err != nil {
		return err
	}

	if len(latestAnalyses) == 0 {
		msgNoExist := `❌ Tidak ditemukan sinyal BUY hari ini.

Coba lagi nanti atau gunakan filter /analyze untuk menemukan peluang baru.`
		_, errSend := t.telegram.Send(ctx, c, msgNoExist)
		if errSend != nil {
			t.log.ErrorContext(ctx, "Failed to send internal error message", logger.ErrorField(errSend))
		}
		return errSend
	}

	var (
		lastStockCode string
	)

	utils.GoSafe(func() {
		newCtx, cancel := context.WithTimeout(t.ctx, t.cfg.Telegram.TimeoutAsyncDuration)
		defer cancel()

		msgRoot, err := t.telegram.Send(newCtx, c, "<i>👍 Memulai menganalisis saham terbaik untuk di beli.....</i>", telebot.ModeHTML)
		if err != nil {
			t.log.ErrorContext(newCtx, "Failed to send message", logger.ErrorField(err))
			return
		}

		var (
			buyListResultMsg = strings.Builder{}
			msgHeader        = &strings.Builder{}
			counter          = 0
			buyCount         = 0
		)

		msgHeader.WriteString("\n 📊 Analisis Saham Sedang Berlangsung...")

		mapSymbolExchangeAnalysis := make(map[string][]model.StockAnalysis)

		for _, analisis := range latestAnalyses {
			if !utils.ShouldContinue(newCtx, t.log) {
				t.log.ErrorContext(newCtx, "Stop signal received", logger.ErrorField(ctx.Err()))
				return
			}
			symbolWithExchange := analisis.Exchange + ":" + analisis.StockCode
			mapSymbolExchangeAnalysis[symbolWithExchange] = append(mapSymbolExchangeAnalysis[symbolWithExchange], analisis)
		}

		wg := &sync.WaitGroup{}
		defer wg.Wait()

		progressCh := make(chan Progress, len(mapSymbolExchangeAnalysis)+1)
		defer close(progressCh)

		wg.Add(1)
		t.showProgressBarWithChannel(newCtx, c, msgRoot, progressCh, len(mapSymbolExchangeAnalysis), wg)

		progressCh <- Progress{Index: 0, StockCode: "initial", Header: msgHeader.String()}
		buyListResult, err := t.service.TradingService.BuyListTradePlan(newCtx, mapSymbolExchangeAnalysis)
		if err != nil {
			t.log.ErrorContext(newCtx, "Failed to get buy list trade plan", logger.ErrorField(err))
			return
		}

		for _, tradePlan := range buyListResult {

			time.Sleep(200 * time.Millisecond)
			counter++
			lastStockCode = tradePlan.Symbol
			if !utils.ShouldContinue(newCtx, t.log) {
				t.log.ErrorContext(newCtx, "Stop signal received", logger.ErrorField(ctx.Err()))
				return
			}

			if !tradePlan.IsBuySignal {
				continue
			}

			buyCount++
			buyListResultMsg.WriteString("\n")
			buyListResultMsg.WriteString(fmt.Sprintf("─<b>%s</b>\n", tradePlan.Symbol))
			buyListResultMsg.WriteString(fmt.Sprintf("├── Market Price: %d\n", int(tradePlan.CurrentMarketPrice)))
			buyListResultMsg.WriteString(fmt.Sprintf("├── Entry: %d\n", int(tradePlan.Entry)))
			buyListResultMsg.WriteString(fmt.Sprintf("├── TP: %d (%s)\n", int(tradePlan.TakeProfit), utils.FormatChange(float64(tradePlan.Entry), tradePlan.TakeProfit)))
			buyListResultMsg.WriteString(fmt.Sprintf("├── SL: %d (%s)\n", int(tradePlan.StopLoss), utils.FormatChange(float64(tradePlan.Entry), tradePlan.StopLoss)))
			buyListResultMsg.WriteString(fmt.Sprintf("├── RR: %.2f\n", tradePlan.RiskReward))
			buyListResultMsg.WriteString(fmt.Sprintf("├── Score: %.2f\n", tradePlan.Score))
			buyListResultMsg.WriteString(fmt.Sprintf("└── Status: %s\n", tradePlan.Status))

			progressCh <- Progress{Index: counter, StockCode: tradePlan.Symbol, Header: msgHeader.String(), Content: buyListResultMsg.String()}

			if buyCount >= t.cfg.Trading.MaxBuyList {
				break
			}
		}

		if buyCount == 0 {
			msgNoExist := `❌ Tidak ditemukan sinyal BUY hari ini.

Coba lagi nanti atau gunakan filter /analyze untuk menemukan peluang baru.`
			_, errSend := t.telegram.Send(ctx, c, msgNoExist)
			if errSend != nil {
				t.log.ErrorContext(ctx, "Failed to send internal error message", logger.ErrorField(errSend))
			}
			return
		}

		msgHeader.Reset()
		msgHeader.WriteString(fmt.Sprintf("📈 Berikut %d saham yang direkomendasikan untuk BUY:", buyCount))
		msgFooter := "\n\n🧠 <i>Rekomendasi berdasarkan analisis teknikal dan sentimen pasar</i>"
		buyListResultMsg.WriteString(msgFooter)
		progressCh <- Progress{Index: len(mapSymbolExchangeAnalysis), StockCode: lastStockCode, Content: buyListResultMsg.String(), Header: msgHeader.String()}
		t.log.InfoContext(newCtx, "Buy list analysis completed", logger.IntField("buyCount", buyCount), logger.IntField("maxBuyList", t.cfg.Trading.MaxBuyList))
	}).Run()

	return nil
}
