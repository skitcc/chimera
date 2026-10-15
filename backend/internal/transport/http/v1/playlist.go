package v1

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
	"chimera/internal/transport/http/middleware"
)

type PlaylistController struct {
	playlists PlaylistService
	log       Logger
}

func NewPlaylistController(playlists PlaylistService, log Logger) *PlaylistController {
	return &PlaylistController{playlists: playlists, log: log}
}

// List godoc
// @Summary Плейлисты
// @Description Одна коллекция для ленты и плейлистов пользователя, как `GET /v1/tracks`. Фильтры складываются через И.
// @Description
// @Description - Без фильтров — лента всех публичных плейлистов.
// @Description - `owner_id` — плейлисты пользователя. Если пользователя нет — 404.
// @Description - `is_public` — по умолчанию только публичные. Если `owner_id` совпадает с владельцем токена,
// @Description по умолчанию видны и скрытые. `is_public=false` для чужих плейлистов — 403.
// @Description
// @Description Если Authorization передан, токен должен быть верным.
// @ID listPlaylists
// @Tags playlists
// @Produce json
// @Param owner_id query string false "Владелец плейлистов" format(uuid)
// @Param is_public query bool false "Видимость плейлиста. `false` — только для своих плейлистов."
// @Param limit query int false "Размер страницы. По умолчанию 20, больше 100 не принимается." minimum(1) maximum(100) default(20)
// @Param cursor query string false "Курсор следующей страницы из прошлого ответа"
// @Success 200 {object} PlaylistPage "Страница плейлистов"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/playlists [get]
func (c *PlaylistController) List(w http.ResponseWriter, r *http.Request) {
	viewer, _ := middleware.UserIDFromCtx(r.Context())
	q := domain.PlaylistListQuery{
		PageQuery: httpapi.ParsePageQuery(r),
		ViewerID:  viewer,
		OwnerID:   r.URL.Query().Get("owner_id"),
	}
	if raw := r.URL.Query().Get("is_public"); raw != "" {
		isPublic, err := strconv.ParseBool(raw)
		if err != nil {
			httpapi.WriteAppError(r.Context(), w, c.log, "list playlists", domain.Invalid("invalid is_public"))
			return
		}
		q.IsPublic = &isPublic
	}
	page, err := c.playlists.List(r.Context(), q)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "list playlists", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistPageToResponse(page))
}

// Create godoc
// @Summary Создать плейлист
// @Description Владелец — владелец токена. Нужны название и флаг публичности.
// @Description Повтор запроса создаёт ещё один плейлист.
// @ID createPlaylist
// @Tags playlists
// @Accept json
// @Produce json
// @Param body body PlaylistReplace true "Название и видимость"
// @Success 201 {object} Playlist "Плейлист создан"
// @Header 201 {string} Location "Адрес созданного ресурса"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/playlists [post]
func (c *PlaylistController) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "create playlist")
	if !ok {
		return
	}
	in, ok := c.replaceInput(w, r, "create playlist")
	if !ok {
		return
	}
	playlist, err := c.playlists.Create(r.Context(), actor, in)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create playlist", err)
		return
	}
	writeCreated(w, "/v1/playlists/"+playlist.ID, playlistToResponse(playlist))
}

// Get godoc
// @Summary Плейлист
// @Description Публичный читается без токена. Скрытый виден только владельцу.
// @Description Чужому скрытому списку отвечаем как отсутствующему.
// @Description Если Authorization передан, токен должен быть верным.
// @ID getPlaylist
// @Tags playlists
// @Produce json
// @Param playlistId path string true "Идентификатор плейлиста" format(uuid)
// @Success 200 {object} Playlist "Плейлист"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/playlists/{playlistId} [get]
func (c *PlaylistController) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "playlistId")
	viewer, _ := middleware.UserIDFromCtx(r.Context())
	playlist, err := c.playlists.Get(r.Context(), id, viewer)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "get playlist", err, "playlist_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistToResponse(playlist))
}

// Replace godoc
// @Summary Заменить название и видимость
// @Description Оба поля обязательны. Повтор с тем же телом не меняет результат. Только владелец.
// @ID replacePlaylist
// @Tags playlists
// @Accept json
// @Produce json
// @Param playlistId path string true "Идентификатор плейлиста" format(uuid)
// @Param body body PlaylistReplace true "Название и видимость"
// @Success 200 {object} Playlist "Плейлист заменён"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/playlists/{playlistId} [put]
func (c *PlaylistController) Replace(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "replace playlist")
	if !ok {
		return
	}
	in, ok := c.replaceInput(w, r, "replace playlist")
	if !ok {
		return
	}
	id := chi.URLParam(r, "playlistId")
	playlist, err := c.playlists.Replace(r.Context(), actor, id, in)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace playlist", err, "playlist_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistToResponse(playlist))
}

// Patch godoc
// @Summary Частично изменить плейлист
// @Description Меняется только переданное поле. Пустое тело не принимается. Только владелец.
// @ID updatePlaylist
// @Tags playlists
// @Accept json
// @Produce json
// @Param playlistId path string true "Идентификатор плейлиста" format(uuid)
// @Param body body PlaylistPatch true "Поля плейлиста"
// @Success 200 {object} Playlist "Плейлист изменён"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/playlists/{playlistId} [patch]
func (c *PlaylistController) Patch(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "patch playlist")
	if !ok {
		return
	}
	var req PlaylistPatch
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch playlist", err)
		return
	}
	id := chi.URLParam(r, "playlistId")
	playlist, err := c.playlists.Patch(r.Context(), actor, id, domain.PlaylistPatch{
		Title: req.Title, IsPublic: req.IsPublic,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch playlist", err, "playlist_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistToResponse(playlist))
}

