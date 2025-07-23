package service

import (
	"context"
	"encoding/json"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/logger"
	"sort"
	"time"
)

// BacktestService mendefinisikan interface untuk layanan backtesting.
type BacktestService interface {
	RunBacktest(ctx context.Context, req dto.BacktestRequest) (*dto.BacktestResult, error)
}

type backtestService struct {
	log               *logger.Logger
	tradingService    TradingService
	stockAnalysisRepo repository.StockAnalysisRepository
}

// NewBacktestService membuat instance baru dari backtestService.
func NewBacktestService(
	log *logger.Logger,
	tradingService TradingService,
	stockAnalysisRepo repository.StockAnalysisRepository,
) BacktestService {
	return &backtestService{
		log:               log,
		tradingService:    tradingService,
		stockAnalysisRepo: stockAnalysisRepo,
	}
}

// RunBacktest menjalankan simulasi trading berdasarkan data historis.
func (s *backtestService) RunBacktest(ctx context.Context, req dto.BacktestRequest) (*dto.BacktestResult, error) {
	// 1. Ambil semua data analisis historis untuk rentang waktu yang diberikan.
	historicalData, err := s.stockAnalysisRepo.GetHistoricalAnalyses(ctx, req.StockCode, req.Exchange, req.StartDate, req.EndDate)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get historical analyses for backtest", logger.ErrorField(err))
		return nil, err
	}

	if len(historicalData) == 0 {
		reqBytes, _ := json.Marshal(req)
		s.log.InfoContext(ctx, "No historical data found for the given backtest request", logger.StringField("request", string(reqBytes)))
		return &dto.BacktestResult{StockCode: req.StockCode, StartDate: req.StartDate, EndDate: req.EndDate}, nil
	}

	// 2. Kelompokkan data berdasarkan hari (timestamp).
	analysesByDay := make(map[time.Time][]model.StockAnalysis)
	var sortedDays []time.Time

	for _, analysis := range historicalData {
		day := analysis.Timestamp.Truncate(24 * time.Hour)
		if _, ok := analysesByDay[day]; !ok {
			sortedDays = append(sortedDays, day)
		}
		analysesByDay[day] = append(analysesByDay[day], analysis)
	}
	// Urutkan hari secara kronologis
	sort.Slice(sortedDays, func(i, j int) bool {
		return sortedDays[i].Before(sortedDays[j])
	})

	var currentPosition *model.StockPosition
	var tradeLogs []dto.TradeLog

	// 3. Iterasi melalui setiap hari dalam data historis
	for _, day := range sortedDays {
		analysesForDay := analysesByDay[day]
		if len(analysesForDay) == 0 {
			continue
		}

		// Ambil harga pasar dari data analisis terakhir pada hari itu
		marketPrice := analysesForDay[len(analysesForDay)-1].MarketPrice

		// Jika ada posisi yang sedang terbuka
		if currentPosition != nil {
			// Cek apakah harga menyentuh SL atau TP
			if marketPrice <= currentPosition.StopLossPrice {
				tradeLogs = append(tradeLogs, closePosition(currentPosition, day, marketPrice, "Stop Loss Hit"))
				currentPosition = nil
				continue // Lanjut ke hari berikutnya setelah menutup posisi
			}
			if marketPrice >= currentPosition.TakeProfitPrice {
				tradeLogs = append(tradeLogs, closePosition(currentPosition, day, marketPrice, "Take Profit Hit"))
				currentPosition = nil
				continue
			}

			// Jika tidak ada SL/TP yang kena, evaluasi posisi menggunakan logika yang ada
			// Untuk evaluasi, kita butuh support & resistance, jadi kita hitung dulu
			supports, resistances, err := s.tradingService.CalculateSupportResistance(ctx, analysesForDay)
			if err != nil {
				s.log.WarnContext(ctx, "Failed to calculate S/R during backtest, skipping day", logger.ErrorField(err), logger.StringField("date", day.String()))
				continue
			}

			posAnalysis, err := s.tradingService.EvaluatePositionMonitoring(ctx, currentPosition, analysesForDay, supports, resistances)
			if err != nil {
				s.log.WarnContext(ctx, "Failed to evaluate position during backtest, skipping day", logger.ErrorField(err), logger.StringField("date", day.String()))
				continue
			}

			// Cek sinyal untuk keluar dari posisi
			if posAnalysis.Signal == dto.CutLoss || posAnalysis.Signal == dto.TakeProfit {
				tradeLogs = append(tradeLogs, closePosition(currentPosition, day, marketPrice, string(posAnalysis.Signal)))
				currentPosition = nil
				continue
			} else if posAnalysis.Signal == dto.TrailingStop && posAnalysis.TrailingStopPrice > currentPosition.StopLossPrice {
				// Update trailing stop loss
				currentPosition.StopLossPrice = posAnalysis.TrailingStopPrice
			}

		} else {
			// Jika tidak ada posisi terbuka, coba buat rencana trading baru
			tradePlan, err := s.tradingService.CreateTradePlan(ctx, analysesForDay)
			if err != nil {
				s.log.WarnContext(ctx, "Failed to create trade plan during backtest, skipping day", logger.ErrorField(err), logger.StringField("date", day.String()))
				continue
			}

			if tradePlan != nil && tradePlan.IsBuySignal && tradePlan.Score >= 50 { // Entry threshold
				currentPosition = &model.StockPosition{
					StockCode:       req.StockCode,
					Exchange:        req.Exchange,
					BuyPrice:        tradePlan.Entry,
					TakeProfitPrice: tradePlan.TakeProfit,
					StopLossPrice:   tradePlan.StopLoss,
					BuyDate:         day,
				}
			}
		}
	}

	result := calculateBacktestResult(req, tradeLogs)

	s.log.InfoContext(ctx, "Backtest simulation completed", logger.StringField("stock_code", req.StockCode), logger.IntField("total_trades", result.TotalTrades))
	return result, nil
}

