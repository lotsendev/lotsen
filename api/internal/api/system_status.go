package api

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// SystemStatusState is the normalized state enum used by status signals.
type SystemStatusState string

const (
	SystemStatusStateHealthy     SystemStatusState = "healthy"
	SystemStatusStateDegraded    SystemStatusState = "degraded"
	SystemStatusStateStale       SystemStatusState = "stale"
	SystemStatusStateUnavailable SystemStatusState = "unavailable"
)

// APISystemStatus carries API availability and freshness information.
type APISystemStatus struct {
	State       SystemStatusState `json:"state"`
	LastUpdated time.Time         `json:"lastUpdated"`
}

// OrchestratorSystemStatus carries orchestrator liveness and freshness information.
type OrchestratorSystemStatus struct {
	State       SystemStatusState `json:"state"`
	LastUpdated time.Time         `json:"lastUpdated,omitempty"`
}

// DockerSystemStatus carries Docker connectivity information as observed by the orchestrator.
type DockerSystemStatus struct {
	State       SystemStatusState `json:"state"`
	LastUpdated time.Time         `json:"lastUpdated,omitempty"`
}

// HostMetricSystemStatus carries a host metric signal value and freshness information.
type HostMetricSystemStatus struct {
	State        SystemStatusState `json:"state"`
	UsagePercent float64           `json:"usagePercent,omitempty"`
	LastUpdated  time.Time         `json:"lastUpdated,omitempty"`
}

// HostSystemStatus carries host-level runtime utilization signals.
type HostSystemStatus struct {
	CPU HostMetricSystemStatus `json:"cpu"`
	RAM HostMetricSystemStatus `json:"ram"`
}

// SystemStatusSnapshot is the typed dashboard-facing system-status contract.
type SystemStatusSnapshot struct {
	API          APISystemStatus          `json:"api"`
	Orchestrator OrchestratorSystemStatus `json:"orchestrator"`
	Docker       DockerSystemStatus       `json:"docker"`
	Host         HostSystemStatus         `json:"host"`
	Error        string                   `json:"error,omitempty"`
}

// SystemStatusProvider returns an aggregated system-status snapshot.
type SystemStatusProvider interface {
	Snapshot(ctx context.Context) (SystemStatusSnapshot, error)
}

// OrchestratorHeartbeatIngestor accepts orchestrator heartbeat updates.
type OrchestratorHeartbeatIngestor interface {
	RecordOrchestratorHeartbeat(ctx context.Context, at time.Time) error
}

// DockerConnectivityIngestor accepts Docker connectivity observations.
type DockerConnectivityIngestor interface {
	RecordDockerConnectivity(ctx context.Context, reachable bool, at time.Time) error
}

// CPUUtilizationIngestor accepts host CPU utilization observations.
type CPUUtilizationIngestor interface {
	RecordCPUUtilization(ctx context.Context, usagePercent float64, at time.Time) error
}

// RAMUtilizationIngestor accepts host RAM utilization observations.
type RAMUtilizationIngestor interface {
	RecordRAMUtilization(ctx context.Context, usagePercent float64, at time.Time) error
}

type defaultSystemStatusProvider struct {
	now              func() time.Time
	staleAfter       time.Duration
	lastHeartbeatMu  sync.RWMutex
	lastHeartbeatUTC time.Time
	dockerSignalMu   sync.RWMutex
	dockerSignal     dockerConnectivitySignal
	hostSignalMu     sync.RWMutex
	cpuSignal        hostMetricSignal
	ramSignal        hostMetricSignal
}

type dockerConnectivitySignal struct {
	lastCheckedUTC time.Time
	reachable      bool
	hasSignal      bool
}

type hostMetricSignal struct {
	lastCheckedUTC time.Time
	usagePercent   float64
	hasSignal      bool
}

func newDefaultSystemStatusProvider(now func() time.Time, staleAfter time.Duration) SystemStatusProvider {
	return &defaultSystemStatusProvider{now: now, staleAfter: staleAfter}
}

func (p *defaultSystemStatusProvider) Snapshot(_ context.Context) (SystemStatusSnapshot, error) {
	orchestrator := p.orchestratorStatus()

	return SystemStatusSnapshot{
		API: APISystemStatus{
			State:       SystemStatusStateHealthy,
			LastUpdated: p.now().UTC(),
		},
		Orchestrator: orchestrator,
		Docker:       p.dockerStatus(),
		Host: HostSystemStatus{
			CPU: p.cpuStatus(),
			RAM: p.ramStatus(),
		},
	}, nil
}

func (p *defaultSystemStatusProvider) RecordOrchestratorHeartbeat(_ context.Context, at time.Time) error {
	if at.IsZero() {
		at = p.now()
	}

	p.lastHeartbeatMu.Lock()
	p.lastHeartbeatUTC = at.UTC()
	p.lastHeartbeatMu.Unlock()

	return nil
}

