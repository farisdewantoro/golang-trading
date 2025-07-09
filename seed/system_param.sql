INSERT INTO system_parameters (
    name,
    value,
    description,
    created_at,
    updated_at
)
VALUES (
    'DEFAULT_ANALYSIS_TIMEFRAMES',
    '[
        {"interval": "1d", "range": "3m"},
        {"interval": "4h", "range": "1m"},
        {"interval": "1h", "range": "14d"}
    ]'::jsonb,
    'Default data timeframe untuk analisis teknikal (interval dan range)',
    NOW(),
    NOW()
);
