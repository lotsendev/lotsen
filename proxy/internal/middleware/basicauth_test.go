package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lotsendev/lotsen/proxy/internal/middleware"
	"github.com/lotsendev/lotsen/store"
	"golang.org/x/crypto/bcrypt"
)

func TestValidBasicAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "secret")

	if !middleware.ValidBasicAuth(req, "admin", "secret") {
		t.Fatal("want credentials to be valid")
	}
}

func TestValidBasicAuth_InvalidCredentials(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "wrong")

	if middleware.ValidBasicAuth(req, "admin", "secret") {
		t.Fatal("want credentials to be invalid")
	}
}

func TestWriteBasicAuthChallenge(t *testing.T) {
	w := httptest.NewRecorder()

	middleware.WriteBasicAuthChallenge(w, "Lotsen")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
	if got := w.Header().Get("WWW-Authenticate"); got != `Basic realm="Lotsen"` {
		t.Fatalf("want Basic realm challenge, got %q", got)
	}
}

func TestValidBasicAuthUsers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "secret")
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}
	auth := &store.BasicAuthConfig{Users: []store.BasicAuthUser{{Username: "admin", Password: string(hash)}}}
	if !middleware.ValidBasicAuthUsers(req, auth) {
		t.Fatal("want credentials to be accepted")
	}
}

func TestValidBasicAuthUsers_Invalid(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "wrong")
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}
	auth := &store.BasicAuthConfig{Users: []store.BasicAuthUser{{Username: "admin", Password: string(hash)}}}
	if middleware.ValidBasicAuthUsers(req, auth) {
		t.Fatal("want credentials to be rejected")
	}
}
