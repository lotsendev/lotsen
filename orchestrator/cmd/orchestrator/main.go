package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	dockerclient "github.com/docker/docker/client"

	"github.com/ercadev/dirigent/orchestrator/internal/docker"
	"github.com/ercadev/dirigent/orchestrator/internal/reconciler"
	"github.com/ercadev/dirigent/store"
)

func dataPath() string {
	if p := os.Getenv("DIRIGENT_DATA"); p != "" {
		return p
	}
	return "/var/lib/dirigent/deployments.json"
}

func main() {
	interval := 15 * time.Second
	if v := os.Getenv("DIRIGENT_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		}
	}

	s, err := store.NewJSONStore(dataPath())
	if err != nil {
		log.Fatalf("orchestrator: open store: %v", err)
	}

	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		log.Fatalf("orchestrator: create docker client: %v", err)
	}
	defer cli.Close()

	d := docker.New(cli)

	if err := d.Ping(context.Background()); err != nil {
		log.Printf("orchestrator: warning: docker unavailable at startup: %v", err)
	}

	r := reconciler.New(s, d)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("orchestrator: starting reconciliation loop (interval=%s)", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := r.Reconcile(ctx); err != nil {
				log.Printf("orchestrator: reconcile: %v", err)
			}
		case <-ctx.Done():
			log.Println("orchestrator: shutting down")
			return
		}
	}
}
