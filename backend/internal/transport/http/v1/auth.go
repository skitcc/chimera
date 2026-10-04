package v1

import (
	"net/http"

	httpapi "chimera/internal/transport/http"
)

type AuthController struct {
	auth AuthService
	log  Logger
}

func NewAuthController(auth AuthService, log Logger) *AuthController {
	return &AuthController{auth: auth, log: log}
}

// Register godoc
// @Summary Register user
// @ID register
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Register"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/auth/register [post]
func (c *AuthController) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "register", err)
		return
	}

	result, err := c.auth.Register(r.Context(), req.toDomain())
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "register", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, authToResponse(result))
}

// Login godoc
// @Summary Login
// @ID login
// @Tags auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Login"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/auth/login [post]
func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "login", err)
		return
	}

	result, err := c.auth.Login(r.Context(), req.toDomain())
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "login", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, authToResponse(result))
}
