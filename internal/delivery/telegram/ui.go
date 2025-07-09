package telegram

import "gopkg.in/telebot.v3"

var (
	btnAskAIAnalyzer          telebot.Btn = telebot.Btn{Text: "🤖 Analisa oleh AI", Unique: "btn_ask_ai_analyzer", Data: "%s"}
	btnGeneralLoadingAnalisis telebot.Btn = telebot.Btn{Text: "⏳ Menganalisis...", Unique: "btn_general_loading_analisis"}
	btnGeneralFinishAnalisis  telebot.Btn = telebot.Btn{Text: "✅ Analisis selesai", Unique: "btn_general_finish_analisis"}
)
