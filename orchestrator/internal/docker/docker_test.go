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
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/ercadev/dirigent/orchestrator/internal/docker"
	"github.com/ercadev/dirigent/store"
)

type mockClient struct {
	pingErr          error
	imagePullErr     error
	containerCreate  container.CreateResponse
	createErr        error
	startErr         error
	listContainers   []dockertypes.Container
	listErr          error
	stopErr          error
	removeErr        error
	renameErr        error
	inspectContainer dockertypes.ContainerJSON
	inspectErr       error
	createdHostCfg   *container.HostConfig
	stopped          []string
	removed          []string
	renamed          []string
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

func (m *mockClient) ContainerCreate(_ context.Context, _ *container.Config, hostCfg *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, _ string) (container.CreateResponse, error) {
	m.createdHostCfg = hostCfg
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

func (m *mockClient) ContainerRename(_ context.Context, id string, _ string) error {
	m.renamed = append(m.renamed, id)
	return m.renameErr
}

func (m *mockClient) ContainerInspect(_ context.Context, _ string) (dockertypes.ContainerJSON, error) {
	if m.inspectErr != nil {
		return dockertypes.ContainerJSON{}, m.inspectErr
	}
	return m.inspectContainer, nil
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
	mock := &mockClient{
		containerCreate:  container.CreateResponse{ID: "c1"},
		inspectContainer: inspectWithPort("80/tcp", "80"),
	}
	d := docker.New(mock)
	if _, err := d.Start(context.Background(), deployment()); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestDocker_Start_ContainerOnlyPort_AssignsHostPort(t *testing.T) {
	mock := &mockClient{
		containerCreate:  container.CreateResponse{ID: "c1"},
		inspectContainer: inspectWithPort("3001/tcp", "49123"),
	}
	d := docker.New(mock)

	dep := deployment()
	dep.Ports = []string{"3001"}

	ports, err := d.Start(context.Background(), dep)
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	if len(ports) != 1 || ports[0] != "49123:3001" {
		t.Fatalf("want runtime ports [49123:3001], got %v", ports)
	}

	if mock.createdHostCfg == nil {
		t.Fatal("want ContainerCreate called with host config")
	}
	b := mock.createdHostCfg.PortBindings["3001/tcp"]
	if len(b) != 1 || b[0].HostPort != "0" {
		t.Fatalf("want host port auto-assigned (0), got %v", b)
	}
}

func TestDocker_Start_ImagePullError(t *testing.T) {
	mock := &mockClient{imagePullErr: errors.New("manifest unknown")}
	d := docker.New(mock)
	_, err := d.Start(context.Background(), deployment())
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

func TestDocker_ListManagedContainers_OOMKilled_PopulatesExitDetails(t *testing.T) {
	mock := &mockClient{
		listContainers: []dockertypes.Container{
			{
				ID:     "c1",
				Names:  []string{"/web"},
				State:  "exited",
				Labels: map[string]string{"dirigent.managed": "true", "dirigent.id": "d1"},
			},
		},
		inspectContainer: inspectWithState(137, true, ""),
	}
	d := docker.New(mock)
	containers, err := d.ListManagedContainers(context.Background())
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("want 1 container, got %d", len(containers))
	}
	c := containers[0]
	if c.Running {
		t.Error("want running=false")
	}
	if c.ExitDetails == nil {
		t.Fatal("want ExitDetails populated, got nil")
	}
	if !c.ExitDetails.OOMKilled {
		t.Error("want OOMKilled=true")
	}
	if c.ExitDetails.ExitCode != 137 {
		t.Errorf("want exit code 137, got %d", c.ExitDetails.ExitCode)
	}
}

func TestDocker_ListManagedContainers_InspectFails_ExitDetailsNil(t *testing.T) {
	mock := &mockClient{
		listContainers: []dockertypes.Container{
			{
				ID:     "c1",
				Names:  []string{"/web"},
				State:  "exited",
				Labels: map[string]string{"dirigent.managed": "true", "dirigent.id": "d1"},
			},
		},
		inspectErr: errors.New("inspect failed"),
	}
	d := docker.New(mock)
	containers, err := d.ListManagedContainers(context.Background())
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("want 1 container, got %d", len(containers))
	}
	if containers[0].ExitDetails != nil {
		t.Errorf("want ExitDetails nil on inspect failure, got %+v", containers[0].ExitDetails)
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

func TestDocker_StartAndReplace_OK(t *testing.T) {
	mock := &mockClient{
		containerCreate: container.CreateResponse{ID: "new-c1"},
		// ContainerList returns the new container as running.
		listContainers: []dockertypes.Container{
			{ID: "new-c1", State: "running"},
		},
		inspectContainer: inspectWithPort("3001/tcp", "49123"),
	}
	d := docker.New(mock)

	dep := deployment()
	dep.Ports = []string{"3001"}
	if _, err := d.StartAndReplace(context.Background(), dep, "old-c1"); err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	// Old container must be stopped and removed.
	if len(mock.stopped) != 1 || mock.stopped[0] != "old-c1" {
		t.Errorf("want old-c1 stopped, got %v", mock.stopped)
	}
	if len(mock.removed) != 1 || mock.removed[0] != "old-c1" {
		t.Errorf("want old-c1 removed, got %v", mock.removed)
	}

	// New container must be renamed to the deployment name.
	if len(mock.renamed) != 1 || mock.renamed[0] != "new-c1" {
		t.Errorf("want new-c1 renamed, got %v", mock.renamed)
	}
}

func TestDocker_StartAndReplace_NewContainerNotRunning(t *testing.T) {
	mock := &mockClient{
		containerCreate: container.CreateResponse{ID: "new-c1"},
		// Empty list means the new container is not running.
		listContainers: []dockertypes.Container{},
	}
	d := docker.New(mock)

	dep := deployment()
	_, err := d.StartAndReplace(context.Background(), dep, "old-c1")
	if err == nil {
		t.Fatal("want error when new container is not running, got nil")
	}

	// Old container must NOT have been touched.
	if len(mock.stopped) != 0 {
		t.Errorf("want old container untouched, got stopped: %v", mock.stopped)
	}
	if len(mock.removed) != 1 || mock.removed[0] != "new-c1" {
		t.Errorf("want only new container cleaned up, got removed: %v", mock.removed)
	}
}

func TestDocker_StartAndReplace_SameExplicitPorts_FallsBackToStopThenStart(t *testing.T) {
	mock := &mockClient{
		containerCreate:  container.CreateResponse{ID: "new-c1"},
		inspectContainer: inspectWithPort("80/tcp", "80"),
		// No running-container list required on fallback path.
		listContainers: []dockertypes.Container{},
	}
	d := docker.New(mock)

	dep := deployment()
	dep.Ports = []string{"80:80"}

	ports, err := d.StartAndReplace(context.Background(), dep, "old-c1")
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	if len(ports) != 1 || ports[0] != "80:80" {
		t.Fatalf("want runtime ports [80:80], got %v", ports)
	}

	if len(mock.stopped) != 1 || mock.stopped[0] != "old-c1" {
		t.Fatalf("want old-c1 stopped first, got %v", mock.stopped)
	}
	if len(mock.removed) != 1 || mock.removed[0] != "old-c1" {
		t.Fatalf("want old-c1 removed first, got %v", mock.removed)
	}
}

// Ensure the mock satisfies the docker.Client interface at compile time.
var _ interface {
	ContainerList(context.Context, container.ListOptions) ([]dockertypes.Container, error)
	Ping(context.Context) (dockertypes.Ping, error)
} = (*mockClient)(nil)

// Ensure filters package is referenced (avoids unused import if linter checks).
var _ = filters.NewArgs

func inspectWithState(exitCode int, oomKilled bool, errMsg string) dockertypes.ContainerJSON {
	return dockertypes.ContainerJSON{
		ContainerJSONBase: &dockertypes.ContainerJSONBase{
			State: &dockertypes.ContainerState{
				ExitCode:  exitCode,
				OOMKilled: oomKilled,
				Error:     errMsg,
			},
		},
	}
}

func inspectWithPort(containerPort, hostPort string) dockertypes.ContainerJSON {
	return dockertypes.ContainerJSON{
		NetworkSettings: &dockertypes.NetworkSettings{
			NetworkSettingsBase: dockertypes.NetworkSettingsBase{
				Ports: nat.PortMap{
					nat.Port(containerPort): []nat.PortBinding{{HostPort: hostPort}},
				},
			},
		},
	}
}
