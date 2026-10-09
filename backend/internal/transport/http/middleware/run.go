package middleware

import (
	"net/http"
	"strings"

	"chimera/internal/runctx"
)

const (
	HeaderRunID     = "X-Run-Id"
	HeaderRunStep   = "X-Run-Step"
	HeaderRunEvent  = "X-Run-Event"
	HeaderRunResult = "X-Run-Result"
)

const maxRunHeader = 64

// RunTrace puts X-Run-Id and X-Run-Step into the request context for the
// logger. X-Run-Event start or finish logs the boundaries of a test run.
func RunTrace(log Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := runHeader(r, HeaderRunID)
			if id == "" {
				next.ServeHTTP(w, r)
				return
			}
			ctx := runctx.With(r.Context(), runctx.Run{ID: id, Step: runHeader(r, HeaderRunStep)})
			switch runHeader(r, HeaderRunEvent) {
			case "start":
				log.InfoContext(ctx, "test run started")
			case "finish":
				log.InfoContext(ctx, "test run finished", "result", runHeader(r, HeaderRunResult))
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func runHeader(r *http.Request, name string) string {
	v := strings.TrimSpace(r.Header.Get(name))
	if len(v) > maxRunHeader {
		return v[:maxRunHeader]
	}
	return v
}
