package engine

// Category: Execution

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

// defaultCacheTTL is used when a node declares caching but specifies no TTL.
const defaultCacheTTL = time.Hour

// cachingWalker wraps a Walker to check a NodeCache before executing a node
// and store the result after a successful execution. Nodes without a cache
// TTL entry are passed through to the inner walker unchanged.
type cachingWalker struct {
	inner       circuit.Walker
	cache       circuit.NodeCache
	cacheTTL    map[string]time.Duration // node name → TTL from CacheDef
	circuitHash string                   // SHA-256 hex of circuit YAML bytes
	log         *slog.Logger
}

func (cw *cachingWalker) Identity() circuit.AgentIdentity       { return cw.inner.Identity() }
func (cw *cachingWalker) SetIdentity(id *circuit.AgentIdentity) { cw.inner.SetIdentity(id) }
func (cw *cachingWalker) State() *circuit.WalkerState           { return cw.inner.State() }

func (cw *cachingWalker) Handle(ctx context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	ttl, cacheable := cw.cacheTTL[node.Name()]
	if !cacheable {
		return cw.inner.Handle(ctx, node, nc)
	}

	key := cw.cacheKey(node.Name(), nc)

	if art, ok := cw.cache.Get(key); ok {
		if cw.log != nil {
			cw.log.DebugContext(ctx, circuit.LogCacheHit,
				slog.String(circuit.LogKeyComponent, circuit.LogComponentWalk),
				slog.String(circuit.LogKeyNode, node.Name()),
				slog.String(circuit.LogKeyCacheKey, key),
			)
		}
		return art, nil
	}

	if cw.log != nil {
		cw.log.DebugContext(ctx, circuit.LogCacheMiss,
			slog.String(circuit.LogKeyComponent, circuit.LogComponentWalk),
			slog.String(circuit.LogKeyNode, node.Name()),
			slog.String(circuit.LogKeyCacheKey, key),
		)
	}

	art, err := cw.inner.Handle(ctx, node, nc)
	if err != nil {
		return nil, err
	}

	cw.cache.Set(key, art, ttl)
	return art, nil
}

// cacheKey builds a deterministic cache key from the circuit hash, node name,
// and the walker state context "input" value. Including the circuit hash
// ensures stale entries are unreachable when the circuit YAML changes.
// When the input is not JSON-serializable (or absent), the input hash is
// omitted.
func (cw *cachingWalker) cacheKey(nodeName string, nc circuit.NodeContext) string {
	input, ok := nc.WalkerState.Context["input"]
	if !ok {
		return fmt.Sprintf("%s:%s", cw.circuitHash, nodeName)
	}

	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Sprintf("%s:%s", cw.circuitHash, nodeName)
	}

	h := sha256.Sum256(data)
	return fmt.Sprintf("%s:%s:%x", cw.circuitHash, nodeName, h)
}

// buildCacheTTLs extracts per-node cache TTLs from the circuit definition.
// Nodes without a Cache field are omitted. Nodes with a Cache field but no
// TTL (or an unparseable TTL) receive defaultCacheTTL.
func buildCacheTTLs(def *circuit.CircuitDef) map[string]time.Duration {
	ttls := make(map[string]time.Duration)
	for i := range def.Nodes {
		nd := &def.Nodes[i]
		if nd.Cache == nil {
			continue
		}
		name := string(nd.Name)
		if nd.Cache.TTL == "" {
			ttls[name] = defaultCacheTTL
			continue
		}
		d, err := time.ParseDuration(nd.Cache.TTL)
		if err != nil || d <= 0 {
			ttls[name] = defaultCacheTTL
			continue
		}
		ttls[name] = d
	}
	return ttls
}
