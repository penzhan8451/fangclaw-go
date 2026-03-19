// Package a2a provides Agent-to-Agent protocol support.
package a2a

import (
	"sync"
	"time"
)

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	mu           sync.Mutex
	tokens       float64
	capacity     float64
	refillRate   float64
	lastRefill   time.Time
}

// NewRateLimiter creates a new rate limiter.
// capacity: maximum number of tokens
// refillRate: tokens per second to refill
func NewRateLimiter(capacity, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.capacity {
		rl.tokens = rl.capacity
	}
	rl.lastRefill = now

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// A2ARateLimiter manages rate limits for A2A operations.
type A2ARateLimiter struct {
	mu            sync.RWMutex
	globalLimiter *RateLimiter
	agentLimiters map[string]*RateLimiter
}

// NewA2ARateLimiter creates a new A2A rate limiter.
func NewA2ARateLimiter(globalCapacity, globalRefillRate float64) *A2ARateLimiter {
	return &A2ARateLimiter{
		globalLimiter: NewRateLimiter(globalCapacity, globalRefillRate),
		agentLimiters: make(map[string]*RateLimiter),
	}
}

// SetAgentRateLimit sets a rate limit for a specific agent.
func (arl *A2ARateLimiter) SetAgentRateLimit(agentID string, capacity, refillRate float64) {
	arl.mu.Lock()
	defer arl.mu.Unlock()
	arl.agentLimiters[agentID] = NewRateLimiter(capacity, refillRate)
}

// Allow checks if a request from an agent is allowed.
func (arl *A2ARateLimiter) Allow(agentID string) bool {
	if !arl.globalLimiter.Allow() {
		return false
	}

	arl.mu.RLock()
	agentLimiter, ok := arl.agentLimiters[agentID]
	arl.mu.RUnlock()

	if !ok {
		return true
	}

	return agentLimiter.Allow()
}
