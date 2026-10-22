package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
)

type UserController struct {
	users UserService
	log   Logger
}

func NewUserController(users UserService, log Logger) *UserController {
	return &UserController{users: users, log: log}
}

// List godoc
// @Summary Пользователи
// @Description Весь справочник по страницам. Подписчики и подписки — в `GET /v1/follows`.
// @ID listUsers
// @Tags users
// @Produce json
// @Param limit query int false "Размер страницы. По умолчанию 20, больше 100 не принимается." minimum(1) maximum(100) default(20)
// @Param cursor query string false "Курсор следующей страницы из прошлого ответа"
// @Success 200 {object} UserPage "Страница профилей"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Router /v1/users [get]
func (c *UserController) List(w http.ResponseWriter, r *http.Request) {
	page, err := c.users.List(r.Context(), httpapi.ParsePageQuery(r))
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "list users", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, userPageToResponse(page))
}

// Create godoc
// @Summary Регистрация
// @Description Создаёт аккаунт. Пароль не короче 8 символов.
// @Description Токен здесь не выдаётся: после регистрации клиент открывает сессию через `POST /v1/sessions`.
// @ID createUser
// @Tags users
// @Accept json
// @Produce json
// @Param body body UserCreate true "Аккаунт"
// @Success 201 {object} User "Аккаунт создан"
// @Header 201 {string} Location "Адрес созданного ресурса"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 409 {object} ErrorResponse "Повтор уникальной пары, занятый email или неверный статус трека"
// @Failure 429 {object} ErrorResponse "Слишком частые регистрация или вход. Ответ не говорит, какая квота сработала."
// @Header 429 {integer} Retry-After "Секунды до конца окна"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Router /v1/users [post]
func (c *UserController) Create(w http.ResponseWriter, r *http.Request) {
	var req UserCreate
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create user", err)
		return
	}
	user, err := c.users.Create(r.Context(), domain.UserWrite{Email: req.Email, Name: req.Name, Password: req.Password})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create user", err)
		return
	}
	writeCreated(w, "/v1/users/"+user.ID, userToResponse(user))
}

// Get godoc
// @Summary Профиль
// @Description Публичный профиль. Пароль в ответ не входит.
// @ID getUser
// @Tags users
// @Produce json
// @Param userId path string true "Идентификатор пользователя" format(uuid)
// @Success 200 {object} User "Профиль"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Router /v1/users/{userId} [get]
func (c *UserController) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userId")
	user, err := c.users.GetByID(r.Context(), id)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "get user", err, "user_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, userToResponse(user))
}

// Replace godoc
// @Summary Заменить профиль
// @Description Нужны и имя, и email. Пароль этой операцией не меняется. Только владелец аккаунта.
// @ID replaceUser
// @Tags users
// @Accept json
// @Produce json
// @Param userId path string true "Идентификатор пользователя" format(uuid)
// @Param body body UserReplace true "Профиль"
// @Success 200 {object} User "Профиль заменён"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 409 {object} ErrorResponse "Повтор уникальной пары, занятый email или неверный статус трека"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/users/{userId} [put]
func (c *UserController) Replace(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "replace user")
	if !ok {
		return
	}
	var req UserReplace
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace user", err)
		return
	}
	if req.Email == nil || req.Name == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace user", domain.Invalid("email and name are required"))
		return
	}
	id := chi.URLParam(r, "userId")
	user, err := c.users.Replace(r.Context(), actor, id, domain.UserReplace{Email: *req.Email, Name: *req.Name})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace user", err, "user_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, userToResponse(user))
}

// Patch godoc
// @Summary Частично изменить профиль
// @Description Меняются только переданные поля. Пустое тело не принимается.
// @Description Пароль меняется только здесь. Только владелец аккаунта.
// @ID updateUser
// @Tags users
// @Accept json
// @Produce json
// @Param userId path string true "Идентификатор пользователя" format(uuid)
// @Param body body UserPatch true "Поля профиля"
// @Success 200 {object} User "Профиль изменён"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 409 {object} ErrorResponse "Повтор уникальной пары, занятый email или неверный статус трека"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/users/{userId} [patch]
func (c *UserController) Patch(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "patch user")
	if !ok {
		return
	}
	var req UserPatch
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch user", err)
		return
	}
	id := chi.URLParam(r, "userId")
	user, err := c.users.Patch(r.Context(), actor, id, domain.UserPatch{
		Email: req.Email, Name: req.Name, Password: req.Password,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch user", err, "user_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, userToResponse(user))
}

// Delete godoc
// @Summary Удалить аккаунт
// @Description Только владелец аккаунта. Повтор по уже удалённому аккаунту — 401, сессии больше нет.
// @ID deleteUser
// @Tags users
// @Produce json
// @Param userId path string true "Идентификатор пользователя" format(uuid)
// @Success 204 "Аккаунт удалён"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/users/{userId} [delete]
func (c *UserController) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "delete user")
	if !ok {
		return
	}
	id := chi.URLParam(r, "userId")
	if err := c.users.Delete(r.Context(), actor, id); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete user", err, "user_id", id)
		return
	}
	httpapi.WriteNoContent(w)
}
