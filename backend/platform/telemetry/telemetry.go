// Package telemetry starts the OpenTelemetry tracer and meter of a process.
//
// Spans go to an OTLP/gRPC collector, metrics to a Prometheus registry served on
// /metrics. W3C traceparent carries the trace between processes, so the trace id
// is the correlation id of a request in every log line, span and Kafka record.
package telemetry

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"chimera/platform/apperr"
	"chimera/platform/env"
)

type Telemetry struct {
	tracer   *sdktrace.TracerProvider
	meter    *sdkmetric.MeterProvider
	registry *prometheus.Registry
	// Registerer adds metrics of the process; each series gets the pod label.
	Registerer prometheus.Registerer
}

// Start installs the process-wide tracer, meter and propagator; the
// instrumentation libraries (otelgrpc, otelhttp, otelpgx) read them from otel.
// Without an endpoint spans stay in the process, and trace ids still tie the logs together.
func Start(ctx context.Context, service string, base env.Base) (*Telemetry, error) {
	res := resource.NewSchemaless(
		semconv.ServiceName(service),
		semconv.ServiceInstanceID(base.Instance),
	)

	traceOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if base.OTLPEndpoint != "" {
		exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(base.OTLPEndpoint))
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeInternal, "open otlp exporter", err)
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(exporter))
	}
	tracer := sdktrace.NewTracerProvider(traceOpts...)

	// Prometheus finds the pods through DNS and knows only their addresses, so
	// the series carry the pod name themselves.
	reg := prometheus.NewRegistry()
	labeled := prometheus.WrapRegistererWith(prometheus.Labels{"pod": base.Instance}, reg)
	labeled.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	reader, err := otelprom.New(otelprom.WithRegisterer(labeled), otelprom.WithoutScopeInfo())
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInternal, "open prometheus exporter", err)
	}
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(reader))

	otel.SetTracerProvider(tracer)
	otel.SetMeterProvider(meter)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return &Telemetry{tracer: tracer, meter: meter, registry: reg, Registerer: labeled}, nil
}

// Shutdown flushes the spans still in the batcher.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	return errors.Join(t.tracer.Shutdown(ctx), t.meter.Shutdown(ctx))
}

type Logger interface {
	ErrorContext(ctx context.Context, msg string, args ...any)
}

// Flush shuts the providers down within timeout, after the process stopped serving.
func (t *Telemetry) Flush(log Logger, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := t.Shutdown(ctx); err != nil {
		log.ErrorContext(ctx, "flush telemetry", "error", err)
	}
}

// Serve runs ServeMetrics in the background and logs a failure.
func (t *Telemetry) Serve(ctx context.Context, log Logger, addr string, readHeaderTimeout time.Duration) {
	go func() {
		if err := t.ServeMetrics(ctx, addr, readHeaderTimeout); err != nil {
			log.ErrorContext(ctx, "metrics server stopped", "error", err)
		}
	}()
}

func (t *Telemetry) MetricsHandler() http.Handler {
	return promhttp.HandlerFor(t.registry, promhttp.HandlerOpts{})
}

// ServeMetrics serves /metrics on addr until ctx ends. An empty addr serves nothing.
func (t *Telemetry) ServeMetrics(ctx context.Context, addr string, readHeaderTimeout time.Duration) error {
	if addr == "" {
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", t.MetricsHandler())
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: readHeaderTimeout}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return apperr.Wrap(apperr.CodeInternal, "serve metrics", err)
	}
	return nil
}