// closePosition adalah helper untuk menutup posisi dan membuat log transaksi.
func closePosition(pos *model.StockPosition, exitDate time.Time, exitPrice float64, reason string) dto.TradeLog {
	pl := exitPrice - pos.BuyPrice
	holdingDays := int(exitDate.Sub(pos.BuyDate).Hours() / 24)
	if holdingDays == 0 {
		holdingDays = 1
	}

	return dto.TradeLog{
		Symbol:        pos.StockCode,
		EntryDate:     pos.BuyDate,
		EntryPrice:    pos.BuyPrice,
		ExitDate:      exitDate,
		ExitPrice:     exitPrice,
		ExitReason:    reason,
		ProfitLoss:    pl,
		HoldingPeriod: holdingDays,
	}
}

// calculateBacktestResult menghitung semua metrik kinerja dari log perdagangan.
func calculateBacktestResult(req dto.BacktestRequest, trades []dto.TradeLog) *dto.BacktestResult {
	result := &dto.BacktestResult{
		StockCode: req.StockCode,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Trades:    trades,
	}

	if len(trades) == 0 {
		return result
	}

	var totalHoldingPeriod int
	for _, trade := range trades {
		result.TotalTrades++
		result.TotalProfitLoss += trade.ProfitLoss
		totalHoldingPeriod += trade.HoldingPeriod

		if trade.ProfitLoss > 0 {
			result.WinningTrades++
			result.TotalProfit += trade.ProfitLoss
		} else {
			result.LosingTrades++
			result.TotalLoss += trade.ProfitLoss // Loss is negative
		}
	}

	if result.TotalTrades > 0 {
		result.WinRate = (float64(result.WinningTrades) / float64(result.TotalTrades)) * 100
		result.AvgHoldingPeriod = float64(totalHoldingPeriod) / float64(result.TotalTrades)
	}

	if result.TotalLoss != 0 {
		result.ProfitFactor = result.TotalProfit / -result.TotalLoss
	}

	// TODO: Implement MaxDrawdown calculation

	return result
}
