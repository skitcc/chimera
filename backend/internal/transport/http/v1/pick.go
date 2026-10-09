package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
)

type PickController struct {
	picks PickService
	log   Logger
}

func NewPickController(picks PickService, log Logger) *PickController {
	return &PickController{picks: picks, log: log}
}

// List godoc
// @Summary List current user's generated picks
// @ID listMyPicks
// @Tags picks
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} PickPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me/picks [get]
func (c *PickController) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "list picks")
	if !ok {
		return
	}
	page, err := c.picks.List(r.Context(), actor, httpapi.ParsePageQuery(r))
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "list picks", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, pickPageToResponse(page))
}

// Generate godoc
// @Summary Generate a temporary random pick
// @ID generatePick
// @Description Persists up to 20 randomly selected ready tracks.
// @Tags picks
// @Produce json
// @Success 201 {object} PickDetailResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me/picks [post]
func (c *PickController) Generate(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "generate pick")
	if !ok {
		return
	}
	pick, err := c.picks.Generate(r.Context(), actor)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "generate pick", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, pickDetailToResponse(pick))
}

// Get godoc
// @Summary Get one generated pick
// @ID getMyPick
// @Tags picks
// @Produce json
// @Param pickId path string true "Pick ID"
// @Success 200 {object} PickDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me/picks/{pickId} [get]
func (c *PickController) Get(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "get pick")
	if !ok {
		return
	}
	id := chi.URLParam(r, "pickId")
	pick, err := c.picks.Get(r.Context(), actor, id)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "get pick", err, "pick_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, pickDetailToResponse(pick))
}

// Rename godoc
// @Summary Rename a generated pick
// @ID renamePick
// @Tags picks
// @Accept json
// @Produce json
// @Param pickId path string true "Pick ID"
// @Param body body PickPatchRequest true "Pick fields"
// @Success 200 {object} PickResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me/picks/{pickId} [patch]
func (c *PickController) Rename(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "rename pick")
	if !ok {
		return
	}
	var req PickPatchRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "rename pick", err)
		return
	}
	if req.Title == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "rename pick", domain.Invalid("title is required"))
		return
	}
	id := chi.URLParam(r, "pickId")
	pick, err := c.picks.Rename(r.Context(), actor, id, domain.PickPatch{Title: *req.Title})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "rename pick", err, "pick_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, pickToResponse(pick))
}

// Delete godoc
// @Summary Delete a generated pick
// @ID deletePick
// @Tags picks
// @Param pickId path string true "Pick ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me/picks/{pickId} [delete]
func (c *PickController) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "delete pick")
	if !ok {
		return
	}
	id := chi.URLParam(r, "pickId")
	if err := c.picks.Delete(r.Context(), actor, id); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete pick", err, "pick_id", id)
		return
	}
	httpapi.WriteNoContent(w)
}
