package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"chimera/gateway/internal/domain"
	httpapi "chimera/gateway/internal/transport/http"
)

type discardLog struct{}

func (discardLog) InfoContext(context.Context, string, ...any)  {}
func (discardLog) ErrorContext(context.Context, string, ...any) {}

func TestWriteAppError(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{"not found", domain.NotFound("track not found"), http.StatusNotFound, "not_found", "track not found"},
		{"invalid", domain.Invalid("invalid id"), http.StatusBadRequest, "invalid", "invalid id"},
		{"unauthorized", domain.Unauthorized("missing token"), http.StatusUnauthorized, "unauthorized", "missing token"},
		{"forbidden", domain.Forbidden("not allowed"), http.StatusForbidden, "forbidden", "not allowed"},
		{"conflict", domain.Conflict("track is not ready"), http.StatusConflict, "conflict", "track is not ready"},
		{"too many requests", domain.TooManyRequests("slow down"), http.StatusTooManyRequests, "too_many_requests", "slow down"},
		{"service down", domain.Wrap(domain.CodeUnavailable, "service unavailable", errors.New("dial")), http.StatusServiceUnavailable, "unavailable", "service unavailable"},
		{"plain error hides details", errors.New("boom"), http.StatusInternalServerError, "internal", "internal error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			httpapi.WriteAppError(context.Background(), rec, discardLog{}, "op", tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			var body httpapi.ErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Code != tc.wantCode || body.Message != tc.wantMessage {
				t.Fatalf("body = %+v, want {%s %s}", body, tc.wantCode, tc.wantMessage)
			}
		})
	}
}
