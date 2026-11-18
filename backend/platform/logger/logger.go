package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"chimera/platform/env"
	"chimera/platform/runctx"
)

func New(service string, base env.Base) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level(base.Mode, base.LogLevel)}
	h := slog.Handler(slog.NewTextHandler(os.Stdout, opts))
	if base.Mode == env.ModeProduction {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(contextHandler{h}).With("service", service, "instance", base.Instance, "mode", string(base.Mode))
}

// contextHandler adds the request's trace_id and span_id, and run_id and step of a
// test run, to every record. One trace id then finds the lines of a request in
// every service, and Grafana links each line to its trace.
type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, rec slog.Record) error {
	span := trace.SpanContextFromContext(ctx)
	run, hasRun := runctx.From(ctx)
	if !span.IsValid() && !hasRun {
		return h.Handler.Handle(ctx, rec)
	}
	rec = rec.Clone()
	if span.IsValid() {
		rec.AddAttrs(slog.String("trace_id", span.TraceID().String()), slog.String("span_id", span.SpanID().String()))
	}
	if hasRun {
		rec.AddAttrs(slog.String("run_id", run.ID))
		if run.Step != "" {
			rec.AddAttrs(slog.String("step", run.Step))
		}
	}
	return h.Handler.Handle(ctx, rec)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
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
