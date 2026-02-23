package apiclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ercadev/dirigent/store"
)

func TestClient_NotifyHeartbeat(t *testing.T) {
	checkedAt := time.Date(2026, time.February, 22, 14, 0, 0, 0, time.UTC)
	cpu := 41.7
	ram := 68.9

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("want POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/system-status/orchestrator-heartbeat" {
			t.Fatalf("want heartbeat path, got %s", r.URL.Path)
		}

		var body struct {
			At           time.Time `json:"at"`
			Orchestrator struct {
				StoreAccessible bool `json:"storeAccessible"`
			} `json:"orchestrator"`
			Docker struct {
				Reachable bool      `json:"reachable"`
				CheckedAt time.Time `json:"checkedAt"`
			} `json:"docker"`
			LoadBalancer struct {
				Responding bool      `json:"responding"`
				CheckedAt  time.Time `json:"checkedAt"`
			} `json:"loadBalancer"`
			Host *struct {
				CPU *struct {
					UsagePercent float64   `json:"usagePercent"`
					CheckedAt    time.Time `json:"checkedAt"`
				} `json:"cpu"`
				RAM *struct {
					UsagePercent float64   `json:"usagePercent"`
					CheckedAt    time.Time `json:"checkedAt"`
				} `json:"ram"`
			} `json:"host"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if !body.At.Equal(checkedAt) {
			t.Fatalf("want at %s, got %s", checkedAt, body.At)
		}
		if body.Docker.Reachable {
			t.Fatal("want docker reachable false")
		}
		if !body.Docker.CheckedAt.Equal(checkedAt) {
			t.Fatalf("want checkedAt %s, got %s", checkedAt, body.Docker.CheckedAt)
		}
		if body.Orchestrator.StoreAccessible {
			t.Fatal("want orchestrator store accessible false")
		}
		if body.LoadBalancer.Responding {
			t.Fatal("want load balancer responding false")
		}
		if !body.LoadBalancer.CheckedAt.Equal(checkedAt) {
			t.Fatalf("want load balancer checkedAt %s, got %s", checkedAt, body.LoadBalancer.CheckedAt)
		}
		if body.Host == nil || body.Host.CPU == nil || body.Host.RAM == nil {
			t.Fatal("want host cpu and ram metrics in heartbeat")
		}
		if body.Host.CPU.UsagePercent != cpu {
			t.Fatalf("want cpu usage %v, got %v", cpu, body.Host.CPU.UsagePercent)
		}
		if !body.Host.CPU.CheckedAt.Equal(checkedAt) {
			t.Fatalf("want cpu checkedAt %s, got %s", checkedAt, body.Host.CPU.CheckedAt)
		}
		if body.Host.RAM.UsagePercent != ram {
			t.Fatalf("want ram usage %v, got %v", ram, body.Host.RAM.UsagePercent)
		}
		if !body.Host.RAM.CheckedAt.Equal(checkedAt) {
			t.Fatalf("want ram checkedAt %s, got %s", checkedAt, body.Host.RAM.CheckedAt)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.NotifyHeartbeat(false, false, false, checkedAt, &cpu, &ram); err != nil {
		t.Fatalf("NotifyHeartbeat: %v", err)
	}
}

func TestClient_NotifyHeartbeat_WithoutHostMetrics(t *testing.T) {
	checkedAt := time.Date(2026, time.February, 22, 14, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Host *struct {
				CPU any `json:"cpu"`
				RAM any `json:"ram"`
			} `json:"host"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Host != nil {
			t.Fatal("want host omitted when no host metrics are available")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.NotifyHeartbeat(false, true, true, checkedAt, nil, nil); err != nil {
		t.Fatalf("NotifyHeartbeat: %v", err)
	}
}

func TestClient_NotifyHeartbeat_UnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.NotifyHeartbeat(true, true, true, time.Time{}, nil, nil); err == nil {
		t.Fatal("want error for non-204 heartbeat response")
	}
}

func TestClient_NotifyStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("want PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/api/deployments/d1/status" {
			t.Fatalf("want path /api/deployments/d1/status, got %s", r.URL.Path)
		}
		var body struct {
			Status store.Status `json:"status"`
			Error  string       `json:"error"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Status != store.StatusHealthy {
			t.Fatalf("want healthy status, got %s", body.Status)
		}
		if body.Error != "" {
			t.Fatalf("want empty error, got %q", body.Error)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.NotifyStatus("d1", store.StatusHealthy, ""); err != nil {
		t.Fatalf("NotifyStatus: %v", err)
	}
}

func TestClient_NotifyStatus_WithError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Status store.Status `json:"status"`
			Error  string       `json:"error"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Status != store.StatusFailed {
			t.Fatalf("want failed status, got %s", body.Status)
		}
		if body.Error == "" {
			t.Fatal("want failure error message")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.NotifyStatus("d1", store.StatusFailed, "image not found"); err != nil {
		t.Fatalf("NotifyStatus: %v", err)
	}
}
