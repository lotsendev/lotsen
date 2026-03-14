package docker_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/lotsendev/lotsen/orchestrator/internal/docker"
	"github.com/lotsendev/lotsen/store"
)

type mockClient struct {
	pingErr          error
	imagePullErr     error
	imagePulled      bool
	imageList        []image.Summary
	imageListErr     error
	containerCreate  container.CreateResponse
	createErr        error
	startErr         error
	listContainers   []dockertypes.Container
	listErr          error
	statsByContainer map[string]container.StatsResponseReader
	statsErr         error
	stopErr          error
	removeErr        error
	renameErr        error
	inspectContainer dockertypes.ContainerJSON
	inspectErr       error
	createdHostCfg   *container.HostConfig
	stopped          []string
	removed          []string
	renamed          []string
	onStop           func(id string)
	onList           func()
}

func (m *mockClient) Ping(_ context.Context) (dockertypes.Ping, error) {
	return dockertypes.Ping{}, m.pingErr
}

func (m *mockClient) ImagePull(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
	if m.imagePullErr != nil {
		return nil, m.imagePullErr
	}
	m.imagePulled = true
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockClient) ImageList(_ context.Context, _ image.ListOptions) ([]image.Summary, error) {
	return m.imageList, m.imageListErr
}

func (m *mockClient) ContainerCreate(_ context.Context, _ *container.Config, hostCfg *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, _ string) (container.CreateResponse, error) {
	m.createdHostCfg = hostCfg
	return m.containerCreate, m.createErr
}

func (m *mockClient) ContainerStart(_ context.Context, _ string, _ container.StartOptions) error {
	return m.startErr
}

func (m *mockClient) ContainerList(_ context.Context, _ container.ListOptions) ([]dockertypes.Container, error) {
	if m.onList != nil {
		m.onList()
	}
	return m.listContainers, m.listErr
}

func (m *mockClient) ContainerStats(_ context.Context, containerID string, _ bool) (container.StatsResponseReader, error) {
	if m.statsErr != nil {
		return container.StatsResponseReader{}, m.statsErr
	}
	if m.statsByContainer == nil {
		return container.StatsResponseReader{}, errors.New("missing stats")
	}
	stats, ok := m.statsByContainer[containerID]
	if !ok {
		return container.StatsResponseReader{}, errors.New("missing container stats")
	}
	return stats, nil
}

