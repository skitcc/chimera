package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"chimera/gateway/internal/transport/http/middleware"
)

func TestProxiesClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cases := []struct {
		name    string
		trusted []netip.Prefix
		remote  string
		xff     []string
		want    string
	}{
		{"no trusted proxies ignores the header", nil, "10.0.0.5:4000", []string{"203.0.113.7"}, "10.0.0.5"},
		{"untrusted peer cannot spoof the header", trusted, "198.51.100.9:4000", []string{"203.0.113.7"}, "198.51.100.9"},
		{"trusted peer forwards the client", trusted, "10.0.0.5:4000", []string{"203.0.113.7"}, "203.0.113.7"},
		{"spoofed leftmost hop is skipped", trusted, "10.0.0.5:4000", []string{"1.1.1.1, 203.0.113.7"}, "203.0.113.7"},
		{"chained trusted proxies are skipped", trusted, "10.0.0.5:4000", []string{"203.0.113.7, 10.1.1.1"}, "203.0.113.7"},
		{"repeated headers are joined in order", trusted, "10.0.0.5:4000", []string{"203.0.113.7", "10.1.1.1"}, "203.0.113.7"},
		{"malformed hop stops at the last trusted address", trusted, "10.0.0.5:4000", []string{"garbage, 10.1.1.1"}, "10.1.1.1"},
		{"trusted peer without the header is the client", trusted, "10.0.0.5:4000", nil, "10.0.0.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil)
			req.RemoteAddr = tc.remote
			for _, v := range tc.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			if got := middleware.NewProxies(tc.trusted).ClientIP(req); got != tc.want {
				t.Fatalf("ClientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}
