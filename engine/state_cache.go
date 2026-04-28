package engine

// Category: Execution

import (
	"sync"
	"time"

	"github.com/dpopsuev/tako/circuit"
)

// InMemoryCache is a thread-safe in-memory NodeCache with TTL-based lazy eviction.
type InMemoryCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

type cacheEntry struct {
	art       circuit.Artifact
	expiresAt time.Time
}

// NewInMemoryCache creates a new in-memory node cache.
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{items: make(map[string]cacheEntry)}
}

func (c *InMemoryCache) Get(key string) (circuit.Artifact, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}
	return e.art, true
}

func (c *InMemoryCache) Set(key string, art circuit.Artifact, ttl time.Duration) {
	c.mu.Lock()
	var expires time.Time
	if ttl > 0 {
		expires = time.Now().Add(ttl)
	}
	c.items[key] = cacheEntry{art: art, expiresAt: expires}
	c.mu.Unlock()
}

// Len returns the number of entries (including expired, not yet evicted).
func (c *InMemoryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
