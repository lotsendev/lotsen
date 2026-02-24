package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ercadev/dirigent/proxy/internal/middleware"
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

	middleware.WriteBasicAuthChallenge(w, "Dirigent")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
	if got := w.Header().Get("WWW-Authenticate"); got != `Basic realm="Dirigent"` {
		t.Fatalf("want Basic realm challenge, got %q", got)
	}
}
