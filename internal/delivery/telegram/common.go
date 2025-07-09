package telegram

import (
	"context"
	"fmt"
	"golang-trading/pkg/cache"
	"strings"

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
