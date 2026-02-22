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

	snapshot, err := provider.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot initial: %v", err)
	}
	if snapshot.Orchestrator.State != SystemStatusStateUnavailable {
		t.Fatalf("want unavailable before first heartbeat, got %s", snapshot.Orchestrator.State)
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
			if !snapshot.Orchestrator.LastUpdated.Equal(base) {
				t.Fatalf("want lastUpdated %s, got %s", base, snapshot.Orchestrator.LastUpdated)
			}
		})
	}
}
