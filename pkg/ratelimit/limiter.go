package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

type LimiterStore struct {
	limiters map[string]*rate.Limiter
	mu       sync.Mutex
	r        rate.Limit
	burst    int
}

func NewLimiterStore(r rate.Limit, burst int) *LimiterStore {
	return &LimiterStore{
		limiters: make(map[string]*rate.Limiter),
		r:        r,
		burst:    burst,
	}
}

func (s *LimiterStore) GetLimiter(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limiter, exists := s.limiters[key]; exists {
		return limiter
	}
	limiter := rate.NewLimiter(s.r, s.burst)
	s.limiters[key] = limiter
	return limiter
}
