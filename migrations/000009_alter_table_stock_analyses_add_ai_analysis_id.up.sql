ALTER TABLE stock_analyses
ADD COLUMN stock_analysis_ai_id INTEGER REFERENCES stock_analyses_ai(id) ON DELETE SET NULL;
