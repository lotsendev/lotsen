package api

import (
	"errors"
	"net/http"

	"github.com/ercadev/dirigent/internal/events"
	"github.com/ercadev/dirigent/store"
)

func (h *Handler) restartDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := h.store.Get(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get deployment", http.StatusInternalServerError)
		return
	}

	updated, err := h.store.Patch(id, store.Deployment{Status: store.StatusDeploying})
	if err != nil {
		http.Error(w, "failed to restart deployment", http.StatusInternalServerError)
		return
	}

	h.events.Publish(events.StatusEvent{
		DeploymentID: id,
		Status:       string(store.StatusDeploying),
	})

	writeJSON(w, http.StatusAccepted, updated)
}
