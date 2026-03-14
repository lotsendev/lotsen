package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lotsendev/lotsen/internal/events"
	"github.com/lotsendev/lotsen/store"
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
	if !body.Public && !h.privateDomainAllowed(body.Domain) {
		http.Error(w, "private deployments must use a domain within LOTSEN_AUTH_COOKIE_DOMAIN", http.StatusBadRequest)
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
	resolvedVolumes, err := resolveVolumeBindings(id, body.Volumes, body.VolumeMounts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resolvedFileMounts, err := resolveFileMounts(id, body.FileMounts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body.Volumes = resolvedVolumes
	body.Security = normalizeSecurityConfig(body.Security)

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

	assignedPorts, err := assignHostPorts(allDeployments, id, existing.Ports, body.Ports)
	if err != nil {
		var conflictErr hostPortConflictError
		switch {
		case errors.As(err, &conflictErr):
			http.Error(w, conflictErr.Error(), http.StatusConflict)
		case errors.Is(err, errNoHostPortsAvailable):
			http.Error(w, "no host ports available", http.StatusServiceUnavailable)
		default:
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	body.Ports = assignedPorts
	if err := validateProxyPortSelection(body.Domain, body.ProxyPort, body.Ports); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if updateRequestMatchesExisting(existing, body, basicAuth) {
		writeJSON(w, http.StatusOK, deploymentResponseFromStore(existing, nil))
		return
	}

	nextStatus := existing.Status
	if updateRequiresRedeploy(existing, body) {
		nextStatus = store.StatusDeploying
	}

	d := store.Deployment{
		ID:         id,
		Name:       body.Name,
		Image:      body.Image,
		Envs:       body.Envs,
		Ports:      body.Ports,
		ProxyPort:  body.ProxyPort,
		Volumes:    body.Volumes,
		FileMounts: resolvedFileMounts,
		Domain:     body.Domain,
		Public:     body.Public,
		BasicAuth:  basicAuth,
		Security:   body.Security,
		Status:     nextStatus,
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

	writeJSON(w, http.StatusOK, deploymentResponseFromStore(updated, nil))
}
