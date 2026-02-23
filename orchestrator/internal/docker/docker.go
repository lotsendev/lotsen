package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
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

// ManagedContainer represents a container managed by Dirigent.
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
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]dockertypes.Container, error)
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerRename(ctx context.Context, containerID string, newName string) error
	ContainerInspect(ctx context.Context, containerID string) (dockertypes.ContainerJSON, error)
}

// Docker manages container lifecycle for Dirigent deployments.
type Docker struct {
	client Client
}

// New creates a Docker backed by the given client.
func New(client Client) *Docker {
	return &Docker{client: client}
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

	rc, err := d.client.ImagePull(ctx, dep.Image, image.PullOptions{})
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return nil, fmt.Errorf("docker: pull image %s: %w", dep.Image, err)
	}
	_, _ = io.Copy(io.Discard, rc)
	rc.Close()

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
			"dirigent.managed": "true",
			"dirigent.id":      dep.ID,
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

// ListManagedContainers returns all containers with the dirigent.managed=true label.
func (d *Docker) ListManagedContainers(ctx context.Context) ([]ManagedContainer, error) {
	f := filters.NewArgs(filters.Arg("label", "dirigent.managed=true"))
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
			DeploymentID: c.Labels["dirigent.id"],
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

	rc, err := d.client.ImagePull(ctx, dep.Image, image.PullOptions{})
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return nil, fmt.Errorf("docker: pull image %s: %w", dep.Image, err)
	}
	_, _ = io.Copy(io.Discard, rc)
	rc.Close()

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
			"dirigent.managed": "true",
			"dirigent.id":      dep.ID,
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

// envsToSlice converts a map of environment variables to the KEY=VALUE slice
// format expected by the Docker API.
func envsToSlice(envs map[string]string) []string {
	out := make([]string, 0, len(envs))
	for k, v := range envs {
		out = append(out, k+"="+v)
	}
	return out
}

// parsePorts converts Dirigent port strings to the Docker port set and binding
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

	current := make(map[string]string, len(inspect.NetworkSettings.Ports))
	for p, bindings := range inspect.NetworkSettings.Ports {
		for _, b := range bindings {
			if b.HostPort == "" {
				continue
			}
			current[p.Port()] = b.HostPort
			break
		}
	}

	if len(current) != len(desired) {
		return false, nil
	}
	for containerPort, hostPort := range desired {
		if current[containerPort] != hostPort {
			return false, nil
		}
	}
	return true, nil
}

func desiredExplicitHostPorts(ports []string) (map[string]string, error) {
	if len(ports) == 0 {
		return map[string]string{}, nil
	}

	_, bindings, err := parsePorts(ports)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(bindings))
	for p, b := range bindings {
		if len(b) == 0 {
			continue
		}
		hostPort := b[0].HostPort
		if hostPort == "" || hostPort == "0" {
			continue
		}
		result[p.Port()] = hostPort
	}
	return result, nil
}