func (p *defaultSystemStatusProvider) orchestratorStatus() OrchestratorSystemStatus {
	p.lastHeartbeatMu.RLock()
	lastHeartbeat := p.lastHeartbeatUTC
	p.lastHeartbeatMu.RUnlock()

	if lastHeartbeat.IsZero() {
		return OrchestratorSystemStatus{State: SystemStatusStateUnavailable}
	}

	age := p.now().UTC().Sub(lastHeartbeat)
	if age < 0 {
		age = 0
	}

	degradedAfter := p.staleAfter / 2
	state := SystemStatusStateStale

	switch {
	case age <= degradedAfter:
		state = SystemStatusStateHealthy
	case age <= p.staleAfter:
		state = SystemStatusStateDegraded
	}

	return OrchestratorSystemStatus{
		State:       state,
		LastUpdated: lastHeartbeat,
	}
}

func (p *defaultSystemStatusProvider) RecordDockerConnectivity(_ context.Context, reachable bool, at time.Time) error {
	if at.IsZero() {
		at = p.now()
	}

	p.dockerSignalMu.Lock()
	p.dockerSignal = dockerConnectivitySignal{
		lastCheckedUTC: at.UTC(),
		reachable:      reachable,
		hasSignal:      true,
	}
	p.dockerSignalMu.Unlock()

	return nil
}

func (p *defaultSystemStatusProvider) dockerStatus() DockerSystemStatus {
	p.dockerSignalMu.RLock()
	signal := p.dockerSignal
	p.dockerSignalMu.RUnlock()

	if !signal.hasSignal {
		return DockerSystemStatus{State: SystemStatusStateUnavailable}
	}

	age := p.now().UTC().Sub(signal.lastCheckedUTC)
	if age < 0 {
		age = 0
	}
	if age > p.staleAfter {
		return DockerSystemStatus{
			State:       SystemStatusStateStale,
			LastUpdated: signal.lastCheckedUTC,
		}
	}

	state := SystemStatusStateDegraded
	if signal.reachable {
		state = SystemStatusStateHealthy
	}

	return DockerSystemStatus{
		State:       state,
		LastUpdated: signal.lastCheckedUTC,
	}
}

func (p *defaultSystemStatusProvider) RecordCPUUtilization(_ context.Context, usagePercent float64, at time.Time) error {
	if at.IsZero() {
		at = p.now()
	}

	p.hostSignalMu.Lock()
	p.cpuSignal = hostMetricSignal{
		lastCheckedUTC: at.UTC(),
		usagePercent:   usagePercent,
		hasSignal:      true,
	}
	p.hostSignalMu.Unlock()

	return nil
}

func (p *defaultSystemStatusProvider) RecordRAMUtilization(_ context.Context, usagePercent float64, at time.Time) error {
	if at.IsZero() {
		at = p.now()
	}

	p.hostSignalMu.Lock()
	p.ramSignal = hostMetricSignal{
		lastCheckedUTC: at.UTC(),
		usagePercent:   usagePercent,
		hasSignal:      true,
	}
	p.hostSignalMu.Unlock()

	return nil
}

func (p *defaultSystemStatusProvider) cpuStatus() HostMetricSystemStatus {
	p.hostSignalMu.RLock()
	signal := p.cpuSignal
	p.hostSignalMu.RUnlock()

	if !signal.hasSignal {
		return HostMetricSystemStatus{State: SystemStatusStateUnavailable}
	}

	return HostMetricSystemStatus{
		State:        SystemStatusStateHealthy,
		UsagePercent: signal.usagePercent,
		LastUpdated:  signal.lastCheckedUTC,
	}
}

func (p *defaultSystemStatusProvider) ramStatus() HostMetricSystemStatus {
	p.hostSignalMu.RLock()
	signal := p.ramSignal
	p.hostSignalMu.RUnlock()

	if !signal.hasSignal {
		return HostMetricSystemStatus{State: SystemStatusStateUnavailable}
	}

	return HostMetricSystemStatus{
		State:        SystemStatusStateHealthy,
		UsagePercent: signal.usagePercent,
		LastUpdated:  signal.lastCheckedUTC,
	}
}

func (h *Handler) systemStatus(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.statusSource.Snapshot(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, SystemStatusSnapshot{
			API: APISystemStatus{
				State:       SystemStatusStateUnavailable,
				LastUpdated: time.Now().UTC(),
			},
			Orchestrator: OrchestratorSystemStatus{State: SystemStatusStateUnavailable},
			Docker:       DockerSystemStatus{State: SystemStatusStateUnavailable},
			Host: HostSystemStatus{
				CPU: HostMetricSystemStatus{State: SystemStatusStateUnavailable},
				RAM: HostMetricSystemStatus{State: SystemStatusStateUnavailable},
			},
			Error: "system status unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}
