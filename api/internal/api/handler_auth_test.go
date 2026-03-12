package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/ercadev/dirigent/auth"
	"github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/internal/events"
)

// stubAuthStore is an in-memory AuthUserStore for tests.
type stubAuthStore struct {
	mu            sync.RWMutex
	users         map[string]struct{}
	listUsersErr  error
	createUserErr error
	deleteUserErr error
	hasAnyUser    bool
}

func (s *stubAuthStore) HasAnyUser() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users) > 0 || s.hasAnyUser, nil
}

func (s *stubAuthStore) CreateUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.createUserErr != nil {
		return s.createUserErr
	}
	if _, exists := s.users[username]; exists {
		return auth.ErrUserExists
	}
	s.users[username] = struct{}{}
	return nil
}

func (s *stubAuthStore) ListUsers() ([]auth.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.listUsersErr != nil {
		return nil, s.listUsersErr
	}

	usernames := make([]string, 0, len(s.users))
	for username := range s.users {
		usernames = append(usernames, username)
	}
	sort.Strings(usernames)

	users := make([]auth.User, 0, len(usernames))
	for _, username := range usernames {
		users = append(users, auth.User{Username: username})
	}
	return users, nil
}

func (s *stubAuthStore) DeleteUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.deleteUserErr != nil {
		return s.deleteUserErr
	}
	if _, exists := s.users[username]; !exists {
		return auth.ErrUserNotFound
	}
	delete(s.users, username)
	return nil
}

// WebAuthn / passkey stubs (no-op for basic auth tests).

func (s *stubAuthStore) GetWebAuthnUser(_ string) (*auth.WebAuthnUser, error) {
	return nil, auth.ErrUserNotFound
}

func (s *stubAuthStore) SavePasskey(_ string, _ *webauthn.Credential, _ string) error {
	return nil
}

func (s *stubAuthStore) ListPasskeys(_ string) ([]auth.PasskeyInfo, error) {
	return nil, nil
}

func (s *stubAuthStore) DeletePasskey(_, _ string) error {
	return nil
}

func (s *stubAuthStore) UpdatePasskeySignCount(_ []byte, _ uint32) error {
	return nil
}

func (s *stubAuthStore) CreateInviteToken(_ string, _ time.Time) error {
	return nil
}

func (s *stubAuthStore) ValidateInviteToken(_ string) error {
	return errors.New("not found")
}

func (s *stubAuthStore) ConsumeInviteToken(_ string) error {
	return nil
}

var testJWTSecret = []byte("test-secret-key")

func newStubAuthStore(usernames ...string) *stubAuthStore {
	users := make(map[string]struct{}, len(usernames))
	for _, u := range usernames {
		users[u] = struct{}{}
	}
	return &stubAuthStore{users: users}
}

func newAuthTestServer(store api.Store, authStore api.AuthUserStore) *httptest.Server {
	mux := http.NewServeMux()
	h := api.New(store, events.NewBroker(), noopDockerLogs{})
	h.SetAuth(authStore, testJWTSecret)
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func newAuthTestServerWithCookieDomain(store api.Store, authStore api.AuthUserStore, domain string) *httptest.Server {
	mux := http.NewServeMux()
	h := api.New(store, events.NewBroker(), noopDockerLogs{})
	h.SetAuth(authStore, testJWTSecret)
	h.SetAuthCookieDomain(domain)
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func newAuthTestServerWithDashboardAccessMode(store api.Store, authStore api.AuthUserStore, mode api.DashboardAccessMode) *httptest.Server {
	mux := http.NewServeMux()
	h := api.New(store, events.NewBroker(), noopDockerLogs{})
	h.SetAuth(authStore, testJWTSecret)
	h.SetDashboardAccessMode(mode)
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func TestLogout_ClearsCookie(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser"))
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
		if c.Name == "lotsen_token" && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("want lotsen_token cookie cleared (MaxAge<0)")
	}
}

func TestLogout_ClearsCookieWithConfiguredDomain(t *testing.T) {
	srv := newAuthTestServerWithCookieDomain(newMemStore(), newStubAuthStore("testuser"), "example.com")
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /auth/logout: %v", err)
	}
	defer resp.Body.Close()

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "lotsen_token" {
			found = true
			if c.Domain != "example.com" {
				t.Fatalf("want cookie domain example.com, got %q", c.Domain)
			}
		}
	}
	if !found {
		t.Error("want lotsen_token cookie in response")
	}
}

func TestMe_ValidToken(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser"))
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "testuser", 24*365*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
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
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser"))
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
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser"))
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
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser"))
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "testuser", 24*365*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/deployments", nil)
	req.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200 with valid token, got %d", resp.StatusCode)
	}
}

func TestMe_WAFOnlyModeReturnsDisabled(t *testing.T) {
	srv := newAuthTestServerWithDashboardAccessMode(newMemStore(), newStubAuthStore("testuser"), api.DashboardAccessModeWAFOnly)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/auth/me")
	if err != nil {
		t.Fatalf("GET /auth/me: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", resp.StatusCode)
	}
}

func TestProtectedRoute_WAFOnlyModeBypassesAuth(t *testing.T) {
	srv := newAuthTestServerWithDashboardAccessMode(newMemStore(), newStubAuthStore("testuser"), api.DashboardAccessModeWAFOnly)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments")
	if err != nil {
		t.Fatalf("GET /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestProtectedRoute_OpenWhenNoAuthConfigured(t *testing.T) {
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

func TestSetupAvailable_NoUsers(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), newStubAuthStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/auth/setup-available")
	if err != nil {
		t.Fatalf("GET /auth/setup-available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Available bool `json:"available"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Available {
		t.Error("want available=true when no users exist")
	}
}

func TestSetupAvailable_HasUsers(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("admin"))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/auth/setup-available")
	if err != nil {
		t.Fatalf("GET /auth/setup-available: %v", err)
	}
	defer resp.Body.Close()

	var body struct {
		Available bool `json:"available"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Available {
		t.Error("want available=false when users exist")
	}
}

func TestUsers_List(t *testing.T) {
	store := newStubAuthStore("admin", "alice", "bob")
	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/users: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body struct {
		Users []struct {
			Username string `json:"username"`
		} `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body.Users) != 3 {
		t.Fatalf("want 3 users, got %d", len(body.Users))
	}
}

func TestUsers_CreateAndDelete(t *testing.T) {
	store := newStubAuthStore("admin")
	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	createBody, _ := json.Marshal(map[string]string{"username": "new-user"})
	createReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/users", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("POST /api/users: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", createResp.StatusCode)
	}

	deleteReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/users/new-user", nil)
	deleteReq.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("DELETE /api/users/new-user: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", deleteResp.StatusCode)
	}
}

func TestUsers_CreateDuplicateConflict(t *testing.T) {
	store := newStubAuthStore("admin", "alice")
	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"username": "alice"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/users: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", resp.StatusCode)
	}
}

func TestUsers_List_StoreError(t *testing.T) {
	store := newStubAuthStore("admin")
	store.listUsersErr = errors.New("db down")
	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/users: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestUsers_Create_StoreError(t *testing.T) {
	store := newStubAuthStore("admin")
	store.createUserErr = errors.New("db down")
	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"username": "alice"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/users: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestUsers_Delete_StoreError(t *testing.T) {
	store := newStubAuthStore("admin", "alice")
	store.deleteUserErr = errors.New("db down")
	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/users/alice", nil)
	req.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/users/alice: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}
