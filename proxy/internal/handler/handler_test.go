package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ercadev/dirigent/proxy/internal/handler"
)

// testTable is an in-memory routing table used only in tests.
type testTable struct {
	mu     sync.RWMutex
	routes map[string]string
}

func newTestTable() *testTable {
	return &testTable{routes: make(map[string]string)}
}

func (t *testTable) Set(domain, upstream string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routes[domain] = upstream
}

func (t *testTable) Get(domain string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	u, ok := t.routes[domain]
	return u, ok
}

func newProxyServer(tbl *testTable) *httptest.Server {
	mux := http.NewServeMux()
	handler.New(tbl).RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

// TestProxy_KnownDomainReachesBackend verifies that a request whose Host header
// matches a registered domain is forwarded to the correct backend container.
func TestProxy_KnownDomainReachesBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String())

	proxy := newProxyServer(tbl)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

// TestProxy_UnknownDomainReturns404 verifies that requests for unregistered
// domains are rejected with 404.
func TestProxy_UnknownDomainReturns404(t *testing.T) {
	proxy := newProxyServer(newTestTable())
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "unknown.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

// TestProxy_RemovedDeploymentIsUnreachable is an integration test confirming
// that after a deployment's domain is removed from the table the proxy stops
// routing requests for that domain.
func TestProxy_RemovedDeploymentIsUnreachable(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String())

	proxy := newProxyServer(tbl)
	defer proxy.Close()

	// First request must succeed.
	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("initial GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initial: want 200, got %d", resp.StatusCode)
	}

	// Simulate deployment deletion: remove the domain from the table.
	// (In production the store poller calls table.Delete; here we call it directly.)
	tbl.mu.Lock()
	delete(tbl.routes, "example.com")
	tbl.mu.Unlock()

	// Second request must now return 404.
	req, _ = http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "example.com"
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post-removal GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("post-removal: want 404, got %d", resp.StatusCode)
	}
}

func TestSetRoute_SetsUpstream(t *testing.T) {
	tbl := newTestTable()
	proxy := newProxyServer(tbl)
	defer proxy.Close()

	body, _ := json.Marshal(map[string]string{
		"domain":   "example.com",
		"upstream": "localhost:3000",
	})
	resp, err := http.Post(proxy.URL+"/internal/routes", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /internal/routes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	upstream, ok := tbl.Get("example.com")
	if !ok {
		t.Fatal("want route registered, got not found")
	}
	if upstream != "localhost:3000" {
		t.Errorf("want upstream localhost:3000, got %s", upstream)
	}
}

func TestSetRoute_MissingFields(t *testing.T) {
	proxy := newProxyServer(newTestTable())
	defer proxy.Close()

	cases := []map[string]string{
		{"upstream": "localhost:3000"}, // missing domain
		{"domain": "example.com"},      // missing upstream
		{},                             // both missing
	}

	for _, payload := range cases {
		body, _ := json.Marshal(payload)
		resp, err := http.Post(proxy.URL+"/internal/routes", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST /internal/routes: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("payload %v: want 400, got %d", payload, resp.StatusCode)
		}
	}
}

func TestSetRoute_InvalidBody(t *testing.T) {
	proxy := newProxyServer(newTestTable())
	defer proxy.Close()

	resp, err := http.Post(proxy.URL+"/internal/routes", "application/json", bytes.NewBufferString("not json"))
	if err != nil {
		t.Fatalf("POST /internal/routes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestHealth_Returns200(t *testing.T) {
	proxy := newProxyServer(newTestTable())
	defer proxy.Close()

	resp, err := http.Get(proxy.URL + "/internal/health")
	if err != nil {
		t.Fatalf("GET /internal/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestProxy_UpstreamUnavailableReturns502(t *testing.T) {
	tbl := newTestTable()
	// Point to a port nobody is listening on.
	tbl.Set("example.com", "localhost:1")

	proxy := newProxyServer(tbl)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", resp.StatusCode)
	}
}
