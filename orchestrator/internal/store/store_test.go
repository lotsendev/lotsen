package store_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ercadev/dirigent/orchestrator/internal/store"
)

func seedStore(t *testing.T, path string, deployments []store.Deployment) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("seed: create file: %v", err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(deployments); err != nil {
		t.Fatalf("seed: encode: %v", err)
	}
}

func TestJSONStore_List(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	seedStore(t, path, []store.Deployment{
		{ID: "d1", Name: "web", Status: store.StatusDeploying},
		{ID: "d2", Name: "api", Status: store.StatusHealthy},
	})

	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("want 2 deployments, got %d", len(list))
	}
}

func TestJSONStore_List_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")

	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("want 0 deployments, got %d", len(list))
	}
}

func TestJSONStore_UpdateStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	seedStore(t, path, []store.Deployment{
		{ID: "d1", Name: "web", Status: store.StatusDeploying},
	})

	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := s.UpdateStatus("d1", store.StatusHealthy); err != nil {
		t.Fatalf("update status: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Status != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", list[0].Status)
	}
}

func TestJSONStore_UpdateStatus_MissingID_NoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")

	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	// Should not error for a missing ID.
	if err := s.UpdateStatus("nonexistent", store.StatusHealthy); err != nil {
		t.Errorf("want nil for missing id, got %v", err)
	}
}
