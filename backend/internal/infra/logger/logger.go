package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"chimera/internal/config"
	"chimera/internal/runctx"
)

func New(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level(cfg)}
	h := slog.Handler(slog.NewTextHandler(os.Stdout, opts))
	if cfg.Mode == config.ModeProduction {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(runHandler{h}).With("mode", string(cfg.Mode))
}

// runHandler adds run_id and step from the context to every record, so all
// lines written for one test request carry the same run id.
type runHandler struct {
	slog.Handler
}

func (h runHandler) Handle(ctx context.Context, rec slog.Record) error {
	run, ok := runctx.From(ctx)
	if !ok {
		return h.Handler.Handle(ctx, rec)
	}
	rec = rec.Clone()
	rec.AddAttrs(slog.String("run_id", run.ID))
	if run.Step != "" {
		rec.AddAttrs(slog.String("step", run.Step))
	}
	return h.Handler.Handle(ctx, rec)
}

func (h runHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return runHandler{h.Handler.WithAttrs(attrs)}
}

func (h runHandler) WithGroup(name string) slog.Handler {
	return runHandler{h.Handler.WithGroup(name)}
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
