package api

import (
	"log"
	"net/http"
	"time"
)

type versionResponse struct {
	CurrentVersion   string     `json:"currentVersion"`
	LatestVersion    string     `json:"latestVersion"`
	ReleaseNotes     string     `json:"releaseNotes"`
	PublishedAt      *time.Time `json:"publishedAt,omitempty"`
	UpgradeAvailable bool       `json:"upgradeAvailable"`
	CachedAt         *time.Time `json:"cachedAt,omitempty"`
}

func (h *Handler) getVersion(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.versions.Snapshot(r.Context())
	if err != nil {
		log.Printf("getVersion: check latest release: %v", err)
	}

	resp := versionResponse{
		CurrentVersion:   snapshot.CurrentVersion,
		LatestVersion:    snapshot.LatestVersion,
		ReleaseNotes:     snapshot.ReleaseNotes,
		UpgradeAvailable: snapshot.UpgradeAvailable,
	}

	if !snapshot.PublishedAt.IsZero() {
		publishedAt := snapshot.PublishedAt.UTC()
		resp.PublishedAt = &publishedAt
	}
	if !snapshot.CachedAt.IsZero() {
		cachedAt := snapshot.CachedAt.UTC()
		resp.CachedAt = &cachedAt
	}

	writeJSON(w, http.StatusOK, resp)
}
