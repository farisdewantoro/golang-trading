CREATE TABLE stock_analyses_ai (
  id SERIAL PRIMARY KEY,
  stock_code VARCHAR(20) NOT NULL,
  exchange VARCHAR(20) NOT NULL,

  prompt TEXT NOT NULL,
  response JSONB NOT NULL,
  recommendation VARCHAR(20),
  score float NOT NULL,
  confidence float NOT NULL,
  market_price float NOT NULL DEFAULT 0,

  deleted_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP NOT NULL DEFAULT now()
);




create index idx_stock_analyses_ai_stock_code on stock_analyses_ai(stock_code);
create index idx_stock_analyses_ai_stock_code_exchange on stock_analyses_ai(stock_code, exchange);