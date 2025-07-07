package dto

type StockOHLCV struct {
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    int64   `json:"volume"`
	Timestamp int64   `json:"timestamp"`
}

type StockData struct {
	MarketPrice float64      `json:"market_price"`
	Range       string       `json:"range"`
	Interval    string       `json:"interval"`
	OHLCV       []StockOHLCV `json:"ohlc"`
}

type StockDataMultiTimeframe struct {
	MarketPrice float64      `json:"market_price"`
	OHLCV1D     []StockOHLCV `json:"ohlc_1d"`
	OHLCV4H     []StockOHLCV `json:"ohlc_4h"`
	OHLCV1H     []StockOHLCV `json:"ohlc_1h"`
}

type GetStockDataParam struct {
	StockCode string `json:"stock_code"`
	Range     string `json:"range"`
	Interval  string `json:"interval"`
}

// Yahoo Finance API Response
type YahooFinanceResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol             string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"chart"`
}

type GetStockPositionsParam struct {
	IDs             []uint   `json:"ids"`
	StockCodes      []string `json:"stock_codes"`
	PriceAlert      *bool    `json:"price_alert"`
	MonitorPosition *bool    `json:"monitor_position"`
	IsActive        *bool    `json:"is_active"`
}

type GetStockSummaryParam struct {
	HashIdentifier string `json:"hash_identifier"`
}
