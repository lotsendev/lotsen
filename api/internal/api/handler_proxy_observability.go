package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AccessLogEntry struct {
	Timestamp      time.Time `json:"timestamp"`
	Method         string    `json:"method"`
	Path           string    `json:"path"`
	StatusCode     int       `json:"statusCode"`
	UpstreamTarget string    `json:"upstreamTarget,omitempty"`
	DurationMs     int64     `json:"durationMs"`
	ClientIP       string    `json:"clientIp,omitempty"`
	Host           string    `json:"host,omitempty"`
}

type SecurityConfig struct {
	Profile                   string   `json:"profile"`
	SuspiciousWindowSeconds   int64    `json:"suspiciousWindowSeconds"`
	SuspiciousThreshold       int      `json:"suspiciousThreshold"`
	SuspiciousBlockForSeconds int64    `json:"suspiciousBlockForSeconds"`
	WAFEnabled                bool     `json:"wafEnabled"`
	WAFMode                   string   `json:"wafMode,omitempty"`
	GlobalIPDenylist          []string `json:"globalIpDenylist,omitempty"`
	GlobalIPAllowlist         []string `json:"globalIpAllowlist,omitempty"`
}

func (h *Handler) accessLogs(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	resp, err := h.fetchProxyJSON(fmt.Sprintf("/internal/access-logs?limit=%d", limit))
	if err != nil {
		http.Error(w, "failed to fetch access logs", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var entries []AccessLogEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		http.Error(w, "invalid access log response", http.StatusBadGateway)
		return
	}
	if entries == nil {
		entries = []AccessLogEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *Handler) securityConfig(w http.ResponseWriter, _ *http.Request) {
	resp, err := h.fetchProxyJSON("/internal/security-config")
	if err != nil {
		http.Error(w, "failed to fetch security config", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var cfg SecurityConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid security config response", http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *Handler) fetchProxyJSON(path string) (*http.Response, error) {
	url := h.proxyBaseURL + path
	resp, err := h.proxyClient.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("proxy returned %d", resp.StatusCode)
	}
	return resp, nil
}
