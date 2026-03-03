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
	"os"
	"path/filepath"
	"strconv"
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
	registries  map[string]store.RegistryEntry
}

func newMemStore() *memStore {
	return &memStore{deployments: make(map[string]store.Deployment), registries: make(map[string]store.RegistryEntry)}
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
	if patch.PublicSet {
		d.Public = patch.Public
	}
	if patch.BasicAuth != nil {
		d.BasicAuth = patch.BasicAuth
	}
	if patch.Security != nil {
		d.Security = patch.Security
	}
	if patch.Status != "" {
		d.Status = patch.Status
		d.Error = patch.Error
	} else if patch.Error != "" {
		d.Error = patch.Error
	}
	m.deployments[id] = d
	return d, nil
}

func (m *memStore) ListRegistries() ([]store.RegistryEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]store.RegistryEntry, 0, len(m.registries))
	for _, r := range m.registries {
		result = append(result, r)
	}
	return result, nil
}

func (m *memStore) CreateRegistry(id, prefix, username, _ string) (store.RegistryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.registries {
		if existing.Prefix == prefix {
			return store.RegistryEntry{}, store.ErrDuplicateRegistryPrefix
		}
	}
	entry := store.RegistryEntry{ID: id, Prefix: prefix, Username: username}
	m.registries[id] = entry
	return entry, nil
}

func (m *memStore) UpdateRegistry(id, prefix, username, _ string) (store.RegistryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.registries[id]; !ok {
		return store.RegistryEntry{}, store.ErrRegistryNotFound
	}
	for rid, existing := range m.registries {
		if rid != id && existing.Prefix == prefix {
			return store.RegistryEntry{}, store.ErrDuplicateRegistryPrefix
		}
	}
	entry := store.RegistryEntry{ID: id, Prefix: prefix, Username: username}
	m.registries[id] = entry
	return entry, nil
}

