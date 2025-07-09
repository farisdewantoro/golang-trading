CREATE TABLE stock_analyses (
  id SERIAL PRIMARY KEY,
  stock_code VARCHAR(20) NOT NULL,
  exchange VARCHAR(20) NOT NULL,
  timeframe VARCHAR(10) NOT NULL,
  timestamp TIMESTAMP NOT NULL,
  market_price float NOT NULL DEFAULT 0,
  ohlcv JSONB NOT NULL,
  technical_data JSONB,
  recommendation VARCHAR(20),
  hash_identifier VARCHAR(255) NOT NULL,
  created_at TIMESTAMP DEFAULT now(),
  updated_at TIMESTAMP DEFAULT now()
);


CREATE INDEX idx_stock_analyses_stock_code ON stock_analyses(stock_code);
CREATE INDEX idx_stock_analyses_stock_code_timeframe ON stock_analyses(stock_code, timeframe);
CREATE INDEX idx_stock_analyses_stock_code_timeframe_timestamp ON stock_analyses(stock_code, timeframe, timestamp);
CREATE INDEX idx_stock_analyses_hash_identifier ON stock_analyses(hash_identifier);
