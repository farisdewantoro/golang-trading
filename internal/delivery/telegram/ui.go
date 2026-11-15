package telegram

import "gopkg.in/telebot.v3"

var (
	btnAskAIAnalyzer           telebot.Btn = telebot.Btn{Text: "🤖 Analisa oleh AI", Unique: "btn_ask_ai_analyzer", Data: "%s"}
	btnGeneralLoadingAnalisis  telebot.Btn = telebot.Btn{Text: "⏳ Menganalisis...", Unique: "btn_general_loading_analisis"}
	btnGeneralFinishAnalisis   telebot.Btn = telebot.Btn{Text: "✅ Analisis selesai", Unique: "btn_general_finish_analisis"}
	btnSetPositionAlertPrice   telebot.Btn = telebot.Btn{Unique: "btn_set_position_alert_price"}
	btnSetPositionAlertMonitor telebot.Btn = telebot.Btn{Unique: "btn_set_position_alert_monitor"}
	btnSetPositionTechnical    telebot.Btn = telebot.Btn{Text: "📥 Simpan posisi ini", Unique: "btn_set_position_by_technical"}
	btnSetPositionAI           telebot.Btn = telebot.Btn{Text: "🤖 Simpan posisi ini", Unique: "btn_set_position_by_ai"}

	btnDeleteMessage   telebot.Btn = telebot.Btn{Text: "🗑️ Hapus Pesan", Unique: "btn_delete_message"}
	btnRefreshAnalysis telebot.Btn = telebot.Btn{Text: "🔄 Refresh Analisis", Unique: "btn_refresh_analysis"}

	btnDeleteStockPosition        telebot.Btn = telebot.Btn{Text: "🗑️ Hapus Posisi", Unique: "btn_delete_stock_position"}
	btnConfirmDeleteStockPosition telebot.Btn = telebot.Btn{Unique: "btn_confirm_delete_stock_position"}
	btnToDetailStockPosition      telebot.Btn = telebot.Btn{Unique: "btn_detail_stock_position"}

	btnSaveExitPosition        telebot.Btn = telebot.Btn{Text: "💾 Simpan", Unique: "btn_save_exit_position"}
	btnCancelGeneral           telebot.Btn = telebot.Btn{Text: "❌ Batal", Unique: "btn_cancel_general"}
	btnExitStockPosition       telebot.Btn = telebot.Btn{Unique: "btn_exit_stock_position"}
	btnBackStockPosition       telebot.Btn = telebot.Btn{Text: "🔙 Kembali", Unique: "btn_back_stock_position"}
	btnRefreshAnalysisPosition telebot.Btn = telebot.Btn{Text: "🔄 Refresh Analisis", Unique: "btn_refresh_analysis_position"}

	//buylist
	btnCancelBuyListAnalysis telebot.Btn = telebot.Btn{Text: "⛔ Hentikan Analisis", Unique: "btn_cancel_buy_list_analysis"}
	btnShowBuyListAnalysis   telebot.Btn = telebot.Btn{Unique: "btn_show_buy_list_analysis"}

	btnGeneralAnalisis telebot.Btn = telebot.Btn{Unique: "btn_general_analisis"}

	//schedule
	btnDetailJob           telebot.Btn = telebot.Btn{Unique: "btn_detail_job"}
	btnActionBackToJobList telebot.Btn = telebot.Btn{Text: "🔙 Kembali", Unique: "btn_action_back_to_job_list"}
	btnActionRunJob        telebot.Btn = telebot.Btn{Text: "🚀 Jalankan", Unique: "btn_action_run_job"}

	//alert signal
	btnAlertSignal telebot.Btn = telebot.Btn{Unique: "btn_alert_signal"}

	// news
	btnWatchNews   telebot.Btn = telebot.Btn{Unique: "btn_watch_news"}
	btnUnwatchNews telebot.Btn = telebot.Btn{Unique: "btn_unwatch_news"}
	btnShowNews    telebot.Btn = telebot.Btn{Unique: "btn_show_news"}
)

const (
	commonErrorInternal            = "Terjadi kesalahan internal, silakan coba lagi"
	commonErrorInternalSetPosition = commonErrorInternal + " dengan /setposition."
	commonErrorInternalMyPosition  = commonErrorInternal + " dengan /myposition."
	commonErrorInternalReport      = commonErrorInternal + " dengan /report."
)

const (
	messageLoadingAnalysis string = "🔍 Menganalisis: $%s"
)
