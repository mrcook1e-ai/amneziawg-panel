package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// bucketLimiter is a tiny fixed-window token bucket keyed by an arbitrary
// string (IP for login throttling, cabinet token for cabinet write throttling,
// etc). Stale buckets are swept opportunistically once the map grows past 1024
// entries so memory stays bounded under abusive traffic.
type bucketLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int
	window  time.Duration
}

type bucket struct {
	tokens int
	reset  time.Time
}

func newBucketLimiter(rate int, window time.Duration) *bucketLimiter {
	return &bucketLimiter{
		buckets: map[string]*bucket{},
		rate:    rate,
		window:  window,
	}
}

func (l *bucketLimiter) allow(key string) bool {
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
	b, ok := l.buckets[key]
	if !ok || now.After(b.reset) {
		l.buckets[key] = &bucket{tokens: l.rate - 1, reset: now.Add(l.window)}
		return true
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

// loginLimiter — admin password attempts, 5/min per IP. Tight enough to make
// brute force impractical against a single-admin panel.
func newLoginLimiter() *bucketLimiter { return newBucketLimiter(5, time.Minute) }

// cabinetLimiter — subscriber-driven device creation. A leaked token would
// otherwise let an attacker drain the port pool (10 ifaces) and key generator
// in seconds. 10 device creations / minute per token is generous for humans.
func newCabinetLimiter() *bucketLimiter { return newBucketLimiter(10, time.Minute) }

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
