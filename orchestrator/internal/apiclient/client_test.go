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
	traffic := &HeartbeatLoadBalancerTraffic{
		TotalRequests:      99,
		SuspiciousRequests: 12,
		BlockedRequests:    5,
		ActiveBlockedIPs:   1,
		BlockedIPs:         []HeartbeatLoadBalancerBlockedIPState{{IP: "203.0.113.7", BlockedUntil: &checkedAt}},
	}
	containerStats := map[string]HeartbeatContainerStats{
		"d1": {
			CPUPercent:       21.4,
			MemoryUsedBytes:  536870912,
			MemoryLimitBytes: 1073741824,
			MemoryPercent:    50,
		},
	}

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
				Traffic    *struct {
					TotalRequests      int64 `json:"totalRequests"`
					SuspiciousRequests int64 `json:"suspiciousRequests"`
					BlockedRequests    int64 `json:"blockedRequests"`
					ActiveBlockedIPs   int   `json:"activeBlockedIps"`
					BlockedIPs         []struct {
						IP           string     `json:"ip"`
						BlockedUntil *time.Time `json:"blockedUntil"`
					} `json:"blockedIps"`
				} `json:"traffic"`
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
			ContainerStats map[string]struct {
				CPUPercent       float64 `json:"cpuPercent"`
				MemoryUsedBytes  uint64  `json:"memoryUsedBytes"`
				MemoryLimitBytes uint64  `json:"memoryLimitBytes"`
				MemoryPercent    float64 `json:"memoryPercent"`
			} `json:"containerStats"`
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
		if body.LoadBalancer.Traffic == nil {
			t.Fatal("want load balancer traffic included")
		}
		if body.LoadBalancer.Traffic.TotalRequests != traffic.TotalRequests {
			t.Fatalf("want total requests %d, got %d", traffic.TotalRequests, body.LoadBalancer.Traffic.TotalRequests)
		}
		if len(body.LoadBalancer.Traffic.BlockedIPs) != 1 || body.LoadBalancer.Traffic.BlockedIPs[0].IP != "203.0.113.7" {
			t.Fatal("want blocked ip payload")
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
		if len(body.ContainerStats) != 1 {
			t.Fatalf("want 1 container stats entry, got %d", len(body.ContainerStats))
		}
		if body.ContainerStats["d1"].MemoryPercent != 50 {
			t.Fatalf("want memory percent 50, got %v", body.ContainerStats["d1"].MemoryPercent)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.NotifyHeartbeat(false, false, traffic, false, checkedAt, &cpu, &ram, containerStats); err != nil {
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
	if err := client.NotifyHeartbeat(false, true, nil, true, checkedAt, nil, nil, nil); err != nil {
		t.Fatalf("NotifyHeartbeat: %v", err)
	}
}

func TestClient_NotifyHeartbeat_UnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.NotifyHeartbeat(true, true, nil, true, time.Time{}, nil, nil, nil); err == nil {
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
