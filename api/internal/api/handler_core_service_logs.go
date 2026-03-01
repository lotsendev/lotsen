package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

const (
	defaultCoreServiceLogTail = 200
	maxCoreServiceLogTail     = 1000
)

var coreServiceUnits = map[string]string{
	"api":          "lotsen-api",
	"orchestrator": "lotsen-orchestrator",
	"proxy":        "lotsen-proxy",
}

type coreServiceLogsResponse struct {
	Service string   `json:"service"`
	Lines   []string `json:"lines"`
}

func (h *Handler) coreServiceLogs(w http.ResponseWriter, r *http.Request) {
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	unit, ok := coreServiceUnits[service]
	if !ok {
		http.Error(w, "service must be one of: api, orchestrator, proxy", http.StatusBadRequest)
		return
	}

	tail, err := parseCoreServiceLogTail(r.URL.Query().Get("tail"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	lines, err := readJournalctlUnitLogs(r.Context(), unit, tail)
	if err != nil {
		log.Printf("coreServiceLogs: read %s logs: %v", service, err)
		http.Error(w, "failed to read service logs", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, coreServiceLogsResponse{Service: service, Lines: lines})
}

func parseCoreServiceLogTail(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultCoreServiceLogTail, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("tail must be a positive integer")
	}
	if n > maxCoreServiceLogTail {
		n = maxCoreServiceLogTail
	}

	return n, nil
}

func readJournalctlUnitLogs(ctx context.Context, unit string, tail int) ([]string, error) {
	if strings.TrimSpace(unit) == "" {
		return nil, errors.New("unit is required")
	}

	cmd := exec.CommandContext(
		ctx,
		"journalctl",
		"--no-pager",
		"--output=short-iso",
		"-u",
		unit,
		"-n",
		strconv.Itoa(tail),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, context.Canceled
		}
		return nil, fmt.Errorf("journalctl: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return []string{}, nil
	}

	return strings.Split(trimmed, "\n"), nil
}
