package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

func (h *Handler) recordOrchestratorHeartbeat(w http.ResponseWriter, r *http.Request) {
	if h.heartbeats == nil {
		http.Error(w, "system status unavailable", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB

	var body struct {
		At           *time.Time `json:"at"`
		Orchestrator *struct {
			StoreAccessible *bool `json:"storeAccessible"`
		} `json:"orchestrator"`
		Docker *struct {
			Reachable *bool      `json:"reachable"`
			CheckedAt *time.Time `json:"checkedAt"`
		} `json:"docker"`
		LoadBalancer *struct {
			Responding *bool      `json:"responding"`
			CheckedAt  *time.Time `json:"checkedAt"`
			Traffic    *struct {
				TotalRequests      *int64 `json:"totalRequests"`
				SuspiciousRequests *int64 `json:"suspiciousRequests"`
				BlockedRequests    *int64 `json:"blockedRequests"`
				ActiveBlockedIPs   *int   `json:"activeBlockedIps"`
				BlockedIPs         []struct {
					IP           string     `json:"ip"`
					BlockedUntil *time.Time `json:"blockedUntil"`
				} `json:"blockedIps"`
			} `json:"traffic"`
		} `json:"loadBalancer"`
		Host *struct {
			CPU *struct {
				UsagePercent *float64   `json:"usagePercent"`
				CheckedAt    *time.Time `json:"checkedAt"`
			} `json:"cpu"`
			RAM *struct {
				UsagePercent *float64   `json:"usagePercent"`
				CheckedAt    *time.Time `json:"checkedAt"`
			} `json:"ram"`
		} `json:"host"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	heartbeatAt := time.Time{}
	if body.At != nil {
		heartbeatAt = body.At.UTC()
	}

	storeAccessible := true
	if body.Orchestrator != nil && body.Orchestrator.StoreAccessible != nil {
		storeAccessible = *body.Orchestrator.StoreAccessible
	}

	if err := h.heartbeats.RecordOrchestratorHeartbeat(r.Context(), heartbeatAt, storeAccessible); err != nil {
		http.Error(w, "failed to record heartbeat", http.StatusInternalServerError)
		return
	}

	if body.Docker != nil && body.Docker.Reachable != nil {
		if h.docker == nil {
			http.Error(w, "system status unavailable", http.StatusServiceUnavailable)
			return
		}

		dockerCheckedAt := heartbeatAt
		if body.Docker.CheckedAt != nil {
			dockerCheckedAt = body.Docker.CheckedAt.UTC()
		}

		if err := h.docker.RecordDockerConnectivity(r.Context(), *body.Docker.Reachable, dockerCheckedAt); err != nil {
			http.Error(w, "failed to record docker connectivity", http.StatusInternalServerError)
			return
		}
	}

	if body.LoadBalancer != nil && body.LoadBalancer.Responding != nil {
		if h.loadBalancer == nil {
			http.Error(w, "system status unavailable", http.StatusServiceUnavailable)
			return
		}

		checkedAt := heartbeatAt
		if body.LoadBalancer.CheckedAt != nil {
			checkedAt = body.LoadBalancer.CheckedAt.UTC()
		}

		traffic := buildLoadBalancerTraffic(body.LoadBalancer.Traffic)

		if err := h.loadBalancer.RecordLoadBalancerHealth(r.Context(), *body.LoadBalancer.Responding, checkedAt, traffic); err != nil {
			http.Error(w, "failed to record load balancer health", http.StatusInternalServerError)
			return
		}
	}

	if body.Host != nil {
		if body.Host.CPU != nil && body.Host.CPU.UsagePercent != nil {
			if h.cpu == nil {
				http.Error(w, "system status unavailable", http.StatusServiceUnavailable)
				return
			}

			if !isValidUsagePercent(*body.Host.CPU.UsagePercent) {
				http.Error(w, "invalid host metrics", http.StatusBadRequest)
				return
			}

			cpuCheckedAt := heartbeatAt
			if body.Host.CPU.CheckedAt != nil {
				cpuCheckedAt = body.Host.CPU.CheckedAt.UTC()
			}

			if err := h.cpu.RecordCPUUtilization(r.Context(), *body.Host.CPU.UsagePercent, cpuCheckedAt); err != nil {
				http.Error(w, "failed to record cpu utilization", http.StatusInternalServerError)
				return
			}
		}

		if body.Host.RAM != nil && body.Host.RAM.UsagePercent != nil {
			if h.ram == nil {
				http.Error(w, "system status unavailable", http.StatusServiceUnavailable)
				return
			}

			if !isValidUsagePercent(*body.Host.RAM.UsagePercent) {
				http.Error(w, "invalid host metrics", http.StatusBadRequest)
				return
			}

			ramCheckedAt := heartbeatAt
			if body.Host.RAM.CheckedAt != nil {
				ramCheckedAt = body.Host.RAM.CheckedAt.UTC()
			}

			if err := h.ram.RecordRAMUtilization(r.Context(), *body.Host.RAM.UsagePercent, ramCheckedAt); err != nil {
				http.Error(w, "failed to record ram utilization", http.StatusInternalServerError)
				return
			}
		}
	}

	if h.statusEvents != nil {
		h.statusEvents.Publish(h.currentSystemStatusSnapshot(r))
	}

	w.WriteHeader(http.StatusNoContent)
}

func isValidUsagePercent(v float64) bool {
	return v >= 0 && v <= 100
}

func buildLoadBalancerTraffic(in *struct {
	TotalRequests      *int64 `json:"totalRequests"`
	SuspiciousRequests *int64 `json:"suspiciousRequests"`
	BlockedRequests    *int64 `json:"blockedRequests"`
	ActiveBlockedIPs   *int   `json:"activeBlockedIps"`
	BlockedIPs         []struct {
		IP           string     `json:"ip"`
		BlockedUntil *time.Time `json:"blockedUntil"`
	} `json:"blockedIps"`
}) *LoadBalancerTrafficSystemStatus {
	if in == nil {
		return nil
	}

	traffic := &LoadBalancerTrafficSystemStatus{}
	if in.TotalRequests != nil {
		traffic.TotalRequests = *in.TotalRequests
	}
	if in.SuspiciousRequests != nil {
		traffic.SuspiciousRequests = *in.SuspiciousRequests
	}
	if in.BlockedRequests != nil {
		traffic.BlockedRequests = *in.BlockedRequests
	}
	if in.ActiveBlockedIPs != nil {
		traffic.ActiveBlockedIPs = *in.ActiveBlockedIPs
	}

	if len(in.BlockedIPs) > 0 {
		traffic.BlockedIPs = make([]LoadBalancerBlockedIPStatus, 0, len(in.BlockedIPs))
		for _, blocked := range in.BlockedIPs {
			ip := blocked.IP
			if ip == "" {
				continue
			}
			traffic.BlockedIPs = append(traffic.BlockedIPs, LoadBalancerBlockedIPStatus{IP: ip, BlockedUntil: blocked.BlockedUntil})
		}
	}

	return traffic
}
