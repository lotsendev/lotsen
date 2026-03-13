package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/crypto/acme/autocert"

	"github.com/lotsendev/lotsen/proxy/internal/handler"
	"github.com/lotsendev/lotsen/proxy/internal/middleware"
	"github.com/lotsendev/lotsen/proxy/internal/routing"
	"github.com/lotsendev/lotsen/store"
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

func (t *tableStub) Set(domain, upstream string, public bool, basicAuth *store.BasicAuthConfig, security *store.SecurityConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routes[domain] = routing.Route{Upstream: upstream, BasicAuth: basicAuth, Security: security}
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
	tbl.Set("example.com", "localhost:3000", false, nil, nil)
	policy := hostPolicyFromTable(tbl)

	if err := policy(context.Background(), "example.com"); err != nil {
		t.Fatalf("want host allowed, got %v", err)
	}
	if err := policy(context.Background(), "missing.example.com"); err == nil {
		t.Fatal("want missing host rejected")
	}
}

func TestNewAutocertManager_UsesDirectoryAndEmail(t *testing.T) {
	mgr := newAutocertManager("/tmp/lotsen-certs-test", "ops@example.com", letsencryptStagingDirectoryURL, autocert.HostWhitelist("example.com"))

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

func TestDashboardAuthFromEnv_DomainWithoutCredentialsSucceeds(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_DOMAIN", "dashboard.example.com")
	t.Setenv("LOTSEN_DASHBOARD_USER", "")
	t.Setenv("LOTSEN_DASHBOARD_PASSWORD", "")

	auth, err := dashboardAuthFromEnv()
	if err != nil {
		t.Fatalf("dashboardAuthFromEnv: %v", err)
	}
	if auth == nil {
		t.Fatal("want dashboard domain config")
	}
}

func TestDashboardAuthFromEnv_ReturnsNormalizedAuth(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_DOMAIN", "Dashboard.Example.com.")
	t.Setenv("LOTSEN_DASHBOARD_USER", "admin")
	t.Setenv("LOTSEN_DASHBOARD_PASSWORD", "secret")

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
	if auth.Username != "" || auth.Password != "" {
		t.Fatal("want dashboard basic auth credentials ignored")
	}
}

func TestDashboardAuthFromEnv_IgnoresCredentialsWithoutDomain(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_DOMAIN", "")
	t.Setenv("LOTSEN_DASHBOARD_USER", "admin")
	t.Setenv("LOTSEN_DASHBOARD_PASSWORD", "secret")

	auth, err := dashboardAuthFromEnv()
	if err != nil {
		t.Fatalf("dashboardAuthFromEnv: %v", err)
	}
	if auth != nil {
		t.Fatal("want nil config when dashboard domain is unset")
	}
}

func TestHardeningProfileFromEnv_DefaultsToStandard(t *testing.T) {
	t.Setenv("LOTSEN_PROXY_HARDENING_PROFILE", "")

	profile, err := hardeningProfileFromEnv()
	if err != nil {
		t.Fatalf("hardeningProfileFromEnv: %v", err)
	}
	if profile != handler.HardeningStandard {
		t.Fatalf("want profile %s, got %s", handler.HardeningStandard, profile)
	}
}

func TestHardeningProfileFromEnv_AcceptsStrict(t *testing.T) {
	t.Setenv("LOTSEN_PROXY_HARDENING_PROFILE", " STRICT ")

	profile, err := hardeningProfileFromEnv()
	if err != nil {
		t.Fatalf("hardeningProfileFromEnv: %v", err)
	}
	if profile != handler.HardeningStrict {
		t.Fatalf("want profile %s, got %s", handler.HardeningStrict, profile)
	}
}

func TestHardeningProfileFromEnv_RejectsInvalidValue(t *testing.T) {
	t.Setenv("LOTSEN_PROXY_HARDENING_PROFILE", "aggressive")

	if _, err := hardeningProfileFromEnv(); err == nil {
		t.Fatal("want validation error for invalid profile")
	}
}

func TestAuthCookieDomainFromEnv_Empty(t *testing.T) {
	t.Setenv("LOTSEN_AUTH_COOKIE_DOMAIN", "")

	domain, err := authCookieDomainFromEnv()
	if err != nil {
		t.Fatalf("authCookieDomainFromEnv: %v", err)
	}
	if domain != "" {
		t.Fatalf("want empty domain, got %q", domain)
	}
}

func TestAuthCookieDomainFromEnv_NormalizesLeadingDot(t *testing.T) {
	t.Setenv("LOTSEN_AUTH_COOKIE_DOMAIN", ".D0001.Erca.Dev")

	domain, err := authCookieDomainFromEnv()
	if err != nil {
		t.Fatalf("authCookieDomainFromEnv: %v", err)
	}
	if domain != "d0001.erca.dev" {
		t.Fatalf("want d0001.erca.dev, got %q", domain)
	}
}

func TestAuthCookieDomainFromEnv_RejectsInvalid(t *testing.T) {
	t.Setenv("LOTSEN_AUTH_COOKIE_DOMAIN", "localhost")

	if _, err := authCookieDomainFromEnv(); err == nil {
		t.Fatal("want validation error for invalid cookie domain")
	}
}

func TestDashboardAccessModeFromEnv_DefaultsToLoginOnly(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_ACCESS_MODE", "")

	mode, err := dashboardAccessModeFromEnv()
	if err != nil {
		t.Fatalf("dashboardAccessModeFromEnv: %v", err)
	}
	if mode != handler.DashboardAccessModeLoginOnly {
		t.Fatalf("want %q, got %q", handler.DashboardAccessModeLoginOnly, mode)
	}
}

func TestDashboardAccessModeFromEnv_RejectsInvalidValue(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_ACCESS_MODE", "strict")

	if _, err := dashboardAccessModeFromEnv(); err == nil {
		t.Fatal("want validation error for invalid dashboard access mode")
	}
}

func TestDashboardAccessModeResolver_UsesHostProfileOverride(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "host_profile.json")
	if err := os.WriteFile(profilePath, []byte(`{"displayName":"prod","dashboardAccessMode":"waf_and_login"}`), 0o644); err != nil {
		t.Fatalf("write host profile: %v", err)
	}

	resolve := dashboardAccessModeResolver(profilePath, handler.DashboardAccessModeLoginOnly)
	if got := resolve(); got != handler.DashboardAccessModeWAFAndLogin {
		t.Fatalf("want %q, got %q", handler.DashboardAccessModeWAFAndLogin, got)
	}
}

