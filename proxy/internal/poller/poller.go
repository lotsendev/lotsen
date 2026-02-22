package poller

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/ercadev/dirigent/store"
)

// Table is the routing table the poller updates as deployments change.
type Table interface {
	Set(domain, upstream string)
	Delete(domain string)
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
	last     map[string]string // domain → upstream from the previous sync
}

// New creates a Poller that reads from s and updates t every interval.
func New(s Store, t Table, interval time.Duration) *Poller {
	return &Poller{
		store:    s,
		table:    t,
		interval: interval,
		last:     make(map[string]string),
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

	// Build the current snapshot: domain → upstream for deployments that have
	// both a domain and at least one port binding.
	current := make(map[string]string, len(deployments))
	for _, d := range deployments {
		if d.Domain == "" {
			continue
		}
		upstream := upstreamFromPorts(d.Ports)
		if upstream == "" {
			continue
		}
		current[d.Domain] = upstream
	}

	// Register new routes and update changed upstreams.
	for domain, upstream := range current {
		if p.last[domain] != upstream {
			p.table.Set(domain, upstream)
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
