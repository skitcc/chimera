package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
	"chimera/internal/transport/http/middleware"
)

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
	r.Get("/users/{id}/tracks", a.tracks.ListUploaderTracks)
	r.Get("/users/{id}", a.users.GetUser)
	r.Get("/tracks", a.tracks.ListTracks)
	r.Get("/tracks/{id}/stream", a.tracks.StreamTrack)
	r.Get("/tracks/{id}", a.tracks.GetTrack)
}

func (a *API) Protected(r chi.Router) {
	r.Get("/me", a.users.Me)
	r.Get("/me/tracks", a.tracks.ListMyTracks)
	r.Post("/users", a.users.CreateUser)
	r.Put("/users/{id}", a.users.UpdateUser)
	r.Delete("/users/{id}", a.users.DeleteUser)
	r.Post("/tracks/upload-init", a.tracks.InitUpload)
	r.Post("/tracks/{id}/upload-complete", a.tracks.CompleteUpload)
	r.Put("/tracks/{id}", a.tracks.UpdateTrack)
	r.Delete("/tracks/{id}", a.tracks.DeleteTrack)
}

func actorID(w http.ResponseWriter, r *http.Request, log Logger, op string) (string, bool) {
	id, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		httpapi.WriteAppError(r.Context(), w, log, op, domain.Unauthorized("missing token"))
		return "", false
	}
	return id, true
}
