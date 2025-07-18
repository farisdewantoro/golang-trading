package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strconv"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleBtnExitStockPosition(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID
	data := c.Data()

	userState, _ := cache.GetFromCache[int](fmt.Sprintf(UserStateKey, userID))
	if userState != StateIdle {
		t.ResetUserState(userID)
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	parts := strings.Split(data, "|")
	if len(parts) != 2 {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	stockPositionIDInt, err := strconv.Atoi(parts[1])
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	stockPosition, err := t.service.TelegramBotService.GetStockPositions(ctx, dto.GetStockPositionsParam{
		TelegramID: &userID,
		IDs:        []uint{uint(stockPositionIDInt)},
	})
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	if len(stockPosition) == 0 {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	stockCodeWithExchange := stockPosition[0].Exchange + ":" + stockPosition[0].StockCode
	marketPrice, _ := cache.GetFromCache[float64](fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange))

	if marketPrice == 0 && len(stockPosition[0].StockPositionMonitorings) > 0 {
		marketPrice = stockPosition[0].StockPositionMonitorings[0].MarketPrice
	}

	msg := fmt.Sprintf(`🚀 Exit posisi saham <b>%s (1/2)</b>
%s
Masukkan <b>harga jual</b> kamu di bawah ini (dalam angka).  
Last Price: %.2f

`, parts[0], t.msgCurrentPosition(&stockPosition[0], marketPrice), marketPrice)

	_, err = t.telegram.Edit(ctx, c, c.Message(), msg, &telebot.SendOptions{ParseMode: telebot.ModeHTML})
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingExitPositionInputExitPrice, t.cfg.Cache.TelegramStateExpDuration)
	t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), &dto.RequestExitPositionData{
		Symbol:          parts[0],
		StockPositionID: uint(stockPositionIDInt),
	}, t.cfg.Cache.TelegramStateExpDuration)

	return nil
}

