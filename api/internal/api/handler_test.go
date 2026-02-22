package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/internal/events"
	"github.com/ercadev/dirigent/store"
)

// memStore is an in-memory store used only in tests.
type memStore struct {
	mu          sync.RWMutex
	deployments map[string]store.Deployment
}

func newMemStore() *memStore {
	return &memStore{deployments: make(map[string]store.Deployment)}
}

func (m *memStore) List() ([]store.Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]store.Deployment, 0, len(m.deployments))
	for _, d := range m.deployments {
		result = append(result, d)
	}
	return result, nil
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

func (m *memStore) Patch(id string, patch store.Deployment) (store.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deployments[id]
	if !ok {
		return store.Deployment{}, store.ErrNotFound
	}
	if patch.Image != "" {
		d.Image = patch.Image
	}
	if patch.Envs != nil {
		d.Envs = patch.Envs
	}
	if patch.Ports != nil {
		d.Ports = patch.Ports
	}
	if patch.Volumes != nil {
		d.Volumes = patch.Volumes
	}
	if patch.Domain != "" {
		d.Domain = patch.Domain
	}
	if patch.Status != "" {
		d.Status = patch.Status
	}
	m.deployments[id] = d
	return d, nil
}

// failUpdateStore wraps memStore but always fails on Update.
type failUpdateStore struct {
	*memStore
}

func (f *failUpdateStore) Update(_ store.Deployment) (store.Deployment, error) {
	return store.Deployment{}, errors.New("disk full")
}

// failPatchStore wraps memStore but always fails on Patch.
type failPatchStore struct {
	*memStore
}

func (f *failPatchStore) Patch(_ string, _ store.Deployment) (store.Deployment, error) {
	return store.Deployment{}, errors.New("disk full")
}

// noopDockerLogs satisfies api.DockerLogs and always reports no running container.
type noopDockerLogs struct{}

func (noopDockerLogs) StreamLogs(_ context.Context, _ string, _ int) (io.ReadCloser, error) {
	return nil, nil
}

// stubDockerLogs satisfies api.DockerLogs and returns the configured reader or error.
type stubDockerLogs struct {
	rc  io.ReadCloser
	err error
}

func (s *stubDockerLogs) StreamLogs(_ context.Context, _ string, _ int) (io.ReadCloser, error) {
	return s.rc, s.err
}

