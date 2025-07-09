ALTER TABLE stock_positions
ADD COLUMN exchange VARCHAR(255) NOT NULL;

CREATE INDEX idx_stock_positions_stock_code_exchange ON stock_positions(stock_code, exchange);
    