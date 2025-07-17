ALTER TABLE stock_positions
ADD COLUMN trailing_profit_price FLOAT,
ADD COLUMN trailing_stop_price FLOAT,
ADD COLUMN highest_price_since_ttp FLOAT;

