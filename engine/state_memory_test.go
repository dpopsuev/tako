package engine

import (
	"sort"
	"sync"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestInMemoryStoreSetAndGet(t *testing.T) {
	store := NewInMemoryStore()
	store.Set("walker-1", "key-a", "value-a")

	got, ok := store.Get("walker-1", "key-a")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if got != "value-a" {
		t.Errorf("Get = %v, want %q", got, "value-a")
	}
}

func TestInMemoryStoreGetMissing(t *testing.T) {
	store := NewInMemoryStore()

	_, ok := store.Get("nonexistent", "key")
	if ok {
		t.Error("expected missing walker to return false")
	}

	store.Set("walker-1", "key-a", "value")
	_, ok = store.Get("walker-1", "key-b")
	if ok {
		t.Error("expected missing key to return false")
	}
}

func TestInMemoryStoreIsolation(t *testing.T) {
	store := NewInMemoryStore()
	store.Set("walker-1", "shared-key", "value-1")
	store.Set("walker-2", "shared-key", "value-2")

	v1, _ := store.Get("walker-1", "shared-key")
	v2, _ := store.Get("walker-2", "shared-key")

	if v1 != "value-1" {
		t.Errorf("walker-1 value = %v, want %q", v1, "value-1")
	}
	if v2 != "value-2" {
		t.Errorf("walker-2 value = %v, want %q", v2, "value-2")
	}
}

func TestInMemoryStoreKeys(t *testing.T) {
	store := NewInMemoryStore()
	store.Set("w1", "b", 2)
	store.Set("w1", "a", 1)
	store.Set("w1", "c", 3)

	keys := store.Keys("w1")
	sort.Strings(keys)

	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("keys = %v, want [a b c]", keys)
	}
}

func TestInMemoryStoreKeysEmpty(t *testing.T) {
	store := NewInMemoryStore()
	keys := store.Keys("nonexistent")
	if keys != nil {
		t.Errorf("expected nil for nonexistent walker, got %v", keys)
	}
}

func TestInMemoryStoreConcurrentSafety(t *testing.T) {
	store := NewInMemoryStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			walkerID := "walker"
			key := "counter"
			store.Set(walkerID, key, n)
			store.Get(walkerID, key)
			store.Keys(walkerID)
		}(i)
	}

	wg.Wait()

	_, ok := store.Get("walker", "counter")
	if !ok {
		t.Error("expected key to exist after concurrent writes")
	}
}

func TestInMemoryStorePersistsAcrossReads(t *testing.T) {
	store := NewInMemoryStore()

	store.Set("agent-a", "step_count", 5)

	got, ok := store.Get("agent-a", "step_count")
	if !ok || got != 5 {
		t.Errorf("first read: got=%v ok=%v, want 5/true", got, ok)
	}

	store.Set("agent-a", "step_count", 10)

	got, ok = store.Get("agent-a", "step_count")
	if !ok || got != 10 {
		t.Errorf("second read after update: got=%v ok=%v, want 10/true", got, ok)
	}
}

func TestInMemoryStore_NamespaceIsolation(t *testing.T) {
	store := NewInMemoryStore()
	store.SetNS("semantic", "w1", "pref", "dark")
	store.SetNS("episodic", "w1", "pref", "light")

	v1, ok := store.GetNS("semantic", "w1", "pref")
	if !ok || v1 != "dark" {
		t.Errorf("semantic pref = %v, want dark", v1)
	}

	v2, ok := store.GetNS("episodic", "w1", "pref")
	if !ok || v2 != "light" {
		t.Errorf("episodic pref = %v, want light", v2)
	}

	_, ok = store.GetNS("procedural", "w1", "pref")
	if ok {
		t.Error("procedural should not have pref")
	}
}

func TestInMemoryStore_BackwardCompat_DefaultNamespace(t *testing.T) {
	store := NewInMemoryStore()
	store.Set("w1", "key", "via-set")

	v, ok := store.GetNS("", "w1", "key")
	if !ok || v != "via-set" {
		t.Errorf("GetNS with default ns = %v, want via-set", v)
	}

	store.SetNS("", "w1", "key2", "via-setns")
	v2, ok := store.Get("w1", "key2")
	if !ok || v2 != "via-setns" {
		t.Errorf("Get from SetNS = %v, want via-setns", v2)
	}
}

