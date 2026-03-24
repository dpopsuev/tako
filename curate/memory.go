package curate

import (
	"context"
	"fmt"
	"sync"
)

// MemoryStore is an in-memory implementation of Store for tests and prototypes.
//
// This is a PoC battery — sufficient for prototyping, not production-grade.
// Consumers should replace it with their own Store for production use.
type MemoryStore struct {
	mu       sync.RWMutex
	datasets map[string]*Dataset
}

// NewMemoryStore creates an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{datasets: make(map[string]*Dataset)}
}

func (s *MemoryStore) List(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.datasets))
	for name := range s.datasets {
		names = append(names, name)
	}
	return names, nil
}

func (s *MemoryStore) Load(_ context.Context, name string) (*Dataset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.datasets[name]
	if !ok {
		return nil, fmt.Errorf("curate/memory: dataset %q not found", name)
	}
	return d, nil
}

func (s *MemoryStore) Save(_ context.Context, d *Dataset) error {
	if d.Name == "" {
		return fmt.Errorf("curate/memory: dataset name must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.datasets[d.Name] = d
	return nil
}
