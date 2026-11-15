package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"chimera/platform/runctx"
)

func TestContextHandlerAddsTraceAndRun(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})

	tests := []struct {
		name string
		ctx  context.Context
		want map[string]string
		none []string
	}{
		{
			name: "span and run",
			ctx:  runctx.With(trace.ContextWithSpanContext(context.Background(), spanCtx), runctx.Run{ID: "go-1", Step: "02"}),
			want: map[string]string{"trace_id": traceID.String(), "span_id": spanID.String(), "run_id": "go-1", "step": "02"},
		},
		{
			name: "span only",
			ctx:  trace.ContextWithSpanContext(context.Background(), spanCtx),
			want: map[string]string{"trace_id": traceID.String()},
			none: []string{"run_id"},
		},
		{
			name: "neither",
			ctx:  context.Background(),
			none: []string{"trace_id", "span_id", "run_id"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(contextHandler{slog.NewJSONHandler(&buf, nil)})
			log.InfoContext(tt.ctx, "hello")

			var line map[string]any
			if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
				t.Fatalf("decode %q: %v", buf.String(), err)
			}
			for k, v := range tt.want {
				if line[k] != v {
					t.Fatalf("%s = %v, want %q in %s", k, line[k], v, buf.String())
				}
			}
			for _, k := range tt.none {
				if _, ok := line[k]; ok {
					t.Fatalf("unexpected %s in %s", k, buf.String())
				}
			}
		})
	}
}
