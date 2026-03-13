package main

import (
	"path/filepath"
	"testing"

	internalapi "github.com/lotsendev/lotsen/internal/api"
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

func TestAuthCookieDomainFromEnv_Empty(t *testing.T) {
	t.Setenv("LOTSEN_AUTH_COOKIE_DOMAIN", "")

	domain, err := authCookieDomainFromEnv()
	if err != nil {
		t.Fatalf("authCookieDomainFromEnv() error = %v", err)
	}
	if domain != "" {
		t.Fatalf("want empty domain, got %q", domain)
	}
}

func TestAuthCookieDomainFromEnv_NormalizesLeadingDot(t *testing.T) {
	t.Setenv("LOTSEN_AUTH_COOKIE_DOMAIN", ".D0001.Erca.Dev")

	domain, err := authCookieDomainFromEnv()
	if err != nil {
		t.Fatalf("authCookieDomainFromEnv() error = %v", err)
	}
	if domain != "d0001.erca.dev" {
		t.Fatalf("want d0001.erca.dev, got %q", domain)
	}
}

func TestAuthCookieDomainFromEnv_RejectsInvalid(t *testing.T) {
	t.Setenv("LOTSEN_AUTH_COOKIE_DOMAIN", "localhost")

	if _, err := authCookieDomainFromEnv(); err == nil {
		t.Fatal("want error for invalid cookie domain")
	}
}

func TestDashboardAccessModeFromEnv_DefaultsToLoginOnly(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_ACCESS_MODE", "")

	mode, err := dashboardAccessModeFromEnv()
	if err != nil {
		t.Fatalf("dashboardAccessModeFromEnv() error = %v", err)
	}
	if mode != internalapi.DashboardAccessModeLoginOnly {
		t.Fatalf("want %q, got %q", internalapi.DashboardAccessModeLoginOnly, mode)
	}
}

func TestDashboardAccessModeFromEnv_AcceptsWAFAndLogin(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_ACCESS_MODE", " waf_and_login ")

	mode, err := dashboardAccessModeFromEnv()
	if err != nil {
		t.Fatalf("dashboardAccessModeFromEnv() error = %v", err)
	}
	if mode != internalapi.DashboardAccessModeWAFAndLogin {
		t.Fatalf("want %q, got %q", internalapi.DashboardAccessModeWAFAndLogin, mode)
	}
}

func TestDashboardAccessModeFromEnv_RejectsInvalid(t *testing.T) {
	t.Setenv("LOTSEN_DASHBOARD_ACCESS_MODE", "strict")

	if _, err := dashboardAccessModeFromEnv(); err == nil {
		t.Fatal("want validation error for invalid dashboard access mode")
	}
}
