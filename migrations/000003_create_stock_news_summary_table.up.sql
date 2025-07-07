CREATE TABLE stock_news_summary (
    id SERIAL PRIMARY KEY,
    stock_code VARCHAR(50) NOT NULL,
    summary_sentiment VARCHAR(50),     -- e.g. positive, negative, neutral
    summary_impact VARCHAR(50),        -- e.g. bullish, bearish, sideways
    summary_confidence_score FLOAT,          -- e.g. 0.75
    key_issues TEXT[],
    suggested_action VARCHAR(10),      -- e.g. buy, hold, sell
    reasoning TEXT,
    short_summary TEXT,
    summary_start TIMESTAMP,           -- waktu mulai agregasi berita
    summary_end TIMESTAMP,             -- waktu akhir agregasi berita
    created_at TIMESTAMP DEFAULT NOW()
);
