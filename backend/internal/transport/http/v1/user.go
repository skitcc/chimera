package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
	"chimera/internal/transport/http/middleware"
)

type UserController struct {
	users UserService
	log   Logger
}

func NewUserController(users UserService, log Logger) *UserController {
	return &UserController{users: users, log: log}
}

// ListUsers godoc
// @Summary List users
// @ID listUsers
// @Tags users
// @Produce json
// @Success 200 {array} UserResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/users [get]
func (c *UserController) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := c.users.List(r.Context())
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "list users", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, usersToResponse(users))
}

// GetUser godoc
// @Summary Get user by id
// @ID getUser
// @Tags users
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/users/{userId} [get]
func (c *UserController) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userId")
	user, err := c.users.GetByID(r.Context(), id)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "get user", err, "user_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, userToResponse(user))
}

// CreateUser godoc
// @Summary Create user
// @Tags users
// @Accept json
// @Produce json
// @Param body body UserWriteRequest true "User"
// @Success 201 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
func (c *UserController) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req UserWriteRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create user", err)
		return
	}

	user, err := c.users.Create(r.Context(), req.toDomain())
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create user", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, userToResponse(user))
}

// UpdateUser godoc
// @Summary Update user
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body UserWriteRequest true "User"
// @Success 200 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
func (c *UserController) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var req UserWriteRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "update user", err)
		return
	}

	actor, ok := actorID(w, r, c.log, "update user")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	user, err := c.users.Update(r.Context(), actor, id, req.toDomain())
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "update user", err, "user_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, userToResponse(user))
}

// DeleteUser godoc
// @Summary Delete user
// @Tags users
// @Param id path string true "User ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
func (c *UserController) DeleteUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "delete user")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := c.users.Delete(r.Context(), actor, id); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete user", err, "user_id", id)
		return
	}
	httpapi.WriteNoContent(w)
}

// Me godoc
// @Summary Current user
// @ID getCurrentUser
// @Tags auth
// @Produce json
// @Success 200 {object} UserResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me [get]
func (c *UserController) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		httpapi.WriteAppError(r.Context(), w, c.log, "me", domain.Unauthorized("missing token"))
		return
	}
	user, err := c.users.GetByID(r.Context(), id)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "me", err, "user_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, userToResponse(user))
}

// ReplaceMe godoc
// @Summary Replace current profile
// @ID replaceCurrentUser
// @Tags users
// @Accept json
// @Produce json
// @Param body body UserReplaceRequest true "Profile"
// @Success 200 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me [put]
func (c *UserController) ReplaceMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := actorID(w, r, c.log, "replace profile")
	if !ok {
		return
	}
	var req UserReplaceRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace profile", err)
		return
	}
	if req.Email == nil || req.Name == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace profile", domain.Invalid("email and name are required"))
		return
	}
	user, err := c.users.ReplaceCurrent(r.Context(), userID, domain.UserReplace{
		Email: *req.Email, Name: *req.Name,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace profile", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, userToResponse(user))
}

// PatchMe godoc
// @Summary Update current profile
// @ID updateCurrentUser
// @Tags users
// @Accept json
// @Produce json
// @Param body body UserPatchRequest true "Profile fields"
// @Success 200 {object} UserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me [patch]
func (c *UserController) PatchMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := actorID(w, r, c.log, "patch profile")
	if !ok {
		return
	}
	var req UserPatchRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch profile", err)
		return
	}
	user, err := c.users.PatchCurrent(r.Context(), userID, domain.UserPatch{
		Email: req.Email, Name: req.Name, Password: req.Password,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch profile", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, userToResponse(user))
}

// DeleteMe godoc
// @Summary Delete current account
// @ID deleteCurrentUser
// @Tags users
// @Success 204 {string} string "No Content"
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/me [delete]
func (c *UserController) DeleteMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := actorID(w, r, c.log, "delete profile")
	if !ok {
		return
	}
	if err := c.users.DeleteCurrent(r.Context(), userID); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete profile", err)
		return
	}
	httpapi.WriteNoContent(w)
}
