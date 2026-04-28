package engine

// Category: Execution

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/tako/circuit"
)

// InMemoryStoreOption configures an InMemoryStore.
type InMemoryStoreOption func(*InMemoryStore)

// WithEmbeddings enables vector-similarity search using the given provider.
// When set, Search computes cosine similarity against stored embeddings
// instead of falling back to substring matching.
func WithEmbeddings(p circuit.EmbeddingProvider) InMemoryStoreOption {
	return func(s *InMemoryStore) {
		s.embedder = p
	}
}

// InMemoryStore is a thread-safe in-process MemoryStore with namespace support.
type InMemoryStore struct {
	mu         sync.RWMutex
	Data       map[string]map[string]map[string]circuit.MemoryItem // namespace -> walkerID -> key -> item
	embedder   circuit.EmbeddingProvider
	embeddings map[string][]float64 // "namespace:walkerID:key" -> vector
}

// NewInMemoryStore creates a ready-to-use InMemoryStore.
func NewInMemoryStore(opts ...InMemoryStoreOption) *InMemoryStore {
	s := &InMemoryStore{
		Data: make(map[string]map[string]map[string]circuit.MemoryItem),
	}
	for _, o := range opts {
		o(s)
	}
	if s.embedder != nil {
		s.embeddings = make(map[string][]float64)
	}
	return s
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
		slog.DebugContext(context.Background(), "memory get miss",
			slog.String(circuit.LogKeyNamespace, namespace),
			slog.String(circuit.LogKeyWalkerID, walkerID),
			slog.String(circuit.LogKeyName, key))
		return nil, false
	}
	wk := ns[walkerID]
	if wk == nil {
		slog.DebugContext(context.Background(), "memory get miss",
			slog.String(circuit.LogKeyNamespace, namespace),
			slog.String(circuit.LogKeyWalkerID, walkerID),
			slog.String(circuit.LogKeyName, key))
		return nil, false
	}
	item, ok := wk[key]
	if !ok {
		slog.DebugContext(context.Background(), "memory get miss",
			slog.String(circuit.LogKeyNamespace, namespace),
			slog.String(circuit.LogKeyWalkerID, walkerID),
			slog.String(circuit.LogKeyName, key))
		return nil, false
	}
	slog.DebugContext(context.Background(), "memory get hit",
		slog.String(circuit.LogKeyNamespace, namespace),
		slog.String(circuit.LogKeyWalkerID, walkerID),
		slog.String(circuit.LogKeyName, key))
	return item.Value, true
}

func (s *InMemoryStore) SetNS(namespace, walkerID, key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Data[namespace] == nil {
		s.Data[namespace] = make(map[string]map[string]circuit.MemoryItem)
	}
	if s.Data[namespace][walkerID] == nil {
		s.Data[namespace][walkerID] = make(map[string]circuit.MemoryItem)
	}
	s.Data[namespace][walkerID][key] = circuit.MemoryItem{
		Namespace: namespace,
		WalkerID:  walkerID,
		Key:       key,
		Value:     value,
		CreatedAt: time.Now(),
	}
	slog.DebugContext(context.Background(), "memory set",
		slog.String(circuit.LogKeyNamespace, namespace),
		slog.String(circuit.LogKeyWalkerID, walkerID),
		slog.String(circuit.LogKeyName, key))
	if s.embedder != nil {
		s.storeEmbedding(namespace, walkerID, key, value)
	}
}

