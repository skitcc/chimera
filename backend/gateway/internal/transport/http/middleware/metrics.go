package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewMetrics registers the HTTP request metrics in reg. The pod comes from the
// scrape target, so the series carry no instance label of their own.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chimera_http_requests_total",
		Help: "HTTP requests served by this process.",
	}, []string{"method", "path", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "chimera_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})
	reg.MustRegister(requests, duration)
	return &Metrics{requests: requests, duration: duration}
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