func (t *TelegramBotHandler) handleExitPositionConversation(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID
	text := c.Text()
	state, ok := cache.GetFromCache[int](fmt.Sprintf(UserStateKey, userID))
	if !ok {
		t.ResetUserState(userID)
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	data, data_ok := cache.GetFromCache[*dto.RequestExitPositionData](fmt.Sprintf(UserDataKey, userID))
	if !data_ok {
		// Should not happen, but as a safeguard
		t.ResetUserState(userID)
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	stockPosition, err := t.service.TelegramBotService.GetStockPositions(ctx, dto.GetStockPositionsParam{
		TelegramID: &userID,
		IDs:        []uint{data.StockPositionID},
	})
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	if len(stockPosition) == 0 {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	stockCodeWithExchange := stockPosition[0].Exchange + ":" + stockPosition[0].StockCode
	marketPrice, _ := cache.GetFromCache[float64](fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange))

	if marketPrice == 0 && len(stockPosition[0].StockPositionMonitorings) > 0 {
		marketPrice = stockPosition[0].StockPositionMonitorings[0].MarketPrice
	}

	switch state {
	case StateWaitingExitPositionInputExitPrice:
		price, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return c.Send("Format harga jual tidak valid. Silakan masukkan angka (contoh: 150.5).")
		}
		data.ExitPrice = price
		t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), data, t.cfg.Cache.TelegramStateExpDuration)

		_, err = t.telegram.Send(ctx, c, fmt.Sprintf(`
🚀 Exit posisi saham <b>%s (2/2)</b>
%s
📅 Kapan tanggal jualnya? (contoh: %s)`, data.Symbol, t.msgCurrentPosition(&stockPosition[0], marketPrice), utils.TimeNowWIB().Format("2006-01-02")), &telebot.SendOptions{ParseMode: telebot.ModeHTML})
		if err != nil {
			return err
		}
		t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingExitPositionInputExitDate, t.cfg.Cache.TelegramStateExpDuration)
		return nil
	case StateWaitingExitPositionInputExitDate:
		date, err := time.Parse("2006-01-02", text)
		if err != nil {
			return c.Send("Format tanggal tidak valid. Silakan gunakan format YYYY-MM-DD.")
		}
		data.ExitDate = date
		t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingExitPositionConfirm, t.cfg.Cache.TelegramStateExpDuration)
		msg := fmt.Sprintf(`
📌 Mohon cek kembali data yang kamu masukkan:

• Kode Saham   : %s 
• Harga Exit   : %.2f %s  
• Tanggal Exit : %s  
		`, data.Symbol, data.ExitPrice, utils.FormatChangeWithIcon(stockPosition[0].BuyPrice, data.ExitPrice), data.ExitDate.Format("2006-01-02"))
		menu := &telebot.ReplyMarkup{}
		btnSave := menu.Data(btnSaveExitPosition.Text, btnSaveExitPosition.Unique)
		btnCancel := menu.Data(btnCancelGeneral.Text, btnCancelGeneral.Unique)
		menu.Inline(
			menu.Row(btnSave, btnCancel),
		)
		_, err = t.telegram.Send(ctx, c, msg, menu, telebot.ModeMarkdown)
		if err != nil {
			return err
		}
	case StateWaitingExitPositionConfirm:
		_, err := t.telegram.Send(ctx, c, "👆 Silakan pilih salah satu opsi di atas, atau kirim /cancel untuk membatalkan.")
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TelegramBotHandler) handleBtnSaveExitPosition(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID

	data, data_ok := cache.GetFromCache[*dto.RequestExitPositionData](fmt.Sprintf(UserDataKey, userID))
	defer t.ResetUserState(userID)

	if !data_ok {
		// Should not happen, but as a safeguard
		_, err := t.telegram.Send(ctx, c, commonErrorInternalMyPosition)
		return err
	}

	if data.ExitPrice == 0 || data.ExitDate.IsZero() {
		_, err := t.telegram.Send(ctx, c, "❌ Data tidak lengkap, silakan masukkan harga exit dan tanggal exit.")
		return err
	}
	newCtx, cancel := context.WithTimeout(t.ctx, t.cfg.Telegram.TimeoutAsyncDuration)

	stopChan := make(chan struct{})
	msg := t.showLoadingGeneral(newCtx, c, stopChan)

	utils.GoSafe(func() {
		defer cancel()

		err := t.service.TelegramBotService.ExitStockPosition(newCtx, userID, data)
		close(stopChan)
		if err != nil {
			t.log.ErrorContext(ctx, "Failed to exit stock position", logger.ErrorField(err))
			_, err = t.telegram.Send(newCtx, c, fmt.Sprintf("❌ Failed to exit stock position: %s", err.Error()))
			if err != nil {
				t.log.ErrorContext(newCtx, "Failed to send error message", logger.ErrorField(err))
			}
			return
		}

		time.Sleep(1 * time.Second)

		_, err = t.telegram.Edit(newCtx, c, msg, "✅ Exit posisi berhasil disimpan.")
		if err != nil {
			t.log.ErrorContext(newCtx, "Failed to send success message", logger.ErrorField(err))
		}
		time.Sleep(1 * time.Second)

		t.handleMyPosition(newCtx, c)

	}).Run()

	return nil
}

func (t *TelegramBotHandler) msgCurrentPosition(stockPosition *model.StockPosition, marketPrice float64) string {
	if stockPosition == nil {
		return ""
	}

	sb := strings.Builder{}
	sb.WriteString("\n<b>Informasi Posisi Saat Ini:</b>\n")
	sb.WriteString(fmt.Sprintf("• Entry: %.2f\n", stockPosition.BuyPrice))
	if stockPosition.TrailingProfitPrice > 0 {
		sb.WriteString(fmt.Sprintf("• TP: %.2f ➡️ %.2f (%s)\n", stockPosition.TakeProfitPrice, stockPosition.TrailingProfitPrice, utils.FormatChange(stockPosition.BuyPrice, stockPosition.TrailingProfitPrice)))
	} else {
		sb.WriteString(fmt.Sprintf("• TP: %.2f (%s)\n", stockPosition.TakeProfitPrice, utils.FormatChange(stockPosition.BuyPrice, stockPosition.TakeProfitPrice)))
	}
	if stockPosition.TrailingStopPrice > 0 {
		sb.WriteString(fmt.Sprintf("• SL: %.2f ➡️ %.2f (%s)\n", stockPosition.StopLossPrice, stockPosition.TrailingStopPrice, utils.FormatChange(stockPosition.BuyPrice, stockPosition.TrailingStopPrice)))
	} else {
		sb.WriteString(fmt.Sprintf("• SL: %.2f (%s)\n", stockPosition.StopLossPrice, utils.FormatChange(stockPosition.BuyPrice, stockPosition.StopLossPrice)))
	}
	sb.WriteString(fmt.Sprintf("• Buy Date: %s\n", stockPosition.BuyDate.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("• PnL: %s\n", utils.FormatChangeWithIcon(stockPosition.BuyPrice, marketPrice)))
	return sb.String()

}
