package docker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/ercadev/dirigent/store"
)

// ErrDockerUnavailable is returned when the Docker daemon cannot be reached.
var ErrDockerUnavailable = errors.New("docker daemon unreachable")

// ExitDetails captures why a stopped container exited.
// It is nil for containers that are currently running.
type ExitDetails struct {
	ExitCode  int
	OOMKilled bool
	Error     string
}

// ManagedContainer represents a container managed by Lotsen.
type ManagedContainer struct {
	ID           string
	Name         string
	DeploymentID string
	Running      bool
	ExitDetails  *ExitDetails // nil when running
}

// Client is the Docker API surface required by the orchestrator.
type Client interface {
	Ping(ctx context.Context) (dockertypes.Ping, error)
	ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]dockertypes.Container, error)
	ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerRename(ctx context.Context, containerID string, newName string) error
	ContainerInspect(ctx context.Context, containerID string) (dockertypes.ContainerJSON, error)
}

// ContainerStats captures point-in-time runtime usage for a running container.
type ContainerStats struct {
	CPUPercent       float64
	MemoryUsedBytes  uint64
	MemoryLimitBytes uint64
	MemoryPercent    float64
}

// Docker manages container lifecycle for Lotsen deployments.
type Docker struct {
	client      Client
	proxyURL    string
	proxyClient *http.Client
}

// New creates a Docker backed by the given client.
func New(client Client) *Docker {
	return &Docker{
		client:      client,
		proxyURL:    proxyURLFromEnv(),
		proxyClient: &http.Client{Timeout: 3 * time.Second},
	}
}

// Ping reports whether the Docker daemon is reachable.
// Returns ErrDockerUnavailable if not.
func (d *Docker) Ping(ctx context.Context) error {
	if _, err := d.client.Ping(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
	}
	return nil
}

// Start pulls the image and creates and starts a container for the deployment.
func (d *Docker) Start(ctx context.Context, dep store.Deployment) ([]string, error) {
	if _, err := d.client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
	}

	if err := d.pullImage(dep.Image); err != nil {
		return nil, err
	}

	env := envsToSlice(dep.Envs)

	exposedPorts, portBindings, err := parsePorts(dep.Ports)
	if err != nil {
		return nil, fmt.Errorf("docker: parse ports: %w", err)
	}

	cfg := &container.Config{
		Image:        dep.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels: map[string]string{
			"lotsen.managed": "true",
			"lotsen.id":      dep.ID,
		},
	}

	hostCfg := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        dep.Volumes,
	}

	resp, err := d.client.ContainerCreate(ctx, cfg, hostCfg, nil, nil, dep.Name)
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return nil, fmt.Errorf("docker: create container %s: %w", dep.Name, err)
	}

	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return nil, fmt.Errorf("docker: start container %s: %w", resp.ID, err)
	}

	runtimePorts, err := d.resolvedPorts(ctx, resp.ID)
	if err != nil {
		_ = d.client.ContainerStop(ctx, resp.ID, container.StopOptions{})
		_ = d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("docker: resolve runtime ports for %s: %w", resp.ID, err)
	}

	return runtimePorts, nil
}

// ListManagedContainers returns all containers with the lotsen.managed=true label.
func (d *Docker) ListManagedContainers(ctx context.Context) ([]ManagedContainer, error) {
	f := filters.NewArgs(filters.Arg("label", "lotsen.managed=true"))
	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return nil, fmt.Errorf("docker: list containers: %w", err)
	}

	result := make([]ManagedContainer, 0, len(containers))
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
		}
		mc := ManagedContainer{
			ID:           c.ID,
			Name:         name,
			DeploymentID: c.Labels["lotsen.id"],
			Running:      c.State == "running",
		}
		if !mc.Running {
			if inspect, err := d.client.ContainerInspect(ctx, c.ID); err == nil && inspect.State != nil {
				mc.ExitDetails = &ExitDetails{
					ExitCode:  inspect.State.ExitCode,
					OOMKilled: inspect.State.OOMKilled,
					Error:     inspect.State.Error,
				}
			}
		}
		result = append(result, mc)
	}

	return result, nil
}

