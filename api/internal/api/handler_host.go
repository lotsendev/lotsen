package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const maxHostDisplayNameLength = 64

type HostProfile struct {
	DisplayName string `json:"displayName"`
}

type hostResponse struct {
	DisplayName string                    `json:"displayName"`
	Metadata    *HostMetadataSystemStatus `json:"metadata,omitempty"`
}

func (h *Handler) getHost(w http.ResponseWriter, r *http.Request) {
	profile := HostProfile{}
	if h.hostProfiles != nil {
		stored, err := h.hostProfiles.Get()
		if err != nil {
			http.Error(w, "failed to load host profile", http.StatusInternalServerError)
			return
		}
		profile = stored
	}

	snapshot := h.currentSystemStatusSnapshot(r)
	writeJSON(w, http.StatusOK, hostResponse{DisplayName: profile.DisplayName, Metadata: snapshot.Host.Metadata})
}

func (h *Handler) updateHost(w http.ResponseWriter, r *http.Request) {
	if h.hostProfiles == nil {
		http.Error(w, "host profile unavailable", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var body struct {
		DisplayName *string `json:"displayName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.DisplayName == nil {
		http.Error(w, "displayName is required", http.StatusBadRequest)
		return
	}

	displayName := strings.TrimSpace(*body.DisplayName)
	if len(displayName) > maxHostDisplayNameLength {
		http.Error(w, "displayName too long", http.StatusBadRequest)
		return
	}

	updated, err := h.hostProfiles.UpdateDisplayName(displayName)
	if err != nil {
		http.Error(w, "failed to update host profile", http.StatusInternalServerError)
		return
	}

	snapshot := h.currentSystemStatusSnapshot(r)
	writeJSON(w, http.StatusOK, hostResponse{DisplayName: updated.DisplayName, Metadata: snapshot.Host.Metadata})
}
