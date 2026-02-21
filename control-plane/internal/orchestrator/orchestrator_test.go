package orchestrator_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/ercadev/dirigent/internal/orchestrator"
	"github.com/ercadev/dirigent/internal/store"
)

// mockDockerClient is a controllable Docker client for testing.
type mockDockerClient struct {
	pingErr         error
	imagePullErr    error
	containerCreate container.CreateResponse
	createErr       error
	startErr        error
}

func (m *mockDockerClient) Ping(_ context.Context) (types.Ping, error) {
	return types.Ping{}, m.pingErr
}

func (m *mockDockerClient) ImagePull(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
	if m.imagePullErr != nil {
		return nil, m.imagePullErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockDockerClient) ContainerCreate(_ context.Context, _ *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, _ string) (container.CreateResponse, error) {
	return m.containerCreate, m.createErr
}

func (m *mockDockerClient) ContainerStart(_ context.Context, _ string, _ container.StartOptions) error {
	return m.startErr
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

func TestOrchestrator_Ping_OK(t *testing.T) {
	o := orchestrator.New(&mockDockerClient{})
	if err := o.Ping(context.Background()); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestOrchestrator_Ping_Unavailable(t *testing.T) {
	o := orchestrator.New(&mockDockerClient{pingErr: errors.New("connection refused")})
	err := o.Ping(context.Background())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, orchestrator.ErrDockerUnavailable) {
		t.Errorf("want ErrDockerUnavailable, got %v", err)
	}
}

func TestOrchestrator_Start_OK(t *testing.T) {
	mock := &mockDockerClient{
		containerCreate: container.CreateResponse{ID: "container123"},
	}
	o := orchestrator.New(mock)
	if err := o.Start(context.Background(), deployment()); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestOrchestrator_Start_DockerUnavailableOnPing(t *testing.T) {
	mock := &mockDockerClient{pingErr: errors.New("connection refused")}
	o := orchestrator.New(mock)
	err := o.Start(context.Background(), deployment())
	if !errors.Is(err, orchestrator.ErrDockerUnavailable) {
		t.Errorf("want ErrDockerUnavailable, got %v", err)
	}
}

func TestOrchestrator_Start_ImagePullError(t *testing.T) {
	mock := &mockDockerClient{imagePullErr: errors.New("manifest unknown")}
	o := orchestrator.New(mock)
	err := o.Start(context.Background(), deployment())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if errors.Is(err, orchestrator.ErrDockerUnavailable) {
		t.Errorf("want non-unavailable error, got ErrDockerUnavailable")
	}
}

func TestOrchestrator_Start_ContainerCreateError(t *testing.T) {
	mock := &mockDockerClient{createErr: errors.New("image not found")}
	o := orchestrator.New(mock)
	err := o.Start(context.Background(), deployment())
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestOrchestrator_Start_ContainerStartError(t *testing.T) {
	mock := &mockDockerClient{
		containerCreate: container.CreateResponse{ID: "c1"},
		startErr:        errors.New("port already allocated"),
	}
	o := orchestrator.New(mock)
	err := o.Start(context.Background(), deployment())
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestOrchestrator_Start_NoPorts(t *testing.T) {
	mock := &mockDockerClient{containerCreate: container.CreateResponse{ID: "c1"}}
	o := orchestrator.New(mock)
	d := deployment()
	d.Ports = nil
	if err := o.Start(context.Background(), d); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestOrchestrator_Start_NoEnvsOrVolumes(t *testing.T) {
	mock := &mockDockerClient{containerCreate: container.CreateResponse{ID: "c1"}}
	o := orchestrator.New(mock)
	d := deployment()
	d.Envs = nil
	d.Volumes = nil
	if err := o.Start(context.Background(), d); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}