// Delete godoc
// @Summary Удалить плейлист
// @Description Только владелец. Повтор по уже удалённому плейлисту — 404.
// @ID deletePlaylist
// @Tags playlists
// @Produce json
// @Param playlistId path string true "Идентификатор плейлиста" format(uuid)
// @Success 204 "Плейлист удалён"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/playlists/{playlistId} [delete]
func (c *PlaylistController) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "delete playlist")
	if !ok {
		return
	}
	id := chi.URLParam(r, "playlistId")
	if err := c.playlists.Delete(r.Context(), actor, id); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete playlist", err, "playlist_id", id)
		return
	}
	httpapi.WriteNoContent(w)
}

// ListTracks godoc
// @Summary Треки плейлиста
// @Description Только готовые треки, в порядке `position`. Доступ как у самого плейлиста.
// @ID listPlaylistTracks
// @Tags playlists
// @Produce json
// @Param playlistId path string true "Идентификатор плейлиста" format(uuid)
// @Param limit query int false "Размер страницы. По умолчанию 20, больше 100 не принимается." minimum(1) maximum(100) default(20)
// @Param cursor query string false "Курсор следующей страницы из прошлого ответа"
// @Success 200 {object} PlaylistTrackPage "Страница состава"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/playlists/{playlistId}/tracks [get]
func (c *PlaylistController) ListTracks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "playlistId")
	viewer, _ := middleware.UserIDFromCtx(r.Context())
	page, err := c.playlists.ListTracks(r.Context(), id, viewer, httpapi.ParsePageQuery(r))
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "list playlist tracks", err, "playlist_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistTrackPageToResponse(page))
}

// AddTrack godoc
// @Summary Добавить трек в плейлист
// @Description Только готовый трек. Повтор пары — конфликт. Только владелец плейлиста.
// @ID createPlaylistTrack
// @Tags playlists
// @Accept json
// @Produce json
// @Param playlistId path string true "Идентификатор плейлиста" format(uuid)
// @Param body body PlaylistTrackCreate true "Трек и позиция"
// @Success 201 {object} PlaylistTrack "Трек добавлен"
// @Header 201 {string} Location "Адрес созданного ресурса"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 409 {object} ErrorResponse "Повтор уникальной пары, занятый email или неверный статус трека"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/playlists/{playlistId}/tracks [post]
func (c *PlaylistController) AddTrack(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "add playlist track")
	if !ok {
		return
	}
	var req PlaylistTrackCreate
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "add playlist track", err)
		return
	}
	if req.Position == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "add playlist track", domain.Invalid("position is required"))
		return
	}
	id := chi.URLParam(r, "playlistId")
	item, err := c.playlists.AddTrack(r.Context(), actor, id, domain.PlaylistTrackAdd{
		TrackID: req.TrackID, Position: *req.Position,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "add playlist track", err, "playlist_id", id)
		return
	}
	writeCreated(w, "/v1/playlists/"+id+"/tracks/"+item.Track.ID, playlistTrackToResponse(item))
}

// MoveTrack godoc
// @Summary Сменить позицию трека
// @Description Меняется только `position`. Повтор с тем же значением снова 200. Только владелец плейлиста.
// @ID updatePlaylistTrack
// @Tags playlists
// @Accept json
// @Produce json
// @Param playlistId path string true "Идентификатор плейлиста" format(uuid)
// @Param trackId path string true "Идентификатор трека" format(uuid)
// @Param body body PlaylistTrackPatch true "Новая позиция"
// @Success 200 {object} PlaylistTrack "Позиция изменена"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/playlists/{playlistId}/tracks/{trackId} [patch]
func (c *PlaylistController) MoveTrack(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "move playlist track")
	if !ok {
		return
	}
	var req PlaylistTrackPatch
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "move playlist track", err)
		return
	}
	if req.Position == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "move playlist track", domain.Invalid("position is required"))
		return
	}
	id := chi.URLParam(r, "playlistId")
	trackID := chi.URLParam(r, "trackId")
	item, err := c.playlists.MoveTrack(r.Context(), actor, id, trackID, domain.PlaylistTrackMove{
		Position: *req.Position,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "move playlist track", err, "playlist_id", id, "track_id", trackID)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, playlistTrackToResponse(item))
}

// RemoveTrack godoc
// @Summary Убрать трек из плейлиста
// @Description Сам плейлист остаётся. Только владелец плейлиста.
// @ID deletePlaylistTrack
// @Tags playlists
// @Produce json
// @Param playlistId path string true "Идентификатор плейлиста" format(uuid)
// @Param trackId path string true "Идентификатор трека" format(uuid)
// @Success 204 "Трек убран из списка"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/playlists/{playlistId}/tracks/{trackId} [delete]
func (c *PlaylistController) RemoveTrack(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "remove playlist track")
	if !ok {
		return
	}
	id := chi.URLParam(r, "playlistId")
	trackID := chi.URLParam(r, "trackId")
	if err := c.playlists.RemoveTrack(r.Context(), actor, id, trackID); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "remove playlist track", err, "playlist_id", id, "track_id", trackID)
		return
	}
	httpapi.WriteNoContent(w)
}

func (c *PlaylistController) replaceInput(w http.ResponseWriter, r *http.Request, op string) (domain.PlaylistReplace, bool) {
	var req PlaylistReplace
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, op, err)
		return domain.PlaylistReplace{}, false
	}
	if req.Title == nil || req.IsPublic == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, op, domain.Invalid("title and is_public are required"))
		return domain.PlaylistReplace{}, false
	}
	return domain.PlaylistReplace{Title: *req.Title, IsPublic: *req.IsPublic}, true
}
