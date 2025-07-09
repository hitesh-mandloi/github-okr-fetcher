package github

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ClientStats tracks API usage statistics
type ClientStats struct {
	APICallsCount  int
	CacheHitsCount int
	ErrorsCount    int
	RetryCount     int
	RateLimitHits  int
	ProcessingTime time.Duration
	LastAPICall    time.Time
	RemainingQuota int
	QuotaResetTime time.Time
	mu             sync.RWMutex
}

// APICache provides simple in-memory caching for API responses
type APICache struct {
	data map[string]CacheEntry
	mu   sync.RWMutex
}

// CacheEntry represents a cached API response
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// NewAPICache creates a new API cache
func NewAPICache() *APICache {
	return &APICache{
		data: make(map[string]CacheEntry),
	}
}

// GetStats returns a copy of the current client statistics
func (s *ClientStats) GetStats() ClientStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s
}

// IncrementAPICall safely increments the API call counter
func (s *ClientStats) IncrementAPICall() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.APICallsCount++
	s.LastAPICall = time.Now()
}

// IncrementError safely increments the error counter
func (s *ClientStats) IncrementError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ErrorsCount++
}

// IncrementRetry safely increments the retry counter
func (s *ClientStats) IncrementRetry() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RetryCount++
}

// IncrementCacheHit safely increments the cache hit counter
func (s *ClientStats) IncrementCacheHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CacheHitsCount++
}

// IncrementRateLimitHit safely increments the rate limit hit counter
func (s *ClientStats) IncrementRateLimitHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RateLimitHits++
}

// UpdateQuota updates the rate limit quota information
func (s *ClientStats) UpdateQuota(remaining int, resetTime time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RemainingQuota = remaining
	s.QuotaResetTime = resetTime
}

// GetFromCache retrieves an item from the cache
func (c *APICache) GetFromCache(key string) (interface{}, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.data[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		// Entry expired, remove it
		delete(c.data, key)
		return nil, false
	}

	return entry.Data, true
}

// SetCache stores an item in the cache with TTL
func (c *APICache) SetCache(key string, data interface{}, ttl time.Duration) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// ClearExpired removes expired entries from the cache
func (c *APICache) ClearExpired() {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.data {
		if now.After(entry.ExpiresAt) {
			delete(c.data, key)
		}
	}
}

// RateLimiter wraps the rate limiter with additional functionality
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerHour int) *RateLimiter {
	if requestsPerHour <= 0 {
		requestsPerHour = 5000 // Default GitHub rate limit
	}

	// Convert to requests per second with burst capacity
	rps := rate.Limit(float64(requestsPerHour) / 3600)
	limiter := rate.NewLimiter(rps, 10)

	return &RateLimiter{
		limiter: limiter,
	}
}

// Wait waits for the rate limiter to allow the request
func (r *RateLimiter) Wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}

// Simple hash function for cache keys
func Hash(s string) uint32 {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}
