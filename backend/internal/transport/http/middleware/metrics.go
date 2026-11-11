package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	handler  http.Handler
}

func NewMetrics(instance string) *Metrics {
	reg := prometheus.NewRegistry()
	labels := prometheus.Labels{"instance": instance}
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "chimera_http_requests_total",
		Help:        "HTTP requests served by this process.",
		ConstLabels: labels,
	}, []string{"method", "path", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "chimera_http_request_duration_seconds",
		Help:        "HTTP request duration in seconds.",
		ConstLabels: labels,
		Buckets:     prometheus.DefBuckets,
	}, []string{"method", "path", "status"})
	reg.MustRegister(
		requests,
		duration,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return &Metrics{
		requests: requests,
		duration: duration,
		handler:  promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
	}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		path := routePattern(r)
		status := strconv.Itoa(sw.status)
		m.requests.WithLabelValues(r.Method, path, status).Inc()
		m.duration.WithLabelValues(r.Method, path, status).Observe(time.Since(start).Seconds())
	})
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.handler.ServeHTTP(w, r)
}

func routePattern(r *http.Request) string {
	rc := chi.RouteContext(r.Context())
	if rc == nil {
		return "unmatched"
	}
	if pattern := rc.RoutePattern(); pattern != "" {
		return pattern
	}
	return "unmatched"
}
