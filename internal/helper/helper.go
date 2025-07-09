package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/logger"
)

func CalculateSummary(ctx context.Context, log *logger.Logger, dtf []dto.DataTimeframe, latestAnalyses []model.StockAnalysis) (int, int, error) {
	var totalScore int

	mapWeight := make(map[string]int)
	mainTrend := ""
	maxWeight := 0

	for _, tf := range dtf {
		mapWeight[tf.Interval] = tf.Weight
		if tf.Weight > maxWeight {
			maxWeight = tf.Weight
			mainTrend = tf.Interval
		}
	}

	mainTrendScore := -999 // Flag awal jika belum ditemukan

	for _, analysis := range latestAnalyses {
		weight, ok := mapWeight[analysis.Timeframe]
		if !ok {
			log.WarnContext(ctx, "Unknown timeframe in analysis", logger.StringField("timeframe", analysis.Timeframe))
			continue
		}

		var technicalData dto.TradingViewScanner
		if err := json.Unmarshal([]byte(analysis.TechnicalData), &technicalData); err != nil {
			log.ErrorContext(ctx, "Failed to unmarshal technical data", logger.ErrorField(err))
			continue
		}

		score := technicalData.Recommend.Global.Summary
		totalScore += weight * score

		if analysis.Timeframe == mainTrend {
			mainTrendScore = score
		}
	}

	// Pastikan main trend score ditemukan
	if mainTrendScore == -999 {
		err := fmt.Errorf("mainTrendScore for timeframe %s not found", mainTrend)
		log.ErrorContext(ctx, "Main trend score not found", logger.ErrorField(err))
		return 0, mainTrendScore, err
	}
	return totalScore, mainTrendScore, nil
}

func EvaluateSignal(ctx context.Context, log *logger.Logger, dtf []dto.DataTimeframe, latestAnalyses []model.StockAnalysis) (int, string, error) {

	totalScore, mainTrendScore, err := CalculateSummary(ctx, log, dtf, latestAnalyses)
	if err != nil {
		return 0, "", err
	}

	// Evaluasi sinyal akhir
	switch {
	case totalScore >= 9 && mainTrendScore >= dto.TradingViewSignalBuy:
		return totalScore, dto.SignalStrongBuy, nil
	case totalScore >= 6 && mainTrendScore >= dto.TradingViewSignalBuy:
		return totalScore, dto.SignalBuy, nil
	case totalScore >= 3 && mainTrendScore >= dto.TradingViewSignalNeutral:
		return totalScore, dto.SignalNeutral, nil
	default:
		return totalScore, dto.SignalSell, nil
	}
}

func EvaluatePosition(ctx context.Context, log *logger.Logger, dtf []dto.DataTimeframe, latestAnalyses []model.StockAnalysis) (int, string, error) {

	totalScore, mainTrendScore, err := CalculateSummary(ctx, log, dtf, latestAnalyses)
	if err != nil {
		return 0, "", err
	}

	// Evaluasi sinyal akhir
	switch {
	case totalScore >= 9 && mainTrendScore >= dto.TradingViewSignalBuy:
		return totalScore, dto.EvalVeryStrong, nil
	case totalScore >= 6 && mainTrendScore >= dto.TradingViewSignalBuy:
		return totalScore, dto.EvalStrong, nil
	case totalScore >= 3 && mainTrendScore >= dto.TradingViewSignalNeutral:
		return totalScore, dto.EvalNeutral, nil
	case totalScore >= 0:
		return totalScore, dto.EvalWeak, nil
	default:
		return totalScore, dto.EvalVeryWeak, nil
	}

}
