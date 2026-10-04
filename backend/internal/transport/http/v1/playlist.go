package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
	"chimera/internal/transport/http/middleware"
)

type PlaylistController struct {
	playlists PlaylistService
	log       Logger
}

func NewPlaylistController(playlists PlaylistService, log Logger) *PlaylistController {
	return &PlaylistController{playlists: playlists, log: log}
}

// ListUserPlaylists godoc
// @Summary List a user's public playlists
// @ID listUserPlaylists
// @Tags playlists
// @Produce json
// @Param userId path string true "User ID"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} PlaylistPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /v1/users/{userId}/playlists [get]
func (c *PlaylistController) ListUserPlaylists(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	page, err := c.playlists.ListByOwner(r.Context(), userID, true, httpapi.ParsePageQuery(r))
	c.writePage(w, r, "list user playlists", page, err)
}

// ListMyPlaylists godoc
// @Summary List current user's playlists
// @ID listMyPlaylists
// @Tags playlists
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} PlaylistPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me/playlists [get]
func (c *PlaylistController) ListMyPlaylists(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "list my playlists")
	if !ok {
		return
	}
	page, err := c.playlists.ListByOwner(r.Context(), actor, false, httpapi.ParsePageQuery(r))
	c.writePage(w, r, "list my playlists", page, err)
}

// Create godoc
// @Summary Create a playlist
// @ID createPlaylist
// @Tags playlists
// @Accept json
// @Produce json
// @Param body body PlaylistReplaceRequest true "Playlist"
// @Success 201 {object} PlaylistResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/playlists [post]
func (c *PlaylistController) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "create playlist")
	if !ok {
		return
	}
	in, ok := c.replaceInput(w, r, "create playlist")
	if !ok {
		return
	}
	playlist, err := c.playlists.Create(r.Context(), actor, in)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create playlist", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, playlistToResponse(playlist))
}

// Get godoc
// @Summary Get a visible playlist
// @ID getPlaylist
// @Tags playlists
// @Produce json
// @Param playlistId path string true "Playlist ID"
// @Success 200 {object} PlaylistResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /v1/playlists/{playlistId} [get]
func (c *PlaylistController) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "playlistId")
	viewer, _ := middleware.UserIDFromCtx(r.Context())
	playlist, err := c.playlists.Get(r.Context(), id, viewer)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "get playlist", err, "playlist_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistToResponse(playlist))
}

// Replace godoc
// @Summary Replace a playlist
// @ID replacePlaylist
// @Tags playlists
// @Accept json
// @Produce json
// @Param playlistId path string true "Playlist ID"
// @Param body body PlaylistReplaceRequest true "Playlist"
// @Success 200 {object} PlaylistResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/playlists/{playlistId} [put]
func (c *PlaylistController) Replace(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "replace playlist")
	if !ok {
		return
	}
	in, ok := c.replaceInput(w, r, "replace playlist")
	if !ok {
		return
	}
	id := chi.URLParam(r, "playlistId")
	playlist, err := c.playlists.Replace(r.Context(), actor, id, in)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace playlist", err, "playlist_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistToResponse(playlist))
}

// Patch godoc
// @Summary Update playlist fields
// @ID updatePlaylist
// @Tags playlists
// @Accept json
// @Produce json
// @Param playlistId path string true "Playlist ID"
// @Param body body PlaylistPatchRequest true "Playlist fields"
// @Success 200 {object} PlaylistResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/playlists/{playlistId} [patch]
func (c *PlaylistController) Patch(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "patch playlist")
	if !ok {
		return
	}
	var req PlaylistPatchRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch playlist", err)
		return
	}
	id := chi.URLParam(r, "playlistId")
	playlist, err := c.playlists.Patch(r.Context(), actor, id, domain.PlaylistPatch{
		Title: req.Title, IsPublic: req.IsPublic,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch playlist", err, "playlist_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistToResponse(playlist))
}

// Delete godoc
// @Summary Delete a playlist
// @ID deletePlaylist
// @Tags playlists
// @Param playlistId path string true "Playlist ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/playlists/{playlistId} [delete]
func (c *PlaylistController) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "delete playlist")
	if !ok {
		return
	}
	id := chi.URLParam(r, "playlistId")
	if err := c.playlists.Delete(r.Context(), actor, id); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete playlist", err, "playlist_id", id)
		return
	}
	httpapi.WriteNoContent(w)
}

