package api

import "net/http"

import "github.com/ercadev/dirigent/store"

func (h *Handler) listDeployments(w http.ResponseWriter, _ *http.Request) {
	deployments, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to list deployments", http.StatusInternalServerError)
		return
	}
	if deployments == nil {
		deployments = []store.Deployment{}
	}
	writeJSON(w, http.StatusOK, deployments)
}
