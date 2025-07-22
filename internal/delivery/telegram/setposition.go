package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/utils"
	"strconv"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleSetPosition(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID

	t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingSetPositionSymbol, t.cfg.Cache.TelegramStateExpDuration)

	reqData := &dto.RequestSetPositionData{
		UserTelegram: dto.ToRequestUserTelegram(c.Sender()),
	}

	t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), reqData, t.cfg.Cache.TelegramStateExpDuration)

	_, err := t.telegram.Send(ctx, c, "📈 Masukkan kode saham dan exchange kamu <i>(contoh: IDX:ANTM, NASDAQ:TSLA )</i>:", telebot.ModeHTML)
	if err != nil {
		return err
	}
	return nil
}

func (t *TelegramBotHandler) handleSetPositionConversation(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID
	text := c.Text()
	state, ok := cache.GetFromCache[int](fmt.Sprintf(UserStateKey, userID))
	if !ok {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	data, data_ok := cache.GetFromCache[*dto.RequestSetPositionData](fmt.Sprintf(UserDataKey, userID))
	if !data_ok {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	switch state {
	case StateWaitingSetPositionSymbol:
		stockCode, exchange, err := utils.ParseStockSymbol(strings.ToUpper(text))
		if err != nil {
			_, err = t.telegram.Send(ctx, c, "Format kode saham tidak valid. Silakan masukkan kode saham dan exchange (contoh: IDX:ANTM, NASDAQ:TSLA).", telebot.ModeMarkdown)
			if err != nil {
				return err
			}
		}
		data.StockCode = stockCode
		data.Exchange = exchange
		_, err = t.telegram.Send(ctx, c, fmt.Sprintf("👍 Oke, kode *%s* tercatat!", text), telebot.ModeMarkdown)
		if err != nil {
			return err
		}
		t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingSetPositionBuyPrice, t.cfg.Cache.TelegramStateExpDuration)
		_, err = t.telegram.Send(ctx, c, "💰 Berapa harga belinya ? (contoh: 150)", telebot.ModeMarkdown)
		if err != nil {
			return err
		}

		t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), data, t.cfg.Cache.TelegramStateExpDuration)

	case StateWaitingSetPositionBuyPrice:
		price, err := strconv.ParseFloat(text, 64)
		if err != nil {
			_, err = t.telegram.Send(ctx, c, "Format harga beli tidak valid. Silakan masukkan angka (contoh: 150).", telebot.ModeMarkdown)
			if err != nil {
				return err
			}
		}
		data.BuyPrice = price
		t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingSetPositionBuyDate, t.cfg.Cache.TelegramStateExpDuration)
		_, err = t.telegram.Send(ctx, c, "📅 Kapan tanggal belinya? (format: YYYY-MM-DD)", telebot.ModeMarkdown)
		if err != nil {
			return err
		}

		t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), data, t.cfg.Cache.TelegramStateExpDuration)

	case StateWaitingSetPositionBuyDate:
		_, err := time.Parse("2006-01-02", text)
		if err != nil {
			_, err = t.telegram.Send(ctx, c, "Format tanggal tidak valid. Silakan gunakan format YYYY-MM-DD.", telebot.ModeMarkdown)
			if err != nil {
				return err
			}
		}
		data.BuyDate = text
		t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingSetPositionTakeProfit, t.cfg.Cache.TelegramStateExpDuration)
		_, err = t.telegram.Send(ctx, c, "🎯 Target take profit-nya di harga berapa? (contoh: 180)", telebot.ModeMarkdown)
		if err != nil {
			return err
		}

		t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), data, t.cfg.Cache.TelegramStateExpDuration)

	case StateWaitingSetPositionTakeProfit:
		price, err := strconv.ParseFloat(text, 64)
		if err != nil {
			_, err = t.telegram.Send(ctx, c, "Format harga take profit tidak valid. Silakan masukkan angka.", telebot.ModeMarkdown)
			if err != nil {
				return err
			}
		}
		data.TakeProfit = price
		t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingSetPositionStopLoss, t.cfg.Cache.TelegramStateExpDuration)
		_, err = t.telegram.Send(ctx, c, "📉 Stop loss-nya di harga berapa? (contoh: 140)", telebot.ModeMarkdown)
		if err != nil {
			return err
		}

		t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), data, t.cfg.Cache.TelegramStateExpDuration)

	case StateWaitingSetPositionStopLoss:
		price, err := strconv.ParseFloat(text, 64)
		if err != nil {
			_, err = t.telegram.Send(ctx, c, "Format harga stop loss tidak valid. Silakan masukkan angka.", telebot.ModeMarkdown)
			if err != nil {
				return err
			}
		}
		data.StopLoss = price
		t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingSetPositionMaxHolding, t.cfg.Cache.TelegramStateExpDuration)
		_, err = t.telegram.Send(ctx, c, "⏳ Berapa maksimal hari mau di-hold? (contoh: 1) \n\n📌 *Note:* Isi angka dari *1* sampai *14* hari.", &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})
		if err != nil {
			return err
		}

		t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), data, t.cfg.Cache.TelegramStateExpDuration)

	case StateWaitingSetPositionMaxHolding:
		intVal, err := strconv.Atoi(text)
		if err != nil || intVal <= 0 {
			_, err = t.telegram.Send(ctx, c, "Format maksimal hari hold tidak valid. Silakan masukkan angka bulat positif.", telebot.ModeMarkdown)
			if err != nil {
				return err
			}
		}
		data.MaxHolding = intVal
		t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingSetPositionAlertPrice, t.cfg.Cache.TelegramStateExpDuration)

		menu := &telebot.ReplyMarkup{}
		btnYes := menu.Data("✅ Ya", btnSetPositionAlertPrice.Unique, "true")
		btnNo := menu.Data("❌ Tidak", btnSetPositionAlertPrice.Unique, "false")

		menu.Inline(
			menu.Row(btnYes, btnNo),
		)

		_, err = t.telegram.Send(ctx, c, "🚨 Aktifkan alert untuk data ini?\n\nNote: Sistem akan kirim pesan kalau harga mencapai take profit atau stop loss yang kamu tentukan.", menu)
		if err != nil {
			return err
		}

		t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), data, t.cfg.Cache.TelegramStateExpDuration)

	case StateWaitingSetPositionAlertPrice, StateWaitingSetPositionAlertMonitor:
		_, err := t.telegram.Send(ctx, c, "👆 Silakan pilih salah satu opsi di atas, atau kirim /cancel untuk membatalkan.")
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TelegramBotHandler) handleBtnSetPositionAlertPrice(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID
	state, _ := cache.GetFromCache[int](fmt.Sprintf(UserStateKey, userID))
	if state != StateWaitingSetPositionAlertPrice {
		t.ResetUserState(userID)
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	data, ok := cache.GetFromCache[*dto.RequestSetPositionData](fmt.Sprintf(UserDataKey, userID))
	if !ok {
		t.ResetUserState(userID)
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	isSet := c.Data() == "true"
	data.AlertPrice = isSet
	t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), data, t.cfg.Cache.TelegramStateExpDuration)

	if isSet {
		t.telegram.Edit(ctx, c, c.Message(), "✅ Alert harga saham diaktifkan.", &telebot.SendOptions{
			ParseMode: telebot.ModeMarkdown,
		})
	} else {
		t.telegram.Edit(ctx, c, c.Message(), "❌ Alert harga saham dinonaktifkan.", &telebot.SendOptions{
			ParseMode: telebot.ModeMarkdown,
		})
	}

	t.inmemoryCache.Set(fmt.Sprintf(UserStateKey, userID), StateWaitingSetPositionAlertMonitor, t.cfg.Cache.TelegramStateExpDuration)

	menu := &telebot.ReplyMarkup{}
	btnYes := menu.Data("✅ Ya", btnSetPositionAlertMonitor.Unique, "true")
	btnNo := menu.Data("❌ Tidak", btnSetPositionAlertMonitor.Unique, "false")
	menu.Inline(
		menu.Row(btnYes, btnNo),
	)
	_, err := t.telegram.Send(ctx, c, "🔎 Aktifkan monitoring alert?\n\nNote: Sistem akan menganalisis posisi ini dan kirim laporan singkat: apakah masih aman, rawan, atau mendekati batas hold/SL.", menu)
	if err != nil {
		return err
	}
	return nil
}

