package main

import (
	"crypto/sha256"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestSha256File_ReturnsHashForKnownContent(t *testing.T) {
	content := []byte("hello lotsen")
	tmp, err := os.CreateTemp("", "sha256test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	got, err := sha256File(tmp.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sum := sha256.Sum256(content)
	want := make([]byte, len(sum)*2)
	const hextable = "0123456789abcdef"
	for i, b := range sum {
		want[i*2] = hextable[b>>4]
		want[i*2+1] = hextable[b&0x0f]
	}
	if got != string(want) {
		t.Fatalf("sha256File = %q, want %q", got, string(want))
	}
}

func TestSha256File_ErrorsOnMissingFile(t *testing.T) {
	_, err := sha256File("/nonexistent/path/to/binary")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseChecksumEntry_FindsMatchingBinary(t *testing.T) {
	input := "abc123  lotsen-proxy\n"
	hash, err := parseChecksumEntry(strings.NewReader(input), "lotsen-proxy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash != "abc123" {
		t.Fatalf("hash = %q, want %q", hash, "abc123")
	}
}

func TestParseChecksumEntry_ReturnsEmptyWhenNotFound(t *testing.T) {
	input := "abc123  lotsen-api\n"
	hash, err := parseChecksumEntry(strings.NewReader(input), "lotsen-proxy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash != "" {
		t.Fatalf("hash = %q, want empty string", hash)
	}
}

func TestParseChecksumEntry_HandlesMultipleEntries(t *testing.T) {
	input := "aaa111  lotsen-api\nbbb222  lotsen-proxy\nccc333  lotsen-orchestrator\n"
	hash, err := parseChecksumEntry(strings.NewReader(input), "lotsen-proxy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash != "bbb222" {
		t.Fatalf("hash = %q, want %q", hash, "bbb222")
	}
}

func TestProxyRestartNeeded_SkipsWhenHashesMatch(t *testing.T) {
	if proxyRestartNeeded("deadbeef", "deadbeef") {
		t.Fatal("expected no restart when hashes match")
	}
}

func TestProxyRestartNeeded_RestartsWhenHashDiffers(t *testing.T) {
	if !proxyRestartNeeded("deadbeef", "cafebabe") {
		t.Fatal("expected restart when hashes differ")
	}
}

func TestProxyRestartNeeded_RestartsWhenNoPreHash(t *testing.T) {
	if !proxyRestartNeeded("", "cafebabe") {
		t.Fatal("expected restart when no pre-hash provided")
	}
}

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
