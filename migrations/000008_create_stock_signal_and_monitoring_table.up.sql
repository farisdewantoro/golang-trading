CREATE TABLE stock_signals (
    id SERIAL PRIMARY KEY,
    stock_code VARCHAR(50) NOT NULL,
    signal VARCHAR(255) NOT NULL,
    confidence_score FLOAT NOT NULL DEFAULT 0,
    technical_score FLOAT NOT NULL DEFAULT 0,
    news_score FLOAT NOT NULL DEFAULT 0,
    data JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL
);

CREATE TABLE stock_position_monitorings (
    id SERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    stock_position_id BIGINT REFERENCES stock_positions(id) ON DELETE CASCADE,
    signal VARCHAR(255) NOT NULL,
    confidence_score FLOAT NOT NULL DEFAULT 0,
    technical_score FLOAT NOT NULL DEFAULT 0,
    news_score FLOAT NOT NULL DEFAULT 0,
    triggered_alert BOOLEAN DEFAULT false,
    data JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL
);