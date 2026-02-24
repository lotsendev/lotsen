package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ercadev/dirigent/store"
)

// Client calls the Dirigent API to notify it of deployment status transitions.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

type heartbeatRequest struct {
	At           time.Time                  `json:"at"`
	Orchestrator heartbeatOrchestratorState `json:"orchestrator"`
	Docker       heartbeatDockerState       `json:"docker"`
	LoadBalancer heartbeatLoadBalancerState `json:"loadBalancer"`
	Host         *heartbeatHostState        `json:"host,omitempty"`
}

type heartbeatOrchestratorState struct {
	StoreAccessible bool `json:"storeAccessible"`
}

type heartbeatDockerState struct {
	Reachable bool      `json:"reachable"`
	CheckedAt time.Time `json:"checkedAt"`
}

type heartbeatLoadBalancerState struct {
	Responding bool                          `json:"responding"`
	CheckedAt  time.Time                     `json:"checkedAt"`
	Traffic    *HeartbeatLoadBalancerTraffic `json:"traffic,omitempty"`
}

type HeartbeatLoadBalancerTraffic struct {
	TotalRequests      int64                                 `json:"totalRequests"`
	SuspiciousRequests int64                                 `json:"suspiciousRequests"`
	BlockedRequests    int64                                 `json:"blockedRequests"`
	ActiveBlockedIPs   int                                   `json:"activeBlockedIps"`
	BlockedIPs         []HeartbeatLoadBalancerBlockedIPState `json:"blockedIps,omitempty"`
}

type HeartbeatLoadBalancerBlockedIPState struct {
	IP           string     `json:"ip"`
	BlockedUntil *time.Time `json:"blockedUntil,omitempty"`
}

type heartbeatHostState struct {
	CPU *heartbeatHostMetric `json:"cpu,omitempty"`
	RAM *heartbeatHostMetric `json:"ram,omitempty"`
}

type heartbeatHostMetric struct {
	UsagePercent float64   `json:"usagePercent"`
	CheckedAt    time.Time `json:"checkedAt"`
}

// New creates a Client that targets the given API base URL (e.g. "http://localhost:8080").
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// NotifyStatus calls PATCH /api/deployments/{id}/status so the API's event
// broker emits an SSE event to all connected clients.
func (c *Client) NotifyStatus(id string, status store.Status, errorMessage string) error {
	body, err := json.Marshal(struct {
		Status store.Status `json:"status"`
		Error  string       `json:"error,omitempty"`
	}{Status: status, Error: errorMessage})
	if err != nil {
		return fmt.Errorf("apiclient: marshal body: %w", err)
	}

	url := fmt.Sprintf("%s/api/deployments/%s/status", c.baseURL, id)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apiclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("apiclient: patch status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apiclient: patch status: unexpected response %d", resp.StatusCode)
	}

	return nil
}

// NotifyHeartbeat calls POST /api/system-status/orchestrator-heartbeat so the
// API can update orchestrator liveness, Docker connectivity, and host metrics.
func (c *Client) NotifyHeartbeat(dockerReachable bool, loadBalancerResponding bool, loadBalancerTraffic *HeartbeatLoadBalancerTraffic, storeAccessible bool, checkedAt time.Time, cpuUsagePercent *float64, ramUsagePercent *float64) error {
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	} else {
		checkedAt = checkedAt.UTC()
	}

	host := buildHeartbeatHostState(checkedAt, cpuUsagePercent, ramUsagePercent)

	body, err := json.Marshal(heartbeatRequest{
		At: checkedAt,
		Orchestrator: heartbeatOrchestratorState{
			StoreAccessible: storeAccessible,
		},
		Docker: heartbeatDockerState{
			Reachable: dockerReachable,
			CheckedAt: checkedAt,
		},
		LoadBalancer: heartbeatLoadBalancerState{
			Responding: loadBalancerResponding,
			CheckedAt:  checkedAt,
			Traffic:    cloneHeartbeatLoadBalancerTraffic(loadBalancerTraffic),
		},
		Host: host,
	})
	if err != nil {
		return fmt.Errorf("apiclient: marshal heartbeat body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/system-status/orchestrator-heartbeat", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apiclient: build heartbeat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("apiclient: post heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("apiclient: post heartbeat: unexpected response %d", resp.StatusCode)
	}

	return nil
}

func cloneHeartbeatLoadBalancerTraffic(in *HeartbeatLoadBalancerTraffic) *HeartbeatLoadBalancerTraffic {
	if in == nil {
		return nil
	}

	out := *in
	if len(in.BlockedIPs) > 0 {
		out.BlockedIPs = make([]HeartbeatLoadBalancerBlockedIPState, len(in.BlockedIPs))
		copy(out.BlockedIPs, in.BlockedIPs)
	}

	return &out
}

func buildHeartbeatHostState(checkedAt time.Time, cpuUsagePercent *float64, ramUsagePercent *float64) *heartbeatHostState {
	host := &heartbeatHostState{}

	if cpuUsagePercent != nil {
		host.CPU = &heartbeatHostMetric{UsagePercent: *cpuUsagePercent, CheckedAt: checkedAt}
	}
	if ramUsagePercent != nil {
		host.RAM = &heartbeatHostMetric{UsagePercent: *ramUsagePercent, CheckedAt: checkedAt}
	}

	if host.CPU == nil && host.RAM == nil {
		return nil
	}

	return host
}
