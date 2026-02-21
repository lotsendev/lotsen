package reconciler

import (
	"context"
	"fmt"
	"log"

	"github.com/ercadev/dirigent/orchestrator/internal/docker"
	"github.com/ercadev/dirigent/orchestrator/internal/store"
)

// Docker is the container management interface required by the reconciler.
type Docker interface {
	Start(ctx context.Context, d store.Deployment) error
	ListManagedContainers(ctx context.Context) ([]docker.ManagedContainer, error)
	StopAndRemove(ctx context.Context, containerID string) error
}

// Store is the persistence interface required by the reconciler.
type Store interface {
	List() ([]store.Deployment, error)
	UpdateStatus(id string, status store.Status) error
}

// Reconciler syncs the desired state in the store with actual Docker containers.
type Reconciler struct {
	store  Store
	docker Docker
}

// New creates a Reconciler backed by the given store and Docker client.
func New(s Store, d Docker) *Reconciler {
	return &Reconciler{store: s, docker: d}
}

// Reconcile performs one reconciliation pass: reads desired state from the store,
// reads actual state from Docker, and converges them.
func (r *Reconciler) Reconcile(ctx context.Context) error {
	deployments, err := r.store.List()
	if err != nil {
		return fmt.Errorf("reconcile: list deployments: %w", err)
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
		c, hasContainer := containerByDeployment[d.ID]

		switch d.Status {
		case store.StatusDeploying:
			if !hasContainer {
				// Start the container.
				if err := r.docker.Start(ctx, d); err != nil {
					log.Printf("reconciler: start %s (%s): %v", d.ID, d.Name, err)
					r.updateStatus(d.ID, store.StatusFailed)
				} else {
					r.updateStatus(d.ID, store.StatusHealthy)
				}
			} else if c.Running {
				r.updateStatus(d.ID, store.StatusHealthy)
			} else {
				r.updateStatus(d.ID, store.StatusFailed)
			}

		case store.StatusHealthy:
			if !hasContainer || !c.Running {
				r.updateStatus(d.ID, store.StatusFailed)
			}

		case store.StatusFailed, store.StatusIdle:
			// Leave as-is; operator must intervene.
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

func (r *Reconciler) updateStatus(id string, status store.Status) {
	if err := r.store.UpdateStatus(id, status); err != nil {
		log.Printf("reconciler: update status %s → %s: %v", id, status, err)
	}
}
