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
	auth      *AuthController
	tracks    *TrackController
	follows   *FollowController
	playlists *PlaylistController
	picks     *PickController
	log       Logger
	tokens    middleware.TokenParser
	limit     *middleware.Limiter
}

func New(
	users UserService,
	auth AuthService,
	tracks TrackService,
	follows FollowService,
	playlists PlaylistService,
	picks PickService,
	tokens middleware.TokenParser,
	limit *middleware.Limiter,
	streamHost string,
	log Logger,
) *API {
	return &API{
		users:     NewUserController(users, log),
		auth:      NewAuthController(auth, log),
		tracks:    NewTrackController(tracks, streamHost, log),
		follows:   NewFollowController(follows, log),
		playlists: NewPlaylistController(playlists, log),
		picks:     NewPickController(picks, log),
		log:       log,
		tokens:    tokens,
		limit:     limit,
	}
}

func (a *API) Public(r chi.Router) {
	limited := r.With(middleware.RateLimit(a.limit, a.log, httpapi.WriteAppError))
	limited.Post("/auth/register", a.auth.Register)
	limited.Post("/auth/login", a.auth.Login)
	r.Get("/users", a.users.ListUsers)
	r.Get("/users/{userId}/tracks", a.tracks.ListUploaderTracks)
	r.Get("/users/{userId}/playlists", a.playlists.ListUserPlaylists)
	r.Get("/users/{userId}/followers", a.follows.ListFollowers)
	r.Get("/users/{userId}/following", a.follows.ListUserFollowing)
	r.Get("/users/{userId}", a.users.GetUser)
	r.Get("/tracks", a.tracks.ListTracks)
	r.Get("/tracks/{trackId}/stream", a.tracks.StreamTrack)
	r.With(middleware.OptionalAuth(a.log, a.tokens, httpapi.WriteAppError)).Get("/tracks/{trackId}", a.tracks.GetTrack)
	r.With(middleware.OptionalAuth(a.log, a.tokens, httpapi.WriteAppError)).Get("/playlists/{playlistId}", a.playlists.Get)
	r.With(middleware.OptionalAuth(a.log, a.tokens, httpapi.WriteAppError)).Get("/playlists/{playlistId}/tracks", a.playlists.ListTracks)
}

func (a *API) Protected(r chi.Router) {
	r.Get("/me", a.users.Me)
	r.Put("/me", a.users.ReplaceMe)
	r.Patch("/me", a.users.PatchMe)
	r.Delete("/me", a.users.DeleteMe)
	r.Get("/me/tracks", a.tracks.ListMyTracks)
	r.Get("/me/likes", a.tracks.ListLikedTracks)
	r.Get("/me/playlists", a.playlists.ListMyPlaylists)
	r.Get("/me/following", a.follows.ListMyFollowing)
	r.Get("/me/picks", a.picks.List)
	r.Post("/me/picks", a.picks.Generate)
	r.Get("/me/picks/{pickId}", a.picks.Get)
	r.Patch("/me/picks/{pickId}", a.picks.Rename)
	r.Delete("/me/picks/{pickId}", a.picks.Delete)
	r.Put("/users/{userId}/follow", a.follows.Follow)
	r.Delete("/users/{userId}/follow", a.follows.Unfollow)
	r.Post("/tracks/{trackId}/like", a.tracks.LikeTrack)
	r.Delete("/tracks/{trackId}/like", a.tracks.UnlikeTrack)
	r.Post("/tracks/upload-init", a.tracks.InitUpload)
	r.Post("/tracks/{trackId}/upload-complete", a.tracks.CompleteUpload)
	r.Put("/tracks/{trackId}", a.tracks.UpdateTrack)
	r.Patch("/tracks/{trackId}", a.tracks.PatchTrack)
	r.Delete("/tracks/{trackId}", a.tracks.DeleteTrack)
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
