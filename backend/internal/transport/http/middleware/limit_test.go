package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chimera/internal/domain"
	httpapi "chimera/internal/transport/http"
	"chimera/internal/transport/http/middleware"
)

type discardLog struct{}

func (discardLog) InfoContext(context.Context, string, ...any)  {}
func (discardLog) ErrorContext(context.Context, string, ...any) {}

type stubTokens struct {
	id  string
	err error
}

func (s stubTokens) Parse(string) (string, error) {
	return s.id, s.err
}

func TestOptionalAuth(t *testing.T) {
	t.Run("continues without a token", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := middleware.UserIDFromCtx(r.Context()); ok {
				t.Fatal("guest request has a user id")
			}
			w.WriteHeader(http.StatusNoContent)
		})
		h := middleware.OptionalAuth(discardLog{}, stubTokens{}, httpapi.WriteAppError)(next)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tracks/1", nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("rejects an invalid token", func(t *testing.T) {
		called := false
		next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
		tokens := stubTokens{err: domain.Unauthorized("invalid token")}
		h := middleware.OptionalAuth(discardLog{}, tokens, httpapi.WriteAppError)(next)
		req := httptest.NewRequest(http.MethodGet, "/v1/tracks/1", nil)
		req.Header.Set("Authorization", "Bearer bad")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if called {
			t.Fatal("handler ran after an invalid token")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("accepts a valid token", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := middleware.UserIDFromCtx(r.Context())
			if !ok || id != "user-1" {
				t.Fatalf("user id = %q ok=%v", id, ok)
			}
			w.WriteHeader(http.StatusNoContent)
		})
		h := middleware.OptionalAuth(discardLog{}, stubTokens{id: "user-1"}, httpapi.WriteAppError)(next)
		req := httptest.NewRequest(http.MethodGet, "/v1/tracks/1", nil)
		req.Header.Set("Authorization", "Bearer good")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestRateLimit(t *testing.T) {
	limit := middleware.NewLimiter(1, time.Minute)
	var calls int
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})
	h := middleware.RateLimit(limit, discardLog{}, httpapi.WriteAppError)(next)

	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d", first.Code)
	}

	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d", calls)
	}
}
