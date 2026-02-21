package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/internal/orchestrator"
	"github.com/ercadev/dirigent/internal/store"
)

// memStore is an in-memory store used only in tests.
type memStore struct {
	mu          sync.RWMutex
	deployments map[string]store.Deployment
}

func newMemStore() *memStore {
	return &memStore{deployments: make(map[string]store.Deployment)}
}

func (m *memStore) List() []store.Deployment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]store.Deployment, 0, len(m.deployments))
	for _, d := range m.deployments {
		result = append(result, d)
	}
	return result
}

func (m *memStore) Get(id string) (store.Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.deployments[id]
	if !ok {
		return store.Deployment{}, store.ErrNotFound
	}
	return d, nil
}

func (m *memStore) Create(d store.Deployment) (store.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.deployments {
		if existing.Name == d.Name {
			return store.Deployment{}, store.ErrDuplicateName
		}
	}
	m.deployments[d.ID] = d
	return d, nil
}

func (m *memStore) Update(d store.Deployment) (store.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.deployments[d.ID]; !ok {
		return store.Deployment{}, store.ErrNotFound
	}
	m.deployments[d.ID] = d
	return d, nil
}

func (m *memStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.deployments[id]; !ok {
		return store.ErrNotFound
	}
	delete(m.deployments, id)
	return nil
}

// noopOrchestrator succeeds silently — used in tests that don't care about orchestration.
type noopOrchestrator struct{}

func (n *noopOrchestrator) Ping(_ context.Context) error                        { return nil }
func (n *noopOrchestrator) Start(_ context.Context, _ store.Deployment) error  { return nil }

// failOrchestrator simulates a Docker daemon that is always unreachable.
type failOrchestrator struct{}

func (f *failOrchestrator) Ping(_ context.Context) error {
	return fmt.Errorf("%w: connection refused", orchestrator.ErrDockerUnavailable)
}
func (f *failOrchestrator) Start(_ context.Context, _ store.Deployment) error {
	return fmt.Errorf("%w: connection refused", orchestrator.ErrDockerUnavailable)
}

// startErrOrchestrator succeeds on Ping but fails on Start (Docker goes down between check and run).
type startErrOrchestrator struct{ err error }

func (s *startErrOrchestrator) Ping(_ context.Context) error                       { return nil }
func (s *startErrOrchestrator) Start(_ context.Context, _ store.Deployment) error { return s.err }

