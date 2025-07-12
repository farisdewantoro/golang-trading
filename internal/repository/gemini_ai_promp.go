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
2. Jika sinyal adalah **BUY**, tentukan Target Price (TP) dan Stop Loss (SL) berdasarkan analisis level teknikal yang valid dan realistis:
   - Gunakan level resistance/support signifikan, swing high/low, pivot point, Fibonacci extension (1.272 / 1.618), atau moving average penting (seperti EMA20, EMA50).
   - TP sebaiknya diambil dari resistance terdekat yang valid, area psikologis (angka bulat seperti 200, 500), atau level Fibonacci.
   - SL sebaiknya diletakkan di bawah swing low terakhir, support signifikan, MA penting, atau berdasarkan volatilitas (seperti ATR).
   - Evaluasi kekuatan level support/resistance berdasarkan jumlah candlestick historis yang menyentuh level tersebut ("touch count"):
     - Level yang telah diuji minimal 2-3 kali tanpa ditembus dianggap kuat dan lebih layak untuk dijadikan TP atau SL.
     - Hindari memilih level yang belum teruji atau baru disentuh sekali, kecuali ada konfirmasi tambahan seperti breakout volume besar.
   - Dalam strategi swing trading dengan waktu 2-10 hari, target price (TP) dan stop loss (SL) harus disesuaikan dengan volatilitas historis (seperti ATR).
     - Hindari menetapkan TP melebihi **6%** dan SL melebihi **4%** dari entry price, kecuali ada konfirmasi kuat seperti breakout volume besar.
     - Jika estimasi waktu ke TP sangat pendek (misalnya 2-5 hari), maka TP sebaiknya dibatasi maksimal **3-4%** saja.
   - Idealnya Risk:Reward (RR) **≥ 1:2**, namun tidak wajib. Jika tidak memenuhi, tetap prioritaskan level teknikal yang paling masuk akal.
   - Jelaskan alasan pemilihan TP dan SL secara profesional dalam field **exit_strategy_reason** (maksimal 2 kalimat).
   - Jangan membuat angka acak untuk TP atau SL — semua harus berdasarkan logika teknikal yang kuat dari timeframe utama (misal: 4H, 1D).
3. Hitung skor teknikal antara 0-100, semakin tinggi berarti sinyal BUY lebih kuat.
4. Tentukan tingkat keyakinan (confidence level) dari AI dalam bentuk persentase (0-100).
5. Estimasikan waktu yang diperlukan untuk mencapai Target Price dalam hari (integer).
6. Tambahkan key_insights dalam format map[string]string:
   - Maksimal hanya 3 item yang **paling penting dan paling berdampak** terhadap keputusan sinyal.
   - Key bebas ditentukan (contoh: "trend", "volume", "candlestick", "pattern", "momentum", dsb).
   - Value berisi insight teknikal singkat, maksimal **100 karakter per item**.
   - Key boleh dalam bahasa inggris (lowercase), namun untuk Value WAJIB dalam bahasa Indonesia.
7. Berikan **alasan utama** kenapa sinyal BUY atau HOLD diberikan, berdasarkan indikator dominan dan timeframe utama.
8. Sertakan juga kekuatan level support dan resistance dalam bentuk "level_strength", yaitu jumlah sentuhan sebelumnya untuk TP dan SL.

`)

	sb.WriteString(`
### Penting: Arti Nilai Indikator *Recommend* dari TradingView
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
  "estimated_time_to_tp_days": 0,
  "key_insights": {
     "key": "value",
     "key2": "value2",
     "key3": "value3"
  },
  "level_strength": {
     "tp_touch_count": 0,
     "sl_touch_count": 0
  },
  "exit_strategy_reason": "Penjelasan kenapa TP dan SL ditentukan di titik tersebut",
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
