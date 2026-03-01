package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ercadev/dirigent/auth"
	"github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/internal/events"
)

// stubAuthStore is an in-memory AuthUserStore for tests.
type stubAuthStore struct {
	password string // expected password for the single test user "testuser"
	err      error  // if non-nil, returned from Authenticate regardless of input
}

func (s *stubAuthStore) Authenticate(username, password string) error {
	if s.err != nil {
		return s.err
	}
	if username == "testuser" && password == s.password {
		return nil
	}
	return auth.ErrInvalidCredentials
}

var testJWTSecret = []byte("test-secret-key")

func newAuthTestServer(store api.Store, authStore api.AuthUserStore) *httptest.Server {
	mux := http.NewServeMux()
	h := api.New(store, events.NewBroker(), noopDockerLogs{})
	h.SetAuth(authStore, testJWTSecret)
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func TestLogin_ValidCredentials(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), &stubAuthStore{password: "correctpass"})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"username": "testuser", "password": "correctpass"})
	resp, err := http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /auth/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "dirigent_token" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("want dirigent_token cookie in response")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), &stubAuthStore{password: "correct"})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"username": "testuser", "password": "wrong"})
	resp, err := http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /auth/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), &stubAuthStore{password: "pass"})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"username": "testuser"})
	resp, err := http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /auth/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestLogin_StoreError(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), &stubAuthStore{err: errors.New("db error")})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"username": "testuser", "password": "pass"})
	resp, err := http.Post(srv.URL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /auth/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", resp.StatusCode)
	}
}

func TestLogout_ClearsCookie(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), &stubAuthStore{password: "pass"})
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /auth/logout: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("want 204, got %d", resp.StatusCode)
	}

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "dirigent_token" && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("want dirigent_token cookie cleared (MaxAge<0)")
	}
}

func TestMe_ValidToken(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), &stubAuthStore{password: "pass"})
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "testuser", 24*365*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "dirigent_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /auth/me: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestMe_NoToken(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), &stubAuthStore{password: "pass"})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/auth/me")
	if err != nil {
		t.Fatalf("GET /auth/me: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestProtectedRoute_RequiresToken(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), &stubAuthStore{password: "pass"})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments")
	if err != nil {
		t.Fatalf("GET /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401 without token, got %d", resp.StatusCode)
	}
}

func TestProtectedRoute_AllowsWithToken(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), &stubAuthStore{password: "pass"})
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "testuser", 24*365*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/deployments", nil)
	req.AddCookie(&http.Cookie{Name: "dirigent_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200 with valid token, got %d", resp.StatusCode)
	}
}

func TestProtectedRoute_OpenWhenNoAuthConfigured(t *testing.T) {
	// When auth is not configured, all routes are open.
	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments")
	if err != nil {
		t.Fatalf("GET /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200 without auth configured, got %d", resp.StatusCode)
	}
}
