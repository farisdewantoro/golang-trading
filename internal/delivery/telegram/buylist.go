package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/common"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strings"
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
		msgNoExist := `❌ Data Tidak Tersedia

Coba lagi nanti atau gunakan filter /analyze untuk menemukan peluang baru atau /scheduler untuk trigger data baru.`
		_, errSend := t.telegram.Send(ctx, c, msgNoExist)
		if errSend != nil {
			t.log.ErrorContext(ctx, "Failed to send internal error message", logger.ErrorField(errSend))
		}
		return errSend
	}

	utils.GoSafe(func() {
		newCtx, cancel := context.WithTimeout(t.ctx, t.cfg.Telegram.TimeoutAsyncDuration)
		defer cancel()

		var (
			mapSymbolExchangeAnalysis = make(map[string][]model.StockAnalysis)
			buyListResultMsg          = strings.Builder{}
			msgHeader                 = &strings.Builder{}
			buySymbolMap              = make(map[string]string)
			buyCount                  int
			stopChan                  = make(chan struct{})
		)

		msg := t.showLoadingGeneral(newCtx, c, stopChan)

		positions, err := t.service.TelegramBotService.GetStockPositions(newCtx, dto.GetStockPositionsParam{
			TelegramID: &c.Sender().ID,
			IsActive:   utils.ToPointer(true),
		})
		if err != nil {
			_, errSend := t.telegram.Send(newCtx, c, commonErrorInternal)
			if errSend != nil {
				t.log.ErrorContext(newCtx, "Failed to send internal error message", logger.ErrorField(errSend))
			}
			return
		}

		positionsMap := make(map[string]model.StockPosition)
		for _, position := range positions {
			positionsMap[position.Exchange+":"+position.StockCode] = position
		}

		for _, analisis := range latestAnalyses {
			symbolWithExchange := analisis.Exchange + ":" + analisis.StockCode
			mapSymbolExchangeAnalysis[symbolWithExchange] = append(mapSymbolExchangeAnalysis[symbolWithExchange], analisis)
		}

		buyListResult, err := t.service.TradingService.BuyListTradePlan(newCtx, mapSymbolExchangeAnalysis)

		time.Sleep(300 * time.Millisecond)
		close(stopChan)

		if err != nil {
			t.log.ErrorContext(newCtx, "Failed to get buy list trade plan", logger.ErrorField(err))
			return
		}
		for _, tradePlan := range buyListResult {
			tradePlan := tradePlan

			if !utils.ShouldContinue(newCtx, t.log) {
				t.log.ErrorContext(newCtx, "Stop signal received", logger.ErrorField(ctx.Err()))
				return
			}

			if !tradePlan.IsBuySignal {
				continue
			}

			buyCount++
			symbolWithExchange := tradePlan.Exchange + ":" + tradePlan.Symbol

			if _, ok := positionsMap[symbolWithExchange]; ok {
				buyListResultMsg.WriteString("\n")
				buyListResultMsg.WriteString(fmt.Sprintf("<b>%d. %s - [OWNED ✅]</b>\n", buyCount, tradePlan.Symbol))
				buyListResultMsg.WriteString(fmt.Sprintf("- <b>Signal:</b> %s | Score: %.2f\n", tradePlan.TechnicalSignal, tradePlan.Score))
				continue
			}

			if tradePlan.RiskReward == 0 {
				buyListResultMsg.WriteString("\n")
				buyListResultMsg.WriteString(fmt.Sprintf("<b>%d. %s <i>(Plan: %s)</i></b> \n", buyCount, tradePlan.Symbol, tradePlan.PlanType.String()))
				buyListResultMsg.WriteString(fmt.Sprintf("- <b>Signal:</b> %s | Score: %.2f\n", tradePlan.TechnicalSignal, tradePlan.Score))

				continue
			}

			buySymbolMap[tradePlan.Symbol] = symbolWithExchange
			buyListResultMsg.WriteString("\n")
			buyListResultMsg.WriteString(fmt.Sprintf("<b>%d. %s <i>(Plan: %s)</i></b>\n", buyCount, tradePlan.Symbol, tradePlan.PlanType.String()))
			buyListResultMsg.WriteString(fmt.Sprintf("- <b>Buy:</b> %s | RR: %.2f\n", utils.FormatPrice(tradePlan.Entry, tradePlan.Exchange), tradePlan.RiskReward))
			buyListResultMsg.WriteString(fmt.Sprintf("- <b>TP:</b> %s (%s)\n", utils.FormatPrice(tradePlan.TakeProfit, tradePlan.Exchange), utils.FormatChange(tradePlan.Entry, tradePlan.TakeProfit)))
			buyListResultMsg.WriteString(fmt.Sprintf("- <b>SL:</b> %s (%s)\n", utils.FormatPrice(tradePlan.StopLoss, tradePlan.Exchange), utils.FormatChange(tradePlan.Entry, tradePlan.StopLoss)))
			buyListResultMsg.WriteString(fmt.Sprintf("- <b>Signal:</b> %s | Score: %.2f\n", tradePlan.TechnicalSignal, tradePlan.Score))
			buyListResultMsg.WriteString(fmt.Sprintf("- <b>Status:</b> %s\n", dto.PositionStatus(tradePlan.Status)))

			if len(buySymbolMap) >= t.cfg.Trading.MaxBuyList {
				break
			}

		}

		if len(buySymbolMap) == 0 {
			msgNoExist := `❌ Tidak ditemukan sinyal BUY hari ini.

Coba lagi nanti atau gunakan filter /analyze untuk menemukan peluang baru.`
			_, errSend := t.telegram.Edit(newCtx, c, msg, msgNoExist)
			if errSend != nil {
				t.log.ErrorContext(newCtx, "Failed to edit message", logger.ErrorField(errSend))
			}
			return
		}

		msgHeader.Reset()
		msgHeader.WriteString(fmt.Sprintf("📈 Berikut %d %s yang direkomendasikan untuk BUY:\n", len(buySymbolMap), exchange))
		msgFooter := "\n\n<i>🔍 Pilih saham di bawah untuk melihat detail analisa:</i>"
		buyListResultMsg.WriteString(msgFooter)

		menu := &telebot.ReplyMarkup{}
		rows := []telebot.Row{}
		var tempRow []telebot.Btn

		for k, v := range buySymbolMap {
			tempRow = append(tempRow, menu.Data(k, btnGeneralAnalisis.Unique, v))
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

		_, errSend := t.telegram.Edit(newCtx, c, msg, msgHeader.String()+buyListResultMsg.String(), menu, telebot.ModeHTML)
		if errSend != nil {
			t.log.ErrorContext(newCtx, "Failed to send internal error message", logger.ErrorField(errSend))
		}
	}).Run()

	return nil
}
