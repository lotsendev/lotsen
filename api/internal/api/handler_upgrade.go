package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ercadev/lotsen/internal/upgrade"
)

func (h *Handler) startUpgrade(w http.ResponseWriter, r *http.Request) {
	targetVersion := ""
	if r.ContentLength > 0 {
		var body struct {
			TargetVersion string `json:"targetVersion"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid upgrade payload", http.StatusBadRequest)
			return
		}
		targetVersion = strings.TrimSpace(body.TargetVersion)
	}

	if targetVersion == "" {
		targetVersion = "latest"
	}

	snapshot, err := h.versions.Snapshot(r.Context())
	if err != nil {
		log.Printf("startUpgrade: version snapshot failed: %v", err)
	}
	if targetVersion == "latest" && snapshot.LatestVersion != "" {
		targetVersion = snapshot.LatestVersion
	}

	if err := h.upgrade.Start(targetVersion); err != nil {
		if errors.Is(err, upgrade.ErrAlreadyRunning) {
			http.Error(w, "upgrade already in progress", http.StatusConflict)
			return
		}
		log.Printf("startUpgrade: start failed: %v", err)
		http.Error(w, "failed to start upgrade", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "running"})
}

func (h *Handler) upgradeLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	lines, unsubscribe, err := h.upgrade.Subscribe()
	if err != nil {
		if errors.Is(err, upgrade.ErrNotRunning) {
			http.Error(w, "no upgrade in progress", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to subscribe upgrade logs", http.StatusInternalServerError)
		return
	}
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	payload := struct {
		Line string `json:"line"`
	}{}

	for {
		select {
		case <-r.Context().Done():
			return
		case line, ok := <-lines:
			if !ok {
				return
			}
			payload.Line = line
			data, err := json.Marshal(payload)
			if err != nil {
				log.Printf("upgradeLogs: marshal line: %v", err)
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				log.Printf("upgradeLogs: write: %v", err)
				return
			}
			flusher.Flush()
		}
	}
}
