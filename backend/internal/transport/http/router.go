package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"

	"chimera/internal/transport/http/middleware"
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
	ReadOnly    bool
	Instance    string
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	metrics := middleware.NewMetrics(deps.Instance)
	r.Use(middleware.Instance(deps.Instance))
	r.Use(metrics.Middleware)
	r.Use(middleware.CORS(deps.CORSOrigins))
	r.Use(middleware.RunTrace(deps.Log))
	r.Use(middleware.RequestLogger(deps.Log))
	if deps.ReadOnly {
		r.Use(middleware.ReadOnly(deps.Log, WriteAppError))
	}

	r.Get("/metrics", metrics.ServeHTTP)
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

	return r
}
