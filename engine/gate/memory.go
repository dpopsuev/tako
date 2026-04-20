package gate

import (
	"context"
	"fmt"
	"sync"
)

// MemoryStore is a thread-safe in-memory ApprovalStore.
// Suitable for single-process circuits. For durable persistence
// across restarts, use a file-backed or database store.
type MemoryStore struct {
	mu    sync.Mutex
	items map[string]*ApprovalItem
}

// NewMemoryStore creates an empty in-memory approval store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string]*ApprovalItem)}
}

// Park stores an approval item.
func (s *MemoryStore) Park(_ context.Context, item ApprovalItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := item
	s.items[item.ID] = &cp
	return nil
}

// Get retrieves an approval item by ID.
func (s *MemoryStore) Get(_ context.Context, id string) (*ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrApprovalNotFound, id)
	}
	cp := *item
	return &cp, nil
}

// List returns all items matching the given status.
func (s *MemoryStore) List(_ context.Context, status ApprovalStatus) ([]ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []ApprovalItem
	for _, item := range s.items {
		if item.Status == status {
			result = append(result, *item)
		}
	}
	return result, nil
}

// Resolve updates an item's status and decision.
func (s *MemoryStore) Resolve(_ context.Context, id string, decision Decision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("%w: %q", ErrApprovalNotFound, id)
	}
	item.Status = decision.Status
	item.Decision = &decision
	return nil
}

// AddComment appends a comment to a pending item.
func (s *MemoryStore) AddComment(_ context.Context, id string, comment Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("%w: %q", ErrApprovalNotFound, id)
	}
	if item.Status != ApprovalPending {
		return fmt.Errorf("%w: %q", ErrApprovalNotPending, id)
	}
	item.Comments = append(item.Comments, comment)
	return nil
}

var _ ApprovalStore = (*MemoryStore)(nil)
