package v1

import (
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
	"chimera/internal/transport/http/middleware"
)

type TrackController struct {
	tracks     TrackService
	log        Logger
	streamHost string
}

func NewTrackController(tracks TrackService, streamHost string, log Logger) *TrackController {
	return &TrackController{tracks: tracks, streamHost: streamHost, log: log}
}

// List godoc
// @Summary Треки
// @Description Одна коллекция для ленты, каталога автора, своих загрузок и любимого. Фильтры складываются через И.
// @Description
// @Description - `user_id` — треки автора.
// @Description - `artist` — точное имя без учёта регистра и краевых пробелов.
// @Description - `liked_by` — любимое пользователя. Только свой `user_id`: без токена 401, чужой — 403.
// @Description - `status` — по умолчанию только `ready`. Если `user_id` совпадает с владельцем токена,
// @Description по умолчанию видны все статусы. Статус кроме `ready` для чужих треков — 403.
// @Description
// @Description Если Authorization передан, токен должен быть верным.
// @ID listTracks
// @Tags tracks,likes
// @Produce json
// @Param user_id query string false "Автор треков" format(uuid)
// @Param artist query string false "Точное имя артиста"
// @Param liked_by query string false "Только треки, отмеченные любимыми этим пользователем. Должен совпадать с владельцем токена." format(uuid)
// @Param status query string false "Статус трека. Кроме `ready` — только для своих треков." Enums(pending, processing, ready)
// @Param limit query int false "Размер страницы. По умолчанию 20, больше 100 не принимается." minimum(1) maximum(100) default(20)
// @Param cursor query string false "Курсор следующей страницы из прошлого ответа"
// @Success 200 {object} TrackPage "Страница треков"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/tracks [get]
func (c *TrackController) List(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := middleware.UserIDFromCtx(r.Context())
	q := r.URL.Query()
	page, err := c.tracks.Search(r.Context(), domain.TrackListQuery{
		PageQuery: httpapi.ParsePageQuery(r),
		ViewerID:  viewerID,
		UserID:    q.Get("user_id"),
		Artist:    q.Get("artist"),
		LikedBy:   q.Get("liked_by"),
		Status:    domain.TrackStatus(q.Get("status")),
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "list tracks", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, trackPageToResponse(page))
}

// Create godoc
// @Summary Создать трек
// @Description Создаёт трек `pending` у владельца токена и временную ссылку на запись объекта.
// @Description Байты файла в этот запрос не входят. Публикация — `PATCH /v1/tracks/{trackId}` со `status: ready`.
// @ID createTrack
// @Tags tracks
// @Accept json
// @Produce json
// @Param body body TrackCreate true "Метаданные трека"
// @Success 201 {object} TrackUpload "Карточка создана"
// @Header 201 {string} Location "Адрес созданного ресурса"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/tracks [post]
func (c *TrackController) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := actorID(w, r, c.log, "create track")
	if !ok {
		return
	}
	var req TrackCreate
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create track", err)
		return
	}
	if req.Artist == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create track", domain.Invalid("artist is required"))
		return
	}
	session, err := c.tracks.InitUpload(r.Context(), req.toDomain(userID))
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "create track", err)
		return
	}
	writeCreated(w, "/v1/tracks/"+session.Track.ID, uploadSessionToResponse(session))
}

// Get godoc
// @Summary Карточка трека
// @Description Без токена виден только `ready`. Владелец с верным токеном видит свой трек в любом статусе.
// @Description Если заголовок Authorization передан, токен должен быть верным.
// @ID getTrack
// @Tags tracks
// @Produce json
// @Param trackId path string true "Идентификатор трека" format(uuid)
// @Success 200 {object} Track "Карточка"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/tracks/{trackId} [get]
func (c *TrackController) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "trackId")
	viewerID, _ := middleware.UserIDFromCtx(r.Context())
	track, err := c.tracks.GetByID(r.Context(), id, viewerID)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "get track", err, "track_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, trackToResponse(track))
}