func (t *TelegramBotHandler) handleBtnSetPositionAlertMonitor(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID
	state, _ := cache.GetFromCache[int](fmt.Sprintf(UserStateKey, userID))
	if state != StateWaitingSetPositionAlertMonitor {
		t.ResetUserState(userID)
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}
	data, ok := cache.GetFromCache[*dto.RequestSetPositionData](fmt.Sprintf(UserDataKey, userID))
	if !ok {
		t.ResetUserState(userID)
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}
	isSet := c.Data() == "true"
	data.AlertMonitor = isSet
	t.inmemoryCache.Set(fmt.Sprintf(UserDataKey, userID), data, t.cfg.Cache.TelegramStateExpDuration)

	if isSet {
		t.telegram.Edit(ctx, c, c.Message(), "✅ Alert monitor diaktifkan.", &telebot.SendOptions{
			ParseMode: telebot.ModeMarkdown,
		})
	} else {
		t.telegram.Edit(ctx, c, c.Message(), "❌ Alert monitor dinonaktifkan.", &telebot.SendOptions{
			ParseMode: telebot.ModeMarkdown,
		})
	}
	return t.handleSetPositionFinish(ctx, c)
}

func (t *TelegramBotHandler) handleSetPositionFinish(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID
	data, ok := cache.GetFromCache[*dto.RequestSetPositionData](fmt.Sprintf(UserDataKey, userID))
	defer t.ResetUserState(userID)

	if !ok {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	if err := t.service.TelegramBotService.SetStockPosition(ctx, data); err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	return t.showSetPositionSuccess(ctx, c, data)
}

func (t *TelegramBotHandler) showSetPositionSuccess(ctx context.Context, c telebot.Context, data *dto.RequestSetPositionData) error {
	var sb strings.Builder
	symbolWithExchange := data.Exchange + ":" + data.StockCode
	sb.WriteString("💾 Posisi saham berhasil disimpan!\n\n")
	sb.WriteString("📊 Detail:\n")
	sb.WriteString("— Saham: " + symbolWithExchange + "\n")
	sb.WriteString("— Harga Beli: " + strconv.FormatFloat(data.BuyPrice, 'f', 0, 64) + "\n")
	sb.WriteString("— Tanggal Beli: " + data.BuyDate + "\n")
	sb.WriteString("— Take Profit: " + strconv.FormatFloat(data.TakeProfit, 'f', 0, 64) + "\n")
	sb.WriteString("— Stop Loss: " + strconv.FormatFloat(data.StopLoss, 'f', 0, 64) + "\n")
	sb.WriteString("— Max Hold: " + strconv.Itoa(data.MaxHolding) + " hari\n\n")

	if data.AlertPrice {
		sb.WriteString("🔔 Alert harga *ON* — sistem akan kirim notifikasi jika harga menyentuh TP atau SL.\n")
	} else {
		sb.WriteString("🔕 Alert harga *OFF*.\n")
	}

	if data.AlertMonitor {
		sb.WriteString("🧠 Monitoring *ON* — kamu akan dapat laporan harian selama posisi masih berjalan.")
	} else {
		sb.WriteString("🧠 Monitoring *OFF*.\n")
	}

	if data.IsMessageEdit {
		_, err := t.telegram.Edit(ctx, c, c.Message(), sb.String(), telebot.ModeMarkdown)
		if err != nil {
			return err
		}
		return nil
	}
	_, err := t.telegram.Send(ctx, c, sb.String(), telebot.ModeMarkdown)
	if err != nil {
		return err
	}
	return nil
}

func (t *TelegramBotHandler) handleBtnSetPositionByTechnical(ctx context.Context, c telebot.Context) error {
	userTelegram := dto.ToRequestUserTelegram(c.Sender())
	symbolWithExchange := c.Data()

	parts := strings.Split(symbolWithExchange, ":")
	if len(parts) != 2 {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}
	exchange := parts[0]
	stockCode := parts[1]

	latestAnalyses, err := t.service.TelegramBotService.AnalyzeStock(ctx, c, symbolWithExchange)
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	tradePlanResult, err := t.service.TradingService.CreateTradePlan(ctx, latestAnalyses)
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	data := &dto.RequestSetPositionData{
		UserTelegram:  userTelegram,
		StockCode:     stockCode,
		Exchange:      exchange,
		BuyPrice:      tradePlanResult.Entry,
		TakeProfit:    tradePlanResult.TakeProfit,
		StopLoss:      tradePlanResult.StopLoss,
		BuyDate:       utils.TimeNowWIB().Format("2006-01-02"),
		MaxHolding:    5,
		AlertPrice:    true,
		AlertMonitor:  true,
		SourceType:    model.StockPositionSourceTypeTechnical,
		IsMessageEdit: true,
		PlanScore:     tradePlanResult.Score,
		PositionScore: tradePlanResult.PositionScore,
	}

	if err := t.service.TelegramBotService.SetStockPosition(ctx, data); err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	return t.showSetPositionSuccess(ctx, c, data)
}

func (t *TelegramBotHandler) handleBtnSetPositionByAI(ctx context.Context, c telebot.Context) error {
	userTelegram := dto.ToRequestUserTelegram(c.Sender())
	symbolWithExchange := c.Data()

	parts := strings.Split(symbolWithExchange, ":")
	if len(parts) != 2 {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}
	exchange := parts[0]
	stockCode := parts[1]

	analysis, err := t.service.TelegramBotService.AnalyzeStockAI(ctx, c, symbolWithExchange)
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	data := &dto.RequestSetPositionData{
		UserTelegram:  userTelegram,
		StockCode:     stockCode,
		Exchange:      exchange,
		BuyPrice:      analysis.MarketPrice,
		TakeProfit:    analysis.TargetPrice,
		StopLoss:      analysis.StopLoss,
		BuyDate:       utils.TimeNowWIB().Format("2006-01-02"),
		MaxHolding:    5,
		AlertPrice:    true,
		AlertMonitor:  true,
		SourceType:    model.StockPositionSourceTypeAI,
		IsMessageEdit: true,
		PlanScore:     analysis.TechnicalScore,
	}

	if err := t.service.TelegramBotService.SetStockPosition(ctx, data); err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternalSetPosition)
		return err
	}

	return t.showSetPositionSuccess(ctx, c, data)
}
