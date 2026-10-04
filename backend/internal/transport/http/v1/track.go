package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
	"chimera/internal/transport/http/middleware"
)

type TrackController struct {
	tracks TrackService
	log    Logger
}

func NewTrackController(tracks TrackService, log Logger) *TrackController {
	return &TrackController{tracks: tracks, log: log}
}

// ListTracks godoc
// @Summary Track feed
// @Description Lists ready tracks only.
// @Tags tracks
// @Produce json
// @Param artist query string false "Exact artist credit (case-insensitive)"
// @Param limit query int false "Page size" default(20)
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} TrackPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/tracks [get]
func (c *TrackController) ListTracks(w http.ResponseWriter, r *http.Request) {
	page, err := c.tracks.List(r.Context(), domain.TrackFeedQuery{
		PageQuery: httpapi.ParsePageQuery(r),
		Artist:    r.URL.Query().Get("artist"),
	})
	c.writeTrackPage(w, r, "list tracks", page, err)
}

// ListMyTracks godoc
// @Summary My uploaded tracks
// @Description All tracks uploaded by the current user, including drafts.
// @Tags tracks
// @Produce json
// @Param limit query int false "Page size" default(20)
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} TrackPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me/tracks [get]
func (c *TrackController) ListMyTracks(w http.ResponseWriter, r *http.Request) {
	userID, ok := actorID(w, r, c.log, "list my tracks")
	if !ok {
		return
	}
	page, err := c.tracks.ListByUploader(r.Context(), domain.TrackOwnerQuery{
		PageQuery: httpapi.ParsePageQuery(r),
		UserID:    userID,
	})
	c.writeTrackPage(w, r, "list my tracks", page, err)
}

// ListUploaderTracks godoc
// @Summary Tracks uploaded by a user
// @Description Ready tracks uploaded by the given user.
// @Tags tracks
// @Produce json
// @Param id path string true "User ID"
// @Param limit query int false "Page size" default(20)
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} TrackPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/users/{id}/tracks [get]
func (c *TrackController) ListUploaderTracks(w http.ResponseWriter, r *http.Request) {
	page, err := c.tracks.ListByUploader(r.Context(), domain.TrackOwnerQuery{
		PageQuery: httpapi.ParsePageQuery(r),
		UserID:    chi.URLParam(r, "id"),
		Status:    domain.TrackReady,
	})
	c.writeTrackPage(w, r, "list uploader tracks", page, err)
}

// ListLikedTracks godoc
// @Summary My liked tracks
// @Tags tracks
// @Produce json
// @Param limit query int false "Page size" default(20)
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} TrackPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me/likes [get]
func (c *TrackController) ListLikedTracks(w http.ResponseWriter, r *http.Request) {
	userID, ok := actorID(w, r, c.log, "list liked tracks")
	if !ok {
		return
	}
	page, err := c.tracks.ListLiked(r.Context(), domain.TrackLikeListQuery{
		PageQuery: httpapi.ParsePageQuery(r),
		UserID:    userID,
	})
	c.writeTrackPage(w, r, "list liked tracks", page, err)
}

// LikeTrack godoc
// @Summary Like a ready track
// @Tags tracks
// @Param id path string true "Track ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/tracks/{id}/like [post]
func (c *TrackController) LikeTrack(w http.ResponseWriter, r *http.Request) {
	in, ok := c.likeInput(w, r, "like track")
	if !ok {
		return
	}
	if err := c.tracks.Like(r.Context(), in); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "like track", err, "track_id", in.TrackID)
		return
	}
	httpapi.WriteNoContent(w)
}

// UnlikeTrack godoc
// @Summary Remove a like
// @Tags tracks
// @Param id path string true "Track ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/tracks/{id}/like [delete]
func (c *TrackController) UnlikeTrack(w http.ResponseWriter, r *http.Request) {
	in, ok := c.likeInput(w, r, "unlike track")
	if !ok {
		return
	}
	if err := c.tracks.Unlike(r.Context(), in); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "unlike track", err, "track_id", in.TrackID)
		return
	}
	httpapi.WriteNoContent(w)
}

func (c *TrackController) likeInput(w http.ResponseWriter, r *http.Request, op string) (domain.TrackLike, bool) {
	userID, ok := actorID(w, r, c.log, op)
	if !ok {
		return domain.TrackLike{}, false
	}
	return domain.TrackLike{UserID: userID, TrackID: chi.URLParam(r, "id")}, true
}

func (c *TrackController) writeTrackPage(w http.ResponseWriter, r *http.Request, op string, page domain.TrackPage, err error) {
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, op, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, trackPageToResponse(page))
}

// GetTrack godoc
// @Summary Get track by id
// @Tags tracks
// @Produce json
// @Param id path string true "Track ID"
// @Success 200 {object} TrackResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/tracks/{id} [get]
func (c *TrackController) GetTrack(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	viewerID, _ := middleware.UserIDFromCtx(r.Context())
	track, err := c.tracks.GetByID(r.Context(), id, viewerID)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "get track", err, "track_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, trackToResponse(track))
}

// StreamTrack godoc
// @Summary Stream a ready track
// @Tags tracks
// @Param id path string true "Track ID"
// @Success 302 {string} string "Redirect to audio"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/tracks/{id}/stream [get]
func (c *TrackController) StreamTrack(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	url, err := c.tracks.StreamURL(r.Context(), id)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "stream track", err, "track_id", id)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// UpdateTrack godoc
// @Summary Update track
// @Tags tracks
// @Accept json
// @Produce json
// @Param id path string true "Track ID"
// @Param body body TrackWriteRequest true "Track"
// @Success 200 {object} TrackResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/tracks/{id} [put]
func (c *TrackController) UpdateTrack(w http.ResponseWriter, r *http.Request) {
	var req TrackWriteRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "update track", err)
		return
	}

	actor, ok := actorID(w, r, c.log, "update track")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	track, err := c.tracks.Update(r.Context(), actor, id, req.toDomain())
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "update track", err, "track_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, trackToResponse(track))
}

// DeleteTrack godoc
// @Summary Delete track
// @Tags tracks
// @Param id path string true "Track ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/tracks/{id} [delete]
func (c *TrackController) DeleteTrack(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "delete track")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := c.tracks.Delete(r.Context(), actor, id); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete track", err, "track_id", id)
		return
	}
	httpapi.WriteNoContent(w)
}

// InitUpload godoc
// @Summary Start track upload
// @Tags tracks
// @Accept json
// @Produce json
// @Param body body TrackUploadInitRequest true "Track metadata"
// @Success 201 {object} TrackUploadInitResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/tracks/upload-init [post]
func (c *TrackController) InitUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := actorID(w, r, c.log, "init upload")
	if !ok {
		return
	}

	var req TrackUploadInitRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "init upload", err)
		return
	}

	session, err := c.tracks.InitUpload(r.Context(), req.toDomain(userID))
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "init upload", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, uploadSessionToResponse(session))
}

// CompleteUpload godoc
// @Summary Finish track upload
// @Description Checks the object in MinIO and publishes the track as ready.
// @Tags tracks
// @Produce json
// @Param id path string true "Track ID"
// @Success 200 {object} TrackResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/tracks/{id}/upload-complete [post]
func (c *TrackController) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := actorID(w, r, c.log, "complete upload")
	if !ok {
		return
	}

	id := chi.URLParam(r, "id")
	track, err := c.tracks.CompleteUpload(r.Context(), domain.TrackUploadComplete{
		TrackID: id,
		UserID:  userID,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "complete upload", err, "track_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, trackToResponse(track))
}
