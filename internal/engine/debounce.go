package engine

import (
	"sync"
	"time"
)

// CooldownTracker prevents repeated notifications for the same device within a time window.
type CooldownTracker struct {
	mu         sync.RWMutex
	entries    map[string]time.Time
	sweepEvery time.Duration
	done       chan struct{}
}

// NewCooldownTracker creates a new cooldown tracker and starts its background sweeper.
// The sweeper goroutine removes stale entries periodically to prevent memory leaks.
func NewCooldownTracker() *CooldownTracker {
	ct := &CooldownTracker{
		entries:    make(map[string]time.Time),
		sweepEvery: 10 * time.Minute,
		done:       make(chan struct{}),
	}
	go ct.sweeper()
	return ct
}

// IsOnCooldown returns true if the key was notified less than cooldownMinutes ago.
func (ct *CooldownTracker) IsOnCooldown(key string, cooldownMinutes int) bool {
	if cooldownMinutes <= 0 {
		return false
	}

	ct.mu.RLock()
	lastNotified, exists := ct.entries[key]
	ct.mu.RUnlock()

	if !exists {
		return false
	}

	return time.Since(lastNotified) < time.Duration(cooldownMinutes)*time.Minute
}

// Set marks the key as notified right now.
func (ct *CooldownTracker) Set(key string) {
	ct.mu.Lock()
	ct.entries[key] = time.Now()
	ct.mu.Unlock()
}

// Reset clears the cooldown for a specific key.
func (ct *CooldownTracker) Reset(key string) {
	ct.mu.Lock()
	delete(ct.entries, key)
	ct.mu.Unlock()
}

// sweeper periodically removes entries older than 1 hour + the cooldown window.
// This prevents unbounded memory growth from devices that appear once and never again.
func (ct *CooldownTracker) sweeper() {
	ticker := time.NewTicker(ct.sweepEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ct.sweep(1 * time.Hour)
		case <-ct.done:
			return
		}
	}
}

// sweep removes entries older than maxAge.
func (ct *CooldownTracker) sweep(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)

	ct.mu.Lock()
	for key, lastNotified := range ct.entries {
		if lastNotified.Before(cutoff) {
			delete(ct.entries, key)
		}
	}
	ct.mu.Unlock()
}

// Stop terminates the background sweeper goroutine.
func (ct *CooldownTracker) Stop() {
	select {
	case <-ct.done:
		// Already stopped.
	default:
		close(ct.done)
	}
}

// Size returns the number of tracked entries (useful for diagnostics).
func (ct *CooldownTracker) Size() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.entries)
}
