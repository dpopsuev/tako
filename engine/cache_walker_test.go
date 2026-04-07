package engine

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
)

func TestCachingWalker_CacheHit(t *testing.T) {
	cache := NewInMemoryCache()
	cached := &stubCacheArtifact{val: "cached-result"}
	cache.Set("testhash:nodeA", cached, time.Minute)

	inner := &runnerTestWalker{state: circuit.NewWalkerState("test")}
	cw := &cachingWalker{
		inner:       inner,
		cache:       cache,
		cacheTTL:    map[string]time.Duration{"nodeA": time.Minute},
		circuitHash: "testhash",
	}

	node := &runnerTestNode{name: "nodeA", out: &stubCacheArtifact{val: "fresh"}}
	nc := circuit.NodeContext{WalkerState: inner.State(), Meta: map[string]any{}}

	art, err := cw.Handle(context.Background(), node, nc)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if art.(*stubCacheArtifact).val != "cached-result" {
		t.Errorf("got %q, want cached-result", art.(*stubCacheArtifact).val)
	}
	if len(inner.visited) != 0 {
		t.Error("inner walker should not have been called on cache hit")
	}
}

func TestCachingWalker_CacheMiss(t *testing.T) {
	cache := NewInMemoryCache()

	fresh := &stubCacheArtifact{val: "fresh-result"}
	inner := &runnerTestWalker{state: circuit.NewWalkerState("test")}
	cw := &cachingWalker{
		inner:       inner,
		cache:       cache,
		cacheTTL:    map[string]time.Duration{"nodeA": time.Minute},
		circuitHash: "testhash",
	}

	node := &runnerTestNode{name: "nodeA", out: fresh}
	nc := circuit.NodeContext{WalkerState: inner.State(), Meta: map[string]any{}}

	art, err := cw.Handle(context.Background(), node, nc)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if art.(*stubCacheArtifact).val != "fresh-result" {
		t.Errorf("got %q, want fresh-result", art.(*stubCacheArtifact).val)
	}
	if len(inner.visited) != 1 {
		t.Error("inner walker should have been called on cache miss")
	}

	// Verify result was stored in cache.
	got, ok := cache.Get("testhash:nodeA")
	if !ok {
		t.Fatal("expected result to be cached after miss")
	}
	if got.(*stubCacheArtifact).val != "fresh-result" {
		t.Errorf("cached value = %q, want fresh-result", got.(*stubCacheArtifact).val)
	}
}

func TestCachingWalker_NoCacheDef_Passthrough(t *testing.T) {
	cache := NewInMemoryCache()
	fresh := &stubCacheArtifact{val: "passthrough"}
	inner := &runnerTestWalker{state: circuit.NewWalkerState("test")}
	cw := &cachingWalker{
		inner:       inner,
		cache:       cache,
		cacheTTL:    map[string]time.Duration{}, // no nodes configured for caching
		circuitHash: "testhash",
	}

	node := &runnerTestNode{name: "nodeB", out: fresh}
	nc := circuit.NodeContext{WalkerState: inner.State(), Meta: map[string]any{}}

	art, err := cw.Handle(context.Background(), node, nc)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if art.(*stubCacheArtifact).val != "passthrough" {
		t.Errorf("got %q, want passthrough", art.(*stubCacheArtifact).val)
	}
	if len(inner.visited) != 1 {
		t.Error("inner walker should be called for non-cached node")
	}
	if cache.Len() != 0 {
		t.Error("cache should remain empty for non-cached node")
	}
}

func TestCachingWalker_InnerError_NotCached(t *testing.T) {
	cache := NewInMemoryCache()
	inner := &runnerTestWalker{state: circuit.NewWalkerState("test")}
	cw := &cachingWalker{
		inner:       inner,
		cache:       cache,
		cacheTTL:    map[string]time.Duration{"nodeA": time.Minute},
		circuitHash: "testhash",
	}

	node := &runnerTestNode{name: "nodeA", err: circuit.ErrNode}
	nc := circuit.NodeContext{WalkerState: inner.State(), Meta: map[string]any{}}

	_, err := cw.Handle(context.Background(), node, nc)
	if err == nil {
		t.Fatal("expected error from inner walker")
	}
	if cache.Len() != 0 {
		t.Error("failed results should not be cached")
	}
}

