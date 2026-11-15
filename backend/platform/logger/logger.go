package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"chimera/platform/env"
	"chimera/platform/runctx"
)

func New(service string, mode env.Mode, levelName string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level(mode, levelName)}
	h := slog.Handler(slog.NewTextHandler(os.Stdout, opts))
	if mode == env.ModeProduction {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(runHandler{h}).With("service", service, "mode", string(mode))
}

// runHandler adds run_id and step from the context to every record, so all
// lines written for one test request carry the same run id in every service.
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

func level(mode env.Mode, name string) slog.Leveler {
	switch strings.ToLower(name) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	if mode == env.ModeProduction {
		return slog.LevelInfo
	}
	return slog.LevelDebug
}
