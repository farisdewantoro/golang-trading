package logger

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger
type Logger struct {
	*zap.Logger
}

// New creates a new logger instance
func New(level, encoding string) (*Logger, error) {
	var config zap.Config

	if encoding == "console" {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else { // default to json
		config = zap.NewProductionConfig()
		// Use ISO8601 time format as it's more human-readable and parsed by Railway.
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		// Use Zap's default keys ("ts", "level", "msg") which are common standards
		// that log aggregators like Railway are more likely to parse correctly.
		config.EncoderConfig.TimeKey = "ts"
		config.EncoderConfig.LevelKey = "level"
		config.EncoderConfig.MessageKey = "msg"
		config.EncoderConfig.CallerKey = "caller"
		config.EncoderConfig.StacktraceKey = "stacktrace"
		config.EncoderConfig.NameKey = "logger"
	}

	logLevel := zap.NewAtomicLevel()
	if err := logLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	config.Level = logLevel

	logger, err := config.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, err
	}

	return &Logger{logger}, nil
}

// With creates a child logger with the given fields
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{l.Logger.With(fields...)}
}

// FromContext retrieves a logger from context if it exists, or returns the default logger
func (l *Logger) FromContext(ctx context.Context) *Logger {
	if ctx == nil {
		return l
	}

	loggerFromCtx, ok := ctx.Value(loggerContextKey).(*Logger)
	if !ok || loggerFromCtx == nil {
		return l
	}

	return loggerFromCtx
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

// DebugContext logs a debug message with context
func (l *Logger) DebugContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.FromContext(ctx).Debug(msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

// InfoContext logs an info message with context
func (l *Logger) InfoContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.FromContext(ctx).Info(msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

// WarnContext logs a warning message with context
func (l *Logger) WarnContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.FromContext(ctx).Warn(msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

// ErrorContext logs an error message with context
func (l *Logger) ErrorContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.FromContext(ctx).Error(msg, fields...)
}

// Fatal logs a fatal message and then calls os.Exit(1)
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, fields...)
}

// FatalContext logs a fatal message with context and then calls os.Exit(1)
func (l *Logger) FatalContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.FromContext(ctx).Fatal(msg, fields...)
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// Field creates a zap.Field
func Field(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

// StringField creates a zap.Field with string value
func StringField(key, value string) zap.Field {
	return zap.String(key, value)
}

// IntField creates a zap.Field with int value
func IntField(key string, value int) zap.Field {
	return zap.Int(key, value)
}

// ErrorField creates a zap.Field with error value
func ErrorField(err error) zap.Field {
	return zap.Error(err)
}

// Context key for logger
type contextKey string

const loggerContextKey contextKey = "logger"

// NewContext creates a new context with the logger
func NewContext(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}
