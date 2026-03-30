package resource

import (
	"fmt"
	"sort"
	"sync"

	"github.com/dpopsuev/origami/circuit"
)

// KindRegistry maps kind strings to their handlers.
// Thread-safe for concurrent access.
type KindRegistry struct {
	mu       sync.RWMutex
	handlers map[circuit.Kind]KindHandler
}

// NewKindRegistry creates an empty registry.
func NewKindRegistry() *KindRegistry {
	return &KindRegistry{handlers: make(map[circuit.Kind]KindHandler)}
}

// Register adds a handler for a kind. Panics on duplicate registration.
func (r *KindRegistry) Register(h KindHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[h.Kind()]; exists {
		panic(fmt.Sprintf("duplicate resource kind registration: %q", h.Kind()))
	}
	r.handlers[h.Kind()] = h
}

// Lookup returns the handler for a kind, or nil if not registered.
func (r *KindRegistry) Lookup(k circuit.Kind) KindHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handlers[k]
}

// Has returns true if a handler is registered for the kind.
func (r *KindRegistry) Has(k circuit.Kind) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.handlers[k]
	return ok
}

// Kinds returns a sorted list of all registered kind values.
func (r *KindRegistry) Kinds() []circuit.Kind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]circuit.Kind, 0, len(r.handlers))
	for k := range r.handlers {
		result = append(result, k)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
