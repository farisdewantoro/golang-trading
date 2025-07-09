package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/httpclient"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/ratelimit"
	"golang-trading/pkg/utils"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/genai"
	"gorm.io/gorm"
)

type AIRepository interface {
	AnalyzeStock(ctx context.Context, techAnalyses []model.StockAnalysis) (*dto.AIAnalyzeStockResponse, error)
}

// geminiAIRepository is an implementation of NewsAnalyzerRepository that uses the Google Gemini API.
type geminiAIRepository struct {
	db             *gorm.DB
	httpClient     httpclient.HTTPClient
	cfg            *config.Config
	logger         *logger.Logger
	tokenLimiter   *ratelimit.TokenLimiter
	requestLimiter *rate.Limiter
	genAiClient    *genai.Client
}

// NewGeminiAIRepository creates a new instance of geminiAIRepository.
func NewGeminiAIRepository(db *gorm.DB, cfg *config.Config, log *logger.Logger) (AIRepository, error) {
	secondsPerRequest := time.Minute / time.Duration(cfg.Gemini.MaxRequestPerMinute)
	requestLimiter := rate.NewLimiter(rate.Every(secondsPerRequest), 1)

	tokenLimiter := ratelimit.NewTokenLimiter(cfg.Gemini.MaxTokenPerMinute)
	genAiClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: cfg.Gemini.APIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	return &geminiAIRepository{
		db:             db,
		httpClient:     httpclient.New(log, cfg.Gemini.BaseURL, cfg.Gemini.Timeout, ""),
		cfg:            cfg,
		logger:         log,
		requestLimiter: requestLimiter,
		tokenLimiter:   tokenLimiter,
		genAiClient:    genAiClient,
	}, nil
}

func (r *geminiAIRepository) AnalyzeStock(ctx context.Context, techAnalyses []model.StockAnalysis) (*dto.AIAnalyzeStockResponse, error) {

	var (
		params []dto.AIAnalyzeStockParam
		result dto.AIAnalyzeStockResponse
	)

	if len(techAnalyses) == 0 {
		r.logger.ErrorContext(ctx, "no data when analyze stock")
		return nil, fmt.Errorf("no data when analyze stock")
	}

	stockCode := techAnalyses[0].StockCode
	exchange := techAnalyses[0].Exchange
	hash := techAnalyses[0].HashIdentifier
	marketPrice := techAnalyses[0].MarketPrice
	stockAnalysisIds := []uint{}
	for _, techAnalysis := range techAnalyses {
		stockAnalysisIds = append(stockAnalysisIds, techAnalysis.ID)

		params = append(params, dto.AIAnalyzeStockParam{
			Timeframe:    techAnalysis.Timeframe,
			TechAnalysis: techAnalysis.TechnicalData,
			OHCLV:        techAnalysis.OHLCV,
			MarketPrice:  techAnalysis.MarketPrice,
		})
	}

	prompt, err := r.promptAnalyzeStock(stockCode, exchange, params)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to generate prompt when analyze stock", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to generate prompt when analyze stock: %w", err)
	}

	geminiAPIResponse, err := r.sendRequest(ctx, prompt)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to send request to gemini", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to send request to gemini: %w", err)
	}

	if err := r.parseResponse(geminiAPIResponse, &result); err != nil {
		r.logger.ErrorContext(ctx, "failed to parse response from gemini", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to parse response from gemini: %w", err)
	}

	// Set default
	result.MarketPrice = marketPrice
	result.StockCode = stockCode
	result.Exchange = exchange
	result.Timestamp = utils.TimeNowWIB()

	jsonResult, err := json.Marshal(result)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to marshal result", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	stockAnalysisAI := model.StockAnalysisAI{
		StockCode:      stockCode,
		Exchange:       exchange,
		Prompt:         prompt,
		HashIdentifier: hash,
		Response:       jsonResult,
		MarketPrice:    marketPrice,
		Recommendation: result.Signal,
		Score:          result.TechnicalScore,
		Confidence:     result.Confidence,
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&stockAnalysisAI).Error; err != nil {
			return fmt.Errorf("failed to create stock analysis AI: %w", err)
		}

		if err := tx.Model(&model.StockAnalysis{}).Where("id IN (?)", stockAnalysisIds).Update("stock_analysis_ai_id", stockAnalysisAI.ID).Error; err != nil {
			return fmt.Errorf("failed to update stock analysis: %w", err)
		}

		return nil
	})

	if err != nil {
		r.logger.ErrorContext(ctx, "failed set stock analysis ai", logger.ErrorField(err))
		return nil, err
	}

	return &result, nil
}

func (r *geminiAIRepository) sendRequest(ctx context.Context, prompt string) (*dto.GeminiAPIResponse, error) {
	contents := []*genai.Content{
		genai.NewContentFromText(prompt, "user"),
	}
	geminiTokenResp, err := r.genAiClient.Models.CountTokens(ctx, r.cfg.Gemini.BaseModel, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count tokens: %w", err)
	}

	r.logger.Debug("Gemini token count",
		logger.IntField("total_tokens", int(geminiTokenResp.TotalTokens)),
		logger.IntField("remaining", r.tokenLimiter.GetRemaining()),
	)
	if err := r.tokenLimiter.Wait(ctx, int(geminiTokenResp.TotalTokens)); err != nil {
		return nil, fmt.Errorf("failed to wait for token gemini limit: %w", err)
	}

	if err := r.requestLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("failed to wait for request gemini limit: %w", err)
	}

	if int(geminiTokenResp.TotalTokens) > r.cfg.Gemini.MaxTokenPerMinute/2 {
		r.logger.Warn("Token has exceeded 50% of the limit", logger.IntField("remaining", r.tokenLimiter.GetRemaining()))
	}

	payload := dto.GeminiAPIRequest{
		Contents: []dto.Content{{Parts: []dto.Part{{Text: prompt}}}},
	}

	geminiAPIResponse := dto.GeminiAPIResponse{}

	apiURL := fmt.Sprintf("/%s:generateContent?key=%s", r.cfg.Gemini.BaseModel, r.cfg.Gemini.APIKey)

	geminiResp, err := r.httpClient.Post(ctx, apiURL, payload, nil, &geminiAPIResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to gemini: %w", err)
	}

	if geminiResp.StatusCode != http.StatusOK {
		r.logger.ErrorContext(ctx, "failed to get data from gemini", logger.IntField("status_code", geminiResp.StatusCode))
		return nil, fmt.Errorf("failed to get data: %v", geminiResp.Body)
	}

	return &geminiAPIResponse, nil
}

func (r *geminiAIRepository) parseResponse(response *dto.GeminiAPIResponse, dest interface{}) error {
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("invalid response from Gemini API: no content found")
	}

	jsonString := response.Candidates[0].Content.Parts[0].Text
	jsonString = strings.Trim(jsonString, "`json\n`")

	return json.Unmarshal([]byte(jsonString), dest)
}
