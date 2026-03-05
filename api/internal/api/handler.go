package api

import (
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

	"golang.org/x/crypto/bcrypt"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/ercadev/dirigent/auth"
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
	ListRegistries() ([]store.RegistryEntry, error)
	CreateRegistry(id, prefix, username, password string) (store.RegistryEntry, error)
	UpdateRegistry(id, prefix, username, password string) (store.RegistryEntry, error)
	DeleteRegistry(id string) error
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
	RecentLogs(ctx context.Context, deploymentID string, tail int) ([]string, error)
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

// AuthUserStore manages Lotsen users and passkey credentials.
type AuthUserStore interface {
	HasAnyUser() (bool, error)
	CreateUser(username string) error
	ListUsers() ([]auth.User, error)
	DeleteUser(username string) error

	GetWebAuthnUser(username string) (*auth.WebAuthnUser, error)
	SavePasskey(username string, cred *webauthn.Credential, deviceName string) error
	ListPasskeys(username string) ([]auth.PasskeyInfo, error)
	DeletePasskey(credID, username string) error
	UpdatePasskeySignCount(credID []byte, count uint32) error

	CreateInviteToken(token string, expiresAt time.Time) error
	ValidateInviteToken(token string) error
	ConsumeInviteToken(token string) error
}

type HostProfileStore interface {
	Get() (HostProfile, error)
	UpdateDisplayName(displayName string) (HostProfile, error)
}

type basicAuthUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type basicAuthRequest struct {
	Users []basicAuthUserRequest `json:"users"`
}

type deploymentRequest struct {
	Name      string                `json:"name"`
	Image     string                `json:"image"`
	Envs      map[string]string     `json:"envs"`
	Ports     []string              `json:"ports"`
	Volumes   []string              `json:"volumes"`
	Domain    string                `json:"domain"`
	Public    bool                  `json:"public"`
	BasicAuth *basicAuthRequest     `json:"basic_auth"`
	Security  *store.SecurityConfig `json:"security"`
}

type patchDeploymentRequest struct {
	Image     string                `json:"image"`
	Envs      map[string]string     `json:"envs"`
	Ports     []string              `json:"ports"`
	Volumes   []string              `json:"volumes"`
	Domain    string                `json:"domain"`
	Public    *bool                 `json:"public"`
	BasicAuth *basicAuthRequest     `json:"basic_auth"`
	Security  *store.SecurityConfig `json:"security"`
}

// Handler holds the dependencies for the API layer.
type Handler struct {
	store            Store
	events           EventBus
	dockerLogs       DockerLogs
	statusEvents     *systemStatusBroker
	accessLogDir     string
	statusSource     SystemStatusProvider
	heartbeats       OrchestratorHeartbeatIngestor
	docker           DockerConnectivityIngestor
	loadBalancer     LoadBalancerHealthIngestor
	proxyClient      *http.Client
	proxyBaseURL     string
	cpu              CPUUtilizationIngestor
	ram              RAMUtilizationIngestor
	versions         VersionInfoProvider
	upgrade          UpgradeRunner
	containerStats   *ContainerStatsCache
	authStore        AuthUserStore
	jwtSecret        []byte
	authCookieDomain string
	webAuthn         *webauthn.WebAuthn
	challenges       *passkeySessionStore
	hostProfiles     HostProfileStore
	hostMetadata     HostMetadataIngestor
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
	hostMetadataIngestor, _ := statusSource.(HostMetadataIngestor)

	return &Handler{
		store:          s,
		events:         eb,
		dockerLogs:     dl,
		statusEvents:   newSystemStatusBroker(),
		accessLogDir:   proxyAccessLogDirFromEnv(),
		statusSource:   statusSource,
		heartbeats:     heartbeatIngestor,
		docker:         dockerIngestor,
		loadBalancer:   loadBalancerIngestor,
		proxyClient:    &http.Client{Timeout: 3 * time.Second},
		proxyBaseURL:   proxyInternalBaseURLFromEnv(),
		cpu:            cpuIngestor,
		ram:            ramIngestor,
		versions:       versions,
		upgrade:        upgrader,
		containerStats: NewContainerStatsCache(),
		hostMetadata:   hostMetadataIngestor,
	}
}

// SetAuth configures the user store and JWT secret for API authentication.
// When not called (or called with nil/empty arguments), all API routes are open.
func (h *Handler) SetAuth(userStore AuthUserStore, jwtSecret []byte) {
	h.authStore = userStore
	h.jwtSecret = jwtSecret
}

func (h *Handler) SetAuthCookieDomain(domain string) {
	h.authCookieDomain = normalizeDomain(strings.TrimPrefix(strings.TrimSpace(domain), "."))
}

// SetWebAuthn configures the WebAuthn relying party for passkey authentication.
func (h *Handler) SetWebAuthn(wa *webauthn.WebAuthn) {
	h.webAuthn = wa
	h.challenges = newPasskeySessionStore()
}

func (h *Handler) SetHostProfileStore(store HostProfileStore) {
	h.hostProfiles = store
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

// RegisterRoutes wires all endpoints into mux. Auth endpoints are open;
// all /api/* routes require a valid JWT when an authStore is configured.
// Internal orchestrator endpoints (heartbeat and status update) are exempt.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Auth — always open.
	mux.HandleFunc("POST /auth/logout", h.logout)
	mux.HandleFunc("GET /auth/me", h.me)

	// Setup (first-run, open only when no users exist).
	mux.HandleFunc("GET /auth/setup-available", h.setupAvailable)
	mux.HandleFunc("POST /auth/passkey/setup/begin", h.passkeySetupBegin)
	mux.HandleFunc("POST /auth/passkey/setup/finish", h.passkeySetupFinish)

	// Invite-based registration (open, token-gated).
	mux.HandleFunc("GET /auth/invite", h.validateInvite)
	mux.HandleFunc("POST /auth/passkey/invite/begin", h.passkeyInviteBegin)
	mux.HandleFunc("POST /auth/passkey/invite/finish", h.passkeyInviteFinish)

	// Login (open).
	mux.HandleFunc("POST /auth/passkey/login/begin", h.passkeyLoginBegin)
	mux.HandleFunc("POST /auth/passkey/login/finish", h.passkeyLoginFinish)

	// Orchestrator-internal endpoints — exempt from JWT so the orchestrator
	// process can call them without user credentials.
	mux.HandleFunc("POST /api/system-status/orchestrator-heartbeat", h.recordOrchestratorHeartbeat)
	mux.HandleFunc("PATCH /api/deployments/{id}/status", h.updateDeploymentStatus)

	// All remaining API routes require JWT when auth is configured.
	protect := h.requireAuth
	mux.Handle("GET /api/deployments", protect(http.HandlerFunc(h.listDeployments)))
	mux.Handle("GET /api/system-status", protect(http.HandlerFunc(h.systemStatus)))
	mux.Handle("GET /api/system-status/events", protect(http.HandlerFunc(h.systemStatusEvents)))
	mux.Handle("GET /api/core-services/logs", protect(http.HandlerFunc(h.coreServiceLogs)))
	mux.Handle("GET /api/load-balancer/access-logs", protect(http.HandlerFunc(h.loadBalancerAccessLogs)))
	mux.Handle("GET /api/version", protect(http.HandlerFunc(h.getVersion)))
	mux.Handle("GET /api/version/releases", protect(http.HandlerFunc(h.getVersionReleases)))
	mux.Handle("GET /api/host", protect(http.HandlerFunc(h.getHost)))
	mux.Handle("PUT /api/host", protect(http.HandlerFunc(h.updateHost)))
	mux.Handle("GET /api/users", protect(http.HandlerFunc(h.listUsers)))
	mux.Handle("POST /api/users", protect(http.HandlerFunc(h.createUser)))
	mux.Handle("DELETE /api/users/{username}", protect(http.HandlerFunc(h.deleteUser)))
	mux.Handle("POST /api/invites", protect(http.HandlerFunc(h.createInvite)))
	mux.Handle("GET /api/passkeys", protect(http.HandlerFunc(h.listPasskeys)))
	mux.Handle("DELETE /api/passkeys/{id}", protect(http.HandlerFunc(h.deletePasskey)))
	mux.Handle("GET /api/registries", protect(http.HandlerFunc(h.listRegistries)))
	mux.Handle("POST /api/registries", protect(http.HandlerFunc(h.createRegistry)))
	mux.Handle("PUT /api/registries/{id}", protect(http.HandlerFunc(h.updateRegistry)))
	mux.Handle("DELETE /api/registries/{id}", protect(http.HandlerFunc(h.deleteRegistry)))
	mux.Handle("GET /api/access-logs", protect(http.HandlerFunc(h.accessLogs)))
	mux.Handle("GET /api/security-config", protect(http.HandlerFunc(h.securityConfig)))
	mux.Handle("POST /api/upgrade", protect(http.HandlerFunc(h.startUpgrade)))
	mux.Handle("GET /api/upgrade/logs", protect(http.HandlerFunc(h.upgradeLogs)))
	mux.Handle("POST /api/deployments", protect(http.HandlerFunc(h.createDeployment)))
	mux.Handle("GET /api/deployments/events", protect(http.HandlerFunc(h.deploymentEvents)))
	mux.Handle("GET /api/deployments/{id}/logs", protect(http.HandlerFunc(h.deploymentLogs)))
	mux.Handle("GET /api/deployments/{id}/logs/recent", protect(http.HandlerFunc(h.deploymentRecentLogs)))
	mux.Handle("GET /api/deployments/{id}", protect(http.HandlerFunc(h.getDeployment)))
	mux.Handle("PUT /api/deployments/{id}", protect(http.HandlerFunc(h.updateDeployment)))
	mux.Handle("POST /api/deployments/{id}/restart", protect(http.HandlerFunc(h.restartDeployment)))
	mux.Handle("DELETE /api/deployments/{id}", protect(http.HandlerFunc(h.deleteDeployment)))
	mux.Handle("PATCH /api/deployments/{id}", protect(http.HandlerFunc(h.patchDeployment)))
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
	raw := os.Getenv("LOTSEN_ORCHESTRATOR_STALE_AFTER")
	if raw == "" {
		return defaultOrchestratorStaleAfter
	}

	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		log.Printf("invalid LOTSEN_ORCHESTRATOR_STALE_AFTER=%q; using default %s", raw, defaultOrchestratorStaleAfter)
		return defaultOrchestratorStaleAfter
	}

	return d
}

