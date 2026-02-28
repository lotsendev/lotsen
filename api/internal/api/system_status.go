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
	SystemStatusStateUnavailable SystemStatusState = "unavailable"
)

type APISystemStatusChecks struct {
	ProcessRunning     bool `json:"processRunning"`
	DashboardReachable bool `json:"dashboardReachable"`
	StoreAccessible    bool `json:"storeAccessible"`
}

// APISystemStatus carries API availability and freshness information.
type APISystemStatus struct {
	State       SystemStatusState     `json:"state"`
	LastUpdated time.Time             `json:"lastUpdated"`
	Checks      APISystemStatusChecks `json:"checks"`
}

type OrchestratorSystemStatusChecks struct {
	ProcessRunning  bool `json:"processRunning"`
	DockerReachable bool `json:"dockerReachable"`
	StoreAccessible bool `json:"storeAccessible"`
}

// OrchestratorSystemStatus carries orchestrator liveness and freshness information.
type OrchestratorSystemStatus struct {
	State       SystemStatusState              `json:"state"`
	LastUpdated *time.Time                     `json:"lastUpdated,omitempty"`
	Checks      OrchestratorSystemStatusChecks `json:"checks"`
}

type LoadBalancerSystemStatusChecks struct {
	ProcessRunning        bool `json:"processRunning"`
	HealthcheckResponding bool `json:"healthcheckResponding"`
}

type LoadBalancerBlockedIPStatus struct {
	IP           string     `json:"ip"`
	BlockedUntil *time.Time `json:"blockedUntil,omitempty"`
}

type LoadBalancerTrafficSystemStatus struct {
	TotalRequests      int64                         `json:"totalRequests"`
	SuspiciousRequests int64                         `json:"suspiciousRequests"`
	BlockedRequests    int64                         `json:"blockedRequests"`
	WAFBlockedRequests int64                         `json:"wafBlockedRequests"`
	UABlockedRequests  int64                         `json:"uaBlockedRequests"`
	ActiveBlockedIPs   int                           `json:"activeBlockedIps"`
	BlockedIPs         []LoadBalancerBlockedIPStatus `json:"blockedIps,omitempty"`
}

type LoadBalancerSystemStatus struct {
	State       SystemStatusState                `json:"state"`
	LastUpdated *time.Time                       `json:"lastUpdated,omitempty"`
	Checks      LoadBalancerSystemStatusChecks   `json:"checks"`
	Traffic     *LoadBalancerTrafficSystemStatus `json:"traffic,omitempty"`
}

type DockerSystemStatusChecks struct {
	DaemonHealthy bool `json:"daemonHealthy"`
}

// DockerSystemStatus carries Docker connectivity information as observed by the orchestrator.
type DockerSystemStatus struct {
	State       SystemStatusState        `json:"state"`
	LastUpdated *time.Time               `json:"lastUpdated,omitempty"`
	Checks      DockerSystemStatusChecks `json:"checks"`
}

// HostMetricSystemStatus carries a host metric signal value and freshness information.
type HostMetricSystemStatus struct {
	State        SystemStatusState `json:"state"`
	UsagePercent float64           `json:"usagePercent,omitempty"`
	LastUpdated  *time.Time        `json:"lastUpdated,omitempty"`
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
	LoadBalancer LoadBalancerSystemStatus `json:"loadBalancer"`
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
	RecordOrchestratorHeartbeat(ctx context.Context, at time.Time, storeAccessible bool) error
}

// DockerConnectivityIngestor accepts Docker connectivity observations.
type DockerConnectivityIngestor interface {
	RecordDockerConnectivity(ctx context.Context, reachable bool, at time.Time) error
}

type LoadBalancerHealthIngestor interface {
	RecordLoadBalancerHealth(ctx context.Context, responding bool, at time.Time, traffic *LoadBalancerTrafficSystemStatus) error
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
	now                func() time.Time
	staleAfter         time.Duration
	apiStoreCheck      func(context.Context) bool
	lastHeartbeatMu    sync.RWMutex
	orchestratorSignal orchestratorHeartbeatSignal
	dockerSignalMu     sync.RWMutex
	dockerSignal       dockerConnectivitySignal
	loadBalancerMu     sync.RWMutex
	loadBalancerSignal loadBalancerHealthSignal
	hostSignalMu       sync.RWMutex
	cpuSignal          hostMetricSignal
	ramSignal          hostMetricSignal
}

