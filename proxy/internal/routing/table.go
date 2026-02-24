package routing

import "sync"

// Table is an in-memory domain→upstream routing table safe for concurrent use.
type Table struct {
	mu            sync.RWMutex
	dynamicRoutes map[string]string
	staticRoutes  map[string]string
}

// NewTable creates an empty Table.
func NewTable() *Table {
	return &Table{
		dynamicRoutes: make(map[string]string),
		staticRoutes:  make(map[string]string),
	}
}

// Set registers or replaces the upstream for domain.
func (t *Table) Set(domain, upstream string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.dynamicRoutes[domain] = upstream
}

// SetStatic registers or replaces a static upstream for domain.
func (t *Table) SetStatic(domain, upstream string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.staticRoutes[domain] = upstream
}

// Get returns the upstream for domain and whether it exists.
func (t *Table) Get(domain string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if u, ok := t.staticRoutes[domain]; ok {
		return u, true
	}
	u, ok := t.dynamicRoutes[domain]
	return u, ok
}

// Delete removes domain from the table. No-op if domain is not registered.
func (t *Table) Delete(domain string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.dynamicRoutes, domain)
}
