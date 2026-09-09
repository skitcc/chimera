package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"chimera/internal/business_logic/apperrors"
	"chimera/internal/business_logic/domain"
	"chimera/internal/business_logic/port"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return apperrors.Wrap(apperrors.CodeInvalid, "invalid json", err)
	}
	return nil
}

func writeAppError(w http.ResponseWriter, log port.Logger, msg string, err error) {
	app, ok := apperrors.As(err)
	if !ok {
		log.Error(msg, "err", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Code:    string(apperrors.CodeInternal),
			Message: "internal error",
		})
		return
	}

	status := httpStatus(app.Code)
	if status >= 500 {
		log.Error(msg, "err", err, "code", app.Code)
	} else {
		log.Info(msg, "err", err, "code", app.Code)
	}

	writeJSON(w, status, ErrorResponse{
		Code:    string(app.Code),
		Message: app.Message,
	})
}

func httpStatus(code apperrors.Code) int {
	switch code {
	case apperrors.CodeNotFound:
		return http.StatusNotFound
	case apperrors.CodeInvalid:
		return http.StatusBadRequest
	case apperrors.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperrors.CodeConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func parsePageQuery(r *http.Request) domain.PageQuery {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	return domain.PageQuery{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	}
}
