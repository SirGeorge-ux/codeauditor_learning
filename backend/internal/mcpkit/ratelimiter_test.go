package mcpkit

import (
	"testing"
	"time"
)

// ---- RED 1.2: rate limiter returns -32609 at the 11th call within a minute ----

func TestRateLimiter_AllowsUpToLimit(t *testing.T) {
	rl := NewRateLimiter()
	for i := 0; i < DefaultRateLimit; i++ {
		if err := rl.Allow("user-1", "mcp-sandbox.run_code"); err != nil {
			t.Fatalf("call #%d should be allowed, got error: %v", i+1, err)
		}
	}
}

func TestRateLimiter_EnforcesLimitAt11thCall(t *testing.T) {
	rl := NewRateLimiter()
	// Exhaust the 10-call budget for (user-1, mcp-sandbox.run_code).
	for i := 0; i < DefaultRateLimit; i++ {
		if err := rl.Allow("user-1", "mcp-sandbox.run_code"); err != nil {
			t.Fatalf("call #%d should be allowed, got error: %v", i+1, err)
		}
	}
	// 11th call within the same minute window MUST be rejected with -32609.
	err := rl.Allow("user-1", "mcp-sandbox.run_code")
	if err == nil {
		t.Fatal("expected rate-limit error on the 11th call, got nil")
	}
	if !IsRateLimitedError(err) {
		t.Fatalf("expected a rate-limited error, got %v", err)
	}
	// The dispatcher maps a rate-limited error to JSON-RPC code -32609.
	rpcErr := rpcErrorFrom(err, "mcp-sandbox.run_code")
	if rpcErr.Code != CodeRateLimited {
		t.Errorf("expected code %d (-32609), got %d", CodeRateLimited, rpcErr.Code)
	}
}

func TestRateLimiter_IsolatesUsers(t *testing.T) {
	rl := NewRateLimiter()
	// user-1 exhausts its budget on its tool.
	for i := 0; i < DefaultRateLimit; i++ {
		if err := rl.Allow("user-1", "mcp-sandbox.run_code"); err != nil {
			t.Fatalf("user-1 call #%d should be allowed: %v", i+1, err)
		}
	}
	// user-2 has its own independent bucket and must still succeed.
	if err := rl.Allow("user-2", "mcp-sandbox.run_code"); err != nil {
		t.Fatalf("user-2 first call should be allowed (isolated bucket): %v", err)
	}
}

func TestRateLimiter_IsolatesTools(t *testing.T) {
	rl := NewRateLimiter()
	// (user-1, toolA) is exhausted.
	for i := 0; i < DefaultRateLimit; i++ {
		_ = rl.Allow("user-1", "mcp-tool-a.act")
	}
	// (user-1, toolB) is a separate bucket and must succeed.
	if err := rl.Allow("user-1", "mcp-tool-b.act"); err != nil {
		t.Fatalf("distinct tool should be allowed (isolated bucket): %v", err)
	}
}

func TestRateLimiter_WindowResetAllowsAgain(t *testing.T) {
	// Use a controllable clock so the test does not sleep 60 seconds.
	start := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	mock := &mockClock{now: start}
	clockFn := func() time.Time { return mock.now }
	rl := NewRateLimiter().WithClock(clockFn)

	// Exhaust the budget at t=0.
	for i := 0; i < DefaultRateLimit; i++ {
		if err := rl.Allow("u", "t"); err != nil {
			t.Fatalf("call #%d should be allowed: %v", i+1, err)
		}
	}
	// Still rejected just before the window elapses.
	mock.set(start.Add(59 * time.Second))
	if err := rl.Allow("u", "t"); err == nil {
		t.Fatal("expected rejection before window reset")
	}
	// After the window elapses the calls are pruned and new calls succeed.
	mock.set(start.Add(61 * time.Second))
	if err := rl.Allow("u", "t"); err != nil {
		t.Fatalf("expected call to succeed after window reset: %v", err)
	}
}

// mockClock is a controllable time source for deterministic rate-limit tests.
type mockClock struct{ now time.Time }

func (m *mockClock) set(t time.Time) { m.now = t }
