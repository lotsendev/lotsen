package main

import (
	"errors"
	"testing"
)

func TestDetermineUpgradeVersions_WithLatestAndSnapshot(t *testing.T) {
	lookup := func() (versionSnapshot, error) {
		return versionSnapshot{CurrentVersion: "v0.1.0", LatestVersion: "v0.2.0"}, nil
	}

	from, to := determineUpgradeVersions("latest", lookup)
	if from != "v0.1.0" {
		t.Fatalf("from = %q, want %q", from, "v0.1.0")
	}
	if to != "v0.2.0" {
		t.Fatalf("to = %q, want %q", to, "v0.2.0")
	}
}

func TestDetermineUpgradeVersions_WithPinnedTarget(t *testing.T) {
	lookup := func() (versionSnapshot, error) {
		return versionSnapshot{CurrentVersion: "v0.1.0", LatestVersion: "v0.2.0"}, nil
	}

	from, to := determineUpgradeVersions("v0.1.5", lookup)
	if from != "v0.1.0" {
		t.Fatalf("from = %q, want %q", from, "v0.1.0")
	}
	if to != "v0.1.5" {
		t.Fatalf("to = %q, want %q", to, "v0.1.5")
	}
}

func TestDetermineUpgradeVersions_OnLookupFailure(t *testing.T) {
	lookup := func() (versionSnapshot, error) {
		return versionSnapshot{}, errors.New("boom")
	}

	from, to := determineUpgradeVersions("latest", lookup)
	if from != "unknown" {
		t.Fatalf("from = %q, want %q", from, "unknown")
	}
	if to != "latest" {
		t.Fatalf("to = %q, want %q", to, "latest")
	}
}