func proxyAccessLogDirFromEnv() string {
	if dir := strings.TrimSpace(os.Getenv("LOTSEN_PROXY_ACCESS_LOG_DIR")); dir != "" {
		return dir
	}
	return "/var/lib/lotsen/logs/proxy"
}

func proxyInternalBaseURLFromEnv() string {
	baseURL := strings.TrimSpace(os.Getenv("LOTSEN_PROXY_INTERNAL_URL"))
	if baseURL == "" {
		return "http://127.0.0.1"
	}
	return strings.TrimSuffix(baseURL, "/")
}

const (
	hostPortRangeMin = 32768
	hostPortRangeMax = 60999
	wafModeDetection = "detection"
	wafModeEnforce   = "enforcement"
)

var errNoHostPortsAvailable = errors.New("no host ports available")

type hostPortConflictError struct {
	HostPort int
	Protocol string
}

func (e hostPortConflictError) Error() string {
	return fmt.Sprintf("host port %d/%s is already in use", e.HostPort, e.Protocol)
}

type portSpec struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

func (p portSpec) withHostPort(hostPort int) portSpec {
	p.HostPort = hostPort
	return p
}

func (p portSpec) containerKey() string {
	return fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol)
}

func (p portSpec) hostProtocolKey() string {
	if p.HostPort <= 0 {
		return ""
	}
	return fmt.Sprintf("%d/%s", p.HostPort, p.Protocol)
}

