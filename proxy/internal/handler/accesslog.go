package handler

import "sync"

// AccessLogBuffer stores a fixed number of recent access log entries in memory.
type AccessLogBuffer struct {
	mu      sync.RWMutex
	entries []AccessLogEntry
	next    int
	count   int
}

// NewAccessLogBuffer creates a bounded in-memory access log store.
func NewAccessLogBuffer(capacity int) *AccessLogBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &AccessLogBuffer{entries: make([]AccessLogEntry, capacity)}
}

// Log appends a new access log entry.
func (b *AccessLogBuffer) Log(entry AccessLogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries[b.next] = entry
	b.next = (b.next + 1) % len(b.entries)
	if b.count < len(b.entries) {
		b.count++
	}
}

// List returns the most recent entries, newest first.
func (b *AccessLogBuffer) List(limit int) []AccessLogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 {
		return []AccessLogEntry{}
	}
	if limit > b.count {
		limit = b.count
	}
	out := make([]AccessLogEntry, 0, limit)
	for i := 0; i < limit; i++ {
		idx := (b.next - 1 - i + len(b.entries)) % len(b.entries)
		out = append(out, b.entries[idx])
	}
	return out
}
