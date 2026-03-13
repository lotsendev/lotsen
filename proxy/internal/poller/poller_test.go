package poller_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lotsendev/lotsen/proxy/internal/poller"
	"github.com/lotsendev/lotsen/proxy/internal/routing"
	"github.com/lotsendev/lotsen/store"
)

// memStore is an in-memory store for tests.
type memStore struct {
	mu          sync.RWMutex
	deployments []store.Deployment
}

func (m *memStore) List() ([]store.Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]store.Deployment, len(m.deployments))
	copy(out, m.deployments)
	return out, nil
}

// spyTable records Set and Delete calls.
type spyTable struct {
	mu      sync.Mutex
	routes  map[string]routing.Route
	deleted []string
}

func newSpyTable() *spyTable {
	return &spyTable{routes: make(map[string]routing.Route)}
}

func (s *spyTable) Set(domain, upstream string, public bool, basicAuth *store.BasicAuthConfig, security *store.SecurityConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[domain] = routing.Route{Upstream: upstream, BasicAuth: basicAuth, Security: security}
}

func (s *spyTable) Delete(domain string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.routes, domain)
	s.deleted = append(s.deleted, domain)
}

func (s *spyTable) get(domain string) (routing.Route, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	route, ok := s.routes[domain]
	return route, ok
}

func (s *spyTable) wasDeleted(domain string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.deleted {
		if d == domain {
			return true
		}
	}
	return false
}

func TestPoller_RegistersDeploymentWithDomainAndPort(t *testing.T) {
	ms := &memStore{
		deployments: []store.Deployment{
			{ID: "d1", Domain: "example.com", Ports: []string{"8080:80"}},
		},
	}
	tbl := newSpyTable()
	p := poller.New(ms, tbl, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go p.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	route, ok := tbl.get("example.com")
	if !ok {
		t.Fatal("want example.com registered, got not found")
	}
	if route.Upstream != "localhost:8080" {
		t.Errorf("want upstream localhost:8080, got %s", route.Upstream)
	}
}

func TestPoller_SkipsDeploymentWithoutDomain(t *testing.T) {
	ms := &memStore{
		deployments: []store.Deployment{
			{ID: "d1", Ports: []string{"8080:80"}}, // no domain
		},
	}
	tbl := newSpyTable()
	p := poller.New(ms, tbl, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go p.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	if len(tbl.routes) != 0 {
		t.Errorf("want no routes registered, got %d", len(tbl.routes))
	}
}

func TestPoller_SkipsDeploymentWithoutPorts(t *testing.T) {
	ms := &memStore{
		deployments: []store.Deployment{
			{ID: "d1", Domain: "example.com"}, // no ports
		},
	}
	tbl := newSpyTable()
	p := poller.New(ms, tbl, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go p.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	if _, ok := tbl.get("example.com"); ok {
		t.Error("want no route without ports, but route was registered")
	}
}

func TestPoller_SkipsDeploymentWithOnlyUDPPorts(t *testing.T) {
	ms := &memStore{
		deployments: []store.Deployment{
			{ID: "d1", Domain: "dns.example.com", Ports: []string{"53:53/udp"}},
		},
	}
	tbl := newSpyTable()
	p := poller.New(ms, tbl, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go p.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	if _, ok := tbl.get("dns.example.com"); ok {
		t.Error("want no route for udp-only deployment, but route was registered")
	}
}

func TestPoller_DeletesRemovedDomain(t *testing.T) {
	ms := &memStore{
		deployments: []store.Deployment{
			{ID: "d1", Domain: "example.com", Ports: []string{"8080:80"}},
		},
	}
	tbl := newSpyTable()
	p := poller.New(ms, tbl, 20*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go p.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	if _, ok := tbl.get("example.com"); !ok {
		t.Fatal("want example.com registered before removal")
	}

	// Remove the deployment from the store.
	ms.mu.Lock()
	ms.deployments = nil
	ms.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	if _, ok := tbl.get("example.com"); ok {
		t.Error("want example.com deleted after deployment removed, but it is still registered")
	}
	if !tbl.wasDeleted("example.com") {
		t.Error("want Delete called for example.com")
	}
}

func TestPoller_UpdatesChangedUpstream(t *testing.T) {
	ms := &memStore{
		deployments: []store.Deployment{
			{ID: "d1", Domain: "example.com", Ports: []string{"8080:80"}},
		},
	}
	tbl := newSpyTable()
	p := poller.New(ms, tbl, 20*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go p.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	if route, _ := tbl.get("example.com"); route.Upstream != "localhost:8080" {
		t.Fatalf("initial upstream: want localhost:8080, got %s", route.Upstream)
	}

	// Simulate a port change (e.g. after zero-downtime redeploy).
	ms.mu.Lock()
	ms.deployments[0].Ports = []string{"9090:80"}
	ms.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	if route, _ := tbl.get("example.com"); route.Upstream != "localhost:9090" {
		t.Errorf("updated upstream: want localhost:9090, got %s", route.Upstream)
	}
}

func TestPoller_UsesSelectedProxyPort(t *testing.T) {
	ms := &memStore{
		deployments: []store.Deployment{
			{ID: "d1", Domain: "example.com", Ports: []string{"53:53/udp", "32770:8080", "49123:80"}, ProxyPort: 80},
		},
	}
	tbl := newSpyTable()
	p := poller.New(ms, tbl, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go p.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	route, ok := tbl.get("example.com")
	if !ok {
		t.Fatal("want example.com registered, got not found")
	}
	if route.Upstream != "localhost:49123" {
		t.Fatalf("want upstream localhost:49123 from selected proxy port, got %s", route.Upstream)
	}
}

func TestPoller_DoesNotDeleteStaticRoutes(t *testing.T) {
	ms := &memStore{}
	tbl := routing.NewTable()
	tbl.SetStatic("dashboard.example.com", "localhost:3000")
	p := poller.New(ms, tbl, 20*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go p.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	if route, ok := tbl.Get("dashboard.example.com"); !ok || route.Upstream != "localhost:3000" {
		t.Fatalf("want static dashboard route preserved, got %q (ok=%v)", route.Upstream, ok)
	}
}

func TestPoller_PropagatesSecurityConfig(t *testing.T) {
	ms := &memStore{
		deployments: []store.Deployment{{
			ID:     "d1",
			Domain: "example.com",
			Ports:  []string{"8080:80"},
			Security: &store.SecurityConfig{
				WAFEnabled:  true,
				IPDenylist:  []string{"10.0.0.0/8"},
				IPAllowlist: []string{"203.0.113.0/24"},
			},
		}},
	}
	tbl := newSpyTable()
	p := poller.New(ms, tbl, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go p.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	route, ok := tbl.get("example.com")
	if !ok {
		t.Fatal("want example.com registered, got not found")
	}
	if route.Security == nil || !route.Security.WAFEnabled {
		t.Fatalf("want security config with waf enabled, got %#v", route.Security)
	}
	if len(route.Security.IPAllowlist) != 1 || route.Security.IPAllowlist[0] != "203.0.113.0/24" {
		t.Fatalf("want security allowlist propagated, got %#v", route.Security)
	}
}
