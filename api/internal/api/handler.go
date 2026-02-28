package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ercadev/dirigent/internal/events"
	"github.com/ercadev/dirigent/internal/upgrade"
	"github.com/ercadev/dirigent/internal/version"
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

type VersionInfoProvider interface {
	Snapshot(ctx context.Context) (version.Snapshot, error)
	Releases(ctx context.Context, limit int) ([]version.Release, error)
}

type UpgradeRunner interface {
	Start(targetVersion string) error
	Subscribe() (<-chan string, func(), error)
	IsRunning() bool
}

type basicAuthUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type basicAuthRequest struct {
	Users []basicAuthUserRequest `json:"users"`
}

type deploymentRequest struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Envs      map[string]string `json:"envs"`
	Ports     []string          `json:"ports"`
	Volumes   []string          `json:"volumes"`
	Domain    string            `json:"domain"`
	BasicAuth *basicAuthRequest `json:"basic_auth"`
}

type patchDeploymentRequest struct {
	Image     string            `json:"image"`
	Envs      map[string]string `json:"envs"`
	Ports     []string          `json:"ports"`
	Volumes   []string          `json:"volumes"`
	Domain    string            `json:"domain"`
	BasicAuth *basicAuthRequest `json:"basic_auth"`
}

// Handler holds the dependencies for the API layer.
type Handler struct {
	store        Store
	events       EventBus
	dockerLogs   DockerLogs
	statusEvents *systemStatusBroker
	accessLogDir string
	statusSource SystemStatusProvider
	heartbeats   OrchestratorHeartbeatIngestor
	docker       DockerConnectivityIngestor
	loadBalancer LoadBalancerHealthIngestor
	proxyClient  *http.Client
	proxyBaseURL string
	cpu          CPUUtilizationIngestor
	ram          RAMUtilizationIngestor
	versions     VersionInfoProvider
	upgrade      UpgradeRunner
}

const defaultOrchestratorStaleAfter = 30 * time.Second

// New creates a Handler backed by the given store, event bus, and Docker log streamer.
func New(s Store, eb EventBus, dl DockerLogs) *Handler {
	return NewWithVersion(s, eb, dl, "dev")
}

func NewWithVersion(s Store, eb EventBus, dl DockerLogs, currentVersion string) *Handler {
	if currentVersion == "" {
		currentVersion = "dev"
	}

	return NewWithDependencies(
		s,
		eb,
		dl,
		nil,
		version.New(currentVersion),
		upgrade.New(),
	)
}

// NewWithSystemStatus creates a Handler with a custom system-status provider.
// If statusSource is nil, a default provider is used.
func NewWithSystemStatus(s Store, eb EventBus, dl DockerLogs, statusSource SystemStatusProvider) *Handler {
	return NewWithDependencies(s, eb, dl, statusSource, version.New("dev"), upgrade.New())
}

