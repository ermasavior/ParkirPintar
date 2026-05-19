package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// LogConfig holds logger configuration.
type LogConfig struct {
	Level  string // "debug" | "info" | "warn" | "error"
	Format string // "json" | "text"
}

var Logger = slog.Default()

// SetupLogger initialises the slog logger from config.
func SetupLogger(cfg LogConfig) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	replaceAttr := func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Int64(slog.TimeKey, a.Value.Time().UnixMilli())
		}
		if a.Key == slog.LevelKey {
			return slog.String(slog.LevelKey, strings.ToLower(a.Value.String()))
		}
		return a
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level, ReplaceAttr: replaceAttr})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level, ReplaceAttr: replaceAttr})
	}

	Logger = slog.New(handler)
	slog.SetDefault(Logger)
	return Logger
}

func Info(ctx context.Context, msg string, args ...any) {
	Logger.InfoContext(ctx, msg, args...)
}

func Error(ctx context.Context, msg string, args ...any) {
	Logger.ErrorContext(ctx, msg, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	Logger.WarnContext(ctx, msg, args...)
}
