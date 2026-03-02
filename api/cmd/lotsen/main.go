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
