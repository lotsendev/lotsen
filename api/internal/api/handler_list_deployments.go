package api

import (
	"net/http"

	"github.com/lotsendev/lotsen/store"
)

type deploymentResponse struct {
	store.Deployment
	VolumeMounts []volumeMountResponse `json:"volume_mounts,omitempty"`
	Stats        *ContainerStats       `json:"stats,omitempty"`
}

func deploymentResponseFromStore(deployment store.Deployment, stats *ContainerStats) deploymentResponse {
	response := deploymentResponse{
		Deployment:   normalizeDeploymentSecurity(deployment),
		VolumeMounts: volumeMountsFromBindings(deployment.ID, deployment.Volumes),
	}
	if stats != nil {
		response.Stats = stats
	}
	return response
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
		response := deploymentResponseFromStore(deployment, nil)
		if h.containerStats != nil {
			if stats, ok := h.containerStats.Get(deployment.ID); ok {
				statsCopy := stats
				response = deploymentResponseFromStore(deployment, &statsCopy)
			}
		}
		responses = append(responses, response)
	}

	writeJSON(w, http.StatusOK, responses)
}