func newTestServer(s api.Store) *httptest.Server {
	mux := http.NewServeMux()
	api.New(s, events.NewBroker(), noopDockerLogs{}).RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func newTestServerWithBroker(s api.Store, b *events.Broker) *httptest.Server {
	mux := http.NewServeMux()
	api.New(s, b, noopDockerLogs{}).RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func newTestServerWithDockerLogs(s api.Store, dl api.DockerLogs) *httptest.Server {
	mux := http.NewServeMux()
	api.New(s, events.NewBroker(), dl).RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

type statusProviderStub struct {
	snapshot api.SystemStatusSnapshot
	err      error
}

func (s *statusProviderStub) Snapshot(_ context.Context) (api.SystemStatusSnapshot, error) {
	if s.err != nil {
		return api.SystemStatusSnapshot{}, s.err
	}
	return s.snapshot, nil
}

func newTestServerWithStatusProvider(s api.Store, provider api.SystemStatusProvider) *httptest.Server {
	mux := http.NewServeMux()
	api.NewWithSystemStatus(s, events.NewBroker(), noopDockerLogs{}, provider).RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func TestSystemStatus_Healthy(t *testing.T) {
	lastUpdated := time.Date(2026, time.February, 22, 10, 0, 0, 0, time.UTC)
	provider := &statusProviderStub{
		snapshot: api.SystemStatusSnapshot{
			API: api.APISystemStatus{
				State:       api.SystemStatusStateHealthy,
				LastUpdated: lastUpdated,
			},
		},
	}

	srv := newTestServerWithStatusProvider(newMemStore(), provider)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/system-status")
	if err != nil {
		t.Fatalf("GET /api/system-status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body api.SystemStatusSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.API.State != api.SystemStatusStateHealthy {
		t.Errorf("want api state healthy, got %s", body.API.State)
	}
	if !body.API.LastUpdated.Equal(lastUpdated) {
		t.Errorf("want lastUpdated %s, got %s", lastUpdated, body.API.LastUpdated)
	}
	if body.Error != "" {
		t.Errorf("want empty error, got %q", body.Error)
	}
}

func TestSystemStatus_UnavailableOnProviderError(t *testing.T) {
	provider := &statusProviderStub{err: errors.New("status backend down")}

	srv := newTestServerWithStatusProvider(newMemStore(), provider)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/system-status")
	if err != nil {
		t.Fatalf("GET /api/system-status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", resp.StatusCode)
	}

	var body api.SystemStatusSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.API.State != api.SystemStatusStateUnavailable {
		t.Errorf("want api state unavailable, got %s", body.API.State)
	}
	if body.API.LastUpdated.IsZero() {
		t.Error("want non-zero lastUpdated")
	}
	if body.Error != "system status unavailable" {
		t.Errorf("want error system status unavailable, got %q", body.Error)
	}
	if body.Orchestrator.State != api.SystemStatusStateUnavailable {
		t.Errorf("want orchestrator state unavailable, got %s", body.Orchestrator.State)
	}
	if body.Docker.State != api.SystemStatusStateUnavailable {
		t.Errorf("want docker state unavailable, got %s", body.Docker.State)
	}
}

func TestRecordOrchestratorHeartbeat_UpdatesSystemStatus(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/system-status/orchestrator-heartbeat", bytes.NewBufferString("{}"))
	if err != nil {
		t.Fatalf("POST heartbeat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/system-status/orchestrator-heartbeat: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	statusResp, err := http.Get(srv.URL + "/api/system-status")
	if err != nil {
		t.Fatalf("GET /api/system-status: %v", err)
	}
	defer statusResp.Body.Close()

	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", statusResp.StatusCode)
	}

	var body api.SystemStatusSnapshot
	if err := json.NewDecoder(statusResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Orchestrator.State == api.SystemStatusStateUnavailable {
		t.Fatalf("want orchestrator heartbeat state to update, got %s", body.Orchestrator.State)
	}
	if body.Orchestrator.LastUpdated.IsZero() {
		t.Fatal("want non-zero orchestrator lastUpdated")
	}
}

func TestRecordOrchestratorHeartbeat_InvalidBody(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/system-status/orchestrator-heartbeat", bytes.NewBufferString("not json"))
	if err != nil {
		t.Fatalf("POST heartbeat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/system-status/orchestrator-heartbeat: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestRecordOrchestratorHeartbeat_UpdatesDockerConnectivity(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	reqBody := `{"docker":{"reachable":false}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/system-status/orchestrator-heartbeat", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("POST heartbeat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/system-status/orchestrator-heartbeat: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	statusResp, err := http.Get(srv.URL + "/api/system-status")
	if err != nil {
		t.Fatalf("GET /api/system-status: %v", err)
	}
	defer statusResp.Body.Close()

	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", statusResp.StatusCode)
	}

	var body api.SystemStatusSnapshot
	if err := json.NewDecoder(statusResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Docker.State != api.SystemStatusStateDegraded {
		t.Fatalf("want docker state degraded, got %s", body.Docker.State)
	}
	if body.Docker.LastUpdated.IsZero() {
		t.Fatal("want non-zero docker lastUpdated")
	}
}

func TestListDeployments_Empty(t *testing.T) {
	srv := newTestServer(newMemStore())
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

	srv := newTestServer(s)
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

	srv := newTestServer(s)
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
	srv := newTestServer(newMemStore())
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
	srv := newTestServer(&errStore{})
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
	srv := newTestServer(newMemStore())
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

func TestCreateDeployment_DuplicateName(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	srv := newTestServer(s)
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

func TestCreateDeployment_InvalidBody(t *testing.T) {
	srv := newTestServer(newMemStore())
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
	srv := newTestServer(newMemStore())
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
	srv := newTestServer(&errStore{})
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

	srv := newTestServer(s)
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
	srv := newTestServer(newMemStore())
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

func (e *errStore) List() ([]store.Deployment, error) {
	return nil, errors.New("disk full")
}
func (e *errStore) Get(_ string) (store.Deployment, error) {
	return store.Deployment{}, errors.New("disk full")
}
func (e *errStore) Create(_ store.Deployment) (store.Deployment, error) {
	return store.Deployment{}, errors.New("disk full")
}
func (e *errStore) Update(_ store.Deployment) (store.Deployment, error) {
	return store.Deployment{}, errors.New("disk full")
}
func (e *errStore) Patch(_ string, _ store.Deployment) (store.Deployment, error) {
	return store.Deployment{}, errors.New("disk full")
}
func (e *errStore) Delete(_ string) error { return errors.New("disk full") }

func TestDeleteDeployment_StoreError(t *testing.T) {
	srv := newTestServer(&errStore{})
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

// readSSEEvents connects to the SSE endpoint and sends parsed events to the
// returned channel until ctx is cancelled or the connection drops.
func readSSEEvents(ctx context.Context, t *testing.T, url string) <-chan events.StatusEvent {
	t.Helper()
	out := make(chan events.StatusEvent, 8)
	go func() {
		defer close(out)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var e events.StatusEvent
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e); err != nil {
				continue
			}
			select {
			case out <- e:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func TestDeploymentEvents_EmitsDeployingOnCreate(t *testing.T) {
	broker := events.NewBroker()
	srv := newTestServerWithBroker(newMemStore(), broker)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	evtCh := readSSEEvents(ctx, t, srv.URL+"/api/deployments/events")
	// Give the SSE goroutine time to connect and subscribe before we trigger.
	time.Sleep(50 * time.Millisecond)

	body, _ := json.Marshal(map[string]string{"name": "web", "image": "nginx:latest"})
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	var created store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	resp.Body.Close()

	select {
	case event := <-evtCh:
		if event.DeploymentID != created.ID {
			t.Errorf("want deploymentId %s, got %s", created.ID, event.DeploymentID)
		}
		if event.Status != string(store.StatusDeploying) {
			t.Errorf("want status deploying, got %s", event.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: no SSE event received after create")
	}
}

func TestDeploymentEvents_EmitsAllStatusesViaPatch(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusDeploying}

	broker := events.NewBroker()
	srv := newTestServerWithBroker(s, broker)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	evtCh := readSSEEvents(ctx, t, srv.URL+"/api/deployments/events")
	time.Sleep(50 * time.Millisecond)

	statuses := []store.Status{
		store.StatusHealthy,
		store.StatusFailed,
		store.StatusIdle,
		store.StatusDeploying,
	}

	for _, st := range statuses {
		body, _ := json.Marshal(map[string]string{"status": string(st)})
		req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1/status", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PATCH status %s: %v", st, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PATCH status %s: want 200, got %d", st, resp.StatusCode)
		}

		select {
		case event := <-evtCh:
			if event.DeploymentID != "d1" {
				t.Errorf("status %s: want deploymentId d1, got %s", st, event.DeploymentID)
			}
			if event.Status != string(st) {
				t.Errorf("want status %s, got %s", st, event.Status)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout: no SSE event received for status %s", st)
		}
	}
}

func TestDeploymentEvents_FailedStatusCarriesError(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusDeploying}

	broker := events.NewBroker()
	srv := newTestServerWithBroker(s, broker)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	evtCh := readSSEEvents(ctx, t, srv.URL+"/api/deployments/events")
	time.Sleep(50 * time.Millisecond)

	body, _ := json.Marshal(map[string]string{
		"status": string(store.StatusFailed),
		"error":  "container exited with code 1",
	})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()

	select {
	case event := <-evtCh:
		if event.Status != string(store.StatusFailed) {
			t.Errorf("want status failed, got %s", event.Status)
		}
		if event.Error != "container exited with code 1" {
			t.Errorf("want error message, got %q", event.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: no SSE event received")
	}
}

func TestDeploymentEvents_ClientDisconnectCleansUp(t *testing.T) {
	broker := events.NewBroker()
	srv := newTestServerWithBroker(newMemStore(), broker)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	evtCh := readSSEEvents(ctx, t, srv.URL+"/api/deployments/events")
	time.Sleep(50 * time.Millisecond)

	// Cancel the client — simulates disconnect.
	cancel()

	// After disconnect the channel should close (goroutine exits).
	select {
	case _, open := <-evtCh:
		if open {
			t.Error("expected channel to be closed after client disconnect")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: SSE goroutine did not exit after client disconnect")
	}
}

func TestUpdateDeploymentStatus(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusDeploying}

	srv := newTestServer(s)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"status": "healthy"})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/deployments/d1/status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var updated store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.Status != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", updated.Status)
	}
}

func TestUpdateDeploymentStatus_InvalidStatus(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusDeploying}

	srv := newTestServer(s)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"status": "unknown"})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/deployments/d1/status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestUpdateDeploymentStatus_NotFound(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"status": "healthy"})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/nonexistent/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestUpdateDeploymentStatus_StoreGetError(t *testing.T) {
	srv := newTestServer(&errStore{})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"status": "healthy"})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/any/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status store error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestUpdateDeploymentStatus_StoreUpdateError(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusDeploying}

	srv := newTestServer(&failUpdateStore{s})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"status": "healthy"})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status update error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestUpdateDeploymentStatus_InvalidBody(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1/status", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status invalid body: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestPatchDeployment(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:     "d1",
		Name:   "web",
		Image:  "nginx:1",
		Envs:   map[string]string{"PORT": "80"},
		Ports:  []string{"80:80"},
		Status: store.StatusHealthy,
	}

	srv := newTestServer(s)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"image": "nginx:2"})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/deployments/d1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", resp.StatusCode)
	}

	var updated store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.Image != "nginx:2" {
		t.Errorf("want image nginx:2, got %s", updated.Image)
	}
	if updated.Status != store.StatusDeploying {
		t.Errorf("want status deploying, got %s", updated.Status)
	}
	// Unpatched fields must be preserved.
	if updated.Name != "web" {
		t.Errorf("want name web, got %s", updated.Name)
	}
	if updated.Envs["PORT"] != "80" {
		t.Errorf("want PORT=80 preserved, got %v", updated.Envs)
	}
}

func TestPatchDeployment_NotFound(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"image": "nginx:2"})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/deployments/nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestPatchDeployment_StoreError(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	srv := newTestServer(&failPatchStore{s})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"image": "nginx:2"})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/deployments/d1 store error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestPatchDeployment_InvalidBody(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/deployments/d1 invalid body: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestPatchDeployment_EmitsDeployingEvent(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Image: "nginx:1", Status: store.StatusHealthy}

	broker := events.NewBroker()
	srv := newTestServerWithBroker(s, broker)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	evtCh := readSSEEvents(ctx, t, srv.URL+"/api/deployments/events")
	time.Sleep(50 * time.Millisecond)

	body, _ := json.Marshal(map[string]string{"image": "nginx:2"})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/deployments/d1: %v", err)
	}
	resp.Body.Close()

	select {
	case event := <-evtCh:
		if event.DeploymentID != "d1" {
			t.Errorf("want deploymentId d1, got %s", event.DeploymentID)
		}
		if event.Status != string(store.StatusDeploying) {
			t.Errorf("want status deploying, got %s", event.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: no SSE event received after PATCH")
	}
}

func TestUpdateDeployment(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:     "d1",
		Name:   "web",
		Image:  "nginx:1",
		Status: store.StatusHealthy,
	}

	srv := newTestServer(s)
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"name":  "web",
		"image": "nginx:2",
		"ports": []string{"8080:80"},
	})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/deployments/d1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/deployments/d1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var updated store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.ID != "d1" {
		t.Errorf("want id d1, got %s", updated.ID)
	}
	if updated.Image != "nginx:2" {
		t.Errorf("want image nginx:2, got %s", updated.Image)
	}
	if updated.Status != store.StatusDeploying {
		t.Errorf("want status deploying, got %s", updated.Status)
	}
}

func TestUpdateDeployment_NotFound(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"name": "web", "image": "nginx:2"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/deployments/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/deployments/nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestUpdateDeployment_InvalidBody(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/deployments/d1", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/deployments/d1 invalid body: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestUpdateDeployment_MissingFields(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	cases := []map[string]string{
		{"image": "nginx:2"}, // missing name
		{"name": "web"},      // missing image
		{},                   // both missing
	}
	for _, payload := range cases {
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/deployments/d1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/deployments/d1: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("payload %v: want 400, got %d", payload, resp.StatusCode)
		}
	}
}

