package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// deploymentEvents streams deployment status-change events as SSE.
// The stream stays open until the client disconnects.
func (h *Handler) deploymentEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, cancel := h.events.Subscribe()
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("deploymentEvents: marshal event: %v", err)
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				log.Printf("deploymentEvents: write: %v", err)
				return
			}
			flusher.Flush()
		}
	}
}
