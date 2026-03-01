package docker

import (
	"context"
	"io"
	"strings"
	"testing"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

type mockClient struct {
	containers []dockertypes.Container
	listErr    error
	logsErr    error

	listOptions   container.ListOptions
	logsContainer string
	logsOptions   container.LogsOptions
}

func (m *mockClient) ContainerList(_ context.Context, options container.ListOptions) ([]dockertypes.Container, error) {
	m.listOptions = options
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.containers, nil
}

func (m *mockClient) ContainerLogs(_ context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	m.logsContainer = containerID
	m.logsOptions = options
	if m.logsErr != nil {
		return nil, m.logsErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func TestStreamLogs_UsesNewestContainerIncludingStopped(t *testing.T) {
	m := &mockClient{
		containers: []dockertypes.Container{
			{ID: "old", Created: 100},
			{ID: "new", Created: 200},
		},
	}

	streamer := New(m)
	rc, err := streamer.StreamLogs(context.Background(), "dep-1", 100)
	if err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	if rc == nil {
		t.Fatal("want non-nil log stream")
	}
	rc.Close()

	if !m.listOptions.All {
		t.Fatal("want container list to include stopped containers")
	}
	if m.logsContainer != "new" {
		t.Fatalf("want logs from newest container 'new', got %q", m.logsContainer)
	}
	if !m.logsOptions.Follow {
		t.Fatal("want Follow=true")
	}
	if m.logsOptions.Tail != "100" {
		t.Fatalf("want Tail=100, got %q", m.logsOptions.Tail)
	}
}

func TestStreamLogs_NoContainers(t *testing.T) {
	m := &mockClient{}
	streamer := New(m)

	rc, err := streamer.StreamLogs(context.Background(), "dep-1", 100)
	if err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	if rc != nil {
		t.Fatal("want nil stream when deployment has no containers")
	}
	if m.logsContainer != "" {
		t.Fatal("did not expect ContainerLogs call")
	}
}

func TestRecentLogs_UsesNewestContainerWithoutFollow(t *testing.T) {
	m := &mockClient{
		containers: []dockertypes.Container{
			{ID: "old", Created: 100},
			{ID: "new", Created: 200},
		},
	}

	streamer := New(m)
	lines, err := streamer.RecentLogs(context.Background(), "dep-1", 150)
	if err != nil {
		t.Fatalf("RecentLogs: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("want no lines from empty mock output, got %d", len(lines))
	}

	if m.logsContainer != "new" {
		t.Fatalf("want logs from newest container 'new', got %q", m.logsContainer)
	}
	if m.logsOptions.Follow {
		t.Fatal("want Follow=false")
	}
	if m.logsOptions.Tail != "150" {
		t.Fatalf("want Tail=150, got %q", m.logsOptions.Tail)
	}
}