func TestDashboardWAFConfigFromEnv_DefaultsToDetection(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_WAF_MODE", "")
	t.Setenv("LOTSEN_DASHBOARD_WAF_RULES", "")

	cfg, err := dashboardWAFConfigFromEnv()
	if err != nil {
		t.Fatalf("dashboardWAFConfigFromEnv: %v", err)
	}
	if cfg.Mode != middleware.WAFModeDetection {
		t.Fatalf("want mode detection, got %q", cfg.Mode)
	}
	if len(cfg.CustomRules) != 0 {
		t.Fatalf("want no rules, got %d", len(cfg.CustomRules))
	}
}

func TestDashboardWAFConfigResolver_UsesHostProfileOverride(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "host_profile.json")
	if err := os.WriteFile(profilePath, []byte(`{"dashboardWaf":{"mode":"enforcement","ipAllowlist":["203.0.113.0/24"],"customRules":["SecRule REQUEST_URI \"@contains block\" \"id:10001,phase:1,deny,status:403\""]}}`), 0o644); err != nil {
		t.Fatalf("write host profile: %v", err)
	}

	fallback := handler.DashboardWAFConfig{Mode: middleware.WAFModeDetection}
	resolve := dashboardWAFConfigResolver(profilePath, fallback)
	got := resolve()

	if got.Mode != middleware.WAFModeEnforcement {
		t.Fatalf("want mode enforcement, got %q", got.Mode)
	}
	if len(got.CustomRules) != 1 {
		t.Fatalf("want 1 custom rule, got %d", len(got.CustomRules))
	}
	if len(got.IPAllowlist) != 1 || got.IPAllowlist[0] != "203.0.113.0/24" {
		t.Fatalf("want ip allowlist [203.0.113.0/24], got %#v", got.IPAllowlist)
	}
}
