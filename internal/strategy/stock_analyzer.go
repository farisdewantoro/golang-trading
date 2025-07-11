package strategy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"strings"
	"sync"

	"gorm.io/datatypes"
)

type StockAnalyzer interface {
	JobExecutionStrategy
	AnalyzeStock(ctx context.Context, stock dto.StockInfo) ([]model.StockAnalysis, error)
}

type StockAnalyzerStrategy struct {
	cfg                            *config.Config
	logger                         *logger.Logger
	cache                          cache.Cache
	stockPositionRepo              repository.StockPositionsRepository
	tradingViewScreenersRepository repository.TradingViewScreenersRepository
	yahooFinanceRepository         repository.YahooFinanceRepository
	stockAnalysisRepo              repository.StockAnalysisRepository
	systemParamRepository          repository.SystemParamRepository
}

type StockAnalyzerPayload struct {
	TradingViewBuyListParams []map[string]interface{} `json:"trading_view_buy_list_params"`
}

type StockAnalyzerResult struct {
	StockCode string `json:"stock_code"`
	Errors    string `json:"errors"`
}

func NewStockAnalyzerStrategy(
	cfg *config.Config,
	logger *logger.Logger,
	cache cache.Cache,
	stockPositionsRepository repository.StockPositionsRepository,
	tradingViewScreenersRepository repository.TradingViewScreenersRepository,
	yahooFinanceRepository repository.YahooFinanceRepository,
	stockAnalysisRepository repository.StockAnalysisRepository,
	systemParamRepository repository.SystemParamRepository) StockAnalyzer {
	return &StockAnalyzerStrategy{
		cfg:                            cfg,
		logger:                         logger,
		cache:                          cache,
		stockPositionRepo:              stockPositionsRepository,
		tradingViewScreenersRepository: tradingViewScreenersRepository,
		yahooFinanceRepository:         yahooFinanceRepository,
		stockAnalysisRepo:              stockAnalysisRepository,
		systemParamRepository:          systemParamRepository,
	}
}

func (s *StockAnalyzerStrategy) GetType() JobType {
	return JobTypeStockAnalyzer
}

func (s *StockAnalyzerStrategy) Execute(ctx context.Context, job *model.Job) (JobResult, error) {
	var (
		payload StockAnalyzerPayload
		stocks  []dto.StockInfo
	)
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		s.logger.Error("Failed to unmarshal job payload", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to unmarshal job payload: %v", err)}, fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	mapStockCode := map[string]bool{}

	for _, params := range payload.TradingViewBuyListParams {
		buyList, err := s.tradingViewScreenersRepository.GetBuyList(ctx, params)
		if err != nil {
			s.logger.Error("Failed to get buy list", logger.ErrorField(err))
			return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to get buy list: %v", err)}, fmt.Errorf("failed to get buy list: %w", err)
		}

		for _, stock := range buyList {
			if _, ok := mapStockCode[stock.StockCode+":"+stock.Exchange]; ok {
				continue
			}

			mapStockCode[stock.StockCode+":"+stock.Exchange] = true
			stocks = append(stocks, stock)
		}
	}

	if len(stocks) == 0 {
		s.logger.Info("No stocks to analyze")
		return JobResult{ExitCode: JOB_EXIT_CODE_SKIPPED, Output: "no stocks to analyze"}, nil
	}

	wg := sync.WaitGroup{}
	results := []StockAnalyzerResult{}
	mu := sync.Mutex{}
	isHasError := false
	isHasSuccess := false

	s.logger.Debug("Start analyzing stocks", logger.IntField("total_stock", len(stocks)))

	for _, stock := range stocks {
		if !utils.ShouldContinue(ctx, s.logger) {
			s.logger.Info("Received stop signal, Stock analyzer execution stopped")
			break
		}
		wg.Add(1)
		utils.GoSafe(func() {
			defer wg.Done()
			resultData := StockAnalyzerResult{
				StockCode: stock.Exchange + ":" + stock.StockCode,
			}
			_, err := s.AnalyzeStock(ctx, stock)
			if err != nil {
				s.logger.Error("Failed to analyze stock", logger.ErrorField(err), logger.StringField("stock_code", stock.StockCode))
				resultData.Errors = err.Error()
				isHasError = true
			} else {
				isHasSuccess = true
			}

			mu.Lock()
			results = append(results, resultData)
			mu.Unlock()
		}).Run()
	}

	wg.Wait()

	s.logger.Info("Stock analyzer completed", logger.IntField("total_stock", len(stocks)))

	if len(results) == 0 {
		return JobResult{ExitCode: JOB_EXIT_CODE_SKIPPED, Output: "result is empty no stocks analyzed"}, nil
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to marshal results: %v", err)}, fmt.Errorf("failed to marshal results: %w", err)
	}

	if isHasError && isHasSuccess {
		return JobResult{ExitCode: JOB_EXIT_CODE_PARTIAL_SUCCESS, Output: string(resultJSON)}, nil
	}

	if isHasError && !isHasSuccess {
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: string(resultJSON)}, nil
	}

	return JobResult{ExitCode: JOB_EXIT_CODE_SUCCESS, Output: string(resultJSON)}, nil
}

