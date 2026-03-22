package engine

// Category: Execution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/origami/circuit"
)

// JSONCheckpointer persists WalkerState to a JSON file between nodes,
// enabling resume-from-failure for circuits.
//
// This is a PoC battery — sufficient for prototyping, not production-grade.
// Consumers should replace it with their own checkpointing for production use.
type JSONCheckpointer struct {
	Dir string
}

// Compile-time interface check.
var _ circuit.Checkpointer = (*JSONCheckpointer)(nil)

// NewJSONCheckpointer creates a checkpointer that writes to the given directory.
func NewJSONCheckpointer(dir string) (*JSONCheckpointer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("checkpoint: create dir: %w", err)
	}
	return &JSONCheckpointer{Dir: dir}, nil
}

// Save persists the walker state to a JSON file named by the walker's ID.
func (c *JSONCheckpointer) Save(state *circuit.WalkerState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("checkpoint: marshal state: %w", err)
	}
	path := c.path(state.ID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("checkpoint: write %s: %w", path, err)
	}
	return nil
}

// Load restores a walker state from a previously saved checkpoint file.
// Returns nil and no error if no checkpoint exists for the given ID.
func (c *JSONCheckpointer) Load(id string) (*circuit.WalkerState, error) {
	path := c.path(id)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checkpoint: read %s: %w", path, err)
	}
	var state circuit.WalkerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("checkpoint: unmarshal %s: %w", path, err)
	}
	if state.LoopCounts == nil {
		state.LoopCounts = make(map[string]int)
	}
	if state.Context == nil {
		state.Context = make(map[string]any)
	}
	if state.Outputs == nil {
		state.Outputs = make(map[string]circuit.Artifact)
	}
	return &state, nil
}

// Remove deletes the checkpoint file for the given walker ID.
func (c *JSONCheckpointer) Remove(id string) error {
	path := c.path(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("checkpoint: remove %s: %w", path, err)
	}
	return nil
}

// List returns the IDs of all saved checkpoints.
func (c *JSONCheckpointer) List() ([]string, error) {
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return nil, fmt.Errorf("checkpoint: list %s: %w", c.Dir, err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".checkpoint.json") {
			ids = append(ids, strings.TrimSuffix(name, ".checkpoint.json"))
		}
	}
	return ids, nil
}

func (c *JSONCheckpointer) path(id string) string {
	return filepath.Join(c.Dir, id+".checkpoint.json")
}
