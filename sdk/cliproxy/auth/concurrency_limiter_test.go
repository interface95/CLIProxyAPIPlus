package auth

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestConcurrencyLimiter_AcquireRelease(t *testing.T) {
	limiter := NewConcurrencyLimiter()

	for i := 0; i < 3; i++ {
		if !limiter.Acquire("auth-1", 3) {
			t.Fatalf("acquire #%d = false, want true", i+1)
		}
	}
	if limiter.Acquire("auth-1", 3) {
		t.Fatal("acquire #4 = true, want false")
	}

	limiter.Release("auth-1")
	if !limiter.Acquire("auth-1", 3) {
		t.Fatal("acquire after release = false, want true")
	}
}

func TestConcurrencyLimiter_ZeroLimitUnlimited(t *testing.T) {
	limiter := NewConcurrencyLimiter()

	for i := 0; i < 10000; i++ {
		if !limiter.Acquire("auth-unlimited", 0) {
			t.Fatalf("acquire with zero limit #%d = false, want true", i+1)
		}
	}

	if got := limiter.Count("auth-unlimited"); got != 0 {
		t.Fatalf("count with zero limit = %d, want 0", got)
	}
}

func TestConcurrencyLimiter_Concurrent(t *testing.T) {
	limiter := NewConcurrencyLimiter()

	const (
		workers = 100
		limit   = 10
	)

	var (
		wg      sync.WaitGroup
		success atomic.Int64
	)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if limiter.Acquire("auth-concurrent", limit) {
				success.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := success.Load(); got > limit {
		t.Fatalf("concurrent success = %d, want <= %d", got, limit)
	}
}

func TestConcurrencyLimiter_ReleaseClampToZero(t *testing.T) {
	limiter := NewConcurrencyLimiter()

	for i := 0; i < 10; i++ {
		limiter.Release("auth-clamp")
	}

	if got := limiter.Count("auth-clamp"); got < 0 {
		t.Fatalf("count after over-release = %d, want >= 0", got)
	}
	if got := limiter.Count("auth-clamp"); got != 0 {
		t.Fatalf("count after over-release = %d, want 0", got)
	}
}

func TestConcurrencyLimiter_MultipleAuthIDs(t *testing.T) {
	limiter := NewConcurrencyLimiter()

	if !limiter.Acquire("auth-a", 2) || !limiter.Acquire("auth-a", 2) {
		t.Fatal("auth-a acquire failed")
	}
	if !limiter.Acquire("auth-b", 1) {
		t.Fatal("auth-b acquire failed")
	}

	if got := limiter.Count("auth-a"); got != 2 {
		t.Fatalf("auth-a count = %d, want 2", got)
	}
	if got := limiter.Count("auth-b"); got != 1 {
		t.Fatalf("auth-b count = %d, want 1", got)
	}
}

func TestConcurrencyLimiter_Stats(t *testing.T) {
	limiter := NewConcurrencyLimiter()

	if !limiter.Acquire("auth-a", 2) || !limiter.Acquire("auth-a", 2) {
		t.Fatal("auth-a setup failed")
	}
	if !limiter.Acquire("auth-b", 2) {
		t.Fatal("auth-b setup failed")
	}

	stats := limiter.Stats()

	if got := stats["auth-a"]; got != 2 {
		t.Fatalf("stats[auth-a] = %d, want 2", got)
	}
	if got := stats["auth-b"]; got != 1 {
		t.Fatalf("stats[auth-b] = %d, want 1", got)
	}
}
