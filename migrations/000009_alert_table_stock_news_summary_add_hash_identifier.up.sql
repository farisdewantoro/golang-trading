ALTER TABLE stock_news_summary
ADD COLUMN IF NOT EXISTS hash_identifier TEXT UNIQUE NOT NULL;
