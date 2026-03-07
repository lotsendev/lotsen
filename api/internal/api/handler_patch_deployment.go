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
	if body.Domain != "" && conflictsWithDashboardDomain(body.Domain) {
		http.Error(w, "domain is reserved for dashboard", http.StatusConflict)
		return
	}
	body.Security = normalizeSecurityConfig(body.Security)

	basicAuth, err := sanitizeAndHashBasicAuth(body.BasicAuth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	effectiveDomain := existing.Domain
	if body.Domain != "" {
		effectiveDomain = body.Domain
	}
	effectivePublic := existing.Public
	if body.Public != nil {
		effectivePublic = *body.Public
	}
	effectivePorts := existing.Ports
	if body.Ports != nil {
		effectivePorts = body.Ports
	}
	effectiveProxyPort := existing.ProxyPort
	if body.ProxyPort != nil {
		effectiveProxyPort = *body.ProxyPort
	}
	if !effectivePublic && !h.privateDomainAllowed(effectiveDomain) {
		http.Error(w, "private deployments must use a domain within LOTSEN_AUTH_COOKIE_DOMAIN", http.StatusBadRequest)
		return
	}

	if body.Ports != nil {
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
		effectivePorts = assignedPorts
	}

	if err := validateProxyPortSelection(effectiveDomain, effectiveProxyPort, effectivePorts); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Preserve Public when the field is absent from the patch request.
	public := existing.Public
	if body.Public != nil {
		public = *body.Public
	}

	patch := store.Deployment{
		Image:        body.Image,
		Envs:         body.Envs,
		Ports:        body.Ports,
		ProxyPort:    effectiveProxyPort,
		ProxyPortSet: body.ProxyPort != nil,
		Volumes:      body.Volumes,
		Domain:       body.Domain,
		Public:       public,
		PublicSet:    body.Public != nil,
		BasicAuth:    basicAuth,
		Security:     body.Security,
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

	writeJSON(w, http.StatusAccepted, normalizeDeploymentSecurity(updated))
}
