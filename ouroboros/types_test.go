package ouroboros

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

func TestDiscoveryResult_JSONRoundTrip(t *testing.T) {
	dr := DiscoveryResult{
		Iteration: 3,
		Model: circuit.ModelIdentity{
			ModelName: "claude-sonnet-4-20250514",
			Provider:  "Anthropic",
			Version:   "20250514",
			Wrapper:   "cursor",
		},
		ExclusionPrompt: "Excluding: gpt-4o, gemini-2.5-pro",
		Probe: ProbeResult{
			ProbeID:   "refactor-v1",
			RawOutput: "func renamed() { ... }",
			Score: ProbeScore{
				Renames:           3,
				FunctionSplits:    1,
				CommentsAdded:     2,
				StructuralChanges: 4,
				TotalScore:        0.72,
			},
			Elapsed: 5 * time.Second,
		},
		Timestamp: time.Date(2026, 2, 21, 15, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got DiscoveryResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Iteration != 3 {
		t.Errorf("iteration: got %d, want 3", got.Iteration)
	}
	if got.Model.ModelName != "claude-sonnet-4-20250514" {
		t.Errorf("model: got %q, want claude-sonnet-4-20250514", got.Model.ModelName)
	}
	if got.Probe.Score.Renames != 3 {
		t.Errorf("renames: got %d, want 3", got.Probe.Score.Renames)
	}
	if got.Probe.Score.TotalScore != 0.72 {
		t.Errorf("total_score: got %f, want 0.72", got.Probe.Score.TotalScore)
	}
}

func TestRunReport_JSONRoundTrip(t *testing.T) {
	report := RunReport{
		RunID:     "run-20260221-150000",
		StartTime: time.Date(2026, 2, 21, 15, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 21, 15, 5, 0, 0, time.UTC),
		Config:    DefaultConfig(),
		Results: []DiscoveryResult{
			{Iteration: 0, Model: circuit.ModelIdentity{ModelName: "gpt-4o", Provider: "OpenAI"}},
			{Iteration: 1, Model: circuit.ModelIdentity{ModelName: "claude-sonnet-4", Provider: "Anthropic"}},
		},
		UniqueModels: []circuit.ModelIdentity{
			{ModelName: "gpt-4o", Provider: "OpenAI"},
			{ModelName: "claude-sonnet-4", Provider: "Anthropic"},
		},
		TermReason: "repeat",
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got RunReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.RunID != "run-20260221-150000" {
		t.Errorf("run_id: got %q, want run-20260221-150000", got.RunID)
	}
	if len(got.Results) != 2 {
		t.Fatalf("results: got %d, want 2", len(got.Results))
	}
	if len(got.UniqueModels) != 2 {
		t.Fatalf("unique_models: got %d, want 2", len(got.UniqueModels))
	}
	if got.TermReason != "repeat" {
		t.Errorf("term_reason: got %q, want repeat", got.TermReason)
	}
}

func TestRunReport_ModelNames(t *testing.T) {
	report := RunReport{
		UniqueModels: []circuit.ModelIdentity{
			{ModelName: "gpt-4o", Provider: "OpenAI"},
			{ModelName: "claude-sonnet-4", Provider: "Anthropic", Version: "20250514"},
		},
	}

	names := report.ModelNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "gpt-4o/OpenAI" {
		t.Errorf("names[0]: got %q, want gpt-4o/OpenAI", names[0])
	}
	if names[1] != "claude-sonnet-4@20250514/Anthropic" {
		t.Errorf("names[1]: got %q, want claude-sonnet-4@20250514/Anthropic", names[1])
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxIterations != 15 {
		t.Errorf("max_iterations: got %d, want 15", cfg.MaxIterations)
	}
	if cfg.ProbeID != "refactor-v1" {
		t.Errorf("probe_id: got %q, want refactor-v1", cfg.ProbeID)
	}
	if !cfg.TerminateOnRepeat {
		t.Error("terminate_on_repeat: got false, want true")
	}
}
