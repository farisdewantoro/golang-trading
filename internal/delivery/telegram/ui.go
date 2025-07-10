package telegram

import "gopkg.in/telebot.v3"

var (
	btnAskAIAnalyzer           telebot.Btn = telebot.Btn{Text: "🤖 Analisa oleh AI", Unique: "btn_ask_ai_analyzer", Data: "%s"}
	btnGeneralLoadingAnalisis  telebot.Btn = telebot.Btn{Text: "⏳ Menganalisis...", Unique: "btn_general_loading_analisis"}
	btnGeneralFinishAnalisis   telebot.Btn = telebot.Btn{Text: "✅ Analisis selesai", Unique: "btn_general_finish_analisis"}
	btnSetPositionAlertPrice   telebot.Btn = telebot.Btn{Unique: "btn_set_position_alert_price"}
	btnSetPositionAlertMonitor telebot.Btn = telebot.Btn{Unique: "btn_set_position_alert_monitor"}
	btnDeleteMessage           telebot.Btn = telebot.Btn{Text: "🗑️ Hapus Pesan", Unique: "btn_delete_message"}

	btnDeleteStockPosition   telebot.Btn = telebot.Btn{Text: "🗑️ Hapus Posisi", Unique: "btn_delete_stock_position"}
	btnToDetailStockPosition telebot.Btn = telebot.Btn{Unique: "btn_detail_stock_position"}
)

const (
	commonErrorInternal            = "Terjadi kesalahan internal, silakan coba lagi"
	commonErrorInternalSetPosition = commonErrorInternal + " dengan /setposition."
	commonErrorInternalMyPosition  = commonErrorInternal + " dengan /myposition."
)
