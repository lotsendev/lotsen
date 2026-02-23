package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/ercadev/dirigent/store"
)

// deploymentLogs streams container log lines for a deployment as SSE.
// The last 100 lines are sent immediately on connect; new lines are pushed as
// they appear. The stream stays open until the client disconnects or the
// container exits.
func (h *Handler) deploymentLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	id := r.PathValue("id")

	if _, err := h.store.Get(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get deployment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	rc, err := h.dockerLogs.StreamLogs(r.Context(), id, 100)
	if err != nil {
		log.Printf("deploymentLogs: stream logs for %s: %v", id, err)
		return
	}
	if rc == nil {
		return
	}
	defer rc.Close()

	logLine := struct {
		Line string `json:"line"`
	}{}

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		logLine.Line = scanner.Text()
		data, err := json.Marshal(logLine)
		if err != nil {
			log.Printf("deploymentLogs: marshal line: %v", err)
			continue
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			log.Printf("deploymentLogs: write: %v", err)
			return
		}
		flusher.Flush()
	}
}
