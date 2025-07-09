package dto

import "time"

type AIAnalyzeStockParam struct {
	Timeframe    string      `json:"timeframe"`
	TechAnalysis interface{} `json:"tech_analysis"`
	OHCLV        interface{} `json:"ohclv"`
	MarketPrice  float64     `json:"market_price"`
}

type GeminiAPIRequest struct {
	Contents []Content `json:"contents"`
}

type GeminiAPIResponse struct {
	Candidates []Candidate `json:"candidates"`
}

// Candidate is a candidate response from the Gemini API.
type Candidate struct {
	Content Content `json:"content"`
}

type Content struct {
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type AIAnalyzeStockResponse struct {
	Signal            string            `json:"signal"`
	StockCode         string            `json:"stock_code"`
	Exchange          string            `json:"exchange"`
	TargetPrice       float64           `json:"target_price"`
	StopLoss          float64           `json:"stop_loss"`
	TechnicalScore    float64           `json:"technical_score"`
	Confidence        float64           `json:"confidence"`
	KeyInsights       map[string]string `json:"key_insights"`
	EstimatedTimeToTP string            `json:"estimated_time_to_tp"`
	Reason            string            `json:"reason"`
	MarketPrice       float64           `json:"market_price"`
	Timestamp         time.Time         `json:"timestamp"`
}
