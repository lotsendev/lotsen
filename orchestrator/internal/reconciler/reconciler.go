package reconciler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ercadev/dirigent/orchestrator/internal/docker"
	"github.com/ercadev/dirigent/store"
)

// Docker is the container management interface required by the reconciler.
type Docker interface {
	Ping(ctx context.Context) error
	Start(ctx context.Context, d store.Deployment) ([]string, error)
	StartAndReplace(ctx context.Context, d store.Deployment, oldContainerID string) ([]string, error)
	ListManagedContainers(ctx context.Context) ([]docker.ManagedContainer, error)
	StopAndRemove(ctx context.Context, containerID string) error
}

// Store is the persistence interface required by the reconciler.
type Store interface {
	List() ([]store.Deployment, error)
	Patch(id string, patch store.Deployment) (store.Deployment, error)
}

// Notifier notifies the API of deployment status transitions so the event
// broker can push real-time updates to SSE subscribers.
type Notifier interface {
	NotifyStatus(id string, status store.Status, errorMessage string) error
}

// Reconciler syncs the desired state in the store with actual Docker containers.
type Reconciler struct {
	store    Store
	docker   Docker
	notifier Notifier

	mu                     sync.Mutex
	lastDockerReachable    bool
	hasLastDockerReachable bool
	retryByDeployment      map[string]retryState
	now                    func() time.Time
}

type retryState struct {
	attempts      int
	nextAllowedAt time.Time
}

const (
	dockerUnavailableError = "docker daemon is unavailable"

	transientRetryBaseDelay = 15 * time.Second
	transientRetryMaxDelay  = 2 * time.Minute
)

// New creates a Reconciler backed by the given store and Docker client.
// n may be nil; if so, API notification is skipped.
func New(s Store, d Docker, n Notifier) *Reconciler {
	return &Reconciler{
		store:             s,
		docker:            d,
		notifier:          n,
		retryByDeployment: make(map[string]retryState),
		now:               time.Now,
	}
}

// Reconcile performs one reconciliation pass: reads desired state from the store,
// reads actual state from Docker, and converges them.
func (r *Reconciler) Reconcile(ctx context.Context) error {
	now := r.now().UTC()

	deployments, err := r.store.List()
	if err != nil {
		return fmt.Errorf("reconcile: list deployments: %w", err)
	}

	// Check Docker availability before any container operations. If the daemon
	// is unreachable we update the store directly — no Docker calls required —
	// so active deployments reflect reality even when Docker is fully down.
	if err := r.docker.Ping(ctx); err != nil {
		r.recordDockerReachability(false)
		for _, d := range deployments {
			if d.Status == store.StatusHealthy || d.Status == store.StatusDeploying {
				r.updateStatus(d.ID, store.StatusFailed, dockerUnavailableError)
			}
		}
		return fmt.Errorf("reconcile: docker unavailable: %w", err)
	}

	reconnected := r.recordDockerReachability(true)
	if reconnected {
		log.Printf("reconciler: docker reconnected, retrying transient failed deployments")
	}

	containers, err := r.docker.ListManagedContainers(ctx)
	if err != nil {
		return fmt.Errorf("reconcile: list containers: %w", err)
	}

	// Build a map of deploymentID → container for quick lookup.
	containerByDeployment := make(map[string]docker.ManagedContainer, len(containers))
	for _, c := range containers {
		containerByDeployment[c.DeploymentID] = c
	}

	// Build a set of known deployment IDs for orphan detection.
	knownIDs := make(map[string]struct{}, len(deployments))
	for _, d := range deployments {
		knownIDs[d.ID] = struct{}{}
	}

	// Reconcile each deployment in the store.
	for _, d := range deployments {
		if d.Status == store.StatusHealthy || d.Status == store.StatusDeploying || d.Status == store.StatusIdle {
			r.clearRetryState(d.ID)
		}

		c, hasContainer := containerByDeployment[d.ID]

		switch d.Status {
		case store.StatusDeploying:
			if !hasContainer {
				// Fresh deploy: no container exists yet — start one.
				runtimePorts, err := r.docker.Start(ctx, d)
				if err != nil {
					log.Printf("reconciler: start %s (%s): %v", d.ID, d.Name, err)
					r.updateStatus(d.ID, store.StatusFailed, err.Error())
				} else {
					r.updatePortsAndStatus(d.ID, runtimePorts, store.StatusHealthy)
				}
			} else if c.Running {
				// Container already running — this is a redeploy; apply start-then-stop.
				runtimePorts, err := r.docker.StartAndReplace(ctx, d, c.ID)
				if err != nil {
					log.Printf("reconciler: redeploy %s (%s): %v", d.ID, d.Name, err)
					r.updateStatus(d.ID, store.StatusFailed, err.Error())
				} else {
					r.updatePortsAndStatus(d.ID, runtimePorts, store.StatusHealthy)
				}
			} else {
				r.updateStatus(d.ID, store.StatusFailed, exitMessage(c.ExitDetails))
			}

		case store.StatusHealthy:
			if !hasContainer || !c.Running {
				r.updateStatus(d.ID, store.StatusFailed, exitMessage(c.ExitDetails))
			}

		case store.StatusFailed, store.StatusIdle:
			if d.Status == store.StatusIdle {
				continue
			}

			if !isTransientFailure(d.Error) {
				continue
			}

			if hasContainer && c.Running {
				r.clearRetryState(d.ID)
				r.updateStatus(d.ID, store.StatusHealthy, "")
				continue
			}

			if !r.canRetryTransientFailure(d.ID, now, reconnected) {
				continue
			}

			var runtimePorts []string
			if hasContainer {
				runtimePorts, err = r.docker.StartAndReplace(ctx, d, c.ID)
			} else {
				runtimePorts, err = r.docker.Start(ctx, d)
			}
			if err != nil {
				log.Printf("reconciler: heal failed deployment %s (%s): %v", d.ID, d.Name, err)
				r.recordRetryFailure(d.ID, now)
				r.updateStatus(d.ID, store.StatusFailed, err.Error())
				continue
			}

			r.clearRetryState(d.ID)
			r.updatePortsAndStatus(d.ID, runtimePorts, store.StatusHealthy)
		}
	}

	// Stop and remove containers that have no corresponding store entry (deleted deployments).
	for _, c := range containers {
		if _, known := knownIDs[c.DeploymentID]; !known {
			if err := r.docker.StopAndRemove(ctx, c.ID); err != nil {
				log.Printf("reconciler: stop+remove orphan %s: %v", c.ID, err)
			}
		}
	}

	return nil
}

