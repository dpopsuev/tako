package sqlite

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

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