func (p portSpec) binding() string {
	binding := fmt.Sprintf("%d:%d", p.HostPort, p.ContainerPort)
	if p.Protocol != "tcp" {
		binding += "/" + p.Protocol
	}
	return binding
}

func parsePortSpecs(specs []string) ([]portSpec, error) {
	parsed := make([]portSpec, 0, len(specs))
	seenContainer := make(map[string]struct{}, len(specs))
	seenBinding := make(map[string]struct{}, len(specs))

	for _, raw := range specs {
		p, err := parsePortSpec(raw)
		if err != nil {
			return nil, err
		}

		if p.HostPort > 0 {
			key := p.hostProtocolKey() + ":" + p.containerKey()
			if _, dup := seenBinding[key]; dup {
				continue
			}
			seenBinding[key] = struct{}{}
			parsed = append(parsed, p)
			continue
		}

		key := p.containerKey()
		if _, dup := seenContainer[key]; dup {
			continue
		}
		seenContainer[key] = struct{}{}
		parsed = append(parsed, p)
	}

	return parsed, nil
}

func parsePortSpec(raw string) (portSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return portSpec{}, fmt.Errorf("invalid port mapping: value is empty")
	}

	mainPart, protocol, hasProtocol := strings.Cut(raw, "/")
	if hasProtocol {
		protocol = strings.ToLower(strings.TrimSpace(protocol))
		if protocol != "tcp" && protocol != "udp" {
			return portSpec{}, fmt.Errorf("invalid port mapping %q: protocol must be tcp or udp", raw)
		}
	} else {
		protocol = "tcp"
	}

	parts := strings.Split(mainPart, ":")
	if len(parts) > 2 {
		return portSpec{}, fmt.Errorf("invalid port mapping %q", raw)
	}

	containerPart := strings.TrimSpace(parts[len(parts)-1])
	containerPort, err := parseValidPortNumber(containerPart)
	if err != nil {
		return portSpec{}, fmt.Errorf("invalid port mapping %q: %w", raw, err)
	}

	p := portSpec{ContainerPort: containerPort, Protocol: protocol}
	if len(parts) == 1 {
		return p, nil
	}

	hostPart := strings.TrimSpace(parts[0])
	hostPort, err := parseValidPortNumber(hostPart)
	if err != nil {
		return portSpec{}, fmt.Errorf("invalid port mapping %q: %w", raw, err)
	}
	p.HostPort = hostPort

	return p, nil
}

