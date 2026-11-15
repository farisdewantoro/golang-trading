package telegram

import (
	"context"

	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) handleStart(ctx context.Context, c telebot.Context) error {
	message := `👋 *Halo, selamat datang di Bot Swing Trading!* 🤖  
Saya di sini untuk membantu kamu memantau saham dan mencari peluang terbaik dari pergerakan harga.

🔧 Berikut beberapa perintah yang bisa kamu gunakan:

📈 /analyze - Analisa saham pilihanmu berdasarkan strategi  
📋 /buylist - Lihat daftar saham potensial untuk dibeli  
📝 /setposition - Catat posisi saham yang sedang kamu pegang  
📊 /myposition - Lihat semua posisi yang sedang dipantau  
💰 /report Melihat ringkasan hasil trading kamu berdasarkan posisi yang sudah kamu entry dan exit.
🔄 /scheduler	- Lihat status scheduler & jalankan job secara manual  
📡 /alertsignal - Notifikasi sinyal BUY terbaik yang dikirim otomatis sesuai jadwal oleh sistem
📰 /news - Lihat & pantau berita saham (notifikasi otomatis saat ada berita baru)

💡 Info & Bantuan:
🆘 /help - Lihat panduan penggunaan lengkap  
🔁 /start - Tampilkan pesan ini lagi  
❌ /cancel - Batalkan perintah yang sedang berjalan

🚀 *Siap mulai?* Coba ketik /analyze untuk memulai analisa pertamamu!`
	return c.Send(message, &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})
}

func (t *TelegramBotHandler) handleHelp(ctx context.Context, c telebot.Context) error {
	message := `❓ *Panduan Penggunaan Bot Swing Trading* ❓

Bot ini membantu kamu memantau saham dan mencari peluang terbaik dengan analisa teknikal yang disesuaikan untuk swing trading.

Berikut daftar perintah yang bisa kamu gunakan:

🤖 *Perintah Utama:*
/start - Menampilkan pesan sambutan  
/help - Menampilkan panduan ini  
/analyze - Mulai analisa interaktif untuk saham tertentu  
/buylist - Lihat saham potensial yang sedang menarik untuk dibeli  
/setposition - Catat saham yang kamu beli agar bisa dipantau otomatis  
/myposition - Lihat semua posisi yang sedang kamu pantau  
/cancel - Batalkan perintah yang sedang berjalan
/report - Melihat ringkasan hasil trading kamu berdasarkan posisi yang sudah kamu entry dan exit.
/scheduler	- Lihat status scheduler & jalankan job secara manual  
/alertsignal - Notifikasi sinyal BUY terbaik yang dikirim otomatis sesuai jadwal oleh sistem
/news - Lihat berita terbaru (contoh: /news BBCA) & daftar watchlist berita
/watchnews - Pantau berita saham (contoh: /watchnews BBCA TLKM)
/unwatchnews - Berhenti pantau berita saham

💡 *Tips Penggunaan:*
1. Gunakan /analyze untuk analisa cepat atau mendalam (bisa juga langsung kirim kode saham, misalnya: 'BBCA')  
2. Jalankan /buylist setiap pagi untuk melihat peluang baru  
3. Setelah beli saham, gunakan /setposition agar bot bisa bantu awasi harga  
4. Pantau semua posisi aktif kamu lewat /myposition


📌 Gunakan sinyal ini sebagai referensi tambahan saja, ya.  
Keputusan tetap di tangan kamu — jangan lupa *Do Your Own Research!* 🔍`
	return c.Send(message, &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})
}
