package api

import (
	"errors"
	"net/http"

	"github.com/ercadev/dirigent/store"
)

func (h *Handler) getDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	d, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get deployment", http.StatusInternalServerError)
		return
	}

	response := deploymentResponse{Deployment: normalizeDeploymentSecurity(d)}
	if h.containerStats != nil {
		if stats, ok := h.containerStats.Get(d.ID); ok {
			statsCopy := stats
			response.Stats = &statsCopy
		}
	}

	writeJSON(w, http.StatusOK, response)
}
