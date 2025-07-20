ALTER TABLE stock_positions
ADD COLUMN IF NOT EXISTS trailing_profit_price FLOAT,
ADD COLUMN IF NOT EXISTS trailing_stop_price FLOAT,
ADD COLUMN IF NOT EXISTS highest_price_since_ttp FLOAT,
ADD COLUMN IF NOT EXISTS initial_score FLOAT,
ADD COLUMN IF NOT EXISTS final_score FLOAT;

