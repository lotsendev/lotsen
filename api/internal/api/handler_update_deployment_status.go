package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ercadev/dirigent/internal/events"
	"github.com/ercadev/dirigent/store"
)

func (h *Handler) updateDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Status store.Status `json:"status"`
		Error  string       `json:"error"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	switch body.Status {
	case store.StatusIdle, store.StatusDeploying, store.StatusHealthy, store.StatusFailed:
		// valid
	default:
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	d, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get deployment", http.StatusInternalServerError)
		return
	}

	d.Status = body.Status
	d.Error = body.Error
	updated, err := h.store.Update(d)
	if err != nil {
		http.Error(w, "failed to update deployment", http.StatusInternalServerError)
		return
	}

	h.events.Publish(events.StatusEvent{
		DeploymentID: id,
		Status:       string(body.Status),
		Error:        body.Error,
	})

	writeJSON(w, http.StatusOK, updated)
}
