package contracts_test

import (
	"testing"

	"github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/testkit/contracts"
)

func TestScenarioLoaderContract_GenericScenarioLoader(t *testing.T) {
	contracts.RunScenarioLoaderContract(t, func() calibrate.ScenarioLoader {
		return &calibrate.GenericScenarioLoader{
			Scenario: &calibrate.GenericScenario{
				Name: "test-scenario",
				Cases: []calibrate.GenericCase{
					{ID: "C01", Input: map[string]any{"key": "val1"}},
					{ID: "C02", Input: map[string]any{"key": "val2"}},
				},
			},
		}
	})
}
