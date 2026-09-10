package logger

import (
	"log/slog"
	"os"
	"strings"

	"chimera/internal/config"
)

func New(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level(cfg)}
	h := slog.Handler(slog.NewTextHandler(os.Stdout, opts))
	if cfg.Mode == config.ModeProduction {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h).With("mode", string(cfg.Mode))
}

func level(cfg config.Config) slog.Leveler {
	switch strings.ToLower(cfg.Log.Level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	if cfg.Mode == config.ModeProduction {
		return slog.LevelInfo
	}
	return slog.LevelDebug
}
