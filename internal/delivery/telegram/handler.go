package telegram

import (
	"context"
	"golang-trading/internal/dto"
	"golang-trading/pkg/logger"
	"net/http"

	"github.com/labstack/echo/v4"
	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) WithContext(handler func(ctx context.Context, c telebot.Context) error) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		ctx, cancel := context.WithTimeout(t.ctx, t.cfg.Telegram.TimeoutDuration)
		defer cancel()

		return handler(ctx, c)
	}
}

func (t *TelegramBotHandler) RegisterHandlers() {
	t.echo.POST("/api/v1/telegram/webhook", func(c echo.Context) error {
		var update telebot.Update
		if err := c.Bind(&update); err != nil {
			t.log.ErrorContext(t.ctx, "Cannot bind JSON", logger.ErrorField(err))
			badRequest := dto.NewBadRequestResponse(err.Error())
			return c.JSON(http.StatusBadRequest, badRequest)
		}
		t.bot.ProcessUpdate(update)
		return c.JSON(http.StatusOK, dto.NewBaseResponse(http.StatusOK, "ok", nil))
	})

	t.bot.Handle("/start", t.WithContext(t.handleStart))
	t.bot.Handle("/cancel", (t.handleCancel))
	t.bot.Handle("/help", t.WithContext(t.handleHelp))
	t.bot.Handle("/analyze", t.WithContext(t.handleStartAnalyze))
	t.bot.Handle("/setposition", t.WithContext(t.handleSetPosition), t.IsOnConversationMiddleware())
	t.bot.Handle("/myposition", t.WithContext(t.handleMyPosition))
	t.bot.Handle("/report", t.WithContext(t.handleReport))
	t.bot.Handle("/buylist", t.WithContext(t.handleBuyList))
	t.bot.Handle("/scheduler", t.WithContext(t.handleScheduler))
	t.bot.Handle("/alertsignal", t.WithContext(t.handleAlertSignal))

	t.bot.Handle(telebot.OnText, t.WithContext(t.handleConversation))

	t.bot.Handle(&btnAskAIAnalyzer, t.WithContext(t.handleAskAIAnalyzer))
	t.bot.Handle(&btnGeneralAnalisis, t.WithContext(t.handleBtnGeneralAnalysis))
	t.bot.Handle(&btnRefreshAnalysis, t.WithContext(t.handleBtnRefreshAnalysis))

	// set position
	t.bot.Handle(&btnSetPositionAlertPrice, t.WithContext(t.handleBtnSetPositionAlertPrice))
	t.bot.Handle(&btnSetPositionAlertMonitor, t.WithContext(t.handleBtnSetPositionAlertMonitor))
	t.bot.Handle(&btnSetPositionTechnical, t.WithContext(t.handleBtnSetPositionByTechnical))

	// common
	t.bot.Handle(&btnCancelGeneral, t.handleCancel)
	t.bot.Handle(&btnDeleteMessage, t.WithContext(t.handleBtnDeleteMessage))

	// my position
	t.bot.Handle(&btnToDetailStockPosition, t.WithContext(t.handleBtnToDetailStockPosition))
	t.bot.Handle(&btnDeleteStockPosition, t.WithContext(t.handleBtnDeleteStockPosition))
	t.bot.Handle(&btnConfirmDeleteStockPosition, t.WithContext(t.handleBtnConfirmDeleteStockPosition))
	t.bot.Handle(&btnBackStockPosition, t.WithContext(t.handleBtnBackStockPosition))
	t.bot.Handle(&btnRefreshAnalysisPosition, t.WithContext(t.handleBtnRefreshAnalysisPosition))

	// exit position
	t.bot.Handle(&btnExitStockPosition, t.WithContext(t.handleBtnExitStockPosition))
	t.bot.Handle(&btnSaveExitPosition, t.WithContext(t.handleBtnSaveExitPosition))

	//buylist
	t.bot.Handle(&btnShowBuyListAnalysis, t.WithContext(t.handleBtnShowBuyListAnalysis))

	//schedule
	t.bot.Handle(&btnDetailJob, t.WithContext(t.handleBtnDetailJob))
	t.bot.Handle(&btnActionBackToJobList, t.WithContext(t.handleBtnActionBackToJobList))
	t.bot.Handle(&btnActionRunJob, t.WithContext(t.handleBtnActionRunJob))

	// alert signal
	t.bot.Handle(&btnAlertSignal, t.WithContext(t.handleBtnAlertSignal))

}