func TestUpdateDeployment_StoreError(t *testing.T) {
	srv := newTestServer(&errStore{})
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"name": "web", "image": "nginx:2"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/deployments/any", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/deployments/any store error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestUpdateDeployment_Lifecycle(t *testing.T) {
	broker := events.NewBroker()
	srv := newTestServerWithBroker(newMemStore(), broker)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	evtCh := readSSEEvents(ctx, t, srv.URL+"/api/deployments/events")
	time.Sleep(50 * time.Millisecond)

	// Step 1: create.
	createBody, _ := json.Marshal(map[string]string{"name": "web", "image": "nginx:1"})
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	var created store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()
	if created.Status != store.StatusDeploying {
		t.Fatalf("after create: want status deploying, got %s", created.Status)
	}

	select {
	case event := <-evtCh:
		if event.DeploymentID != created.ID || event.Status != string(store.StatusDeploying) {
			t.Errorf("create event: want {%s deploying}, got {%s %s}", created.ID, event.DeploymentID, event.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: no SSE event after create")
	}

	// Step 2: edit (PUT) — triggers redeployment.
	editBody, _ := json.Marshal(map[string]string{"name": "web", "image": "nginx:2"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/deployments/"+created.ID, bytes.NewReader(editBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/deployments/%s: %v", created.ID, err)
	}
	var edited store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&edited); err != nil {
		t.Fatalf("decode edit response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT: want 200, got %d", resp.StatusCode)
	}
	if edited.Image != "nginx:2" {
		t.Errorf("after edit: want image nginx:2, got %s", edited.Image)
	}
	if edited.Status != store.StatusDeploying {
		t.Errorf("after edit: want status deploying, got %s", edited.Status)
	}

	// Step 3: SSE must emit deploying for the edit.
	select {
	case event := <-evtCh:
		if event.DeploymentID != created.ID {
			t.Errorf("edit event: want deploymentId %s, got %s", created.ID, event.DeploymentID)
		}
		if event.Status != string(store.StatusDeploying) {
			t.Errorf("edit event: want status deploying, got %s", event.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: no SSE event after PUT")
	}
}

// readLogLines connects to a log SSE endpoint and sends parsed log line strings
// to the returned channel until ctx is cancelled or the connection drops.
func readLogLines(ctx context.Context, t *testing.T, url string) <-chan string {
	t.Helper()
	out := make(chan string, 16)
	go func() {
		defer close(out)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var payload struct {
				Line string `json:"line"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &payload); err != nil {
				continue
			}
			select {
			case out <- payload.Line:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func TestDeploymentLogs_NotFound(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments/nonexistent/logs")
	if err != nil {
		t.Fatalf("GET /api/deployments/nonexistent/logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestDeploymentLogs_StoreError(t *testing.T) {
	srv := newTestServer(&errStore{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments/any/logs")
	if err != nil {
		t.Fatalf("GET /api/deployments/any/logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestDeploymentLogs_NoContainer(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusIdle}

	// Stub reports no running container (nil, nil).
	srv := newTestServerWithDockerLogs(s, &stubDockerLogs{rc: nil})
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logCh := readLogLines(ctx, t, srv.URL+"/api/deployments/d1/logs")

	// Stream should close immediately — no container to read from.
	select {
	case _, open := <-logCh:
		if open {
			t.Error("expected channel to close when no container is running")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: channel did not close for no-container case")
	}
}

func TestDeploymentLogs_StreamsLines(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	// Stub returns a reader with three plain log lines (already demultiplexed).
	stub := &stubDockerLogs{rc: io.NopCloser(strings.NewReader("line one\nline two\nline three\n"))}
	srv := newTestServerWithDockerLogs(s, stub)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logCh := readLogLines(ctx, t, srv.URL+"/api/deployments/d1/logs")

	for _, want := range []string{"line one", "line two", "line three"} {
		select {
		case got, ok := <-logCh:
			if !ok {
				t.Fatalf("channel closed before receiving %q", want)
			}
			if got != want {
				t.Errorf("want %q, got %q", want, got)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout: did not receive %q", want)
		}
	}
}

func TestDeploymentLogs_ClientDisconnectCleansUp(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	// Use a pipe so the server-side scanner blocks waiting for new data.
	pr, pw := io.Pipe()
	defer pw.Close()

	srv := newTestServerWithDockerLogs(s, &stubDockerLogs{rc: pr})
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	logCh := readLogLines(ctx, t, srv.URL+"/api/deployments/d1/logs")
	time.Sleep(50 * time.Millisecond)

	cancel()   // disconnect the client
	pw.Close() // unblock the server-side scanner so the handler goroutine can exit

	select {
	case _, open := <-logCh:
		if open {
			t.Error("expected channel closed after client disconnect")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: channel did not close after client disconnect")
	}
}

func TestDeploymentLogs_DockerError(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	stub := &stubDockerLogs{err: errors.New("docker daemon unreachable")}
	srv := newTestServerWithDockerLogs(s, stub)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logCh := readLogLines(ctx, t, srv.URL+"/api/deployments/d1/logs")

	// Stream should close after the error — handler logs and returns.
	select {
	case _, open := <-logCh:
		if open {
			t.Error("expected channel to close on docker error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: channel did not close on docker error")
	}
}
