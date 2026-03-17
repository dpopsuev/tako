package calibrate

import (
	"context"
	"fmt"

	framework "github.com/dpopsuev/origami"
)

// ContractCollector is a generic CaseCollector that uses a CalibrationContract
// to extract actual values from circuit outputs and compares them against
// expected values from a GenericScenario using the ScoreCard's scorers.
//
// This eliminates the need for domain-specific collector code. Any circuit
// that declares a calibration contract in its YAML can be scored generically.
type ContractCollector struct {
	Contract  *CalibrationContract
	ScoreCard *ScoreCard
	Scenario  *GenericScenario
	Registry  ScorerRegistry
}

// NewContractCollector creates a ContractCollector with the default scorer
// registry. The contract is typically derived from the circuit definition
// via ContractFromDef.
func NewContractCollector(contract *CalibrationContract, sc *ScoreCard, scenario *GenericScenario) *ContractCollector {
	return &ContractCollector{
		Contract:  contract,
		ScoreCard: sc,
		Scenario:  scenario,
		Registry:  DefaultScorerRegistry(),
	}
}

// Collect extracts values from BatchWalkResults using the contract, merges
// with expected values from the scenario, and runs each scorecard scorer.
func (c *ContractCollector) Collect(_ context.Context, results []framework.BatchWalkResult) (
	map[string]float64, map[string]string, error,
) {
	// Build batch: one item per case with actual (contract) + expected (scenario).
	batch := make([]map[string]any, len(results))
	for i, result := range results {
		item := make(map[string]any)

		// Add contract-extracted actual values.
		if c.Contract != nil {
			extracted := ExtractFields(c.Contract, result)
			for k, v := range extracted {
				item[k] = v
			}
		}

		// Add expected values from scenario (prefixed with "expected_").
		if i < len(c.Scenario.Cases) {
			for k, v := range c.Scenario.Cases[i].Expected {
				item["expected_"+k] = v
			}
		}

		// Include error info if the walk failed.
		if result.Error != nil {
			item["_error"] = result.Error.Error()
		}

		batch[i] = item
	}

	// Run each scorer from the scorecard against the batch.
	values := make(map[string]float64)
	details := make(map[string]string)

	for _, def := range c.ScoreCard.MetricDefs {
		if def.Scorer == "" {
			continue
		}
		scorer, err := c.Registry.Get(def.Scorer)
		if err != nil {
			details[def.ID] = fmt.Sprintf("scorer not found: %s", def.Scorer)
			continue
		}
		val, detail, err := scorer(batch, nil, def.Params)
		if err != nil {
			details[def.ID] = fmt.Sprintf("scorer error: %v", err)
			continue
		}
		values[def.ID] = val
		details[def.ID] = detail
	}

	return values, details, nil
}