// CollectStats returns runtime CPU and memory usage for each running managed
// container keyed by deployment ID.
func (d *Docker) CollectStats(ctx context.Context) (map[string]ContainerStats, error) {
	f := filters.NewArgs(
		filters.Arg("label", "lotsen.managed=true"),
		filters.Arg("status", "running"),
	)

	containers, err := d.client.ContainerList(ctx, container.ListOptions{All: false, Filters: f})
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return nil, fmt.Errorf("docker: list running containers: %w", err)
	}

	type result struct {
		deploymentID string
		stats        ContainerStats
	}

	resultCh := make(chan result, len(containers))
	var wg sync.WaitGroup

	for _, c := range containers {
		deploymentID := strings.TrimSpace(c.Labels["lotsen.id"])
		if deploymentID == "" {
			continue
		}

		wg.Add(1)
		go func(containerID, depID string) {
			defer wg.Done()

			reader, err := d.client.ContainerStats(ctx, containerID, false)
			if err != nil {
				log.Printf("docker: stats for container %s: %v", containerID, err)
				return
			}
			stat, decodeErr := decodeContainerStats(reader.Body)
			_ = reader.Body.Close()
			if decodeErr != nil {
				log.Printf("docker: decode stats for container %s: %v", containerID, decodeErr)
				return
			}

			resultCh <- result{deploymentID: depID, stats: stat}
		}(c.ID, deploymentID)
	}

	wg.Wait()
	close(resultCh)

	statsByDeployment := make(map[string]ContainerStats, len(containers))
	for r := range resultCh {
		statsByDeployment[r.deploymentID] = r.stats
	}

	return statsByDeployment, nil
}

// StopAndRemove stops and removes the container with the given ID.
func (d *Docker) StopAndRemove(ctx context.Context, containerID string) error {
	if err := d.client.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return fmt.Errorf("docker: stop container %s: %w", containerID, err)
	}

	if err := d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{}); err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return fmt.Errorf("docker: remove container %s: %w", containerID, err)
	}

	return nil
}

