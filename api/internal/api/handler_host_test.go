package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/internal/events"
)

func newTestServerWithHostProfileStore(t *testing.T, storePath string) *httptest.Server {
	t.Helper()

	hostProfiles, err := api.NewFileHostProfileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileHostProfileStore: %v", err)
	}

	h := api.New(newMemStore(), events.NewBroker(), noopDockerLogs{})
	h.SetHostProfileStore(hostProfiles)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func TestHost_GetAndUpdateDisplayName(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithHostProfileStore(t, filepath.Join(t.TempDir(), "host_profile.json"))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/host")
	if err != nil {
		t.Fatalf("GET /api/host: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var initial struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&initial); err != nil {
		t.Fatalf("decode host response: %v", err)
	}
	if initial.DisplayName != "" {
		t.Fatalf("want empty initial display name, got %q", initial.DisplayName)
	}

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/host", bytes.NewBufferString(`{"displayName":"prod-eu"}`))
	if err != nil {
		t.Fatalf("build PUT /api/host request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/host: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", updateResp.StatusCode)
	}

	var updated struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated host response: %v", err)
	}
	if updated.DisplayName != "prod-eu" {
		t.Fatalf("want updated display name prod-eu, got %q", updated.DisplayName)
	}

	verifyResp, err := http.Get(srv.URL + "/api/host")
	if err != nil {
		t.Fatalf("GET /api/host after update: %v", err)
	}
	defer verifyResp.Body.Close()

	var persisted struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(verifyResp.Body).Decode(&persisted); err != nil {
		t.Fatalf("decode persisted host response: %v", err)
	}
	if persisted.DisplayName != "prod-eu" {
		t.Fatalf("want persisted display name prod-eu, got %q", persisted.DisplayName)
	}
}

func TestHost_MetadataFromHeartbeat(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithHostProfileStore(t, filepath.Join(t.TempDir(), "host_profile.json"))
	defer srv.Close()

	body := `{"host":{"metadata":{"ipAddress":"10.0.0.5","osName":"Ubuntu","osVersion":"24.04","specs":{"cpuCores":4,"memoryBytes":8589934592,"diskBytes":68719476736}}}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/system-status/orchestrator-heartbeat", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("build heartbeat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	heartbeatResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST orchestrator heartbeat: %v", err)
	}
	heartbeatResp.Body.Close()
	if heartbeatResp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204 from heartbeat endpoint, got %d", heartbeatResp.StatusCode)
	}

	resp, err := http.Get(srv.URL + "/api/host")
	if err != nil {
		t.Fatalf("GET /api/host: %v", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Metadata *struct {
			IPAddress string `json:"ipAddress"`
			OSName    string `json:"osName"`
			OSVersion string `json:"osVersion"`
			Specs     struct {
				CPUCores    int    `json:"cpuCores"`
				MemoryBytes uint64 `json:"memoryBytes"`
				DiskBytes   uint64 `json:"diskBytes"`
			} `json:"specs"`
		} `json:"metadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode host payload: %v", err)
	}

	if payload.Metadata == nil {
		t.Fatal("want metadata in host payload")
	}
	if payload.Metadata.IPAddress != "10.0.0.5" {
		t.Fatalf("want ip 10.0.0.5, got %q", payload.Metadata.IPAddress)
	}
	if payload.Metadata.Specs.CPUCores != 4 {
		t.Fatalf("want cpu cores 4, got %d", payload.Metadata.Specs.CPUCores)
	}
}
