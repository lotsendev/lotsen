package reconciler_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ercadev/dirigent/orchestrator/internal/docker"
	"github.com/ercadev/dirigent/orchestrator/internal/reconciler"
	"github.com/ercadev/dirigent/store"
)

// mockNotifier records NotifyStatus calls and can be configured to fail.
type mockNotifier struct {
	mu      sync.Mutex
	calls   []notifyCall
	notifyErr error
}

type notifyCall struct {
	id     string
	status store.Status
}

func (m *mockNotifier) NotifyStatus(id string, status store.Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, notifyCall{id: id, status: status})
	return m.notifyErr
}

func (m *mockNotifier) getCalls() []notifyCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]notifyCall, len(m.calls))
	copy(out, m.calls)
	return out
}

// mockStore is a controllable in-memory store for testing.
type mockStore struct {
	mu          sync.Mutex
	deployments []store.Deployment
}

func (m *mockStore) List() ([]store.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.Deployment, len(m.deployments))
	copy(out, m.deployments)
	return out, nil
}

func (m *mockStore) UpdateStatus(id string, status store.Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, d := range m.deployments {
		if d.ID == id {
			m.deployments[i].Status = status
			return nil
		}
	}
	return nil
}

func (m *mockStore) getStatus(id string) store.Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.deployments {
		if d.ID == id {
			return d.Status
		}
	}
	return ""
}

// mockDocker is a controllable Docker client for testing.
type mockDocker struct {
	mu                    sync.Mutex
	containers            []docker.ManagedContainer
	startErr              error
	startAndReplaceErr    error
	started               []string
	replaced              []string // old container IDs passed to StartAndReplace
	removed               []string
}

func (m *mockDocker) Start(_ context.Context, d store.Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.started = append(m.started, d.ID)
	return nil
}

func (m *mockDocker) StartAndReplace(_ context.Context, d store.Deployment, oldContainerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startAndReplaceErr != nil {
		return m.startAndReplaceErr
	}
	m.started = append(m.started, d.ID)
	m.replaced = append(m.replaced, oldContainerID)
	return nil
}

func (m *mockDocker) ListManagedContainers(_ context.Context) ([]docker.ManagedContainer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]docker.ManagedContainer, len(m.containers))
	copy(out, m.containers)
	return out, nil
}

func (m *mockDocker) StopAndRemove(_ context.Context, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, containerID)
	return nil
}

func TestReconcile_DeployingNoContainer_StartsAndBecomesHealthy(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Image: "nginx:latest", Status: store.StatusDeploying},
		},
	}
	d := &mockDocker{}
	r := reconciler.New(s, d, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if len(d.started) != 1 || d.started[0] != "d1" {
		t.Errorf("want d1 started, got %v", d.started)
	}
	if s.getStatus("d1") != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", s.getStatus("d1"))
	}
}

func TestReconcile_DeployingNoContainer_StartFails_BecomesFailed(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Image: "bad:image", Status: store.StatusDeploying},
		},
	}
	d := &mockDocker{startErr: errors.New("image not found")}
	r := reconciler.New(s, d, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if s.getStatus("d1") != store.StatusFailed {
		t.Errorf("want status failed, got %s", s.getStatus("d1"))
	}
}

func TestReconcile_DeployingWithRunningContainer_RedeploysAndBecomesHealthy(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Image: "nginx:2", Status: store.StatusDeploying},
		},
	}
	d := &mockDocker{
		containers: []docker.ManagedContainer{
			{ID: "c1", DeploymentID: "d1", Running: true},
		},
	}
	r := reconciler.New(s, d, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// StartAndReplace must have been called with the old container ID.
	if len(d.replaced) != 1 || d.replaced[0] != "c1" {
		t.Errorf("want StartAndReplace called with c1, got %v", d.replaced)
	}
	if s.getStatus("d1") != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", s.getStatus("d1"))
	}
}

