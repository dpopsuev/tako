package stubs

import (
	"context"
	"fmt"
	"sync"

	"github.com/dpopsuev/tako/engine/gate"
)

// compile-time checks.
var (
	_ gate.ApprovalStore = (*MemoryApprovalStore)(nil)
	_ gate.Notifier      = (*StubNotifier)(nil)
)

// MemoryApprovalStore is an in-memory ApprovalStore for testing.
// Thread-safe. Satisfies RunApprovalStoreContract.
type MemoryApprovalStore struct {
	mu    sync.Mutex
	items map[string]*gate.ApprovalItem
}

// NewMemoryApprovalStore creates an empty in-memory store.
func NewMemoryApprovalStore() *MemoryApprovalStore {
	return &MemoryApprovalStore{items: make(map[string]*gate.ApprovalItem)}
}

func (s *MemoryApprovalStore) Park(_ context.Context, item gate.ApprovalItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := item
	s.items[item.ID] = &cp
	return nil
}

func (s *MemoryApprovalStore) Get(_ context.Context, id string) (*gate.ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", gate.ErrApprovalNotFound, id)
	}
	cp := *item
	return &cp, nil
}

func (s *MemoryApprovalStore) List(_ context.Context, status gate.ApprovalStatus) ([]gate.ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []gate.ApprovalItem
	for _, item := range s.items {
		if item.Status == status {
			result = append(result, *item)
		}
	}
	return result, nil
}

func (s *MemoryApprovalStore) Resolve(_ context.Context, id string, decision gate.Decision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("%w: %q", gate.ErrApprovalNotFound, id)
	}
	item.Status = decision.Status
	item.Decision = &decision
	return nil
}

func (s *MemoryApprovalStore) AddComment(_ context.Context, id string, comment gate.Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("%w: %q", gate.ErrApprovalNotFound, id)
	}
	if item.Status != gate.ApprovalPending {
		return fmt.Errorf("%w: %q", gate.ErrApprovalNotPending, id)
	}
	item.Comments = append(item.Comments, comment)
	return nil
}

// StubNotifier records notification calls for testing.
// Thread-safe, supports error injection.
type StubNotifier struct {
	mu    sync.Mutex
	calls []gate.ApprovalItem
	err   error
}

// NewStubNotifier creates a notifier that records calls.
func NewStubNotifier() *StubNotifier {
	return &StubNotifier{}
}

func (n *StubNotifier) Notify(_ context.Context, item gate.ApprovalItem) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, item)
	return n.err
}

// SetError injects an error for all subsequent Notify calls.
func (n *StubNotifier) SetError(err error) {
	n.mu.Lock()
	n.err = err
	n.mu.Unlock()
}

// Calls returns the items that were notified.
func (n *StubNotifier) Calls() []gate.ApprovalItem {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]gate.ApprovalItem, len(n.calls))
	copy(out, n.calls)
	return out
}

// CallCount returns how many times Notify was called.
func (n *StubNotifier) CallCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.calls)
}
