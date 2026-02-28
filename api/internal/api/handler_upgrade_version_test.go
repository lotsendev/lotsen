package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/internal/events"
	"github.com/ercadev/dirigent/internal/upgrade"
	"github.com/ercadev/dirigent/internal/version"
)

type versionProviderStub struct {
	snapshot version.Snapshot
	err      error
}

func (s *versionProviderStub) Snapshot(_ context.Context) (version.Snapshot, error) {
	if s.err != nil {
		return version.Snapshot{}, s.err
	}
	return s.snapshot, nil
}

type versionProviderWithRefreshStub struct {
	snapshot        version.Snapshot
	refreshSnapshot version.Snapshot
	snapshotCalls   int
	refreshCalls    int
}

func (s *versionProviderWithRefreshStub) Snapshot(_ context.Context) (version.Snapshot, error) {
	s.snapshotCalls++
	return s.snapshot, nil
}

func (s *versionProviderWithRefreshStub) RefreshSnapshot(_ context.Context) (version.Snapshot, error) {
	s.refreshCalls++
	return s.refreshSnapshot, nil
}

type upgradeRunnerStub struct {
	startErr error
	lines    chan string
}

func (s *upgradeRunnerStub) Start(_ string) error {
	return s.startErr
}

func (s *upgradeRunnerStub) Subscribe() (<-chan string, func(), error) {
	if s.lines == nil {
		return nil, nil, upgrade.ErrNotRunning
	}
	unsubscribe := func() {
	}
	return s.lines, unsubscribe, nil
}

func (s *upgradeRunnerStub) IsRunning() bool {
	return s.lines != nil
}

func newTestServerWithUpgradeAndVersion(s api.Store, versions api.VersionInfoProvider, upgrader api.UpgradeRunner) *httptest.Server {
	mux := http.NewServeMux()
	h := api.NewWithDependencies(s, events.NewBroker(), noopDockerLogs{}, nil, versions, upgrader)
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func TestGetVersion_ReturnsExpectedJSON(t *testing.T) {
	now := time.Date(2026, time.February, 24, 10, 0, 0, 0, time.UTC)
	publishedAt := time.Date(2026, time.February, 23, 12, 34, 56, 0, time.UTC)

	srv := newTestServerWithUpgradeAndVersion(
		newMemStore(),
		&versionProviderStub{snapshot: version.Snapshot{
			CurrentVersion:   "v1.0.0",
			LatestVersion:    "v1.1.0",
			ReleaseNotes:     "notes",
			PublishedAt:      publishedAt,
			UpgradeAvailable: true,
			CachedAt:         now,
		}},
		&upgradeRunnerStub{},
	)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/version")
	if err != nil {
		t.Fatalf("GET /api/version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body struct {
		CurrentVersion   string    `json:"currentVersion"`
		LatestVersion    string    `json:"latestVersion"`
		ReleaseNotes     string    `json:"releaseNotes"`
		PublishedAt      time.Time `json:"publishedAt"`
		UpgradeAvailable bool      `json:"upgradeAvailable"`
		CachedAt         time.Time `json:"cachedAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.CurrentVersion != "v1.0.0" {
		t.Fatalf("currentVersion = %q, want v1.0.0", body.CurrentVersion)
	}
	if body.LatestVersion != "v1.1.0" {
		t.Fatalf("latestVersion = %q, want v1.1.0", body.LatestVersion)
	}
	if body.ReleaseNotes != "notes" {
		t.Fatalf("releaseNotes = %q, want notes", body.ReleaseNotes)
	}
	if !body.PublishedAt.Equal(publishedAt) {
		t.Fatalf("publishedAt = %s, want %s", body.PublishedAt, publishedAt)
	}
	if !body.UpgradeAvailable {
		t.Fatal("upgradeAvailable = false, want true")
	}
	if !body.CachedAt.Equal(now) {
		t.Fatalf("cachedAt = %s, want %s", body.CachedAt, now)
	}
}

func TestGetVersion_RefreshQueryUsesFreshSnapshot(t *testing.T) {
	provider := &versionProviderWithRefreshStub{
		snapshot:        version.Snapshot{CurrentVersion: "v1.0.0", LatestVersion: "v1.1.0"},
		refreshSnapshot: version.Snapshot{CurrentVersion: "v1.0.0", LatestVersion: "v1.2.0", UpgradeAvailable: true},
	}

	srv := newTestServerWithUpgradeAndVersion(newMemStore(), provider, &upgradeRunnerStub{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/version?refresh=1")
	if err != nil {
		t.Fatalf("GET /api/version?refresh=1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body struct {
		LatestVersion string `json:"latestVersion"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.LatestVersion != "v1.2.0" {
		t.Fatalf("latestVersion = %q, want v1.2.0", body.LatestVersion)
	}
	if provider.refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", provider.refreshCalls)
	}
}

func TestPostUpgrade_Returns202WhenIdle(t *testing.T) {
	srv := newTestServerWithUpgradeAndVersion(
		newMemStore(),
		&versionProviderStub{snapshot: version.Snapshot{LatestVersion: "v1.1.0"}},
		&upgradeRunnerStub{},
	)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/upgrade", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/upgrade: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", resp.StatusCode)
	}
}

func TestPostUpgrade_Returns409WhenAlreadyRunning(t *testing.T) {
	srv := newTestServerWithUpgradeAndVersion(
		newMemStore(),
		&versionProviderStub{snapshot: version.Snapshot{LatestVersion: "v1.1.0"}},
		&upgradeRunnerStub{startErr: upgrade.ErrAlreadyRunning},
	)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/upgrade", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/upgrade: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", resp.StatusCode)
	}
}

func TestUpgradeLogs_Returns404WhenNotRunning(t *testing.T) {
	srv := newTestServerWithUpgradeAndVersion(
		newMemStore(),
		&versionProviderStub{},
		&upgradeRunnerStub{startErr: errors.New("unused")},
	)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/upgrade/logs")
	if err != nil {
		t.Fatalf("GET /api/upgrade/logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestUpgradeLogs_StreamsAndCloses(t *testing.T) {
	lines := make(chan string, 2)
	lines <- "line one"
	lines <- "line two"
	close(lines)

	srv := newTestServerWithUpgradeAndVersion(
		newMemStore(),
		&versionProviderStub{},
		&upgradeRunnerStub{lines: lines},
	)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/upgrade/logs")
	if err != nil {
		t.Fatalf("GET /api/upgrade/logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	readLines := make([]string, 0, 2)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			readLines = append(readLines, line)
		}
	}

	if len(readLines) != 2 {
		t.Fatalf("want 2 SSE lines, got %d (%v)", len(readLines), readLines)
	}
}
