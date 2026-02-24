package version

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSnapshot_UpgradeAvailableComparison(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		latestVersion  string
		wantUpgrade    bool
	}{
		{name: "current lower than latest", currentVersion: "v1.2.2", latestVersion: "v1.2.3", wantUpgrade: true},
		{name: "current equals latest", currentVersion: "v1.2.3", latestVersion: "v1.2.3", wantUpgrade: false},
		{name: "dev sentinel", currentVersion: "dev", latestVersion: "v1.2.3", wantUpgrade: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, time.February, 24, 10, 0, 0, 0, time.UTC)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w, `{"tag_name":"%s","body":"notes","published_at":"2026-02-24T09:00:00Z"}`+"\n", tt.latestVersion)
			}))
			defer server.Close()

			oldURL := latestReleaseURL
			latestReleaseURL = server.URL
			defer func() { latestReleaseURL = oldURL }()

			svc := NewWithOptions(tt.currentVersion, server.Client(), func() time.Time { return now }, time.Hour)

			snapshot, err := svc.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("Snapshot() error = %v", err)
			}
			if snapshot.UpgradeAvailable != tt.wantUpgrade {
				t.Fatalf("UpgradeAvailable = %v, want %v", snapshot.UpgradeAvailable, tt.wantUpgrade)
			}
		})
	}
}

func TestSnapshot_ParsesGitHubResponse(t *testing.T) {
	now := time.Date(2026, time.February, 24, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"tag_name":"v1.4.0","body":"release notes","published_at":"2026-02-23T12:34:56Z"}`)
	}))
	defer server.Close()

	oldURL := latestReleaseURL
	latestReleaseURL = server.URL
	defer func() { latestReleaseURL = oldURL }()

	svc := NewWithOptions("v1.3.0", server.Client(), func() time.Time { return now }, time.Hour)

	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if snapshot.CurrentVersion != "v1.3.0" {
		t.Fatalf("CurrentVersion = %q, want v1.3.0", snapshot.CurrentVersion)
	}
	if snapshot.LatestVersion != "v1.4.0" {
		t.Fatalf("LatestVersion = %q, want v1.4.0", snapshot.LatestVersion)
	}
	if snapshot.ReleaseNotes != "release notes" {
		t.Fatalf("ReleaseNotes = %q, want release notes", snapshot.ReleaseNotes)
	}
	if !snapshot.PublishedAt.Equal(time.Date(2026, time.February, 23, 12, 34, 56, 0, time.UTC)) {
		t.Fatalf("PublishedAt = %s, want 2026-02-23T12:34:56Z", snapshot.PublishedAt)
	}
	if !snapshot.CachedAt.Equal(now) {
		t.Fatalf("CachedAt = %s, want %s", snapshot.CachedAt, now)
	}
}

func TestSnapshot_UsesCacheWithinTTL(t *testing.T) {
	base := time.Date(2026, time.February, 24, 10, 0, 0, 0, time.UTC)
	currentTime := base

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = fmt.Fprint(w, `{"tag_name":"v1.0.1","body":"notes","published_at":"2026-02-24T08:00:00Z"}`)
	}))
	defer server.Close()

	oldURL := latestReleaseURL
	latestReleaseURL = server.URL
	defer func() { latestReleaseURL = oldURL }()

	svc := NewWithOptions("v1.0.0", server.Client(), func() time.Time { return currentTime }, time.Hour)

	if _, err := svc.Snapshot(context.Background()); err != nil {
		t.Fatalf("first Snapshot() error = %v", err)
	}
	currentTime = currentTime.Add(30 * time.Minute)
	if _, err := svc.Snapshot(context.Background()); err != nil {
		t.Fatalf("second Snapshot() error = %v", err)
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("GitHub API calls = %d, want 1", got)
	}
}

func TestSnapshot_ReportsInstalledVersionForBranchBuilds(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		latestVersion  string
		wantCurrent    string
		wantUpgrade    bool
	}{
		{name: "main remains installed version", currentVersion: "main", latestVersion: "v1.2.3", wantCurrent: "main", wantUpgrade: false},
		{name: "master remains installed version", currentVersion: "master", latestVersion: "v1.2.3", wantCurrent: "master", wantUpgrade: false},
		{name: "main keeps branch name when latest is not semver", currentVersion: "main", latestVersion: "release-candidate", wantCurrent: "main", wantUpgrade: false},
		{name: "dev keeps sentinel", currentVersion: "dev", latestVersion: "v1.2.3", wantCurrent: "dev", wantUpgrade: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, time.February, 24, 10, 0, 0, 0, time.UTC)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w, `{"tag_name":"%s","body":"notes","published_at":"2026-02-24T09:00:00Z"}`+"\n", tt.latestVersion)
			}))
			defer server.Close()

			oldURL := latestReleaseURL
			latestReleaseURL = server.URL
			defer func() { latestReleaseURL = oldURL }()

			svc := NewWithOptions(tt.currentVersion, server.Client(), func() time.Time { return now }, time.Hour)

			snapshot, err := svc.Snapshot(context.Background())
			if err != nil {
				t.Fatalf("Snapshot() error = %v", err)
			}

			if snapshot.CurrentVersion != tt.wantCurrent {
				t.Fatalf("CurrentVersion = %q, want %q", snapshot.CurrentVersion, tt.wantCurrent)
			}
			if snapshot.UpgradeAvailable != tt.wantUpgrade {
				t.Fatalf("UpgradeAvailable = %v, want %v", snapshot.UpgradeAvailable, tt.wantUpgrade)
			}
		})
	}
}
