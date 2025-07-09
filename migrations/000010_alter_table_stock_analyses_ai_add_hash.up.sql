ALTER TABLE stock_analyses_ai
ADD COLUMN hash_identifier VARCHAR(255) NOT NULL;

CREATE INDEX idx_stock_analyses_ai_hash_identifier ON stock_analyses_ai(hash_identifier);
