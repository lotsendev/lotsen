package routing

import "sync"

// Table is an in-memory domain→upstream routing table safe for concurrent use.
type Table struct {
	mu     sync.RWMutex
	routes map[string]string
}

// NewTable creates an empty Table.
func NewTable() *Table {
	return &Table{routes: make(map[string]string)}
}

// Set registers or replaces the upstream for domain.
func (t *Table) Set(domain, upstream string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routes[domain] = upstream
}

// Get returns the upstream for domain and whether it exists.
func (t *Table) Get(domain string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	u, ok := t.routes[domain]
	return u, ok
}

// Delete removes domain from the table. No-op if domain is not registered.
func (t *Table) Delete(domain string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.routes, domain)
}
