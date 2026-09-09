package logger

import (
	"log/slog"
	"os"
	"strings"

	"chimera/internal/business_logic/port"
	"chimera/internal/config"
)

var _ port.Logger = (*slog.Logger)(nil)

func New(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level(cfg)}

	var handler slog.Handler
	if cfg.Mode.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler).With("mode", string(cfg.Mode))
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

	if cfg.Mode.IsProduction() {
		return slog.LevelInfo
	}
	return slog.LevelDebug
}
