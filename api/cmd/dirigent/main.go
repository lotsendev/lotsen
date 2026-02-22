package main

import (
	"log"
	"net/http"
	"os"

	dockerclient "github.com/docker/docker/client"

	"github.com/ercadev/dirigent/internal/api"
	"github.com/ercadev/dirigent/internal/docker"
	"github.com/ercadev/dirigent/internal/events"
	"github.com/ercadev/dirigent/store"
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

	mux := http.NewServeMux()
	api.New(s, broker, logStreamer).RegisterRoutes(mux)

	log.Printf("dirigent API listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("dirigent: %v", err)
	}
}
