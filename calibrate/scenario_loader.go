package calibrate

import (
	"context"
	"fmt"

	framework "github.com/dpopsuev/origami"
	"gopkg.in/yaml.v3"
)

// GenericCase is a single test case in a generic calibration scenario.
// It contains input data for the circuit and expected outputs for scoring.
type GenericCase struct {
	ID       string         `yaml:"id"`
	Input    map[string]any `yaml:"input"`
	Expected map[string]any `yaml:"expected"`
}

// GenericScenario holds a set of test cases for generic calibration.
type GenericScenario struct {
	Name  string        `yaml:"name"`
	Cases []GenericCase `yaml:"cases"`
}

// GenericScenarioLoader implements ScenarioLoader by converting
// GenericCases into framework.BatchCases.
type GenericScenarioLoader struct {
	Scenario *GenericScenario
}

// Load returns BatchCases from the generic scenario. Each case's context
// contains the input fields, which circuit nodes can reference via
// template variables.
func (l *GenericScenarioLoader) Load(_ context.Context) ([]framework.BatchCase, error) {
	if l.Scenario == nil || len(l.Scenario.Cases) == 0 {
		return nil, fmt.Errorf("scenario has no cases")
	}
	cases := make([]framework.BatchCase, len(l.Scenario.Cases))
	for i, c := range l.Scenario.Cases {
		cases[i] = framework.BatchCase{
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
		return nil, fmt.Errorf("scenario name is required")
	}
	if len(s.Cases) == 0 {
		return nil, fmt.Errorf("scenario has no cases")
	}
	return &s, nil
}
