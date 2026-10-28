package httpapi

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type Limiter struct {
	enabled bool
	mu      sync.Mutex
	entries map[string]*limiterEntry
	calls   int
}

func newLimiter(enabled bool) *Limiter {
	return &Limiter{enabled: enabled, entries: make(map[string]*limiterEntry)}
}

func (l *Limiter) Allow(key string, limit rate.Limit, burst int) bool {
	if !l.enabled {
		return true
	}
	if limit <= 0 {
		return true
	}
	if burst <= 0 {
		burst = 1
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, ok := l.entries[key]
	if !ok || now.Sub(entry.lastSeen) > 10*time.Minute {
		entry = &limiterEntry{limiter: rate.NewLimiter(limit, burst)}
		l.entries[key] = entry
	}
	entry.lastSeen = now

	l.calls++
	if l.calls%1024 == 0 || len(l.entries) > 10000 {
		for candidate, value := range l.entries {
			if now.Sub(value.lastSeen) > 10*time.Minute {
				delete(l.entries, candidate)
			}
		}
	}

	return entry.limiter.Allow()
}
