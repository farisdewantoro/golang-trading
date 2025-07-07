package telegram

import (
	"context"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/pkg/logger"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) WithContext(handler func(ctx context.Context, c telebot.Context) error) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		ctx, cancel := context.WithTimeout(t.ctx, 5*time.Minute)
		defer cancel()

		return handler(ctx, c)
	}
}

func (t *TelegramBotHandler) RegisterHandlers() {
	t.echo.POST("/api/v1/telegram/webhook", func(c echo.Context) error {
		var update telebot.Update
		if err := c.Bind(&update); err != nil {
			t.log.ErrorContext(t.ctx, "Cannot bind JSON", logger.ErrorField(err))
			badRequest := dto.NewBadRequestResponse(err.Error())
			return c.JSON(http.StatusBadRequest, badRequest)
		}
		t.bot.ProcessUpdate(update)
		return c.JSON(http.StatusOK, dto.NewBaseResponse(http.StatusOK, "ok", nil))
	})

	t.bot.Handle("/start", t.WithContext(t.handleStart))
	t.bot.Handle("/help", t.WithContext(t.handleHelp))
	t.bot.Handle(telebot.OnText, t.WithContext(t.handleConversation))

}

func (t *TelegramBotHandler) handleStart(ctx context.Context, c telebot.Context) error {
	message := `👋 *Halo, selamat datang di Bot Swing Trading!* 🤖  
Saya di sini untuk membantu kamu memantau saham dan mencari peluang terbaik dari pergerakan harga.

🔧 Berikut beberapa perintah yang bisa kamu gunakan:

📈 /analyze - Analisa saham pilihanmu berdasarkan strategi  
📋 /buylist - Lihat daftar saham potensial untuk dibeli  
📝 /setposition - Catat posisi saham yang sedang kamu pegang  
📊 /myposition - Lihat semua posisi yang sedang dipantau  
📰 /news - Lihat berita terkini, alert berita penting saham, ringkasan berita
💰 /report Melihat ringkasan hasil trading kamu berdasarkan posisi yang sudah kamu entry dan exit.
🔄 /scheduler	- Lihat status scheduler & jalankan job secara manual  


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
/news - Lihat berita terkini, alert berita penting saham, ringkasan berita
/cancel - Batalkan perintah yang sedang berjalan
/report - Melihat ringkasan hasil trading kamu berdasarkan posisi yang sudah kamu entry dan exit.
/scheduler	- Lihat status scheduler & jalankan job secara manual  

💡 *Tips Penggunaan:*
1. Gunakan /analyze untuk analisa cepat atau mendalam (bisa juga langsung kirim kode saham, misalnya: 'BBCA')  
2. Jalankan /buylist setiap pagi untuk melihat peluang baru  
3. Setelah beli saham, gunakan /setposition agar bot bisa bantu awasi harga  
4. Pantau semua posisi aktif kamu lewat /myposition


📌 Gunakan sinyal ini sebagai referensi tambahan saja, ya.  
Keputusan tetap di tangan kamu — jangan lupa *Do Your Own Research!* 🔍`
	return c.Send(message, &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})
}

func (t *TelegramBotHandler) handleConversation(ctx context.Context, c telebot.Context) error {
	userID := c.Sender().ID
	fmt.Println("handleConversation", userID)
	return t.handleTextMessage(ctx, c)

}

func (t *TelegramBotHandler) handleTextMessage(ctx context.Context, c telebot.Context) error {
	// userID := c.Sender().ID

	// If user is in a conversation, handle it
	// if state, ok := t.userStates[userID]; ok && state != StateIdle {
	// 	t.handleConversation(ctx, c)
	// 	return nil
	// }

	// Cek apakah bukan command
	if !strings.HasPrefix(c.Text(), "/") {
		return c.Send("Saya tidak mengenali perintahmu. Gunakan /help untuk melihat daftar perintah.")
	}

	return nil
}
