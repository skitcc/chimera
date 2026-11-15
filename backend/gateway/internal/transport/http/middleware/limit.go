package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"chimera/gateway/internal/domain"
)

type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{
		hits:   map[string][]time.Time{},
		limit:  limit,
		window: window,
	}
}

func (l *Limiter) Allow(now time.Time, key string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, hit := range l.hits[key] {
		if hit.After(cutoff) {
			kept = append(kept, hit)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		wait := int(kept[0].Add(l.window).Sub(now) / time.Second)
		if wait < 1 {
			wait = 1
		}
		return false, wait
	}
	if len(kept) == 0 {
		delete(l.hits, key)
		kept = nil
	}
	l.hits[key] = append(kept, now)
	return true, 0
}

// Proxies resolves the client address behind trusted reverse proxies such as
// the ingress controller. X-Forwarded-For is read only when the direct peer is
// trusted, so a client cannot pick its own rate-limit key.
type Proxies struct {
	trusted []netip.Prefix
}

func NewProxies(trusted []netip.Prefix) Proxies {
	return Proxies{trusted: trusted}
}

// ClientIP walks X-Forwarded-For from the nearest hop and returns the first
// address that is not a trusted proxy.
func (p Proxies) ClientIP(r *http.Request) string {
	peer := remoteHost(r)
	addr, err := netip.ParseAddr(peer)
	if err != nil || !p.isTrusted(addr) {
		return peer
	}
	hops := forwardedFor(r)
	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(hops[i])
		if err != nil {
			return addr.String()
		}
		addr = hop.Unmap()
		if !p.isTrusted(addr) {
			return addr.String()
		}
	}
	return addr.String()
}

func (p Proxies) isTrusted(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range p.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func forwardedFor(r *http.Request) []string {
	var hops []string
	for _, header := range r.Header.Values("X-Forwarded-For") {
		for _, hop := range strings.Split(header, ",") {
			if hop = strings.TrimSpace(hop); hop != "" {
				hops = append(hops, hop)
			}
		}
	}
	return hops
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func RateLimit(limit *Limiter, proxies Proxies, log Logger, writeErr ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, retryAfter := limit.Allow(time.Now(), proxies.ClientIP(r))
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				writeErr(r.Context(), w, log, "auth rate limit", domain.TooManyRequests("slow down"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
