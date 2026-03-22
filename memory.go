package framework

// Category: Execution — aliases to core/ (interface) and state/ (implementation).

import (
	"github.com/dpopsuev/origami/core"
	"github.com/dpopsuev/origami/state"
)

// MemoryStore provides cross-walk, identity-scoped key-value persistence.
type MemoryStore = core.MemoryStore

// MemoryItem represents a stored memory entry with metadata.
type MemoryItem = core.MemoryItem

// Conventional namespace constants for the three memory types.
const (
	NamespaceSemantic   = core.NamespaceSemantic
	NamespaceEpisodic   = core.NamespaceEpisodic
	NamespaceProcedural = core.NamespaceProcedural
)

// InMemoryStore is a thread-safe in-process MemoryStore with namespace support.
type InMemoryStore = state.InMemoryStore

// NewInMemoryStore creates a ready-to-use InMemoryStore.
func NewInMemoryStore() *InMemoryStore { return state.NewInMemoryStore() }

// taggedSetter is implemented by MemoryStore backends that support tagged writes.
type taggedSetter = state.TaggedSetter

// taggedMemoryStore wraps a MemoryStore and auto-appends tags to every SetNS call.
type taggedMemoryStore = state.TaggedMemoryStore

// --- Memory type helper functions (unexported, used by root tests) ---

// setFact stores a semantic fact about a walker.
func setFact(store MemoryStore, walkerID, key string, value any) {
	store.SetNS(NamespaceSemantic, walkerID, key, value)
}

// recordEpisode stores an episodic memory (a walk summary).
func recordEpisode(store MemoryStore, walkerID, walkID string, summary string) {
	store.SetNS(NamespaceEpisodic, walkerID, walkID, summary)
}

// updateInstruction stores a procedural memory (a prompt refinement).
func updateInstruction(store MemoryStore, walkerID, key string, instruction string) {
	store.SetNS(NamespaceProcedural, walkerID, key, instruction)
}
