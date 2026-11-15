package telegram

import (
	"fmt"
	"strings"
	"time"

	"golang-trading/pkg/utils"
)

// AlertType represents the type of alert
type AlertType string

const (
	TakeProfit     AlertType = "TAKE_PROFIT"
	StopLoss       AlertType = "STOP_LOSS"
	TrailingProfit AlertType = "TRAILING_PROFIT"
	TrailingStop   AlertType = "TRAILING_STOP"
)

// FormatStockAlertResultForTelegram formats the stock alert result into a Markdown string for Telegram.
func FormatStockAlertResultForTelegram(alertType AlertType, stockCode string, triggerPrice float64, targetPrice float64, timestamp int64) string {
	var builder strings.Builder

	var title, emoji string
	switch alertType {
	case TakeProfit:
		title = "Take Profit Triggered!"
		emoji = "🎯"
	case StopLoss:
		title = "Stop Loss Triggered!"
		emoji = "⚠️"
	case TrailingProfit:
		title = "Trailing Profit Triggered!"
		emoji = "📈"
	case TrailingStop:
		title = "Trailing Stop Triggered!"
		emoji = "📉"
	default:
		title = "Price Alert"
		emoji = "🔔"
	}

	builder.WriteString(fmt.Sprintf("%s [%s] %s\n", emoji, stockCode, title))
	builder.WriteString(fmt.Sprintf("💰Harga menyentuh: %d (target: %d)\n", int(triggerPrice), int(targetPrice)))
	builder.WriteString(fmt.Sprintf("%s\n", utils.PrettyDate(time.Unix(timestamp, 0))))
	return builder.String()
}

func FormatErrorAlertMessage(time time.Time, errType string, errMsg string, data string) string {
	return fmt.Sprintf(`📛 [ERROR ALERT] 
%s
🔧 %s
⚠️ %s	

📄 Data: %s
`, utils.PrettyDate(time), errType, errMsg, data)
}

// NewsLine is a single headline shown in a news message.
type NewsLine struct {
	Title       string
	Link        string
	Source      string
	PublishedAt time.Time
}

// FormatStockNewsForTelegram formats news headlines for a stock as Telegram HTML.
func FormatStockNewsForTelegram(header string, stockCode string, news []NewsLine) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>%s %s</b>\n\n", header, utils.EscapeHTMLForTelegram(stockCode)))

	for i, n := range news {
		sb.WriteString(fmt.Sprintf("%d. <a href=\"%s\">%s</a>\n",
			i+1,
			strings.ReplaceAll(utils.EscapeHTMLForTelegram(n.Link), "\"", "&quot;"),
			utils.EscapeHTMLForTelegram(n.Title)))

		meta := []string{}
		if n.Source != "" {
			meta = append(meta, utils.EscapeHTMLForTelegram(n.Source))
		}
		if !n.PublishedAt.IsZero() {
			meta = append(meta, utils.PrettyDate(utils.TimeToWIB(n.PublishedAt)))
		}
		if len(meta) > 0 {
			sb.WriteString(fmt.Sprintf("   <i>%s</i>\n", strings.Join(meta, " • ")))
		}
	}

	sb.WriteString("\n<i>📌 Berita hanya referensi, tetap Do Your Own Research.</i>")
	return sb.String()
}
