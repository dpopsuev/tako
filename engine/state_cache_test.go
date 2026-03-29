package engine

import (
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

type stubCacheArtifact struct{ val string }

func (a *stubCacheArtifact) Type() string        { return "cache-test" }
func (a *stubCacheArtifact) Confidence() float64 { return 1.0 }
func (a *stubCacheArtifact) Raw() any            { return a.val }

func TestInMemoryCache_MissExecuteSet(t *testing.T) {
	c := NewInMemoryCache()

	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected miss on empty cache")
	}

	art := &stubCacheArtifact{val: "hello"}
	c.Set("key1", art, time.Minute)

	got, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected hit after set")
	}
	if got.(*stubCacheArtifact).val != "hello" {
		t.Errorf("got %v, want hello", got.Raw())
	}
}

func TestInMemoryCache_TTLExpiry(t *testing.T) {
	c := NewInMemoryCache()
	art := &stubCacheArtifact{val: "expire-me"}
	c.Set("key", art, time.Millisecond)

	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("key")
	if ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestInMemoryCache_ZeroTTL_NeverExpires(t *testing.T) {
	c := NewInMemoryCache()
	art := &stubCacheArtifact{val: "forever"}
	c.Set("key", art, 0)

	got, ok := c.Get("key")
	if !ok || got.(*stubCacheArtifact).val != "forever" {
		t.Fatal("zero TTL should never expire")
	}
}

func TestInMemoryCache_Concurrency(t *testing.T) {
	c := NewInMemoryCache()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Set("k", &stubCacheArtifact{val: "v"}, time.Minute)
			c.Get("k")
		}()
	}

	wg.Wait()

	_, ok := c.Get("k")
	if !ok {
		t.Fatal("expected hit after concurrent writes")
	}
}

func TestInMemoryCache_Len(t *testing.T) {
	c := NewInMemoryCache()
	if c.Len() != 0 {
		t.Fatalf("want 0, got %d", c.Len())
	}
	c.Set("a", &stubCacheArtifact{val: "a"}, time.Minute)
	c.Set("b", &stubCacheArtifact{val: "b"}, time.Minute)
	if c.Len() != 2 {
		t.Fatalf("want 2, got %d", c.Len())
	}
}

func TestInMemoryCache_ImplementsNodeCache(t *testing.T) {
	var _ circuit.NodeCache = (*InMemoryCache)(nil)
}
