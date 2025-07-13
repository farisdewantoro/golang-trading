package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/model"
	"golang-trading/pkg/common"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strings"
	"sync"
	"time"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleBuyList(ctx context.Context, c telebot.Context) error {
	msg := `📊 <b>Pilih Exchange untuk Daftar BUY Hari Ini:</b>

Silakan pilih jenis pasar yang ingin Anda lihat sinyal BUY-nya:

🇮🇩 IDX — Saham Indonesia  
📈 NASDAQ — Saham Amerika Serikat  
💰 BINANCE — Cryptocurrency

Pilih salah satu tombol di bawah untuk melihat daftar rekomendasi BUY dari masing-masing exchange 👇
	
`
	menu := &telebot.ReplyMarkup{}
	rows := []telebot.Row{}
	var tempRow []telebot.Btn

	exchanges := common.GetExchangeList()

	exchanges = append(exchanges, "SEMUA")

	for _, exchange := range exchanges {
		tempRow = append(tempRow, menu.Data(exchange, btnShowBuyListAnalysis.Unique, exchange))
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
	_, errSend := t.telegram.Send(ctx, c, msg, menu, telebot.ModeHTML)
	if errSend != nil {
		t.log.ErrorContext(ctx, "Failed to send internal error message", logger.ErrorField(errSend))
	}
	return errSend
}

func (t *TelegramBotHandler) handleBtnShowBuyListAnalysis(ctx context.Context, c telebot.Context) error {
	exchange := c.Data()
	latestAnalyses, err := t.service.TelegramBotService.GetAllLatestAnalyses(ctx, exchange)
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
		msgRoot       = c.Message()
	)

	utils.GoSafe(func() {
		newCtx, cancel := context.WithTimeout(t.ctx, t.cfg.Telegram.TimeoutAsyncDuration)
		defer cancel()

		if msgRoot == nil {
			msgRoot, err = t.telegram.Send(newCtx, c, fmt.Sprintf("<i>👍 Memulai menganalisis %s terbaik untuk di beli.....</i>", exchange), telebot.ModeHTML)
			if err != nil {
				t.log.ErrorContext(newCtx, "Failed to send message", logger.ErrorField(err))
				return
			}
		}

		var (
			buyListResultMsg = strings.Builder{}
			msgHeader        = &strings.Builder{}
			counter          = 0
			buySymbols       []string
		)

		msgHeader.WriteString(fmt.Sprintf("\n 📊 Analisis %s Sedang Berlangsung...", exchange))

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
			tradePlan := tradePlan

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

			buySymbols = append(buySymbols, tradePlan.Symbol)
			buyListResultMsg.WriteString("\n")
			buyListResultMsg.WriteString(fmt.Sprintf("<b>%s</b>\n", tradePlan.Symbol))
			buyListResultMsg.WriteString(fmt.Sprintf("Buy: %.2f | RR: %.2f\n", tradePlan.Entry, tradePlan.RiskReward))
			buyListResultMsg.WriteString(fmt.Sprintf("TP: %.2f (%s)\n", tradePlan.TakeProfit, utils.FormatChange(tradePlan.Entry, tradePlan.TakeProfit)))
			buyListResultMsg.WriteString(fmt.Sprintf("SL: %.2f (%s)\n", tradePlan.StopLoss, utils.FormatChange(tradePlan.Entry, tradePlan.StopLoss)))
			buyListResultMsg.WriteString(fmt.Sprintf("%s | Score: %.2f\n", tradePlan.Status, tradePlan.Score))

			progressCh <- Progress{Index: counter, StockCode: tradePlan.Symbol, Header: msgHeader.String(), Content: buyListResultMsg.String()}

			if len(buySymbols) >= t.cfg.Trading.MaxBuyList {
				break
			}
		}

		if len(buySymbols) == 0 {
			msgNoExist := `❌ Tidak ditemukan sinyal BUY hari ini.

Coba lagi nanti atau gunakan filter /analyze untuk menemukan peluang baru.`
			_, errSend := t.telegram.Edit(ctx, c, msgRoot, msgNoExist)
			if errSend != nil {
				t.log.ErrorContext(ctx, "Failed to send internal error message", logger.ErrorField(errSend))
			}
			return
		}

		msgHeader.Reset()
		msgHeader.WriteString(fmt.Sprintf("📈 Berikut %d %s yang direkomendasikan untuk BUY:", len(buySymbols), exchange))
		msgFooter := "\n\n<i>🔍 Pilih saham di bawah untuk melihat detail analisa:</i>"
		buyListResultMsg.WriteString(msgFooter)

		menu := &telebot.ReplyMarkup{}
		rows := []telebot.Row{}
		var tempRow []telebot.Btn

		for _, symbolBuy := range buySymbols {
			tempRow = append(tempRow, menu.Data(symbolBuy, btnGeneralAnalisis.Unique, symbolBuy))
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

		progressCh <- Progress{Index: len(mapSymbolExchangeAnalysis), StockCode: lastStockCode, Content: buyListResultMsg.String(), Header: msgHeader.String(), Menu: menu}
		t.log.InfoContext(newCtx, "Buy list analysis completed", logger.IntField("buyCount", len(buySymbols)), logger.IntField("maxBuyList", t.cfg.Trading.MaxBuyList))
	}).Run()

	return nil
}
