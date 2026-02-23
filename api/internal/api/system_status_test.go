package api

import (
	"context"
	"testing"
	"time"
)

func TestDefaultSystemStatusProvider_OrchestratorStateMapping(t *testing.T) {
	base := time.Date(2026, time.February, 22, 12, 0, 0, 0, time.UTC)
	now := base

	provider := newDefaultSystemStatusProvider(func() time.Time { return now }, 10*time.Second)
	ingestor, ok := provider.(OrchestratorHeartbeatIngestor)
	if !ok {
		t.Fatal("default provider must implement OrchestratorHeartbeatIngestor")
	}
	dockerIngestor, ok := provider.(DockerConnectivityIngestor)
	if !ok {
		t.Fatal("default provider must implement DockerConnectivityIngestor")
	}
	cpuIngestor, ok := provider.(CPUUtilizationIngestor)
	if !ok {
		t.Fatal("default provider must implement CPUUtilizationIngestor")
	}
	ramIngestor, ok := provider.(RAMUtilizationIngestor)
	if !ok {
		t.Fatal("default provider must implement RAMUtilizationIngestor")
	}

	snapshot, err := provider.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot initial: %v", err)
	}
	if snapshot.Orchestrator.State != SystemStatusStateUnavailable {
		t.Fatalf("want unavailable before first heartbeat, got %s", snapshot.Orchestrator.State)
	}
	if snapshot.Docker.State != SystemStatusStateUnavailable {
		t.Fatalf("want docker unavailable before first signal, got %s", snapshot.Docker.State)
	}
	if snapshot.Host.CPU.State != SystemStatusStateUnavailable {
		t.Fatalf("want cpu unavailable before first signal, got %s", snapshot.Host.CPU.State)
	}
	if snapshot.Host.RAM.State != SystemStatusStateUnavailable {
		t.Fatalf("want ram unavailable before first signal, got %s", snapshot.Host.RAM.State)
	}

	if err := ingestor.RecordOrchestratorHeartbeat(context.Background(), base); err != nil {
		t.Fatalf("RecordOrchestratorHeartbeat: %v", err)
	}

	tests := []struct {
		name      string
		now       time.Time
		wantState SystemStatusState
	}{
		{name: "healthy at heartbeat time", now: base, wantState: SystemStatusStateHealthy},
		{name: "healthy at half threshold", now: base.Add(5 * time.Second), wantState: SystemStatusStateHealthy},
		{name: "degraded after half threshold", now: base.Add(6 * time.Second), wantState: SystemStatusStateDegraded},
		{name: "degraded at stale threshold", now: base.Add(10 * time.Second), wantState: SystemStatusStateDegraded},
		{name: "stale after threshold", now: base.Add(11 * time.Second), wantState: SystemStatusStateStale},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now = tc.now

			snapshot, err := provider.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("Snapshot: %v", err)
			}

			if snapshot.Orchestrator.State != tc.wantState {
				t.Fatalf("want state %s, got %s", tc.wantState, snapshot.Orchestrator.State)
			}
			if snapshot.Orchestrator.LastUpdated == nil || !snapshot.Orchestrator.LastUpdated.Equal(base) {
				t.Fatalf("want lastUpdated %s, got %v", base, snapshot.Orchestrator.LastUpdated)
			}
		})
	}

	now = base
	if err := dockerIngestor.RecordDockerConnectivity(context.Background(), true, base.Add(2*time.Second)); err != nil {
		t.Fatalf("RecordDockerConnectivity healthy: %v", err)
	}

	snapshot, err = provider.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot after healthy docker signal: %v", err)
	}
	if snapshot.Docker.State != SystemStatusStateHealthy {
		t.Fatalf("want docker state healthy, got %s", snapshot.Docker.State)
	}
	if snapshot.Docker.LastUpdated == nil || !snapshot.Docker.LastUpdated.Equal(base.Add(2*time.Second)) {
		t.Fatalf("want docker lastUpdated %s, got %v", base.Add(2*time.Second), snapshot.Docker.LastUpdated)
	}

	if err := dockerIngestor.RecordDockerConnectivity(context.Background(), false, base.Add(3*time.Second)); err != nil {
		t.Fatalf("RecordDockerConnectivity degraded: %v", err)
	}

	snapshot, err = provider.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot after degraded docker signal: %v", err)
	}
	if snapshot.Docker.State != SystemStatusStateDegraded {
		t.Fatalf("want docker state degraded, got %s", snapshot.Docker.State)
	}
	if snapshot.Docker.LastUpdated == nil || !snapshot.Docker.LastUpdated.Equal(base.Add(3*time.Second)) {
		t.Fatalf("want docker lastUpdated %s, got %v", base.Add(3*time.Second), snapshot.Docker.LastUpdated)
	}

	// Advance time past the stale threshold — Docker signal should go stale even if last
	// signal said reachable, because the orchestrator has stopped reporting.
	if err := dockerIngestor.RecordDockerConnectivity(context.Background(), true, base.Add(4*time.Second)); err != nil {
		t.Fatalf("RecordDockerConnectivity healthy again: %v", err)
	}
	now = base.Add(4*time.Second + 10*time.Second + 1) // signal at +4s, staleAfter=10s → stale at +14s+1ns
	snapshot, err = provider.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot after stale docker signal: %v", err)
	}
	if snapshot.Docker.State != SystemStatusStateStale {
		t.Fatalf("want docker state stale when signal is old, got %s", snapshot.Docker.State)
	}
	now = base // reset for subsequent assertions

	if err := cpuIngestor.RecordCPUUtilization(context.Background(), 42.5, base.Add(4*time.Second)); err != nil {
		t.Fatalf("RecordCPUUtilization: %v", err)
	}

	snapshot, err = provider.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot after cpu signal: %v", err)
	}
	if snapshot.Host.CPU.State != SystemStatusStateHealthy {
		t.Fatalf("want cpu state healthy, got %s", snapshot.Host.CPU.State)
	}
	if snapshot.Host.CPU.UsagePercent != 42.5 {
		t.Fatalf("want cpu usage 42.5, got %v", snapshot.Host.CPU.UsagePercent)
	}
	if snapshot.Host.CPU.LastUpdated == nil || !snapshot.Host.CPU.LastUpdated.Equal(base.Add(4*time.Second)) {
		t.Fatalf("want cpu lastUpdated %s, got %v", base.Add(4*time.Second), snapshot.Host.CPU.LastUpdated)
	}
	if snapshot.Host.RAM.State != SystemStatusStateUnavailable {
		t.Fatalf("want ram state unavailable without signal, got %s", snapshot.Host.RAM.State)
	}

	if err := ramIngestor.RecordRAMUtilization(context.Background(), 73.2, base.Add(5*time.Second)); err != nil {
		t.Fatalf("RecordRAMUtilization: %v", err)
	}

	snapshot, err = provider.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot after ram signal: %v", err)
	}
	if snapshot.Host.RAM.State != SystemStatusStateHealthy {
		t.Fatalf("want ram state healthy, got %s", snapshot.Host.RAM.State)
	}
	if snapshot.Host.RAM.UsagePercent != 73.2 {
		t.Fatalf("want ram usage 73.2, got %v", snapshot.Host.RAM.UsagePercent)
	}
	if snapshot.Host.RAM.LastUpdated == nil || !snapshot.Host.RAM.LastUpdated.Equal(base.Add(5*time.Second)) {
		t.Fatalf("want ram lastUpdated %s, got %v", base.Add(5*time.Second), snapshot.Host.RAM.LastUpdated)
	}
}