func (m *memStore) DeleteRegistry(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.registries[id]; !ok {
		return store.ErrRegistryNotFound
	}
	delete(m.registries, id)
	return nil
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

func (noopDockerLogs) RecentLogs(_ context.Context, _ string, _ int) ([]string, error) {
	return []string{}, nil
}

// stubDockerLogs satisfies api.DockerLogs and returns the configured reader or error.
type stubDockerLogs struct {
	rc         io.ReadCloser
	err        error
	recentLogs []string
	recentErr  error
}

func (s *stubDockerLogs) StreamLogs(_ context.Context, _ string, _ int) (io.ReadCloser, error) {
	return s.rc, s.err
}

func (s *stubDockerLogs) RecentLogs(_ context.Context, _ string, _ int) ([]string, error) {
	if s.recentErr != nil {
		return nil, s.recentErr
	}
	if s.recentLogs == nil {
		return []string{}, nil
	}
	return s.recentLogs, nil
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

func newTestServerWithAuthCookieDomain(s api.Store, domain string) *httptest.Server {
	mux := http.NewServeMux()
	h := api.New(s, events.NewBroker(), noopDockerLogs{})
	h.SetAuthCookieDomain(domain)
	h.RegisterRoutes(mux)
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
	if body.LoadBalancer.State != api.SystemStatusStateUnavailable {
		t.Errorf("want load balancer state unavailable, got %s", body.LoadBalancer.State)
	}
	if body.Host.CPU.State != api.SystemStatusStateUnavailable {
		t.Errorf("want cpu state unavailable, got %s", body.Host.CPU.State)
	}
	if body.Host.RAM.State != api.SystemStatusStateUnavailable {
		t.Errorf("want ram state unavailable, got %s", body.Host.RAM.State)
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

func TestRecordOrchestratorHeartbeat_UpdatesHostMetrics(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	reqBody := `{"host":{"cpu":{"usagePercent":41.7},"ram":{"usagePercent":68.9}}}`
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

	if body.Host.CPU.State != api.SystemStatusStateHealthy {
		t.Fatalf("want cpu state healthy, got %s", body.Host.CPU.State)
	}
	if body.Host.CPU.UsagePercent != 41.7 {
		t.Fatalf("want cpu usage 41.7, got %v", body.Host.CPU.UsagePercent)
	}
	if body.Host.CPU.LastUpdated.IsZero() {
		t.Fatal("want non-zero cpu lastUpdated")
	}

	if body.Host.RAM.State != api.SystemStatusStateHealthy {
		t.Fatalf("want ram state healthy, got %s", body.Host.RAM.State)
	}
	if body.Host.RAM.UsagePercent != 68.9 {
		t.Fatalf("want ram usage 68.9, got %v", body.Host.RAM.UsagePercent)
	}
	if body.Host.RAM.LastUpdated.IsZero() {
		t.Fatal("want non-zero ram lastUpdated")
	}
}

func TestRecordOrchestratorHeartbeat_UpdatesLoadBalancerHealth(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	reqBody := `{"loadBalancer":{"responding":false}}`
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

	if body.LoadBalancer.State != api.SystemStatusStateDegraded {
		t.Fatalf("want load balancer state degraded, got %s", body.LoadBalancer.State)
	}
	if body.LoadBalancer.LastUpdated == nil || body.LoadBalancer.LastUpdated.IsZero() {
		t.Fatal("want non-zero load balancer lastUpdated")
	}
}

func TestRecordOrchestratorHeartbeat_UpdatesLoadBalancerTrafficTelemetry(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	reqBody := `{"loadBalancer":{"responding":true,"traffic":{"totalRequests":201,"suspiciousRequests":19,"blockedRequests":4,"activeBlockedIps":2,"blockedIps":[{"ip":"203.0.113.7"},{"ip":"198.51.100.11"}]}}}`
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

	var body api.SystemStatusSnapshot
	if err := json.NewDecoder(statusResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.LoadBalancer.Traffic == nil {
		t.Fatal("want load balancer traffic telemetry")
	}
	if body.LoadBalancer.Traffic.TotalRequests != 201 {
		t.Fatalf("want total requests 201, got %d", body.LoadBalancer.Traffic.TotalRequests)
	}
	if body.LoadBalancer.Traffic.ActiveBlockedIPs != 2 {
		t.Fatalf("want active blocked ips 2, got %d", body.LoadBalancer.Traffic.ActiveBlockedIPs)
	}
	if len(body.LoadBalancer.Traffic.BlockedIPs) != 2 {
		t.Fatalf("want 2 blocked ips, got %d", len(body.LoadBalancer.Traffic.BlockedIPs))
	}
}

func TestRecordOrchestratorHeartbeat_ReplacesContainerStatsCache(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	srv := newTestServer(s)
	defer srv.Close()

	firstHeartbeat := `{"containerStats":{"d1":{"cpuPercent":22.5,"memoryUsedBytes":268435456,"memoryLimitBytes":536870912,"memoryPercent":50}}}`
	firstReq, err := http.NewRequest(http.MethodPost, srv.URL+"/api/system-status/orchestrator-heartbeat", bytes.NewBufferString(firstHeartbeat))
	if err != nil {
		t.Fatalf("build first heartbeat request: %v", err)
	}
	firstReq.Header.Set("Content-Type", "application/json")

	firstResp, err := http.DefaultClient.Do(firstReq)
	if err != nil {
		t.Fatalf("POST first heartbeat: %v", err)
	}
	firstResp.Body.Close()

	secondHeartbeat := `{"containerStats":{}}`
	secondReq, err := http.NewRequest(http.MethodPost, srv.URL+"/api/system-status/orchestrator-heartbeat", bytes.NewBufferString(secondHeartbeat))
	if err != nil {
		t.Fatalf("build second heartbeat request: %v", err)
	}
	secondReq.Header.Set("Content-Type", "application/json")

	secondResp, err := http.DefaultClient.Do(secondReq)
	if err != nil {
		t.Fatalf("POST second heartbeat: %v", err)
	}
	secondResp.Body.Close()

	getResp, err := http.Get(srv.URL + "/api/deployments/d1")
	if err != nil {
		t.Fatalf("GET /api/deployments/d1: %v", err)
	}
	defer getResp.Body.Close()

	var body struct {
		Stats any `json:"stats"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Stats != nil {
		t.Fatalf("want stats omitted after cache replace, got %+v", body.Stats)
	}
}

func TestSystemStatusEvents_StreamSendsHeartbeatUpdates(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/system-status/events", nil)
	if err != nil {
		t.Fatalf("build events request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/system-status/events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	readEvent := func(r *bufio.Reader) api.SystemStatusSnapshot {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			line, readErr := r.ReadString('\n')
			if readErr != nil {
				t.Fatalf("read stream line: %v", readErr)
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			var snapshot api.SystemStatusSnapshot
			if err := json.Unmarshal([]byte(payload), &snapshot); err != nil {
				t.Fatalf("unmarshal stream payload: %v", err)
			}
			return snapshot
		}
		t.Fatal("timed out waiting for system-status event")
		return api.SystemStatusSnapshot{}
	}

	reader := bufio.NewReader(resp.Body)
	_ = readEvent(reader) // initial snapshot

	heartbeatReqBody := `{"loadBalancer":{"responding":false,"traffic":{"totalRequests":88,"blockedRequests":3,"activeBlockedIps":1,"blockedIps":[{"ip":"203.0.113.7"}]}}}`
	heartbeatReq, err := http.NewRequest(http.MethodPost, srv.URL+"/api/system-status/orchestrator-heartbeat", bytes.NewBufferString(heartbeatReqBody))
	if err != nil {
		t.Fatalf("build heartbeat request: %v", err)
	}
	heartbeatReq.Header.Set("Content-Type", "application/json")

	heartbeatResp, err := http.DefaultClient.Do(heartbeatReq)
	if err != nil {
		t.Fatalf("POST /api/system-status/orchestrator-heartbeat: %v", err)
	}
	heartbeatResp.Body.Close()

	if heartbeatResp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", heartbeatResp.StatusCode)
	}

	updated := readEvent(reader)
	if updated.LoadBalancer.State != api.SystemStatusStateDegraded {
		t.Fatalf("want degraded load balancer state, got %s", updated.LoadBalancer.State)
	}
	if updated.LoadBalancer.Traffic == nil || updated.LoadBalancer.Traffic.BlockedRequests != 3 {
		t.Fatalf("want blocked request count 3, got %+v", updated.LoadBalancer.Traffic)
	}
}

func TestRecordOrchestratorHeartbeat_RejectsInvalidHostMetricPercent(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	reqBody := `{"host":{"cpu":{"usagePercent":101}}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/system-status/orchestrator-heartbeat", bytes.NewBufferString(reqBody))
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

func TestLoadBalancerAccessLogs_PaginatesNewestFirst(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv("LOTSEN_PROXY_ACCESS_LOG_DIR", logDir)

	writeAccessLogFile := func(name string, lines []string) {
		t.Helper()
		path := filepath.Join(logDir, name)
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatalf("write access log file %s: %v", name, err)
		}
	}

	writeAccessLogFile("access-2026-02-24-20.log", []string{
		`{"timestamp":"2026-02-24T20:30:00Z","clientIp":"203.0.113.5","host":"api.example.com","method":"GET","path":"/health","status":200,"durationMs":4,"bytesWritten":12,"outcome":"proxied","headers":{"user-agent":"ua-3"}}`,
		`{"timestamp":"2026-02-24T20:31:00Z","clientIp":"203.0.113.6","host":"api.example.com","method":"GET","path":"/ready","status":200,"durationMs":5,"bytesWritten":15,"outcome":"proxied","headers":{"user-agent":"ua-4"}}`,
	})
	writeAccessLogFile("access-2026-02-24-19.log", []string{
		`{"timestamp":"2026-02-24T19:10:00Z","clientIp":"198.51.100.7","host":"api.example.com","method":"POST","path":"/deploy","status":201,"durationMs":12,"bytesWritten":80,"outcome":"proxied","headers":{"user-agent":"ua-2"}}`,
		`{"timestamp":"2026-02-24T19:09:00Z","clientIp":"198.51.100.8","host":"api.example.com","method":"GET","path":"/metrics","status":200,"durationMs":6,"bytesWritten":22,"outcome":"proxied","headers":{"user-agent":"ua-1"}}`,
	})

	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/load-balancer/access-logs?limit=2")
	if err != nil {
		t.Fatalf("GET access logs page1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var page1 struct {
		Items []struct {
			Path string `json:"path"`
		} `json:"items"`
		HasMore    bool   `json:"hasMore"`
		NextCursor string `json:"nextCursor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&page1); err != nil {
		t.Fatalf("decode page1: %v", err)
	}

	if len(page1.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(page1.Items))
	}
	if page1.Items[0].Path != "/ready" || page1.Items[1].Path != "/health" {
		t.Fatalf("unexpected page1 order: %+v", page1.Items)
	}
	if !page1.HasMore || page1.NextCursor == "" {
		t.Fatalf("want next cursor and more=true, got hasMore=%v cursor=%q", page1.HasMore, page1.NextCursor)
	}

	resp2, err := http.Get(srv.URL + "/api/load-balancer/access-logs?limit=2&cursor=" + page1.NextCursor)
	if err != nil {
		t.Fatalf("GET access logs page2: %v", err)
	}
	defer resp2.Body.Close()

	var page2 struct {
		Items []struct {
			Path string `json:"path"`
		} `json:"items"`
		HasMore bool `json:"hasMore"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&page2); err != nil {
		t.Fatalf("decode page2: %v", err)
	}

	if len(page2.Items) != 2 {
		t.Fatalf("want 2 items in page2, got %d", len(page2.Items))
	}
	if page2.Items[0].Path != "/metrics" || page2.Items[1].Path != "/deploy" {
		t.Fatalf("unexpected page2 order: %+v", page2.Items)
	}
	if page2.HasMore {
		t.Fatal("want hasMore=false at end")
	}
}

func TestLoadBalancerAccessLogs_InvalidCursorReturns400(t *testing.T) {
	t.Setenv("LOTSEN_PROXY_ACCESS_LOG_DIR", t.TempDir())
	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/load-balancer/access-logs?cursor=!!!!")
	if err != nil {
		t.Fatalf("GET access logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestLoadBalancerAccessLogs_FiltersByMethodStatusHostAndIP(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv("LOTSEN_PROXY_ACCESS_LOG_DIR", logDir)

	path := filepath.Join(logDir, "access-2026-02-24-20.log")
	content := strings.Join([]string{
		`{"timestamp":"2026-02-24T20:31:00Z","clientIp":"203.0.113.10","host":"api.example.com","method":"GET","path":"/health","status":200,"durationMs":2,"bytesWritten":10,"outcome":"proxied"}`,
		`{"timestamp":"2026-02-24T20:32:00Z","clientIp":"198.51.100.8","host":"admin.example.com","method":"POST","path":"/login","status":401,"durationMs":5,"bytesWritten":15,"outcome":"unauthorized"}`,
		`{"timestamp":"2026-02-24T20:33:00Z","clientIp":"203.0.113.7","host":"api.example.com","method":"POST","path":"/deploy","status":201,"durationMs":8,"bytesWritten":25,"outcome":"proxied"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write access log file: %v", err)
	}

	srv := newTestServer(newMemStore())
	defer srv.Close()

	url := srv.URL + "/api/load-balancer/access-logs?method=POST&status=201&host=api.example&ip=203.0.113.7"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET filtered access logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body struct {
		Items []struct {
			Path   string `json:"path"`
			Method string `json:"method"`
			Status int    `json:"status"`
			Host   string `json:"host"`
			IP     string `json:"clientIp"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body.Items) != 1 {
		t.Fatalf("want 1 filtered item, got %d", len(body.Items))
	}
	entry := body.Items[0]
	if entry.Path != "/deploy" || entry.Method != "POST" || entry.Status != 201 || entry.Host != "api.example.com" || entry.IP != "203.0.113.7" {
		t.Fatalf("unexpected filtered entry: %+v", entry)
	}
}

func TestLoadBalancerAccessLogs_InvalidStatusFilterReturns400(t *testing.T) {
	t.Setenv("LOTSEN_PROXY_ACCESS_LOG_DIR", t.TempDir())
	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/load-balancer/access-logs?status=abc")
	if err != nil {
		t.Fatalf("GET access logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
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

func TestListDeployments_IncludesContainerStatsWhenCached(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	srv := newTestServer(s)
	defer srv.Close()

	heartbeat := `{"containerStats":{"d1":{"cpuPercent":22.5,"memoryUsedBytes":268435456,"memoryLimitBytes":536870912,"memoryPercent":50}}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/system-status/orchestrator-heartbeat", bytes.NewBufferString(heartbeat))
	if err != nil {
		t.Fatalf("build heartbeat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST heartbeat: %v", err)
	}
	resp.Body.Close()

	listResp, err := http.Get(srv.URL + "/api/deployments")
	if err != nil {
		t.Fatalf("GET /api/deployments: %v", err)
	}
	defer listResp.Body.Close()

	var body []struct {
		ID    string `json:"id"`
		Stats *struct {
			CPUPercent       float64 `json:"cpuPercent"`
			MemoryUsedBytes  uint64  `json:"memoryUsedBytes"`
			MemoryLimitBytes uint64  `json:"memoryLimitBytes"`
			MemoryPercent    float64 `json:"memoryPercent"`
		} `json:"stats"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body) != 1 {
		t.Fatalf("want 1 deployment, got %d", len(body))
	}
	if body[0].Stats == nil {
		t.Fatal("want stats payload")
	}
	if body[0].Stats.MemoryPercent != 50 {
		t.Fatalf("want memory percent 50, got %v", body[0].Stats.MemoryPercent)
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

func TestGetDeployment_IncludesContainerStatsWhenCached(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	srv := newTestServer(s)
	defer srv.Close()

	heartbeat := `{"containerStats":{"d1":{"cpuPercent":11.1,"memoryUsedBytes":134217728,"memoryLimitBytes":268435456,"memoryPercent":50}}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/system-status/orchestrator-heartbeat", bytes.NewBufferString(heartbeat))
	if err != nil {
		t.Fatalf("build heartbeat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST heartbeat: %v", err)
	}
	resp.Body.Close()

	getResp, err := http.Get(srv.URL + "/api/deployments/d1")
	if err != nil {
		t.Fatalf("GET /api/deployments/d1: %v", err)
	}
	defer getResp.Body.Close()

	var body struct {
		ID    string `json:"id"`
		Stats *struct {
			CPUPercent float64 `json:"cpuPercent"`
		} `json:"stats"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.ID != "d1" {
		t.Fatalf("want id d1, got %s", body.ID)
	}
	if body.Stats == nil || body.Stats.CPUPercent != 11.1 {
		t.Fatalf("want cpu stats 11.1, got %+v", body.Stats)
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

func TestCreateDeployment_DashboardDomainConflict(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_DOMAIN", "dashboard.example.com")

	srv := newTestServer(newMemStore())
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"name":   "web",
		"image":  "nginx:latest",
		"domain": "Dashboard.Example.com.",
	})
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", resp.StatusCode)
	}
}

func TestCreateDeployment_PrivateDomainMustMatchAuthCookieDomain(t *testing.T) {
	srv := newTestServerWithAuthCookieDomain(newMemStore(), "example.com")
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"name":   "web",
		"image":  "nginx:latest",
		"domain": "other.dev",
		"public": false,
	})
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
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
func (e *errStore) ListRegistries() ([]store.RegistryEntry, error) {
	return nil, errors.New("disk full")
}
func (e *errStore) CreateRegistry(_, _, _, _ string) (store.RegistryEntry, error) {
	return store.RegistryEntry{}, errors.New("disk full")
}
func (e *errStore) UpdateRegistry(_, _, _, _ string) (store.RegistryEntry, error) {
	return store.RegistryEntry{}, errors.New("disk full")
}
func (e *errStore) DeleteRegistry(_ string) error { return errors.New("disk full") }

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
		"reason": string(store.StatusReasonContainerExited),
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
		if event.Reason != string(store.StatusReasonContainerExited) {
			t.Errorf("want reason %q, got %q", store.StatusReasonContainerExited, event.Reason)
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

	body, _ := json.Marshal(map[string]string{"status": "healthy", "reason": string(store.StatusReasonDeployStartSucceeded)})
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
	if updated.Reason != store.StatusReasonDeployStartSucceeded {
		t.Errorf("want reason %q, got %q", store.StatusReasonDeployStartSucceeded, updated.Reason)
	}
	if updated.Error != "" {
		t.Errorf("want empty error, got %q", updated.Error)
	}
}

func TestUpdateDeploymentStatus_FailedStoresError(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusDeploying}

	srv := newTestServer(s)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{
		"status": "failed",
		"reason": string(store.StatusReasonDeployStartFailed),
		"error":  "image not found",
	})
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
	if updated.Status != store.StatusFailed {
		t.Errorf("want status failed, got %s", updated.Status)
	}
	if updated.Reason != store.StatusReasonDeployStartFailed {
		t.Errorf("want reason %q, got %q", store.StatusReasonDeployStartFailed, updated.Reason)
	}
	if updated.Error != "image not found" {
		t.Errorf("want error image not found, got %q", updated.Error)
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
		Public: true,
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

func TestPatchDeployment_DomainOnly_DoesNotRedeploy(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:     "d1",
		Name:   "web",
		Image:  "nginx:1",
		Domain: "old.example.com",
		Status: store.StatusHealthy,
	}

	broker := events.NewBroker()
	srv := newTestServerWithBroker(s, broker)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	evtCh := readSSEEvents(ctx, t, srv.URL+"/api/deployments/events")
	time.Sleep(50 * time.Millisecond)

	body, _ := json.Marshal(map[string]string{"domain": "new.example.com"})
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
	if updated.Domain != "new.example.com" {
		t.Errorf("want domain new.example.com, got %s", updated.Domain)
	}
	if updated.Status != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", updated.Status)
	}

	select {
	case event := <-evtCh:
		t.Fatalf("unexpected SSE event for domain-only patch: %+v", event)
	case <-time.After(250 * time.Millisecond):
	}
}

func TestPatchDeployment_DashboardDomainConflict(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_DOMAIN", "dashboard.example.com")

	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:     "d1",
		Name:   "web",
		Image:  "nginx:1",
		Domain: "app.example.com",
		Status: store.StatusHealthy,
	}

	srv := newTestServer(s)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"domain": "dashboard.example.com"})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/deployments/d1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", resp.StatusCode)
	}
}

func TestPatchDeployment_CannotEnablePrivateOutsideAuthCookieDomain(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:     "d1",
		Name:   "web",
		Image:  "nginx:1",
		Domain: "app.other.dev",
		Public: true,
		Status: store.StatusHealthy,
	}

	srv := newTestServerWithAuthCookieDomain(s, "example.com")
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{"public": false})
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/d1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/deployments/d1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestUpdateDeployment(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:     "d1",
		Name:   "web",
		Image:  "nginx:1",
		Public: true,
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

func TestUpdateDeployment_NoChanges_SkipsStoreUpdate(t *testing.T) {
	base := newMemStore()
	base.deployments["d1"] = store.Deployment{
		ID:      "d1",
		Name:    "web",
		Image:   "nginx:1",
		Envs:    map[string]string{"PORT": "80"},
		Ports:   []string{"32768:80"},
		Volumes: []string{"/data:/data"},
		Domain:  "app.example.com",
		Public:  false,
		Status:  store.StatusHealthy,
	}

	srv := newTestServer(&failUpdateStore{memStore: base})
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"name":    "web",
		"image":   "nginx:1",
		"envs":    map[string]string{"PORT": "80"},
		"ports":   []string{"80"},
		"volumes": []string{"/data:/data"},
		"domain":  "app.example.com",
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
	if updated.Status != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", updated.Status)
	}
}

func TestUpdateDeployment_PublicOnly_UpdatesVisibility(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:      "d1",
		Name:    "web",
		Image:   "nginx:1",
		Envs:    map[string]string{"PORT": "80"},
		Ports:   []string{"32768:80"},
		Volumes: []string{"/data:/data"},
		Domain:  "app.example.com",
		Public:  false,
		Status:  store.StatusHealthy,
	}

	srv := newTestServer(s)
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"name":    "web",
		"image":   "nginx:1",
		"envs":    map[string]string{"PORT": "80"},
		"ports":   []string{"80"},
		"volumes": []string{"/data:/data"},
		"domain":  "app.example.com",
		"public":  true,
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

	if !updated.Public {
		t.Fatalf("want public=true, got false")
	}
	if updated.Status != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", updated.Status)
	}
}

func TestUpdateDeployment_PrivateDomainMustMatchAuthCookieDomain(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:      "d1",
		Name:    "web",
		Image:   "nginx:1",
		Domain:  "app.example.com",
		Public:  true,
		Status:  store.StatusHealthy,
		Envs:    map[string]string{},
		Ports:   []string{"32768:80"},
		Volumes: []string{},
	}

	srv := newTestServerWithAuthCookieDomain(s, "example.com")
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"name":    "web",
		"image":   "nginx:1",
		"domain":  "app.other.dev",
		"public":  false,
		"envs":    map[string]string{},
		"ports":   []string{"80"},
		"volumes": []string{},
	})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/deployments/d1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/deployments/d1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
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

func TestUpdateDeployment_DomainOnly_DoesNotRedeploy(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:      "d1",
		Name:    "web",
		Image:   "nginx:1",
		Envs:    map[string]string{"PORT": "80"},
		Ports:   []string{"80:80"},
		Volumes: []string{"/data:/data"},
		Domain:  "old.example.com",
		Status:  store.StatusHealthy,
	}

	broker := events.NewBroker()
	srv := newTestServerWithBroker(s, broker)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	evtCh := readSSEEvents(ctx, t, srv.URL+"/api/deployments/events")
	time.Sleep(50 * time.Millisecond)

	body, _ := json.Marshal(map[string]any{
		"name":    "web",
		"image":   "nginx:1",
		"envs":    map[string]string{"PORT": "80"},
		"ports":   []string{"80:80"},
		"volumes": []string{"/data:/data"},
		"domain":  "new.example.com",
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
	if updated.Domain != "new.example.com" {
		t.Errorf("want domain new.example.com, got %s", updated.Domain)
	}
	if updated.Status != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", updated.Status)
	}

	select {
	case event := <-evtCh:
		t.Fatalf("unexpected SSE event for domain-only update: %+v", event)
	case <-time.After(250 * time.Millisecond):
	}
}

func TestUpdateDeployment_DashboardDomainConflict(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_DOMAIN", "dashboard.example.com")

	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:     "d1",
		Name:   "web",
		Image:  "nginx:1",
		Domain: "app.example.com",
		Status: store.StatusHealthy,
	}

	srv := newTestServer(s)
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"name":   "web",
		"image":  "nginx:1",
		"domain": "dashboard.example.com",
	})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/deployments/d1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/deployments/d1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", resp.StatusCode)
	}
}

