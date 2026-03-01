package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ercadev/dirigent/auth"
)

func TestAuthFromEnvBootstrapsFirstUser(t *testing.T) {
	t.Setenv("LOTSEN_JWT_SECRET", "test-secret")
	t.Setenv("LOTSEN_AUTH_USER", "admin")
	t.Setenv("LOTSEN_AUTH_PASSWORD", "bootstrap-pass")

	storePath := filepath.Join(t.TempDir(), "deployments.json")
	userStore, secret, err := authFromEnv(storePath)
	if err != nil {
		t.Fatalf("authFromEnv() error = %v", err)
	}
	t.Cleanup(func() {
		_ = userStore.Close()
	})

	if string(secret) != "test-secret" {
		t.Fatalf("secret = %q, want %q", string(secret), "test-secret")
	}

	if err := userStore.Authenticate("admin", "bootstrap-pass"); err != nil {
		t.Fatalf("Authenticate() error = %v, want nil", err)
	}
}

func TestAuthFromEnvIgnoresBootstrapEnvWhenUsersExist(t *testing.T) {
	storeDir := t.TempDir()
	storePath := filepath.Join(storeDir, "deployments.json")

	seedStore, err := auth.NewUserStore(filepath.Join(storeDir, "users.db"))
	if err != nil {
		t.Fatalf("NewUserStore() error = %v", err)
	}
	if err := seedStore.SetPassword("admin", "existing-pass"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	if err := seedStore.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	t.Setenv("LOTSEN_JWT_SECRET", "test-secret")
	t.Setenv("LOTSEN_AUTH_USER", "admin")
	t.Setenv("LOTSEN_AUTH_PASSWORD", "new-bootstrap-pass")

	userStore, _, err := authFromEnv(storePath)
	if err != nil {
		t.Fatalf("authFromEnv() error = %v", err)
	}
	t.Cleanup(func() {
		_ = userStore.Close()
	})

	if err := userStore.Authenticate("admin", "existing-pass"); err != nil {
		t.Fatalf("Authenticate(existing) error = %v, want nil", err)
	}

	err = userStore.Authenticate("admin", "new-bootstrap-pass")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("Authenticate(new bootstrap pass) error = %v, want %v", err, auth.ErrInvalidCredentials)
	}
}
