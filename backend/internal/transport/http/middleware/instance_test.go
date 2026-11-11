package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"chimera/internal/transport/http/middleware"
)

func TestInstance(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	named := middleware.Instance("api-read-1")(next)
	rec := httptest.NewRecorder()
	named.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tracks", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("X-Chimera-Instance"); got != "api-read-1" {
		t.Fatalf("instance = %q", got)
	}

	blank := middleware.Instance("")(next)
	rec = httptest.NewRecorder()
	blank.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tracks", nil))
	if got := rec.Header().Get("X-Chimera-Instance"); got != "" {
		t.Fatalf("empty instance = %q", got)
	}
}
