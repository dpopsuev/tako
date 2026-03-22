package calibrate

import (
	"context"
	"fmt"
	"sort"

	"github.com/dpopsuev/origami/engine"
)

// ScenarioRegistry maps scenario names to loaders. When registered,
// scenario names can be auto-populated into ExtraParamDef.Enum for
// MCP schema generation.
type ScenarioRegistry struct {
	loaders map[string]ScenarioLoader
}

// NewScenarioRegistry creates an empty registry.
func NewScenarioRegistry() *ScenarioRegistry {
	return &ScenarioRegistry{loaders: make(map[string]ScenarioLoader)}
}

// Register adds a named scenario loader.
func (r *ScenarioRegistry) Register(name string, loader ScenarioLoader) {
	r.loaders[name] = loader
}

// List returns sorted scenario names.
func (r *ScenarioRegistry) List() []string {
	names := make([]string, 0, len(r.loaders))
	for name := range r.loaders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Load loads cases for the named scenario.
func (r *ScenarioRegistry) Load(ctx context.Context, name string) ([]engine.BatchCase, error) {
	loader, ok := r.loaders[name]
	if !ok {
		return nil, fmt.Errorf("unknown scenario %q; available: %v", name, r.List())
	}
	return loader.Load(ctx)
}

// ExtraParamEnum returns the scenario names as an enum list for
// ExtraParamDef population.
func (r *ScenarioRegistry) ExtraParamEnum() []string {
	return r.List()
}