func newTestServer(s api.Store, o api.Orchestrator) *httptest.Server {
	mux := http.NewServeMux()
	api.New(s, o).RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

// waitFor polls condition every 10 ms until it returns true or 2 s elapse.
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

func TestListDeployments_Empty(t *testing.T) {
	srv := newTestServer(newMemStore(), &noopOrchestrator{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments")
	if err != nil {
		t.Fatalf("GET /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body []store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("want empty list, got %d items", len(body))
	}
}

func TestListDeployments_WithItems(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusIdle}
	s.deployments["d2"] = store.Deployment{ID: "d2", Name: "api", Status: store.StatusHealthy}

	srv := newTestServer(s, &noopOrchestrator{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments")
	if err != nil {
		t.Fatalf("GET /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body []store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 2 {
		t.Errorf("want 2 items, got %d", len(body))
	}
}

func TestGetDeployment_Found(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	srv := newTestServer(s, &noopOrchestrator{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments/d1")
	if err != nil {
		t.Fatalf("GET /api/deployments/d1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var d store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if d.ID != "d1" {
		t.Errorf("want id d1, got %s", d.ID)
	}
	if d.Status != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", d.Status)
	}
}

func TestGetDeployment_NotFound(t *testing.T) {
	srv := newTestServer(newMemStore(), &noopOrchestrator{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments/nonexistent")
	if err != nil {
		t.Fatalf("GET /api/deployments/nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestGetDeployment_StoreError(t *testing.T) {
	srv := newTestServer(&errStore{}, &noopOrchestrator{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments/any")
	if err != nil {
		t.Fatalf("GET /api/deployments/any: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestCreateDeployment(t *testing.T) {
	srv := newTestServer(newMemStore(), &noopOrchestrator{})
	defer srv.Close()

	payload := map[string]any{
		"name":  "web",
		"image": "nginx:latest",
		"ports": []string{"80:80"},
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}

	var created store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == "" {
		t.Error("want non-empty id")
	}
	if created.Name != "web" {
		t.Errorf("want name web, got %s", created.Name)
	}
	if created.Image != "nginx:latest" {
		t.Errorf("want image nginx:latest, got %s", created.Image)
	}
	if created.Status != store.StatusDeploying {
		t.Errorf("want status deploying, got %s", created.Status)
	}
}

func TestCreateDeployment_AsyncStatusHealthy(t *testing.T) {
	s := newMemStore()
	srv := newTestServer(s, &noopOrchestrator{})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"name": "web", "image": "nginx:latest"})
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}

	var created store.Deployment
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	if created.Status != store.StatusDeploying {
		t.Errorf("want initial status deploying, got %s", created.Status)
	}

	waitFor(t, func() bool {
		d, err := s.Get(created.ID)
		return err == nil && d.Status == store.StatusHealthy
	})
}

func TestCreateDeployment_AsyncStatusFailed(t *testing.T) {
	s := newMemStore()
	srv := newTestServer(s, &startErrOrchestrator{err: errors.New("image not found")})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"name": "web", "image": "bad:image"})
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}

	var created store.Deployment
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	waitFor(t, func() bool {
		d, err := s.Get(created.ID)
		return err == nil && d.Status == store.StatusFailed
	})
}

func TestCreateDeployment_DuplicateName(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	srv := newTestServer(s, &noopOrchestrator{})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"name": "web", "image": "nginx:latest"})
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", resp.StatusCode)
	}
}

func TestCreateDeployment_DockerUnavailable(t *testing.T) {
	srv := newTestServer(newMemStore(), &failOrchestrator{})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"name": "web", "image": "nginx:latest"})
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", resp.StatusCode)
	}
}

func TestCreateDeployment_InvalidBody(t *testing.T) {
	srv := newTestServer(newMemStore(), &noopOrchestrator{})
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewBufferString("not json"))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestCreateDeployment_MissingFields(t *testing.T) {
	srv := newTestServer(newMemStore(), &noopOrchestrator{})
	defer srv.Close()

	cases := []map[string]string{
		{"image": "nginx"}, // missing name
		{"name": "web"},    // missing image
		{},                 // both missing
	}
	for _, payload := range cases {
		body, _ := json.Marshal(payload)
		resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST /api/deployments: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("payload %v: want 400, got %d", payload, resp.StatusCode)
		}
	}
}

func TestCreateDeployment_StoreError(t *testing.T) {
	srv := newTestServer(&errStore{}, &noopOrchestrator{})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"name": "web", "image": "nginx"})
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestDeleteDeployment(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusIdle}

	srv := newTestServer(s, &noopOrchestrator{})
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/deployments/d1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/deployments/d1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}
	if len(s.deployments) != 0 {
		t.Error("want deployment removed from store")
	}
}

func TestDeleteDeployment_NotFound(t *testing.T) {
	srv := newTestServer(newMemStore(), &noopOrchestrator{})
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/deployments/nonexistent", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/deployments/nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

// errStore always returns a generic error — used to test internal server error paths.
type errStore struct{}

func (e *errStore) List() []store.Deployment                            { return nil }
func (e *errStore) Get(_ string) (store.Deployment, error)             { return store.Deployment{}, errors.New("disk full") }
func (e *errStore) Create(_ store.Deployment) (store.Deployment, error) { return store.Deployment{}, errors.New("disk full") }
func (e *errStore) Update(_ store.Deployment) (store.Deployment, error) { return store.Deployment{}, errors.New("disk full") }
func (e *errStore) Delete(_ string) error                               { return errors.New("disk full") }

func TestDeleteDeployment_StoreError(t *testing.T) {
	srv := newTestServer(&errStore{}, &noopOrchestrator{})
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/deployments/any", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/deployments/any: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}