// ListTracks godoc
// @Summary List tracks in a visible playlist
// @ID listPlaylistTracks
// @Tags playlists
// @Produce json
// @Param playlistId path string true "Playlist ID"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} PlaylistTrackPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /v1/playlists/{playlistId}/tracks [get]
func (c *PlaylistController) ListTracks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "playlistId")
	viewer, _ := middleware.UserIDFromCtx(r.Context())
	page, err := c.playlists.ListTracks(r.Context(), id, viewer, httpapi.ParsePageQuery(r))
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "list playlist tracks", err, "playlist_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistTrackPageToResponse(page))
}

// AddTrack godoc
// @Summary Add a track to a playlist
// @ID addPlaylistTrack
// @Tags playlists
// @Accept json
// @Produce json
// @Param playlistId path string true "Playlist ID"
// @Param body body PlaylistTrackAddRequest true "Track and position"
// @Success 201 {object} PlaylistTrackResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/playlists/{playlistId}/tracks [post]
func (c *PlaylistController) AddTrack(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "add playlist track")
	if !ok {
		return
	}
	var req PlaylistTrackAddRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "add playlist track", err)
		return
	}
	if req.Position == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "add playlist track", domain.Invalid("position is required"))
		return
	}
	id := chi.URLParam(r, "playlistId")
	item, err := c.playlists.AddTrack(r.Context(), actor, id, domain.PlaylistTrackAdd{
		TrackID: req.TrackID, Position: *req.Position,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "add playlist track", err, "playlist_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, playlistTrackToResponse(item))
}

// MoveTrack godoc
// @Summary Move a track within a playlist
// @ID movePlaylistTrack
// @Tags playlists
// @Accept json
// @Produce json
// @Param playlistId path string true "Playlist ID"
// @Param trackId path string true "Track ID"
// @Param body body PlaylistTrackMoveRequest true "New position"
// @Success 200 {object} PlaylistTrackResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/playlists/{playlistId}/tracks/{trackId} [patch]
func (c *PlaylistController) MoveTrack(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "move playlist track")
	if !ok {
		return
	}
	var req PlaylistTrackMoveRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "move playlist track", err)
		return
	}
	if req.Position == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "move playlist track", domain.Invalid("position is required"))
		return
	}
	id := chi.URLParam(r, "playlistId")
	trackID := chi.URLParam(r, "trackId")
	item, err := c.playlists.MoveTrack(r.Context(), actor, id, trackID, domain.PlaylistTrackMove{
		Position: *req.Position,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "move playlist track", err, "playlist_id", id, "track_id", trackID)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistTrackToResponse(item))
}

// RemoveTrack godoc
// @Summary Remove a track from a playlist
// @ID removePlaylistTrack
// @Tags playlists
// @Param playlistId path string true "Playlist ID"
// @Param trackId path string true "Track ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/playlists/{playlistId}/tracks/{trackId} [delete]
func (c *PlaylistController) RemoveTrack(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "remove playlist track")
	if !ok {
		return
	}
	id := chi.URLParam(r, "playlistId")
	trackID := chi.URLParam(r, "trackId")
	if err := c.playlists.RemoveTrack(r.Context(), actor, id, trackID); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "remove playlist track", err, "playlist_id", id, "track_id", trackID)
		return
	}
	httpapi.WriteNoContent(w)
}

func (c *PlaylistController) replaceInput(w http.ResponseWriter, r *http.Request, op string) (domain.PlaylistReplace, bool) {
	var req PlaylistReplaceRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, op, err)
		return domain.PlaylistReplace{}, false
	}
	if req.Title == nil || req.IsPublic == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, op, domain.Invalid("title and is_public are required"))
		return domain.PlaylistReplace{}, false
	}
	return domain.PlaylistReplace{Title: *req.Title, IsPublic: *req.IsPublic}, true
}

func (c *PlaylistController) writePage(w http.ResponseWriter, r *http.Request, op string, page domain.PlaylistPage, err error) {
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, op, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistPageToResponse(page))
}
