package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

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

// CreateTrack godoc
// @Summary Create track
// @Tags tracks
// @Accept json
// @Produce json
// @Param body body TrackWriteRequest true "Track"
// @Success 201 {object} TrackResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/tracks [post]
func (c *TrackController) CreateTrack(w http.ResponseWriter, r *http.Request) {
	var req TrackWriteRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create track", err)
		return
	}

	track, err := c.tracks.Create(r.Context(), req.toDomain())
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create track", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, trackToResponse(track))
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
// @Router /v1/tracks/{id} [delete]
func (c *TrackController) DeleteTrack(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := c.tracks.Delete(r.Context(), id); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete track", err, "track_id", id)
		return
	}
	httpapi.WriteNoContent(w)
}
