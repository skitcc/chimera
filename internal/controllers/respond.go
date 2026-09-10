package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"chimera/internal/business_logic/apperrors"
	"chimera/internal/business_logic/domain"
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

func writeAppError(ctx context.Context, w http.ResponseWriter, log Logger, msg string, err error, attrs ...any) {
	app, ok := apperrors.As(err)
	if !ok {
		log.ErrorContext(ctx, msg, append(attrs, "error", err)...)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Code:    string(apperrors.CodeInternal),
			Message: "internal error",
		})
		return
	}

	status := httpStatus(app.Code)
	args := append(attrs, "error", err, "code", app.Code)
	if status >= 500 {
		log.ErrorContext(ctx, msg, args...)
	} else {
		log.InfoContext(ctx, msg, args...)
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
	q := domain.PageQuery{Cursor: r.URL.Query().Get("cursor")}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil {
			q.Limit = n
		}
	}
	return q
}