func TestCachingWalker_DelegatesIdentityAndState(t *testing.T) {
	inner := &runnerTestWalker{
		identity: roster.AgentIdentity{Name: "test-persona"},
		state:    circuit.NewWalkerState("test-id"),
	}
	cw := &cachingWalker{
		inner:       inner,
		cache:       NewInMemoryCache(),
		cacheTTL:    map[string]time.Duration{},
		circuitHash: "testhash",
	}

	if cw.Identity().Name != "test-persona" {
		t.Errorf("Identity().Name = %q, want test-persona", cw.Identity().Name)
	}
	if cw.State().ID != "test-id" {
		t.Errorf("State().ID = %q, want test-id", cw.State().ID)
	}

	newID := roster.AgentIdentity{Name: "updated"}
	cw.SetIdentity(&newID)
	if inner.identity.Name != "updated" {
		t.Errorf("SetIdentity not delegated: inner Name = %q", inner.identity.Name)
	}
}

func TestCacheKey_WithInput(t *testing.T) {
	cw := &cachingWalker{circuitHash: "abc123"}

	state := circuit.NewWalkerState("test")
	state.Context["input"] = map[string]any{"query": "hello"}
	nc := circuit.NodeContext{WalkerState: state}

	key := cw.cacheKey("nodeA", nc)
	if key == "abc123:nodeA" {
		t.Error("cache key should include input hash when input is present")
	}

	// Same input should produce the same key.
	key2 := cw.cacheKey("nodeA", nc)
	if key != key2 {
		t.Errorf("same input produced different keys: %q vs %q", key, key2)
	}

	// Different input should produce a different key.
	state.Context["input"] = map[string]any{"query": "world"}
	key3 := cw.cacheKey("nodeA", nc)
	if key == key3 {
		t.Error("different input should produce different cache key")
	}
}

func TestCacheKey_WithoutInput(t *testing.T) {
	cw := &cachingWalker{circuitHash: "abc123"}

	state := circuit.NewWalkerState("test")
	nc := circuit.NodeContext{WalkerState: state}

	key := cw.cacheKey("nodeA", nc)
	if key != "abc123:nodeA" {
		t.Errorf("cache key without input = %q, want abc123:nodeA", key)
	}
}

func TestCacheKey_DifferentCircuitHash(t *testing.T) {
	state := circuit.NewWalkerState("test")
	state.Context["input"] = map[string]any{"query": "hello"}
	nc := circuit.NodeContext{WalkerState: state}

	cw1 := &cachingWalker{circuitHash: "hash_v1"}
	cw2 := &cachingWalker{circuitHash: "hash_v2"}

	key1 := cw1.cacheKey("nodeA", nc)
	key2 := cw2.cacheKey("nodeA", nc)

	if key1 == key2 {
		t.Errorf("different circuit hashes should produce different cache keys, both got %q", key1)
	}

	// Also verify without input.
	stateNoInput := circuit.NewWalkerState("test")
	ncNoInput := circuit.NodeContext{WalkerState: stateNoInput}

	key3 := cw1.cacheKey("nodeA", ncNoInput)
	key4 := cw2.cacheKey("nodeA", ncNoInput)

	if key3 == key4 {
		t.Errorf("different circuit hashes without input should produce different keys, both got %q", key3)
	}
}

func TestBuildCacheTTLs(t *testing.T) {
	def := &circuit.CircuitDef{
		Nodes: []circuit.NodeDef{
			{Name: "cached-explicit", Cache: &circuit.CacheDef{TTL: "5m"}},
			{Name: "cached-default", Cache: &circuit.CacheDef{}},
			{Name: "not-cached"},
			{Name: "cached-bad-ttl", Cache: &circuit.CacheDef{TTL: "invalid"}},
			{Name: "cached-negative", Cache: &circuit.CacheDef{TTL: "-1s"}},
		},
	}

	ttls := buildCacheTTLs(def)

	if ttls["cached-explicit"] != 5*time.Minute {
		t.Errorf("cached-explicit TTL = %v, want 5m", ttls["cached-explicit"])
	}
	if ttls["cached-default"] != defaultCacheTTL {
		t.Errorf("cached-default TTL = %v, want %v", ttls["cached-default"], defaultCacheTTL)
	}
	if _, ok := ttls["not-cached"]; ok {
		t.Error("not-cached should not have a TTL entry")
	}
	if ttls["cached-bad-ttl"] != defaultCacheTTL {
		t.Errorf("cached-bad-ttl TTL = %v, want %v (default)", ttls["cached-bad-ttl"], defaultCacheTTL)
	}
	if ttls["cached-negative"] != defaultCacheTTL {
		t.Errorf("cached-negative TTL = %v, want %v (default)", ttls["cached-negative"], defaultCacheTTL)
	}
}
