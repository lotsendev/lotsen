package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ercadev/lotsen/proxy/internal/handler"
	"github.com/ercadev/lotsen/proxy/internal/middleware"
	"github.com/ercadev/lotsen/proxy/internal/routing"
	"github.com/ercadev/lotsen/store"
	"golang.org/x/crypto/bcrypt"
)

// testTable is an in-memory routing table used only in tests.
type testTable struct {
	mu     sync.RWMutex
	routes map[string]routing.Route
}

func newTestTable() *testTable {
	return &testTable{routes: make(map[string]routing.Route)}
}

func (t *testTable) Set(domain, upstream string, public bool, basicAuth *store.BasicAuthConfig, security *store.SecurityConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routes[domain] = routing.Route{Upstream: upstream, BasicAuth: basicAuth, Security: security}
}

func (t *testTable) Get(domain string) (routing.Route, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	route, ok := t.routes[domain]
	return route, ok
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

func newProxyServerWithDashboardAuthAndOptions(tbl *testTable, auth *handler.DashboardAuth, options ...handler.Option) *httptest.Server {
	mux := http.NewServeMux()
	handler.New(tbl, auth, options...).RegisterRoutes(mux)
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
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

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
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

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

	route, ok := tbl.Get("example.com")
	if !ok {
		t.Fatal("want route registered, got not found")
	}
	if route.Upstream != "localhost:3000" {
		t.Errorf("want upstream localhost:3000, got %s", route.Upstream)
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

func TestProxy_AtomicSwap_KeepsInflightRequestsAndRoutesNewTraffic(t *testing.T) {
	oldStarted := make(chan struct{}, 1)
	oldRelease := make(chan struct{})

	oldBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oldStarted <- struct{}{}
		<-oldRelease
		_, _ = w.Write([]byte("old"))
	}))
	defer oldBackend.Close()

	newBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("new"))
	}))
	defer newBackend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", oldBackend.Listener.Addr().String(), false, nil, nil)

	proxy := newProxyServer(tbl)
	defer proxy.Close()

	firstDone := make(chan string, 1)
	firstErr := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
		req.Host = "example.com"
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			firstErr <- err
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			firstErr <- err
			return
		}
		if resp.StatusCode != http.StatusOK {
			firstErr <- errors.New("first request did not return 200")
			return
		}
		firstDone <- string(body)
	}()

	select {
	case <-oldStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first request to reach old backend")
	}

	body, _ := json.Marshal(map[string]string{
		"domain":   "example.com",
		"upstream": newBackend.Listener.Addr().String(),
	})
	resp, err := http.Post(proxy.URL+"/internal/routes", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /internal/routes: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204 from swap, got %d", resp.StatusCode)
	}

	secondReq, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	secondReq.Host = "example.com"
	secondResp, err := http.DefaultClient.Do(secondReq)
	if err != nil {
		t.Fatalf("second request through proxy: %v", err)
	}
	secondBody, err := io.ReadAll(secondResp.Body)
	secondResp.Body.Close()
	if err != nil {
		t.Fatalf("read second response: %v", err)
	}
	if secondResp.StatusCode != http.StatusOK {
		t.Fatalf("want second request 200, got %d", secondResp.StatusCode)
	}
	if string(secondBody) != "new" {
		t.Fatalf("want second request routed to new backend, got %q", string(secondBody))
	}

	close(oldRelease)

	select {
	case err := <-firstErr:
		t.Fatalf("first request failed: %v", err)
	case body := <-firstDone:
		if body != "old" {
			t.Fatalf("want inflight request to complete on old backend, got %q", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first request completion")
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
	tbl.Set("example.com", "localhost:1", false, nil, nil)

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
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

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

func TestProxy_DashboardDomainDoesNotRequireBasicAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("dashboard.example.com", backend.Listener.Addr().String(), false, nil, nil)

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

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestProxy_DashboardDomainWithValidBasicAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("dashboard.example.com", backend.Listener.Addr().String(), false, nil, nil)

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

func TestProxy_DashboardDomainBypassesWAF(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("dashboard.example.com", backend.Listener.Addr().String(), false, nil, nil)

	waf, err := middleware.NewWAF()
	if err != nil {
		t.Fatalf("NewWAF enforcement: %v", err)
	}
	proxy := newProxyServerWithDashboardAuthAndOptions(tbl, &handler.DashboardAuth{
		Domain:   "dashboard.example.com",
		Username: "admin",
		Password: "secret",
	}, handler.WithWAF(waf))
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodPut, proxy.URL+"/api/deployments/dashboard-waf-trigger", strings.NewReader(`{"name":"ok"}`))
	req.Host = "dashboard.example.com"
	req.SetBasicAuth("admin", "secret")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/deployments/dashboard-waf-trigger: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	trafficResp, err := http.Get(proxy.URL + "/internal/traffic")
	if err != nil {
		t.Fatalf("GET /internal/traffic: %v", err)
	}
	defer trafficResp.Body.Close()

	var body struct {
		WAFBlockedRequests int64 `json:"wafBlockedRequests"`
	}
	if err := json.NewDecoder(trafficResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode traffic response: %v", err)
	}
	if body.WAFBlockedRequests != 0 {
		t.Fatalf("want waf blocked requests 0, got %d", body.WAFBlockedRequests)
	}
}

func TestProxy_DashboardDomainAppliesWAFWhenEnabledByMode(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	wafRule := `SecRule REQUEST_URI "@contains dashboard-waf-trigger" "id:10100,phase:1,deny,status:403,log,msg:'dashboard waf trigger mode'"`

	tbl := newTestTable()
	tbl.Set("dashboard.example.com", backend.Listener.Addr().String(), false, nil, nil)

	waf, err := middleware.NewWAF()
	if err != nil {
		t.Fatalf("NewWAF enforcement: %v", err)
	}
	proxy := newProxyServerWithDashboardAuthAndOptions(tbl, &handler.DashboardAuth{
		Domain: "dashboard.example.com",
	},
		handler.WithWAF(waf),
		handler.WithDashboardAccessMode(handler.DashboardAccessModeWAFAndLogin),
		handler.WithDashboardWAFConfig(handler.DashboardWAFConfig{Mode: middleware.WAFModeEnforcement, CustomRules: []string{wafRule}}),
	)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodPut, proxy.URL+"/api/deployments/dashboard-waf-trigger", strings.NewReader(`{"name":"ok"}`))
	req.Host = "dashboard.example.com"
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/deployments/dashboard-waf-trigger: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestProxy_DashboardDomainIPAllowlistBlocksNonMatchingClient(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("dashboard.example.com", backend.Listener.Addr().String(), false, nil, nil)

	ipFilter, err := middleware.NewIPFilter(nil, nil)
	if err != nil {
		t.Fatalf("NewIPFilter: %v", err)
	}

	proxy := newProxyServerWithDashboardAuthAndOptions(tbl, &handler.DashboardAuth{Domain: "dashboard.example.com"},
		handler.WithIPFilter(ipFilter),
		handler.WithDashboardWAFConfig(handler.DashboardWAFConfig{IPAllowlist: []string{"203.0.113.0/24"}}),
	)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "dashboard.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestProxy_DashboardDomainBypassesJWTGate(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("dashboard.example.com", backend.Listener.Addr().String(), false, nil, nil)

	proxy := newProxyServerWithDashboardAuthAndOptions(tbl, &handler.DashboardAuth{
		Domain: "dashboard.example.com",
	}, handler.WithJWTSecret([]byte("secret")))
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/login", nil)
	req.Host = "dashboard.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestProxy_NonDashboardDomainRedirectsToDashboardLoginWithoutJWT(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("app.example.com", backend.Listener.Addr().String(), false, nil, nil)

	proxy := newProxyServerWithDashboardAuthAndOptions(tbl, &handler.DashboardAuth{
		Domain: "dashboard.example.com",
	}, handler.WithJWTSecret([]byte("secret")), handler.WithAuthCookieDomain("example.com"))
	defer proxy.Close()

	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "app.example.com"

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("want 302, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "https://dashboard.example.com/login?redirect=http%3A%2F%2Fapp.example.com%2F" {
		t.Fatalf("want dashboard login redirect, got %q", location)
	}
}

func TestProxy_NonDashboardDomainReturnsUnauthorizedWithoutJWTWhenSharedCookieDisabled(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("app.example.com", backend.Listener.Addr().String(), false, nil, nil)

	proxy := newProxyServerWithDashboardAuthAndOptions(tbl, &handler.DashboardAuth{
		Domain: "dashboard.example.com",
	}, handler.WithJWTSecret([]byte("secret")))
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "app.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestProxy_NonDashboardDomainDoesNotRequireBasicAuth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("app.example.com", backend.Listener.Addr().String(), false, nil, nil)

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
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

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
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

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
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

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
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

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

func TestInternalTraffic_ReportsBlockedIPsAndCounters(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

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
	blockedResp, err := http.DefaultClient.Do(blocked)
	if err != nil {
		t.Fatalf("GET / blocked request: %v", err)
	}
	blockedResp.Body.Close()

	trafficResp, err := http.Get(proxy.URL + "/internal/traffic")
	if err != nil {
		t.Fatalf("GET /internal/traffic: %v", err)
	}
	defer trafficResp.Body.Close()

	if trafficResp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", trafficResp.StatusCode)
	}

	var body struct {
		TotalRequests      int64 `json:"totalRequests"`
		SuspiciousRequests int64 `json:"suspiciousRequests"`
		BlockedRequests    int64 `json:"blockedRequests"`
		ActiveBlockedIPs   int   `json:"activeBlockedIps"`
		BlockedIPs         []struct {
			IP           string     `json:"ip"`
			BlockedUntil *time.Time `json:"blockedUntil"`
		} `json:"blockedIps"`
	}

	if err := json.NewDecoder(trafficResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode traffic response: %v", err)
	}

	if body.TotalRequests != 13 {
		t.Fatalf("want total requests 13, got %d", body.TotalRequests)
	}
	if body.SuspiciousRequests != 12 {
		t.Fatalf("want suspicious requests 12, got %d", body.SuspiciousRequests)
	}
	if body.BlockedRequests != 1 {
		t.Fatalf("want blocked requests 1, got %d", body.BlockedRequests)
	}
	if body.ActiveBlockedIPs != 1 {
		t.Fatalf("want active blocked ips 1, got %d", body.ActiveBlockedIPs)
	}
	if len(body.BlockedIPs) != 1 || body.BlockedIPs[0].IP != "203.0.113.7" {
		t.Fatalf("want blocked ip 203.0.113.7, got %+v", body.BlockedIPs)
	}
	if body.BlockedIPs[0].BlockedUntil == nil || body.BlockedIPs[0].BlockedUntil.IsZero() {
		t.Fatal("want blockedUntil for blocked ip")
	}
}

func TestProxy_WritesAccessLogWithWhitelistedHeaders(t *testing.T) {
	dir := t.TempDir()
	logger, err := handler.NewFileAccessLogger(handler.AccessLogConfig{
		Dir:             dir,
		Retention:       24 * time.Hour,
		WhitelistedKeys: []string{"host", "user-agent", "x-forwarded-for"},
	})
	if err != nil {
		t.Fatalf("NewFileAccessLogger: %v", err)
	}
	defer logger.Close()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

	proxy := newProxyServerWithOptions(tbl, handler.WithAccessLogger(logger))
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/v1/health?deep=1", nil)
	req.Host = "example.com"
	req.Header.Set("User-Agent", "lotsen-test")
	req.Header.Set("Authorization", "secret-token")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("proxy request: %v", err)
	}
	resp.Body.Close()

	files, err := filepath.Glob(filepath.Join(dir, "access-*.log"))
	if err != nil {
		t.Fatalf("glob access logs: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 access log file, got %d", len(files))
	}

	raw, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read access log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want exactly one log line, got %d", len(lines))
	}

	var entry struct {
		Host    string            `json:"host"`
		Method  string            `json:"method"`
		Path    string            `json:"path"`
		Query   string            `json:"query"`
		Status  int               `json:"status"`
		Outcome string            `json:"outcome"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("unmarshal access log line: %v", err)
	}

	if entry.Host != "example.com" {
		t.Fatalf("want host example.com, got %q", entry.Host)
	}
	if entry.Method != http.MethodGet {
		t.Fatalf("want method GET, got %q", entry.Method)
	}
	if entry.Path != "/v1/health" {
		t.Fatalf("want path /v1/health, got %q", entry.Path)
	}
	if entry.Query != "deep=1" {
		t.Fatalf("want query deep=1, got %q", entry.Query)
	}
	if entry.Status != http.StatusCreated {
		t.Fatalf("want status 201, got %d", entry.Status)
	}
	if entry.Outcome != "proxied" {
		t.Fatalf("want outcome proxied, got %q", entry.Outcome)
	}
	if _, ok := entry.Headers["authorization"]; ok {
		t.Fatal("authorization header must not be logged")
	}
	if entry.Headers["user-agent"] != "lotsen-test" {
		t.Fatalf("want user-agent header, got %q", entry.Headers["user-agent"])
	}
}

func TestProxy_DeploymentBasicAuthRequiresCredentials(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}

	tbl := newTestTable()
	tbl.Set("private.example.com", backend.Listener.Addr().String(), false, &store.BasicAuthConfig{Users: []store.BasicAuthUser{{
		Username: "admin",
		Password: string(hash),
	}}}, nil)

	proxy := newProxyServer(tbl)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "private.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); got != `Basic realm="Lotsen"` {
		t.Fatalf("want Basic realm challenge, got %q", got)
	}
}

func TestProxy_DeploymentBasicAuthWithValidCredentials(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}

	tbl := newTestTable()
	tbl.Set("private.example.com", backend.Listener.Addr().String(), false, &store.BasicAuthConfig{Users: []store.BasicAuthUser{{
		Username: "admin",
		Password: string(hash),
	}}}, nil)

	proxy := newProxyServer(tbl)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "private.example.com"
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

func TestProxy_GlobalIPDenylistBlocksRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

	ipFilter, err := middleware.NewIPFilter([]string{"203.0.113.0/24"}, nil)
	if err != nil {
		t.Fatalf("NewIPFilter: %v", err)
	}
	proxy := newProxyServerWithOptions(tbl, handler.WithIPFilter(ipFilter))
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "example.com"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestProxy_UABlockingIncrementsTrafficCounter(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tbl := newTestTable()
	tbl.Set("example.com", backend.Listener.Addr().String(), false, nil, nil)

	proxy := newProxyServerWithOptions(tbl, handler.WithUAFilter(middleware.NewUAFilter(false, nil)))
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/", nil)
	req.Host = "example.com"
	req.Header.Set("User-Agent", "sqlmap/1.8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()

	trafficResp, err := http.Get(proxy.URL + "/internal/traffic")
	if err != nil {
		t.Fatalf("GET /internal/traffic: %v", err)
	}
	defer trafficResp.Body.Close()

	var body struct {
		UABlockedRequests int64 `json:"uaBlockedRequests"`
	}
	if err := json.NewDecoder(trafficResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode traffic response: %v", err)
	}
	if body.UABlockedRequests != 1 {
		t.Fatalf("want ua blocked requests 1, got %d", body.UABlockedRequests)
	}
}

func TestProxy_WAFDetectionAndEnforcement(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	wafRule := `SecRule REQUEST_URI "@contains waf-trigger" "id:10099,phase:1,deny,status:403,log,msg:'waf trigger'"`

	newRoute := func() *store.SecurityConfig {
		return &store.SecurityConfig{WAFEnabled: true, WAFMode: "detection", CustomRules: []string{wafRule}}
	}

	// detection mode should not block but should mark request as detected.
	tblDetection := newTestTable()
	tblDetection.Set("example.com", backend.Listener.Addr().String(), false, nil, newRoute())
	wafDetection, err := middleware.NewWAF()
	if err != nil {
		t.Fatalf("NewWAF detection: %v", err)
	}
	proxyDetection := newProxyServerWithOptions(tblDetection, handler.WithWAF(wafDetection))
	defer proxyDetection.Close()

	req, _ := http.NewRequest(http.MethodGet, proxyDetection.URL+"/waf-trigger", nil)
	req.Host = "example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /waf-trigger detection: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detection mode: want 200, got %d", resp.StatusCode)
	}

	// enforcement mode should block and increment waf blocked counter.
	tblEnforcement := newTestTable()
	tblEnforcement.Set("example.com", backend.Listener.Addr().String(), false, nil, &store.SecurityConfig{WAFEnabled: true, WAFMode: "enforcement", CustomRules: []string{wafRule}})
	wafEnforcement, err := middleware.NewWAF()
	if err != nil {
		t.Fatalf("NewWAF enforcement: %v", err)
	}
	proxyEnforcement := newProxyServerWithOptions(tblEnforcement, handler.WithWAF(wafEnforcement))
	defer proxyEnforcement.Close()

	req, _ = http.NewRequest(http.MethodGet, proxyEnforcement.URL+"/waf-trigger", nil)
	req.Host = "example.com"
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /waf-trigger enforcement: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("enforcement mode: want 403, got %d", resp.StatusCode)
	}

	trafficResp, err := http.Get(proxyEnforcement.URL + "/internal/traffic")
	if err != nil {
		t.Fatalf("GET /internal/traffic: %v", err)
	}
	defer trafficResp.Body.Close()

	var body struct {
		WAFBlockedRequests int64 `json:"wafBlockedRequests"`
	}
	if err := json.NewDecoder(trafficResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode traffic response: %v", err)
	}
	if body.WAFBlockedRequests != 1 {
		t.Fatalf("want waf blocked requests 1, got %d", body.WAFBlockedRequests)
	}
}
