package prompt

import (
	"fmt"
	"sync"
)

// LiveStore is an in-memory Store with full CRUD and version history.
// Designed for auto-tune loops where prompts are edited at runtime without
// fold/deploy cycles. Each update increments the version and preserves history
// for rollback.
type LiveStore struct {
	mu      sync.RWMutex
	prompts map[string]*promptEntry
}

type promptEntry struct {
	current *Prompt
	history []*Prompt // previous versions (oldest first)
}

// NewLiveStore creates an empty LiveStore.
func NewLiveStore() *LiveStore {
	return &LiveStore{prompts: make(map[string]*promptEntry)}
}

// NewLiveStoreFrom seeds a LiveStore from an existing Store (typically a FileStore).
// All prompts are loaded as version 1.
func NewLiveStoreFrom(source Store) (*LiveStore, error) {
	all, err := source.List()
	if err != nil {
		return nil, fmt.Errorf("seed live store: %w", err)
	}
	ls := NewLiveStore()
	for _, p := range all {
		cp := *p
		cp.Version = 1
		ls.prompts[cp.Name] = &promptEntry{current: &cp}
	}
	return ls, nil
}

func (s *LiveStore) Get(name string) (*Prompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.prompts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	cp := *entry.current
	return &cp, nil
}

func (s *LiveStore) List() ([]*Prompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Prompt, 0, len(s.prompts))
	for _, entry := range s.prompts {
		cp := *entry.current
		result = append(result, &cp)
	}
	return result, nil
}

func (s *LiveStore) Update(name, content string) (*Prompt, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if content == "" {
		return nil, ErrContentEmpty
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.prompts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}

	// Archive current version.
	prev := *entry.current
	entry.history = append(entry.history, &prev)

	// Create new version.
	entry.current = &Prompt{
		Name:     name,
		Step:     prev.Step,
		Version:  prev.Version + 1,
		Content:  content,
		Sections: ParseSections(content),
		Meta:     prev.Meta,
	}

	cp := *entry.current
	return &cp, nil
}

func (s *LiveStore) Create(name, step, content string) (*Prompt, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if content == "" {
		return nil, ErrContentEmpty
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.prompts[name]; exists {
		return nil, fmt.Errorf("%w: %q", ErrAlreadyExists, name)
	}

	p := &Prompt{
		Name:     name,
		Step:     step,
		Version:  1,
		Content:  content,
		Sections: ParseSections(content),
	}
	s.prompts[name] = &promptEntry{current: p}

	cp := *p
	return &cp, nil
}

func (s *LiveStore) Rollback(name string, version int) (*Prompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.prompts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}

	// Find the requested version in history.
	var target *Prompt
	for _, h := range entry.history {
		if h.Version == version {
			target = h
			break
		}
	}
	if target == nil && entry.current.Version == version {
		// Already at this version.
		cp := *entry.current
		return &cp, nil
	}
	if target == nil {
		return nil, fmt.Errorf("%w: %q v%d", ErrVersionMissing, name, version)
	}

	// Archive current, restore target at new version number.
	prev := *entry.current
	entry.history = append(entry.history, &prev)

	restored := *target
	restored.Version = prev.Version + 1
	restored.Sections = ParseSections(restored.Content)
	entry.current = &restored

	cp := *entry.current
	return &cp, nil
}

// History returns all versions of a prompt (oldest first), including current.
func (s *LiveStore) History(name string) ([]*Prompt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.prompts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	result := make([]*Prompt, 0, len(entry.history)+1)
	for _, h := range entry.history {
		cp := *h
		result = append(result, &cp)
	}
	cp := *entry.current
	result = append(result, &cp)
	return result, nil
}
