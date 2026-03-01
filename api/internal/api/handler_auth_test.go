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

	"github.com/ercadev/dirigent/auth"
	"github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/internal/events"
)

// stubAuthStore is an in-memory AuthUserStore for tests.
type stubAuthStore struct {
	mu            sync.RWMutex
	users         map[string]string
	authErr       error
	listUsersErr  error
	createUserErr error
	updatePassErr error
	deleteUserErr error
}

func (s *stubAuthStore) Authenticate(username, password string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.authErr != nil {
		return s.authErr
	}
	if s.users[username] == password {
		return nil
	}
	return auth.ErrInvalidCredentials
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

func (s *stubAuthStore) CreateUser(username, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.createUserErr != nil {
		return s.createUserErr
	}

	if _, exists := s.users[username]; exists {
		return auth.ErrUserExists
	}
	s.users[username] = password
	return nil
}

func (s *stubAuthStore) UpdatePassword(username, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.updatePassErr != nil {
		return s.updatePassErr
	}

	if _, exists := s.users[username]; !exists {
		return auth.ErrUserNotFound
	}
	s.users[username] = password
	return nil
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

var testJWTSecret = []byte("test-secret-key")

func newStubAuthStore(username, password string) *stubAuthStore {
	users := map[string]string{}
	if username != "" {
		users[username] = password
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

func TestLogin_ValidCredentials(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser", "correctpass"))
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
		if c.Name == "lotsen_token" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("want lotsen_token cookie in response")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser", "correct"))
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
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser", "pass"))
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
	store := newStubAuthStore("testuser", "pass")
	store.authErr = errors.New("db error")
	srv := newAuthTestServer(newMemStore(), store)
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
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser", "pass"))
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

func TestMe_ValidToken(t *testing.T) {
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser", "pass"))
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
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser", "pass"))
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
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser", "pass"))
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
	srv := newAuthTestServer(newMemStore(), newStubAuthStore("testuser", "pass"))
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

func TestUsers_List(t *testing.T) {
	store := newStubAuthStore("admin", "pass")
	if err := store.CreateUser("alice", "alice-pass"); err != nil {
		t.Fatalf("CreateUser alice: %v", err)
	}
	if err := store.CreateUser("bob", "bob-pass"); err != nil {
		t.Fatalf("CreateUser bob: %v", err)
	}

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
	if body.Users[0].Username != "admin" || body.Users[1].Username != "alice" || body.Users[2].Username != "bob" {
		t.Fatalf("unexpected usernames: %#v", body.Users)
	}
}

func TestUsers_CreateAndDelete(t *testing.T) {
	store := newStubAuthStore("admin", "pass")
	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	createBody, _ := json.Marshal(map[string]string{"username": "new-user", "password": "new-pass"})
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

	if err := store.Authenticate("new-user", "new-pass"); err != nil {
		t.Fatalf("Authenticate(new-user): %v", err)
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

	err = store.Authenticate("new-user", "new-pass")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestUsers_CreateDuplicateConflict(t *testing.T) {
	store := newStubAuthStore("admin", "pass")
	if err := store.CreateUser("alice", "pass"); err != nil {
		t.Fatalf("CreateUser alice: %v", err)
	}

	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "new-pass"})
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

func TestUsers_UpdatePassword(t *testing.T) {
	store := newStubAuthStore("admin", "pass")
	if err := store.CreateUser("alice", "old-pass"); err != nil {
		t.Fatalf("CreateUser alice: %v", err)
	}

	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"password": "new-pass"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/users/alice/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/users/alice/password: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	if err := store.Authenticate("alice", "new-pass"); err != nil {
		t.Fatalf("Authenticate(alice,new-pass): %v", err)
	}
}

func TestUsers_UpdatePassword_UserNotFound(t *testing.T) {
	store := newStubAuthStore("admin", "pass")
	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"password": "new-pass"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/users/missing/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/users/missing/password: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestUsers_List_StoreError(t *testing.T) {
	store := newStubAuthStore("admin", "pass")
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
	store := newStubAuthStore("admin", "pass")
	store.createUserErr = errors.New("db down")
	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "pass"})
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

func TestUsers_UpdatePassword_StoreError(t *testing.T) {
	store := newStubAuthStore("admin", "pass")
	if err := store.CreateUser("alice", "old-pass"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	store.updatePassErr = errors.New("db down")
	srv := newAuthTestServer(newMemStore(), store)
	defer srv.Close()

	token, err := auth.CreateToken(testJWTSecret, "admin", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"password": "new-pass"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/users/alice/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "lotsen_token", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/users/alice/password: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestUsers_Delete_StoreError(t *testing.T) {
	store := newStubAuthStore("admin", "pass")
	if err := store.CreateUser("alice", "pass"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
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
