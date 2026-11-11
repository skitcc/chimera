package middleware

import "net/http"

func Instance(name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if name != "" {
				w.Header().Set("X-Chimera-Instance", name)
			}
			next.ServeHTTP(w, r)
		})
	}
}
