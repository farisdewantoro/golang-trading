package utils

import (
	"bytes"
	"context"
	"fmt"
	"golang-trading/pkg/logger"
	"html"
	"log"
	"runtime"
	"strings"
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

// GoSafe runs the given function in a new goroutine and recovers from any panic.
func GoSafe(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Panic Recovered] %v", r)
			}
		}()
		fn()
	}()
}

func ToPointer[T any](value T) *T {
	return &value
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
