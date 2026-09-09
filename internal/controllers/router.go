package controllers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"

	"chimera/internal/business_logic/port"
)

type Dependencies struct {
	Users  port.UserService
	Auth   port.AuthService
	Tracks port.TrackService
	Log    port.Logger
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(requestLogger(deps.Log))

	users := NewUserController(deps.Users, deps.Log)
	auth := NewAuthController(deps.Auth, deps.Log)
	tracks := NewTrackController(deps.Tracks, deps.Log)

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/auth/register", auth.Register)
		r.Post("/auth/login", auth.Login)

		r.Get("/users", users.ListUsers)
		r.Post("/users", users.CreateUser)
		r.Get("/users/{id}", users.GetUser)
		r.Put("/users/{id}", users.UpdateUser)
		r.Delete("/users/{id}", users.DeleteUser)

		r.Get("/tracks", tracks.ListTracks)
		r.Post("/tracks", tracks.CreateTrack)
		r.Get("/tracks/{id}", tracks.GetTrack)
		r.Put("/tracks/{id}", tracks.UpdateTrack)
		r.Delete("/tracks/{id}", tracks.DeleteTrack)
	})

	return r
}
