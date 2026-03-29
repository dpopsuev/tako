package calibrate

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

func TestContractCollector_Collect(t *testing.T) {
	contract := &CalibrationContract{
		Outputs: []ContractField{
			{Field: "prompt.answer", ScorerName: "answer"},
		},
	}

	sc := &ScoreCard{
		Name: "test",
		MetricDefs: []MetricDef{
			{
				ID:        "M1",
				Name:      "answer_accuracy",
				Threshold: 1.0,
				Scorer:    "batch_field_match",
				Tier:      TierOutcome,
				Direction: HigherIsBetter,
				Params: map[string]any{
					"actual":   "answer",
					"expected": "expected_answer",
				},
			},
		},
	}

	scenario := &GenericScenario{
		Name: "test",
		Cases: []GenericCase{
			{ID: "C1", Expected: map[string]any{"answer": float64(4)}},
			{ID: "C2", Expected: map[string]any{"answer": float64(6)}},
		},
	}

	// Simulate circuit outputs: C1 correct, C2 wrong.
	results := []engine.BatchWalkResult{
		{
			CaseID: "C1",
			StepArtifacts: map[string]circuit.Artifact{
				"prompt": &testArtifact{raw: map[string]any{"answer": float64(4)}},
			},
		},
		{
			CaseID: "C2",
			StepArtifacts: map[string]circuit.Artifact{
				"prompt": &testArtifact{raw: map[string]any{"answer": float64(7)}},
			},
		},
	}

	collector := NewContractCollector(contract, sc, scenario)
	values, details, err := collector.Collect(context.Background(), results)
	if err != nil {
		t.Fatal(err)
	}

	val, ok := values["M1"]
	if !ok {
		t.Fatal("M1 not in values")
	}
	// 1 of 2 correct = 0.5
	if val != 0.5 {
		t.Errorf("M1 = %f, want 0.5; detail: %s", val, details["M1"])
	}
}

// testArtifact implements circuit.Artifact for testing.
type testArtifact struct {
	raw any
}

func (a *testArtifact) Type() string        { return "test" }
func (a *testArtifact) Confidence() float64 { return 1.0 }
func (a *testArtifact) Raw() any            { return a.raw }
