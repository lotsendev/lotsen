package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	dockerclient "github.com/docker/docker/client"

	"github.com/ercadev/lotsen/orchestrator/internal/apiclient"
	"github.com/ercadev/lotsen/orchestrator/internal/docker"
	"github.com/ercadev/lotsen/orchestrator/internal/hostinfo"
	"github.com/ercadev/lotsen/orchestrator/internal/hostmetrics"
	"github.com/ercadev/lotsen/orchestrator/internal/reconciler"
	"github.com/ercadev/lotsen/store"
)

func dataPath() string {
	if p := os.Getenv("LOTSEN_DATA"); p != "" {
		return p
	}
	return "/var/lib/lotsen/deployments.json"
}

func main() {
	interval := 15 * time.Second
	if v := os.Getenv("LOTSEN_INTERVAL"); v != "" {
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

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := d.Ping(startupCtx); err != nil {
		log.Printf("orchestrator: warning: docker unavailable at startup: %v", err)
	}
	startupCancel()

	apiURL := "http://localhost:8080"
	if v := os.Getenv("LOTSEN_API_URL"); v != "" {
		apiURL = v
	}
	proxyHealthURL := "http://localhost/internal/health"
	if v := os.Getenv("LOTSEN_PROXY_HEALTH_URL"); v != "" {
		proxyHealthURL = v
	}
	proxyTrafficURL := "http://localhost/internal/traffic"
	if v := os.Getenv("LOTSEN_PROXY_TRAFFIC_URL"); v != "" {
		proxyTrafficURL = v
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
			hostInfo := hostinfo.Collect()
			dockerReachable := true
			containerStats := map[string]apiclient.HeartbeatContainerStats{}
			loadBalancerResponding := false
			var loadBalancerTraffic *apiclient.HeartbeatLoadBalancerTraffic
			hostMetadata := &apiclient.HeartbeatHostMetadata{
				IPAddress: hostInfo.IPAddress,
				OSName:    hostInfo.OSName,
				OSVersion: hostInfo.OSVersion,
				Specs: apiclient.HeartbeatHostSpecs{
					CPUCores:    hostInfo.CPUCores,
					MemoryBytes: hostInfo.MemoryBytes,
					DiskBytes:   hostInfo.DiskBytes,
				},
			}
			storeAccessible := true
			var cpuUsagePercent *float64
			var ramUsagePercent *float64

			// Ping with a hard deadline so a hung Docker socket never delays the heartbeat.
			pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
			if err := d.Ping(pingCtx); err != nil {
				dockerReachable = false
				log.Printf("orchestrator: docker unreachable: %v", err)
			} else {
				statsCtx, statsCancel := context.WithTimeout(ctx, 12*time.Second)
				statsByDeployment, statsErr := d.CollectStats(statsCtx)
				statsCancel()
				if statsErr != nil {
					log.Printf("orchestrator: collect container stats: %v", statsErr)
				} else {
					for deploymentID, stats := range statsByDeployment {
						containerStats[deploymentID] = apiclient.HeartbeatContainerStats{
							CPUPercent:       stats.CPUPercent,
							MemoryUsedBytes:  stats.MemoryUsedBytes,
							MemoryLimitBytes: stats.MemoryLimitBytes,
							MemoryPercent:    stats.MemoryPercent,
						}
					}
				}
			}
			pingCancel()

			if _, err := s.List(); err != nil {
				storeAccessible = false
				log.Printf("orchestrator: store unavailable: %v", err)
			}

			if err := probeProxyHealth(ctx, proxyHealthURL); err != nil {
				log.Printf("orchestrator: load balancer healthcheck failed: %v", err)
			} else {
				loadBalancerResponding = true
			}

			if traffic, err := probeProxyTraffic(ctx, proxyTrafficURL); err != nil {
				log.Printf("orchestrator: load balancer traffic telemetry failed: %v", err)
			} else {
				loadBalancerTraffic = traffic
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

			// Heartbeat is always sent, regardless of Docker state.
			if err := notifier.NotifyHeartbeat(dockerReachable, loadBalancerResponding, loadBalancerTraffic, storeAccessible, now, cpuUsagePercent, ramUsagePercent, hostMetadata, containerStats); err != nil {
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

func probeProxyTraffic(ctx context.Context, trafficURL string) (*apiclient.HeartbeatLoadBalancerTraffic, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, trafficURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var body struct {
		TotalRequests      int64 `json:"totalRequests"`
		SuspiciousRequests int64 `json:"suspiciousRequests"`
		BlockedRequests    int64 `json:"blockedRequests"`
		WAFBlockedRequests int64 `json:"wafBlockedRequests"`
		UABlockedRequests  int64 `json:"uaBlockedRequests"`
		ActiveBlockedIPs   int   `json:"activeBlockedIps"`
		BlockedIPs         []struct {
			IP           string     `json:"ip"`
			BlockedUntil *time.Time `json:"blockedUntil"`
		} `json:"blockedIps"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	traffic := &apiclient.HeartbeatLoadBalancerTraffic{
		TotalRequests:      body.TotalRequests,
		SuspiciousRequests: body.SuspiciousRequests,
		BlockedRequests:    body.BlockedRequests,
		WAFBlockedRequests: body.WAFBlockedRequests,
		UABlockedRequests:  body.UABlockedRequests,
		ActiveBlockedIPs:   body.ActiveBlockedIPs,
	}

	if len(body.BlockedIPs) > 0 {
		traffic.BlockedIPs = make([]apiclient.HeartbeatLoadBalancerBlockedIPState, 0, len(body.BlockedIPs))
		for _, blocked := range body.BlockedIPs {
			if blocked.IP == "" {
				continue
			}
			traffic.BlockedIPs = append(traffic.BlockedIPs, apiclient.HeartbeatLoadBalancerBlockedIPState{IP: blocked.IP, BlockedUntil: blocked.BlockedUntil})
		}
	}

	return traffic, nil
}

func probeProxyHealth(ctx context.Context, healthURL string) error {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return nil
}
