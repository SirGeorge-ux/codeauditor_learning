package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

// DefaultRateLimit is the per-user, per-tool budget: 10 admitted requests per
// rolling 60-second window. The spec mandates this default (JSON-RPC -32609 on
// the 11th call within the minute).
const (
	DefaultRateLimit  = 10
	DefaultRateWindow = 60 * time.Second
)

// rateLimitedError signals a rate-limit violation. rpcErrorFrom (in
// dispatcher.go) maps it to JSON-RPC code -32609.
type rateLimitedError struct{ message string }

func (e *rateLimitedError) Error() string { return e.message }

// IsRateLimitedError reports whether err is a rate-limit rejection.
func IsRateLimitedError(err error) bool {
	var rl *rateLimitedError
	return errors.As(err, &rl)
}

// NewRateLimitedError constructs a rate-limit error with the standard message.
func NewRateLimitedError(tool string) error {
	return &rateLimitedError{message: fmt.Sprintf("rate limit exceeded for tool %q (-32609)", tool)}
}

// callRecord tracks the timestamps of recent admits for one (user, tool) pair.
// Timestamps older than the window are pruned on each check (sliding window).
type callRecord struct {
	mu     sync.Mutex
	events []time.Time
}

// RateLimiter enforces a per-user, per-tool sliding-window limit. It is safe
// for concurrent use across all (user, tool) pairs.
type RateLimiter struct {
	mu      sync.Mutex
	records map[string]*callRecord
	limit   int
	window  time.Duration
	clock   func() time.Time // injectable for deterministic tests
}

// NewRateLimiter builds a limiter using the default 10 calls / 60s policy.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		records: make(map[string]*callRecord),
		limit:   DefaultRateLimit,
		window:  DefaultRateWindow,
		clock:   time.Now,
	}
}

// WithClock injects a deterministic clock (for tests).
func (rl *RateLimiter) WithClock(c func() time.Time) *RateLimiter {
	rl.clock = c
	return rl
}

// rateKey is the composite identifier for a (user, tool) bucket.
func rateKey(userID, toolName string) string { return userID + "\x00" + toolName }

func (rl *RateLimiter) bucket(userID, toolName string) *callRecord {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	k := rateKey(userID, toolName)
	rec, ok := rl.records[k]
	if !ok {
		rec = &callRecord{}
		rl.records[k] = rec
	}
	return rec
}

// Allow reports whether userID may invoke toolName again within the current
// window. On success the call is recorded; on rejection a -32609 error is
// returned and the call is NOT recorded.
func (rl *RateLimiter) Allow(userID, toolName string) error {
	rec := rl.bucket(userID, toolName)
	rec.mu.Lock()
	defer rec.mu.Unlock()

	now := rl.clock()
	cutoff := now.Add(-rl.window)

	// Prune entries older than the window (sliding window).
	drop := 0
	for drop < len(rec.events) && rec.events[drop].Before(cutoff) {
		drop++
	}
	rec.events = rec.events[drop:]

	if len(rec.events) >= rl.limit {
		return NewRateLimitedError(toolName) // -32609
	}
	rec.events = append(rec.events, now)
	return nil
}

// Middleware builds a chi-compatible http.Handler wrapper that enforces the
// rate limit on /mcp/tools/call before the dispatcher runs. The tool name is
// read from the JSON-RPC request body (then the body is restored for the
// downstream handler). Requests without an authenticated user id are passed
// through so the auth middleware mounted earlier can reject them with 401.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := authmiddleware.GetUserID(r.Context())
		if userID == "" {
			next.ServeHTTP(w, r)
			return
		}

		toolName, body := peekToolName(r)
		if body != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
		}

		if err := rl.Allow(userID, toolName); err != nil {
			writeRPCError(w, nil, &RPCError{Code: CodeRateLimited, Message: "Rate limit exceeded (-32609)"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// peekToolName buffers the request body and extracts params.name without
// consuming it for downstream handlers. On any error it returns "" and leaves
// the body restored so the dispatcher can surface the appropriate -326xx.
func peekToolName(r *http.Request) (string, []byte) {
	if r.Body == nil {
		return "", nil
	}
	const maxPeek = 1 << 20 // 1 MB
	data, err := io.ReadAll(io.LimitReader(r.Body, maxPeek))
	if err != nil {
		return "", data
	}

	var req rpcRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return "", data
	}
	return req.Params.Name, data
}
