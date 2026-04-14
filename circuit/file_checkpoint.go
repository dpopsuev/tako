package circuit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileCheckpointer persists WalkerState as JSON files in a directory.
// Each walker gets its own file: {dir}/{walkerID}.json.
// Thread-safe for concurrent walkers with distinct IDs.
type FileCheckpointer struct {
	mu  sync.Mutex
	dir string
}

// NewFileCheckpointer creates a checkpointer that saves state to the given directory.
// Creates the directory if it doesn't exist.
func NewFileCheckpointer(dir string) (*FileCheckpointer, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create checkpoint dir: %w", err)
	}
	return &FileCheckpointer{dir: dir}, nil
}

// Save persists the walker state to disk atomically.
func (f *FileCheckpointer) Save(state *WalkerState) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	path := f.path(state.ID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename checkpoint: %w", err)
	}
	return nil
}

// Load restores a walker state from disk.
func (f *FileCheckpointer) Load(id string) (*WalkerState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, err := os.ReadFile(f.path(id))
	if err != nil {
		return nil, fmt.Errorf("read checkpoint %q: %w", id, err)
	}

	var state WalkerState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint %q: %w", id, err)
	}
	return &state, nil
}

// Remove deletes a checkpoint file.
func (f *FileCheckpointer) Remove(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.Remove(f.path(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove checkpoint %q: %w", id, err)
	}
	return nil
}

// List returns all checkpoint IDs (walker IDs with saved state).
func (f *FileCheckpointer) List() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
	}
	return ids, nil
}

func (f *FileCheckpointer) path(id string) string {
	return filepath.Join(f.dir, id+".json")
}

var _ Checkpointer = (*FileCheckpointer)(nil)
