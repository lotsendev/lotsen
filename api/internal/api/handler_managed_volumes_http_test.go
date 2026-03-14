package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDeployment_ManagedVolumeMount(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", base)

	srv := newTestServer(newMemStore())
	defer srv.Close()

	body := []byte(`{
		"name":"postgres",
		"image":"postgres:16",
		"ports":["5432"],
		"envs":{},
		"volume_mounts":[{"mode":"managed","source":"data","target":"/var/lib/postgresql/data"}]
	}`)

	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}

	var created struct {
		ID           string   `json:"id"`
		Volumes      []string `json:"volumes"`
		VolumeMounts []struct {
			Mode   string `json:"mode"`
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"volume_mounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(created.Volumes) != 1 {
		t.Fatalf("want one resolved volume binding, got %v", created.Volumes)
	}
	wantPrefix := filepath.Join(base, created.ID, "data") + ":"
	if len(created.Volumes[0]) <= len(wantPrefix) || created.Volumes[0][:len(wantPrefix)] != wantPrefix {
		t.Fatalf("want managed source prefix %q in %q", wantPrefix, created.Volumes[0])
	}

	if len(created.VolumeMounts) != 1 {
		t.Fatalf("want one volume_mounts entry, got %d", len(created.VolumeMounts))
	}
	if created.VolumeMounts[0].Mode != "managed" || created.VolumeMounts[0].Source != "data" {
		t.Fatalf("want managed mount data, got %+v", created.VolumeMounts[0])
	}
}

func TestCreateDeployment_ManagedVolumeMountRejectsTraversalName(t *testing.T) {
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", t.TempDir())

	srv := newTestServer(newMemStore())
	defer srv.Close()

	body := []byte(`{
		"name":"postgres",
		"image":"postgres:16",
		"ports":["5432"],
		"envs":{},
		"volume_mounts":[{"mode":"managed","source":"../escape","target":"/var/lib/postgresql/data"}]
	}`)

	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestCreateDeployment_ManagedVolumeMountAppliesDirMode(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", base)

	srv := newTestServer(newMemStore())
	defer srv.Close()

	body := []byte(`{
		"name":"pgadmin",
		"image":"dpage/pgadmin4",
		"ports":["80"],
		"envs":{},
		"volume_mounts":[{"mode":"managed","source":"data","target":"/var/lib/pgadmin","dir_mode":"0770"}]
	}`)

	resp, err := http.Post(srv.URL+"/api/deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/deployments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	managedPath := filepath.Join(base, created.ID, "data")
	info, statErr := os.Stat(managedPath)
	if statErr != nil {
		t.Fatalf("stat managed path: %v", statErr)
	}
	if got := info.Mode().Perm(); got != 0o770 {
		t.Fatalf("want managed dir mode 0770, got %04o", got)
	}
}