// StartAndReplace implements a start-then-stop redeploy strategy: it pulls the new
// image, starts a temporary container, verifies it reaches running state, then stops
// and removes the old container before renaming the new one to the deployment name.
// If the new container fails to reach running state the old container is left intact
// and an error is returned.
func (d *Docker) StartAndReplace(ctx context.Context, dep store.Deployment, oldContainerID string) ([]string, error) {
	if _, err := d.client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
	}

	fallback, err := d.shouldStopBeforeStart(ctx, oldContainerID, dep.Ports)
	if err != nil {
		return nil, fmt.Errorf("docker: compare port bindings: %w", err)
	}
	if fallback {
		if err := d.StopAndRemove(ctx, oldContainerID); err != nil {
			return nil, fmt.Errorf("docker: stop old container %s before replace: %w", oldContainerID, err)
		}
		return d.Start(ctx, dep)
	}

	if err := d.pullImage(dep.Image); err != nil {
		return nil, err
	}

	env := envsToSlice(dep.Envs)

	exposedPorts, portBindings, err := parsePorts(dep.Ports)
	if err != nil {
		return nil, fmt.Errorf("docker: parse ports: %w", err)
	}

	cfg := &container.Config{
		Image:        dep.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels: map[string]string{
			"lotsen.managed": "true",
			"lotsen.id":      dep.ID,
		},
	}
	hostCfg := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        dep.Volumes,
	}

	nextName := dep.Name + "-next"
	resp, err := d.client.ContainerCreate(ctx, cfg, hostCfg, nil, nil, nextName)
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return nil, fmt.Errorf("docker: create container %s: %w", nextName, err)
	}

	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		if dockerclient.IsErrConnectionFailed(err) {
			return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return nil, fmt.Errorf("docker: start container %s: %w", resp.ID, err)
	}

	// Verify the new container reached running state.
	f := filters.NewArgs(filters.Arg("id", resp.ID))
	running, err := d.client.ContainerList(ctx, container.ListOptions{All: false, Filters: f})
	if err != nil {
		_ = d.client.ContainerStop(ctx, resp.ID, container.StopOptions{})
		_ = d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("docker: inspect new container %s: %w", resp.ID, err)
	}
	if len(running) == 0 {
		_ = d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("docker: new container %s did not reach running state", resp.ID)
	}

	runtimePorts, err := d.resolvedPorts(ctx, resp.ID)
	if err != nil {
		_ = d.client.ContainerStop(ctx, resp.ID, container.StopOptions{})
		_ = d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("docker: resolve runtime ports for %s: %w", resp.ID, err)
	}

	if err := d.swapProxyRoute(ctx, dep.Domain, dep.ProxyPort, runtimePorts); err != nil {
		_ = d.client.ContainerStop(ctx, resp.ID, container.StopOptions{})
		_ = d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("docker: swap proxy route for %q: %w", dep.Domain, err)
	}

	// New container is healthy; tear down the old one.
	if err := d.StopAndRemove(ctx, oldContainerID); err != nil {
		log.Printf("docker: remove old container %s: %v", oldContainerID, err)
	}

	// Rename new container to the canonical deployment name.
	if err := d.client.ContainerRename(ctx, resp.ID, dep.Name); err != nil {
		log.Printf("docker: rename %s to %s: %v", resp.ID, dep.Name, err)
	}

	return runtimePorts, nil
}

// pullImage pulls imageRef from the registry, unless the image already exists
// locally under an immutable (non-latest) tag, in which case the pull is
// skipped to avoid an unnecessary CPU spike from layer decompression.
//
// The pull runs with its own 10-minute context, decoupled from the reconcile
// loop's short deadline. Without this, a large image on a slow connection
// would time out, be retried on every reconcile cycle, and cause multiple
// concurrent decompression jobs that saturate the host CPU.
func (d *Docker) pullImage(imageRef string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// For pinned tags and digest refs, skip the pull when the image already
	// exists locally — re-pulling is unnecessary and causes a CPU spike.
	if !isLatestTag(imageRef) {
		f := filters.NewArgs(filters.Arg("reference", imageRef))
		imgs, err := d.client.ImageList(ctx, image.ListOptions{Filters: f})
		if err == nil && len(imgs) > 0 {
			return nil
		}
	}

	log.Printf("docker: pulling image %s", imageRef)
	options := image.PullOptions{}
	registryAuth, err := d.resolveRegistryAuth(imageRef)
	if err != nil {
		return fmt.Errorf("docker: resolve registry auth for %s: %w", imageRef, err)
	}
	if registryAuth != "" {
		options.RegistryAuth = registryAuth
	}

	rc, err := d.client.ImagePull(ctx, imageRef, options)
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return fmt.Errorf("docker: pull image %s: %w", imageRef, err)
	}
	_, _ = io.Copy(io.Discard, rc)
	rc.Close()
	return nil
}

func (d *Docker) resolveRegistryAuth(imageRef string) (string, error) {
	s, err := store.NewJSONStore(dataPath())
	if err != nil {
		log.Printf("docker: registry auth unavailable: %v", err)
		return "", nil
	}

	auth, err := s.ResolveRegistryAuth(imageRef)
	if err != nil {
		return "", err
	}
	if auth == nil {
		return "", nil
	}

	payload, err := json.Marshal(registry.AuthConfig{Username: auth.Username, Password: auth.Password})
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(payload), nil
}

