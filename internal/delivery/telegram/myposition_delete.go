package telegram

import (
	"context"
	"fmt"
	"strconv"

	"gopkg.in/telebot.v3"
)

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
