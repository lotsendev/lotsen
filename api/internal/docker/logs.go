package docker

import (
	"context"
	"fmt"
	"io"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/stdcopy"
)

// Client is the Docker API surface required by the log streamer.
type Client interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]dockertypes.Container, error)
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
}

// LogStreamer streams container logs for a deployment on demand.
type LogStreamer struct {
	client Client
}

// New creates a LogStreamer backed by the given Docker client.
func New(client Client) *LogStreamer {
	return &LogStreamer{client: client}
}

// StreamLogs returns a reader that emits demultiplexed log lines (stdout and
// stderr combined) for the running container associated with deploymentID,
// starting from the last tail lines. The caller must close the reader when
// done; closing it also terminates the underlying Docker log stream.
//
// Returns (nil, nil) when no container is currently running for the deployment.
func (s *LogStreamer) StreamLogs(ctx context.Context, deploymentID string, tail int) (io.ReadCloser, error) {
	f := filters.NewArgs(
		filters.Arg("label", "dirigent.managed=true"),
		filters.Arg("label", "dirigent.id="+deploymentID),
	)
	containers, err := s.client.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		return nil, fmt.Errorf("docker: list containers for deployment %s: %w", deploymentID, err)
	}
	if len(containers) == 0 {
		return nil, nil
	}

	raw, err := s.client.ContainerLogs(ctx, containers[0].ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       fmt.Sprintf("%d", tail),
	})
	if err != nil {
		return nil, fmt.Errorf("docker: logs for container %s: %w", containers[0].ID, err)
	}

	// Docker logs are multiplexed (8-byte header per frame). Demultiplex
	// stdout and stderr into a single pipe so the caller gets plain lines.
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer raw.Close()
		stdcopy.StdCopy(pw, pw, raw) //nolint:errcheck
	}()
	return pr, nil
}
