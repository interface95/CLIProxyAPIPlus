package auth

import (
	"sync"
	"sync/atomic"
)

type ConcurrencyLimiter struct {
	mu     sync.Mutex
	counts map[string]*atomic.Int64
}

func NewConcurrencyLimiter() *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		counts: make(map[string]*atomic.Int64),
	}
}

func (l *ConcurrencyLimiter) Acquire(authID string, limit int) bool {
	if l == nil {
		return true
	}
	if limit == 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	counter := l.counts[authID]
	if counter == nil {
		counter = &atomic.Int64{}
		l.counts[authID] = counter
	}

	current := counter.Load()
	if current < 0 {
		counter.Store(0)
		current = 0
	}
	if current >= int64(limit) {
		return false
	}

	counter.Add(1)
	return true
}

func (l *ConcurrencyLimiter) Release(authID string) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	counter := l.counts[authID]
	if counter == nil {
		return
	}
	if next := counter.Add(-1); next < 0 {
		counter.Store(0)
	}
}

func (l *ConcurrencyLimiter) Count(authID string) int {
	if l == nil {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	counter := l.counts[authID]
	if counter == nil {
		return 0
	}
	current := counter.Load()
	if current < 0 {
		counter.Store(0)
		current = 0
	}
	return int(current)
}

func (l *ConcurrencyLimiter) Stats() map[string]int {
	if l == nil {
		return map[string]int{}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	stats := make(map[string]int, len(l.counts))
	for authID, counter := range l.counts {
		if counter == nil {
			stats[authID] = 0
			continue
		}
		current := counter.Load()
		if current < 0 {
			counter.Store(0)
			current = 0
		}
		stats[authID] = int(current)
	}

	return stats
}