func (m *mockClient) ContainerStop(_ context.Context, id string, _ container.StopOptions) error {
	m.stopped = append(m.stopped, id)
	if m.onStop != nil {
		m.onStop(id)
	}
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

func TestDocker_Start_IncludesManagedFileMounts(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOTSEN_MANAGED_FILES_DIR", base)

	mock := &mockClient{
		containerCreate:  container.CreateResponse{ID: "c1"},
		inspectContainer: inspectWithPort("80/tcp", "80"),
	}
	d := docker.New(mock)

	dep := deployment()
	dep.FileMounts = []store.FileMount{
		{Source: "prometheus.yml", Target: "/etc/prometheus/prometheus.yml", Content: "global:\n", ReadOnly: true},
	}

	if _, err := d.Start(context.Background(), dep); err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	if mock.createdHostCfg == nil {
		t.Fatal("want ContainerCreate called with host config")
	}

	want := filepath.Join(base, dep.ID, "prometheus.yml") + ":/etc/prometheus/prometheus.yml:ro"
	found := false
	for _, bind := range mock.createdHostCfg.Binds {
		if bind == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want bind %q in %v", want, mock.createdHostCfg.Binds)
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
				Labels: map[string]string{"lotsen.managed": "true", "lotsen.id": "d1"},
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
				Labels: map[string]string{"lotsen.managed": "true", "lotsen.id": "d1"},
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

func TestDocker_CollectStats_CollectsRunningManagedContainers(t *testing.T) {
	statsForC1 := newStatsReader(t, map[string]any{
		"cpu_stats": map[string]any{
			"cpu_usage":        map[string]any{"total_usage": uint64(4_500_000_000)},
			"system_cpu_usage": uint64(10_000_000_000),
			"online_cpus":      uint32(2),
		},
		"precpu_stats": map[string]any{
			"cpu_usage":        map[string]any{"total_usage": uint64(4_000_000_000)},
			"system_cpu_usage": uint64(9_000_000_000),
		},
		"memory_stats": map[string]any{
			"usage": uint64(600 * 1024 * 1024),
			"limit": uint64(1024 * 1024 * 1024),
			"stats": map[string]any{"inactive_file": uint64(100 * 1024 * 1024)},
		},
	})

	mock := &mockClient{
		listContainers: []dockertypes.Container{
			{ID: "c1", Labels: map[string]string{"lotsen.id": "d1"}, State: "running"},
		},
		statsByContainer: map[string]container.StatsResponseReader{"c1": statsForC1},
	}

	d := docker.New(mock)
	stats, err := d.CollectStats(context.Background())
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	collected, ok := stats["d1"]
	if !ok {
		t.Fatal("want stats for deployment d1")
	}
	if collected.CPUPercent <= 0 {
		t.Fatalf("want positive cpu percent, got %v", collected.CPUPercent)
	}
	if collected.MemoryUsedBytes != uint64(500*1024*1024) {
		t.Fatalf("want memory used 524288000, got %d", collected.MemoryUsedBytes)
	}
	if collected.MemoryLimitBytes != uint64(1024*1024*1024) {
		t.Fatalf("want memory limit 1073741824, got %d", collected.MemoryLimitBytes)
	}
	if collected.MemoryPercent <= 0 {
		t.Fatalf("want positive memory percent, got %v", collected.MemoryPercent)
	}
}

func TestDocker_CollectStats_SkipsContainerOnStatsFailure(t *testing.T) {
	mock := &mockClient{
		listContainers: []dockertypes.Container{
			{ID: "c1", Labels: map[string]string{"lotsen.id": "d1"}, State: "running"},
		},
		statsErr: errors.New("stats unavailable"),
	}

	d := docker.New(mock)
	stats, err := d.CollectStats(context.Background())
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("want no stats, got %d entries", len(stats))
	}
}

func TestDocker_ListManagedContainers_InspectFails_ExitDetailsNil(t *testing.T) {
	mock := &mockClient{
		listContainers: []dockertypes.Container{
			{
				ID:     "c1",
				Names:  []string{"/web"},
				State:  "exited",
				Labels: map[string]string{"lotsen.managed": "true", "lotsen.id": "d1"},
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

func TestDocker_StopAndRemove_AlreadyStoppedStillRemoves(t *testing.T) {
	mock := &mockClient{stopErr: errdefs.NotModified(errors.New("container already stopped"))}
	d := docker.New(mock)
	if err := d.StopAndRemove(context.Background(), "c1"); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(mock.stopped) != 1 || mock.stopped[0] != "c1" {
		t.Errorf("want c1 stop attempted, got %v", mock.stopped)
	}
	if len(mock.removed) != 1 || mock.removed[0] != "c1" {
		t.Errorf("want c1 removed, got %v", mock.removed)
	}
}

func TestDocker_StopAndRemove_MissingContainerIgnored(t *testing.T) {
	mock := &mockClient{removeErr: errdefs.NotFound(errors.New("no such container"))}
	d := docker.New(mock)
	if err := d.StopAndRemove(context.Background(), "c1"); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(mock.stopped) != 1 || mock.stopped[0] != "c1" {
		t.Errorf("want c1 stop attempted, got %v", mock.stopped)
	}
	if len(mock.removed) != 1 || mock.removed[0] != "c1" {
		t.Errorf("want c1 remove attempted, got %v", mock.removed)
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

func TestDocker_StartAndReplace_OverlappingExplicitHostPort_FallsBackToStopThenStart(t *testing.T) {
	mock := &mockClient{
		containerCreate:  container.CreateResponse{ID: "new-c1"},
		inspectContainer: inspectWithPort("8080/tcp", "32770"),
		listContainers:   []dockertypes.Container{},
	}
	d := docker.New(mock)

	dep := deployment()
	dep.Ports = []string{"32770:8080"}

	ports, err := d.StartAndReplace(context.Background(), dep, "old-c1")
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	if len(ports) != 1 || ports[0] != "32770:8080" {
		t.Fatalf("want runtime ports [32770:8080], got %v", ports)
	}

	if len(mock.stopped) != 1 || mock.stopped[0] != "old-c1" {
		t.Fatalf("want old-c1 stopped first, got %v", mock.stopped)
	}
	if len(mock.removed) != 1 || mock.removed[0] != "old-c1" {
		t.Fatalf("want old-c1 removed first, got %v", mock.removed)
	}
}

func TestDocker_StartAndReplace_DifferentProtocolSameHostPort_DoesNotFallback(t *testing.T) {
	var listedBeforeStop atomic.Bool

	mock := &mockClient{
		containerCreate:  container.CreateResponse{ID: "new-c1"},
		inspectContainer: inspectWithPort("53/udp", "53"),
		listContainers:   []dockertypes.Container{{ID: "new-c1", State: "running"}},
		onList: func() {
			listedBeforeStop.Store(true)
		},
		onStop: func(_ string) {
			if !listedBeforeStop.Load() {
				t.Fatal("old container was stopped before new container readiness check")
			}
		},
	}
	d := docker.New(mock)

	dep := deployment()
	dep.Ports = []string{"53:53/tcp"}

	ports, err := d.StartAndReplace(context.Background(), dep, "old-c1")
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	if len(ports) != 1 || ports[0] != "53:53" {
		t.Fatalf("want runtime ports [53:53], got %v", ports)
	}
}

func TestDocker_StartAndReplace_DomainConfigured_SwapsProxyBeforeStoppingOld(t *testing.T) {
	var stopCalled atomic.Bool
	var swapCalled atomic.Bool

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		swapCalled.Store(true)
		if stopCalled.Load() {
			t.Fatal("proxy swap happened after stopping old container")
		}
		if r.Method != http.MethodPost || r.URL.Path != "/internal/routes" {
			t.Fatalf("want POST /internal/routes, got %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		r.Body.Close()
		if string(body) != `{"domain":"example.com","upstream":"localhost:49123"}` {
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()

	t.Setenv("LOTSEN_PROXY_URL", proxy.URL)

	mock := &mockClient{
		containerCreate:  container.CreateResponse{ID: "new-c1"},
		listContainers:   []dockertypes.Container{{ID: "new-c1", State: "running"}},
		inspectContainer: inspectWithPort("3001/tcp", "49123"),
		onStop: func(_ string) {
			stopCalled.Store(true)
		},
	}
	d := docker.New(mock)

	dep := deployment()
	dep.Ports = []string{"3001"}
	dep.Domain = "Example.com."

	if _, err := d.StartAndReplace(context.Background(), dep, "old-c1"); err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	if !swapCalled.Load() {
		t.Fatal("want proxy route swap to be called")
	}
	if len(mock.stopped) != 1 || mock.stopped[0] != "old-c1" {
		t.Fatalf("want old-c1 stopped, got %v", mock.stopped)
	}
}

func TestDocker_StartAndReplace_DomainConfigured_UsesSelectedProxyPort(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/routes" {
			t.Fatalf("want POST /internal/routes, got %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		r.Body.Close()
		if string(body) != `{"domain":"example.com","upstream":"localhost:49123"}` {
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()

	t.Setenv("LOTSEN_PROXY_URL", proxy.URL)

	mock := &mockClient{
		containerCreate: container.CreateResponse{ID: "new-c1"},
		listContainers:  []dockertypes.Container{{ID: "new-c1", State: "running"}},
		inspectContainer: inspectWithPorts(map[string]string{
			"53/udp":  "53",
			"80/tcp":  "49123",
			"443/tcp": "49443",
		}),
	}
	d := docker.New(mock)

	dep := deployment()
	dep.Ports = []string{"53:53/udp", "80", "443"}
	dep.Domain = "example.com"
	dep.ProxyPort = 80

	if _, err := d.StartAndReplace(context.Background(), dep, "old-c1"); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestDocker_StartAndReplace_ProxySwapFails_CleansUpNewAndKeepsOld(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer proxy.Close()

	t.Setenv("LOTSEN_PROXY_URL", proxy.URL)

	mock := &mockClient{
		containerCreate:  container.CreateResponse{ID: "new-c1"},
		listContainers:   []dockertypes.Container{{ID: "new-c1", State: "running"}},
		inspectContainer: inspectWithPort("3001/tcp", "49123"),
	}
	d := docker.New(mock)

	dep := deployment()
	dep.Ports = []string{"3001"}
	dep.Domain = "example.com"

	_, err := d.StartAndReplace(context.Background(), dep, "old-c1")
	if err == nil {
		t.Fatal("want error when proxy swap fails, got nil")
	}
	if !strings.Contains(err.Error(), "swap proxy route") {
		t.Fatalf("want proxy swap error, got %v", err)
	}

	if len(mock.stopped) != 1 || mock.stopped[0] != "new-c1" {
		t.Fatalf("want only new-c1 stopped for cleanup, got %v", mock.stopped)
	}
	if len(mock.removed) != 1 || mock.removed[0] != "new-c1" {
		t.Fatalf("want only new-c1 removed for cleanup, got %v", mock.removed)
	}
	if len(mock.renamed) != 0 {
		t.Fatalf("want no rename on failure, got %v", mock.renamed)
	}
}

func TestDocker_Start_PinnedImageExistsLocally_SkipsPull(t *testing.T) {
	mock := &mockClient{
		imageList:        []image.Summary{{ID: "sha256:abc"}},
		containerCreate:  container.CreateResponse{ID: "c1"},
		inspectContainer: inspectWithPort("80/tcp", "80"),
	}
	d := docker.New(mock)

	dep := deployment()
	dep.Image = "nginx:1.25.3" // pinned tag — should not be re-pulled
	if _, err := d.Start(context.Background(), dep); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if mock.imagePulled {
		t.Error("want pull skipped for pinned image that exists locally, but ImagePull was called")
	}
}

func TestDocker_Start_LatestTagAlwaysPulls(t *testing.T) {
	mock := &mockClient{
		imageList:        []image.Summary{{ID: "sha256:abc"}}, // image exists locally
		containerCreate:  container.CreateResponse{ID: "c1"},
		inspectContainer: inspectWithPort("80/tcp", "80"),
	}
	d := docker.New(mock)

	dep := deployment() // image is "nginx:latest"
	if _, err := d.Start(context.Background(), dep); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if !mock.imagePulled {
		t.Error("want pull for :latest tag even when image exists locally, but ImagePull was not called")
	}
}

func TestDocker_Start_PinnedImageNotLocal_Pulls(t *testing.T) {
	mock := &mockClient{
		imageList:        []image.Summary{}, // image not present locally
		containerCreate:  container.CreateResponse{ID: "c1"},
		inspectContainer: inspectWithPort("80/tcp", "80"),
	}
	d := docker.New(mock)

	dep := deployment()
	dep.Image = "nginx:1.25.3"
	if _, err := d.Start(context.Background(), dep); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if !mock.imagePulled {
		t.Error("want pull when pinned image is not cached locally, but ImagePull was not called")
	}
}

// Ensure the mock satisfies the docker.Client interface at compile time.
var _ interface {
	ContainerList(context.Context, container.ListOptions) ([]dockertypes.Container, error)
	ContainerStats(context.Context, string, bool) (container.StatsResponseReader, error)
	ImageList(context.Context, image.ListOptions) ([]image.Summary, error)
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
	return inspectWithPorts(map[string]string{containerPort: hostPort})
}

func inspectWithPorts(bindings map[string]string) dockertypes.ContainerJSON {
	ports := make(nat.PortMap, len(bindings))
	for containerPort, hostPort := range bindings {
		ports[nat.Port(containerPort)] = []nat.PortBinding{{HostPort: hostPort}}
	}
	return dockertypes.ContainerJSON{
		NetworkSettings: &dockertypes.NetworkSettings{
			NetworkSettingsBase: dockertypes.NetworkSettingsBase{
				Ports: ports,
			},
		},
	}
}

func newStatsReader(t *testing.T, payload map[string]any) container.StatsResponseReader {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal stats payload: %v", err)
	}

	return container.StatsResponseReader{Body: io.NopCloser(strings.NewReader(string(body)))}
}
