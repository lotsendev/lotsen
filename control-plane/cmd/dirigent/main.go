package main

import (
	"context"
	"log"
	"net/http"
	"os"

	dockerclient "github.com/docker/docker/client"

	"github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/internal/orchestrator"
	"github.com/ercadev/dirigent/internal/store"
)

const addr = ":8080"

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

	dockerCli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		log.Fatalf("dirigent: create docker client: %v", err)
	}
	defer dockerCli.Close()

	orch := orchestrator.New(dockerCli)

	if err := orch.Ping(context.Background()); err != nil {
		log.Printf("dirigent: warning: %v", err)
	}

	mux := http.NewServeMux()
	api.New(s, orch).RegisterRoutes(mux)

	log.Printf("dirigent API listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("dirigent: %v", err)
	}
}
