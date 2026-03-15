package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalapi "github.com/lotsendev/lotsen/internal/api"
	"github.com/lotsendev/lotsen/internal/configplan"
	"github.com/lotsendev/lotsen/internal/configv1"
	"github.com/lotsendev/lotsen/store"
)

func TestRunConfigValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lotsen.json")
	err := os.WriteFile(path, []byte(`{"apiVersion":"lotsen/v1","kind":"LotsenConfig","spec":{"deployments":[],"registries":[],"host":{}}}`), 0o644)
	if err != nil {
		t.Fatalf("write config file: %v", err)
	}

	var out bytes.Buffer
	if err := runConfig([]string{"validate", "-f", path}, &out); err != nil {
		t.Fatalf("run config validate: %v", err)
	}

	if strings.TrimSpace(out.String()) != "config is valid" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunConfigExport_DeterministicOutput(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "deployments.json")
	t.Setenv("LOTSEN_DATA", storePath)
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", filepath.Join(filepath.Dir(storePath), "volumes"))

	s, err := store.NewJSONStore(storePath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	created, err := s.Create(store.Deployment{
		ID:      "dep-z",
		Name:    "zeta",
		Image:   "ghcr.io/acme/zeta:1",
		Envs:    map[string]string{"DATABASE_URL": "postgres://live", "NODE_ENV": "production"},
		Ports:   []string{"8080:8080", "8443:8443/tcp"},
		Domain:  "zeta.example.com",
		Public:  true,
		Volumes: []string{filepath.Join(filepath.Dir(storePath), "volumes", "dep-z", "db") + ":/var/lib/postgres"},
		FileMounts: []store.FileMount{
			{Source: "prometheus.yml", Target: "/etc/prometheus/prometheus.yml", Content: "global:\n", ReadOnly: true},
		},
		BasicAuth: &store.BasicAuthConfig{Users: []store.BasicAuthUser{
			{Username: "zz", Password: "not-placeholder"},
			{Username: "aa", Password: "${LOTSEN_SECRET_EXISTING}"},
		}},
		Status: store.StatusHealthy,
	})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created deployment id")
	}

	if _, err := s.CreateRegistry("r-z", "z.example", "z-user", "z-token"); err != nil {
		t.Fatalf("create registry z: %v", err)
	}
	if _, err := s.CreateRegistry("r-a", "a.example", "a-user", "${LOTSEN_SECRET_ALREADY}"); err != nil {
		t.Fatalf("create registry a: %v", err)
	}

	hostStore, err := internalapi.NewFileHostProfileStore(hostProfilePath(storePath))
	if err != nil {
		t.Fatalf("new host profile store: %v", err)
	}
	_, err = hostStore.Update(internalapi.HostProfile{
		DisplayName:         "prod-vps-1",
		DashboardAccessMode: internalapi.DashboardAccessModeWAFAndLogin,
		DashboardWAF: internalapi.DashboardWAFConfig{
			Mode:        "detection",
			IPAllowlist: []string{"203.0.113.0/24"},
		},
	})
	if err != nil {
		t.Fatalf("update host profile: %v", err)
	}

	outPath1 := filepath.Join(t.TempDir(), "export-1.json")
	outPath2 := filepath.Join(t.TempDir(), "export-2.json")

	if err := runConfig([]string{"export", "-o", outPath1}, io.Discard); err != nil {
		t.Fatalf("first export: %v", err)
	}
	if err := runConfig([]string{"export", "-o", outPath2}, io.Discard); err != nil {
		t.Fatalf("second export: %v", err)
	}

	b1, err := os.ReadFile(outPath1)
	if err != nil {
		t.Fatalf("read first export: %v", err)
	}
	b2, err := os.ReadFile(outPath2)
	if err != nil {
		t.Fatalf("read second export: %v", err)
	}

	if string(b1) != string(b2) {
		t.Fatal("want deterministic export output")
	}

	doc, err := configv1.DecodeStrict(bytes.NewReader(b1))
	if err != nil {
		t.Fatalf("decode export: %v", err)
	}
	if err := configv1.Validate(doc); err != nil {
		t.Fatalf("validate export: %v", err)
	}

	if len(doc.Spec.Deployments) != 1 {
		t.Fatalf("want 1 deployment, got %d", len(doc.Spec.Deployments))
	}
	if doc.Spec.Deployments[0].BasicAuth == nil || len(doc.Spec.Deployments[0].BasicAuth.Users) != 2 {
		t.Fatalf("want exported basic auth users, got %#v", doc.Spec.Deployments[0].BasicAuth)
	}
	if doc.Spec.Deployments[0].BasicAuth.Users[0].Username != "aa" {
		t.Fatalf("want sorted users by username, got %#v", doc.Spec.Deployments[0].BasicAuth.Users)
	}
	if !strings.HasPrefix(doc.Spec.Deployments[0].Env["DATABASE_URL"], "${LOTSEN_SECRET_") {
		t.Fatalf("want DATABASE_URL exported as placeholder, got %q", doc.Spec.Deployments[0].Env["DATABASE_URL"])
	}
	if len(doc.Spec.Deployments[0].FileMounts) != 1 || doc.Spec.Deployments[0].FileMounts[0].Source != "prometheus.yml" {
		t.Fatalf("want exported file mount, got %#v", doc.Spec.Deployments[0].FileMounts)
	}

	if len(doc.Spec.Registries) != 2 || doc.Spec.Registries[0].Prefix != "a.example" {
		t.Fatalf("want registries sorted by prefix, got %#v", doc.Spec.Registries)
	}
	if doc.Spec.Host == nil || doc.Spec.Host.DashboardAccessMode != "waf_and_login" {
		t.Fatalf("want host settings in export, got %#v", doc.Spec.Host)
	}
}

