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
)

type tableStub struct {
	mu     sync.RWMutex
	routes map[string]string
}

func newTableStub() *tableStub {
	return &tableStub{routes: make(map[string]string)}
}

func (t *tableStub) Get(domain string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	u, ok := t.routes[domain]
	return u, ok
}

func (t *tableStub) Set(domain, upstream string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routes[domain] = upstream
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
	h := handler.New(newTableStub())
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
	h := handler.New(tbl)
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
	tbl.Set("example.com", "localhost:3000")
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
