package main

import (
	"path/filepath"
	"testing"
)

func TestAuthFromEnv_NoSecret(t *testing.T) {
	t.Setenv("LOTSEN_JWT_SECRET", "")
	storePath := filepath.Join(t.TempDir(), "deployments.json")
	userStore, secret, err := authFromEnv(storePath)
	if err != nil {
		t.Fatalf("authFromEnv() error = %v", err)
	}
	if userStore != nil {
		t.Error("want nil userStore when no JWT secret set")
		_ = userStore.Close()
	}
	if secret != nil {
		t.Error("want nil secret when no JWT secret set")
	}
}

func TestAuthFromEnv_WithSecret(t *testing.T) {
	t.Setenv("LOTSEN_JWT_SECRET", "test-secret")
	storePath := filepath.Join(t.TempDir(), "deployments.json")
	userStore, secret, err := authFromEnv(storePath)
	if err != nil {
		t.Fatalf("authFromEnv() error = %v", err)
	}
	if userStore == nil {
		t.Fatal("want non-nil userStore when JWT secret set")
	}
	t.Cleanup(func() { _ = userStore.Close() })

	if string(secret) != "test-secret" {
		t.Fatalf("secret = %q, want %q", string(secret), "test-secret")
	}
}

func TestAuthFromEnv_NoUsers_LogsNotice(t *testing.T) {
	// Just verify the function doesn't error when store is empty.
	t.Setenv("LOTSEN_JWT_SECRET", "some-secret")
	storePath := filepath.Join(t.TempDir(), "deployments.json")
	userStore, _, err := authFromEnv(storePath)
	if err != nil {
		t.Fatalf("authFromEnv() error = %v", err)
	}
	if userStore == nil {
		t.Fatal("want non-nil userStore")
	}
	defer userStore.Close()

	has, err := userStore.HasAnyUser()
	if err != nil {
		t.Fatalf("HasAnyUser: %v", err)
	}
	if has {
		t.Error("want no users in fresh store")
	}
}
