CREATE TABLE stock_positions (
  id SERIAL PRIMARY KEY,
  user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
  stock_code VARCHAR(50) NOT NULL,
  buy_price FLOAT NOT NULL,
  take_profit_price FLOAT,     
  stop_loss_price FLOAT,        
  buy_date DATE NOT NULL,
  max_holding_period_days INT,
  is_active BOOLEAN DEFAULT true,
  exit_price FLOAT,
  exit_date DATE,
  price_alert BOOLEAN DEFAULT false,
  last_price_alert_at TIMESTAMP,
  monitor_position BOOLEAN DEFAULT true,
  last_monitor_position_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT now(),
  updated_at TIMESTAMP DEFAULT now()
);

CREATE INDEX idx_stock_positions_user_id ON stock_positions(user_id);
CREATE INDEX idx_stock_positions_stock_code ON stock_positions(stock_code);
CREATE INDEX idx_stock_positions_stock_code_user_id ON stock_positions(stock_code, user_id);
