package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpapi "chimera/internal/transport/http"
	"chimera/internal/transport/http/middleware"
)

func TestReadOnly(t *testing.T) {
	var calls int
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})
	h := middleware.ReadOnly(discardLog{}, httpapi.WriteAppError)(next)

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		calls = 0
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/v1/tracks", nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d", method, rec.Code)
		}
		if calls != 1 {
			t.Fatalf("%s handler calls = %d", method, calls)
		}
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		calls = 0
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/v1/tracks", nil))
		if calls != 0 {
			t.Fatalf("%s reached the handler", method)
		}
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s status = %d", method, rec.Code)
		}
		body, err := io.ReadAll(rec.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"code":"forbidden"`) || !strings.Contains(string(body), `"message":"read only"`) {
			t.Fatalf("%s body = %s", method, body)
		}
	}
}