func TestInMemoryStore_KeysNS(t *testing.T) {
	store := NewInMemoryStore()
	store.SetNS("semantic", "w1", "a", 1)
	store.SetNS("semantic", "w1", "b", 2)
	store.SetNS("episodic", "w1", "c", 3)

	keys := store.KeysNS("semantic", "w1")
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("semantic keys = %v, want [a b]", keys)
	}

	keys = store.KeysNS("episodic", "w1")
	if len(keys) != 1 || keys[0] != "c" {
		t.Errorf("episodic keys = %v, want [c]", keys)
	}
}

func TestInMemoryStore_Search(t *testing.T) {
	store := NewInMemoryStore()
	store.SetNS("semantic", "w1", "theme-preference", "dark mode")
	store.SetNS("semantic", "w1", "language", "english")
	store.SetNS("semantic", "w2", "theme-preference", "light mode")

	results := store.Search("semantic", "theme")
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'theme', got %d", len(results))
	}

	results = store.Search("semantic", "dark")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'dark', got %d", len(results))
	}
	if results[0].Value != "dark mode" {
		t.Errorf("search result = %v, want 'dark mode'", results[0].Value)
	}
}

func TestInMemoryStore_SearchByTag(t *testing.T) {
	store := NewInMemoryStore()
	store.SetNSTagged("semantic", "w1", "k1", "v1", []string{"rca", "ptp"})
	store.SetNSTagged("semantic", "w1", "k2", "v2", []string{"security"})

	results := store.Search("semantic", "ptp")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for tag 'ptp', got %d", len(results))
	}
	if results[0].Key != "k1" {
		t.Errorf("result key = %q, want k1", results[0].Key)
	}
}

func TestTaggedMemoryStore_AutoTags(t *testing.T) {
	inner := NewInMemoryStore()
	wrapped := &TaggedMemoryStore{Inner: inner, Tags: []string{"run-001", "rca"}}

	wrapped.SetNS("semantic", "w1", "finding", "goroutine leak")

	item := inner.Data["semantic"]["w1"]["finding"]
	if item.Value != "goroutine leak" {
		t.Errorf("value = %v, want 'goroutine leak'", item.Value)
	}
	if len(item.Tags) != 2 || item.Tags[0] != "run-001" || item.Tags[1] != "rca" {
		t.Errorf("tags = %v, want [run-001 rca]", item.Tags)
	}
}

func TestTaggedMemoryStore_ReadDelegation(t *testing.T) {
	inner := NewInMemoryStore()
	inner.SetNS("semantic", "w1", "key", "val")

	wrapped := &TaggedMemoryStore{Inner: inner, Tags: []string{"tag"}}

	v, ok := wrapped.GetNS("semantic", "w1", "key")
	if !ok || v != "val" {
		t.Errorf("GetNS = %v, %v, want val/true", v, ok)
	}

	keys := wrapped.KeysNS("semantic", "w1")
	if len(keys) != 1 || keys[0] != "key" {
		t.Errorf("KeysNS = %v, want [key]", keys)
	}

	results := wrapped.Search("semantic", "val")
	if len(results) != 1 {
		t.Errorf("Search = %d results, want 1", len(results))
	}
}

func TestTaggedMemoryStore_BackwardCompatSet(t *testing.T) {
	inner := NewInMemoryStore()
	wrapped := &TaggedMemoryStore{Inner: inner, Tags: []string{"auto"}}

	wrapped.Set("w1", "k", "v")

	item := inner.Data[""]["w1"]["k"]
	if len(item.Tags) != 1 || item.Tags[0] != "auto" {
		t.Errorf("Set via tagged wrapper: tags = %v, want [auto]", item.Tags)
	}

	v, ok := wrapped.Get("w1", "k")
	if !ok || v != "v" {
		t.Errorf("Get = %v, %v, want v/true", v, ok)
	}
}

func TestInMemoryStore_ImplementsMemoryStore(t *testing.T) {
	var _ circuit.MemoryStore = (*InMemoryStore)(nil)
}

func TestTaggedMemoryStore_ImplementsMemoryStore(t *testing.T) {
	var _ circuit.MemoryStore = (*TaggedMemoryStore)(nil)
}
