package api

import (
	"context"
	"net/http"
	"time"
)

// SystemStatusState is the normalized state enum used by status signals.
type SystemStatusState string

const (
	SystemStatusStateHealthy     SystemStatusState = "healthy"
	SystemStatusStateUnavailable SystemStatusState = "unavailable"
)

// APISystemStatus carries API availability and freshness information.
type APISystemStatus struct {
	State       SystemStatusState `json:"state"`
	LastUpdated time.Time         `json:"lastUpdated"`
}

// SystemStatusSnapshot is the typed dashboard-facing system-status contract.
type SystemStatusSnapshot struct {
	API   APISystemStatus `json:"api"`
	Error string          `json:"error,omitempty"`
}

// SystemStatusProvider returns an aggregated system-status snapshot.
type SystemStatusProvider interface {
	Snapshot(ctx context.Context) (SystemStatusSnapshot, error)
}

type defaultSystemStatusProvider struct {
	now func() time.Time
}

func newDefaultSystemStatusProvider(now func() time.Time) SystemStatusProvider {
	return &defaultSystemStatusProvider{now: now}
}

func (p *defaultSystemStatusProvider) Snapshot(_ context.Context) (SystemStatusSnapshot, error) {
	return SystemStatusSnapshot{
		API: APISystemStatus{
			State:       SystemStatusStateHealthy,
			LastUpdated: p.now().UTC(),
		},
	}, nil
}

func (h *Handler) systemStatus(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.statusSource.Snapshot(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, SystemStatusSnapshot{
			API: APISystemStatus{
				State:       SystemStatusStateUnavailable,
				LastUpdated: time.Now().UTC(),
			},
			Error: "system status unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}
