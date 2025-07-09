package repository

import (
	"encoding/json"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/pkg/logger"
	"strings"
)

func (r *geminiAIRepository) promptAnalyzeStock(
	stockCode string,
	exchange string,
	params []dto.AIAnalyzeStockParam,
) (string, error) {
	var sb strings.Builder

	// Gabungkan timeframe
	timeframes := []string{}
	for _, p := range params {
		timeframes = append(timeframes, p.Timeframe)
	}
	timeframeStr := strings.Join(timeframes, ", ")

	// Intro Prompt
	sb.WriteString(fmt.Sprintf(
		"Kamu adalah sistem AI analis teknikal profesional yang bertugas memberikan sinyal swing trading untuk saham %s di exchange %s berdasarkan data teknikal dan OHCLV dari beberapa timeframe (%s).\n\n",
		stockCode, exchange, timeframeStr,
	))

	sb.WriteString(`### Tugas Utama:
1. Berikan sinyal swing trading: hanya "BUY" atau "HOLD" (tidak boleh SELL).
2. Jika sinyal adalah **BUY**, tentukan Target Price (TP) dan Stop Loss (SL) berdasarkan risk:reward ratio minimal **1:2**. Jika sinyal adalah HOLD, tidak perlu menghitung TP dan SL.
3. Hitung skor teknikal antara 0-100, semakin tinggi berarti sinyal BUY lebih kuat.
4. Tentukan tingkat keyakinan (confidence level) dari AI dalam bentuk persentase (0-100).
5. Estimasikan waktu yang diperlukan untuk mencapai Target Price, misalnya: "2-5 hari".
6. Tambahkan key_insights dalam format map[string]string:
   - AI bebas menentukan key-nya (contoh: "trend", "volume", "candlestick", "pattern", "momentum", dsb).
   - Value berisi insight teknikal singkat, maksimal **100 karakter per item**.
   - Key boleh dalam bahasa inggris(lowercase) namun untuk Value WAJIB dalam bahasa Indonesia.
   - Maksimal 10 item dalam key_insights.
7. Berikan **alasan utama** kenapa sinyal BUY atau HOLD diberikan, berdasarkan indikator dominan dan timeframe utama.
`)

	sb.WriteString("\n### Penting: Arti Nilai Indikator `Recommend` dari TradingView")
	sb.WriteString(`
  2 = STRONG_BUY
  1 = BUY
  0 = NEUTRAL
 -1 = SELL
 -2 = STRONG_SELL
`)

	sb.WriteString(`
### Aturan untuk Menghindari False BUY (Minimalkan Risiko):
- ❌ Jangan berikan sinyal BUY jika mayoritas timeframe menunjukkan SELL berdasarkan Recommend.Global.Summary, MA, atau Oscillators.
- ✅ Berikan sinyal BUY **hanya jika** terdapat bukti pembalikan atau tren naik yang cukup kuat, seperti:
  - MACD menunjukkan crossover bullish di 4H atau 1D
  - Harga memantul dari support signifikan (pivot atau historis)
  - Muncul candlestick reversal (Hammer, Engulfing, Pin Bar, dll)
  - RSI/Stoch dalam kondisi oversold dan mulai naik
  - Harga mulai menembus EMA20 atau EMA50 dari bawah
- Jika kondisi masih belum mendukung, tetap berikan sinyal HOLD dan jelaskan alasannya dengan objektif.
`)

	sb.WriteString(`
### Format Output JSON (WAJIB - tanpa tambahan teks lainnya):
{
  "signal": "BUY | HOLD",
  "target_price": 0,
  "stop_loss": 0,
  "technical_score": 0,
  "confidence": 0,
  "estimated_time_to_tp": "1-7 hari",
  "key_insights": {
     "key": "value",
	 "key2": "value2",
	 ....
  },
  "reason": "Alasan utama pengambilan keputusan dari indikator dominan dan timeframe utama"
}
`)

	// Tambahkan input data ke prompt
	inputDataJson, err := json.Marshal(params)
	if err != nil {
		r.logger.Error("failed to marshal params when analyze stock", logger.ErrorField(err))
		return "", err
	}

	sb.WriteString("\n\n### Input Data (Teknikal + OHCLV Multi-Timeframe):\n")
	sb.WriteString(string(inputDataJson))

	return sb.String(), nil
}
