package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
	"chimera/internal/transport/http/middleware"
)

type API struct {
	users     *UserController
	sessions  *SessionController
	tracks    *TrackController
	follows   *FollowController
	playlists *PlaylistController
	log       Logger
	tokens    middleware.TokenParser
	limit     *middleware.Limiter
}

func New(
	users UserService,
	sessions SessionService,
	tracks TrackService,
	follows FollowService,
	playlists PlaylistService,
	tokens middleware.TokenParser,
	limit *middleware.Limiter,
	streamHost string,
	log Logger,
) *API {
	return &API{
		users:     NewUserController(users, log),
		sessions:  NewSessionController(sessions, log),
		tracks:    NewTrackController(tracks, streamHost, log),
		follows:   NewFollowController(follows, log),
		playlists: NewPlaylistController(playlists, log),
		log:       log,
		tokens:    tokens,
		limit:     limit,
	}
}

func (a *API) Public(r chi.Router) {
	limited := r.With(middleware.RateLimit(a.limit, a.log, httpapi.WriteAppError))
	limited.Post("/sessions", a.sessions.Create)
	limited.Post("/users", a.users.Create)
	r.Get("/users", a.users.List)
	r.Get("/users/{userId}", a.users.Get)
	r.Get("/follows", a.follows.List)
	r.Get("/follows/{followerId}/{followingId}", a.follows.Get)
	r.Get("/tracks/{trackId}/stream", a.tracks.Stream)

	optional := r.With(middleware.OptionalAuth(a.log, a.tokens, httpapi.WriteAppError))
	optional.Get("/tracks", a.tracks.List)
	optional.Get("/tracks/{trackId}", a.tracks.Get)
	optional.Get("/playlists", a.playlists.List)
	optional.Get("/playlists/{playlistId}", a.playlists.Get)
	optional.Get("/playlists/{playlistId}/tracks", a.playlists.ListTracks)
}

func (a *API) Protected(r chi.Router) {
	r.Put("/users/{userId}", a.users.Replace)
	r.Patch("/users/{userId}", a.users.Patch)
	r.Delete("/users/{userId}", a.users.Delete)
	r.Put("/users/{userId}/likes/{trackId}", a.tracks.PutLike)
	r.Delete("/users/{userId}/likes/{trackId}", a.tracks.DeleteLike)
	r.Post("/follows", a.follows.CreateMany)
	r.Delete("/follows", a.follows.DeleteMany)
	r.Put("/follows/{followerId}/{followingId}", a.follows.Put)
	r.Delete("/follows/{followerId}/{followingId}", a.follows.Delete)
	r.Post("/tracks", a.tracks.Create)
	r.Put("/tracks/{trackId}", a.tracks.Replace)
	r.Patch("/tracks/{trackId}", a.tracks.Patch)
	r.Delete("/tracks/{trackId}", a.tracks.Delete)
	r.Post("/playlists", a.playlists.Create)
	r.Put("/playlists/{playlistId}", a.playlists.Replace)
	r.Patch("/playlists/{playlistId}", a.playlists.Patch)
	r.Delete("/playlists/{playlistId}", a.playlists.Delete)
	r.Post("/playlists/{playlistId}/tracks", a.playlists.AddTrack)
	r.Patch("/playlists/{playlistId}/tracks/{trackId}", a.playlists.MoveTrack)
	r.Delete("/playlists/{playlistId}/tracks/{trackId}", a.playlists.RemoveTrack)
}

func actorID(w http.ResponseWriter, r *http.Request, log Logger, op string) (string, bool) {
	id, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		httpapi.WriteAppError(r.Context(), w, log, op, domain.Unauthorized("missing token"))
		return "", false
	}
	return id, true
}

func writeCreated(w http.ResponseWriter, location string, v any) {
	w.Header().Set("Location", location)
	httpapi.WriteJSON(w, http.StatusCreated, v)
}
