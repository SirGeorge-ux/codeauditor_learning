// Package driven holds the secondary (driven) adapters that implement
// the ports under internal/ports. This file wires the SessionCache
// adapter for the CachePort that the tutor-chat phase 2 service
// (PR 2) will consume.
package driven

import (
	"context"
	"sync"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ringCapacity is the hard cap on messages per session. The value
// comes from spec.md#Session-Memory ("rolling 10-message window") and
// matches design.md §9 ("It holds the last 10 messages"). Changing
// this constant requires updating the spec scenarios that assert the
// cap ("Window rolls over at 10 messages").
const ringCapacity = 10

// SessionCache is the in-memory 10-message ring buffer the tutor
// service uses for short-term memory. It implements ports.CachePort.
//
// Concurrency: backed by a single sync.Mutex that guards the map of
// sessions AND each ring buffer's slice header. The map is small
// (one entry per active challenge) and the lock window is short
// (append/evict/Get are O(1)), so a single mutex keeps the design
// honest without a parallel concurrent-map structure.
//
// Lifecycle: a single SessionCache is constructed at process start
// (cmd/api/wiring.go in PR 2) and shared across HTTP handlers. The
// cache is in-memory and does NOT survive a process restart; spec
// design §9 chose in-memory specifically to avoid a Redis dep.
type SessionCache struct {
	mu      sync.Mutex
	buckets map[string]*ringBuffer
}

// ringBuffer is the per-session FIFO. The slice is the backing
// storage; it grows up to ringCapacity and stays there forever
// (after the first overflow we reuse the existing slots, so we
// never re-allocate).
type ringBuffer struct {
	msgs []models.ChatMessage
}

// NewSessionCache returns an empty cache. The cache uses a non-nil
// map so callers can call Set/Get without first triggering a nil-map
// lazy initialization.
func NewSessionCache() *SessionCache {
	return &SessionCache{
		buckets: make(map[string]*ringBuffer),
	}
}

// Get returns the most recently appended message for sessionID. It
// matches the Set→Get round-trip the existing TutorServicePort
// consumer expects (PR 2 calls Get to surface "what did the user just
// send?" while Snapshot is used to feed the LLM with the full 10-
// message window). Returns (zero, false) when the session is unknown
// or empty.
func (c *SessionCache) Get(_ context.Context, sessionID string) (models.ChatMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rb, ok := c.buckets[sessionID]
	if !ok || len(rb.msgs) == 0 {
		return models.ChatMessage{}, false
	}
	return rb.msgs[len(rb.msgs)-1], true
}

// Set appends msg to sessionID's ring buffer. If the buffer is at
// ringCapacity, the oldest message is evicted (FIFO) so the window
// never grows past 10. The eviction is O(1) via in-place shift;
// for ringCapacity=10 the cost is trivial (vs. the alternative of a
// circular index, which would complicate Snapshot's oldest-first
// invariant).
func (c *SessionCache) Set(_ context.Context, sessionID string, msg models.ChatMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rb, ok := c.buckets[sessionID]
	if !ok {
		rb = &ringBuffer{}
		c.buckets[sessionID] = rb
	}
	if len(rb.msgs) < ringCapacity {
		rb.msgs = append(rb.msgs, msg)
		return
	}
	// Cap reached: shift left and append. ringCapacity is 10 so this
	// is ten pointer assignments at worst.
	rb.msgs = append(rb.msgs[1:], msg)
}

// Snapshot returns a copy of the current ring buffer in insertion
// order (oldest first). The copy protects callers from observing
// the slice header mutate under concurrent Set calls — the LLM can
// iterate the snapshot without a defensive lock of its own.
func (c *SessionCache) Snapshot(_ context.Context, sessionID string) []models.ChatMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	rb, ok := c.buckets[sessionID]
	if !ok || len(rb.msgs) == 0 {
		return []models.ChatMessage{} // non-nil empty slice
	}
	out := make([]models.ChatMessage, len(rb.msgs))
	copy(out, rb.msgs)
	return out
}

// Keys returns the active session IDs. The slice is freshly allocated
// so the caller can range over it without holding the cache's lock.
// Iteration order is non-deterministic (Go map semantics) which is
// fine: administrative tooling never relies on order.
func (c *SessionCache) Keys(_ context.Context) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	keys := make([]string, 0, len(c.buckets))
	for k := range c.buckets {
		keys = append(keys, k)
	}
	return keys
}

// Reset clears the ring buffer for sessionID. Idempotent on an
// unknown session.
func (c *SessionCache) Reset(_ context.Context, sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.buckets, sessionID)
}

// Compile-time guard: SessionCache must satisfy ports.CachePort so a
// signature drift in the port surfaces at PR review rather than at
// runtime wiring.
var _ ports.CachePort = (*SessionCache)(nil)
