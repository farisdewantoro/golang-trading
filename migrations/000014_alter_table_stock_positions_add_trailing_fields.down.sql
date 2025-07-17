ALTER TABLE stock_positions
DROP COLUMN IF EXISTS trailing_profit_price,
DROP COLUMN IF EXISTS trailing_stop_price,
DROP COLUMN IF EXISTS highest_price_since_ttp;
