package dto

import "time"

// BacktestRequest mendefinisikan parameter untuk menjalankan sebuah backtest.
type BacktestRequest struct {
	StockCode string    `json:"stock_code"`
	Exchange  string    `json:"exchange"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// TradeLog mencatat setiap transaksi yang terjadi selama backtest.
type TradeLog struct {
	Symbol      string    `json:"symbol"`
	EntryDate   time.Time `json:"entry_date"`
	EntryPrice  float64   `json:"entry_price"`
	ExitDate    time.Time `json:"exit_date"`
	ExitPrice   float64   `json:"exit_price"`
	ExitReason  string    `json:"exit_reason"`
	ProfitLoss  float64   `json:"profit_loss"`
	HoldingPeriod int     `json:"holding_period"`
}

// BacktestResult merangkum hasil dari sebuah sesi backtest.
type BacktestResult struct {
	StockCode         string     `json:"stock_code"`
	StartDate         time.Time  `json:"start_date"`
	EndDate           time.Time  `json:"end_date"`
	TotalTrades       int        `json:"total_trades"`
	WinningTrades     int        `json:"winning_trades"`
	LosingTrades      int        `json:"losing_trades"`
	WinRate           float64    `json:"win_rate"`
	TotalProfitLoss   float64    `json:"total_profit_loss"`
	TotalProfit       float64    `json:"total_profit"`
	TotalLoss         float64    `json:"total_loss"`
	ProfitFactor      float64    `json:"profit_factor"` // Total Profit / Total Loss
	MaxDrawdown       float64    `json:"max_drawdown"`
	AvgHoldingPeriod  float64    `json:"avg_holding_period"`
	Trades            []TradeLog `json:"trades"`
}