func TestRunConfigPlan_DeterministicOutputAndSections(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "deployments.json")
	t.Setenv("LOTSEN_DATA", storePath)
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", filepath.Join(filepath.Dir(storePath), "volumes"))

	s, err := store.NewJSONStore(storePath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if _, err := s.Create(store.Deployment{ID: "dep-update", Name: "app", Image: "ghcr.io/acme/app:1", Domain: "old.example.com", Public: true, Status: store.StatusHealthy}); err != nil {
		t.Fatalf("create update deployment: %v", err)
	}
	if _, err := s.Create(store.Deployment{ID: "dep-delete", Name: "legacy", Image: "ghcr.io/acme/legacy:1", Domain: "legacy.example.com", Public: true, Status: store.StatusHealthy}); err != nil {
		t.Fatalf("create delete deployment: %v", err)
	}

	if _, err := s.CreateRegistry("r-keep", "a.example", "user-a", "token-a"); err != nil {
		t.Fatalf("create keep registry: %v", err)
	}
	if _, err := s.CreateRegistry("r-delete", "c.example", "user-c", "token-c"); err != nil {
		t.Fatalf("create delete registry: %v", err)
	}

	hostStore, err := internalapi.NewFileHostProfileStore(hostProfilePath(storePath))
	if err != nil {
		t.Fatalf("new host profile store: %v", err)
	}
	if _, err := hostStore.Update(internalapi.HostProfile{DisplayName: "old-host", DashboardAccessMode: internalapi.DashboardAccessModeLoginOnly}); err != nil {
		t.Fatalf("update host profile: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "desired.json")
	desired := `{
  "apiVersion": "lotsen/v1",
  "kind": "LotsenConfig",
  "spec": {
    "deployments": [
      {
        "name": "app",
        "image": "ghcr.io/acme/app:2",
        "domain": "app.example.com",
        "public": true
      },
      {
        "name": "new",
        "image": "ghcr.io/acme/new:1",
        "domain": "new.example.com",
        "public": true
      }
    ],
    "registries": [
      {
        "prefix": "a.example",
        "username": "user-a",
        "password": "${LOTSEN_SECRET_NEW_A}"
      },
      {
        "prefix": "b.example",
        "username": "user-b",
        "password": "${LOTSEN_SECRET_B}"
      }
    ],
    "host": {
      "displayName": "prod-vps-1",
      "dashboardAccessMode": "waf_and_login"
    }
  }
}`

	if err := os.WriteFile(configPath, []byte(desired), 0o644); err != nil {
		t.Fatalf("write desired config: %v", err)
	}

	planPath1 := filepath.Join(t.TempDir(), "plan-1.json")
	planPath2 := filepath.Join(t.TempDir(), "plan-2.json")

	if err := runConfig([]string{"plan", "-f", configPath, "--out", planPath1}, io.Discard); err != nil {
		t.Fatalf("first plan run: %v", err)
	}
	if err := runConfig([]string{"plan", "-f", configPath, "--out", planPath2}, io.Discard); err != nil {
		t.Fatalf("second plan run: %v", err)
	}

	b1, err := os.ReadFile(planPath1)
	if err != nil {
		t.Fatalf("read first plan: %v", err)
	}
	b2, err := os.ReadFile(planPath2)
	if err != nil {
		t.Fatalf("read second plan: %v", err)
	}

	if string(b1) != string(b2) {
		t.Fatal("want deterministic plan output")
	}

	var plan configplan.Document
	if err := json.Unmarshal(b1, &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}

	if plan.Fingerprint == "" {
		t.Fatal("want non-empty fingerprint")
	}
	if len(plan.Actions.Deployments.Create) != 1 || plan.Actions.Deployments.Create[0].Resource != "new" {
		t.Fatalf("unexpected deployment create actions: %#v", plan.Actions.Deployments.Create)
	}
	if len(plan.Actions.Deployments.Update) != 1 || plan.Actions.Deployments.Update[0].Resource != "app" {
		t.Fatalf("unexpected deployment update actions: %#v", plan.Actions.Deployments.Update)
	}
	if len(plan.Actions.Deployments.Delete) != 1 || plan.Actions.Deployments.Delete[0].Resource != "legacy" {
		t.Fatalf("unexpected deployment delete actions: %#v", plan.Actions.Deployments.Delete)
	}

	if len(plan.Actions.Registries.Create) != 1 || plan.Actions.Registries.Create[0].Resource != "b.example" {
		t.Fatalf("unexpected registry create actions: %#v", plan.Actions.Registries.Create)
	}
	if len(plan.Actions.Registries.Noop) != 1 || plan.Actions.Registries.Noop[0].Resource != "a.example" {
		t.Fatalf("unexpected registry noop actions: %#v", plan.Actions.Registries.Noop)
	}
	if len(plan.Actions.Registries.Delete) != 1 || plan.Actions.Registries.Delete[0].Resource != "c.example" {
		t.Fatalf("unexpected registry delete actions: %#v", plan.Actions.Registries.Delete)
	}

	if len(plan.Actions.Host.Update) != 1 || plan.Actions.Host.Update[0].Resource != "host" {
		t.Fatalf("unexpected host actions: %#v", plan.Actions.Host)
	}

	if len(plan.Destructive.Deployments) != 1 || plan.Destructive.Deployments[0].Resource != "legacy" {
		t.Fatalf("unexpected destructive deployment actions: %#v", plan.Destructive.Deployments)
	}
	if len(plan.Destructive.Registries) != 1 || plan.Destructive.Registries[0].Resource != "c.example" {
		t.Fatalf("unexpected destructive registry actions: %#v", plan.Destructive.Registries)
	}
}

func TestRunConfigApply_EndToEndAndIdempotentRerun(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "deployments.json")
	t.Setenv("LOTSEN_DATA", storePath)
	t.Setenv("LOTSEN_MANAGED_VOLUMES_DIR", filepath.Join(filepath.Dir(storePath), "volumes"))

	s, err := store.NewJSONStore(storePath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if _, err := s.Create(store.Deployment{ID: "dep-update", Name: "app", Image: "ghcr.io/acme/app:1", Domain: "old.example.com", Public: true, Status: store.StatusHealthy}); err != nil {
		t.Fatalf("create update deployment: %v", err)
	}
	if _, err := s.Create(store.Deployment{ID: "dep-delete", Name: "legacy", Image: "ghcr.io/acme/legacy:1", Domain: "legacy.example.com", Public: true, Status: store.StatusHealthy}); err != nil {
		t.Fatalf("create delete deployment: %v", err)
	}

	if _, err := s.CreateRegistry("r-keep", "a.example", "user-a", "token-a"); err != nil {
		t.Fatalf("create keep registry: %v", err)
	}
	if _, err := s.CreateRegistry("r-delete", "c.example", "user-c", "token-c"); err != nil {
		t.Fatalf("create delete registry: %v", err)
	}

	hostStore, err := internalapi.NewFileHostProfileStore(hostProfilePath(storePath))
	if err != nil {
		t.Fatalf("new host profile store: %v", err)
	}
	if _, err := hostStore.Update(internalapi.HostProfile{DisplayName: "old-host", DashboardAccessMode: internalapi.DashboardAccessModeLoginOnly}); err != nil {
		t.Fatalf("seed host profile: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "desired.json")
	desired := `{
  "apiVersion": "lotsen/v1",
  "kind": "LotsenConfig",
  "spec": {
    "deployments": [
      {
        "name": "app",
        "image": "ghcr.io/acme/app:2",
        "domain": "app.example.com",
        "public": true
      },
      {
        "name": "new",
        "image": "ghcr.io/acme/new:1",
        "domain": "new.example.com",
        "public": true
      }
    ],
    "registries": [
      {
        "prefix": "a.example",
        "username": "user-a",
        "password": "${LOTSEN_SECRET_NEW_A}"
      },
      {
        "prefix": "b.example",
        "username": "user-b",
        "password": "${LOTSEN_SECRET_B}"
      }
    ],
    "host": {
      "displayName": "prod-vps-1",
      "dashboardAccessMode": "waf_and_login"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(desired), 0o644); err != nil {
		t.Fatalf("write desired config: %v", err)
	}

	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := runConfig([]string{"plan", "-f", configPath, "--out", planPath}, io.Discard); err != nil {
		t.Fatalf("plan run: %v", err)
	}

	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan: %v", err)
	}
	var plan configplan.Document
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}

	var out bytes.Buffer
	if err := runConfig([]string{"apply", "-f", configPath, "--plan", planPath, "--approve", plan.Fingerprint}, &out); err != nil {
		t.Fatalf("apply run: %v", err)
	}

	var applyResult struct {
		Summary struct {
			Applied int `json:"applied"`
			Noop    int `json:"noop"`
			Failed  int `json:"failed"`
		} `json:"summary"`
		PartialFailed bool `json:"partialFailed"`
	}
	if err := json.Unmarshal(out.Bytes(), &applyResult); err != nil {
		t.Fatalf("decode apply output: %v", err)
	}
	if applyResult.PartialFailed {
		t.Fatalf("want successful apply, got partial failure: %s", out.String())
	}
	if applyResult.Summary.Failed != 0 {
		t.Fatalf("want 0 failed outcomes, got %d", applyResult.Summary.Failed)
	}

	deployments, err := s.List()
	if err != nil {
		t.Fatalf("list deployments after apply: %v", err)
	}
	byName := map[string]store.Deployment{}
	for _, deployment := range deployments {
		byName[deployment.Name] = deployment
	}
	if _, exists := byName["legacy"]; exists {
		t.Fatalf("want legacy deployment deleted, got %#v", byName["legacy"])
	}
	if byName["app"].Image != "ghcr.io/acme/app:2" {
		t.Fatalf("want app image updated, got %q", byName["app"].Image)
	}
	if _, exists := byName["new"]; !exists {
		t.Fatal("want new deployment created")
	}

	registries, err := s.ListRegistries()
	if err != nil {
		t.Fatalf("list registries after apply: %v", err)
	}
	prefixes := make(map[string]struct{}, len(registries))
	for _, registry := range registries {
		prefixes[registry.Prefix] = struct{}{}
	}
	if _, exists := prefixes["c.example"]; exists {
		t.Fatal("want c.example registry deleted")
	}
	if _, exists := prefixes["b.example"]; !exists {
		t.Fatal("want b.example registry created")
	}

	hostProfile, err := hostStore.Get()
	if err != nil {
		t.Fatalf("get host profile after apply: %v", err)
	}
	if hostProfile.DisplayName != "prod-vps-1" || hostProfile.DashboardAccessMode != internalapi.DashboardAccessModeWAFAndLogin {
		t.Fatalf("unexpected host profile after apply: %#v", hostProfile)
	}

	planPath2 := filepath.Join(t.TempDir(), "plan-2.json")
	if err := runConfig([]string{"plan", "-f", configPath, "--out", planPath2}, io.Discard); err != nil {
		t.Fatalf("second plan run: %v", err)
	}
	planBytes2, err := os.ReadFile(planPath2)
	if err != nil {
		t.Fatalf("read second plan: %v", err)
	}
	var plan2 configplan.Document
	if err := json.Unmarshal(planBytes2, &plan2); err != nil {
		t.Fatalf("decode second plan: %v", err)
	}

	out.Reset()
	if err := runConfig([]string{"apply", "-f", configPath, "--plan", planPath2, "--approve", plan2.Fingerprint}, &out); err != nil {
		t.Fatalf("second apply run: %v", err)
	}
	if err := json.Unmarshal(out.Bytes(), &applyResult); err != nil {
		t.Fatalf("decode second apply output: %v", err)
	}
	if applyResult.PartialFailed || applyResult.Summary.Failed != 0 {
		t.Fatalf("want idempotent successful second apply, got %s", out.String())
	}
}

func TestRunConfigApply_RejectsStaleFingerprint(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "deployments.json")
	t.Setenv("LOTSEN_DATA", storePath)

	s, err := store.NewJSONStore(storePath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if _, err := s.Create(store.Deployment{ID: "dep-1", Name: "app", Image: "ghcr.io/acme/app:1", Domain: "app.example.com", Public: true, Status: store.StatusHealthy}); err != nil {
		t.Fatalf("seed deployment: %v", err)
	}

	hostStore, err := internalapi.NewFileHostProfileStore(hostProfilePath(storePath))
	if err != nil {
		t.Fatalf("new host profile store: %v", err)
	}
	if _, err := hostStore.Update(internalapi.HostProfile{}); err != nil {
		t.Fatalf("seed host profile: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "desired.json")
	desired := `{
  "apiVersion": "lotsen/v1",
  "kind": "LotsenConfig",
  "spec": {
    "deployments": [
      {
        "name": "app",
        "image": "ghcr.io/acme/app:2",
        "domain": "app.example.com",
        "public": true
      }
    ],
    "registries": [],
    "host": {}
  }
}`
	if err := os.WriteFile(configPath, []byte(desired), 0o644); err != nil {
		t.Fatalf("write desired config: %v", err)
	}

	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := runConfig([]string{"plan", "-f", configPath, "--out", planPath}, io.Discard); err != nil {
		t.Fatalf("plan run: %v", err)
	}

	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan: %v", err)
	}
	var plan configplan.Document
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}

	if _, err := s.Create(store.Deployment{ID: "dep-race", Name: "race", Image: "ghcr.io/acme/race:1", Domain: "race.example.com", Public: true, Status: store.StatusHealthy}); err != nil {
		t.Fatalf("introduce drift: %v", err)
	}

	err = runConfig([]string{"apply", "-f", configPath, "--plan", planPath, "--approve", plan.Fingerprint}, io.Discard)
	if err == nil {
		t.Fatal("want stale fingerprint error")
	}
	if !strings.Contains(err.Error(), "stale approval fingerprint") {
		t.Fatalf("want stale fingerprint error, got %v", err)
	}
}
