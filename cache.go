package framework

// Category: Execution — aliases to core/ (interface) and state/ (implementation).

import (
	"time"

	"github.com/dpopsuev/origami/core"
	"github.com/dpopsuev/origami/state"
)

// NodeCache stores and retrieves node output artifacts by cache key.
type NodeCache = core.NodeCache

// InMemoryCache is a thread-safe in-memory NodeCache with TTL-based lazy eviction.
type InMemoryCache = state.InMemoryCache

// NewInMemoryCache creates a new in-memory node cache.
func NewInMemoryCache() *InMemoryCache { return state.NewInMemoryCache() }

// cachePolicy configures caching behavior for a node via the DSL.
type cachePolicy struct {
	TTL     time.Duration            `yaml:"ttl,omitempty"`
	KeyFunc func(NodeContext) string `yaml:"-"`
}

// eventNodeCacheHit is emitted when a cached artifact is returned instead of
// processing the node.
const eventNodeCacheHit WalkEventType = "node_cache_hit"
