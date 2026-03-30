package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func TestScheduler_SkipsRPMBlocked(t *testing.T) {
	t.Parallel()

	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))
	scheduler := newAuthScheduler(&RoundRobinSelector{}, limiter, 0, nil, 0)
	scheduler.rebuild([]*Auth{
		{ID: "auth-1", Provider: "gemini", Attributes: map[string]string{"rpm_limit": "2"}},
		{ID: "auth-2", Provider: "gemini", Attributes: map[string]string{"rpm_limit": "2"}},
	})

	if !limiter.Allow("auth-1", 2) {
		t.Fatal("allow auth-1 #1 = false, want true")
	}
	if !limiter.Allow("auth-1", 2) {
		t.Fatal("allow auth-1 #2 = false, want true")
	}

	got, provider, errPick := scheduler.pickMixed(context.Background(), []string{"gemini"}, "", cliproxyexecutor.Options{}, nil)
	if errPick != nil {
		t.Fatalf("pickMixed() error = %v", errPick)
	}
	if provider != "gemini" {
		t.Fatalf("pickMixed() provider = %q, want %q", provider, "gemini")
	}
	if got == nil {
		t.Fatal("pickMixed() auth = nil")
	}
	if got.ID != "auth-2" {
		t.Fatalf("pickMixed() auth.ID = %q, want %q", got.ID, "auth-2")
	}
}

func TestScheduler_AllRPMBlocked(t *testing.T) {
	t.Parallel()

	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))
	scheduler := newAuthScheduler(&RoundRobinSelector{}, limiter, 0, nil, 0)
	scheduler.rebuild([]*Auth{
		{ID: "auth-a", Provider: "gemini", Attributes: map[string]string{"rpm_limit": "1"}},
		{ID: "auth-b", Provider: "gemini", Attributes: map[string]string{"rpm_limit": "1"}},
	})

	if !limiter.Allow("auth-a", 1) {
		t.Fatal("allow auth-a #1 = false, want true")
	}
	if !limiter.Allow("auth-b", 1) {
		t.Fatal("allow auth-b #1 = false, want true")
	}

	got, provider, errPick := scheduler.pickMixed(context.Background(), []string{"gemini"}, "", cliproxyexecutor.Options{}, nil)
	if got != nil {
		t.Fatalf("pickMixed() auth = %v, want nil", got)
	}
	if provider != "" {
		t.Fatalf("pickMixed() provider = %q, want empty", provider)
	}
	if errPick == nil {
		t.Fatal("pickMixed() error = nil, want non-nil")
	}

	var cooldownErr *modelCooldownError
	if errors.As(errPick, &cooldownErr) {
		return
	}
	var authErr *Error
	if !errors.As(errPick, &authErr) || authErr == nil {
		t.Fatalf("pickMixed() error type = %T, want *modelCooldownError or *Error", errPick)
	}
	if authErr.Code != "auth_unavailable" && authErr.Code != "auth_not_found" {
		t.Fatalf("pickMixed() error code = %q, want auth_unavailable/auth_not_found", authErr.Code)
	}
}

func TestScheduler_NoRPMLimitUnaffected(t *testing.T) {
	t.Parallel()

	clk := newManualClock(time.Unix(0, 0))
	limiter := NewRPMLimiter(WithClock(clk.Now))
	scheduler := newAuthScheduler(&RoundRobinSelector{}, limiter, 0, nil, 0)
	scheduler.rebuild([]*Auth{{ID: "auth-unlimited", Provider: "gemini"}})

	for i := 0; i < 100; i++ {
		got, provider, errPick := scheduler.pickMixed(context.Background(), []string{"gemini"}, "", cliproxyexecutor.Options{}, nil)
		if errPick != nil {
			t.Fatalf("pickMixed() #%d error = %v", i, errPick)
		}
		if provider != "gemini" {
			t.Fatalf("pickMixed() #%d provider = %q, want %q", i, provider, "gemini")
		}
		if got == nil || got.ID != "auth-unlimited" {
			t.Fatalf("pickMixed() #%d auth = %v, want auth-unlimited", i, got)
		}
	}
}
