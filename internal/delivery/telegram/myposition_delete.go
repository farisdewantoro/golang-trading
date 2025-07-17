package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/dto"
	"strconv"
	"strings"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleBtnConfirmDeleteStockPosition(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID

	data := c.Data()
	stockPositionIDInt, err := strconv.Atoi(data)
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternal)
		return err
	}

	stockPosition, err := t.service.TelegramBotService.GetStockPositions(ctx, dto.GetStockPositionsParam{
		TelegramID: &userID,
		IDs:        []uint{uint(stockPositionIDInt)},
	})
	if err != nil {
		_, err := t.telegram.Send(ctx, c, commonErrorInternal)
		return err
	}
	if len(stockPosition) == 0 {
		_, err := t.telegram.Send(ctx, c, commonErrorInternal)
		return err
	}

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf(`<b>🗑 Konfirmasi Hapus Posisi Saham</b>

Apakah kamu yakin ingin menghapus posisi ini?
- Symbol : %s:%s
- Entry : %.2f
- Buy Date : %s
- Take Profit : %.2f
- Stop Loss : %.2f

<b><i>👇 Klik tombol di bawah untuk konfirmasi.</i></b>
`, stockPosition[0].Exchange, stockPosition[0].StockCode, stockPosition[0].BuyPrice, stockPosition[0].BuyDate.Format("2006-01-02"), stockPosition[0].TakeProfitPrice, stockPosition[0].StopLossPrice))

	menu := &telebot.ReplyMarkup{}
	menu.Inline(menu.Row(menu.Data("❌ Batal", btnDeleteMessage.Unique), menu.Data("✅ Hapus Posisi", btnDeleteStockPosition.Unique, fmt.Sprintf("%d", stockPosition[0].ID))))

	_, err = t.telegram.Edit(ctx, c, c.Message(), sb.String(), menu, telebot.ModeHTML)
	return err

}

func (t *TelegramBotHandler) handleBtnDeleteStockPosition(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID

	t.telegram.Respond(ctx, c, &telebot.CallbackResponse{
		Text:      "🔄 Menghapus....",
		ShowAlert: false,
	})
	stockPositionID := c.Data()

	stockPositionIDInt, err := strconv.Atoi(stockPositionID)
	if err != nil {
		_, err := t.telegram.Edit(ctx, c, c.Message(), fmt.Sprintf("❌ Gagal mengambil posisi untuk %s: %s", stockPositionID, err.Error()))
		return err
	}

	if err := t.service.TelegramBotService.DeleteStockPositionTelegramUser(ctx, userID, uint(stockPositionIDInt)); err != nil {
		_, err := t.telegram.Edit(ctx, c, c.Message(), fmt.Sprintf("❌ Gagal menghapus posisi untuk %s: %s", stockPositionID, err.Error()))
		return err
	}

	_, err = t.telegram.Edit(ctx, c, c.Message(), "✅ Posisi berhasil dihapus")
	return err
}
