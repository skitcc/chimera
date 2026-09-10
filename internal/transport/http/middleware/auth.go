package middleware

import (
	"context"
	"net/http"
	"strings"

	"chimera/internal/domain"
)

type ctxKey int

const userIDKey ctxKey = 1

type TokenParser interface {
	Parse(token string) (string, error)
}

type ErrorWriter func(ctx context.Context, w http.ResponseWriter, log Logger, msg string, err error, attrs ...any)

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func UserIDFromCtx(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok && id != ""
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}

func RequireAuth(log Logger, tokens TokenParser, writeErr ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r)
			if raw == "" {
				writeErr(r.Context(), w, log, "auth", domain.Unauthorized("missing token"))
				return
			}
			id, err := tokens.Parse(raw)
			if err != nil {
				writeErr(r.Context(), w, log, "auth", err)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), id)))
		})
	}
}
