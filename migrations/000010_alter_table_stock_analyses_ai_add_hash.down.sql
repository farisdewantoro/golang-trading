DROP INDEX IF EXISTS idx_stock_analyses_ai_hash_identifier;

ALTER TABLE stock_analyses_ai
DROP COLUMN IF EXISTS hash_identifier;