type orchestratorHeartbeatSignal struct {
	lastHeartbeatUTC time.Time
	storeAccessible  bool
	hasSignal        bool
}

type dockerConnectivitySignal struct {
	lastCheckedUTC time.Time
	reachable      bool
	hasSignal      bool
}

type loadBalancerHealthSignal struct {
	lastCheckedUTC time.Time
	responding     bool
	traffic        *LoadBalancerTrafficSystemStatus
	hasSignal      bool
}

type hostMetricSignal struct {
	lastCheckedUTC time.Time
	usagePercent   float64
	hasSignal      bool
}

func newDefaultSystemStatusProvider(now func() time.Time, staleAfter time.Duration, apiStoreCheck func(context.Context) bool) SystemStatusProvider {
	if apiStoreCheck == nil {
		apiStoreCheck = func(context.Context) bool { return true }
	}

	return &defaultSystemStatusProvider{now: now, staleAfter: staleAfter, apiStoreCheck: apiStoreCheck}
}

func (p *defaultSystemStatusProvider) Snapshot(ctx context.Context) (SystemStatusSnapshot, error) {
	orchestrator := p.orchestratorStatus()
	storeAccessible := p.apiStoreCheck(ctx)
	apiState := SystemStatusStateHealthy
	if !storeAccessible {
		apiState = SystemStatusStateDegraded
	}

	return SystemStatusSnapshot{
		API: APISystemStatus{
			State:       apiState,
			LastUpdated: p.now().UTC(),
			Checks: APISystemStatusChecks{
				ProcessRunning:     true,
				DashboardReachable: true,
				StoreAccessible:    storeAccessible,
			},
		},
		Orchestrator: orchestrator,
		LoadBalancer: p.loadBalancerStatus(),
		Docker:       p.dockerStatus(),
		Host: HostSystemStatus{
			CPU: p.cpuStatus(),
			RAM: p.ramStatus(),
		},
	}, nil
}

func (p *defaultSystemStatusProvider) RecordOrchestratorHeartbeat(_ context.Context, at time.Time, storeAccessible bool) error {
	if at.IsZero() {
		at = p.now()
	}

	p.lastHeartbeatMu.Lock()
	p.orchestratorSignal = orchestratorHeartbeatSignal{
		lastHeartbeatUTC: at.UTC(),
		storeAccessible:  storeAccessible,
		hasSignal:        true,
	}
	p.lastHeartbeatMu.Unlock()

	return nil
}

