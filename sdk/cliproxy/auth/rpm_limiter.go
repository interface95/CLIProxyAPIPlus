package auth

import (
	"context"
	"sync"
	"time"
)

const (
	rpmWindow         = 60 * time.Second
	rpmCleanupEvery   = 5 * time.Minute
	rpmEntryIdleLimit = 10 * time.Minute
)

type rpmEntry struct {
	timestamps []int64
	lastSeen   time.Time
}

type RPMLimiterOption func(*RPMLimiter)

type RPMLimiter struct {
	mu      sync.Mutex
	entries map[string]*rpmEntry
	clock   func() time.Time

	cleanupStop chan struct{}
}

type RPMStats struct {
	Current    int
	Limit      int
	RetryAfter time.Duration
}

func WithClock(clock func() time.Time) RPMLimiterOption {
	return func(l *RPMLimiter) {
		if clock != nil {
			l.clock = clock
		}
	}
}

func NewRPMLimiter(opts ...RPMLimiterOption) *RPMLimiter {
	l := &RPMLimiter{
		entries: make(map[string]*rpmEntry),
		clock:   time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(l)
		}
	}
	return l
}

func (l *RPMLimiter) Allow(authID string, limit int) bool {
	if limit == 0 {
		return true
	}

	now := l.clock()
	nowMilli := now.UnixMilli()
	windowStart := nowMilli - rpmWindow.Milliseconds()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[authID]
	if entry == nil {
		entry = &rpmEntry{}
		l.entries[authID] = entry
	}

	entry.timestamps = pruneMillis(entry.timestamps, windowStart)
	entry.lastSeen = now

	if len(entry.timestamps) >= limit {
		return false
	}

	entry.timestamps = append(entry.timestamps, nowMilli)
	return true
}

func (l *RPMLimiter) RetryAfter(authID string, limit int) time.Duration {
	if limit == 0 {
		return 0
	}

	now := l.clock()
	nowMilli := now.UnixMilli()
	windowStart := nowMilli - rpmWindow.Milliseconds()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[authID]
	if entry == nil {
		return 0
	}

	entry.timestamps = pruneMillis(entry.timestamps, windowStart)
	entry.lastSeen = now
	if len(entry.timestamps) < limit {
		return 0
	}

	retryAt := time.UnixMilli(entry.timestamps[0] + rpmWindow.Milliseconds())
	wait := retryAt.Sub(now)
	if wait < 0 {
		return 0
	}
	return wait
}

func (l *RPMLimiter) Count(authID string) int {
	now := l.clock()
	nowMilli := now.UnixMilli()
	windowStart := nowMilli - rpmWindow.Milliseconds()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[authID]
	if entry == nil {
		return 0
	}

	entry.timestamps = pruneMillis(entry.timestamps, windowStart)
	entry.lastSeen = now
	return len(entry.timestamps)
}

func (l *RPMLimiter) Stats(limitResolver func(authID string) int) map[string]RPMStats {
	now := l.clock()
	nowMilli := now.UnixMilli()
	windowStart := nowMilli - rpmWindow.Milliseconds()

	l.mu.Lock()
	defer l.mu.Unlock()

	stats := make(map[string]RPMStats, len(l.entries))
	for authID, entry := range l.entries {
		entry.timestamps = pruneMillis(entry.timestamps, windowStart)
		entry.lastSeen = now

		limit := 0
		if limitResolver != nil {
			limit = limitResolver(authID)
			if limit < 0 {
				limit = 0
			}
		}

		retryAfter := time.Duration(0)
		if limit > 0 && len(entry.timestamps) >= limit {
			retryAt := time.UnixMilli(entry.timestamps[0] + rpmWindow.Milliseconds())
			retryAfter = retryAt.Sub(now)
			if retryAfter < 0 {
				retryAfter = 0
			}
		}

		stats[authID] = RPMStats{
			Current:    len(entry.timestamps),
			Limit:      limit,
			RetryAfter: retryAfter,
		}
	}

	return stats
}

func (l *RPMLimiter) RunCleanup() {
	now := l.clock()

	l.mu.Lock()
	defer l.mu.Unlock()

	for authID, entry := range l.entries {
		if now.Sub(entry.lastSeen) > rpmEntryIdleLimit {
			delete(l.entries, authID)
		}
	}
}

func (l *RPMLimiter) StartCleanup(ctx context.Context) {
	l.mu.Lock()
	if l.cleanupStop != nil {
		l.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	l.cleanupStop = stop
	l.mu.Unlock()

	go func(localStop chan struct{}) {
		ticker := time.NewTicker(rpmCleanupEvery)
		defer ticker.Stop()
		defer func() {
			l.mu.Lock()
			if l.cleanupStop == localStop {
				l.cleanupStop = nil
			}
			l.mu.Unlock()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-localStop:
				return
			case <-ticker.C:
				l.RunCleanup()
			}
		}
	}(stop)
}

func (l *RPMLimiter) Stop() {
	l.mu.Lock()
	stop := l.cleanupStop
	l.cleanupStop = nil
	l.mu.Unlock()

	if stop != nil {
		close(stop)
	}
}

func pruneMillis(timestamps []int64, windowStart int64) []int64 {
	if len(timestamps) == 0 {
		return timestamps
	}

	idx := 0
	for idx < len(timestamps) && timestamps[idx] <= windowStart {
		idx++
	}
	if idx == 0 {
		return timestamps
	}
	if idx >= len(timestamps) {
		return timestamps[:0]
	}
	return timestamps[idx:]
}
