package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lotsendev/lotsen/internal/events"
	"github.com/lotsendev/lotsen/store"
)

func (h *Handler) createDeployment(w http.ResponseWriter, r *http.Request) {
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

	id, err := newID()
	if err != nil {
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}

	if body.Envs == nil {
		body.Envs = map[string]string{}
	}
	if body.Volumes == nil {
		body.Volumes = []string{}
	}
	body.Security = normalizeSecurityConfig(body.Security)

	allDeployments, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to list deployments", http.StatusInternalServerError)
		return
	}

	assignedPorts, err := assignHostPorts(allDeployments, "", nil, body.Ports)
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
	if err := validateProxyPortSelection(body.Domain, body.ProxyPort, assignedPorts); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	d := store.Deployment{
		ID:        id,
		Name:      body.Name,
		Image:     body.Image,
		Envs:      body.Envs,
		Ports:     assignedPorts,
		ProxyPort: body.ProxyPort,
		Volumes:   body.Volumes,
		Domain:    body.Domain,
		Public:    body.Public,
		BasicAuth: basicAuth,
		Security:  body.Security,
		Status:    store.StatusDeploying,
	}

	created, err := h.store.Create(d)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateName) {
			http.Error(w, "deployment name already in use", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create deployment", http.StatusInternalServerError)
		return
	}

	h.events.Publish(events.StatusEvent{
		DeploymentID: created.ID,
		Status:       string(store.StatusDeploying),
	})

	writeJSON(w, http.StatusCreated, normalizeDeploymentSecurity(created))
}
