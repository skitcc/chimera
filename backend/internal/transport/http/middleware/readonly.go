package middleware

import (
	"net/http"

	"chimera/internal/domain"
)

func ReadOnly(log Logger, writeErr ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
			default:
				writeErr(r.Context(), w, log, "read only", domain.Forbidden("read only"))
			}
		})
	}
}