func TestRestartDeployment(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{
		ID:     "d1",
		Name:   "web",
		Image:  "nginx:1",
		Public: true,
		Status: store.StatusHealthy,
	}

	broker := events.NewBroker()
	srv := newTestServerWithBroker(s, broker)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	evtCh := readSSEEvents(ctx, t, srv.URL+"/api/deployments/events")
	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/deployments/d1/restart", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/deployments/d1/restart: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", resp.StatusCode)
	}

	var updated store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.Status != store.StatusDeploying {
		t.Errorf("want status deploying, got %s", updated.Status)
	}
	if !updated.Public {
		t.Errorf("want visibility preserved as public after restart")
	}

	select {
	case event := <-evtCh:
		if event.DeploymentID != "d1" {
			t.Errorf("want deployment id d1, got %s", event.DeploymentID)
		}
		if event.Status != string(store.StatusDeploying) {
			t.Errorf("want status deploying, got %s", event.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: no SSE event after restart")
	}
}

func TestRestartDeployment_NotFound(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/deployments/nonexistent/restart", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/deployments/nonexistent/restart: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestRestartDeployment_StoreError(t *testing.T) {
	srv := newTestServer(&errStore{})
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/deployments/any/restart", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/deployments/any/restart: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestCoreServiceLogs_OK(t *testing.T) {
	setFakeJournalctl(t, "printf 'line one\nline two\n'")

	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/core-services/logs?service=api&tail=2")
	if err != nil {
		t.Fatalf("GET /api/core-services/logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body struct {
		Service string   `json:"service"`
		Lines   []string `json:"lines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Service != "api" {
		t.Fatalf("want service api, got %q", body.Service)
	}
	if len(body.Lines) != 2 {
		t.Fatalf("want 2 log lines, got %d", len(body.Lines))
	}
	if body.Lines[0] != "line one" || body.Lines[1] != "line two" {
		t.Fatalf("unexpected log lines: %#v", body.Lines)
	}
}

func TestCoreServiceLogs_InvalidService(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/core-services/logs?service=unknown")
	if err != nil {
		t.Fatalf("GET /api/core-services/logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestCoreServiceLogs_JournalctlError(t *testing.T) {
	setFakeJournalctl(t, "echo 'boom' >&2; exit 1")

	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/core-services/logs?service=proxy")
	if err != nil {
		t.Fatalf("GET /api/core-services/logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

func TestCoreServiceLogs_InvalidTail(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/core-services/logs?service=api&tail=abc")
	if err != nil {
		t.Fatalf("GET /api/core-services/logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func setFakeJournalctl(t *testing.T, body string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "journalctl")
	script := "#!/usr/bin/env bash\nset -euo pipefail\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake journalctl: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
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

func TestDeploymentRecentLogs_NotFound(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments/nonexistent/logs/recent")
	if err != nil {
		t.Fatalf("GET /api/deployments/nonexistent/logs/recent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestDeploymentRecentLogs_OK(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	srv := newTestServerWithDockerLogs(s, &stubDockerLogs{recentLogs: []string{"booting", "ready"}})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments/d1/logs/recent?tail=2")
	if err != nil {
		t.Fatalf("GET /api/deployments/d1/logs/recent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body struct {
		DeploymentID string   `json:"deploymentId"`
		Lines        []string `json:"lines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.DeploymentID != "d1" {
		t.Fatalf("want deploymentId d1, got %q", body.DeploymentID)
	}
	if len(body.Lines) != 2 || body.Lines[0] != "booting" || body.Lines[1] != "ready" {
		t.Fatalf("unexpected lines: %#v", body.Lines)
	}
}

func TestDeploymentRecentLogs_InvalidTail(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	srv := newTestServerWithDockerLogs(s, &stubDockerLogs{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments/d1/logs/recent?tail=abc")
	if err != nil {
		t.Fatalf("GET /api/deployments/d1/logs/recent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestDeploymentRecentLogs_DockerError(t *testing.T) {
	s := newMemStore()
	s.deployments["d1"] = store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy}

	srv := newTestServerWithDockerLogs(s, &stubDockerLogs{recentErr: errors.New("docker unavailable")})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/deployments/d1/logs/recent")
	if err != nil {
		t.Fatalf("GET /api/deployments/d1/logs/recent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", resp.StatusCode)
	}
}

// portHostPart returns the host portion of a "host:container" port binding.
func portHostPart(t *testing.T, binding string) int {
	t.Helper()
	idx := strings.Index(binding, ":")
	if idx == -1 {
		t.Fatalf("port binding %q has no colon", binding)
	}
	n, err := strconv.Atoi(binding[:idx])
	if err != nil {
		t.Fatalf("port binding %q: host part is not a number: %v", binding, err)
	}
	return n
}

func TestCreateDeployment_AutoAssignsHostPort(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"name":  "web",
		"image": "nginx:latest",
		"ports": []string{"80"},
	})
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

	if len(created.Ports) != 1 {
		t.Fatalf("want 1 port binding, got %d", len(created.Ports))
	}
	hp := portHostPart(t, created.Ports[0])
	if hp < 32768 || hp > 60999 {
		t.Errorf("host port %d not in auto-assigned range [32768, 60999]", hp)
	}
}

func TestCreateDeployment_StripsUserHostPort(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"name":  "web",
		"image": "nginx:latest",
		"ports": []string{"8080:80"},
	})
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

	if len(created.Ports) != 1 {
		t.Fatalf("want 1 port binding, got %d", len(created.Ports))
	}
	hp := portHostPart(t, created.Ports[0])
	if hp == 8080 {
		t.Errorf("user-specified host port 8080 was not stripped; got binding %s", created.Ports[0])
	}
}

func TestCreateDeployment_NoPortConflict(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	post := func(name string) store.Deployment {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"name":  name,
			"image": "nginx:latest",
			"ports": []string{"80"},
		})
		resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST /api/deployments (%s): %v", name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST (%s): want 201, got %d", name, resp.StatusCode)
		}
		var d store.Deployment
		if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
			t.Fatalf("decode (%s): %v", name, err)
		}
		return d
	}

	first := post("web")
	second := post("api")

	if len(first.Ports) != 1 || len(second.Ports) != 1 {
		t.Fatalf("want 1 port each, got %d and %d", len(first.Ports), len(second.Ports))
	}

	hp1 := portHostPart(t, first.Ports[0])
	hp2 := portHostPart(t, second.Ports[0])
	if hp1 == hp2 {
		t.Errorf("both deployments got the same host port %d — conflict not prevented", hp1)
	}
}

