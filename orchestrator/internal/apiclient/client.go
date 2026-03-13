package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lotsendev/lotsen/store"
)

// Client calls the Lotsen API to notify it of deployment status transitions.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

type heartbeatRequest struct {
	At             time.Time                          `json:"at"`
	Orchestrator   heartbeatOrchestratorState         `json:"orchestrator"`
	Docker         heartbeatDockerState               `json:"docker"`
	LoadBalancer   heartbeatLoadBalancerState         `json:"loadBalancer"`
	Host           *heartbeatHostState                `json:"host,omitempty"`
	ContainerStats map[string]HeartbeatContainerStats `json:"containerStats"`
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
	WAFBlockedRequests int64                                 `json:"wafBlockedRequests"`
	UABlockedRequests  int64                                 `json:"uaBlockedRequests"`
	ActiveBlockedIPs   int                                   `json:"activeBlockedIps"`
	BlockedIPs         []HeartbeatLoadBalancerBlockedIPState `json:"blockedIps,omitempty"`
}

type HeartbeatLoadBalancerBlockedIPState struct {
	IP           string     `json:"ip"`
	BlockedUntil *time.Time `json:"blockedUntil,omitempty"`
}

type heartbeatHostState struct {
	CPU      *heartbeatHostMetric   `json:"cpu,omitempty"`
	RAM      *heartbeatHostMetric   `json:"ram,omitempty"`
	Metadata *HeartbeatHostMetadata `json:"metadata,omitempty"`
}

type heartbeatHostMetric struct {
	UsagePercent float64   `json:"usagePercent"`
	CheckedAt    time.Time `json:"checkedAt"`
}

type HeartbeatHostSpecs struct {
	CPUCores    int    `json:"cpuCores,omitempty"`
	MemoryBytes uint64 `json:"memoryBytes,omitempty"`
	DiskBytes   uint64 `json:"diskBytes,omitempty"`
}

type HeartbeatHostMetadata struct {
	IPAddress string             `json:"ipAddress,omitempty"`
	OSName    string             `json:"osName,omitempty"`
	OSVersion string             `json:"osVersion,omitempty"`
	Specs     HeartbeatHostSpecs `json:"specs,omitempty"`
}

type HeartbeatContainerStats struct {
	CPUPercent       float64 `json:"cpuPercent"`
	MemoryUsedBytes  uint64  `json:"memoryUsedBytes"`
	MemoryLimitBytes uint64  `json:"memoryLimitBytes"`
	MemoryPercent    float64 `json:"memoryPercent"`
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
func (c *Client) NotifyStatus(id string, status store.Status, reason store.StatusReason, errorMessage string) error {
	body, err := json.Marshal(struct {
		Status store.Status       `json:"status"`
		Reason store.StatusReason `json:"reason,omitempty"`
		Error  string             `json:"error,omitempty"`
	}{Status: status, Reason: reason, Error: errorMessage})
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
func (c *Client) NotifyHeartbeat(dockerReachable bool, loadBalancerResponding bool, loadBalancerTraffic *HeartbeatLoadBalancerTraffic, storeAccessible bool, checkedAt time.Time, cpuUsagePercent *float64, ramUsagePercent *float64, hostMetadata *HeartbeatHostMetadata, containerStats map[string]HeartbeatContainerStats) error {
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	} else {
		checkedAt = checkedAt.UTC()
	}

	host := buildHeartbeatHostState(checkedAt, cpuUsagePercent, ramUsagePercent, hostMetadata)

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
		Host:           host,
		ContainerStats: cloneHeartbeatContainerStats(containerStats),
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

func cloneHeartbeatContainerStats(in map[string]HeartbeatContainerStats) map[string]HeartbeatContainerStats {
	if len(in) == 0 {
		return map[string]HeartbeatContainerStats{}
	}

	out := make(map[string]HeartbeatContainerStats, len(in))
	for deploymentID, stats := range in {
		out[deploymentID] = stats
	}

	return out
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

func buildHeartbeatHostState(checkedAt time.Time, cpuUsagePercent *float64, ramUsagePercent *float64, metadata *HeartbeatHostMetadata) *heartbeatHostState {
	host := &heartbeatHostState{}

	if cpuUsagePercent != nil {
		host.CPU = &heartbeatHostMetric{UsagePercent: *cpuUsagePercent, CheckedAt: checkedAt}
	}
	if ramUsagePercent != nil {
		host.RAM = &heartbeatHostMetric{UsagePercent: *ramUsagePercent, CheckedAt: checkedAt}
	}
	if metadata != nil {
		host.Metadata = cloneHeartbeatHostMetadata(metadata)
	}

	if host.CPU == nil && host.RAM == nil && host.Metadata == nil {
		return nil
	}

	return host
}

func cloneHeartbeatHostMetadata(in *HeartbeatHostMetadata) *HeartbeatHostMetadata {
	if in == nil {
		return nil
	}

	out := *in
	out.Specs = HeartbeatHostSpecs{
		CPUCores:    in.Specs.CPUCores,
		MemoryBytes: in.Specs.MemoryBytes,
		DiskBytes:   in.Specs.DiskBytes,
	}

	return &out
}
