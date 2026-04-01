package calibrate

import (
	"context"
	"fmt"

	"github.com/dpopsuev/origami/engine"
	"gopkg.in/yaml.v3"
)

// GenericCase is a single test case in a generic calibration scenario.
// It contains input data for the circuit and expected outputs for scoring.
type GenericCase struct {
	ID       string         `yaml:"id"`
	Input    map[string]any `yaml:"input"`
	Expected map[string]any `yaml:"expected"`
	Canary   bool           `yaml:"canary,omitempty"` // sentinel case — must always pass
}

// GenericScenario holds a set of test cases for generic calibration.
type GenericScenario struct {
	Name  string        `yaml:"name"`
	Cases []GenericCase `yaml:"cases"`
}

// GenericScenarioLoader implements ScenarioLoader by converting
// GenericCases into circuit.BatchCases.
type GenericScenarioLoader struct {
	Scenario *GenericScenario
}

// Load returns BatchCases from the generic scenario. Each case's context
// contains the input fields, which circuit nodes can reference via
// template variables.
func (l *GenericScenarioLoader) Load(_ context.Context) ([]engine.BatchCase, error) {
	if l.Scenario == nil || len(l.Scenario.Cases) == 0 {
		return nil, ErrScenarioHasNoCases
	}
	cases := make([]engine.BatchCase, len(l.Scenario.Cases))
	for i, c := range l.Scenario.Cases {
		cases[i] = engine.BatchCase{
			ID:      c.ID,
			Context: c.Input,
		}
	}
	return cases, nil
}

// LoadGenericScenario parses YAML bytes into a GenericScenario.
func LoadGenericScenario(data []byte) (*GenericScenario, error) {
	var s GenericScenario
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse scenario: %w", err)
	}
	if s.Name == "" {
		return nil, ErrScenarioNameIsRequired
	}
	if len(s.Cases) == 0 {
		return nil, ErrScenarioHasNoCases
	}
	return &s, nil
}

// CompositeScenarioLoader merges cases from multiple loaders into one run.
// Case IDs must be unique across all loaders.
type CompositeScenarioLoader struct {
	Loaders []ScenarioLoader
}

// Load loads and merges cases from all loaders. Returns an error if any
// case IDs collide.
func (c *CompositeScenarioLoader) Load(ctx context.Context) ([]engine.BatchCase, error) {
	var all []engine.BatchCase
	seen := make(map[string]bool)
	for i, loader := range c.Loaders {
		cases, err := loader.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("loader %d: %w", i, err)
		}
		for _, bc := range cases {
			if seen[bc.ID] {
				return nil, fmt.Errorf("%w: %q across loaders", ErrDuplicateCaseID, bc.ID)
			}
			seen[bc.ID] = true
		}
		all = append(all, cases...)
	}
	if len(all) == 0 {
		return nil, ErrCompositeScenarioHasNoCases
	}
	return all, nil
}

// LoadAndMergeScenarios loads multiple YAML scenario files and merges
// their cases. Returns a single GenericScenario with concatenated cases.
func LoadAndMergeScenarios(datasets ...[]byte) (*GenericScenario, error) {
	merged := &GenericScenario{}
	seen := make(map[string]bool)
	for i, data := range datasets {
		s, err := LoadGenericScenario(data)
		if err != nil {
			return nil, fmt.Errorf("scenario %d: %w", i, err)
		}
		if merged.Name == "" {
			merged.Name = s.Name
		} else {
			merged.Name += "+" + s.Name
		}
		for _, c := range s.Cases {
			if seen[c.ID] {
				return nil, fmt.Errorf("%w: %q in scenario %d", ErrDuplicateCaseID, c.ID, i)
			}
			seen[c.ID] = true
		}
		merged.Cases = append(merged.Cases, s.Cases...)
	}
	return merged, nil
}
