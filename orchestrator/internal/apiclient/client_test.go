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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("want POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/system-status/orchestrator-heartbeat" {
			t.Fatalf("want heartbeat path, got %s", r.URL.Path)
		}

		var body struct {
			At     time.Time `json:"at"`
			Docker struct {
				Reachable bool      `json:"reachable"`
				CheckedAt time.Time `json:"checkedAt"`
			} `json:"docker"`
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

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.NotifyHeartbeat(false, checkedAt); err != nil {
		t.Fatalf("NotifyHeartbeat: %v", err)
	}
}

func TestClient_NotifyHeartbeat_UnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.NotifyHeartbeat(true, time.Time{}); err == nil {
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
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := New(srv.URL)
	if err := client.NotifyStatus("d1", store.StatusHealthy); err != nil {
		t.Fatalf("NotifyStatus: %v", err)
	}
}
