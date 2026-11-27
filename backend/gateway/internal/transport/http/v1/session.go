package v1

import (
	"context"
	"net/http"

	"chimera/gateway/internal/domain"
	httpapi "chimera/gateway/internal/transport/http"
)

type SessionController struct {
	sessions SessionService
	log      Logger
}

func NewSessionController(sessions SessionService, log Logger) *SessionController {
	return &SessionController{sessions: sessions, log: log}
}

// Create godoc
// @Summary Вход
// @Description Проверяет пароль и отправляет email-код. JWT выдаётся после /sessions/verify.
// @Description Неизвестный email и неверный пароль дают один и тот же `unauthorized`.
// @ID createSession
// @Tags sessions
// @Accept json
// @Produce json
// @Param body body SessionCreate true "Email и пароль"
// @Success 202 {object} AuthChallenge "Код отправлен"
// @Success 201 {object} Session "Только perf-режим"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 429 {object} ErrorResponse "Слишком частые регистрация или вход. Ответ не говорит, какая квота сработала."
// @Header 429 {integer} Retry-After "Секунды до конца окна"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Router /v1/sessions [post]
func (c *SessionController) Create(w http.ResponseWriter, r *http.Request) {
	var req SessionCreate
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create session", err)
		return
	}
	result, err := c.sessions.Login(r.Context(), domain.LoginInput{Email: req.Email, Password: req.Password})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create session", err)
		return
	}
	writeAuthResult(w, result)
}

type AuthChallenge struct {
	ChallengeID string `json:"challenge_id"`
	ExpiresAt   int64  `json:"expires_at"`
}
type AuthVerify struct {
	Email       string `json:"email,omitempty"`
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password,omitempty"`
}
type RecoveryAccepted struct {
	Message string `json:"message"`
}

type RecoveryStart struct {
	Email string `json:"email"`
}
type PasswordChangeStart struct {
	CurrentPassword string `json:"current_password"`
}
type securitySessions interface {
	Verify(ctx context.Context, id, purpose, code, password, actor, email string) (domain.AuthResult, error)
	Recover(context.Context, string) (domain.AuthResult, error)
	Change(context.Context, string, string) (domain.AuthResult, error)
}

func writeAuthResult(w http.ResponseWriter, res domain.AuthResult) {
	if res.ChallengeID != "" {
		httpapi.WriteJSON(w, http.StatusAccepted, AuthChallenge{res.ChallengeID, res.ExpiresAt})
		return
	}
	if res.Token != "" {
		httpapi.WriteJSON(w, http.StatusCreated, sessionToResponse(res))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (c *SessionController) security() (securitySessions, error) {
	s, ok := c.sessions.(securitySessions)
	if !ok {
		return nil, domain.Unavailable("authentication unavailable")
	}
	return s, nil
}
func (c *SessionController) verify(w http.ResponseWriter, r *http.Request, purpose string) {
	var req AuthVerify
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "verify", err)
		return
	}
	actor := ""
	if purpose == "change" {
		var ok bool
		actor, ok = actorID(w, r, c.log, "change password")
		if !ok {
			return
		}
	}
	s, err := c.security()
	var res domain.AuthResult
	if err == nil {
		res, err = s.Verify(r.Context(), req.ChallengeID, purpose, req.Code, req.NewPassword, actor, req.Email)
	}
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "verify", err)
		return
	}
	writeAuthResult(w, res)
}

// Verify godoc
// @Summary Подтвердить второй фактор
// @Tags sessions
// @Accept json
// @Produce json
// @Param body body AuthVerify true "Проверка и код"
// @Success 201 {object} Session
// @Failure 401 {object} ErrorResponse
// @Router /v1/sessions/verify [post]
func (c *SessionController) Verify(w http.ResponseWriter, r *http.Request) { c.verify(w, r, "login") }

// Recover godoc
// @Summary Запросить восстановление доступа
// @Tags sessions
// @Accept json
// @Produce json
// @Param body body RecoveryStart true "Email"
// @Success 202 {object} RecoveryAccepted
// @Router /v1/password-recovery [post]
func (c *SessionController) Recover(w http.ResponseWriter, r *http.Request) {
	var req RecoveryStart
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "recover", err)
		return
	}
	s, err := c.security()

	if err == nil {
		_, err = s.Recover(r.Context(), req.Email)
	}
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "recover", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusAccepted, RecoveryAccepted{Message: "If the account exists, a recovery code has been sent."})
}

// RecoverVerify godoc
// @Summary Подтвердить восстановление; затем войти заново с 2FA
// @Tags sessions
// @Accept json
// @Param body body AuthVerify true "Код и новый пароль"
// @Success 204
// @Failure 401 {object} ErrorResponse
// @Router /v1/password-recovery/verify [post]
func (c *SessionController) RecoverVerify(w http.ResponseWriter, r *http.Request) {
	c.verify(w, r, "recovery")
}

// Change godoc
// @Summary Запросить смену пароля
// @Tags sessions
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param body body PasswordChangeStart true "Текущий пароль"
// @Success 202 {object} AuthChallenge
// @Failure 401 {object} ErrorResponse
// @Router /v1/password-change [post]
func (c *SessionController) Change(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "change password")
	if !ok {
		return
	}
	var req PasswordChangeStart
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "change password", err)
		return
	}
	s, err := c.security()
	var res domain.AuthResult
	if err == nil {
		res, err = s.Change(r.Context(), actor, req.CurrentPassword)
	}
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "change password", err)
		return
	}
	writeAuthResult(w, res)
}

// ChangeVerify godoc
// @Summary Подтвердить смену пароля и отозвать все сессии
// @Tags sessions
// @Security bearerAuth
// @Accept json
// @Param body body AuthVerify true "Код и новый пароль"
// @Success 204
// @Failure 401 {object} ErrorResponse
// @Router /v1/password-change/verify [post]
func (c *SessionController) ChangeVerify(w http.ResponseWriter, r *http.Request) {
	c.verify(w, r, "change")
}

// Current godoc
// @Summary Проверить действительность текущей сессии
// @Tags sessions
// @Security bearerAuth
// @Success 204
// @Failure 401 {object} ErrorResponse
// @Router /v1/session [get]
func (c *SessionController) Current(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
