package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	if p := os.Getenv("DIRIGENT_DATA"); p != "" {
		return p
	}
	return "/var/lib/dirigent/deployments.json"
}

func main() {
	s, err := store.NewJSONStore(dataPath())
	if err != nil {
		log.Fatalf("dirigent: open store: %v", err)
	}

	dc, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		log.Fatalf("dirigent: create docker client: %v", err)
	}
	defer dc.Close()

	broker := events.NewBroker()
	logStreamer := docker.New(dc)

	userStore, jwtSecret, err := authFromEnv(dataPath())
	if err != nil {
		log.Fatalf("dirigent: %v", err)
	}
	if userStore != nil {
		defer userStore.Close()
		log.Printf("dirigent: auth enabled")
	} else {
		log.Printf("dirigent: auth disabled (set DIRIGENT_JWT_SECRET to enable)")
	}

	h := internalapi.NewWithVersion(s, broker, logStreamer, version)
	h.SetAuth(userStore, jwtSecret)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	handler := dashboard.New(mux)

	log.Printf("dirigent API listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("dirigent: %v", err)
	}
}

func authFromEnv(storePath string) (*auth.UserStore, []byte, error) {
	secret := strings.TrimSpace(os.Getenv("DIRIGENT_JWT_SECRET"))
	if secret == "" {
		return nil, nil, nil
	}

	dbPath := filepath.Join(filepath.Dir(storePath), "users.db")
	userStore, err := auth.NewUserStore(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open user store: %w", err)
	}

	// Bootstrap credentials from env on startup.
	user := strings.TrimSpace(os.Getenv("DIRIGENT_AUTH_USER"))
	password := strings.TrimSpace(os.Getenv("DIRIGENT_AUTH_PASSWORD"))
	if user != "" && password != "" {
		if err := userStore.SetPassword(user, password); err != nil {
			userStore.Close()
			return nil, nil, fmt.Errorf("set initial credentials: %w", err)
		}
	} else {
		has, err := userStore.HasUsers()
		if err != nil {
			userStore.Close()
			return nil, nil, fmt.Errorf("check users: %w", err)
		}
		if !has {
			log.Printf("dirigent: WARNING: no users in store and DIRIGENT_AUTH_USER/DIRIGENT_AUTH_PASSWORD not set; login will not work")
		}
	}

	return userStore, []byte(secret), nil
}
