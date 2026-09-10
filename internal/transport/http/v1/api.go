package v1

import "github.com/go-chi/chi/v5"

type API struct {
	users  *UserController
	auth   *AuthController
	tracks *TrackController
}

func New(users UserService, auth AuthService, tracks TrackService, log Logger) *API {
	return &API{
		users:  NewUserController(users, log),
		auth:   NewAuthController(auth, log),
		tracks: NewTrackController(tracks, log),
	}
}

func (a *API) Public(r chi.Router) {
	r.Post("/auth/register", a.auth.Register)
	r.Post("/auth/login", a.auth.Login)
	r.Get("/users", a.users.ListUsers)
	r.Get("/users/{id}", a.users.GetUser)
	r.Get("/tracks", a.tracks.ListTracks)
	r.Get("/tracks/{id}", a.tracks.GetTrack)
}

func (a *API) Protected(r chi.Router) {
	r.Get("/me", a.users.Me)
	r.Post("/users", a.users.CreateUser)
	r.Put("/users/{id}", a.users.UpdateUser)
	r.Delete("/users/{id}", a.users.DeleteUser)
	r.Post("/tracks", a.tracks.CreateTrack)
	r.Put("/tracks/{id}", a.tracks.UpdateTrack)
	r.Delete("/tracks/{id}", a.tracks.DeleteTrack)
}
