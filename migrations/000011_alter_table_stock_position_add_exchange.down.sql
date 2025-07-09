DROP INDEX IF EXISTS idx_stock_positions_stock_code_exchange;

ALTER TABLE stock_positions
DROP COLUMN IF EXISTS exchange;

