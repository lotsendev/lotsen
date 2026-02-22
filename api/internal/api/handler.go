package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ercadev/dirigent/internal/events"
	"github.com/ercadev/dirigent/store"
)

// Store is the persistence interface required by the API handlers.
type Store interface {
	List() ([]store.Deployment, error)
	Get(id string) (store.Deployment, error)
	Create(d store.Deployment) (store.Deployment, error)
	Update(d store.Deployment) (store.Deployment, error)
	Patch(id string, patch store.Deployment) (store.Deployment, error)
	Delete(id string) error
}

// EventBus is the pub/sub interface required by the API handlers.
type EventBus interface {
	Subscribe() (<-chan events.StatusEvent, func())
	Publish(events.StatusEvent)
}

type deploymentRequest struct {
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Envs    map[string]string `json:"envs"`
	Ports   []string          `json:"ports"`
	Volumes []string          `json:"volumes"`
	Domain  string            `json:"domain"`
}

type patchDeploymentRequest struct {
	Image   string            `json:"image"`
	Envs    map[string]string `json:"envs"`
	Ports   []string          `json:"ports"`
	Volumes []string          `json:"volumes"`
	Domain  string            `json:"domain"`
}

// Handler holds the dependencies for the API layer.
type Handler struct {
	store        Store
	events       EventBus
	statusSource SystemStatusProvider
	heartbeats   OrchestratorHeartbeatIngestor
	docker       DockerConnectivityIngestor
}

const defaultOrchestratorStaleAfter = 30 * time.Second

// New creates a Handler backed by the given store and event bus.
func New(s Store, eb EventBus) *Handler {
	return NewWithSystemStatus(s, eb, nil)
}

// NewWithSystemStatus creates a Handler with a custom system-status provider.
// If statusSource is nil, a default provider is used.
func NewWithSystemStatus(s Store, eb EventBus, statusSource SystemStatusProvider) *Handler {
	if statusSource == nil {
		statusSource = newDefaultSystemStatusProvider(time.Now, orchestratorStaleAfterFromEnv())
	}

	heartbeatIngestor, _ := statusSource.(OrchestratorHeartbeatIngestor)
	dockerIngestor, _ := statusSource.(DockerConnectivityIngestor)

	return &Handler{store: s, events: eb, statusSource: statusSource, heartbeats: heartbeatIngestor, docker: dockerIngestor}
}

// RegisterRoutes wires all deployment endpoints into mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/deployments", h.listDeployments)
	mux.HandleFunc("GET /api/system-status", h.systemStatus)
	mux.HandleFunc("POST /api/system-status/orchestrator-heartbeat", h.recordOrchestratorHeartbeat)
	mux.HandleFunc("POST /api/deployments", h.createDeployment)
	mux.HandleFunc("GET /api/deployments/events", h.deploymentEvents)
	mux.HandleFunc("GET /api/deployments/{id}", h.getDeployment)
	mux.HandleFunc("PUT /api/deployments/{id}", h.updateDeployment)
	mux.HandleFunc("DELETE /api/deployments/{id}", h.deleteDeployment)
	mux.HandleFunc("PATCH /api/deployments/{id}/status", h.updateDeploymentStatus)
	mux.HandleFunc("PATCH /api/deployments/{id}", h.patchDeployment)
}

