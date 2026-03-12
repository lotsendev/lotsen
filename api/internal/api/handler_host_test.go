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
		DisplayName         string `json:"displayName"`
		DashboardAccessMode string `json:"dashboardAccessMode"`
		DashboardWAF        struct {
			Mode        string   `json:"mode"`
			IPAllowlist []string `json:"ipAllowlist"`
			CustomRules []string `json:"customRules"`
		} `json:"dashboardWaf"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&initial); err != nil {
		t.Fatalf("decode host response: %v", err)
	}
	if initial.DisplayName != "" {
		t.Fatalf("want empty initial display name, got %q", initial.DisplayName)
	}
	if initial.DashboardAccessMode != "login_only" {
		t.Fatalf("want default dashboardAccessMode login_only, got %q", initial.DashboardAccessMode)
	}
	if initial.DashboardWAF.Mode != "detection" {
		t.Fatalf("want default dashboardWaf.mode detection, got %q", initial.DashboardWAF.Mode)
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
		DisplayName         string `json:"displayName"`
		DashboardAccessMode string `json:"dashboardAccessMode"`
		DashboardWAF        struct {
			Mode        string   `json:"mode"`
			IPAllowlist []string `json:"ipAllowlist"`
			CustomRules []string `json:"customRules"`
		} `json:"dashboardWaf"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated host response: %v", err)
	}
	if updated.DisplayName != "prod-eu" {
		t.Fatalf("want updated display name prod-eu, got %q", updated.DisplayName)
	}
	if updated.DashboardAccessMode != "login_only" {
		t.Fatalf("want dashboardAccessMode login_only, got %q", updated.DashboardAccessMode)
	}
	if updated.DashboardWAF.Mode != "detection" {
		t.Fatalf("want dashboardWaf.mode detection, got %q", updated.DashboardWAF.Mode)
	}

	verifyResp, err := http.Get(srv.URL + "/api/host")
	if err != nil {
		t.Fatalf("GET /api/host after update: %v", err)
	}
	defer verifyResp.Body.Close()

	var persisted struct {
		DisplayName         string `json:"displayName"`
		DashboardAccessMode string `json:"dashboardAccessMode"`
		DashboardWAF        struct {
			Mode        string   `json:"mode"`
			IPAllowlist []string `json:"ipAllowlist"`
			CustomRules []string `json:"customRules"`
		} `json:"dashboardWaf"`
	}
	if err := json.NewDecoder(verifyResp.Body).Decode(&persisted); err != nil {
		t.Fatalf("decode persisted host response: %v", err)
	}
	if persisted.DisplayName != "prod-eu" {
		t.Fatalf("want persisted display name prod-eu, got %q", persisted.DisplayName)
	}
	if persisted.DashboardAccessMode != "login_only" {
		t.Fatalf("want persisted dashboardAccessMode login_only, got %q", persisted.DashboardAccessMode)
	}
	if persisted.DashboardWAF.Mode != "detection" {
		t.Fatalf("want persisted dashboardWaf.mode detection, got %q", persisted.DashboardWAF.Mode)
	}
}

func TestHost_UpdateDashboardAccessMode(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithHostProfileStore(t, filepath.Join(t.TempDir(), "host_profile.json"))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/host", bytes.NewBufferString(`{"dashboardAccessMode":"waf_and_login"}`))
	if err != nil {
		t.Fatalf("build PUT /api/host request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/host: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var updated struct {
		DashboardAccessMode string `json:"dashboardAccessMode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated host response: %v", err)
	}
	if updated.DashboardAccessMode != "waf_and_login" {
		t.Fatalf("want updated dashboardAccessMode waf_and_login, got %q", updated.DashboardAccessMode)
	}
}

func TestHost_UpdateDashboardWAFConfig(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithHostProfileStore(t, filepath.Join(t.TempDir(), "host_profile.json"))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/host", bytes.NewBufferString(`{"dashboardWaf":{"mode":"enforcement","ipAllowlist":["203.0.113.0/24"],"customRules":["SecRule REQUEST_URI \"@contains blocked\" \"id:10010,phase:1,deny,status:403\""]}}`))
	if err != nil {
		t.Fatalf("build PUT /api/host request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/host: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var updated struct {
		DashboardWAF struct {
			Mode        string   `json:"mode"`
			IPAllowlist []string `json:"ipAllowlist"`
			CustomRules []string `json:"customRules"`
		} `json:"dashboardWaf"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated host response: %v", err)
	}
	if updated.DashboardWAF.Mode != "enforcement" {
		t.Fatalf("want dashboardWaf.mode enforcement, got %q", updated.DashboardWAF.Mode)
	}
	if len(updated.DashboardWAF.CustomRules) != 1 {
		t.Fatalf("want 1 custom rule, got %d", len(updated.DashboardWAF.CustomRules))
	}
	if len(updated.DashboardWAF.IPAllowlist) != 1 || updated.DashboardWAF.IPAllowlist[0] != "203.0.113.0/24" {
		t.Fatalf("want ipAllowlist [203.0.113.0/24], got %#v", updated.DashboardWAF.IPAllowlist)
	}
}

func TestHost_UpdateDashboardWAFConfigRejectsInvalidAllowlist(t *testing.T) {
	t.Parallel()

	srv := newTestServerWithHostProfileStore(t, filepath.Join(t.TempDir(), "host_profile.json"))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/host", bytes.NewBufferString(`{"dashboardWaf":{"ipAllowlist":["not-a-cidr"]}}`))
	if err != nil {
		t.Fatalf("build PUT /api/host request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/host: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
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
