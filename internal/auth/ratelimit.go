package auth

import (
	"sync"
	"time"
)

// LoginRateLimiter tracks failed login attempts per IP using a sliding window.
// In-memory only — fine for a single-instance deployment, swap for Redis if
// the API ever runs on more than one node.
type LoginRateLimiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu       sync.Mutex
	attempts map[string][]time.Time
}

// NewLoginRateLimiter returns a limiter that allows up to max failures per ip
// in any rolling window of the given duration.
func NewLoginRateLimiter(max int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{
		max:      max,
		window:   window,
		now:      time.Now,
		attempts: make(map[string][]time.Time),
	}
}

// Allow reports whether ip is currently allowed to attempt a login. It does
// not record an attempt — call RecordFailure after a failed login.
func (l *LoginRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(ip)) < l.max
}

// RecordFailure appends a failure timestamp for ip.
func (l *LoginRateLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	pruned := l.prune(ip)
	l.attempts[ip] = append(pruned, l.now())
}

// Reset clears the failure record for ip — call on successful login.
func (l *LoginRateLimiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

// prune drops timestamps older than the window and returns the surviving slice.
// Caller must hold l.mu.
func (l *LoginRateLimiter) prune(ip string) []time.Time {
	cutoff := l.now().Add(-l.window)
	in := l.attempts[ip]
	kept := in[:0]
	for _, t := range in {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.attempts, ip)
		return nil
	}
	l.attempts[ip] = kept
	return kept
}
