package auth

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type manualClock struct {
	unixMilli atomic.Int64
}

func newManualClock(base time.Time) *manualClock {
	clk := &manualClock{}
	clk.unixMilli.Store(base.UnixMilli())
	return clk
}

func (c *manualClock) Now() time.Time {
	return time.UnixMilli(c.unixMilli.Load())
}

func (c *manualClock) Advance(d time.Duration) {
	c.unixMilli.Add(d.Milliseconds())
}

func TestRPMLimiter_BlocksAfterLimit(t *testing.T) {
	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	for i := 0; i < 5; i++ {
		if !limiter.Allow("auth-1", 5) {
			t.Fatalf("allow #%d = false, want true", i+1)
		}
	}

	if limiter.Allow("auth-1", 5) {
		t.Fatal("allow #6 = true, want false")
	}
}

func TestRPMLimiter_WindowRollover(t *testing.T) {
	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	for i := 0; i < 5; i++ {
		if !limiter.Allow("auth-rollover", 5) {
			t.Fatalf("allow before rollover #%d = false, want true", i+1)
		}
	}
	if limiter.Allow("auth-rollover", 5) {
		t.Fatal("allow before rollover exhausted = true, want false")
	}

	clk.Advance(61 * time.Second)
	if !limiter.Allow("auth-rollover", 5) {
		t.Fatal("allow after window rollover = false, want true")
	}
}

func TestRPMLimiter_ZeroLimitUnlimited(t *testing.T) {
	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	for i := 0; i < 10000; i++ {
		if !limiter.Allow("auth-unlimited", 0) {
			t.Fatalf("allow with zero limit #%d = false, want true", i+1)
		}
	}

	if got := limiter.Count("auth-unlimited"); got != 0 {
		t.Fatalf("count with zero limit = %d, want 0 (no state mutation)", got)
	}
}

func TestRPMLimiter_Concurrent(t *testing.T) {
	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	const (
		workers = 100
		limit   = 50
	)

	var (
		wg      sync.WaitGroup
		success atomic.Int64
	)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if limiter.Allow("auth-concurrent", limit) {
				success.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := success.Load(); got > limit {
		t.Fatalf("concurrent success = %d, want <= %d", got, limit)
	}
}

func TestRPMLimiter_RetryAfter(t *testing.T) {
	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	for i := 0; i < 5; i++ {
		if !limiter.Allow("auth-retry", 5) {
			t.Fatalf("allow for retry setup #%d = false, want true", i+1)
		}
	}

	clk.Advance(30 * time.Second)
	retry := limiter.RetryAfter("auth-retry", 5)
	if retry < 29*time.Second || retry > 30*time.Second {
		t.Fatalf("retryAfter = %v, want around 30s", retry)
	}
}

func TestRPMLimiter_Cleanup(t *testing.T) {
	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	if !limiter.Allow("auth-clean", 5) {
		t.Fatal("allow setup for cleanup = false, want true")
	}

	clk.Advance(15 * time.Minute)
	limiter.RunCleanup()

	if got := limiter.Count("auth-clean"); got != 0 {
		t.Fatalf("count after cleanup = %d, want 0", got)
	}

	limiter.mu.Lock()
	_, exists := limiter.entries["auth-clean"]
	limiter.mu.Unlock()
	if exists {
		t.Fatal("entry still exists after cleanup, want removed")
	}
}

func TestRPMLimiter_MultipleAuthIDs(t *testing.T) {
	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	if !limiter.Allow("auth-a", 2) || !limiter.Allow("auth-a", 2) {
		t.Fatal("auth-a initial allows failed")
	}
	if limiter.Allow("auth-a", 2) {
		t.Fatal("auth-a third allow = true, want false")
	}

	if !limiter.Allow("auth-b", 2) || !limiter.Allow("auth-b", 2) {
		t.Fatal("auth-b initial allows failed")
	}
	if limiter.Allow("auth-b", 2) {
		t.Fatal("auth-b third allow = true, want false")
	}
}

func TestRPMLimiter_Stats(t *testing.T) {
	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	if !limiter.Allow("auth-a", 2) || !limiter.Allow("auth-a", 2) {
		t.Fatal("auth-a setup allows failed")
	}
	if !limiter.Allow("auth-b", 5) {
		t.Fatal("auth-b setup allow failed")
	}

	stats := limiter.Stats(func(authID string) int {
		switch authID {
		case "auth-a":
			return 2
		case "auth-b":
			return 5
		default:
			return 0
		}
	})

	a, ok := stats["auth-a"]
	if !ok {
		t.Fatal("stats missing auth-a")
	}
	if a.Current != 2 || a.Limit != 2 {
		t.Fatalf("auth-a stats = %+v, want Current=2 Limit=2", a)
	}
	if a.RetryAfter < 59*time.Second || a.RetryAfter > 60*time.Second {
		t.Fatalf("auth-a RetryAfter = %v, want around 60s", a.RetryAfter)
	}

	b, ok := stats["auth-b"]
	if !ok {
		t.Fatal("stats missing auth-b")
	}
	if b.Current != 1 || b.Limit != 5 {
		t.Fatalf("auth-b stats = %+v, want Current=1 Limit=5", b)
	}
	if b.RetryAfter != 0 {
		t.Fatalf("auth-b RetryAfter = %v, want 0", b.RetryAfter)
	}
}

// Evidence 文件约定（按测试粒度保存到 .sisyphus/evidence/task-1-*.txt）：
// - TestRPMLimiter_BlocksAfterLimit -> task-1-blocks-after-limit.txt
// - TestRPMLimiter_WindowRollover -> task-1-window-rollover.txt
// - TestRPMLimiter_ZeroLimitUnlimited -> task-1-zero-limit-unlimited.txt
// - TestRPMLimiter_Concurrent -> task-1-concurrent.txt
// - TestRPMLimiter_RetryAfter -> task-1-retry-after.txt
// - TestRPMLimiter_Cleanup -> task-1-cleanup.txt
// - TestRPMLimiter_MultipleAuthIDs -> task-1-multiple-auth-ids.txt
