package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"chimera/internal/domain"
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

func RateLimit(limit *Limiter, log Logger, writeErr ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, retryAfter := limit.Allow(time.Now(), clientIP(r))
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				writeErr(r.Context(), w, log, "auth rate limit", domain.TooManyRequests("slow down"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
