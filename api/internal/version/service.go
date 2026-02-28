package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var latestReleaseURL = "https://api.github.com/repos/ercadev/dirigent-releases/releases/latest"
var releasesURL = "https://api.github.com/repos/ercadev/dirigent-releases/releases"

type Snapshot struct {
	CurrentVersion   string
	LatestVersion    string
	ReleaseNotes     string
	PublishedAt      time.Time
	UpgradeAvailable bool
	CachedAt         time.Time
}

type Release struct {
	TagName     string
	Body        string
	PublishedAt time.Time
}

type Service struct {
	currentVersion string
	client         *http.Client
	now            func() time.Time
	ttl            time.Duration

	mu     sync.RWMutex
	cached cachedRelease
	hasHit bool
}

type cachedRelease struct {
	latestVersion string
	releaseNotes  string
	publishedAt   time.Time
	cachedAt      time.Time
}

type latestReleaseResponse struct {
	TagName     string `json:"tag_name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
}

type releaseResponse struct {
	TagName     string `json:"tag_name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}

type latestRelease struct {
	TagName     string
	Body        string
	PublishedAt time.Time
}

func New(currentVersion string) *Service {
	if currentVersion == "" {
		currentVersion = "dev"
	}

	return &Service{
		currentVersion: currentVersion,
		client:         &http.Client{Timeout: 10 * time.Second},
		now:            time.Now,
		ttl:            time.Hour,
	}
}

func NewWithOptions(currentVersion string, client *http.Client, now func() time.Time, ttl time.Duration) *Service {
	if currentVersion == "" {
		currentVersion = "dev"
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if now == nil {
		now = time.Now
	}
	if ttl <= 0 {
		ttl = time.Hour
	}

	return &Service{currentVersion: currentVersion, client: client, now: now, ttl: ttl}
}

func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	s.mu.RLock()
	if s.hasHit && s.now().Sub(s.cached.cachedAt) < s.ttl {
		snap := s.snapshotFromCache(s.cached)
		s.mu.RUnlock()
		return snap, nil
	}
	stale := s.cached
	hasStale := s.hasHit
	s.mu.RUnlock()

	release, err := s.fetchLatestRelease(ctx)
	if err != nil {
		if hasStale {
			return s.snapshotFromCache(stale), nil
		}
		return Snapshot{
			CurrentVersion:   s.currentVersion,
			UpgradeAvailable: false,
		}, err
	}

	cached := cachedRelease{
		latestVersion: release.TagName,
		releaseNotes:  release.Body,
		publishedAt:   release.PublishedAt,
		cachedAt:      s.now().UTC(),
	}

	s.mu.Lock()
	s.cached = cached
	s.hasHit = true
	s.mu.Unlock()

	return s.snapshotFromCache(cached), nil
}

func (s *Service) snapshotFromCache(cached cachedRelease) Snapshot {
	currentVersion := s.currentVersion
	upgradeAvailable := upgradeAvailable(currentVersion, cached.latestVersion)

	return Snapshot{
		CurrentVersion:   currentVersion,
		LatestVersion:    cached.latestVersion,
		ReleaseNotes:     cached.releaseNotes,
		PublishedAt:      cached.publishedAt,
		UpgradeAvailable: upgradeAvailable,
		CachedAt:         cached.cachedAt,
	}
}

func (s *Service) fetchLatestRelease(ctx context.Context) (latestRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return latestRelease{}, fmt.Errorf("new request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return latestRelease{}, fmt.Errorf("call github latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return latestRelease{}, fmt.Errorf("unexpected github status: %d", resp.StatusCode)
	}

	var payload latestReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return latestRelease{}, fmt.Errorf("decode github latest release: %w", err)
	}

	release := latestRelease{
		TagName: strings.TrimSpace(payload.TagName),
		Body:    strings.TrimSpace(payload.Body),
	}
	if release.TagName == "" {
		return latestRelease{}, fmt.Errorf("github latest release missing tag_name")
	}

	if payload.PublishedAt != "" {
		at, err := time.Parse(time.RFC3339, payload.PublishedAt)
		if err != nil {
			return latestRelease{}, fmt.Errorf("parse published_at: %w", err)
		}
		release.PublishedAt = at.UTC()
	}

	return release, nil
}

func (s *Service) Releases(ctx context.Context, limit int) ([]Release, error) {
	if limit <= 0 {
		limit = 25
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?per_page=%d", releasesURL, limit), nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call github releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected github status: %d", resp.StatusCode)
	}

	var payload []releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github releases: %w", err)
	}

	releases := make([]Release, 0, len(payload))
	for _, item := range payload {
		if item.Draft || item.Prerelease {
			continue
		}

		tag := strings.TrimSpace(item.TagName)
		if tag == "" {
			continue
		}

		release := Release{
			TagName: tag,
			Body:    strings.TrimSpace(item.Body),
		}

		if item.PublishedAt != "" {
			at, err := time.Parse(time.RFC3339, item.PublishedAt)
			if err != nil {
				continue
			}
			release.PublishedAt = at.UTC()
		}

		releases = append(releases, release)
	}

	return releases, nil
}

func upgradeAvailable(currentVersion, latestVersion string) bool {
	if currentVersion == "" || latestVersion == "" {
		return false
	}
	if currentVersion == "dev" {
		return false
	}

	current, ok := parseSemver(currentVersion)
	if !ok {
		return false
	}
	latest, ok := parseSemver(latestVersion)
	if !ok {
		return false
	}

	if current[0] != latest[0] {
		return current[0] < latest[0]
	}
	if current[1] != latest[1] {
		return current[1] < latest[1]
	}
	return current[2] < latest[2]
}

func parseSemver(raw string) ([3]int, bool) {
	var out [3]int
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "v"))
	if raw == "" {
		return out, false
	}

	parts := strings.Split(raw, ".")
	if len(parts) < 3 {
		return out, false
	}

	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}

	return out, true
}
