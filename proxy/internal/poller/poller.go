package poller

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/ercadev/dirigent/store"
)

// Table is the routing table the poller updates as deployments change.
type Table interface {
	Set(domain, upstream string, basicAuth *store.BasicAuthConfig, security *store.SecurityConfig)
	Delete(domain string)
}

type routeState struct {
	Upstream      string
	BasicAuthJSON string
	SecurityJSON  string
}

// Store is the persistence interface the poller reads from.
type Store interface {
	List() ([]store.Deployment, error)
}

// Poller watches the JSON store on disk and syncs the routing table whenever
// deployments are created, updated, or removed.
type Poller struct {
	store    Store
	table    Table
	interval time.Duration
	last     map[string]routeState // domain → route state from the previous sync
}

// New creates a Poller that reads from s and updates t every interval.
func New(s Store, t Table, interval time.Duration) *Poller {
	return &Poller{
		store:    s,
		table:    t,
		interval: interval,
		last:     make(map[string]routeState),
	}
}

// Run starts the polling loop and blocks until ctx is cancelled.
// It performs an initial sync before the first tick so the routing table is
// populated immediately on startup.
func (p *Poller) Run(ctx context.Context) {
	p.sync()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.sync()
		case <-ctx.Done():
			return
		}
	}
}

func (p *Poller) sync() {
	deployments, err := p.store.List()
	if err != nil {
		log.Printf("proxy poller: list deployments: %v", err)
		return
	}

	// Build the current snapshot: domain → route for deployments that have
	// both a domain and at least one port binding.
	current := make(map[string]routeState, len(deployments))
	for _, d := range deployments {
		if d.Domain == "" {
			continue
		}
		domain := normalizeDomain(d.Domain)
		if domain == "" {
			continue
		}
		upstream := upstreamFromPorts(d.Ports)
		if upstream == "" {
			continue
		}
		current[domain] = routeState{
			Upstream:      upstream,
			BasicAuthJSON: mustBasicAuthJSON(d.BasicAuth),
			SecurityJSON:  mustSecurityJSON(d.Security),
		}
	}

	// Register new routes and update changed upstreams.
	for domain, route := range current {
		if p.last[domain] != route {
			var basicAuth *store.BasicAuthConfig
			var security *store.SecurityConfig
			if route.BasicAuthJSON != "" {
				if err := json.Unmarshal([]byte(route.BasicAuthJSON), &basicAuth); err != nil {
					log.Printf("proxy poller: decode basic auth for %s: %v", domain, err)
					continue
				}
			}
			if route.SecurityJSON != "" {
				if err := json.Unmarshal([]byte(route.SecurityJSON), &security); err != nil {
					log.Printf("proxy poller: decode security config for %s: %v", domain, err)
					continue
				}
			}
			p.table.Set(domain, route.Upstream, basicAuth, security)
		}
	}

	// Remove routes for deployments that no longer have a domain or were deleted.
	for domain := range p.last {
		if _, ok := current[domain]; !ok {
			p.table.Delete(domain)
		}
	}

	p.last = current
}

func mustBasicAuthJSON(auth *store.BasicAuthConfig) string {
	if auth == nil {
		return ""
	}
	b, err := json.Marshal(auth)
	if err != nil {
		return ""
	}
	return string(b)
}

func mustSecurityJSON(security *store.SecurityConfig) string {
	if security == nil {
		return ""
	}
	b, err := json.Marshal(security)
	if err != nil {
		return ""
	}
	return string(b)
}

// upstreamFromPorts extracts "localhost:<hostPort>" from the first usable
// Docker port binding in the slice. Returns an empty string if no usable
// binding is found.
//
// Accepted formats (matching Docker port specs):
//   - "8080:80"              → localhost:8080
//   - "8080:80/tcp"          → localhost:8080
//   - "127.0.0.1:8080:80"   → localhost:8080
func upstreamFromPorts(ports []string) string {
	for _, p := range ports {
		// Strip protocol suffix: "8080:80/tcp" → "8080:80"
		if i := strings.IndexByte(p, '/'); i >= 0 {
			p = p[:i]
		}
		parts := strings.Split(p, ":")
		switch len(parts) {
		case 2:
			// "hostPort:containerPort"
			if parts[0] != "" {
				return "localhost:" + parts[0]
			}
		case 3:
			// "ip:hostPort:containerPort"
			if parts[1] != "" {
				return "localhost:" + parts[1]
			}
		}
	}
	return ""
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")
	return strings.ToLower(domain)
}
