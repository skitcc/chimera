package middleware

import (
	"crypto/rand"
	"net/http"

	"chimera/platform/idempotency"
)

const HeaderIdempotencyKey = "Idempotency-Key"

// Idempotency gives every request a key: the client's Idempotency-Key, or a fresh
// one. The key goes with each create call, so a gRPC retry after a lost reply
// returns the resource the first attempt created. A client that repeats a request
// itself sends the same Idempotency-Key.
func Idempotency(log Logger, writeErr ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(HeaderIdempotencyKey)
			if key == "" {
				key = rand.Text()
			}
			ctx, err := idempotency.With(r.Context(), key)
			if err != nil {
				writeErr(r.Context(), w, log, "idempotency key", err)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
