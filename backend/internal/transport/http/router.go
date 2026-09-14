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
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.CORS(deps.CORSOrigins))
	r.Use(middleware.RequestLogger(deps.Log))

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