func (p *defaultSystemStatusProvider) orchestratorStatus() OrchestratorSystemStatus {
	p.lastHeartbeatMu.RLock()
	signal := p.orchestratorSignal
	p.lastHeartbeatMu.RUnlock()

	if !signal.hasSignal {
		return OrchestratorSystemStatus{State: SystemStatusStateUnavailable}
	}
	lastHeartbeat := signal.lastHeartbeatUTC

	dockerState := p.dockerStatus()
	dockerReachable := dockerState.Checks.DaemonHealthy

	age := p.now().UTC().Sub(lastHeartbeat)
	if age < 0 {
		age = 0
	}

	state := SystemStatusStateHealthy
	if age > p.staleAfter || !signal.storeAccessible || !dockerReachable {
		state = SystemStatusStateDegraded
	}

	return OrchestratorSystemStatus{
		State:       state,
		LastUpdated: &lastHeartbeat,
		Checks: OrchestratorSystemStatusChecks{
			ProcessRunning:  true,
			DockerReachable: dockerReachable,
			StoreAccessible: signal.storeAccessible,
		},
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
	checkedAt := signal.lastCheckedUTC
	if age > p.staleAfter {
		return DockerSystemStatus{
			State:       SystemStatusStateDegraded,
			LastUpdated: &checkedAt,
			Checks:      DockerSystemStatusChecks{DaemonHealthy: false},
		}
	}

	state := SystemStatusStateDegraded
	if signal.reachable {
		state = SystemStatusStateHealthy
	}

	return DockerSystemStatus{
		State:       state,
		LastUpdated: &checkedAt,
		Checks:      DockerSystemStatusChecks{DaemonHealthy: signal.reachable},
	}
}

func (p *defaultSystemStatusProvider) RecordLoadBalancerHealth(_ context.Context, responding bool, at time.Time, traffic *LoadBalancerTrafficSystemStatus) error {
	if at.IsZero() {
		at = p.now()
	}

	trafficCopy := cloneLoadBalancerTraffic(traffic)

	p.loadBalancerMu.Lock()
	p.loadBalancerSignal = loadBalancerHealthSignal{
		lastCheckedUTC: at.UTC(),
		responding:     responding,
		traffic:        trafficCopy,
		hasSignal:      true,
	}
	p.loadBalancerMu.Unlock()

	return nil
}

func (p *defaultSystemStatusProvider) loadBalancerStatus() LoadBalancerSystemStatus {
	p.loadBalancerMu.RLock()
	signal := p.loadBalancerSignal
	p.loadBalancerMu.RUnlock()

	if !signal.hasSignal {
		return LoadBalancerSystemStatus{State: SystemStatusStateUnavailable}
	}

	age := p.now().UTC().Sub(signal.lastCheckedUTC)
	if age < 0 {
		age = 0
	}

	state := SystemStatusStateHealthy
	if age > p.staleAfter || !signal.responding {
		state = SystemStatusStateDegraded
	}

	checkedAt := signal.lastCheckedUTC
	return LoadBalancerSystemStatus{
		State:       state,
		LastUpdated: &checkedAt,
		Checks: LoadBalancerSystemStatusChecks{
			ProcessRunning:        true,
			HealthcheckResponding: signal.responding,
		},
		Traffic: cloneLoadBalancerTraffic(signal.traffic),
	}
}

func cloneLoadBalancerTraffic(in *LoadBalancerTrafficSystemStatus) *LoadBalancerTrafficSystemStatus {
	if in == nil {
		return nil
	}

	out := *in
	if len(in.BlockedIPs) > 0 {
		out.BlockedIPs = make([]LoadBalancerBlockedIPStatus, len(in.BlockedIPs))
		copy(out.BlockedIPs, in.BlockedIPs)
	}

	return &out
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

	cpuCheckedAt := signal.lastCheckedUTC
	state := SystemStatusStateHealthy
	if p.now().UTC().Sub(cpuCheckedAt) > p.staleAfter {
		state = SystemStatusStateDegraded
	}
	return HostMetricSystemStatus{
		State:        state,
		UsagePercent: signal.usagePercent,
		LastUpdated:  &cpuCheckedAt,
	}
}

func (p *defaultSystemStatusProvider) ramStatus() HostMetricSystemStatus {
	p.hostSignalMu.RLock()
	signal := p.ramSignal
	p.hostSignalMu.RUnlock()

	if !signal.hasSignal {
		return HostMetricSystemStatus{State: SystemStatusStateUnavailable}
	}

	ramCheckedAt := signal.lastCheckedUTC
	state := SystemStatusStateHealthy
	if p.now().UTC().Sub(ramCheckedAt) > p.staleAfter {
		state = SystemStatusStateDegraded
	}
	return HostMetricSystemStatus{
		State:        state,
		UsagePercent: signal.usagePercent,
		LastUpdated:  &ramCheckedAt,
	}
}

func (h *Handler) systemStatus(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.statusSource.Snapshot(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, unavailableSystemStatusSnapshot(time.Now().UTC()))
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}

func (h *Handler) currentSystemStatusSnapshot(r *http.Request) SystemStatusSnapshot {
	snapshot, err := h.statusSource.Snapshot(r.Context())
	if err != nil {
		return unavailableSystemStatusSnapshot(time.Now().UTC())
	}

	return snapshot
}

func unavailableSystemStatusSnapshot(now time.Time) SystemStatusSnapshot {
	return SystemStatusSnapshot{
		API: APISystemStatus{
			State:       SystemStatusStateUnavailable,
			LastUpdated: now,
		},
		Orchestrator: OrchestratorSystemStatus{State: SystemStatusStateUnavailable},
		LoadBalancer: LoadBalancerSystemStatus{State: SystemStatusStateUnavailable},
		Docker:       DockerSystemStatus{State: SystemStatusStateUnavailable},
		Host: HostSystemStatus{
			CPU: HostMetricSystemStatus{State: SystemStatusStateUnavailable},
			RAM: HostMetricSystemStatus{State: SystemStatusStateUnavailable},
		},
		Error: "system status unavailable",
	}
}
