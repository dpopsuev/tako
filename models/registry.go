// Package models provides the ModelRegistry for managing foundation LLM models
// and hosting wrappers. ModelIdentity itself lives in the parent framework
// package to avoid circular dependencies.
package models

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dpopsuev/origami/circuit"
	"gopkg.in/yaml.v3"
)

// ModelRegistry is a configurable registry of foundation LLM models and
// hosting wrappers. Consumers can use DefaultModelRegistry() for built-in
// models or create custom registries via NewModelRegistry().
type ModelRegistry struct {
	mu       sync.RWMutex
	models   map[string]circuit.ModelIdentity
	wrappers map[string]bool
}

// NewModelRegistry creates an empty registry.
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models:   make(map[string]circuit.ModelIdentity),
		wrappers: make(map[string]bool),
	}
}

// Register adds a foundation model to the registry.
func (r *ModelRegistry) Register(mi circuit.ModelIdentity) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[strings.ToLower(mi.ModelName)] = mi
}

// RegisterWrapper adds a hosting environment name.
func (r *ModelRegistry) RegisterWrapper(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.wrappers[strings.ToLower(name)] = true
}

// IsKnown checks whether a probed ModelIdentity matches a registered
// foundation model. Matches on ModelName (case-insensitive).
func (r *ModelRegistry) IsKnown(mi circuit.ModelIdentity) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.models[strings.ToLower(mi.ModelName)]
	return ok
}

// IsWrapper returns true if the name is a known wrapper/IDE rather than a
// foundation model. Matches exact names and compound names with a wrapper
// prefix (e.g. "cursor-auto"). Case-insensitive.
func (r *ModelRegistry) IsWrapper(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lower := strings.ToLower(name)
	if r.wrappers[lower] {
		return true
	}
	for w := range r.wrappers {
		if strings.HasPrefix(lower, w+"-") {
			return true
		}
	}
	return false
}

// Lookup returns the registered identity for a foundation model name.
func (r *ModelRegistry) Lookup(modelName string) (circuit.ModelIdentity, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	mi, ok := r.models[strings.ToLower(modelName)]
	return mi, ok
}

// Models returns a copy of all registered foundation models.
func (r *ModelRegistry) Models() map[string]circuit.ModelIdentity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]circuit.ModelIdentity, len(r.models))
	for k, v := range r.models {
		out[k] = v
	}
	return out
}

// Wrappers returns a copy of all registered wrapper names.
func (r *ModelRegistry) Wrappers() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]bool, len(r.wrappers))
	for k, v := range r.wrappers {
		out[k] = v
	}
	return out
}

// modelFile is the YAML schema for loading models from disk.
type modelFile struct {
	Models []struct {
		ModelName string `yaml:"model_name"`
		Provider  string `yaml:"provider"`
		Version   string `yaml:"version,omitempty"`
	} `yaml:"models"`
	Wrappers []string `yaml:"wrappers"`
}

// LoadModels reads model definitions from a YAML file and registers them.
func (r *ModelRegistry) LoadModels(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read models file %s: %w", path, err)
	}
	var f modelFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse models file %s: %w", path, err)
	}
	for _, m := range f.Models {
		r.Register(circuit.ModelIdentity{
			ModelName: m.ModelName,
			Provider:  m.Provider,
			Version:   m.Version,
		})
	}
	for _, w := range f.Wrappers {
		r.RegisterWrapper(w)
	}
	return nil
}

// defaultRegistry is the singleton pre-populated with built-in models.
var defaultRegistry = func() *ModelRegistry {
	r := NewModelRegistry()
	r.Register(circuit.ModelIdentity{ModelName: "stub", Provider: "origami"})
	r.Register(circuit.ModelIdentity{ModelName: "basic-heuristic", Provider: "origami"})
	r.Register(circuit.ModelIdentity{ModelName: "claude-sonnet-4-20250514", Provider: "Anthropic", Version: "20250514"})
	for _, w := range []string{"auto", "composer", "copilot", "cursor", "azure"} {
		r.RegisterWrapper(w)
	}
	return r
}()

// DefaultModelRegistry returns the singleton registry pre-populated with
// built-in foundation models and wrappers.
func DefaultModelRegistry() *ModelRegistry { return defaultRegistry }

// IsKnownModel delegates to the default registry.
func IsKnownModel(mi circuit.ModelIdentity) bool { return defaultRegistry.IsKnown(mi) }

// IsWrapperName delegates to the default registry.
func IsWrapperName(name string) bool { return defaultRegistry.IsWrapper(name) }

// LookupModel delegates to the default registry.
func LookupModel(modelName string) (circuit.ModelIdentity, bool) {
	return defaultRegistry.Lookup(modelName)
}
