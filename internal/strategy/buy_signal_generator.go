package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/contract"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"sync"
	"time"
)

type BuySignalGenerator interface {
	JobExecutionStrategy
}

type BuySignalGeneratorStrategy struct {
	cfg               *config.Config
	log               *logger.Logger
	stockAnalysisRepo repository.StockAnalysisRepository
	candleRepository  repository.CandleRepository
	inmemoryCache     cache.Cache
	signalContract    contract.SignalContract
}

func NewBuySignalGeneratorStrategy(
	cfg *config.Config,
	log *logger.Logger,
	candleRepository repository.CandleRepository,
	inmemoryCache cache.Cache,
	signalContract contract.SignalContract,
	stockAnalysisRepo repository.StockAnalysisRepository,
) BuySignalGenerator {
	return &BuySignalGeneratorStrategy{
		cfg:               cfg,
		log:               log,
		candleRepository:  candleRepository,
		inmemoryCache:     inmemoryCache,
		signalContract:    signalContract,
		stockAnalysisRepo: stockAnalysisRepo,
	}
}

type BuySignalGeneratorPayload struct {
	LatestAnalysisDuration string  `json:"latest_analysis_duration"`
	MaxConcurrency         int     `json:"max_concurrency"`
	Range                  string  `json:"range"`
	Interval               string  `json:"interval"`
	LastPriceCacheDuration string  `json:"last_price_cache_duration"`
	Score                  float64 `json:"score"`
	Exchange               string  `json:"exchange"`
}

type BuySignalGeneratorResult struct {
	Symbol string `json:"symbol"`
	IsSent bool   `json:"is_sent"`
	Error  string `json:"error,omitempty"`
}

func (s *BuySignalGeneratorStrategy) GetType() JobType {
	return JobTypeBuySignalGenerator
}

func (s *BuySignalGeneratorStrategy) Execute(ctx context.Context, job *model.Job) (JobResult, error) {
	var payload BuySignalGeneratorPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		s.log.ErrorContext(ctx, "Failed to unmarshal job payload", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to unmarshal job payload: %v", err)}, fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	latestAnalysisDuration, err := time.ParseDuration(payload.LatestAnalysisDuration)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to parse latest analysis duration", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to parse latest analysis duration: %v", err)}, fmt.Errorf("failed to parse latest analysis duration: %w", err)
	}

	lastPriceCacheDuration, err := time.ParseDuration(payload.LastPriceCacheDuration)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to parse last price cache duration", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to parse last price cache duration: %v", err)}, fmt.Errorf("failed to parse last price cache duration: %w", err)
	}

	analyses, err := s.stockAnalysisRepo.GetLatestAnalyses(ctx, model.GetLatestAnalysisParam{
		TimestampAfter: utils.TimeNowWIB().Add(-latestAnalysisDuration),
		Exchange:       payload.Exchange,
	})
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get latest analyses", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to get latest analyses: %v", err)}, fmt.Errorf("failed to get latest analyses: %w", err)
	}

	if len(analyses) == 0 {
		return JobResult{ExitCode: JOB_EXIT_CODE_SKIPPED, Output: "no latest analyses found"}, nil
	}

	semaphore := make(chan struct{}, payload.MaxConcurrency)
	var (
		wg     sync.WaitGroup
		result []BuySignalGeneratorResult
		mu     sync.Mutex
	)

	mapSymbolExchangeAnalysis := map[string][]model.StockAnalysis{}
	for _, analysis := range analyses {
		symbolWitExchange := analysis.Exchange + ":" + analysis.StockCode

		if _, ok := mapSymbolExchangeAnalysis[symbolWitExchange]; !ok {
			mapSymbolExchangeAnalysis[symbolWitExchange] = []model.StockAnalysis{}
		}
		mapSymbolExchangeAnalysis[symbolWitExchange] = append(mapSymbolExchangeAnalysis[symbolWitExchange], analysis)
	}

	for _, analyses := range mapSymbolExchangeAnalysis {
		semaphore <- struct{}{}
		wg.Add(1)
		analyses := analyses
		utils.GoSafe(func() {
			defer func() {
				<-semaphore
				wg.Done()

			}()

			analysis := analyses[0]
			tempResult := BuySignalGeneratorResult{
				Symbol: analyses[0].StockCode,
			}
			defer func() {
				mu.Lock()
				result = append(result, tempResult)
				mu.Unlock()
			}()

			candles, err := s.candleRepository.Get(ctx, dto.GetStockDataParam{
				StockCode: analysis.StockCode,
				Exchange:  analysis.Exchange,
				Range:     payload.Range,
				Interval:  payload.Interval,
			})
			if err != nil {
				tempResult.Error = err.Error()
				s.log.ErrorContextWithAlert(ctx, "Failed to get candles", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
				return
			}

			if candles == nil {
				tempResult.Error = "no candles found"
				s.log.DebugContext(ctx, "No candles found", logger.StringField("stock_code", analysis.StockCode), logger.StringField("exchange", analysis.Exchange))
				return
			}

			stockCodeWithExchange := analysis.Exchange + ":" + analysis.StockCode
			key := fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange)
			s.inmemoryCache.Set(key, candles.MarketPrice, lastPriceCacheDuration)

			minScore := payload.Score
			if minScore == 0 {
				minScore = s.cfg.Trading.BuySignalScore
			}

			isSend, err := s.signalContract.SendBuySignal(ctx, analyses, minScore)
			if err != nil {
				tempResult.Error = err.Error()
				s.log.ErrorContextWithAlert(ctx, "Failed to send buy signal", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
				return
			}

			tempResult.IsSent = isSend
		}).Run()
	}

	wg.Wait()

	return JobResult{ExitCode: JOB_EXIT_CODE_SUCCESS, Output: "success"}, nil
}