// storeEmbedding computes and caches an embedding for the string
// representation of value. Must be called with s.mu held.
func (s *InMemoryStore) storeEmbedding(namespace, walkerID, key string, value any) {
	text := fmt.Sprintf("%v", value)
	vec, err := s.embedder.Embed(context.Background(), text)
	if err != nil {
		slog.WarnContext(context.Background(), "embedding failed",
			slog.String(circuit.LogKeyNamespace, namespace),
			slog.String(circuit.LogKeyWalkerID, walkerID),
			slog.String(circuit.LogKeyError, err.Error()))
		return
	}
	eKey := namespace + ":" + walkerID + ":" + key
	s.embeddings[eKey] = vec
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

// Search returns items matching query in the given namespace.
// When an EmbeddingProvider is configured, it uses cosine similarity
// to rank results. Otherwise it falls back to substring matching on
// keys, string values, and tags.
func (s *InMemoryStore) Search(namespace, query string) []circuit.MemoryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ns := s.Data[namespace]
	if ns == nil {
		return nil
	}
	if s.embedder != nil {
		return s.searchByEmbedding(namespace, ns, query)
	}
	return s.searchBySubstring(ns, query)
}

func (s *InMemoryStore) searchBySubstring(ns map[string]map[string]circuit.MemoryItem, query string) []circuit.MemoryItem {
	lower := strings.ToLower(query)
	var results []circuit.MemoryItem
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

type scoredItem struct {
	item  circuit.MemoryItem
	score float64
}

func (s *InMemoryStore) searchByEmbedding(namespace string, ns map[string]map[string]circuit.MemoryItem, query string) []circuit.MemoryItem {
	qVec, err := s.embedder.Embed(context.Background(), query)
	if err != nil {
		slog.WarnContext(context.Background(), "query embedding failed, falling back to substring",
			slog.String(circuit.LogKeyNamespace, namespace),
			slog.String(circuit.LogKeyQuery, query),
			slog.String(circuit.LogKeyError, err.Error()))
		return s.searchBySubstring(ns, query)
	}
	var scored []scoredItem
	prefix := namespace + ":"
	for eKey, vec := range s.embeddings {
		if !strings.HasPrefix(eKey, prefix) {
			continue
		}
		sim := cosineSimilarity(qVec, vec)
		// Split "namespace:walkerID:key" to locate the item.
		rest := eKey[len(prefix):]
		sep := strings.Index(rest, ":")
		if sep < 0 {
			continue
		}
		walkerID := rest[:sep]
		key := rest[sep+1:]
		if item, ok := ns[walkerID][key]; ok {
			scored = append(scored, scoredItem{item: item, score: sim})
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	results := make([]circuit.MemoryItem, len(scored))
	for i, si := range scored {
		results[i] = si.item
	}
	slog.DebugContext(context.Background(), "embedding search complete",
		slog.String(circuit.LogKeyNamespace, namespace),
		slog.String(circuit.LogKeyQuery, query),
		slog.Int(circuit.LogKeyResults, len(results)))
	return results
}

// cosineSimilarity returns the cosine similarity between two vectors.
// Returns 0 if either vector has zero magnitude.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

// SetNSTagged is like SetNS but also attaches tags to the memory item.
func (s *InMemoryStore) SetNSTagged(namespace, walkerID, key string, value any, tags []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Data[namespace] == nil {
		s.Data[namespace] = make(map[string]map[string]circuit.MemoryItem)
	}
	if s.Data[namespace][walkerID] == nil {
		s.Data[namespace][walkerID] = make(map[string]circuit.MemoryItem)
	}
	s.Data[namespace][walkerID][key] = circuit.MemoryItem{
		Namespace: namespace,
		WalkerID:  walkerID,
		Key:       key,
		Value:     value,
		Tags:      tags,
		CreatedAt: time.Now(),
	}
	if s.embedder != nil {
		s.storeEmbedding(namespace, walkerID, key, value)
	}
}

// TaggedSetter is implemented by MemoryStore backends that support tagged writes.
type TaggedSetter interface {
	SetNSTagged(namespace, walkerID, key string, value any, tags []string)
}

// TaggedMemoryStore wraps a MemoryStore and auto-appends tags to every SetNS call.
// Read operations are delegated unchanged.
type TaggedMemoryStore struct {
	Inner circuit.MemoryStore
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

func (t *TaggedMemoryStore) Search(namespace, query string) []circuit.MemoryItem {
	return t.Inner.Search(namespace, query)
}
