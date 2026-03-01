package store_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ercadev/dirigent/store"
)

// seedStore writes deployments directly to the JSON file, bypassing the store.
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

func TestJSONStore_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")

	s1, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	created, err := s1.Create(store.Deployment{
		ID:     "abc123",
		Name:   "web",
		Image:  "nginx:latest",
		Envs:   map[string]string{"PORT": "80"},
		Ports:  []string{"80:80"},
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

	list, err := s2.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
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

func TestJSONStore_EmptyFileOnFirstStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")

	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store on missing file: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("want empty list, got %d items", len(list))
	}
}

func TestJSONStore_CorruptedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	if err := os.WriteFile(path, []byte("not valid json {{{"), 0o644); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}

	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	// Corrupted file is detected on first read.
	_, err = s.List()
	if err == nil {
		t.Fatal("want error reading corrupted file, got nil")
	}
}

func TestJSONStore_InvalidPath(t *testing.T) {
	_, err := store.NewJSONStore("")
	if err == nil {
		t.Fatal("want error for empty path, got nil")
	}

	_, err = store.NewJSONStore("relative/path.json")
	if err == nil {
		t.Fatal("want error for relative path, got nil")
	}
}

func TestJSONStore_Get_Found(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Create(store.Deployment{ID: "d1", Name: "web", Status: store.StatusDeploying})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	d, err := s.Get("d1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if d.ID != "d1" {
		t.Errorf("want id d1, got %s", d.ID)
	}
}

func TestJSONStore_Get_NotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Get("nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestJSONStore_Create_DuplicateName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Create(store.Deployment{ID: "d1", Name: "web", Status: store.StatusDeploying})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}

	_, err = s.Create(store.Deployment{ID: "d2", Name: "web", Status: store.StatusDeploying})
	if !errors.Is(err, store.ErrDuplicateName) {
		t.Errorf("want ErrDuplicateName, got %v", err)
	}
}

func TestJSONStore_Update(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Create(store.Deployment{ID: "d1", Name: "web", Status: store.StatusDeploying})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := s.Update(store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Status != store.StatusHealthy {
		t.Errorf("want status healthy, got %s", updated.Status)
	}

	d, _ := s.Get("d1")
	if d.Status != store.StatusHealthy {
		t.Errorf("want persisted status healthy, got %s", d.Status)
	}
}

func TestJSONStore_Update_NotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Update(store.Deployment{ID: "nonexistent", Name: "web"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
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

	list, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
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

func TestJSONStore_Patch_MergesFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Create(store.Deployment{
		ID:      "d1",
		Name:    "web",
		Image:   "nginx:1",
		Envs:    map[string]string{"PORT": "80"},
		Ports:   []string{"80:80"},
		Volumes: []string{"/data:/data"},
		Domain:  "example.com",
		Security: &store.SecurityConfig{
			WAFEnabled: true,
			IPDenylist: []string{"10.0.0.0/8"},
		},
		Status: store.StatusHealthy,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Patch only image and status; all other fields must be preserved.
	updated, err := s.Patch("d1", store.Deployment{
		Image:  "nginx:2",
		Status: store.StatusDeploying,
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	if updated.Image != "nginx:2" {
		t.Errorf("want image nginx:2, got %s", updated.Image)
	}
	if updated.Status != store.StatusDeploying {
		t.Errorf("want status deploying, got %s", updated.Status)
	}
	// Unspecified fields must be retained.
	if updated.Name != "web" {
		t.Errorf("want name web, got %s", updated.Name)
	}
	if updated.Envs["PORT"] != "80" {
		t.Errorf("want PORT=80, got %v", updated.Envs)
	}
	if len(updated.Ports) != 1 || updated.Ports[0] != "80:80" {
		t.Errorf("want ports [80:80], got %v", updated.Ports)
	}
	if len(updated.Volumes) != 1 || updated.Volumes[0] != "/data:/data" {
		t.Errorf("want volumes [/data:/data], got %v", updated.Volumes)
	}
	if updated.Domain != "example.com" {
		t.Errorf("want domain example.com, got %s", updated.Domain)
	}
	if updated.Security == nil || !updated.Security.WAFEnabled {
		t.Fatalf("want security config retained, got %#v", updated.Security)
	}
	if len(updated.Security.IPDenylist) != 1 || updated.Security.IPDenylist[0] != "10.0.0.0/8" {
		t.Fatalf("want security denylist retained, got %#v", updated.Security)
	}

	// Verify persistence.
	reloaded, err := s.Get("d1")
	if err != nil {
		t.Fatalf("get after patch: %v", err)
	}
	if reloaded.Image != "nginx:2" {
		t.Errorf("want persisted image nginx:2, got %s", reloaded.Image)
	}
}

func TestJSONStore_Patch_UpdatesSecurityConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Create(store.Deployment{ID: "d1", Name: "web", Status: store.StatusHealthy})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := s.Patch("d1", store.Deployment{Security: &store.SecurityConfig{
		WAFEnabled:  true,
		IPDenylist:  []string{"10.0.0.0/8"},
		IPAllowlist: []string{"203.0.113.0/24"},
		CustomRules: []string{"SecRule ARGS:test \"@streq blocked\" \"id:10001,phase:2,deny,status:403\""},
	}})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	if updated.Security == nil || !updated.Security.WAFEnabled {
		t.Fatalf("want security waf enabled, got %#v", updated.Security)
	}
	if len(updated.Security.IPAllowlist) != 1 || updated.Security.IPAllowlist[0] != "203.0.113.0/24" {
		t.Fatalf("want security allowlist persisted, got %#v", updated.Security)
	}
	if len(updated.Security.CustomRules) != 1 {
		t.Fatalf("want custom rules persisted, got %#v", updated.Security)
	}
}

func TestJSONStore_Patch_NotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Patch("nonexistent", store.Deployment{Image: "nginx:2"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
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

func TestJSONStore_Patch_StatusClearsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Create(store.Deployment{ID: "d1", Name: "web", Status: store.StatusFailed, Error: "image not found"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := s.Patch("d1", store.Deployment{Status: store.StatusHealthy})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	if updated.Status != store.StatusHealthy {
		t.Fatalf("want status healthy, got %s", updated.Status)
	}
	if updated.Error != "" {
		t.Fatalf("want error cleared, got %q", updated.Error)
	}
}

func TestJSONStore_RegistryAuthEncryptedAtRest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployments.json")
	t.Setenv("LOTSEN_SECRET_KEY", "12345678901234567890123456789012")

	s, err := store.NewJSONStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, err = s.Create(store.Deployment{
		ID:    "d1",
		Name:  "api",
		Image: "ghcr.io/acme/private:1",
		RegistryAuth: &store.RegistryAuth{
			ServerAddress: "ghcr.io",
			Username:      "acme",
			Password:      "secret",
		},
		Status: store.StatusDeploying,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read store file: %v", err)
	}
	if strings.Contains(string(raw), "secret") {
		t.Fatalf("expected password encrypted at rest")
	}

	d, err := s.Get("d1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if d.RegistryAuth == nil || d.RegistryAuth.Password != "secret" {
		t.Fatalf("expected decrypted password, got %#v", d.RegistryAuth)
	}
}
