package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"chimera/gateway/internal/transport/http/middleware"
)

func TestMetricsRecordsRoutePattern(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := middleware.NewMetrics(reg)
	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	r.Get("/v1/tracks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tracks", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "chimera_http_requests_total{") {
		t.Fatalf("counter missing: %s", body)
	}
	if !strings.Contains(body, `path="/v1/tracks"`) {
		t.Fatalf("path missing: %s", body)
	}
}
