-- Berita saham yang sudah pernah dikirim / dilihat, dipakai untuk dedup.
CREATE TABLE stock_news (
    id SERIAL PRIMARY KEY,
    stock_code VARCHAR(20) NOT NULL,
    exchange VARCHAR(60) NOT NULL,
    title TEXT NOT NULL,
    link TEXT NOT NULL,
    source VARCHAR(255),
    published_at TIMESTAMP WITH TIME ZONE,
    hash_identifier VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_stock_news_stock_code_exchange ON stock_news(stock_code, exchange, published_at DESC);

-- Daftar saham yang beritanya dipantau oleh user.
CREATE TABLE user_news_watchlists (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stock_code VARCHAR(20) NOT NULL,
    exchange VARCHAR(60) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, stock_code, exchange)
);

CREATE INDEX idx_user_news_watchlists_active ON user_news_watchlists(exchange, is_active);

-- Job news watcher + jadwal (tiap 15 menit, jam 07-17 WIB, Senin-Jumat).
INSERT INTO public.jobs ("name", description, "type", payload, retry_policy, timeout)
VALUES (
    '📰 Stock News Watcher',
    'Memantau berita terbaru untuk saham di watchlist berita dan posisi aktif pengguna, lalu mengirim notifikasi Telegram untuk berita baru.',
    'stock_news_watcher',
    '{"exchange":"IDX","max_age":"24h","max_items_per_stock":5,"include_positions":true,"query_template":"\"%s\" saham","language":"id","country":"ID"}'::jsonb,
    '{"max_retries": 0, "backoff_strategy": "string", "initial_interval": "string"}'::jsonb,
    900
);

INSERT INTO public.task_schedules (job_id, cron_expression, is_active)
SELECT id, '*/15 7-17 * * 1-5', true FROM public.jobs WHERE "type" = 'stock_news_watcher';
