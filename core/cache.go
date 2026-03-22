package core

// Category: Execution

import "time"

// NodeCache stores and retrieves node output artifacts by cache key.
type NodeCache interface {
	Get(key string) (Artifact, bool)
	Set(key string, art Artifact, ttl time.Duration)
}