func TestUpdateDeployment_HostPortStableOnRedeploy(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	// Create initial deployment.
	createBody, _ := json.Marshal(map[string]any{
		"name":  "web",
		"image": "nginx:1",
		"ports": []string{"80"},
	})
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	var created store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	resp.Body.Close()

	if len(created.Ports) != 1 {
		t.Fatalf("want 1 port after create, got %d", len(created.Ports))
	}
	originalHostPort := portHostPart(t, created.Ports[0])

	// Update only the image — host port must be preserved.
	updateBody, _ := json.Marshal(map[string]any{
		"name":  "web",
		"image": "nginx:2",
		"ports": []string{"80"},
	})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/deployments/"+created.ID, bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/deployments/%s: %v", created.ID, err)
	}
	var updated store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	resp.Body.Close()

	if len(updated.Ports) != 1 {
		t.Fatalf("want 1 port after update, got %d", len(updated.Ports))
	}
	updatedHostPort := portHostPart(t, updated.Ports[0])
	if updatedHostPort != originalHostPort {
		t.Errorf("host port changed from %d to %d — should be stable", originalHostPort, updatedHostPort)
	}
}

func TestAccessLogs_ProxiesProxyLogs(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/access-logs" {
			t.Fatalf("want /internal/access-logs path, got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]api.AccessLogEntry{{Method: http.MethodGet, Path: "/", StatusCode: 200}})
	}))
	defer proxy.Close()

	t.Setenv("LOTSEN_PROXY_INTERNAL_URL", proxy.URL)
	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/access-logs?limit=5")
	if err != nil {
		t.Fatalf("GET /api/access-logs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var entries []api.AccessLogEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want one entry, got %d", len(entries))
	}
}

