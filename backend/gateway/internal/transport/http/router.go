package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"chimera/gateway/internal/transport/http/middleware"
)

type Routes interface {
	Public(r chi.Router)
	Protected(r chi.Router)
}

type Dependencies struct {
	Log         middleware.Logger
	Tokens      middleware.TokenParser
	Routes      Routes
	CORSOrigins []string
	Health      Readiness
	Instance    string
	// Metrics receives the HTTP request metrics; MetricsHandler serves them
	// together with the OTel and runtime metrics of the process.
	Metrics        prometheus.Registerer
	MetricsHandler http.Handler
}

// NewRouter serves the public API. Each request gets a server span, continuing
// the trace of an incoming traceparent.
func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	metrics := middleware.NewMetrics(deps.Metrics)
	r.Use(middleware.Trace)
	r.Use(middleware.Instance(deps.Instance))
	r.Use(metrics.Middleware)
	r.Use(middleware.CORS(deps.CORSOrigins))
	r.Use(middleware.RunTrace(deps.Log))
	r.Use(middleware.RequestLogger(deps.Log))
	r.Use(middleware.Idempotency(deps.Log, WriteAppError))
	r.Handle("/metrics", deps.MetricsHandler)
	r.Get("/live", Live)
	r.Get("/ready", Ready(deps.Health))
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Route("/v1", func(r chi.Router) {
		deps.Routes.Public(r)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(deps.Log, deps.Tokens, WriteAppError))
			deps.Routes.Protected(r)
		})
	})

	return otelhttp.NewHandler(r, "http",
		otelhttp.WithFilter(func(r *http.Request) bool { return !probePath(r.URL.Path) }),
	)
}

func probePath(path string) bool {
	return path == "/live" || path == "/ready" || path == "/metrics"
}