func TestReconcile_Redeploy_FailedNewContainer_KeepsOldAndBecomesFailed(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Image: "nginx:bad", Status: store.StatusDeploying},
		},
	}
	d := &mockDocker{
		containers: []docker.ManagedContainer{
			{ID: "c1", DeploymentID: "d1", Running: true},
		},
		startAndReplaceErr: errors.New("new container did not reach running state"),
	}
	r := reconciler.New(s, d, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Old container must not have been removed.
	if len(d.removed) != 0 {
		t.Errorf("want old container untouched, got removed: %v", d.removed)
	}
	if s.getStatus("d1") != store.StatusFailed {
		t.Errorf("want status failed, got %s", s.getStatus("d1"))
	}
}

func TestReconcile_DeployingWithStoppedContainer_BecomesFailed(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Status: store.StatusDeploying},
		},
	}
	d := &mockDocker{
		containers: []docker.ManagedContainer{
			{ID: "c1", DeploymentID: "d1", Running: false},
		},
	}
	r := reconciler.New(s, d, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if s.getStatus("d1") != store.StatusFailed {
		t.Errorf("want status failed, got %s", s.getStatus("d1"))
	}
}

func TestReconcile_HealthyNoContainer_BecomesFailed(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Status: store.StatusHealthy},
		},
	}
	d := &mockDocker{}
	r := reconciler.New(s, d, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if s.getStatus("d1") != store.StatusFailed {
		t.Errorf("want status failed, got %s", s.getStatus("d1"))
	}
}

func TestReconcile_OrphanContainer_StoppedAndRemoved(t *testing.T) {
	s := &mockStore{} // no deployments in store
	d := &mockDocker{
		containers: []docker.ManagedContainer{
			{ID: "c1", DeploymentID: "orphan-id", Running: true},
		},
	}
	r := reconciler.New(s, d, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if len(d.removed) != 1 || d.removed[0] != "c1" {
		t.Errorf("want c1 removed, got %v", d.removed)
	}
}

func TestReconcile_FailedDeployment_Skipped(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Status: store.StatusFailed},
		},
	}
	d := &mockDocker{}
	r := reconciler.New(s, d, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if len(d.started) != 0 {
		t.Errorf("want no starts for failed deployment, got %v", d.started)
	}
	if s.getStatus("d1") != store.StatusFailed {
		t.Errorf("want status to remain failed, got %s", s.getStatus("d1"))
	}
}

func TestReconcile_DeployingNoContainer_NotifiesHealthy(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Image: "nginx:latest", Status: store.StatusDeploying},
		},
	}
	d := &mockDocker{}
	n := &mockNotifier{}
	r := reconciler.New(s, d, n)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	calls := n.getCalls()
	if len(calls) != 1 || calls[0].id != "d1" || calls[0].status != store.StatusHealthy {
		t.Errorf("want notify(d1, healthy), got %v", calls)
	}
}

func TestReconcile_DeployingStartFails_NotifiesFailed(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Image: "bad:image", Status: store.StatusDeploying},
		},
	}
	d := &mockDocker{startErr: errors.New("image not found")}
	n := &mockNotifier{}
	r := reconciler.New(s, d, n)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	calls := n.getCalls()
	if len(calls) != 1 || calls[0].id != "d1" || calls[0].status != store.StatusFailed {
		t.Errorf("want notify(d1, failed), got %v", calls)
	}
}

func TestReconcile_HealthyContainerGone_NotifiesFailed(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Status: store.StatusHealthy},
		},
	}
	d := &mockDocker{} // no containers
	n := &mockNotifier{}
	r := reconciler.New(s, d, n)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	calls := n.getCalls()
	if len(calls) != 1 || calls[0].id != "d1" || calls[0].status != store.StatusFailed {
		t.Errorf("want notify(d1, failed), got %v", calls)
	}
}

func TestReconcile_NotifyAPIFailure_DoesNotCrash(t *testing.T) {
	s := &mockStore{
		deployments: []store.Deployment{
			{ID: "d1", Name: "web", Image: "nginx:latest", Status: store.StatusDeploying},
		},
	}
	d := &mockDocker{}
	n := &mockNotifier{notifyErr: errors.New("connection refused")}
	r := reconciler.New(s, d, n)

	// Must not return an error even when the notifier fails.
	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Store should still be updated even though notification failed.
	if s.getStatus("d1") != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", s.getStatus("d1"))
	}
}
