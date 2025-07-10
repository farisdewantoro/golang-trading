package telegram

import (
	"context"
	"fmt"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleConversation(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID
	state, ok := cache.GetFromCache[int](fmt.Sprintf(UserStateKey, userID))
	if !ok || state == StateIdle {
		// This should not be treated as a conversation.
		// Let the generic text handler deal with it.
		return t.handleTextMessage(ctx, c)
	}

	switch {
	case state >= StateWaitingSetPositionSymbol && state <= StateWaitingSetPositionAlertMonitor:
		return t.handleSetPositionConversation(ctx, c)
	case state == StateWaitingAnalyzeSymbol:
		return t.handleAnalyzeSymbol(ctx, c)
	case state >= StateWaitingExitPositionInputExitPrice && state <= StateWaitingExitPositionConfirm:
		return t.handleExitPositionConversation(ctx, c)
	default:
		// If no specific conversation is matched, maybe it's a dangling state.
		t.ResetUserState(userID)
		_, err := t.telegram.Send(ctx, c, "Sepertinya Anda tidak sedang dalam percakapan aktif. Gunakan /help untuk melihat perintah yang tersedia.")
		return err
	}
}

func (t *TelegramBotHandler) handleTextMessage(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID

	if state, ok := cache.GetFromCache[int](fmt.Sprintf(UserStateKey, userID)); ok && state != StateIdle {
		t.handleConversation(ctx, c)
		return nil
	}

	// Cek apakah bukan command
	if !strings.HasPrefix(c.Text(), "/") {
		return c.Send("Saya tidak mengenali perintahmu. Gunakan /help untuk melihat daftar perintah.")
	}

	return nil
}

func (t *TelegramBotHandler) ResetUserState(userID int64) {
	t.inmemoryCache.Delete(fmt.Sprintf(UserStateKey, userID))
	t.inmemoryCache.Delete(fmt.Sprintf(UserDataKey, userID))
}

func (t *TelegramBotHandler) IsOnConversationMiddleware() telebot.MiddlewareFunc {
	return func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) (err error) {
			if _, inConversation := cache.GetFromCache[int](fmt.Sprintf(UserStateKey, c.Sender().ID)); inConversation {
				t.handleCancel(c)
			}
			return next(c)
		}
	}
}

func (t *TelegramBotHandler) handleCancel(c telebot.Context) error {
	userID := c.Sender().ID

	defer t.ResetUserState(userID)

	// Check if user is in any conversation state
	if state, ok := cache.GetFromCache[int](fmt.Sprintf(UserStateKey, userID)); ok && state != StateIdle {
		return c.Send("✅ Percakapan dibatalkan.")
	}

	return nil

}

func (t *TelegramBotHandler) showLoadingGeneral(ctx context.Context, c telebot.Context, stop <-chan struct{}) *telebot.Message {
	msgRoot := c.Message()

	initial := "Mohon tunggu sebentar, bot sedang memproses data"
	msg, _ := t.telegram.Edit(ctx, c, msgRoot, initial)

	utils.GoSafe(func() {
		dots := []string{"⏳", "⏳⏳", "⏳⏳⏳"}
		i := 0
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
			time.Sleep(200 * time.Millisecond)
		}
	}).Run()

	return msg
}

func (t *TelegramBotHandler) handleBtnDeleteMessage(ctx context.Context, c telebot.Context) error {
	t.telegram.Edit(ctx, c, c.Message(), "✅ Pesan akan dihapus....")
	time.Sleep(1 * time.Second)
	return t.telegram.Delete(ctx, c, c.Message())
}
