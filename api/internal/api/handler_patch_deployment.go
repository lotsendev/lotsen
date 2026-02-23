package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ercadev/dirigent/internal/events"
	"github.com/ercadev/dirigent/store"
)

func (h *Handler) patchDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body patchDeploymentRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	existing, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get deployment", http.StatusInternalServerError)
		return
	}

	if body.Ports != nil {
		allDeployments, err := h.store.List()
		if err != nil {
			http.Error(w, "failed to list deployments", http.StatusInternalServerError)
			return
		}
		assignedPorts, err := assignHostPorts(allDeployments, id, existing.Ports, normalizeContainerPorts(body.Ports))
		if err != nil {
			http.Error(w, "no host ports available", http.StatusServiceUnavailable)
			return
		}
		body.Ports = assignedPorts
	}

	patch := store.Deployment{
		Image:   body.Image,
		Envs:    body.Envs,
		Ports:   body.Ports,
		Volumes: body.Volumes,
		Domain:  body.Domain,
	}
	if patchRequiresRedeploy(existing, body) {
		patch.Status = store.StatusDeploying
	}

	updated, err := h.store.Patch(id, patch)
	if err != nil {
		http.Error(w, "failed to update deployment", http.StatusInternalServerError)
		return
	}

	if patch.Status == store.StatusDeploying {
		h.events.Publish(events.StatusEvent{
			DeploymentID: id,
			Status:       string(store.StatusDeploying),
		})
	}

	writeJSON(w, http.StatusAccepted, updated)
}
