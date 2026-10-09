package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
)

type FollowController struct {
	follows FollowService
	log     Logger
}

func NewFollowController(follows FollowService, log Logger) *FollowController {
	return &FollowController{follows: follows, log: log}
}

// Follow godoc
// @Summary Follow a user
// @ID followUser
// @Tags follows
// @Param userId path string true "User ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/users/{userId}/follow [put]
func (c *FollowController) Follow(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "follow user")
	if !ok {
		return
	}
	userID := chi.URLParam(r, "userId")
	if err := c.follows.Follow(r.Context(), actor, userID); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "follow user", err, "user_id", userID)
		return
	}
	httpapi.WriteNoContent(w)
}

// Unfollow godoc
// @Summary Unfollow a user
// @ID unfollowUser
// @Tags follows
// @Param userId path string true "User ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/users/{userId}/follow [delete]
func (c *FollowController) Unfollow(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "unfollow user")
	if !ok {
		return
	}
	userID := chi.URLParam(r, "userId")
	if err := c.follows.Unfollow(r.Context(), actor, userID); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "unfollow user", err, "user_id", userID)
		return
	}
	httpapi.WriteNoContent(w)
}

// ListFollowers godoc
// @Summary List user followers
// @ID listFollowers
// @Tags follows
// @Produce json
// @Param userId path string true "User ID"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} UserPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /v1/users/{userId}/followers [get]
func (c *FollowController) ListFollowers(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	page, err := c.follows.ListFollowers(r.Context(), userID, httpapi.ParsePageQuery(r))
	c.writePage(w, r, "list followers", page, err)
}

// ListUserFollowing godoc
// @Summary List users followed by a user
// @ID listUserFollowing
// @Tags follows
// @Produce json
// @Param userId path string true "User ID"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} UserPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /v1/users/{userId}/following [get]
func (c *FollowController) ListUserFollowing(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	page, err := c.follows.ListFollowing(r.Context(), userID, httpapi.ParsePageQuery(r))
	c.writePage(w, r, "list user following", page, err)
}

// ListMyFollowing godoc
// @Summary List users followed by the current user
// @ID listMyFollowing
// @Tags follows
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} UserPageResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me/following [get]
func (c *FollowController) ListMyFollowing(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "list my following")
	if !ok {
		return
	}
	page, err := c.follows.ListFollowing(r.Context(), actor, httpapi.ParsePageQuery(r))
	c.writePage(w, r, "list my following", page, err)
}

func (c *FollowController) writePage(w http.ResponseWriter, r *http.Request, op string, page domain.UserPage, err error) {
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, op, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, userPageToResponse(page))
}
