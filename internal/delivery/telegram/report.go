package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strings"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleReport(ctx context.Context, c telebot.Context) error {
	telegramID := c.Sender().ID

	param := dto.GetStockPositionsParam{
		TelegramID: &telegramID,
		IsExit:     utils.ToPointer(true),
		SortBy:     utils.ToPointer("exit_date"),
		SortOrder:  utils.ToPointer("desc"),
	}
	stockPositions, err := t.service.TelegramBotService.GetStockPositions(ctx, param)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to get stock positions for report", logger.ErrorField(err))
		_, errSend := t.telegram.Send(ctx, c, commonErrorInternalReport)
		if errSend != nil {
			t.log.ErrorContext(ctx, "Failed to send internal error message", logger.ErrorField(errSend))
		}
		return err
	}

	if len(stockPositions) == 0 {
		msgNotExist := `📭 *Belum Ada Riwayat Trading*

Kamu belum memiliki data trading yang bisa ditampilkan.

📌 Berikut alur untuk mulai mencatat performa trading kamu:

1️⃣ Gunakan perintah */setposition* untuk mencatat saat kamu masuk posisi (BUY/SELL).

2️⃣ Setelah keluar dari posisi, klik tombol *Exit Posisi* dan isi form exit (harga keluar, tanggal, dll).

3️⃣ Setelah posisi ditutup, kamu bisa menggunakan perintah */report* untuk melihat performa trading kamu.

💡 Data baru akan muncul di report setelah kamu menyelesaikan langkah di atas minimal 1 kali.`

		_, errSend := t.telegram.Send(ctx, c, msgNotExist, telebot.ModeMarkdown)
		if errSend != nil {
			t.log.ErrorContext(ctx, "Failed to send no exit positions message", logger.ErrorField(errSend))
		}
		return errSend
	}

	return t.showReport(ctx, c, stockPositions)
}

func (t *TelegramBotHandler) showReport(ctx context.Context, c telebot.Context, positions []model.StockPosition) error {
	sb := &strings.Builder{}
	// header
	sb.WriteString("📊 <b>Trading Report</b>\n")
	sb.WriteString("Laporan ini menampilkan ringkasan performa dari posisi trading yang sudah selesai. Gunakan sebagai bahan evaluasi untuk strategi swing trading kamu.\n")

	sbBody := &strings.Builder{}
	sbBody.WriteString("\n\n🔎 Detail Saham:\n")

	countWin := 0
	countLose := 0
	countPnL := 0.0
	for _, position := range positions {
		if position.ExitPrice == nil {
			continue
		}

		if *position.ExitPrice >= position.BuyPrice {
			countWin++
		} else {
			countLose++
		}

		countPnL += utils.CalculateChangePercent(position.BuyPrice, *position.ExitPrice)

		symbolWithExchange := fmt.Sprintf("%s:%s", position.Exchange, position.StockCode)
		sbBody.WriteString(fmt.Sprintf("\n<b>─ %s</b>\n", symbolWithExchange))
		sbBody.WriteString(fmt.Sprintf("- Date: %s - %s\n", position.BuyDate.Format("01/02"), position.ExitDate.Format("01/02")))
		sbBody.WriteString(fmt.Sprintf("- E/X: %d ⮕ %d %s\n", int(position.BuyPrice), int(*position.ExitPrice), utils.FormatChangeWithIcon(position.BuyPrice, *position.ExitPrice)))
		sbBody.WriteString(fmt.Sprintf("- Score (Pos): %.2f ⮕ %.2f\n", position.InitialScore, position.FinalScore))
		sbBody.WriteString(fmt.Sprintf("- Score (Plan): %.2f\n", position.PlanScore))
	}

	sbSummary := &strings.Builder{}
	sbSummary.WriteString(fmt.Sprintf("\n🟢 <b>Win</b>: %d | 🔴 Lose: %d", countWin, countLose))
	sbSummary.WriteString(fmt.Sprintf("\n📈 <b>Total PnL</b>: %s", utils.FormatChgIcon(countPnL)))
	sbSummary.WriteString(fmt.Sprintf("\n🏆 <b>Win Rate</b>: %.2f%%", float64(countWin)/float64(len(positions))*100))

	result := fmt.Sprintf("%s%s%s", sb.String(), sbSummary.String(), sbBody.String())
	_, err := t.telegram.Send(ctx, c, result, telebot.ModeHTML)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to send report message", logger.ErrorField(err))
		return err
	}
	return nil
}
