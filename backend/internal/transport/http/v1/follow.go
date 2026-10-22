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

// List godoc
// @Summary Подписки
// @Description Нужен хотя бы один фильтр, иначе 400.
// @Description
// @Description - `follower_id` — на кого подписан пользователь.
// @Description - `following_id` — кто подписан на пользователя.
// @Description - Оба сразу — есть ли подписка у этой пары: страница из одной записи или пустая.
// @Description
// @Description Если указанного пользователя нет — 404.
// @ID listFollows
// @Tags follows
// @Produce json
// @Param follower_id query string false "Только подписки этого пользователя" format(uuid)
// @Param following_id query string false "Только подписки на этого пользователя" format(uuid)
// @Param limit query int false "Размер страницы. По умолчанию 20, больше 100 не принимается." minimum(1) maximum(100) default(20)
// @Param cursor query string false "Курсор следующей страницы из прошлого ответа"
// @Success 200 {object} FollowPage "Страница подписок"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Router /v1/follows [get]
func (c *FollowController) List(w http.ResponseWriter, r *http.Request) {
	page, err := c.follows.List(r.Context(), domain.FollowListQuery{
		PageQuery:   httpapi.ParsePageQuery(r),
		FollowerID:  r.URL.Query().Get("follower_id"),
		FollowingID: r.URL.Query().Get("following_id"),
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "list follows", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, followPageToResponse(page))
}

// CreateMany godoc
// @Summary Подписаться на нескольких авторов
// @Description Создаёт подписки `follower_id` на всех из `following_ids`.
// @Description `follower_id` должен совпадать с владельцем токена, иначе 403.
// @Description
// @Description - Атомарно: если хоть одного автора нет — 404, ничего не меняется.
// @Description - Идемпотентно: уже существующие подписки пропускаются, повтор запроса снова 204.
// @Description - Пустой список, повтор внутри списка, `follower_id` в списке или больше 100 идентификаторов — 400.
// @Description
// @Description Для одного автора — `PUT /v1/follows/{followerId}/{followingId}`.
// @ID createFollows
// @Tags follows
// @Accept json
// @Produce json
// @Param body body FollowsCreate true "Подписчик и авторы"
// @Success 204 "Подписки на всех из списка есть"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/follows [post]
func (c *FollowController) CreateMany(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "create follows")
	if !ok {
		return
	}
	var req FollowsCreate
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create follows", err)
		return
	}
	in := domain.FollowBatch{FollowerID: req.FollowerID, FollowingIDs: req.FollowingIDs}
	if err := c.follows.CreateMany(r.Context(), actor, in); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create follows", err, "follower_id", in.FollowerID)
		return
	}
	httpapi.WriteNoContent(w)
}

// DeleteMany godoc
// @Summary Отписаться от нескольких авторов
// @Description Снимает подписки `follower_id` на всех из `following_id`.
// @Description `follower_id` должен совпадать с владельцем токена, иначе 403.
// @Description Список передаётся в query, а не в теле: тело у DELETE многие клиенты и прокси отбрасывают.
// @Description
// @Description - Атомарно и идемпотентно: отсутствующие подписки и несуществующие авторы пропускаются, повтор снова 204.
// @Description - Без `following_id` — 400: запрос без списка не снимает все подписки.
// @Description - Повтор внутри списка или больше 100 идентификаторов — 400.
// @Description
// @Description Для одного автора — `DELETE /v1/follows/{followerId}/{followingId}`.
// @ID deleteFollows
// @Tags follows
// @Produce json
// @Param follower_id query string true "Чьи подписки снимаются. Должен совпадать с владельцем токена." format(uuid)
// @Param following_id query []string true "От кого отписаться. Параметр повторяется на каждого автора — `?following_id=a&following_id=b`." collectionFormat(multi)
// @Success 204 "Подписок на авторов из списка нет"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/follows [delete]
func (c *FollowController) DeleteMany(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "delete follows")
	if !ok {
		return
	}
	in := domain.FollowBatch{
		FollowerID:   r.URL.Query().Get("follower_id"),
		FollowingIDs: r.URL.Query()["following_id"],
	}
	if err := c.follows.DeleteMany(r.Context(), actor, in); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete follows", err, "follower_id", in.FollowerID)
		return
	}
	httpapi.WriteNoContent(w)
}

// Get godoc
// @Summary Подписка пары
// @Description 200 — подписка есть, 404 — нет её или нет одного из пользователей.
// @ID getFollow
// @Tags follows
// @Produce json
// @Param followerId path string true "Кто подписывается" format(uuid)
// @Param followingId path string true "На кого подписываются" format(uuid)
// @Success 200 {object} Follow "Подписка"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Router /v1/follows/{followerId}/{followingId} [get]
func (c *FollowController) Get(w http.ResponseWriter, r *http.Request) {
	followerID, followingID := chi.URLParam(r, "followerId"), chi.URLParam(r, "followingId")
	follow, err := c.follows.Get(r.Context(), followerID, followingID)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "get follow", err,
			"follower_id", followerID, "following_id", followingID)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, followToResponse(follow))
}

// Put godoc
// @Summary Подписаться на автора
// @Description `followerId` должен совпадать с владельцем токена, иначе 403.
// @Description Идемпотентно: повтор той же пары снова 204. На себя подписаться нельзя — 400.
// @ID putFollow
// @Tags follows
// @Produce json
// @Param followerId path string true "Кто подписывается" format(uuid)
// @Param followingId path string true "На кого подписываются" format(uuid)
// @Success 204 "Подписка есть"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/follows/{followerId}/{followingId} [put]
func (c *FollowController) Put(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "put follow")
	if !ok {
		return
	}
	followerID, followingID := chi.URLParam(r, "followerId"), chi.URLParam(r, "followingId")
	if err := c.follows.Put(r.Context(), actor, followerID, followingID); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "put follow", err,
			"follower_id", followerID, "following_id", followingID)
		return
	}
	httpapi.WriteNoContent(w)
}

// Delete godoc
// @Summary Отписаться от автора
// @Description `followerId` должен совпадать с владельцем токена, иначе 403.
// @Description Идемпотентно: повторное снятие снова 204.
// @ID deleteFollow
// @Tags follows
// @Produce json
// @Param followerId path string true "Кто подписывается" format(uuid)
// @Param followingId path string true "На кого подписываются" format(uuid)
// @Success 204 "Подписки нет"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/follows/{followerId}/{followingId} [delete]
func (c *FollowController) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "delete follow")
	if !ok {
		return
	}
	followerID, followingID := chi.URLParam(r, "followerId"), chi.URLParam(r, "followingId")
	if err := c.follows.Delete(r.Context(), actor, followerID, followingID); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete follow", err,
			"follower_id", followerID, "following_id", followingID)
		return
	}
	httpapi.WriteNoContent(w)
}
