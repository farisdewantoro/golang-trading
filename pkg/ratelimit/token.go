package ratelimit

import (
	"context"
	"sync"
	"time"
)

type TokenLimiter struct {
	sync.Mutex
	capacity     int           // max token per minute
	remaining    int           // sisa token saat ini
	refillPeriod time.Duration // biasanya 1 menit
	lastRefill   time.Time
}

func NewTokenLimiter(tokensPerMinute int) *TokenLimiter {
	return &TokenLimiter{
		capacity:     tokensPerMinute,
		remaining:    tokensPerMinute,
		refillPeriod: time.Minute,
		lastRefill:   time.Now(),
	}
}

func (l *TokenLimiter) Wait(ctx context.Context, tokens int) error {
	for {
		l.refill()

		l.Lock()
		if l.remaining >= tokens {
			l.remaining -= tokens
			l.Unlock()
			return nil
		}
		l.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Retry sebentar lagi
		}
	}
}

func (l *TokenLimiter) refill() {
	l.Lock()
	defer l.Unlock()

	now := time.Now()
	if now.Sub(l.lastRefill) >= l.refillPeriod {
		l.remaining = l.capacity
		l.lastRefill = now
	}
}

func (l *TokenLimiter) GetRemaining() int {
	l.Lock()
	defer l.Unlock()
	return l.remaining
}
