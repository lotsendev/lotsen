package api

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
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

// DockerLogs streams container logs on demand. StreamLogs returns a reader
// that emits demultiplexed log lines for the running container associated with
// deploymentID, starting from the last tail lines. Returns (nil, nil) when no
// container is currently running for the deployment.
type DockerLogs interface {
	StreamLogs(ctx context.Context, deploymentID string, tail int) (io.ReadCloser, error)
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
	dockerLogs   DockerLogs
	statusSource SystemStatusProvider
	heartbeats   OrchestratorHeartbeatIngestor
	docker       DockerConnectivityIngestor
	loadBalancer LoadBalancerHealthIngestor
	cpu          CPUUtilizationIngestor
	ram          RAMUtilizationIngestor
}

const defaultOrchestratorStaleAfter = 30 * time.Second

// New creates a Handler backed by the given store, event bus, and Docker log streamer.
func New(s Store, eb EventBus, dl DockerLogs) *Handler {
	return NewWithSystemStatus(s, eb, dl, nil)
}

// NewWithSystemStatus creates a Handler with a custom system-status provider.
// If statusSource is nil, a default provider is used.
func NewWithSystemStatus(s Store, eb EventBus, dl DockerLogs, statusSource SystemStatusProvider) *Handler {
	if statusSource == nil {
		statusSource = newDefaultSystemStatusProvider(time.Now, orchestratorStaleAfterFromEnv(), buildAPIStoreChecker(s))
	}

	heartbeatIngestor, _ := statusSource.(OrchestratorHeartbeatIngestor)
	dockerIngestor, _ := statusSource.(DockerConnectivityIngestor)
	loadBalancerIngestor, _ := statusSource.(LoadBalancerHealthIngestor)
	cpuIngestor, _ := statusSource.(CPUUtilizationIngestor)
	ramIngestor, _ := statusSource.(RAMUtilizationIngestor)

	return &Handler{store: s, events: eb, dockerLogs: dl, statusSource: statusSource, heartbeats: heartbeatIngestor, docker: dockerIngestor, loadBalancer: loadBalancerIngestor, cpu: cpuIngestor, ram: ramIngestor}
}

func buildAPIStoreChecker(s Store) func(context.Context) bool {
	if s == nil {
		return nil
	}

	return func(_ context.Context) bool {
		_, err := s.List()
		return err == nil
	}
}