// Replace godoc
// @Summary Заменить название и артиста
// @Description Оба поля обязательны. Статус, размер и файл не меняются. Только владелец.
// @ID replaceTrack
// @Tags tracks
// @Accept json
// @Produce json
// @Param trackId path string true "Идентификатор трека" format(uuid)
// @Param body body TrackReplace true "Название и артист"
// @Success 200 {object} Track "Карточка заменена"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/tracks/{trackId} [put]
func (c *TrackController) Replace(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "replace track")
	if !ok {
		return
	}
	var req TrackReplace
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace track", err)
		return
	}
	if req.Title == nil || req.Artist == nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace track", domain.Invalid("title and artist are required"))
		return
	}
	id := chi.URLParam(r, "trackId")
	track, err := c.tracks.ReplaceOwned(r.Context(), actor, id, domain.TrackReplace{
		Title: *req.Title, Artist: *req.Artist,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "replace track", err, "track_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, trackToResponse(track))
}

// Patch godoc
// @Summary Частично изменить трек или опубликовать его
// @Description Меняются только переданные поля. Пустое тело не принимается. Только владелец.
// @Description
// @Description `status: ready` подтверждает загрузку: сервер сверяет владельца и размер объекта
// @Description в хранилище и публикует трек. Если объекта нет, размер не совпал или текущий статус
// @Description не допускает перехода — 409.
// @ID updateTrack
// @Tags tracks
// @Accept json
// @Produce json
// @Param trackId path string true "Идентификатор трека" format(uuid)
// @Param body body TrackPatch true "Поля трека или публикация"
// @Success 200 {object} Track "Карточка изменена"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 409 {object} ErrorResponse "Повтор уникальной пары, занятый email или неверный статус трека"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/tracks/{trackId} [patch]
func (c *TrackController) Patch(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "patch track")
	if !ok {
		return
	}
	var req TrackPatch
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch track", err)
		return
	}
	if req.Status != nil && domain.TrackStatus(*req.Status) != domain.TrackReady {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch track", domain.Invalid("status must be ready"))
		return
	}
	id := chi.URLParam(r, "trackId")
	track, err := c.tracks.PatchOwned(r.Context(), actor, id, domain.TrackPatch{
		Title: req.Title, Artist: req.Artist, Publish: req.Status != nil,
	})
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "patch track", err, "track_id", id)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, trackToResponse(track))
}

// Delete godoc
// @Summary Удалить трек
// @Description Только владелец. Строки плейлистов и подборок с этим треком уходят, сами списки остаются.
// @ID deleteTrack
// @Tags tracks
// @Produce json
// @Param trackId path string true "Идентификатор трека" format(uuid)
// @Success 204 "Трек удалён"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 401 {object} ErrorResponse "Токена нет или он неверен. При входе сюда же попадает неверный пароль."
// @Failure 403 {object} ErrorResponse "Токен верный, но ресурс принадлежит другому пользователю"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Security bearerAuth
// @Router /v1/tracks/{trackId} [delete]
func (c *TrackController) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r, c.log, "delete track")
	if !ok {
		return
	}
	id := chi.URLParam(r, "trackId")
	if err := c.tracks.DeleteOwned(r.Context(), actor, id); err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "delete track", err, "track_id", id)
		return
	}
	httpapi.WriteNoContent(w)
}

// Stream godoc
// @Summary Аудио трека
// @Description Ответ — переход на временную ссылку чтения. Неготовый трек не отдаётся.
// @ID getTrackStream
// @Tags tracks
// @Produce json
// @Param trackId path string true "Идентификатор трека" format(uuid)
// @Success 302 "Временная ссылка в Location"
// @Header 302 {string} Location "Временная ссылка на объект"
// @Failure 400 {object} ErrorResponse "Неверное тело, параметр или пустое частичное обновление"
// @Failure 404 {object} ErrorResponse "Ресурс не найден или скрыт от этого клиента"
// @Failure 409 {object} ErrorResponse "Повтор уникальной пары, занятый email или неверный статус трека"
// @Failure 500 {object} ErrorResponse "Сбой сервера. Клиенту не отдаются детали."
// @Router /v1/tracks/{trackId}/stream [get]
func (c *TrackController) Stream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "trackId")
	raw, err := c.tracks.StreamURL(r.Context(), id)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "stream track", err, "track_id", id)
		return
	}
	location, err := streamLocation(raw, c.streamHost)
	if err != nil {
		httpapi.WriteAppError(r.Context(), w, c.log, "stream track", err, "track_id", id)
		return
	}
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusFound)
}

func streamLocation(raw, allowedHost string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || allowedHost == "" || u.Host != allowedHost || u.User != nil {
		return "", domain.Internal("stream url is invalid")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", domain.Internal("stream url is invalid")
	}
	u.Host = allowedHost
	return u.String(), nil
}
