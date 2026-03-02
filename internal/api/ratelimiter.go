package api

import (
	"net/http"
	"sync"
	"time"
)

const (
	DefaultRequestsPerMinute = 60
	DefaultWindowSeconds     = 60
)

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	mu              sync.Mutex
	requests        map[string][]time.Time
	requestsPerMinute int
	windowSeconds   int
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = DefaultRequestsPerMinute
	}
	return &RateLimiter{
		requests:         make(map[string][]time.Time),
		requestsPerMinute: requestsPerMinute,
		windowSeconds:    DefaultWindowSeconds,
	}
}

// Allow checks if a request is allowed from the given key.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-time.Duration(rl.windowSeconds) * time.Second)

	requests, exists := rl.requests[key]
	if !exists {
		requests = []time.Time{}
	}

	filtered := []time.Time{}
	for _, t := range requests {
		if t.After(windowStart) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) >= rl.requestsPerMinute {
		rl.requests[key] = filtered
		return false
	}

	filtered = append(filtered, now)
	rl.requests[key] = filtered
	return true
}

// Reset resets the rate limiter for the given key.
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.requests, key)
}

// ResetAll resets all rate limiters.
func (rl *RateLimiter) ResetAll() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.requests = make(map[string][]time.Time)
}

// Middleware creates an HTTP middleware that applies rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if !rl.Allow(key) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			respondJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "Rate limit exceeded. Please try again later.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetRemaining returns the number of remaining requests for the given key.
func (rl *RateLimiter) GetRemaining(key string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-time.Duration(rl.windowSeconds) * time.Second)

	requests, exists := rl.requests[key]
	if !exists {
		return rl.requestsPerMinute
	}

	filtered := 0
	for _, t := range requests {
		if t.After(windowStart) {
			filtered++
		}
	}

	return rl.requestsPerMinute - filtered
}