func parseValidPortNumber(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("port must be numeric")
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}
	return port, nil
}

// assignHostPorts returns stable, conflict-free host bindings for requested
// port specs.
//
//   - allDeployments: current store snapshot used to find occupied host ports.
//   - skipID: deployment being created/updated — its existing host ports are NOT
//     counted as occupied so they can be reused (stability).
//   - currentBindings: the Ports slice of the deployment being updated, used to
//     carry over existing host-port assignments for unchanged container ports.
func assignHostPorts(allDeployments []store.Deployment, skipID string, currentBindings []string, requestedPorts []string) ([]string, error) {
	if len(requestedPorts) == 0 {
		return []string{}, nil
	}

	requested, err := parsePortSpecs(requestedPorts)
	if err != nil {
		return nil, err
	}

	usedHostPorts := make(map[string]struct{})
	for _, d := range allDeployments {
		if d.ID == skipID {
			continue
		}
		for _, p := range d.Ports {
			spec, err := parsePortSpec(p)
			if err != nil {
				continue
			}
			if key := spec.hostProtocolKey(); key != "" {
				usedHostPorts[key] = struct{}{}
			}
		}
	}

	existing := make(map[string]int, len(currentBindings))
	for _, p := range currentBindings {
		spec, err := parsePortSpec(p)
		if err != nil {
			continue
		}
		if spec.HostPort > 0 {
			existing[spec.containerKey()] = spec.HostPort
		}
	}

	result := make([]string, 0, len(requested))
	for _, requestedPort := range requested {
		if requestedPort.HostPort > 0 {
			key := requestedPort.hostProtocolKey()
			if _, inUse := usedHostPorts[key]; inUse {
				return nil, hostPortConflictError{HostPort: requestedPort.HostPort, Protocol: requestedPort.Protocol}
			}
			usedHostPorts[key] = struct{}{}
			result = append(result, requestedPort.binding())
			continue
		}

		if hp, ok := existing[requestedPort.containerKey()]; ok {
			port := requestedPort.withHostPort(hp)
			key := port.hostProtocolKey()
			if _, inUse := usedHostPorts[key]; !inUse {
				usedHostPorts[key] = struct{}{}
				result = append(result, port.binding())
				continue
			}
		}

		hostPort, err := allocateHostPort(usedHostPorts, requestedPort.Protocol)
		if err != nil {
			return nil, err
		}
		port := requestedPort.withHostPort(hostPort)
		usedHostPorts[port.hostProtocolKey()] = struct{}{}
		result = append(result, port.binding())
	}

	return result, nil
}