func TestSecurityConfig_ProxiesProxySettings(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/security-config" {
			t.Fatalf("want /internal/security-config path, got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(api.SecurityConfig{Profile: "standard", SuspiciousThreshold: 12})
	}))
	defer proxy.Close()

	t.Setenv("LOTSEN_PROXY_INTERNAL_URL", proxy.URL)
	srv := newTestServer(newMemStore())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/security-config")
	if err != nil {
		t.Fatalf("GET /api/security-config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var cfg api.SecurityConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if cfg.Profile != "standard" {
		t.Fatalf("want profile standard, got %s", cfg.Profile)
	}
}

func TestCreateDeployment_HashesBasicAuthPasswords(t *testing.T) {
	s := newMemStore()
	srv := newTestServer(s)
	defer srv.Close()

	body := []byte(`{"name":"web","image":"nginx:latest","envs":{},"ports":["80"],"volumes":[],"domain":"app.example.com","basic_auth":{"users":[{"username":"admin","password":"secret"}]}}`)
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
	if created.BasicAuth == nil || len(created.BasicAuth.Users) != 1 {
		t.Fatalf("want 1 basic auth user, got %#v", created.BasicAuth)
	}
	if created.BasicAuth.Users[0].Password == "secret" {
		t.Fatal("want password hash, got plaintext")
	}
	if !strings.HasPrefix(created.BasicAuth.Users[0].Password, "$2") {
		t.Fatalf("want bcrypt hash prefix, got %q", created.BasicAuth.Users[0].Password)
	}
}

