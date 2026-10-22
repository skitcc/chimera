package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"chimera/internal/domain"
	"chimera/internal/transport/http/middleware"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func DecodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return domain.Wrap(domain.CodeInvalid, "invalid json", err)
	}
	return nil
}

func WriteAppError(ctx context.Context, w http.ResponseWriter, log middleware.Logger, msg string, err error, attrs ...any) {
	app, ok := domain.As(err)
	if !ok {
		log.ErrorContext(ctx, msg, append(attrs, "error", err)...)
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{
			Code:    string(domain.CodeInternal),
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

	WriteJSON(w, status, ErrorResponse{
		Code:    string(app.Code),
		Message: app.Message,
	})
}

func httpStatus(code domain.Code) int {
	switch code {
	case domain.CodeNotFound:
		return http.StatusNotFound
	case domain.CodeInvalid:
		return http.StatusBadRequest
	case domain.CodeUnauthorized:
		return http.StatusUnauthorized
	case domain.CodeForbidden:
		return http.StatusForbidden
	case domain.CodeConflict:
		return http.StatusConflict
	case domain.CodeTooManyRequests:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

func ParsePageQuery(r *http.Request) domain.PageQuery {
	limit := 0
	invalidLimit := false
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil && n > 0 {
			limit = n
		} else {
			invalidLimit = true
		}
	}
	return domain.NewPageQuery(limit, r.URL.Query().Get("cursor"), invalidLimit)
}
