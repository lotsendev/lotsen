package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFileMounts_CreatesManagedFile(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOTSEN_MANAGED_FILES_DIR", base)

	mounts, err := resolveFileMounts("dep-1", []fileMountRequest{
		{Source: "prometheus.yml", Target: "/etc/prometheus/prometheus.yml", Content: "global:\n  scrape_interval: 15s\n", ReadOnly: true},
	})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(mounts) != 1 {
		t.Fatalf("want one mount, got %d", len(mounts))
	}

	hostPath := filepath.Join(base, "dep-1", "prometheus.yml")
	content, err := os.ReadFile(hostPath)
	if err != nil {
		t.Fatalf("read managed file: %v", err)
	}
	if string(content) != mounts[0].Content {
		t.Fatalf("want file content %q, got %q", mounts[0].Content, string(content))
	}

	info, err := os.Stat(hostPath)
	if err != nil {
		t.Fatalf("stat managed file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("want managed file mode 0644 by default, got %04o", got)
	}
}

func TestResolveFileMounts_RejectsDuplicateTarget(t *testing.T) {
	t.Setenv("LOTSEN_MANAGED_FILES_DIR", t.TempDir())

	_, err := resolveFileMounts("dep-1", []fileMountRequest{
		{Source: "a.yml", Target: "/etc/app/config.yml", Content: "a"},
		{Source: "b.yml", Target: "/etc/app/config.yml", Content: "b"},
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestResolveFileMounts_RejectsLargeContent(t *testing.T) {
	t.Setenv("LOTSEN_MANAGED_FILES_DIR", t.TempDir())

	large := make([]byte, maxFileMountContentLen+1)
	for i := range large {
		large[i] = 'a'
	}

	_, err := resolveFileMounts("dep-1", []fileMountRequest{
		{Source: "app.yml", Target: "/etc/app/app.yml", Content: string(large)},
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}
