package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	dockerclient "github.com/docker/docker/client"

	"github.com/ercadev/dirigent/orchestrator/internal/apiclient"
	"github.com/ercadev/dirigent/orchestrator/internal/docker"
	"github.com/ercadev/dirigent/orchestrator/internal/hostmetrics"
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

	apiURL := "http://localhost:8080"
	if v := os.Getenv("DIRIGENT_API_URL"); v != "" {
		apiURL = v
	}
	notifier := apiclient.New(apiURL)
	metrics := hostmetrics.NewCollector()

	r := reconciler.New(s, d, notifier)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("orchestrator: starting reconciliation loop (interval=%s)", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now().UTC()
			dockerReachable := true
			var cpuUsagePercent *float64
			var ramUsagePercent *float64
			if err := d.Ping(ctx); err != nil {
				dockerReachable = false
				log.Printf("orchestrator: docker unreachable: %v", err)
			}

			if usage, ok, err := metrics.CPUUsagePercent(); err != nil {
				log.Printf("orchestrator: collect cpu telemetry: %v", err)
			} else if ok {
				cpuUsagePercent = &usage
			}

			if usage, ok, err := metrics.RAMUsagePercent(); err != nil {
				log.Printf("orchestrator: collect ram telemetry: %v", err)
			} else if ok {
				ramUsagePercent = &usage
			}

			if err := notifier.NotifyHeartbeat(dockerReachable, now, cpuUsagePercent, ramUsagePercent); err != nil {
				log.Printf("orchestrator: notify heartbeat: %v", err)
			}

			if err := r.Reconcile(ctx); err != nil {
				log.Printf("orchestrator: reconcile: %v", err)
			}
		case <-ctx.Done():
			log.Println("orchestrator: shutting down")
			return
		}
	}
}
