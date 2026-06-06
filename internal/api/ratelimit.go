package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// loginLimiter is a very small per-IP token bucket. Five attempts per minute is
// enough room for a typo-prone admin and tight enough to make password brute
// force impractical against a single-admin panel.
type loginLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int           // attempts per window
	window  time.Duration
}

type bucket struct {
	tokens int
	reset  time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{
		buckets: map[string]*bucket{},
		rate:    5,
		window:  time.Minute,
	}
}

// allow returns true if this IP may make another login attempt right now.
// We sweep stale buckets opportunistically to keep the map bounded.
func (l *loginLimiter) allow(ip string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 1024 {
		for k, b := range l.buckets {
			if now.After(b.reset) {
				delete(l.buckets, k)
			}
		}
	}
	b, ok := l.buckets[ip]
	if !ok || now.After(b.reset) {
		l.buckets[ip] = &bucket{tokens: l.rate - 1, reset: now.Add(l.window)}
		return true
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
