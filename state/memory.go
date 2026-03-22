package state

// Category: Execution

import (
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/origami/core"
)

// InMemoryStore is a thread-safe in-process MemoryStore with namespace support.
type InMemoryStore struct {
	mu   sync.RWMutex
	Data map[string]map[string]map[string]core.MemoryItem // namespace -> walkerID -> key -> item
}

// NewInMemoryStore creates a ready-to-use InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		Data: make(map[string]map[string]map[string]core.MemoryItem),
	}
}

// --- Backward-compatible methods (default namespace "")  ---

func (s *InMemoryStore) Get(walkerID, key string) (any, bool) {
	return s.GetNS("", walkerID, key)
}

func (s *InMemoryStore) Set(walkerID, key string, value any) {
	s.SetNS("", walkerID, key, value)
}

func (s *InMemoryStore) Keys(walkerID string) []string {
	return s.KeysNS("", walkerID)
}

// --- Namespace-aware methods ---

func (s *InMemoryStore) GetNS(namespace, walkerID, key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ns := s.Data[namespace]
	if ns == nil {
		return nil, false
	}
	wk := ns[walkerID]
	if wk == nil {
		return nil, false
	}
	item, ok := wk[key]
	if !ok {
		return nil, false
	}
	return item.Value, true
}

func (s *InMemoryStore) SetNS(namespace, walkerID, key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Data[namespace] == nil {
		s.Data[namespace] = make(map[string]map[string]core.MemoryItem)
	}
	if s.Data[namespace][walkerID] == nil {
		s.Data[namespace][walkerID] = make(map[string]core.MemoryItem)
	}
	s.Data[namespace][walkerID][key] = core.MemoryItem{
		Namespace: namespace,
		WalkerID:  walkerID,
		Key:       key,
		Value:     value,
		CreatedAt: time.Now(),
	}
}

func (s *InMemoryStore) KeysNS(namespace, walkerID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ns := s.Data[namespace]
	if ns == nil {
		return nil
	}
	wk := ns[walkerID]
	if wk == nil {
		return nil
	}
	keys := make([]string, 0, len(wk))
	for k := range wk {
		keys = append(keys, k)
	}
	return keys
}

// Search does substring matching on keys and string values across all walkers
// in the given namespace.
func (s *InMemoryStore) Search(namespace, query string) []core.MemoryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ns := s.Data[namespace]
	if ns == nil {
		return nil
	}
	lower := strings.ToLower(query)
	var results []core.MemoryItem
	for _, wk := range ns {
		for _, item := range wk {
			if strings.Contains(strings.ToLower(item.Key), lower) {
				results = append(results, item)
				continue
			}
			if sv, ok := item.Value.(string); ok && strings.Contains(strings.ToLower(sv), lower) {
				results = append(results, item)
				continue
			}
			for _, tag := range item.Tags {
				if strings.Contains(strings.ToLower(tag), lower) {
					results = append(results, item)
					break
				}
			}
		}
	}
	return results
}

// SetNSTagged is like SetNS but also attaches tags to the memory item.
func (s *InMemoryStore) SetNSTagged(namespace, walkerID, key string, value any, tags []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Data[namespace] == nil {
		s.Data[namespace] = make(map[string]map[string]core.MemoryItem)
	}
	if s.Data[namespace][walkerID] == nil {
		s.Data[namespace][walkerID] = make(map[string]core.MemoryItem)
	}
	s.Data[namespace][walkerID][key] = core.MemoryItem{
		Namespace: namespace,
		WalkerID:  walkerID,
		Key:       key,
		Value:     value,
		Tags:      tags,
		CreatedAt: time.Now(),
	}
}

// TaggedSetter is implemented by MemoryStore backends that support tagged writes.
type TaggedSetter interface {
	SetNSTagged(namespace, walkerID, key string, value any, tags []string)
}

// TaggedMemoryStore wraps a MemoryStore and auto-appends tags to every SetNS call.
// Read operations are delegated unchanged.
type TaggedMemoryStore struct {
	Inner core.MemoryStore
	Tags  []string
}

func (t *TaggedMemoryStore) Get(walkerID, key string) (any, bool) {
	return t.Inner.Get(walkerID, key)
}

func (t *TaggedMemoryStore) Set(walkerID, key string, value any) {
	t.SetNS("", walkerID, key, value)
}

func (t *TaggedMemoryStore) Keys(walkerID string) []string {
	return t.Inner.Keys(walkerID)
}

func (t *TaggedMemoryStore) GetNS(namespace, walkerID, key string) (any, bool) {
	return t.Inner.GetNS(namespace, walkerID, key)
}

func (t *TaggedMemoryStore) SetNS(namespace, walkerID, key string, value any) {
	if ts, ok := t.Inner.(TaggedSetter); ok {
		ts.SetNSTagged(namespace, walkerID, key, value, t.Tags)
		return
	}
	t.Inner.SetNS(namespace, walkerID, key, value)
}

func (t *TaggedMemoryStore) KeysNS(namespace, walkerID string) []string {
	return t.Inner.KeysNS(namespace, walkerID)
}

func (t *TaggedMemoryStore) Search(namespace, query string) []core.MemoryItem {
	return t.Inner.Search(namespace, query)
}
