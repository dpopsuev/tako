package resource

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dpopsuev/origami/circuit"
)

// Store errors.
var (
	ErrStoreNotFound      = errors.New("resource not found in store")
	ErrStoreAlreadyExists = errors.New("resource already exists in store")
	ErrStoreReadOnly      = errors.New("store is read-only")
)

// Store provides kind-generic CRUD with versioning for any resource.
// Generalizes prompt.Store to work with any registered kind.
type Store interface {
	// Get returns a resource by kind and name.
	Get(kind circuit.Kind, name string) (*Resource, error)

	// List returns all resources, optionally filtered by kind.
	List(kind circuit.Kind) ([]*Resource, error)

	// Update replaces resource content and increments version.
	Update(kind circuit.Kind, name string, data []byte) (*Resource, error)

	// Create adds a new resource.
	Create(kind circuit.Kind, name string, data []byte) (*Resource, error)

	// Rollback reverts a resource to a previous version.
	Rollback(kind circuit.Kind, name string, version int) (*Resource, error)
}

// storeEntry holds a resource with version history.
type storeEntry struct {
	current *Resource
	rawData []byte      // current raw bytes
	history []storeSnap // previous versions
}

type storeSnap struct {
	version int
	data    []byte
}

// LiveStore is an in-memory Store with full CRUD and version history
// for any resource kind. Designed for auto-tune loops.
type LiveStore struct {
	mu      sync.RWMutex
	reg     *KindRegistry
	entries map[string]*storeEntry // key: "kind/name"
}

// NewLiveStore creates an empty LiveStore backed by a KindRegistry.
func NewLiveStore(reg *KindRegistry) *LiveStore {
	return &LiveStore{
		reg:     reg,
		entries: make(map[string]*storeEntry),
	}
}

// SeedFrom loads all resources from a ResourceIndex into the store.
func (s *LiveStore) SeedFrom(idx *ResourceIndex) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, res := range idx.All() {
		key := storeKey(res.Kind, res.Metadata.Name)
		cp := *res
		cp.Version = "1"
		s.entries[key] = &storeEntry{
			current: &cp,
			rawData: res.Raw,
		}
	}
}

func (s *LiveStore) Get(kind circuit.Kind, name string) (*Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[storeKey(kind, name)]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrStoreNotFound, kind, name)
	}
	cp := *entry.current
	cp.Raw = entry.rawData
	return &cp, nil
}

func (s *LiveStore) List(kind circuit.Kind) ([]*Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Resource, 0, len(s.entries))
	for _, entry := range s.entries {
		if kind != "" && entry.current.Kind != kind {
			continue
		}
		cp := *entry.current
		result = append(result, &cp)
	}
	return result, nil
}

func (s *LiveStore) Update(kind circuit.Kind, name string, data []byte) (*Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := storeKey(kind, name)
	entry, ok := s.entries[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrStoreNotFound, kind, name)
	}

	// Archive current.
	ver := parseVersion(entry.current.Version)
	entry.history = append(entry.history, storeSnap{version: ver, data: entry.rawData})

	// Parse new data via registry.
	res, _, err := Load(s.reg, data, entry.current.Source)
	if err != nil {
		return nil, fmt.Errorf("update parse: %w", err)
	}

	newVer := ver + 1
	res.Version = fmt.Sprintf("%d", newVer)
	entry.current = res
	entry.rawData = data

	cp := *res
	return &cp, nil
}

func (s *LiveStore) Create(kind circuit.Kind, name string, data []byte) (*Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := storeKey(kind, name)
	if _, exists := s.entries[key]; exists {
		return nil, fmt.Errorf("%w: %s/%s", ErrStoreAlreadyExists, kind, name)
	}

	res, _, err := Load(s.reg, data, "")
	if err != nil {
		return nil, fmt.Errorf("create parse: %w", err)
	}
	res.Version = "1"

	s.entries[key] = &storeEntry{
		current: res,
		rawData: data,
	}

	cp := *res
	return &cp, nil
}

func (s *LiveStore) Rollback(kind circuit.Kind, name string, version int) (*Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := storeKey(kind, name)
	entry, ok := s.entries[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrStoreNotFound, kind, name)
	}

	// Find target version in history.
	var targetData []byte
	for _, snap := range entry.history {
		if snap.version == version {
			targetData = snap.data
			break
		}
	}
	if targetData == nil {
		return nil, fmt.Errorf("%w: %s/%s v%d", ErrStoreNotFound, kind, name, version)
	}

	// Archive current, restore target.
	curVer := parseVersion(entry.current.Version)
	entry.history = append(entry.history, storeSnap{version: curVer, data: entry.rawData})

	res, _, err := Load(s.reg, targetData, entry.current.Source)
	if err != nil {
		return nil, fmt.Errorf("rollback parse: %w", err)
	}

	newVer := curVer + 1
	res.Version = fmt.Sprintf("%d", newVer)
	entry.current = res
	entry.rawData = targetData

	cp := *res
	return &cp, nil
}

// History returns all versions for a resource (oldest first + current).
func (s *LiveStore) History(kind circuit.Kind, name string) ([]*Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := storeKey(kind, name)
	entry, ok := s.entries[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrStoreNotFound, kind, name)
	}
	result := make([]*Resource, 0, len(entry.history)+1)
	for _, snap := range entry.history {
		r, _, err := Load(s.reg, snap.data, "")
		if err != nil {
			continue
		}
		r.Version = fmt.Sprintf("%d", snap.version)
		result = append(result, r)
	}
	cp := *entry.current
	result = append(result, &cp)
	return result, nil
}

func storeKey(kind circuit.Kind, name string) string {
	return string(kind) + "/" + name
}

func parseVersion(v string) int {
	var n int
	_, _ = fmt.Sscanf(v, "%d", &n)
	if n == 0 {
		return 1
	}
	return n
}
