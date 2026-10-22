package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
)

// PutLike godoc
// @Summary Отметить трек любимым
// @Description `userId` должен совпадать с владельцем токена. Только готовый трек.
// @Description Идемпотентно: повтор той же пары снова 204.
// @ID putLike
// @Tags likes
// @Produce json
// @Param userId path string true "Идентификатор пользователя" format(uuid)
// @Param trackId path string true "Идентификатор трека" format(uuid)
// @Success 204 "Лайк есть"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 409 {object} ErrorResponse "Повтор уникальной пары, занятый email или неверный статус трека"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/users/{userId}/likes/{trackId} [put]
func (c *TrackController) PutLike(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "put like")
	if !ok {
		return
	}
	in := domain.TrackLike{UserID: chi.URLParam(r, "userId"), TrackID: chi.URLParam(r, "trackId")}
	if err := c.tracks.PutLike(r.Context(), actor, in); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "put like", err, "user_id", in.UserID, "track_id", in.TrackID)
		return
	}
	httpapi.WriteNoContent(w)
}

// DeleteLike godoc
// @Summary Снять отметку
// @Description `userId` должен совпадать с владельцем токена. Повторное снятие тоже 204.
// @Description 404 только если нет самого трека.
// @ID deleteLike
// @Tags likes
// @Produce json
// @Param userId path string true "Идентификатор пользователя" format(uuid)
// @Param trackId path string true "Идентификатор трека" format(uuid)
// @Success 204 "Лайка нет"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/users/{userId}/likes/{trackId} [delete]
func (c *TrackController) DeleteLike(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "delete like")
	if !ok {
		return
	}
	in := domain.TrackLike{UserID: chi.URLParam(r, "userId"), TrackID: chi.URLParam(r, "trackId")}
	if err := c.tracks.DeleteLike(r.Context(), actor, in); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete like", err, "user_id", in.UserID, "track_id", in.TrackID)
		return
	}
	httpapi.WriteNoContent(w)
}
