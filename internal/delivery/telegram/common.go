package telegram

import (
	"context"
	"fmt"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strings"
	"sync"
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

type Progress struct {
	Index     int
	StockCode string
	ID        string
	Content   string
	Header    string
	BuySymbol []string
	Menu      *telebot.ReplyMarkup
}

func (t *TelegramBotHandler) showProgressBarWithChannel(
	ctx context.Context,
	c telebot.Context,
	msgRoot *telebot.Message,
	progressCh <-chan Progress,
	totalSteps int,
	wg *sync.WaitGroup,
) {
	utils.GoSafe(func() {
		const barLength = 15 // total panjang bar, bisa diubah sesuai estetika

		current := Progress{Index: 0}

		defer func() {
			result := fmt.Sprintf("%s\n%s", current.Header, current.Content)

			menu := &telebot.ReplyMarkup{}
			if current.Menu != nil {
				menu = current.Menu
			}
			_, errInner := t.telegram.Edit(ctx, c, msgRoot, result, menu, telebot.ModeHTML)
			if errInner != nil {
				t.log.ErrorContext(ctx, "Gagal edit pesan", logger.ErrorField(errInner))
			}
			wg.Done()
		}()

		for {
			select {
			case <-ctx.Done():
				t.log.ErrorContext(ctx, "Done signal received", logger.ErrorField(ctx.Err()))
				return
			case newProgress, ok := <-progressCh:
				if !ok {
					t.log.WarnContext(ctx, "showProgressBarWithChannel - Progress channel closed")
					return
				}

				current = newProgress

				// Hitung persen dan jumlah "blok" progress
				percent := int(float64(current.Index) / float64(totalSteps) * 100)
				progressBlocks := int(float64(barLength) * float64(current.Index) / float64(totalSteps))
				if progressBlocks > barLength {
					progressBlocks = barLength
				}

				// Buat bar: ▓ untuk progress, ░ untuk sisanya
				currentAnalysis := fmt.Sprintf(messageLoadingAnalysis, current.StockCode)
				filled := strings.Repeat("▓", progressBlocks)
				empty := strings.Repeat("░", barLength-progressBlocks)
				progressBar := fmt.Sprintf("⏳ Progress: [%s%s] %d%%", filled, empty, percent)

				menu := &telebot.ReplyMarkup{}
				btnCancel := menu.Data(btnCancelBuyListAnalysis.Text, btnCancelBuyListAnalysis.Unique)
				menu.Inline(menu.Row(btnCancel))

				body := &strings.Builder{}
				body.WriteString(current.Header)
				body.WriteString("\n")

				if current.Content != "" {
					body.WriteString(current.Content)
					body.WriteString("\n\n")
				}

				body.WriteString(currentAnalysis)
				body.WriteString("\n")
				body.WriteString(progressBar)

				time.Sleep(100 * time.Millisecond)

				if msgRoot == nil {
					msgNew, err := t.telegram.Send(ctx, c, body.String(), menu, telebot.ModeHTML)
					if err != nil {
						t.log.ErrorContext(ctx, "Gagal create progress bar", logger.ErrorField(err))
					}
					msgRoot = msgNew
				} else {
					_, err := t.telegram.Edit(ctx, c, msgRoot, body.String(), menu, telebot.ModeHTML)
					if err != nil {
						t.log.ErrorContext(ctx, "Gagal update progress bar", logger.ErrorField(err))
					}
				}

			}
		}
	}).Run()
}
