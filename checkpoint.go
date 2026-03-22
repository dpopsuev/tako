package framework

// Category: Execution — aliases to core/ (interface) and state/ (implementation).

import (
	"github.com/dpopsuev/origami/core"
	"github.com/dpopsuev/origami/state"
)

// Checkpointer persists and restores WalkerState between nodes, enabling
// resume-from-failure and crash recovery. Implementations must be safe
// for concurrent use by multiple walkers with distinct IDs.
type Checkpointer = core.Checkpointer

// JSONCheckpointer persists WalkerState to a JSON file between nodes,
// enabling resume-from-failure for circuits.
type JSONCheckpointer = state.JSONCheckpointer

// NewJSONCheckpointer creates a checkpointer that writes to the given directory.
func NewJSONCheckpointer(dir string) (*JSONCheckpointer, error) {
	return state.NewJSONCheckpointer(dir)
}
