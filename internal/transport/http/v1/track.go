package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
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
// @Param limit query int false "Page size" default(20)
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} TrackPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/tracks [get]
func (c *TrackController) ListTracks(w http.ResponseWriter, r *http.Request) {
	page, err := c.tracks.List(r.Context(), httpapi.ParsePageQuery(r))
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "list tracks", err)
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
	track, err := c.tracks.GetByID(r.Context(), id)
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

	id := chi.URLParam(r, "id")
	track, err := c.tracks.Update(r.Context(), id, req.toDomain())
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
	id := chi.URLParam(r, "id")
	if err := c.tracks.Delete(r.Context(), id); err != nil {
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
