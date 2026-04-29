package andon

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrApprovalNotFound         = errors.New("approval not found")
	ErrApprovalStillPending     = errors.New("approval still pending")
	ErrUnexpectedApprovalStatus = errors.New("unexpected approval status")
	ErrApprovalNotPending       = errors.New("approval item is not pending")
)

type MemoryApprovalStore struct {
	mu    sync.Mutex
	items map[string]*ApprovalItem
}

var _ ApprovalStore = (*MemoryApprovalStore)(nil)

func NewMemoryApprovalStore() *MemoryApprovalStore {
	return &MemoryApprovalStore{items: make(map[string]*ApprovalItem)}
}

func (s *MemoryApprovalStore) Park(_ context.Context, item ApprovalItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := item
	s.items[item.ID] = &cp
	return nil
}

func (s *MemoryApprovalStore) Get(_ context.Context, id string) (*ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrApprovalNotFound, id)
	}
	cp := *item
	return &cp, nil
}

func (s *MemoryApprovalStore) List(_ context.Context, status ApprovalStatus) ([]ApprovalItem, error) {
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

func (s *MemoryApprovalStore) Resolve(_ context.Context, id string, decision Decision) error {
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

func (s *MemoryApprovalStore) AddComment(_ context.Context, id string, comment Comment) error {
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
