package v1

import (
	"net/http"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
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
// @Description Открывает сессию и возвращает токен и `user_id` владельца.
// @Description Неизвестный email и неверный пароль дают один и тот же `unauthorized`.
// @ID createSession
// @Tags sessions
// @Accept json
// @Produce json
// @Param body body SessionCreate true "Email и пароль"
// @Success 201 {object} Session "Сессия открыта"
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
	httpapi.WriteJSON(w, http.StatusCreated, sessionToResponse(result))
}