func dataPath() string {
	if p := strings.TrimSpace(os.Getenv("LOTSEN_DATA")); p != "" {
		return p
	}
	return "/var/lib/lotsen/deployments.json"
}

// isLatestTag reports whether imageRef resolves to the mutable :latest tag.
// Digest-pinned refs (image@sha256:…) and explicit non-latest tags are
// considered immutable.
func isLatestTag(imageRef string) bool {
	// Digest-pinned refs are immutable.
	if strings.Contains(imageRef, "@") {
		return false
	}
	_, tag, found := strings.Cut(imageRef, ":")
	// No tag specified → Docker defaults to :latest.
	return !found || tag == "" || tag == "latest"
}

// envsToSlice converts a map of environment variables to the KEY=VALUE slice
// format expected by the Docker API.
func envsToSlice(envs map[string]string) []string {
	out := make([]string, 0, len(envs))
	for k, v := range envs {
		out = append(out, k+"="+v)
	}
	return out
}

// parsePorts converts Lotsen port strings to the Docker port set and binding
// map required by HostConfig. Container-only declarations (for example "3001")
// are rewritten to "0:3001" so Docker assigns an available host port.
func parsePorts(ports []string) (nat.PortSet, nat.PortMap, error) {
	if len(ports) == 0 {
		return nat.PortSet{}, nat.PortMap{}, nil
	}
	normalized := make([]string, 0, len(ports))
	for _, p := range ports {
		if p != "" && !strings.Contains(p, ":") {
			normalized = append(normalized, "0:"+p)
			continue
		}
		normalized = append(normalized, p)
	}

	exposed, bindings, err := nat.ParsePortSpecs(normalized)
	if err != nil {
		return nil, nil, err
	}
	return nat.PortSet(exposed), bindings, nil
}

func (d *Docker) resolvedPorts(ctx context.Context, containerID string) ([]string, error) {
	inspect, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return nil, err
	}
	if inspect.NetworkSettings == nil {
		return nil, fmt.Errorf("network settings not available")
	}

	ports := make([]string, 0, len(inspect.NetworkSettings.Ports))
	keys := make([]string, 0, len(inspect.NetworkSettings.Ports))
	byKey := make(map[string]nat.Port, len(inspect.NetworkSettings.Ports))
	for p := range inspect.NetworkSettings.Ports {
		key := string(p)
		keys = append(keys, key)
		byKey[key] = p
	}
	sort.Strings(keys)

	for _, key := range keys {
		p := byKey[key]
		bindings := inspect.NetworkSettings.Ports[p]
		for _, b := range bindings {
			if b.HostPort == "" {
				continue
			}
			ports = append(ports, b.HostPort+":"+p.Port())
			break
		}
	}

	return ports, nil
}

func (d *Docker) shouldStopBeforeStart(ctx context.Context, oldContainerID string, desiredPorts []string) (bool, error) {
	desired, err := desiredExplicitHostPorts(desiredPorts)
	if err != nil {
		return false, err
	}
	if len(desired) == 0 {
		return false, nil
	}

	inspect, err := d.client.ContainerInspect(ctx, oldContainerID)
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return false, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return false, err
	}
	if inspect.NetworkSettings == nil {
		return false, nil
	}

	current := make(map[string]struct{}, len(inspect.NetworkSettings.Ports))
	for port, bindings := range inspect.NetworkSettings.Ports {
		for _, b := range bindings {
			if b.HostPort == "" {
				continue
			}
			current[b.HostPort+"/"+port.Proto()] = struct{}{}
			break
		}
	}

	for _, hostPortProto := range desired {
		if _, overlap := current[hostPortProto]; overlap {
			return true, nil
		}
	}

	return false, nil
}

