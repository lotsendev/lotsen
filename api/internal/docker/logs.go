package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/stdcopy"
)

// Client is the Docker API surface required by the log streamer.
type Client interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
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
// stderr combined) for the newest container associated with deploymentID,
// starting from the last tail lines. The caller must close the reader when
// done; closing it also terminates the underlying Docker log stream.
//
// Returns (nil, nil) when no managed container exists for the deployment.
func (s *LogStreamer) StreamLogs(ctx context.Context, deploymentID string, tail int) (io.ReadCloser, error) {
	latest, err := s.latestContainer(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, nil
	}

	raw, err := s.client.ContainerLogs(ctx, latest.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       fmt.Sprintf("%d", tail),
	})
	if err != nil {
		return nil, fmt.Errorf("docker: logs for container %s: %w", latest.ID, err)
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

// RecentLogs returns the latest log lines for the newest container associated
// with deploymentID without holding an active follow stream.
func (s *LogStreamer) RecentLogs(ctx context.Context, deploymentID string, tail int) ([]string, error) {
	latest, err := s.latestContainer(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return []string{}, nil
	}

	raw, err := s.client.ContainerLogs(ctx, latest.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Tail:       fmt.Sprintf("%d", tail),
	})
	if err != nil {
		return nil, fmt.Errorf("docker: logs for container %s: %w", latest.ID, err)
	}
	defer raw.Close()

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		stdcopy.StdCopy(pw, pw, raw) //nolint:errcheck
	}()

	scanner := bufio.NewScanner(pr)
	lines := make([]string, 0, tail)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("docker: scan logs for deployment %s: %w", deploymentID, err)
	}

	return lines, nil
}

func (s *LogStreamer) latestContainer(ctx context.Context, deploymentID string) (*container.Summary, error) {
	f := filters.NewArgs(
		filters.Arg("label", "lotsen.managed=true"),
		filters.Arg("label", "lotsen.id="+deploymentID),
	)
	containers, err := s.client.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("docker: list containers for deployment %s: %w", deploymentID, err)
	}
	if len(containers) == 0 {
		return nil, nil
	}

	latest := containers[0]
	for _, c := range containers[1:] {
		if c.Created > latest.Created {
			latest = c
		}
	}

	return &latest, nil
}
