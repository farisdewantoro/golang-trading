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
 {
    "range": "3m",
    "interval": "1d",
    "weight":3
  },
  {
    "range": "1m",
    "interval": "4h",
     "weight":2
  },
  {
    "range": "14d",
    "interval": "1h",
     "weight":1
  }
    ]'::jsonb,
    'Default data timeframe untuk analisis teknikal (interval dan range)',
    NOW(),
    NOW()
);
