package curate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileStore implements Store using JSON files in a directory.
type FileStore struct {
	Dir string
}

// NewFileStore creates a FileStore rooted at dir, creating it if needed.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("curate: create store dir: %w", err)
	}
	return &FileStore{Dir: dir}, nil
}

func (s *FileStore) List(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, fmt.Errorf("curate: list: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	return names, nil
}

func (s *FileStore) Load(_ context.Context, name string) (*Dataset, error) {
	path := filepath.Join(s.Dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("curate: load %q: %w", name, err)
	}
	var ds Dataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("curate: unmarshal %q: %w", name, err)
	}
	return &ds, nil
}

func (s *FileStore) Save(_ context.Context, d *Dataset) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("curate: marshal %q: %w", d.Name, err)
	}
	path := filepath.Join(s.Dir, d.Name+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("curate: write %q: %w", d.Name, err)
	}
	return nil
}
