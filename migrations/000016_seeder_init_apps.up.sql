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


INSERT INTO public.jobs
(id, "name", description, "type", payload, retry_policy, timeout, created_at, updated_at)
VALUES(7, '📈 Stock Position Monitoring', 'Menganalisis posisi saham pengguna secara berkala dan memberikan notifikasi jika diperlukan tindakan, seperti take profit, cut loss, atau menyesuaikan stop loss.', 'stock_position_monitor', '{"range": "14d", "interval": "1h", "send_notif": true}'::jsonb, '{"max_retries": 0, "backoff_strategy": "string", "initial_interval": "string"}'::jsonb, 1800, '2025-06-24 13:16:48.057', '2025-06-24 13:16:48.057');
INSERT INTO public.jobs
(id, "name", description, "type", payload, retry_policy, timeout, created_at, updated_at)
VALUES(4, '🚨 Stock Price Alert', 'Mengirim peringatan ke Telegram saat harga saham mencapai batas yang telah ditentukan (TP/SL).
', 'stock_price_alert', '{"data_range": "1d", "data_interval": "1m", "alert_cache_duration": "30m", "alert_trigger_window_duration": "5m", "alert_resend_threshold_percent": 2.0}'::jsonb, '{"max_retries": 0, "backoff_strategy": "string", "initial_interval": "string"}'::jsonb, 360, '2025-06-17 20:44:35.326', '2025-06-17 20:44:35.326');
INSERT INTO public.jobs
(id, "name", description, "type", payload, retry_policy, timeout, created_at, updated_at)
VALUES(5, '📊 Stock Analyze', 'Menganalisis saham-saham dalam whitelist sistem untuk menghasilkan sinyal beli atau jual secara otomatis. Hasil analisis ini juga digunakan dalam fitur /buylist.', 'stock_analyzer', '{"additional_stocks": [], "trading_view_buy_list_params": [{"sort": {"sortBy": "Recommend.All", "sortOrder": "desc"}, "range": [0, 50], "filter": [{"left": "market_cap_basic", "right": 10000000000000, "operation": "egreater"}, {"left": "average_volume_10d_calc", "right": 10000000, "operation": "greater"}, {"left": "is_primary", "right": true, "operation": "equal"}], "columns": ["Recommend.All"], "markets": ["indonesia"], "options": {"lang": "en"}, "symbols": {}, "ignore_unknown_fields": false}]}'::jsonb, '{"max_retries": 0, "backoff_strategy": "string", "initial_interval": "string"}'::jsonb, 1800, '2025-06-23 16:57:18.359', '2025-06-23 16:57:18.359');


INSERT INTO public.task_schedules
(id, job_id, cron_expression, next_execution, last_execution, is_active, created_at, updated_at)
VALUES(7, 4, '*/3 9-16 * * 1-5', '2025-07-07 09:00:00.000', '2025-07-04 16:57:00.695', true, '2025-06-17 20:44:35.328', '2025-07-04 16:57:00.701');
INSERT INTO public.task_schedules
(id, job_id, cron_expression, next_execution, last_execution, is_active, created_at, updated_at)
VALUES(8, 5, '10 8,12,15 * * 1-5', '2025-07-07 08:10:00.000', '2025-07-04 15:10:00.692', true, '2025-06-23 16:57:18.362', '2025-07-04 15:10:00.696');
INSERT INTO public.task_schedules
(id, job_id, cron_expression, next_execution, last_execution, is_active, created_at, updated_at)
VALUES(10, 7, '0 8-16 * * 1-5', '2025-07-07 08:00:00.000', '2025-07-04 16:00:00.693', true, '2025-06-24 13:16:48.060', '2025-07-04 16:00:00.699');