func (h *Handler) recordOrchestratorHeartbeat(w http.ResponseWriter, r *http.Request) {
	if h.heartbeats == nil {
		http.Error(w, "system status unavailable", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB

	var body struct {
		At     *time.Time `json:"at"`
		Docker *struct {
			Reachable *bool      `json:"reachable"`
			CheckedAt *time.Time `json:"checkedAt"`
		} `json:"docker"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	heartbeatAt := time.Time{}
	if body.At != nil {
		heartbeatAt = body.At.UTC()
	}

	if err := h.heartbeats.RecordOrchestratorHeartbeat(r.Context(), heartbeatAt); err != nil {
		http.Error(w, "failed to record heartbeat", http.StatusInternalServerError)
		return
	}

	if body.Docker != nil && body.Docker.Reachable != nil {
		if h.docker == nil {
			http.Error(w, "system status unavailable", http.StatusServiceUnavailable)
			return
		}

		dockerCheckedAt := heartbeatAt
		if body.Docker.CheckedAt != nil {
			dockerCheckedAt = body.Docker.CheckedAt.UTC()
		}

		if err := h.docker.RecordDockerConnectivity(r.Context(), *body.Docker.Reachable, dockerCheckedAt); err != nil {
			http.Error(w, "failed to record docker connectivity", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listDeployments(w http.ResponseWriter, r *http.Request) {
	deployments, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to list deployments", http.StatusInternalServerError)
		return
	}
	if deployments == nil {
		deployments = []store.Deployment{}
	}
	writeJSON(w, http.StatusOK, deployments)
}

func (h *Handler) getDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	d, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get deployment", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, d)
}

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

	id, err := newID()
	if err != nil {
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}

	if body.Envs == nil {
		body.Envs = map[string]string{}
	}
	if body.Ports == nil {
		body.Ports = []string{}
	}
	if body.Volumes == nil {
		body.Volumes = []string{}
	}

	d := store.Deployment{
		ID:      id,
		Name:    body.Name,
		Image:   body.Image,
		Envs:    body.Envs,
		Ports:   body.Ports,
		Volumes: body.Volumes,
		Domain:  body.Domain,
		Status:  store.StatusDeploying,
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

	writeJSON(w, http.StatusCreated, created)
}

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

	if body.Envs == nil {
		body.Envs = map[string]string{}
	}
	if body.Ports == nil {
		body.Ports = []string{}
	}
	if body.Volumes == nil {
		body.Volumes = []string{}
	}

	d := store.Deployment{
		ID:      id,
		Name:    body.Name,
		Image:   body.Image,
		Envs:    body.Envs,
		Ports:   body.Ports,
		Volumes: body.Volumes,
		Domain:  body.Domain,
		Status:  store.StatusDeploying,
	}

	updated, err := h.store.Update(d)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update deployment", http.StatusInternalServerError)
		return
	}

	h.events.Publish(events.StatusEvent{
		DeploymentID: updated.ID,
		Status:       string(store.StatusDeploying),
	})

	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) deleteDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete deployment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) updateDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Status store.Status `json:"status"`
		Error  string       `json:"error"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	switch body.Status {
	case store.StatusIdle, store.StatusDeploying, store.StatusHealthy, store.StatusFailed:
		// valid
	default:
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	d, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get deployment", http.StatusInternalServerError)
		return
	}

	d.Status = body.Status
	updated, err := h.store.Update(d)
	if err != nil {
		http.Error(w, "failed to update deployment", http.StatusInternalServerError)
		return
	}

	h.events.Publish(events.StatusEvent{
		DeploymentID: id,
		Status:       string(body.Status),
		Error:        body.Error,
	})

	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) patchDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body patchDeploymentRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	patch := store.Deployment{
		Image:   body.Image,
		Envs:    body.Envs,
		Ports:   body.Ports,
		Volumes: body.Volumes,
		Domain:  body.Domain,
		Status:  store.StatusDeploying,
	}

	updated, err := h.store.Patch(id, patch)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update deployment", http.StatusInternalServerError)
		return
	}

	h.events.Publish(events.StatusEvent{
		DeploymentID: id,
		Status:       string(store.StatusDeploying),
	})

	writeJSON(w, http.StatusAccepted, updated)
}

// deploymentEvents streams deployment status-change events as SSE.
// The stream stays open until the client disconnects.
func (h *Handler) deploymentEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, cancel := h.events.Subscribe()
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("deploymentEvents: marshal event: %v", err)
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				log.Printf("deploymentEvents: write: %v", err)
				return
			}
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: encode: %v", err)
	}
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("newID: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func orchestratorStaleAfterFromEnv() time.Duration {
	raw := os.Getenv("DIRIGENT_ORCHESTRATOR_STALE_AFTER")
	if raw == "" {
		return defaultOrchestratorStaleAfter
	}

	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		log.Printf("invalid DIRIGENT_ORCHESTRATOR_STALE_AFTER=%q; using default %s", raw, defaultOrchestratorStaleAfter)
		return defaultOrchestratorStaleAfter
	}

	return d
}