func TestCreateDeployment_RejectsEmptyBasicAuthUsername(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	body := []byte(`{"name":"web","image":"nginx:latest","envs":{},"ports":["80"],"volumes":[],"domain":"app.example.com","basic_auth":{"users":[{"username":"","password":"secret"}]}}`)
	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestCreateDeployment_PersistsSecurityConfig(t *testing.T) {
	s := newMemStore()
	srv := newTestServer(s)
	defer srv.Close()

	body := []byte(`{"name":"web","image":"nginx:latest","envs":{},"ports":["80"],"volumes":[],"domain":"app.example.com","security":{"waf_enabled":true,"ip_denylist":["10.0.0.0/8"],"ip_allowlist":["203.0.113.0/24"],"custom_rules":["SecRule REQUEST_URI \"@contains blocked\" \"id:10001,phase:1,deny,status:403\""]}}`)
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
	if created.Security == nil || !created.Security.WAFEnabled {
		t.Fatalf("want security config persisted, got %#v", created.Security)
	}
	if created.Security.WAFMode != "detection" {
		t.Fatalf("want default waf mode detection, got %q", created.Security.WAFMode)
	}
	if len(created.Security.IPAllowlist) != 1 || created.Security.IPAllowlist[0] != "203.0.113.0/24" {
		t.Fatalf("want ip allowlist persisted, got %#v", created.Security)
	}
}

func TestPatchDeployment_UpdatesSecurityConfig(t *testing.T) {
	s := newMemStore()
	srv := newTestServer(s)
	defer srv.Close()

	createBody := []byte(`{"name":"web","image":"nginx:1","envs":{},"ports":["80"],"volumes":[]}`)
	createResp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", createResp.StatusCode)
	}

	var created store.Deployment
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	patchBody := []byte(`{"security":{"waf_enabled":true,"ip_denylist":["10.0.0.0/8"]}}`)
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/deployments/"+created.ID, bytes.NewReader(patchBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/deployments/%s: %v", created.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", resp.StatusCode)
	}

	var updated store.Deployment
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if updated.Security == nil || !updated.Security.WAFEnabled {
		t.Fatalf("want security config updated, got %#v", updated.Security)
	}
	if updated.Security.WAFMode != "detection" {
		t.Fatalf("want default waf mode detection on patch, got %q", updated.Security.WAFMode)
	}
}

