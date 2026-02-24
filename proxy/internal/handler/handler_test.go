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
	handler.New(tbl, nil).RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func newProxyServerWithOptions(tbl *testTable, options ...handler.Option) *httptest.Server {
	mux := http.NewServeMux()
	handler.New(tbl, nil, options...).RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func newProxyServerWithDashboardAuth(tbl *testTable, auth *handler.DashboardAuth) *httptest.Server {
	mux := http.NewServeMux()
	handler.New(tbl, auth).RegisterRoutes(mux)
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

func TestProxy_HTTPSKnownDomainReachesBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String())

	mux := http.NewServeMux()
	handler.New(tbl, nil).RegisterRoutes(mux)
	proxy := httptest.NewTLSServer(mux)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "example.com"

	resp, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatalf("GET / over HTTPS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestProxy_DashboardDomainRequiresBasicAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("dashboard.example.com", backend.Listener.Addr().String())

	proxy := newProxyServerWithDashboardAuth(tbl, &handler.DashboardAuth{
		Domain:   "dashboard.example.com",
		Username: "admin",
		Password: "secret",
	})
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "dashboard.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); got != `Basic realm="Dirigent"` {
		t.Fatalf("want WWW-Authenticate header, got %q", got)
	}
}

func TestProxy_DashboardDomainWithValidBasicAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("dashboard.example.com", backend.Listener.Addr().String())

	proxy := newProxyServerWithDashboardAuth(tbl, &handler.DashboardAuth{
		Domain:   "dashboard.example.com",
		Username: "admin",
		Password: "secret",
	})
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "dashboard.example.com"
	req.SetBasicAuth("admin", "secret")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestProxy_NonDashboardDomainDoesNotRequireBasicAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("app.example.com", backend.Listener.Addr().String())

	proxy := newProxyServerWithDashboardAuth(tbl, &handler.DashboardAuth{
		Domain:   "dashboard.example.com",
		Username: "admin",
		Password: "secret",
	})
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "app.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestProxy_StandardHardeningBlocksSensitivePaths(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String())

	proxy := newProxyServer(tbl)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/.env", nil)
	req.Host = "example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /.env: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestProxy_OffHardeningAllowsSensitivePaths(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.env" {
			t.Fatalf("want path /.env, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String())

	proxy := newProxyServerWithOptions(tbl, handler.WithHardeningProfile(handler.HardeningOff))
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/.env", nil)
	req.Host = "example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /.env: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestProxy_StrictHardeningBlocksScannerPaths(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String())

	proxy := newProxyServerWithOptions(tbl, handler.WithHardeningProfile(handler.HardeningStrict))
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/swagger-ui.html", nil)
	req.Host = "example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /swagger-ui.html: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestProxy_StandardHardeningRateLimitsRepeatedScans(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String())

	proxy := newProxyServer(tbl)
	defer proxy.Close()

	for i := 0; i < 12; i++ {
		req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/.env", nil)
		req.Host = "example.com"
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("scan request %d: %v", i, err)
		}
		resp.Body.Close()
	}

	blocked, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	blocked.Host = "example.com"
	blocked.Header.Set("X-Forwarded-For", "203.0.113.7")
	resp, err := http.DefaultClient.Do(blocked)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", resp.StatusCode)
	}
}

func TestInternalAccessLogs_ReturnsEntries(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String())
	buffer := handler.NewAccessLogBuffer(10)

	mux := http.NewServeMux()
	handler.New(tbl, nil, handler.WithAccessLogger(buffer)).RegisterRoutes(mux)
	proxy := httptest.NewServer(mux)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/api", nil)
	req.Host = "example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api: %v", err)
	}
	resp.Body.Close()

	logResp, err := http.Get(proxy.URL + "/internal/access-logs")
	if err != nil {
		t.Fatalf("GET /internal/access-logs: %v", err)
	}
	defer logResp.Body.Close()
	if logResp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", logResp.StatusCode)
	}

	var entries []handler.AccessLogEntry
	if err := json.NewDecoder(logResp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode entries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("want at least one access log entry")
	}
	if entries[0].Method != http.MethodGet {
		t.Fatalf("want GET method, got %s", entries[0].Method)
	}
}

func TestInternalSecurityConfig_ReturnsHardeningSettings(t *testing.T) {
	mux := http.NewServeMux()
	handler.New(newTestTable(), nil, handler.WithHardeningProfile(handler.HardeningStrict)).RegisterRoutes(mux)
	proxy := httptest.NewServer(mux)
	defer proxy.Close()

	resp, err := http.Get(proxy.URL + "/internal/security-config")
	if err != nil {
		t.Fatalf("GET /internal/security-config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var cfg handler.HardeningSettings
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.Profile != handler.HardeningStrict {
		t.Fatalf("want strict profile, got %s", cfg.Profile)
	}
	if cfg.SuspiciousThreshold == 0 {
		t.Fatal("want non-zero threshold")
	}
}