// RegisterRoutes wires all deployment endpoints into mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/deployments", h.listDeployments)
	mux.HandleFunc("GET /api/system-status", h.systemStatus)
	mux.HandleFunc("POST /api/system-status/orchestrator-heartbeat", h.recordOrchestratorHeartbeat)
	mux.HandleFunc("POST /api/deployments", h.createDeployment)
	mux.HandleFunc("GET /api/deployments/events", h.deploymentEvents)
	mux.HandleFunc("GET /api/deployments/{id}/logs", h.deploymentLogs)
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
		At           *time.Time `json:"at"`
		Orchestrator *struct {
			StoreAccessible *bool `json:"storeAccessible"`
		} `json:"orchestrator"`
		Docker *struct {
			Reachable *bool      `json:"reachable"`
			CheckedAt *time.Time `json:"checkedAt"`
		} `json:"docker"`
		LoadBalancer *struct {
			Responding *bool      `json:"responding"`
			CheckedAt  *time.Time `json:"checkedAt"`
		} `json:"loadBalancer"`
		Host *struct {
			CPU *struct {
				UsagePercent *float64   `json:"usagePercent"`
				CheckedAt    *time.Time `json:"checkedAt"`
			} `json:"cpu"`
			RAM *struct {
				UsagePercent *float64   `json:"usagePercent"`
				CheckedAt    *time.Time `json:"checkedAt"`
			} `json:"ram"`
		} `json:"host"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	heartbeatAt := time.Time{}
	if body.At != nil {
		heartbeatAt = body.At.UTC()
	}

	storeAccessible := true
	if body.Orchestrator != nil && body.Orchestrator.StoreAccessible != nil {
		storeAccessible = *body.Orchestrator.StoreAccessible
	}

	if err := h.heartbeats.RecordOrchestratorHeartbeat(r.Context(), heartbeatAt, storeAccessible); err != nil {
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

	if body.LoadBalancer != nil && body.LoadBalancer.Responding != nil {
		if h.loadBalancer == nil {
			http.Error(w, "system status unavailable", http.StatusServiceUnavailable)
			return
		}

		checkedAt := heartbeatAt
		if body.LoadBalancer.CheckedAt != nil {
			checkedAt = body.LoadBalancer.CheckedAt.UTC()
		}

		if err := h.loadBalancer.RecordLoadBalancerHealth(r.Context(), *body.LoadBalancer.Responding, checkedAt); err != nil {
			http.Error(w, "failed to record load balancer health", http.StatusInternalServerError)
			return
		}
	}

	if body.Host != nil {
		if body.Host.CPU != nil && body.Host.CPU.UsagePercent != nil {
			if h.cpu == nil {
				http.Error(w, "system status unavailable", http.StatusServiceUnavailable)
				return
			}

			if !isValidUsagePercent(*body.Host.CPU.UsagePercent) {
				http.Error(w, "invalid host metrics", http.StatusBadRequest)
				return
			}

			cpuCheckedAt := heartbeatAt
			if body.Host.CPU.CheckedAt != nil {
				cpuCheckedAt = body.Host.CPU.CheckedAt.UTC()
			}

			if err := h.cpu.RecordCPUUtilization(r.Context(), *body.Host.CPU.UsagePercent, cpuCheckedAt); err != nil {
				http.Error(w, "failed to record cpu utilization", http.StatusInternalServerError)
				return
			}
		}

		if body.Host.RAM != nil && body.Host.RAM.UsagePercent != nil {
			if h.ram == nil {
				http.Error(w, "system status unavailable", http.StatusServiceUnavailable)
				return
			}

			if !isValidUsagePercent(*body.Host.RAM.UsagePercent) {
				http.Error(w, "invalid host metrics", http.StatusBadRequest)
				return
			}

			ramCheckedAt := heartbeatAt
			if body.Host.RAM.CheckedAt != nil {
				ramCheckedAt = body.Host.RAM.CheckedAt.UTC()
			}

			if err := h.ram.RecordRAMUtilization(r.Context(), *body.Host.RAM.UsagePercent, ramCheckedAt); err != nil {
				http.Error(w, "failed to record ram utilization", http.StatusInternalServerError)
				return
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func isValidUsagePercent(v float64) bool {
	return v >= 0 && v <= 100
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
	if body.Volumes == nil {
		body.Volumes = []string{}
	}

	allDeployments, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to list deployments", http.StatusInternalServerError)
		return
	}

	assignedPorts, err := assignHostPorts(allDeployments, "", nil, normalizeContainerPorts(body.Ports))
	if err != nil {
		http.Error(w, "no host ports available", http.StatusServiceUnavailable)
		return
	}

	d := store.Deployment{
		ID:      id,
		Name:    body.Name,
		Image:   body.Image,
		Envs:    body.Envs,
		Ports:   assignedPorts,
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
		ID:      id,
		Name:    body.Name,
		Image:   body.Image,
		Envs:    body.Envs,
		Ports:   body.Ports,
		Volumes: body.Volumes,
		Domain:  body.Domain,
		Status:  nextStatus,
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
	d.Error = body.Error
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

// deploymentLogs streams container log lines for a deployment as SSE.
// The last 100 lines are sent immediately on connect; new lines are pushed as
// they appear. The stream stays open until the client disconnects or the
// container exits.
func (h *Handler) deploymentLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	id := r.PathValue("id")

	if _, err := h.store.Get(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get deployment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	rc, err := h.dockerLogs.StreamLogs(r.Context(), id, 100)
	if err != nil {
		log.Printf("deploymentLogs: stream logs for %s: %v", id, err)
		return
	}
	if rc == nil {
		// No container is running for this deployment yet.
		return
	}
	defer rc.Close()

	logLine := struct {
		Line string `json:"line"`
	}{}

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		logLine.Line = scanner.Text()
		data, err := json.Marshal(logLine)
		if err != nil {
			log.Printf("deploymentLogs: marshal line: %v", err)
			continue
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			log.Printf("deploymentLogs: write: %v", err)
			return
		}
		flusher.Flush()
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

const (
	hostPortRangeMin = 32768
	hostPortRangeMax = 60999
)

// containerPortOnly strips any user-supplied host port prefix and protocol suffix
// from a port spec, returning only the container port number.
// "8080:80" → "80", "80/tcp" → "80", "80" → "80".
func containerPortOnly(spec string) string {
	if idx := strings.LastIndex(spec, ":"); idx != -1 {
		spec = spec[idx+1:]
	}
	if idx := strings.Index(spec, "/"); idx != -1 {
		spec = spec[:idx]
	}
	return strings.TrimSpace(spec)
}

// normalizeContainerPorts strips host port prefixes from every entry and
// deduplicates the result, preserving order.
func normalizeContainerPorts(specs []string) []string {
	seen := make(map[string]struct{}, len(specs))
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		cp := containerPortOnly(s)
		if cp == "" {
			continue
		}
		if _, dup := seen[cp]; dup {
			continue
		}
		seen[cp] = struct{}{}
		out = append(out, cp)
	}
	return out
}

// hostPortFromBinding extracts the host port number from a "hostPort:containerPort"
// binding string. Returns 0 if the binding is not in that format.
func hostPortFromBinding(binding string) int {
	idx := strings.Index(binding, ":")
	if idx == -1 {
		return 0
	}
	n, err := strconv.Atoi(binding[:idx])
	if err != nil {
		return 0
	}
	return n
}

// containerPortFromBinding extracts the container port from a "hostPort:containerPort"
// binding string. Returns the full string if there is no colon.
func containerPortFromBinding(binding string) string {
	idx := strings.Index(binding, ":")
	if idx == -1 {
		return binding
	}
	return binding[idx+1:]
}

// assignHostPorts returns stable, conflict-free "hostPort:containerPort" bindings
// for the given container ports.
//
//   - allDeployments: current store snapshot used to find occupied host ports.
//   - skipID: deployment being created/updated — its existing host ports are NOT
//     counted as occupied so they can be reused (stability).
//   - currentBindings: the Ports slice of the deployment being updated, used to
//     carry over existing host-port assignments for unchanged container ports.
func assignHostPorts(allDeployments []store.Deployment, skipID string, currentBindings []string, containerPorts []string) ([]string, error) {
	if len(containerPorts) == 0 {
		return []string{}, nil
	}

	// Collect host ports in use by other deployments.
	usedHostPorts := make(map[int]struct{})
	for _, d := range allDeployments {
		if d.ID == skipID {
			continue
		}
		for _, p := range d.Ports {
			if hp := hostPortFromBinding(p); hp > 0 {
				usedHostPorts[hp] = struct{}{}
			}
		}
	}

	// Build a map of containerPort → existing host port for the deployment being
	// updated so we can reuse the same host port (stable across redeployments).
	existing := make(map[string]int, len(currentBindings))
	for _, p := range currentBindings {
		hp := hostPortFromBinding(p)
		cp := containerPortFromBinding(p)
		if hp > 0 && cp != "" {
			existing[cp] = hp
		}
	}

	result := make([]string, 0, len(containerPorts))
	for _, cp := range containerPorts {
		// Reuse the existing host port for this container port when it is still free.
		if hp, ok := existing[cp]; ok {
			if _, inUse := usedHostPorts[hp]; !inUse {
				result = append(result, fmt.Sprintf("%d:%s", hp, cp))
				usedHostPorts[hp] = struct{}{} // prevent double-allocation within batch
				continue
			}
		}

		// Assign a new free host port from the reserved range.
		hp, err := allocateHostPort(usedHostPorts)
		if err != nil {
			return nil, err
		}
		usedHostPorts[hp] = struct{}{}
		result = append(result, fmt.Sprintf("%d:%s", hp, cp))
	}

	return result, nil
}

func allocateHostPort(used map[int]struct{}) (int, error) {
	for port := hostPortRangeMin; port <= hostPortRangeMax; port++ {
		if _, inUse := used[port]; !inUse {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free host port available in range %d–%d", hostPortRangeMin, hostPortRangeMax)
}

func updateRequiresRedeploy(existing store.Deployment, body deploymentRequest) bool {
	return existing.Image != body.Image ||
		!equalStringMap(existing.Envs, body.Envs) ||
		!slices.Equal(existing.Ports, body.Ports) ||
		!slices.Equal(existing.Volumes, body.Volumes)
}

func patchRequiresRedeploy(existing store.Deployment, body patchDeploymentRequest) bool {
	if body.Image != "" && existing.Image != body.Image {
		return true
	}
	if body.Envs != nil && !equalStringMap(existing.Envs, body.Envs) {
		return true
	}
	if body.Ports != nil && !slices.Equal(existing.Ports, body.Ports) {
		return true
	}
	if body.Volumes != nil && !slices.Equal(existing.Volumes, body.Volumes) {
		return true
	}
	return false
}

func equalStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if bv, ok := b[k]; !ok || bv != av {
			return false
		}
	}
	return true
}