func TestRegistries_CRUD(t *testing.T) {
	srv := newTestServer(newMemStore())
	defer srv.Close()

	body := []byte(`{"prefix":"ghcr.io/myorg","username":"alice","password":"secret-token"}`)
	createReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/registries", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("POST /api/registries: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", createResp.StatusCode)
	}

	var created store.RegistryEntry
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("want created id")
	}

	listResp, err := http.Get(srv.URL + "/api/registries")
	if err != nil {
		t.Fatalf("GET /api/registries: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", listResp.StatusCode)
	}
	var listed struct {
		Registries []store.RegistryEntry `json:"registries"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Registries) != 1 {
		t.Fatalf("want 1 registry, got %d", len(listed.Registries))
	}

	updateBody := []byte(`{"prefix":"ghcr.io/myorg/platform","username":"bob"}`)
	updateReq, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/registries/"+created.ID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		t.Fatalf("PUT /api/registries/{id}: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", updateResp.StatusCode)
	}

	deleteReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/registries/"+created.ID, nil)
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("DELETE /api/registries/{id}: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", deleteResp.StatusCode)
	}
}

func TestCreateRegistry_DuplicatePrefixReturnsConflict(t *testing.T) {
	s := newMemStore()
	_, _ = s.CreateRegistry("r1", "ghcr.io/myorg", "alice", "secret")

	srv := newTestServer(s)
	defer srv.Close()

	body := []byte(`{"prefix":"ghcr.io/myorg","username":"bob","password":"another"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/registries", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/registries: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", resp.StatusCode)
	}
}