func allocateHostPort(used map[string]struct{}, protocol string) (int, error) {
	for port := hostPortRangeMin; port <= hostPortRangeMax; port++ {
		key := fmt.Sprintf("%d/%s", port, protocol)
		if _, inUse := used[key]; !inUse {
			return port, nil
		}
	}
	return 0, errNoHostPortsAvailable
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
	return normalizeDomain(os.Getenv("LOTSEN_DASHBOARD_DOMAIN"))
}

func conflictsWithDashboardDomain(domain string) bool {
	dashboardDomain := dashboardDomainFromEnv()
	if dashboardDomain == "" {
		return false
	}
	return normalizeDomain(domain) == dashboardDomain
}

func (h *Handler) privateDomainAllowed(domain string) bool {
	if h.authCookieDomain == "" {
		return true
	}
	domain = normalizeDomain(domain)
	if domain == "" {
		return false
	}
	return domain == h.authCookieDomain || strings.HasSuffix(domain, "."+h.authCookieDomain)
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

func equalStoredBasicAuthConfig(a, b *store.BasicAuthConfig) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.Users) != len(b.Users) {
		return false
	}
	for i := range a.Users {
		if a.Users[i].Username != b.Users[i].Username || a.Users[i].Password != b.Users[i].Password {
			return false
		}
	}
	return true
}

func equalSecurityConfig(existing, request *store.SecurityConfig) bool {
	if existing == nil || request == nil {
		return existing == request
	}

	return existing.WAFEnabled == request.WAFEnabled &&
		normalizeWAFMode(existing.WAFMode) == normalizeWAFMode(request.WAFMode) &&
		slices.Equal(existing.IPDenylist, request.IPDenylist) &&
		slices.Equal(existing.IPAllowlist, request.IPAllowlist) &&
		slices.Equal(existing.CustomRules, request.CustomRules)
}

func normalizeSecurityConfig(cfg *store.SecurityConfig) *store.SecurityConfig {
	if cfg == nil {
		return nil
	}
	copied := *cfg
	copied.WAFMode = normalizeWAFMode(copied.WAFMode)
	return &copied
}

func normalizeDeploymentSecurity(d store.Deployment) store.Deployment {
	d.Security = normalizeSecurityConfig(d.Security)
	return d
}

func normalizeWAFMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case wafModeEnforce:
		return wafModeEnforce
	default:
		return wafModeDetection
	}
}

func updateRequestMatchesExisting(existing store.Deployment, body deploymentRequest, basicAuth *store.BasicAuthConfig) bool {
	return existing.Name == body.Name &&
		existing.Image == body.Image &&
		equalStringMap(existing.Envs, body.Envs) &&
		slices.Equal(existing.Ports, body.Ports) &&
		slices.Equal(existing.Volumes, body.Volumes) &&
		existing.Domain == body.Domain &&
		existing.Public == body.Public &&
		equalStoredBasicAuthConfig(existing.BasicAuth, basicAuth) &&
		equalSecurityConfig(existing.Security, body.Security)
}
