package api

import (
	"net/http"

	"github.com/ercadev/dirigent/store"
)

type deploymentResponse struct {
	store.Deployment
	Stats *ContainerStats `json:"stats,omitempty"`
}

func (h *Handler) listDeployments(w http.ResponseWriter, _ *http.Request) {
	deployments, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to list deployments", http.StatusInternalServerError)
		return
	}
	if deployments == nil {
		deployments = []store.Deployment{}
	}

	responses := make([]deploymentResponse, 0, len(deployments))
	for _, deployment := range deployments {
		response := deploymentResponse{Deployment: normalizeDeploymentSecurity(deployment)}
		if h.containerStats != nil {
			if stats, ok := h.containerStats.Get(deployment.ID); ok {
				statsCopy := stats
				response.Stats = &statsCopy
			}
		}
		responses = append(responses, response)
	}

	writeJSON(w, http.StatusOK, responses)
}
