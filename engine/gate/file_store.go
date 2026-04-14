package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore is a durable ApprovalStore backed by a JSON file.
// Thread-safe. Writes atomically via temp file + rename.
// Suitable for single-process circuits. For multi-process,
// use SQLite or a proper database.
type FileStore struct {
	mu   sync.Mutex
	path string
	data map[string]*ApprovalItem
}

// NewFileStore creates a durable approval store at the given path.
// If the file exists, it loads existing items. If not, starts empty.
func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{
		path: path,
		data: make(map[string]*ApprovalItem),
	}

	// Load existing data if file exists.
	raw, err := os.ReadFile(path)
	if err == nil && len(raw) > 0 {
		var items map[string]*ApprovalItem
		if json.Unmarshal(raw, &items) == nil {
			s.data = items
		}
	}

	return s, nil
}

// Park stores an approval item and persists to disk.
func (s *FileStore) Park(_ context.Context, item ApprovalItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := item
	s.data[item.ID] = &cp
	return s.flush()
}

// Get retrieves an approval item by ID.
func (s *FileStore) Get(_ context.Context, id string) (*ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrApprovalNotFound, id)
	}
	cp := *item
	return &cp, nil
}

// List returns all items matching the given status.
func (s *FileStore) List(_ context.Context, status ApprovalStatus) ([]ApprovalItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []ApprovalItem
	for _, item := range s.data {
		if item.Status == status {
			result = append(result, *item)
		}
	}
	return result, nil
}

// Resolve updates an item's status and decision, and persists to disk.
func (s *FileStore) Resolve(_ context.Context, id string, decision Decision) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.data[id]
	if !ok {
		return fmt.Errorf("%w: %q", ErrApprovalNotFound, id)
	}
	item.Status = decision.Status
	item.Decision = &decision
	return s.flush()
}

// flush writes the current state to disk atomically (temp file + rename).
func (s *FileStore) flush() error {
	raw, err := json.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("marshal approval store: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return fmt.Errorf("mkdir for approval store: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write approval store: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename approval store: %w", err)
	}
	return nil
}

var _ ApprovalStore = (*FileStore)(nil)
