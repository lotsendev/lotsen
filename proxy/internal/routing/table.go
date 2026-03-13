package routing

import (
	"sync"

	"github.com/lotsendev/lotsen/store"
)

// Route stores upstream and optional per-deployment configuration.
type Route struct {
	Upstream  string
	Public    bool
	BasicAuth *store.BasicAuthConfig
	Security  *store.SecurityConfig
}

// Table is an in-memory domain→upstream routing table safe for concurrent use.
type Table struct {
	mu            sync.RWMutex
	dynamicRoutes map[string]Route
	staticRoutes  map[string]string
}

// NewTable creates an empty Table.
func NewTable() *Table {
	return &Table{
		dynamicRoutes: make(map[string]Route),
		staticRoutes:  make(map[string]string),
	}
}

// Set registers or replaces the route for domain.
func (t *Table) Set(domain, upstream string, public bool, basicAuth *store.BasicAuthConfig, security *store.SecurityConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.dynamicRoutes[domain] = Route{Upstream: upstream, Public: public, BasicAuth: basicAuth, Security: security}
}

// SetStatic registers or replaces a static upstream for domain.
func (t *Table) SetStatic(domain, upstream string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.staticRoutes[domain] = upstream
}

// Get returns the route for domain and whether it exists.
func (t *Table) Get(domain string) (Route, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if u, ok := t.staticRoutes[domain]; ok {
		return Route{Upstream: u}, true
	}
	route, ok := t.dynamicRoutes[domain]
	return route, ok
}

// Delete removes domain from the table. No-op if domain is not registered.
func (t *Table) Delete(domain string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.dynamicRoutes, domain)
}
