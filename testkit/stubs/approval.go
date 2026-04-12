package stubs

import (
	"context"
	"fmt"
	"sync"

	"github.com/dpopsuev/origami/engine"
)

// compile-time checks.
var (
	_ engine.ApprovalStore = (*MemoryApprovalStore)(nil)
	_ engine.Notifier      = (*StubNotifier)(nil)
)

// MemoryApprovalStore is an in-memory ApprovalStore for testing.
// Thread-safe. Satisfies RunApprovalStoreContract.
type MemoryApprovalStore struct {
	mu    sync.Mutex
	items map[string]*engine.ApprovalItem
}

// NewMemoryApprovalStore creates an empty in-memory store.
func NewMemoryApprovalStore() *MemoryApprovalStore {
	return &MemoryApprovalStore{items: make(map[string]*engine.ApprovalItem)}
}

func (s *MemoryApprovalStore) Park(_ context.Context, item engine.ApprovalItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := item
	s.items[item.ID] = &cp
	return nil
}

func (s *MemoryApprovalStore) Get(_ context.Context, id string) (*engine.ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", engine.ErrApprovalNotFound, id)
	}
	cp := *item
	return &cp, nil
}

func (s *MemoryApprovalStore) List(_ context.Context, status engine.ApprovalStatus) ([]engine.ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []engine.ApprovalItem
	for _, item := range s.items {
		if item.Status == status {
			result = append(result, *item)
		}
	}
	return result, nil
}

func (s *MemoryApprovalStore) Resolve(_ context.Context, id string, decision engine.Decision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("%w: %q", engine.ErrApprovalNotFound, id)
	}
	item.Status = decision.Status
	item.Decision = &decision
	return nil
}

// StubNotifier records notification calls for testing.
// Thread-safe, supports error injection.
type StubNotifier struct {
	mu    sync.Mutex
	calls []engine.ApprovalItem
	err   error
}

// NewStubNotifier creates a notifier that records calls.
func NewStubNotifier() *StubNotifier {
	return &StubNotifier{}
}

func (n *StubNotifier) Notify(_ context.Context, item engine.ApprovalItem) error {
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
func (n *StubNotifier) Calls() []engine.ApprovalItem {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]engine.ApprovalItem, len(n.calls))
	copy(out, n.calls)
	return out
}

// CallCount returns how many times Notify was called.
func (n *StubNotifier) CallCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.calls)
}
