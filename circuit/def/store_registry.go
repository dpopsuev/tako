package def

// Category: DSL & Build — store engine registry and resolution.

import (
	"fmt"
	"sync"
)

// StoreRegistry manages multiple StoreEngine instances and resolves
// store bindings at runtime. Consumers register engine factories,
// then resolve named stores from StoreWiring configuration.
type StoreRegistry struct {
	mu      sync.RWMutex
	engines map[string]StoreEngineFactory
	stores  map[string]StoreEngine
	wiring  *StoreWiring
}

// StoreEngineFactory creates a new StoreEngine instance.
type StoreEngineFactory func() StoreEngine

// NewStoreRegistry creates a registry with optional wiring configuration.
func NewStoreRegistry(wiring *StoreWiring) *StoreRegistry {
	return &StoreRegistry{
		engines: make(map[string]StoreEngineFactory),
		stores:  make(map[string]StoreEngine),
		wiring:  wiring,
	}
}

// RegisterEngine registers a factory for a named engine type (e.g.
// "sqlite", "memory"). Panics if the name is already registered.
func (r *StoreRegistry) RegisterEngine(name string, factory StoreEngineFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.engines[name]; exists {
		panic(fmt.Sprintf("store engine %q already registered", name))
	}
	r.engines[name] = factory
}

// Resolve returns a StoreEngine for the given named store. It creates
// the engine on first access using the wiring configuration. Subsequent
// calls return the same instance. Thread-safe.
func (r *StoreRegistry) Resolve(storeName string) (StoreEngine, error) {
	r.mu.RLock()
	if engine, ok := r.stores[storeName]; ok {
		r.mu.RUnlock()
		return engine, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock.
	if engine, ok := r.stores[storeName]; ok {
		return engine, nil
	}

	engineName := r.resolveEngineName(storeName)
	if engineName == "" {
		return nil, fmt.Errorf("%w: %q (check store_wiring in tako.yaml)", ErrNoEngineConfiguredForStore, storeName)
	}

	factory, ok := r.engines[engineName]
	if !ok {
		return nil, fmt.Errorf("%w: %q for store %q (registered: %v)", ErrUnknownEngine, engineName, storeName, r.engineNames())
	}

	engine := factory()

	var config map[string]string
	if r.wiring != nil {
		if binding, ok := r.wiring.Stores[storeName]; ok {
			config = binding.Config
		}
	}

	if err := engine.Open(config); err != nil {
		return nil, fmt.Errorf("open store %q (engine=%s): %w", storeName, engineName, err)
	}

	r.stores[storeName] = engine
	return engine, nil
}

// CloseAll closes all open store engines. Returns the first error
// encountered, continuing to close remaining engines.
func (r *StoreRegistry) CloseAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var firstErr error
	for name, engine := range r.stores {
		if err := engine.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close store %q: %w", name, err)
		}
	}
	r.stores = make(map[string]StoreEngine)
	return firstErr
}

// resolveEngineName looks up the engine name for a store from wiring config.
func (r *StoreRegistry) resolveEngineName(storeName string) string {
	if r.wiring == nil {
		return ""
	}
	if binding, ok := r.wiring.Stores[storeName]; ok {
		return binding.Engine
	}
	return r.wiring.Default
}

func (r *StoreRegistry) engineNames() []string {
	names := make([]string, 0, len(r.engines))
	for name := range r.engines {
		names = append(names, name)
	}
	return names
}
