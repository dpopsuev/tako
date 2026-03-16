package mcp

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/dpopsuev/origami/dispatch"
)

// CircuitType defines a named circuit type that can be registered with
// the Papercup MCP server. Each type provides its own session factory,
// step schemas, and report formatter. When start_circuit specifies a
// circuit_type in extra, the server routes to the matching type.
type CircuitType struct {
	Name           string
	Description    string
	StepSchemas    []StepSchema
	ExtraParamDefs []ExtraParamDef
	WorkerPreamble string

	CreateSession func(ctx context.Context, params StartParams, disp *dispatch.MuxDispatcher, bus *dispatch.SignalBus) (RunFunc, SessionMeta, error)
	FormatReport  func(result any) (formatted string, structured any, err error)
}

// CircuitTypeRegistry holds registered circuit types and routes session
// creation to the appropriate handler. Thread-safe for concurrent reads
// after initial registration.
type CircuitTypeRegistry struct {
	mu    sync.RWMutex
	types map[string]*CircuitType
}

// NewCircuitTypeRegistry creates an empty registry.
func NewCircuitTypeRegistry() *CircuitTypeRegistry {
	return &CircuitTypeRegistry{
		types: make(map[string]*CircuitType),
	}
}

// Register adds a circuit type to the registry. Panics on duplicate names.
func (r *CircuitTypeRegistry) Register(ct *CircuitType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.types[ct.Name]; exists {
		panic(fmt.Sprintf("circuit type %q already registered", ct.Name))
	}
	r.types[ct.Name] = ct
}

// Lookup returns the circuit type for the given name, or nil if not found.
func (r *CircuitTypeRegistry) Lookup(name string) *CircuitType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.types[name]
}

// Names returns all registered circuit type names, sorted alphabetically.
func (r *CircuitTypeRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.types))
	for name := range r.types {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// RouteSession resolves the circuit type from params.Extra["circuit_type"]
// and delegates session creation to that type's factory. Falls back to the
// default type when no circuit_type is specified and exactly one type is
// registered. Returns an error if routing fails.
func (r *CircuitTypeRegistry) RouteSession(
	ctx context.Context,
	params StartParams,
	disp *dispatch.MuxDispatcher,
	bus *dispatch.SignalBus,
) (RunFunc, SessionMeta, error) {
	typeName, _ := params.Extra["circuit_type"].(string)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if typeName == "" {
		if len(r.types) == 1 {
			for _, ct := range r.types {
				return ct.CreateSession(ctx, params, disp, bus)
			}
		}
		return nil, SessionMeta{}, fmt.Errorf(
			"circuit_type is required in extra when multiple types are registered (available: %v)",
			r.Names(),
		)
	}

	ct, ok := r.types[typeName]
	if !ok {
		return nil, SessionMeta{}, fmt.Errorf(
			"unknown circuit_type %q (available: %v)", typeName, r.Names(),
		)
	}
	return ct.CreateSession(ctx, params, disp, bus)
}

// MergedStepSchemas returns the union of all registered types' step schemas.
func (r *CircuitTypeRegistry) MergedStepSchemas() []StepSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool)
	var merged []StepSchema
	for _, ct := range r.types {
		for _, s := range ct.StepSchemas {
			if !seen[s.Name] {
				seen[s.Name] = true
				merged = append(merged, s)
			}
		}
	}
	return merged
}
