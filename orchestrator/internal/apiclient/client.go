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
	At     time.Time            `json:"at"`
	Docker heartbeatDockerState `json:"docker"`
}

type heartbeatDockerState struct {
	Reachable bool      `json:"reachable"`
	CheckedAt time.Time `json:"checkedAt"`
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
func (c *Client) NotifyStatus(id string, status store.Status) error {
	body, err := json.Marshal(struct {
		Status store.Status `json:"status"`
	}{Status: status})
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
// API can update orchestrator liveness and Docker connectivity signals.
func (c *Client) NotifyHeartbeat(dockerReachable bool, checkedAt time.Time) error {
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	} else {
		checkedAt = checkedAt.UTC()
	}

	body, err := json.Marshal(heartbeatRequest{
		At: checkedAt,
		Docker: heartbeatDockerState{
			Reachable: dockerReachable,
			CheckedAt: checkedAt,
		},
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
