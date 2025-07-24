package service

import (
	"context"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/contract"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/telegram"
	"golang-trading/pkg/utils"
	"strings"

	"gopkg.in/telebot.v3"
)

type SendSignalService interface {
	contract.SignalContract
}

type sendSignalService struct {
	cfg                      *config.Config
	log                      *logger.Logger
	stockPositionsRepository repository.StockPositionsRepository
	userSignalAlertRepo      repository.UserSignalAlertRepository
	telegram                 *telegram.TelegramRateLimiter
	TradingPlanContract      contract.TradingPlanContract
}

func NewSendSignalService(
	cfg *config.Config,
	log *logger.Logger,
	telegram *telegram.TelegramRateLimiter,
	stockPositionsRepository repository.StockPositionsRepository,
	userSignalAlertRepo repository.UserSignalAlertRepository,
	tradingPlanContract contract.TradingPlanContract,
) SendSignalService {
	return &sendSignalService{
		cfg:                      cfg,
		log:                      log,
		telegram:                 telegram,
		stockPositionsRepository: stockPositionsRepository,
		userSignalAlertRepo:      userSignalAlertRepo,
		TradingPlanContract:      tradingPlanContract,
	}
}

func (s *sendSignalService) SendBuySignal(ctx context.Context, analyses []model.StockAnalysis, minScore float64) (bool, error) {
	withUser := utils.WithPreload("User")

	if len(analyses) == 0 {
		s.log.Warn("No stock analysis found")
		return false, nil
	}

	exchange := analyses[0].Exchange

	userSignalAlerts, err := s.userSignalAlertRepo.Get(ctx, &model.GetUserSignalAlertParam{
		IsActive: utils.ToPointer(true),
		Exchange: utils.ToPointer(exchange),
	}, withUser)
	if err != nil {
		s.log.Error("Failed to get user signal alert", logger.ErrorField(err))
		return false, err
	}

	if len(userSignalAlerts) == 0 {
		s.log.Debug("No user signal alert found")
		return false, nil
	}

	tradePlan, err := s.TradingPlanContract.CreateTradePlan(ctx, analyses)
	if err != nil {
		s.log.Error("Failed to create trade plan", logger.ErrorField(err))
		return false, err
	}

	if tradePlan == nil || tradePlan.RiskReward == 0 {
		s.log.Warn("No trade plan found", logger.StringField("stock_code", analyses[0].StockCode))
		return false, nil
	}

	defaultMinScore := s.cfg.Trading.BuySignalScore
	if minScore == 0 {
		defaultMinScore = minScore
	}

	isBuySignal := tradePlan.IsBuySignal &&
		tradePlan.Status == string(dto.Safe) &&
		tradePlan.RiskReward > s.cfg.Trading.RiskRewardRatio &&
		tradePlan.Score > defaultMinScore &&
		tradePlan.PlanType == dto.PlanTypePrimary

	if !isBuySignal {
		s.log.DebugContext(ctx, "Not buy signal",
			logger.StringField("stock_code", analyses[0].StockCode),
			logger.StringField("exchange", exchange),
			logger.StringField("risk_reward", fmt.Sprintf("%.2f", tradePlan.RiskReward)),
			logger.StringField("score", fmt.Sprintf("%.2f", tradePlan.Score)),
		)
		return false, nil
	}

	positions, err := s.stockPositionsRepository.Get(ctx, dto.GetStockPositionsParam{
		StockCodes: []string{analyses[0].StockCode},
		Exchange:   utils.ToPointer(exchange),
		IsActive:   utils.ToPointer(true),
	})
	if err != nil {
		s.log.Error("Failed to get stock position", logger.ErrorField(err))
		return false, err
	}

	userMap := map[uint]model.User{}
	for _, user := range userSignalAlerts {
		userMap[user.UserID] = user.User
	}

	for _, position := range positions {
		if _, ok := userMap[position.UserID]; ok {
			delete(userMap, position.UserID)
		}
	}

	if len(userMap) == 0 {
		s.log.Debug("No user to send signal")
		return false, nil
	}

	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("<b>🟢 Signal BUY - %s:%s</b>\n", exchange, analyses[0].StockCode))
	sb.WriteString(fmt.Sprintf("<i>📅 Update: %s</i>\n", utils.PrettyDate(utils.TimeNowWIB())))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("💰 <b>Entry</b>: %s\n", utils.FormatPrice(tradePlan.Entry, exchange)))
	sb.WriteString(fmt.Sprintf("🎯 <b>Take Profit</b>: %s %s\n", utils.FormatPrice(tradePlan.TakeProfit, exchange), utils.FormatChangeWithIcon(tradePlan.Entry, tradePlan.TakeProfit)))
	sb.WriteString(fmt.Sprintf("🛡️ <b>Stop Loss</b>: %s %s\n", utils.FormatPrice(tradePlan.StopLoss, exchange), utils.FormatChangeWithIcon(tradePlan.Entry, tradePlan.StopLoss)))
	sb.WriteString(fmt.Sprintf("📊 <b>Risk Reward</b>: %s \n", fmt.Sprintf("%.2f", tradePlan.RiskReward)))
	sb.WriteString(fmt.Sprintf("🔎 <b>Score</b>: %s (%s)\n", fmt.Sprintf("%.2f", tradePlan.Score), tradePlan.TechnicalSignal))
	sb.WriteString(fmt.Sprintf("<b>%s Plan</b>\n", tradePlan.PlanType.String()))
	sb.WriteString("\n")
	sb.WriteString("📝 <b>Insights:</b>\n")
	for _, insight := range tradePlan.Insights {
		sb.WriteString(fmt.Sprintf("- %s\n", insight.Text))
	}

	sb.WriteString("\n")

	sb.WriteString("👉 <i>Klik tombol di bawah ini untuk melihat detail analisa</i>")
	menu := &telebot.ReplyMarkup{}
	btnAnalyze := menu.Data("📄 Detail Analisa", "btn_general_analisis", fmt.Sprintf("%s:%s", analyses[0].Exchange, analyses[0].StockCode))
	btnDeleteMessage := menu.Data("🗑️ Hapus Pesan", "btn_delete_message")
	menu.Inline(menu.Row(btnAnalyze, btnDeleteMessage))

	for _, user := range userMap {
		errSend := s.telegram.SendMessageUser(ctx, sb.String(), user.TelegramID, menu, telebot.ModeHTML)
		if errSend != nil {
			s.log.ErrorContextWithAlert(ctx, "Failed to send buy signal", logger.ErrorField(errSend))
		}
	}
	return true, nil
}
