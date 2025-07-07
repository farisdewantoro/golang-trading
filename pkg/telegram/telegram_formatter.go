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
	TakeProfit AlertType = "TAKE_PROFIT"
	StopLoss   AlertType = "STOP_LOSS"
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
