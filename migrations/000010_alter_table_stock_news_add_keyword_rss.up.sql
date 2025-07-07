ALTER TABLE stock_news
ADD COLUMN IF NOT EXISTS keyword_rss varchar(255);

ALTER TABLE stock_news
ADD COLUMN IF NOT EXISTS source_name varchar(255);