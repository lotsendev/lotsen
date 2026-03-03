package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-webauthn/webauthn/webauthn"

	dockerclient "github.com/docker/docker/client"

	"github.com/ercadev/dirigent/auth"
	internalapi "github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/internal/dashboard"
	"github.com/ercadev/dirigent/internal/docker"
	"github.com/ercadev/dirigent/internal/events"
	"github.com/ercadev/dirigent/store"
)

const addr = ":8080"

var version = "dev"

func dataPath() string {
	if p := os.Getenv("LOTSEN_DATA"); p != "" {
		return p
	}
	return "/var/lib/lotsen/deployments.json"
}

func main() {
	s, err := store.NewJSONStore(dataPath())
	if err != nil {
		log.Fatalf("lotsen: open store: %v", err)
	}

	dc, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		log.Fatalf("lotsen: create docker client: %v", err)
	}
	defer dc.Close()

	broker := events.NewBroker()
	logStreamer := docker.New(dc)

	userStore, jwtSecret, err := authFromEnv(dataPath())
	if err != nil {
		log.Fatalf("lotsen: %v", err)
	}
	if userStore != nil {
		defer userStore.Close()
		log.Printf("lotsen: auth enabled")
	} else {
		log.Printf("lotsen: auth disabled (set LOTSEN_JWT_SECRET to enable)")
	}

	h := internalapi.NewWithVersion(s, broker, logStreamer, version)
	h.SetAuth(userStore, jwtSecret)
	authCookieDomain, err := authCookieDomainFromEnv()
	if err != nil {
		log.Fatalf("lotsen: %v", err)
	}
	h.SetAuthCookieDomain(authCookieDomain)
	if authCookieDomain != "" {
		log.Printf("lotsen: auth cookie domain set to %s", authCookieDomain)
	}

	// Configure WebAuthn if RP_ID is set.
	if rpID := strings.TrimSpace(os.Getenv("LOTSEN_RP_ID")); rpID != "" {
		originsRaw := strings.TrimSpace(os.Getenv("LOTSEN_RP_ORIGINS"))
		origins := []string{}
		for _, o := range strings.Split(originsRaw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
		displayName := strings.TrimSpace(os.Getenv("LOTSEN_RP_DISPLAY_NAME"))
		if displayName == "" {
			displayName = "Lotsen"
		}

		wa, err := webauthn.New(&webauthn.Config{
			RPDisplayName: displayName,
			RPID:          rpID,
			RPOrigins:     origins,
		})
		if err != nil {
			log.Fatalf("lotsen: init webauthn: %v", err)
		}
		h.SetWebAuthn(wa)
		log.Printf("lotsen: passkeys enabled (RP ID: %s)", rpID)
	} else {
		log.Printf("lotsen: passkeys disabled (set LOTSEN_RP_ID to enable)")
	}

	hostProfileStore, err := internalapi.NewFileHostProfileStore(hostProfilePath(dataPath()))
	if err != nil {
		log.Fatalf("lotsen: open host profile store: %v", err)
	}
	h.SetHostProfileStore(hostProfileStore)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	handler := dashboard.New(mux)

	log.Printf("lotsen API listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("lotsen: %v", err)
	}
}

func hostProfilePath(storePath string) string {
	return filepath.Join(filepath.Dir(storePath), "host_profile.json")
}

func authFromEnv(storePath string) (*auth.UserStore, []byte, error) {
	secret := strings.TrimSpace(os.Getenv("LOTSEN_JWT_SECRET"))
	if secret == "" {
		return nil, nil, nil
	}

	dbPath := filepath.Join(filepath.Dir(storePath), "users.db")
	userStore, err := auth.NewUserStore(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open user store: %w", err)
	}

	// Clean up any expired invite tokens from previous runs.
	if err := userStore.CleanupExpiredTokens(); err != nil {
		log.Printf("lotsen: cleanup expired invite tokens: %v", err)
	}

	hasUsers, err := userStore.HasAnyUser()
	if err != nil {
		userStore.Close()
		return nil, nil, fmt.Errorf("check users: %w", err)
	}

	if !hasUsers {
		log.Printf("lotsen: no users in store — visit the dashboard to complete first-run setup")
	}

	return userStore, []byte(secret), nil
}

func authCookieDomainFromEnv() (string, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(os.Getenv("LOTSEN_AUTH_COOKIE_DOMAIN"), "."))
	if raw == "" {
		return "", nil
	}
	domain := normalizeDomain(raw)
	if !isValidCookieDomain(domain) {
		return "", fmt.Errorf("LOTSEN_AUTH_COOKIE_DOMAIN must be a valid domain")
	}
	return domain, nil
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")
	return strings.ToLower(domain)
}

func isValidCookieDomain(domain string) bool {
	if domain == "" || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}
