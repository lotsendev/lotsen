package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ercadev/dirigent/internal/events"
	"github.com/ercadev/dirigent/store"
)

func (h *Handler) updateDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body deploymentRequest

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.Name == "" || body.Image == "" {
		http.Error(w, "name and image are required", http.StatusBadRequest)
		return
	}
	if conflictsWithDashboardDomain(body.Domain) {
		http.Error(w, "domain is reserved for dashboard", http.StatusConflict)
		return
	}

	basicAuth, err := sanitizeAndHashBasicAuth(body.BasicAuth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if body.Envs == nil {
		body.Envs = map[string]string{}
	}
	if body.Volumes == nil {
		body.Volumes = []string{}
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

	nextStatus := existing.Status
	if updateRequiresRedeploy(existing, body) {
		nextStatus = store.StatusDeploying
	}

	d := store.Deployment{
		ID:        id,
		Name:      body.Name,
		Image:     body.Image,
		Envs:      body.Envs,
		Ports:     body.Ports,
		Volumes:   body.Volumes,
		Domain:    body.Domain,
		BasicAuth: basicAuth,
		Security:  body.Security,
		Status:    nextStatus,
	}

	updated, err := h.store.Update(d)
	if err != nil {
		http.Error(w, "failed to update deployment", http.StatusInternalServerError)
		return
	}

	if nextStatus == store.StatusDeploying {
		h.events.Publish(events.StatusEvent{
			DeploymentID: updated.ID,
			Status:       string(store.StatusDeploying),
		})
	}

	writeJSON(w, http.StatusOK, updated)
}
