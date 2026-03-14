package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveVolumeBindings_ManagedCreatesDirectory(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", base)

	bindings, err := resolveVolumeBindings("dep-1", nil, []volumeMountRequest{
		{Mode: volumeMountModeManaged, Source: "postgres-data", Target: "/var/lib/postgresql/data"},
	})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	wantSource := filepath.Join(base, "dep-1", "postgres-data")
	want := wantSource + ":/var/lib/postgresql/data"
	if len(bindings) != 1 || bindings[0] != want {
		t.Fatalf("want [%q], got %v", want, bindings)
	}

	if _, statErr := os.Stat(wantSource); statErr != nil {
		t.Fatalf("want managed path to exist: %v", statErr)
	}

	info, err := os.Stat(wantSource)
	if err != nil {
		t.Fatalf("stat managed path: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o777 {
		t.Fatalf("want managed dir mode 0777 by default, got %04o", got)
	}
}

func TestResolveVolumeBindings_RejectsInvalidManagedName(t *testing.T) {
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", t.TempDir())

	_, err := resolveVolumeBindings("dep-1", nil, []volumeMountRequest{
		{Mode: volumeMountModeManaged, Source: "../escape", Target: "/data"},
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestResolveVolumeBindings_RejectsDuplicateTarget(t *testing.T) {
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", t.TempDir())

	_, err := resolveVolumeBindings("dep-1", nil, []volumeMountRequest{
		{Mode: volumeMountModeManaged, Source: "a", Target: "/data"},
		{Mode: volumeMountModeBind, Source: "/srv/data", Target: "/data"},
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestResolveVolumeBindings_RejectsBindOwnershipSettings(t *testing.T) {
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", t.TempDir())

	uid := 1000
	_, err := resolveVolumeBindings("dep-1", nil, []volumeMountRequest{
		{Mode: volumeMountModeBind, Source: "/srv/data", Target: "/data", UID: &uid},
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestResolveVolumeBindings_RejectsInvalidDirMode(t *testing.T) {
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", t.TempDir())

	_, err := resolveVolumeBindings("dep-1", nil, []volumeMountRequest{
		{Mode: volumeMountModeManaged, Source: "data", Target: "/data", DirMode: "0999"},
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestResolveVolumeBindings_AppliesManagedDirModeOverride(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", base)

	_, err := resolveVolumeBindings("dep-1", nil, []volumeMountRequest{
		{Mode: volumeMountModeManaged, Source: "data", Target: "/data", DirMode: "0700"},
	})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	path := filepath.Join(base, "dep-1", "data")
	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stat managed path: %v", statErr)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("want managed dir mode 0700, got %04o", got)
	}
}

func TestVolumeMountsFromBindings_MapsManagedAndBind(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", base)

	managedSource := filepath.Join(base, "dep-1", "state")
	mounts := volumeMountsFromBindings("dep-1", []string{
		managedSource + ":/var/lib/postgresql/data",
		"/srv/files:/files",
	})

	if len(mounts) != 2 {
		t.Fatalf("want 2 mounts, got %d", len(mounts))
	}
	if mounts[0].Mode != volumeMountModeManaged || mounts[0].Source != "state" {
		t.Fatalf("want managed mount state, got %+v", mounts[0])
	}
	if mounts[1].Mode != volumeMountModeBind || mounts[1].Source != "/srv/files" {
		t.Fatalf("want bind mount /srv/files, got %+v", mounts[1])
	}
}
