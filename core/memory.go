package core

// Category: Execution

import "time"

// MemoryStore provides cross-walk, identity-scoped key-value persistence.
// Walker identity (walkerID) is the scoping dimension: each walker has
// its own namespace. Values persist across multiple graph walks when
// the same MemoryStore instance is reused.
//
// The namespace-aware methods (GetNS, SetNS, KeysNS, Search) add a second
// scoping dimension. The original Get/Set/Keys use a default namespace ("").
type MemoryStore interface {
	Get(walkerID, key string) (any, bool)
	Set(walkerID, key string, value any)
	Keys(walkerID string) []string

	GetNS(namespace, walkerID, key string) (any, bool)
	SetNS(namespace, walkerID, key string, value any)
	KeysNS(namespace, walkerID string) []string
	Search(namespace, query string) []MemoryItem
}

// MemoryItem represents a stored memory entry with metadata.
type MemoryItem struct {
	Namespace string    `json:"namespace"`
	WalkerID  string    `json:"walker_id"`
	Key       string    `json:"key"`
	Value     any       `json:"value"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Conventional namespace constants for the three memory types.
const (
	NamespaceSemantic   = "semantic"
	NamespaceEpisodic   = "episodic"
	NamespaceProcedural = "procedural"
)
