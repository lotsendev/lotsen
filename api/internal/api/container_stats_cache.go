package api

import "sync"

type ContainerStats struct {
	CPUPercent       float64 `json:"cpuPercent"`
	MemoryUsedBytes  uint64  `json:"memoryUsedBytes"`
	MemoryLimitBytes uint64  `json:"memoryLimitBytes"`
	MemoryPercent    float64 `json:"memoryPercent"`
}

type ContainerStatsCache struct {
	mu             sync.RWMutex
	byDeploymentID map[string]ContainerStats
}

func NewContainerStatsCache() *ContainerStatsCache {
	return &ContainerStatsCache{byDeploymentID: make(map[string]ContainerStats)}
}

func (c *ContainerStatsCache) ReplaceAll(stats map[string]ContainerStats) {
	if c == nil {
		return
	}

	next := make(map[string]ContainerStats, len(stats))
	for deploymentID, stat := range stats {
		next[deploymentID] = stat
	}

	c.mu.Lock()
	c.byDeploymentID = next
	c.mu.Unlock()
}

func (c *ContainerStatsCache) Get(deploymentID string) (ContainerStats, bool) {
	if c == nil {
		return ContainerStats{}, false
	}

	c.mu.RLock()
	stat, ok := c.byDeploymentID[deploymentID]
	c.mu.RUnlock()

	return stat, ok
}

func (c *ContainerStatsCache) Snapshot() map[string]ContainerStats {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	out := make(map[string]ContainerStats, len(c.byDeploymentID))
	for deploymentID, stat := range c.byDeploymentID {
		out[deploymentID] = stat
	}
	c.mu.RUnlock()

	return out
}
