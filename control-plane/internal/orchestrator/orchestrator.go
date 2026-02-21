package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"

	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/ercadev/dirigent/internal/store"
)

// ErrDockerUnavailable is returned when the Docker daemon cannot be reached.
var ErrDockerUnavailable = errors.New("docker daemon unreachable")

// DockerClient is the Docker API surface required by the orchestrator.
// The real *dockerclient.Client satisfies this interface.
type DockerClient interface {
	Ping(ctx context.Context) (types.Ping, error)
	ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
}

// Orchestrator manages Docker container lifecycle for deployments.
type Orchestrator struct {
	docker DockerClient
}

// New creates an Orchestrator backed by the given Docker client.
func New(docker DockerClient) *Orchestrator {
	return &Orchestrator{docker: docker}
}

// Ping reports whether the Docker daemon is reachable.
// Returns ErrDockerUnavailable if not.
func (o *Orchestrator) Ping(ctx context.Context) error {
	if _, err := o.docker.Ping(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
	}
	return nil
}

// Start pulls the image and creates and starts a container for d.
// The container is named after d.Name, labelled with dirigent metadata,
// and configured with d.Envs, d.Ports, and d.Volumes.
func (o *Orchestrator) Start(ctx context.Context, d store.Deployment) error {
	if _, err := o.docker.Ping(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
	}

	rc, err := o.docker.ImagePull(ctx, d.Image, image.PullOptions{})
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return fmt.Errorf("orchestrator: pull image %s: %w", d.Image, err)
	}
	// Drain response to complete the pull before proceeding.
	_, _ = io.Copy(io.Discard, rc)
	rc.Close()

	env := envsToSlice(d.Envs)

	exposedPorts, portBindings, err := parsePorts(d.Ports)
	if err != nil {
		return fmt.Errorf("orchestrator: parse ports: %w", err)
	}

	cfg := &container.Config{
		Image:        d.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels: map[string]string{
			"dirigent.managed": "true",
			"dirigent.id":      d.ID,
		},
	}

	hostCfg := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        d.Volumes,
	}

	resp, err := o.docker.ContainerCreate(ctx, cfg, hostCfg, nil, nil, d.Name)
	if err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return fmt.Errorf("orchestrator: create container %s: %w", d.Name, err)
	}

	if err := o.docker.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		if dockerclient.IsErrConnectionFailed(err) {
			return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
		}
		return fmt.Errorf("orchestrator: start container %s: %w", resp.ID, err)
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
