package tracker

import (
	"sync"
	"time"
)

const clubStatsCacheTTL = 24 * time.Hour

type clubStatsCache struct {
	mu      sync.RWMutex
	entries map[string]clubStatsCacheEntry
}

type clubStatsCacheEntry struct {
	response  APIClubStatsResponse
	expiresAt time.Time
}

func newClubStatsCache() *clubStatsCache {
	return &clubStatsCache{
		entries: make(map[string]clubStatsCacheEntry),
	}
}

func (c *clubStatsCache) get(clubID string) (APIClubStatsResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[clubID]
	if !ok || time.Now().After(entry.expiresAt) {
		return APIClubStatsResponse{}, false
	}

	return entry.response, true
}

func (c *clubStatsCache) set(clubID string, response APIClubStatsResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[clubID] = clubStatsCacheEntry{
		response:  response,
		expiresAt: time.Now().Add(clubStatsCacheTTL),
	}
}