func desiredExplicitHostPorts(ports []string) ([]string, error) {
	if len(ports) == 0 {
		return []string{}, nil
	}

	_, bindings, err := parsePorts(ports)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(bindings))
	for port, b := range bindings {
		if len(b) == 0 {
			continue
		}
		hostPort := b[0].HostPort
		if hostPort == "" || hostPort == "0" {
			continue
		}
		result = append(result, hostPort+"/"+port.Proto())
	}
	return result, nil
}

type routeRequest struct {
	Domain   string `json:"domain"`
	Upstream string `json:"upstream"`
}

func (d *Docker) swapProxyRoute(ctx context.Context, domain string, proxyPort int, runtimePorts []string) error {
	domain = normalizeDomain(domain)
	if domain == "" {
		return nil
	}

	upstream := upstreamFromRuntimePorts(runtimePorts, proxyPort)
	if upstream == "" {
		return fmt.Errorf("runtime upstream not available")
	}

	body, err := json.Marshal(routeRequest{Domain: domain, Upstream: upstream})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.proxyURL+"/internal/routes", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.proxyClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return nil
}

func upstreamFromRuntimePorts(runtimePorts []string, proxyPort int) string {
	if proxyPort > 0 {
		wanted := strconv.Itoa(proxyPort)
		for _, binding := range runtimePorts {
			host, container, ok := strings.Cut(binding, ":")
			if !ok || host == "" || container == "" {
				continue
			}
			if container == wanted {
				return "localhost:" + host
			}
		}
		return ""
	}

	for _, binding := range runtimePorts {
		host, _, ok := strings.Cut(binding, ":")
		if !ok || host == "" {
			continue
		}
		return "localhost:" + host
	}
	return ""
}

func proxyURLFromEnv() string {
	if baseURL := strings.TrimSpace(os.Getenv("LOTSEN_PROXY_URL")); baseURL != "" {
		return strings.TrimSuffix(baseURL, "/")
	}
	return "http://localhost:2019"
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")
	return strings.ToLower(domain)
}

func decodeContainerStats(body io.Reader) (ContainerStats, error) {
	var payload struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs  uint32 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64            `json:"usage"`
			Limit uint64            `json:"limit"`
			Stats map[string]uint64 `json:"stats"`
		} `json:"memory_stats"`
	}

	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return ContainerStats{}, fmt.Errorf("decode body: %w", err)
	}

	return ContainerStats{
		CPUPercent:       calculateCPUPercent(payload.CPUStats, payload.PreCPUStats),
		MemoryUsedBytes:  calculateMemoryUsedBytes(payload.MemoryStats.Usage, payload.MemoryStats.Stats),
		MemoryLimitBytes: payload.MemoryStats.Limit,
		MemoryPercent:    calculateMemoryPercent(payload.MemoryStats.Usage, payload.MemoryStats.Limit, payload.MemoryStats.Stats),
	}, nil
}

func calculateCPUPercent(current struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs  uint32 `json:"online_cpus"`
}, previous struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemUsage uint64 `json:"system_cpu_usage"`
}) float64 {
	cpuDelta := int64(current.CPUUsage.TotalUsage) - int64(previous.CPUUsage.TotalUsage)
	systemDelta := int64(current.SystemUsage) - int64(previous.SystemUsage)
	if cpuDelta <= 0 || systemDelta <= 0 {
		return 0
	}

	onlineCPUs := current.OnlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}

	return (float64(cpuDelta) / float64(systemDelta)) * float64(onlineCPUs) * 100
}

func calculateMemoryUsedBytes(usage uint64, stats map[string]uint64) uint64 {
	if usage == 0 {
		return 0
	}

	cache := stats["total_inactive_file"]
	if cache == 0 {
		cache = stats["inactive_file"]
	}
	if usage > cache {
		return usage - cache
	}

	return usage
}

func calculateMemoryPercent(usage uint64, limit uint64, stats map[string]uint64) float64 {
	if limit == 0 {
		return 0
	}

	used := calculateMemoryUsedBytes(usage, stats)
	return (float64(used) / float64(limit)) * 100
}
