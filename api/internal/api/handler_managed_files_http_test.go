package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDeployment_FileMount(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOTSEN_MANAGED_FILES_DIR", base)

	srv := newTestServer(newMemStore())
	defer srv.Close()

	body := []byte(`{
		"name":"prometheus",
		"image":"prom/prometheus",
		"ports":["9090"],
		"envs":{},
		"file_mounts":[{"source":"prometheus.yml","target":"/etc/prometheus/prometheus.yml","content":"global:\n  scrape_interval: 15s\n","read_only":true}]
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
		ID         string `json:"id"`
		FileMounts []struct {
			Source   string `json:"source"`
			Target   string `json:"target"`
			Content  string `json:"content"`
			ReadOnly bool   `json:"read_only"`
		} `json:"file_mounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(created.FileMounts) != 1 {
		t.Fatalf("want one file_mounts entry, got %d", len(created.FileMounts))
	}
	if created.FileMounts[0].Source != "prometheus.yml" || !created.FileMounts[0].ReadOnly {
		t.Fatalf("unexpected file mount %#v", created.FileMounts[0])
	}

	managedPath := filepath.Join(base, created.ID, "prometheus.yml")
	content, err := os.ReadFile(managedPath)
	if err != nil {
		t.Fatalf("read managed file: %v", err)
	}
	if string(content) != created.FileMounts[0].Content {
		t.Fatalf("want managed content %q, got %q", created.FileMounts[0].Content, string(content))
	}
}
