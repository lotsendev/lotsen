package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"golang.org/x/crypto/acme/autocert"

	"github.com/ercadev/dirigent/proxy/internal/handler"
	"github.com/ercadev/dirigent/proxy/internal/routing"
	"github.com/ercadev/dirigent/store"
)

type tableStub struct {
	mu     sync.RWMutex
	routes map[string]routing.Route
}

func newTableStub() *tableStub {
	return &tableStub{routes: make(map[string]routing.Route)}
}

func (t *tableStub) Get(domain string) (routing.Route, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	route, ok := t.routes[domain]
	return route, ok
}

func (t *tableStub) Set(domain, upstream string, basicAuth *store.BasicAuthConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routes[domain] = routing.Route{Upstream: upstream, BasicAuth: basicAuth}
}

func TestRedirectToHTTPS_DefaultPort(t *testing.T) {
	h := redirectToHTTPS(":443")

	r := httptest.NewRequest(http.MethodGet, "http://example.com/path?q=1", nil)
	r.Host = "example.com"
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("want 301, got %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != "https://example.com/path?q=1" {
		t.Fatalf("want Location https://example.com/path?q=1, got %s", got)
	}
}

func TestRedirectToHTTPS_CustomPort(t *testing.T) {
	h := redirectToHTTPS(":8443")

	r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	r.Host = "example.com:80"
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if got := w.Header().Get("Location"); got != "https://example.com:8443/" {
		t.Fatalf("want Location https://example.com:8443/, got %s", got)
	}
}

func TestHTTPMux_InternalHealthNotRedirected(t *testing.T) {
	h := handler.New(newTableStub(), nil)
	mux := newHTTPMux(h, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com/", http.StatusMovedPermanently)
	}))

	r := httptest.NewRequest(http.MethodGet, "/internal/health", nil)
	r.Host = "localhost"
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestHTTPMux_InternalRoutesCanBeUpdated(t *testing.T) {
	tbl := newTableStub()
	h := handler.New(tbl, nil)
	mux := newHTTPMux(h, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com/", http.StatusMovedPermanently)
	}))

	r := httptest.NewRequest(http.MethodPost, "/internal/routes", bytes.NewBufferString(`{"domain":"Example.com","upstream":"localhost:3000"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}
	if _, ok := tbl.Get("example.com"); !ok {
		t.Fatal("want normalized route to be registered")
	}
}

func TestHostPolicyFromTable(t *testing.T) {
	tbl := newTableStub()
	tbl.Set("example.com", "localhost:3000", nil)
	policy := hostPolicyFromTable(tbl)

	if err := policy(context.Background(), "example.com"); err != nil {
		t.Fatalf("want host allowed, got %v", err)
	}
	if err := policy(context.Background(), "missing.example.com"); err == nil {
		t.Fatal("want missing host rejected")
	}
}

func TestNewAutocertManager_UsesDirectoryAndEmail(t *testing.T) {
	mgr := newAutocertManager("/tmp/dirigent-certs-test", "ops@example.com", letsencryptStagingDirectoryURL, autocert.HostWhitelist("example.com"))

	if mgr.Email != "ops@example.com" {
		t.Fatalf("want email ops@example.com, got %s", mgr.Email)
	}
	if mgr.Client == nil {
		t.Fatal("want acme client configured")
	}
	if mgr.Client.DirectoryURL != letsencryptStagingDirectoryURL {
		t.Fatalf("want directory %s, got %s", letsencryptStagingDirectoryURL, mgr.Client.DirectoryURL)
	}
}

func TestDashboardAuthFromEnv_DomainWithoutCredentialsFails(t *testing.T) {
	t.Setenv("DIRIGENT_DASHBOARD_DOMAIN", "dashboard.example.com")
	t.Setenv("DIRIGENT_DASHBOARD_USER", "")
	t.Setenv("DIRIGENT_DASHBOARD_PASSWORD", "")

	_, err := dashboardAuthFromEnv()
	if err == nil {
		t.Fatal("want validation error when dashboard credentials are missing")
	}
}

func TestDashboardAuthFromEnv_ReturnsNormalizedAuth(t *testing.T) {
	t.Setenv("DIRIGENT_DASHBOARD_DOMAIN", "Dashboard.Example.com.")
	t.Setenv("DIRIGENT_DASHBOARD_USER", "admin")
	t.Setenv("DIRIGENT_DASHBOARD_PASSWORD", "secret")

	auth, err := dashboardAuthFromEnv()
	if err != nil {
		t.Fatalf("dashboardAuthFromEnv: %v", err)
	}
	if auth == nil {
		t.Fatal("want dashboard auth config")
	}
	if auth.Domain != "dashboard.example.com" {
		t.Fatalf("want normalized domain dashboard.example.com, got %s", auth.Domain)
	}
	if auth.Username != "admin" || auth.Password != "secret" {
		t.Fatal("want configured credentials")
	}
}

func TestDashboardAuthFromEnv_IgnoresCredentialsWithoutDomain(t *testing.T) {
	t.Setenv("DIRIGENT_DASHBOARD_DOMAIN", "")
	t.Setenv("DIRIGENT_DASHBOARD_USER", "admin")
	t.Setenv("DIRIGENT_DASHBOARD_PASSWORD", "secret")

	auth, err := dashboardAuthFromEnv()
	if err != nil {
		t.Fatalf("dashboardAuthFromEnv: %v", err)
	}
	if auth != nil {
		t.Fatal("want nil config when dashboard domain is unset")
	}
}

func TestHardeningProfileFromEnv_DefaultsToStandard(t *testing.T) {
	t.Setenv("DIRIGENT_PROXY_HARDENING_PROFILE", "")

	profile, err := hardeningProfileFromEnv()
	if err != nil {
		t.Fatalf("hardeningProfileFromEnv: %v", err)
	}
	if profile != handler.HardeningStandard {
		t.Fatalf("want profile %s, got %s", handler.HardeningStandard, profile)
	}
}

func TestHardeningProfileFromEnv_AcceptsStrict(t *testing.T) {
	t.Setenv("DIRIGENT_PROXY_HARDENING_PROFILE", " STRICT ")

	profile, err := hardeningProfileFromEnv()
	if err != nil {
		t.Fatalf("hardeningProfileFromEnv: %v", err)
	}
	if profile != handler.HardeningStrict {
		t.Fatalf("want profile %s, got %s", handler.HardeningStrict, profile)
	}
}

func TestHardeningProfileFromEnv_RejectsInvalidValue(t *testing.T) {
	t.Setenv("DIRIGENT_PROXY_HARDENING_PROFILE", "aggressive")

	if _, err := hardeningProfileFromEnv(); err == nil {
		t.Fatal("want validation error for invalid profile")
	}
}
