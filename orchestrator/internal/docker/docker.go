package docker

import (
	"context"
	"errors"
	"fmt"
	"io"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/ercadev/dirigent/orchestrator/internal/store"
)

// ErrDockerUnavailable is returned when the Docker daemon cannot be reached.
var ErrDockerUnavailable = errors.New("docker daemon unreachable")

// ManagedContainer represents a running container managed by Dirigent.
type ManagedContainer struct {
	ID           string
	Name         string
	DeploymentID string
	Running      bool
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
func (d *Docker) Start(ctx context.Context, dep store.Deployment) error {
	if _, err := d.client.Ping(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
	}

	rc, err := d.client.ImagePull(ctx, dep.Image, image.PullOptions{})
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return fmt.Errorf("docker: pull image %s: %w", dep.Image, err)
	}
	_, _ = io.Copy(io.Discard, rc)
	rc.Close()

	env := envsToSlice(dep.Envs)

	exposedPorts, portBindings, err := parsePorts(dep.Ports)
	if err != nil {
		return fmt.Errorf("docker: parse ports: %w", err)
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
			return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return fmt.Errorf("docker: create container %s: %w", dep.Name, err)
	}

	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return fmt.Errorf("docker: start container %s: %w", resp.ID, err)
	}

	return nil
}

// ListManagedContainers returns all containers with the dirigent.managed=true label.
func (d *Docker) ListManagedContainers(ctx context.Context) ([]ManagedContainer, error) {
	f := filters.NewArgs(filters.Arg("label", "dirigent.managed=true"))
	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
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
		result = append(result, ManagedContainer{
			ID:           c.ID,
			Name:         name,
			DeploymentID: c.Labels["dirigent.id"],
			Running:      c.State == "running",
		})
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

// envsToSlice converts a map of environment variables to the KEY=VALUE slice
// format expected by the Docker API.
func envsToSlice(envs map[string]string) []string {
	out := make([]string, 0, len(envs))
	for k, v := range envs {
		out = append(out, k+"="+v)
	}
	return out
}

// parsePorts converts Dirigent port strings ("hostPort:containerPort") to the
// Docker port set and binding map required by HostConfig.
func parsePorts(ports []string) (nat.PortSet, nat.PortMap, error) {
	if len(ports) == 0 {
		return nat.PortSet{}, nat.PortMap{}, nil
	}
	exposed, bindings, err := nat.ParsePortSpecs(ports)
	if err != nil {
		return nil, nil, err
	}
	return nat.PortSet(exposed), bindings, nil
}
