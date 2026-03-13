package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ercadev/lotsen/store"
)

const (
	defaultRecentDeploymentLogTail = 300
	maxRecentDeploymentLogTail     = 1000
)

type deploymentRecentLogsResponse struct {
	DeploymentID string   `json:"deploymentId"`
	Lines        []string `json:"lines"`
}

func (h *Handler) deploymentRecentLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := h.store.Get(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get deployment", http.StatusInternalServerError)
		return
	}

	tail, err := parseRecentDeploymentLogTail(r.URL.Query().Get("tail"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	lines, err := h.dockerLogs.RecentLogs(r.Context(), id, tail)
	if err != nil {
		http.Error(w, "failed to read deployment logs", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, deploymentRecentLogsResponse{DeploymentID: id, Lines: lines})
}

func parseRecentDeploymentLogTail(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultRecentDeploymentLogTail, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("tail must be a positive integer")
	}
	if n > maxRecentDeploymentLogTail {
		n = maxRecentDeploymentLogTail
	}

	return n, nil
}
