package api

import (
	"log"
	"net/http"
	"strconv"
	"time"
)

type versionReleaseResponse struct {
	Version      string     `json:"version"`
	ReleaseNotes string     `json:"releaseNotes"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty"`
}

func (h *Handler) getVersionReleases(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 100 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	releases, err := h.versions.Releases(r.Context(), limit)
	if err != nil {
		log.Printf("getVersionReleases: fetch releases: %v", err)
		writeJSON(w, http.StatusOK, []versionReleaseResponse{})
		return
	}

	resp := make([]versionReleaseResponse, 0, len(releases))
	for _, release := range releases {
		entry := versionReleaseResponse{
			Version:      release.TagName,
			ReleaseNotes: release.Body,
		}
		if !release.PublishedAt.IsZero() {
			publishedAt := release.PublishedAt.UTC()
			entry.PublishedAt = &publishedAt
		}
		resp = append(resp, entry)
	}

	writeJSON(w, http.StatusOK, resp)
}
