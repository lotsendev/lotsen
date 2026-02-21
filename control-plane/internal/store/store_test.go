package store_test

import (
	"path/filepath"
	"testing"

	"github.com/ercadev/dirigent/internal/store"
)

func TestJSONStore_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")

	s1, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	created, err := s1.Create(store.Deployment{
		ID:    "abc123",
		Name:  "web",
		Image: "nginx:latest",
		Envs:  map[string]string{"PORT": "80"},
		Ports: []string{"80:80"},
		Status: store.StatusIdle,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID != "abc123" {
		t.Fatalf("want id abc123, got %s", created.ID)
	}

	// Open a second store instance at the same path to verify persistence.
	s2, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}

	list := s2.List()
	if len(list) != 1 {
		t.Fatalf("want 1 deployment after reload, got %d", len(list))
	}
	if list[0].ID != "abc123" {
		t.Errorf("want id abc123, got %s", list[0].ID)
	}
	if list[0].Name != "web" {
		t.Errorf("want name web, got %s", list[0].Name)
	}
}

func TestJSONStore_Delete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")

	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Create(store.Deployment{ID: "d1", Name: "app", Status: store.StatusIdle})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := s.Delete("d1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if len(s.List()) != 0 {
		t.Error("want empty list after delete")
	}
}

func TestJSONStore_DeleteNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")

	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := s.Delete("nonexistent"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestJSONStore_EmptyFileOnFirstStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")

	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store on missing file: %v", err)
	}

	if list := s.List(); len(list) != 0 {
		t.Errorf("want empty list, got %d items", len(list))
	}
}