func isTransientFailure(errorMessage string) bool {
	if errorMessage == "" {
		return false
	}

	msg := strings.ToLower(errorMessage)
	return strings.Contains(msg, dockerUnavailableError) ||
		strings.Contains(msg, "docker daemon unreachable") ||
		strings.Contains(msg, "container is not running")
}

func (r *Reconciler) recordDockerReachability(reachable bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.hasLastDockerReachable {
		r.hasLastDockerReachable = true
		r.lastDockerReachable = reachable
		return false
	}

	reconnected := !r.lastDockerReachable && reachable
	r.lastDockerReachable = reachable
	return reconnected
}

func (r *Reconciler) canRetryTransientFailure(id string, now time.Time, force bool) bool {
	if force {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.retryByDeployment[id]
	if !ok {
		return true
	}
	if state.nextAllowedAt.IsZero() || !now.Before(state.nextAllowedAt) {
		return true
	}
	return false
}

func (r *Reconciler) recordRetryFailure(id string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state := r.retryByDeployment[id]
	delay := transientRetryBaseDelay
	for i := 0; i < state.attempts; i++ {
		delay *= 2
		if delay >= transientRetryMaxDelay {
			delay = transientRetryMaxDelay
			break
		}
	}

	state.attempts++
	state.nextAllowedAt = now.Add(delay)
	r.retryByDeployment[id] = state
}

func (r *Reconciler) clearRetryState(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.retryByDeployment, id)
}

// exitMessage produces a human-readable failure reason from container exit details.
// When d is nil (inspect failed or container missing) the generic fallback is returned.
func exitMessage(d *docker.ExitDetails) string {
	if d == nil {
		return "container is not running"
	}
	if d.OOMKilled {
		return fmt.Sprintf("container killed: out of memory (exit code %d)", d.ExitCode)
	}
	if d.Error != "" {
		return fmt.Sprintf("container exited with error: %s (exit code %d)", d.Error, d.ExitCode)
	}
	if d.ExitCode == 0 {
		return fmt.Sprintf("container exited cleanly (exit code %d)", d.ExitCode)
	}
	return fmt.Sprintf("container exited unexpectedly (exit code %d)", d.ExitCode)
}

func (r *Reconciler) updateStatus(id string, status store.Status, errorMessage string) {
	if _, err := r.store.Patch(id, store.Deployment{Status: status, Error: errorMessage}); err != nil {
		log.Printf("reconciler: update status %s → %s: %v", id, status, err)
	}
	r.notifyStatus(id, status, errorMessage)
}

func (r *Reconciler) updatePortsAndStatus(id string, ports []string, status store.Status) {
	patch := store.Deployment{Status: status, Error: ""}
	if ports != nil {
		patch.Ports = ports
	}
	if _, err := r.store.Patch(id, patch); err != nil {
		log.Printf("reconciler: patch deployment %s: %v", id, err)
	}
	r.notifyStatus(id, status, "")
}

func (r *Reconciler) notifyStatus(id string, status store.Status, errorMessage string) {
	if r.notifier != nil {
		if err := r.notifier.NotifyStatus(id, status, errorMessage); err != nil {
			log.Printf("reconciler: notify api status %s → %s: %v", id, status, err)
		}
	}
}