func (s *StockAnalyzerStrategy) AnalyzeStock(ctx context.Context, stock dto.StockInfo) ([]model.StockAnalysis, error) {
	var (
		mu            sync.Mutex
		wg            sync.WaitGroup
		stockAnalyses []model.StockAnalysis
		now           = utils.TimeNowWIB()
	)

	dataTF, err := s.systemParamRepository.GetDefaultAnalysisTimeframes(ctx)
	if err != nil {
		s.logger.Error("Failed to get system parameter", logger.ErrorField(err))
		return nil, err
	}

	s.logger.Debug("Processing stock",
		logger.StringField("stock_code", stock.StockCode),
		logger.StringField("exchange", stock.Exchange),
		logger.IntField("total_timeframe", len(dataTF)),
	)

	// errgroup with context

	for _, tf := range dataTF {
		if !utils.ShouldContinue(ctx, s.logger) {
			s.logger.Info("Received stop signal, Stock analyzer stopped")
			break
		}

		tf := tf // avoid closure capture bug
		wg.Add(1)
		utils.GoSafe(func() {

			defer wg.Done()
			var stockAnalysis model.StockAnalysis

			stockData, err := s.tradingViewScreenersRepository.Get(ctx,
				fmt.Sprintf("%s:%s", stock.Exchange, stock.StockCode),
				tf.ToTradingViewScreenersInterval(),
			)
			if err != nil {
				s.logger.Error("Failed to get stock data", logger.ErrorField(err), logger.StringField("stock_code", stock.StockCode))
				return
			}

			jsonAnalysis, err := json.Marshal(stockData)
			if err != nil {
				s.logger.Error("Failed to marshal json", logger.ErrorField(err), logger.StringField("stock_code", stock.StockCode))
				return
			}

			stockAnalysis = model.StockAnalysis{
				StockCode:     stock.StockCode,
				Exchange:      stock.Exchange,
				Timeframe:     tf.Interval,
				Timestamp:     now,
				TechnicalData: datatypes.JSON(jsonAnalysis),
				Recommendation: dto.MapTradingViewScreenerRecommend(
					stockData.Recommend.Global.Summary),
			}

			stockDataOHCLV, err := s.yahooFinanceRepository.Get(ctx, dto.GetStockDataParam{
				StockCode: stock.StockCode,
				Exchange:  stock.Exchange,
				Range:     tf.Range,
				Interval:  tf.Interval,
			})
			if err != nil {
				s.logger.Error("Failed to get OHCLV data", logger.ErrorField(err), logger.StringField("stock_code", stock.StockCode))
				return
			}

			jsonOHCLV, err := json.Marshal(stockDataOHCLV.OHLCV)
			if err != nil {
				s.logger.Error("Failed to marshal OHCLV json", logger.ErrorField(err), logger.StringField("stock_code", stock.StockCode))
				return
			}

			stockAnalysis.OHLCV = datatypes.JSON(jsonOHCLV)
			stockAnalysis.MarketPrice = stockDataOHCLV.MarketPrice
			stockAnalysis.HashIdentifier = s.GenerateHashIdentifier(&stockAnalysis)

			// Append safely
			mu.Lock()
			stockAnalyses = append(stockAnalyses, stockAnalysis)
			mu.Unlock()

		}).Run()
	}

	// Wait for all goroutines
	wg.Wait()

	if len(stockAnalyses) > 0 {
		if err := s.stockAnalysisRepo.CreateBulk(ctx, stockAnalyses); err != nil {
			s.logger.Error("Failed to create stock analysis",
				logger.ErrorField(err),
				logger.StringField("stock_code", stock.StockCode),
			)
			return nil, err
		}
	}

	return stockAnalyses, nil
}

func (s *StockAnalyzerStrategy) GenerateHashIdentifier(data *model.StockAnalysis) string {
	parts := []string{
		data.StockCode,
		data.Exchange,
		fmt.Sprintf("%d", data.Timestamp.Unix()),
	}

	hashInput := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hash[:])
}
