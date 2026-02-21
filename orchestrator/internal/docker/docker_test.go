package docker_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/ercadev/dirigent/orchestrator/internal/docker"
	"github.com/ercadev/dirigent/store"
)

type mockClient struct {
	pingErr         error
	imagePullErr    error
	containerCreate container.CreateResponse
	createErr       error
	startErr        error
	listContainers  []dockertypes.Container
	listErr         error
	stopErr         error
	removeErr       error
	stopped         []string
	removed         []string
}

func (m *mockClient) Ping(_ context.Context) (dockertypes.Ping, error) {
	return dockertypes.Ping{}, m.pingErr
}

func (m *mockClient) ImagePull(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
	if m.imagePullErr != nil {
		return nil, m.imagePullErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockClient) ContainerCreate(_ context.Context, _ *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, _ string) (container.CreateResponse, error) {
	return m.containerCreate, m.createErr
}

func (m *mockClient) ContainerStart(_ context.Context, _ string, _ container.StartOptions) error {
	return m.startErr
}

func (m *mockClient) ContainerList(_ context.Context, _ container.ListOptions) ([]dockertypes.Container, error) {
	return m.listContainers, m.listErr
}

func (m *mockClient) ContainerStop(_ context.Context, id string, _ container.StopOptions) error {
	m.stopped = append(m.stopped, id)
	return m.stopErr
}

func (m *mockClient) ContainerRemove(_ context.Context, id string, _ container.RemoveOptions) error {
	m.removed = append(m.removed, id)
	return m.removeErr
}

func deployment() store.Deployment {
	return store.Deployment{
		ID:      "d1",
		Name:    "web",
		Image:   "nginx:latest",
		Envs:    map[string]string{"PORT": "80"},
		Ports:   []string{"80:80"},
		Volumes: []string{"/data:/data"},
		Status:  store.StatusDeploying,
	}
}

func TestDocker_Ping_OK(t *testing.T) {
	d := docker.New(&mockClient{})
	if err := d.Ping(context.Background()); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestDocker_Ping_Unavailable(t *testing.T) {
	d := docker.New(&mockClient{pingErr: errors.New("connection refused")})
	err := d.Ping(context.Background())
	if !errors.Is(err, docker.ErrDockerUnavailable) {
		t.Errorf("want ErrDockerUnavailable, got %v", err)
	}
}

func TestDocker_Start_OK(t *testing.T) {
	mock := &mockClient{containerCreate: container.CreateResponse{ID: "c1"}}
	d := docker.New(mock)
	if err := d.Start(context.Background(), deployment()); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestDocker_Start_ImagePullError(t *testing.T) {
	mock := &mockClient{imagePullErr: errors.New("manifest unknown")}
	d := docker.New(mock)
	err := d.Start(context.Background(), deployment())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if errors.Is(err, docker.ErrDockerUnavailable) {
		t.Error("want non-unavailable error")
	}
}

func TestDocker_ListManagedContainers(t *testing.T) {
	mock := &mockClient{
		listContainers: []dockertypes.Container{
			{
				ID:     "c1",
				Names:  []string{"/web"},
				State:  "running",
				Labels: map[string]string{"dirigent.managed": "true", "dirigent.id": "d1"},
				Ports:  []dockertypes.Port{},
				// Satisfy filters check by having the right labels
			},
		},
	}
	d := docker.New(mock)
	containers, err := d.ListManagedContainers(context.Background())
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("want 1 container, got %d", len(containers))
	}
	if containers[0].DeploymentID != "d1" {
		t.Errorf("want deployment id d1, got %s", containers[0].DeploymentID)
	}
	if !containers[0].Running {
		t.Error("want running=true")
	}
}

func TestDocker_StopAndRemove(t *testing.T) {
	mock := &mockClient{}
	d := docker.New(mock)
	if err := d.StopAndRemove(context.Background(), "c1"); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(mock.stopped) != 1 || mock.stopped[0] != "c1" {
		t.Errorf("want c1 stopped, got %v", mock.stopped)
	}
	if len(mock.removed) != 1 || mock.removed[0] != "c1" {
		t.Errorf("want c1 removed, got %v", mock.removed)
	}
}

// Ensure the mock satisfies the docker.Client interface at compile time.
var _ interface {
	ContainerList(context.Context, container.ListOptions) ([]dockertypes.Container, error)
	Ping(context.Context) (dockertypes.Ping, error)
} = (*mockClient)(nil)

// Ensure filters package is referenced (avoids unused import if linter checks).
var _ = filters.NewArgs
