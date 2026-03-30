package auth

import (
	"testing"
	"time"
)

// TestIntegration_RPM_MultiCredentialRotation 验证多凭证场景下 RPMLimiter 的限流与恢复行为。
func TestIntegration_RPM_MultiCredentialRotation(t *testing.T) {
	t.Parallel()

	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	// auth-A: limit=2, auth-B: limit=5, auth-C: 无限制
	authA := &Auth{ID: "auth-A", Provider: "gemini", Attributes: map[string]string{"rpm_limit": "2"}}
	authB := &Auth{ID: "auth-B", Provider: "gemini", Attributes: map[string]string{"rpm_limit": "5"}}
	authC := &Auth{ID: "auth-C", Provider: "gemini"}

	scheduler := newAuthScheduler(&RoundRobinSelector{}, limiter, 0, nil, 0)
	scheduler.rebuild([]*Auth{authA, authB, authC})

	// auth-A: 前 2 次 Allow 应成功
	for i := 0; i < 2; i++ {
		if !limiter.Allow("auth-A", scheduler.resolveRPMLimit(authA)) {
			t.Fatalf("auth-A allow #%d = false, want true", i+1)
		}
	}
	// auth-A: 第 3 次应被限流
	if limiter.Allow("auth-A", scheduler.resolveRPMLimit(authA)) {
		t.Fatal("auth-A allow #3 = true, want false (should be rate limited)")
	}
	if got := limiter.Count("auth-A"); got != 2 {
		t.Fatalf("auth-A count = %d, want 2", got)
	}

	// auth-B: 前 5 次 Allow 应成功
	for i := 0; i < 5; i++ {
		if !limiter.Allow("auth-B", scheduler.resolveRPMLimit(authB)) {
			t.Fatalf("auth-B allow #%d = false, want true", i+1)
		}
	}
	// auth-B: 第 6 次应被限流
	if limiter.Allow("auth-B", scheduler.resolveRPMLimit(authB)) {
		t.Fatal("auth-B allow #6 = true, want false (should be rate limited)")
	}
	if got := limiter.Count("auth-B"); got != 5 {
		t.Fatalf("auth-B count = %d, want 5", got)
	}

	// auth-C: 无 rpm_limit，resolveRPMLimit 应返回 0（无限制）
	limitC := scheduler.resolveRPMLimit(authC)
	if limitC != 0 {
		t.Fatalf("auth-C resolveRPMLimit = %d, want 0 (unlimited)", limitC)
	}
	// auth-C 始终可用（limit=0 表示不限流）
	for i := 0; i < 100; i++ {
		if !limiter.Allow("auth-C", 0) {
			t.Fatalf("auth-C allow #%d = false, want true (unlimited)", i+1)
		}
	}

	// 推进 61 秒，验证 auth-A 和 auth-B 恢复
	clk.Advance(61 * time.Second)

	if !limiter.Allow("auth-A", scheduler.resolveRPMLimit(authA)) {
		t.Fatal("auth-A allow after window rollover = false, want true")
	}
	if !limiter.Allow("auth-B", scheduler.resolveRPMLimit(authB)) {
		t.Fatal("auth-B allow after window rollover = false, want true")
	}
}

// TestIntegration_RPM_AllBlocked 验证所有凭证均被限流时 Allow 返回 false 且 RetryAfter > 0。
func TestIntegration_RPM_AllBlocked(t *testing.T) {
	t.Parallel()

	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	authA := &Auth{ID: "auth-X", Provider: "gemini", Attributes: map[string]string{"rpm_limit": "1"}}
	authB := &Auth{ID: "auth-Y", Provider: "gemini", Attributes: map[string]string{"rpm_limit": "1"}}

	scheduler := newAuthScheduler(&RoundRobinSelector{}, limiter, 0, nil, 0)
	scheduler.rebuild([]*Auth{authA, authB})

	limitA := scheduler.resolveRPMLimit(authA)
	limitB := scheduler.resolveRPMLimit(authB)

	// 各消耗 1 次
	if !limiter.Allow("auth-X", limitA) {
		t.Fatal("auth-X allow #1 = false, want true")
	}
	if !limiter.Allow("auth-Y", limitB) {
		t.Fatal("auth-Y allow #1 = false, want true")
	}

	// 验证两者均被限流
	if limiter.Allow("auth-X", limitA) {
		t.Fatal("auth-X allow #2 = true, want false (should be blocked)")
	}
	if limiter.Allow("auth-Y", limitB) {
		t.Fatal("auth-Y allow #2 = true, want false (should be blocked)")
	}

	// 验证 RetryAfter > 0
	retryA := limiter.RetryAfter("auth-X", limitA)
	if retryA <= 0 {
		t.Fatalf("auth-X RetryAfter = %v, want > 0", retryA)
	}
	retryB := limiter.RetryAfter("auth-Y", limitB)
	if retryB <= 0 {
		t.Fatalf("auth-Y RetryAfter = %v, want > 0", retryB)
	}
}

// TestIntegration_RPM_DefaultRPMFallback 验证无 rpm_limit 属性时 resolveRPMLimit 回退到 defaultRPM。
func TestIntegration_RPM_DefaultRPMFallback(t *testing.T) {
	t.Parallel()

	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))

	// 凭证无 rpm_limit 属性
	authNoLimit := &Auth{ID: "auth-nolimit", Provider: "gemini"}

	// scheduler.defaultRPM = 3
	scheduler := newAuthScheduler(&RoundRobinSelector{}, limiter, 3, nil, 0)
	scheduler.rebuild([]*Auth{authNoLimit})

	// resolveRPMLimit 应返回 defaultRPM=3
	got := scheduler.resolveRPMLimit(authNoLimit)
	if got != 3 {
		t.Fatalf("resolveRPMLimit (no rpm_limit attr, defaultRPM=3) = %d, want 3", got)
	}

	// 验证实际限流行为：前 3 次通过，第 4 次被拒
	for i := 0; i < 3; i++ {
		if !limiter.Allow("auth-nolimit", got) {
			t.Fatalf("auth-nolimit allow #%d = false, want true", i+1)
		}
	}
	if limiter.Allow("auth-nolimit", got) {
		t.Fatal("auth-nolimit allow #4 = true, want false (defaultRPM=3 exhausted)")
	}
}
