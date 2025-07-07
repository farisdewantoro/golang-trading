CREATE TABLE stock_news (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    link TEXT  NOT NULL,
    google_rss_link TEXT,
    published_at TIMESTAMP WITH TIME ZONE,
    raw_content TEXT,
    summary TEXT,
    key_issue TEXT[],
    hash_identifier TEXT UNIQUE NOT NULL,
    impact_score FLOAT,
    source VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
); 

CREATE INDEX idx_stock_news_hash_identifier ON stock_news(hash_identifier);


CREATE TABLE stock_mentions (
    id SERIAL PRIMARY KEY,
    stock_news_id INTEGER REFERENCES stock_news(id) ON DELETE CASCADE,
    stock_code VARCHAR(50) NOT NULL,        
    sentiment VARCHAR(50),              -- positive | negative | neutral
    impact VARCHAR(50),                 -- bullish | bearish | sideways
    confidence_score  FLOAT,                   -- 0.0 - 1.0
    created_at TIMESTAMP DEFAULT NOW()
);