func NewWithDependencies(s Store, eb EventBus, dl DockerLogs, statusSource SystemStatusProvider, versions VersionInfoProvider, upgrader UpgradeRunner) *Handler {
	if statusSource == nil {
		statusSource = newDefaultSystemStatusProvider(time.Now, orchestratorStaleAfterFromEnv(), buildAPIStoreChecker(s))
	}
	if versions == nil {
		versions = version.New("dev")
	}
	if upgrader == nil {
		upgrader = upgrade.New()
	}

	heartbeatIngestor, _ := statusSource.(OrchestratorHeartbeatIngestor)
	dockerIngestor, _ := statusSource.(DockerConnectivityIngestor)
	loadBalancerIngestor, _ := statusSource.(LoadBalancerHealthIngestor)
	cpuIngestor, _ := statusSource.(CPUUtilizationIngestor)
	ramIngestor, _ := statusSource.(RAMUtilizationIngestor)

	return &Handler{
		store:        s,
		events:       eb,
		dockerLogs:   dl,
		statusEvents: newSystemStatusBroker(),
		accessLogDir: proxyAccessLogDirFromEnv(),
		statusSource: statusSource,
		heartbeats:   heartbeatIngestor,
		docker:       dockerIngestor,
		loadBalancer: loadBalancerIngestor,
		proxyClient:  &http.Client{Timeout: 3 * time.Second},
		proxyBaseURL: proxyInternalBaseURLFromEnv(),
		cpu:          cpuIngestor,
		ram:          ramIngestor,
		versions:     versions,
		upgrade:      upgrader,
	}
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
	mux.HandleFunc("GET /api/system-status/events", h.systemStatusEvents)
	mux.HandleFunc("GET /api/load-balancer/access-logs", h.loadBalancerAccessLogs)
	mux.HandleFunc("POST /api/system-status/orchestrator-heartbeat", h.recordOrchestratorHeartbeat)
	mux.HandleFunc("GET /api/version", h.getVersion)
	mux.HandleFunc("GET /api/version/releases", h.getVersionReleases)
	mux.HandleFunc("GET /api/access-logs", h.accessLogs)
	mux.HandleFunc("GET /api/security-config", h.securityConfig)
	mux.HandleFunc("POST /api/upgrade", h.startUpgrade)
	mux.HandleFunc("GET /api/upgrade/logs", h.upgradeLogs)
	mux.HandleFunc("POST /api/deployments", h.createDeployment)
	mux.HandleFunc("GET /api/deployments/events", h.deploymentEvents)
	mux.HandleFunc("GET /api/deployments/{id}/logs", h.deploymentLogs)
	mux.HandleFunc("GET /api/deployments/{id}", h.getDeployment)
	mux.HandleFunc("PUT /api/deployments/{id}", h.updateDeployment)
	mux.HandleFunc("DELETE /api/deployments/{id}", h.deleteDeployment)
	mux.HandleFunc("PATCH /api/deployments/{id}/status", h.updateDeploymentStatus)
	mux.HandleFunc("PATCH /api/deployments/{id}", h.patchDeployment)
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

func proxyAccessLogDirFromEnv() string {
	if dir := strings.TrimSpace(os.Getenv("DIRIGENT_PROXY_ACCESS_LOG_DIR")); dir != "" {
		return dir
	}
	return "/var/lib/dirigent/logs/proxy"
}

func proxyInternalBaseURLFromEnv() string {
	baseURL := strings.TrimSpace(os.Getenv("DIRIGENT_PROXY_INTERNAL_URL"))
	if baseURL == "" {
		return "http://127.0.0.1"
	}
	return strings.TrimSuffix(baseURL, "/")
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
		!slices.Equal(existing.Volumes, body.Volumes) ||
		!equalBasicAuthConfig(existing.BasicAuth, body.BasicAuth)
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
	if body.BasicAuth != nil && !equalBasicAuthConfig(existing.BasicAuth, body.BasicAuth) {
		return true
	}
	return false
}

func sanitizeAndHashBasicAuth(input *basicAuthRequest) (*store.BasicAuthConfig, error) {
	if input == nil {
		return nil, nil
	}
	if len(input.Users) == 0 {
		return nil, nil
	}
	users := make([]store.BasicAuthUser, 0, len(input.Users))
	for _, user := range input.Users {
		username := strings.TrimSpace(user.Username)
		if username == "" {
			return nil, fmt.Errorf("basic auth username is required")
		}
		password := strings.TrimSpace(user.Password)
		if password == "" {
			return nil, fmt.Errorf("basic auth password is required")
		}

		if _, err := bcrypt.Cost([]byte(password)); err != nil {
			hashed, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if hashErr != nil {
				return nil, fmt.Errorf("hash basic auth password: %w", hashErr)
			}
			password = string(hashed)
		}

		users = append(users, store.BasicAuthUser{Username: username, Password: password})
	}

	return &store.BasicAuthConfig{Users: users}, nil
}

func dashboardDomainFromEnv() string {
	return normalizeDomain(os.Getenv("DIRIGENT_DASHBOARD_DOMAIN"))
}

func conflictsWithDashboardDomain(domain string) bool {
	dashboardDomain := dashboardDomainFromEnv()
	if dashboardDomain == "" {
		return false
	}
	return normalizeDomain(domain) == dashboardDomain
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")
	return strings.ToLower(domain)
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

func equalBasicAuthConfig(existing *store.BasicAuthConfig, request *basicAuthRequest) bool {
	if request == nil {
		return existing == nil
	}
	if existing == nil {
		return false
	}
	if len(existing.Users) != len(request.Users) {
		return false
	}
	for i, user := range request.Users {
		if existing.Users[i].Username != strings.TrimSpace(user.Username) {
			return false
		}
		if existing.Users[i].Password != strings.TrimSpace(user.Password) {
			return false
		}
	}
	return true
}
