package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/contract"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
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
	inmemoryCache            cache.Cache
}

func NewSendSignalService(
	cfg *config.Config,
	log *logger.Logger,
	telegram *telegram.TelegramRateLimiter,
	stockPositionsRepository repository.StockPositionsRepository,
	userSignalAlertRepo repository.UserSignalAlertRepository,
	tradingPlanContract contract.TradingPlanContract,
	inmemoryCache cache.Cache,
) SendSignalService {
	return &sendSignalService{
		cfg:                      cfg,
		log:                      log,
		telegram:                 telegram,
		stockPositionsRepository: stockPositionsRepository,
		userSignalAlertRepo:      userSignalAlertRepo,
		TradingPlanContract:      tradingPlanContract,
		inmemoryCache:            inmemoryCache,
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
		tradePlan.PlanType != dto.PlanTypeATR && tradePlan.Entry >= tradePlan.CurrentMarketPrice*0.98

	if !isBuySignal {
		s.log.DebugContext(ctx, "Not buy signal",
			logger.StringField("stock_code", analyses[0].StockCode),
			logger.StringField("exchange", exchange),
			logger.StringField("risk_reward", fmt.Sprintf("%.2f", tradePlan.RiskReward)),
			logger.StringField("score", fmt.Sprintf("%.2f", tradePlan.Score)),
		)
		return false, nil
	}

	hashIdentifier := s.GenerateHashIdentifier(ctx, analyses, tradePlan.Score, tradePlan.Entry)
	_, alreadySent := s.inmemoryCache.Get(fmt.Sprintf(common.KEY_LAST_SEND_SIGNAL_BUY, hashIdentifier))
	if alreadySent {
		s.log.DebugContext(ctx, "Buy signal already sent", logger.StringField("stock_code", analyses[0].StockCode))
		return false, nil
	} else {
		s.inmemoryCache.Set(fmt.Sprintf(common.KEY_LAST_SEND_SIGNAL_BUY, hashIdentifier), true, s.cfg.Trading.BuySignalCacheDuration)
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
	sb.WriteString("<b>📝 Penjelasan Entry,SL & TP</b>\n")
	sb.WriteString(fmt.Sprintf("<b>🚀 Entry</b> %s\n", tradePlan.EntryReason))
	sb.WriteString(fmt.Sprintf("<b>🛡️ Stop Loss</b> ditentukan berdasarkan %s\n", tradePlan.SLReason))
	sb.WriteString(fmt.Sprintf("<b>🎯 Take Profit</b> berasal dari %s\n", tradePlan.TPReason))
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

func (s *sendSignalService) GenerateHashIdentifier(ctx context.Context, analyses []model.StockAnalysis, score, marketPrice float64) string {

	parts := []string{
		fmt.Sprintf("%s:%s", analyses[0].Exchange, analyses[0].StockCode),
		fmt.Sprintf("%f", marketPrice),
		fmt.Sprintf("%f", score),
	}

	hashInput := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hash[:])
}
