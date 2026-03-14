package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalapi "github.com/lotsendev/lotsen/internal/api"
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

	if len(doc.Spec.Registries) != 2 || doc.Spec.Registries[0].Prefix != "a.example" {
		t.Fatalf("want registries sorted by prefix, got %#v", doc.Spec.Registries)
	}
	if doc.Spec.Host == nil || doc.Spec.Host.DashboardAccessMode != "waf_and_login" {
		t.Fatalf("want host settings in export, got %#v", doc.Spec.Host)
	}
}
