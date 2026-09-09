package controllers

import (
	"net/http"

	"chimera/internal/business_logic/port"
)

type AuthController struct {
	auth port.AuthService
	log  port.Logger
}

func NewAuthController(auth port.AuthService, log port.Logger) *AuthController {
	return &AuthController{auth: auth, log: log}
}

// Register godoc
// @Summary Register user
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Register"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/auth/register [post]
func (c *AuthController) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAppError(w, c.log, "register", err)
		return
	}

	result, err := c.auth.Register(r.Context(), req.toDomain())
	if err != nil {
		writeAppError(w, c.log, "register", err)
		return
	}
	writeJSON(w, http.StatusCreated, authToResponse(result))
}

// Login godoc
// @Summary Login
// @Tags auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Login"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/auth/login [post]
func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAppError(w, c.log, "login", err)
		return
	}

	result, err := c.auth.Login(r.Context(), req.toDomain())
	if err != nil {
		writeAppError(w, c.log, "login", err)
		return
	}
	writeJSON(w, http.StatusOK, authToResponse(result))
}
