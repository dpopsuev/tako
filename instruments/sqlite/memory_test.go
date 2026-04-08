package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

// mockEmbedder returns deterministic vectors for testing.
type mockEmbedder struct {
	vectors map[string][]float64
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	if v, ok := m.vectors[text]; ok {
		return v, nil
	}
	return []float64{0, 0, 0}, nil
}

func newTestStore(t *testing.T) *PersistentStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := NewMemoryStore(dbPath)
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPersistentStore_InterfaceCompliance(t *testing.T) {
	var _ circuit.MemoryStore = (*PersistentStore)(nil)
}

func TestPersistentStore_BackwardCompat(t *testing.T) {
	s := newTestStore(t)
	s.Set("w1", "key", "value")
	v, ok := s.Get("w1", "key")
	if !ok || v != "value" {
		t.Errorf("Get = %v / %v, want value/true", v, ok)
	}
}

func TestPersistentStore_NamespaceIsolation(t *testing.T) {
	s := newTestStore(t)
	s.SetNS("semantic", "w1", "k", "sem")
	s.SetNS("episodic", "w1", "k", "epi")

	v, _ := s.GetNS("semantic", "w1", "k")
	if v != "sem" {
		t.Errorf("semantic = %v, want sem", v)
	}
	v, _ = s.GetNS("episodic", "w1", "k")
	if v != "epi" {
		t.Errorf("episodic = %v, want epi", v)
	}
	_, ok := s.GetNS("procedural", "w1", "k")
	if ok {
		t.Error("procedural should not exist")
	}
}

func TestPersistentStore_KeysNS(t *testing.T) {
	s := newTestStore(t)
	s.SetNS("ns", "w1", "b", 2)
	s.SetNS("ns", "w1", "a", 1)

	keys := s.KeysNS("ns", "w1")
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("keys = %v, want [a b]", keys)
	}
}

func TestPersistentStore_Search(t *testing.T) {
	s := newTestStore(t)
	s.SetNS("semantic", "w1", "theme-preference", "dark mode")
	s.SetNS("semantic", "w1", "language", "english")

	results := s.Search("semantic", "theme")
	if len(results) != 1 {
		t.Fatalf("search 'theme': got %d, want 1", len(results))
	}
	if results[0].Key != "theme-preference" {
		t.Errorf("key = %q", results[0].Key)
	}
}

func TestPersistentStore_SearchByTag(t *testing.T) {
	s := newTestStore(t)
	s.SetNSTagged("semantic", "w1", "k1", "v1", []string{"rca", "ptp"})
	s.SetNSTagged("semantic", "w1", "k2", "v2", []string{"security"})

	results := s.Search("semantic", "ptp")
	if len(results) != 1 || results[0].Key != "k1" {
		t.Errorf("search ptp: %v", results)
	}
}

func TestPersistentStore_PersistsAcrossRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "persist.db")

	s1, _ := NewMemoryStore(dbPath)
	s1.SetNS("semantic", "w1", "pref", "dark")
	s1.Close()

	s2, _ := NewMemoryStore(dbPath)
	defer s2.Close()
	v, ok := s2.GetNS("semantic", "w1", "pref")
	if !ok || v != "dark" {
		t.Errorf("after restart: got %v / %v, want dark/true", v, ok)
	}
}

func TestPersistentStore_ConcurrentAccess(t *testing.T) {
	s := newTestStore(t)
	const workers = 10

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", n)
			s.SetNS("ns", "w1", key, n)
			s.GetNS("ns", "w1", key)
			s.KeysNS("ns", "w1")
		}(i)
	}
	wg.Wait()

	keys := s.KeysNS("ns", "w1")
	if len(keys) != workers {
		t.Errorf("expected %d keys, got %d", workers, len(keys))
	}
}

func newTestStoreWithEmbeddings(t *testing.T, emb circuit.EmbeddingProvider) *PersistentStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := NewMemoryStore(dbPath, WithPersistentEmbeddings(emb))
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPersistentStore_SearchSubstringFallback(t *testing.T) {
	// No embedder — uses LIKE-based substring matching.
	s := newTestStore(t)
	s.SetNS("semantic", "w1", "dark-theme", "enable dark mode")
	s.SetNS("semantic", "w1", "language", "english")

	results := s.Search("semantic", "dark")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Key != "dark-theme" {
		t.Errorf("key = %q, want dark-theme", results[0].Key)
	}
}

func TestPersistentStore_SearchWithEmbeddings(t *testing.T) {
	emb := &mockEmbedder{
		vectors: map[string][]float64{
			"cats are great":     {1, 0, 0},
			"dogs are fine":      {0.9, 0.1, 0},
			"math is hard":       {0, 0, 1},
			"tell me about cats": {0.95, 0.05, 0},
		},
	}
	s := newTestStoreWithEmbeddings(t, emb)
	s.SetNS("semantic", "w1", "k1", "cats are great")
	s.SetNS("semantic", "w1", "k2", "dogs are fine")
	s.SetNS("semantic", "w1", "k3", "math is hard")

	results := s.Search("semantic", "tell me about cats")
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Key != "k1" {
		t.Errorf("top result = %q, want k1", results[0].Key)
	}
	if results[1].Key != "k2" {
		t.Errorf("second result = %q, want k2", results[1].Key)
	}
	if results[2].Key != "k3" {
		t.Errorf("third result = %q, want k3", results[2].Key)
	}
}

func TestPersistentStore_EmbeddingNamespaceIsolation(t *testing.T) {
	emb := &mockEmbedder{
		vectors: map[string][]float64{
			"alpha": {1, 0, 0},
			"beta":  {0, 1, 0},
			"query": {1, 0, 0},
		},
	}
	s := newTestStoreWithEmbeddings(t, emb)
	s.SetNS("ns1", "w1", "k1", "alpha")
	s.SetNS("ns2", "w1", "k2", "beta")

	results := s.Search("ns1", "query")
	if len(results) != 1 {
		t.Fatalf("expected 1 result in ns1, got %d", len(results))
	}
	if results[0].Key != "k1" {
		t.Errorf("key = %q, want k1", results[0].Key)
	}
}
