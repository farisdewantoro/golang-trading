CREATE TABLE stock_position_monitorings (
    id SERIAL PRIMARY KEY,
    stock_position_id BIGINT REFERENCES stock_positions(id) ON DELETE CASCADE,
    stock_analysis_ai_id BIGINT REFERENCES stock_analyses_ai(id) ON DELETE SET NULL,
    evaluation_summary JSONB,
    market_price float NOT NULL DEFAULT 0,
    timestamp TIMESTAMP NOT NULL,
    hash_identifier VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL
);

CREATE TABLE stock_position_monitoring_analysis_refs (
  id SERIAL PRIMARY KEY,
  stock_position_monitoring_id INT NOT NULL REFERENCES stock_position_monitorings(id) ON DELETE CASCADE,
  stock_analysis_id INT NOT NULL REFERENCES stock_analyses(id) ON DELETE CASCADE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP DEFAULT NULL
);

