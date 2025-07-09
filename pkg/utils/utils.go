package utils

import (
	"bytes"
	"context"
	"fmt"
	"golang-trading/pkg/logger"
	"html"
	"log"
	"math"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ContainsString checks if a slice of strings contains a specific string.
func ContainsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func CleanToValidUTF8(s string) string {
	var buf bytes.Buffer
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			// Skip byte yang rusak
			i++
			continue
		}
		buf.WriteRune(r)
		i += size
	}
	return buf.String()
}

func SafeText(text string) string {
	return CleanToValidUTF8(html.UnescapeString(text))
}

type goSafeChain struct {
	fn      func()
	onPanic func(interface{})
}

// GoSafe initializes the chain with the function to run.
func GoSafe(fn func()) *goSafeChain {
	return &goSafeChain{fn: fn}
}

// OnPanic sets a custom panic handler.
func (g *goSafeChain) OnPanic(handler func(interface{})) *goSafeChain {
	g.onPanic = handler
	return g
}

// Run executes the function in a goroutine with panic recovery.
func (g *goSafeChain) Run() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if g.onPanic != nil {
					g.onPanic(r)
				} else {
					log.Printf("[Panic Recovered] %v", r)
				}
			}
		}()
		g.fn()
	}()
}

func ToPointer[T any](value T) *T {
	return &value
}

func MustParseDate(strTime string) time.Time {
	date, _ := time.Parse("2006-01-02", strTime)

	return date
}

func ShouldContinue(ctx context.Context, log *logger.Logger) bool {
	select {
	case <-ctx.Done():
		// Dapatkan nama fungsi caller
		pc, _, _, ok := runtime.Caller(1)
		funcName := "unknown"
		if ok {
			fn := runtime.FuncForPC(pc)
			if fn != nil {
				// Ambil hanya nama fungsi (tanpa path lengkap)
				parts := strings.Split(fn.Name(), "/")
				funcName = parts[len(parts)-1]
			}
		}

		log.Warn("Context cancelled",
			logger.StringField("caller", funcName),
		)
		return false
	default:
		return true
	}
}

func ShouldStopChan(stop <-chan struct{}, log *logger.Logger) bool {
	select {
	case <-stop:
		// Dapatkan nama fungsi caller
		pc, _, _, ok := runtime.Caller(1)
		funcName := "unknown"
		if ok {
			fn := runtime.FuncForPC(pc)
			if fn != nil {
				// Ambil hanya nama fungsi (tanpa path lengkap)
				parts := strings.Split(fn.Name(), "/")
				funcName = parts[len(parts)-1]
			}
		}

		log.Debug("Stop signal received",
			logger.StringField("caller", funcName),
		)
		return true
	default:
		return false
	}
}

func EscapeMarkdownV2(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
}

func CapitalizeSentence(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	runes := []rune(input)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func FormatPercentage(value float64) string {
	return fmt.Sprintf("%+.1f%%", value)
}

func ParseStockSymbol(symbol string) (stockCode string, exchange string, err error) {
	if symbol == "" {
		return "", "", fmt.Errorf("symbol is required")
	}

	result := strings.Split(symbol, ":")
	if len(result) != 2 {
		return "", "", fmt.Errorf("invalid symbol format")
	}

	stockCode = result[1]
	exchange = result[0]
	return
}

func FormatVolume(volume int64) string {
	switch {
	case volume >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(volume)/1e9)
	case volume >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(volume)/1e6)
	case volume >= 1_000:
		return fmt.Sprintf("%.1fK", float64(volume)/1e3)
	default:
		return fmt.Sprintf("%d", volume)
	}
}

func CalculateChangePercent(open, close float64) float64 {
	if open == 0 {
		return 0
	}
	return ((close - open) / open) * 100
}

func FormatChange(open, close float64) string {
	chg := CalculateChangePercent(open, close)
	sign := "+"
	if chg < 0 {
		sign = "-"
		chg = -chg
	}
	return fmt.Sprintf("%s%.2f%%", sign, chg)
}

func CalculateRiskRewardRatio(buyPrice, targetPrice, stopLoss float64) (float64, string, error) {
	if buyPrice <= 0 || targetPrice <= 0 || stopLoss <= 0 {
		return 0, "", fmt.Errorf("harga tidak boleh <= 0")
	}

	risk := math.Abs(buyPrice - stopLoss)
	reward := math.Abs(targetPrice - buyPrice)

	if risk == 0 {
		return 0, "", fmt.Errorf("risk = 0, tidak valid")
	}

	ratio := reward / risk
	roundedRatio := math.Round(ratio*100) / 100 // bulatkan 2 angka desimal

	// Representasi dalam bentuk string (misal "1:2")
	stringRatio := fmt.Sprintf("1:%.2f", roundedRatio)

	return roundedRatio, stringRatio, nil
}

func PrettyKey(key string) string {
	// Replace underscores with space
	key = strings.ReplaceAll(key, "_", " ")

	// Add space before capital letters (for camelCase)
	var result strings.Builder
	for i, r := range key {
		if i > 0 && unicode.IsUpper(r) && key[i-1] != ' ' {
			result.WriteRune(' ')
		}
		result.WriteRune(r)
	}

	// Capitalize each word
	words := strings.Fields(result.String())
	for i, word := range words {
		words[i] = strings.Title(strings.ToLower(word))
	}
	return strings.Join(words, " ")
}
