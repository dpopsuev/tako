package circuit

import (
	"testing"
)

func TestLoadScorecardDef_BasicParsing(t *testing.T) {
	data := []byte(`
name: test-scorecard
description: A test scorecard
metrics:
  - id: m1
    name: Metric One
    tier: outcome
    threshold: 0.9
    weight: 0.5
  - id: m2
    name: Metric Two
    direction: lower_is_better
    threshold: 100
cost_model:
  token_cost: 0.002
  time_cost: 0.1
`)
	sd, err := LoadScorecardDef(data)
	if err != nil {
		t.Fatalf("LoadScorecardDef: %v", err)
	}
	if sd.Name != "test-scorecard" {
		t.Errorf("Name: got %q, want test-scorecard", sd.Name)
	}
	if sd.Description != "A test scorecard" {
		t.Errorf("Description: got %q, want A test scorecard", sd.Description)
	}
	if len(sd.Metrics) != 2 {
		t.Fatalf("Metrics: got %d, want 2", len(sd.Metrics))
	}
	if sd.Metrics[0].ID != "m1" || sd.Metrics[0].Threshold != 0.9 || sd.Metrics[0].Weight != 0.5 {
		t.Errorf("Metrics[0]: got id=%q threshold=%v weight=%v", sd.Metrics[0].ID, sd.Metrics[0].Threshold, sd.Metrics[0].Weight)
	}
	if sd.Metrics[1].ID != "m2" || sd.Metrics[1].Threshold != 100 {
		t.Errorf("Metrics[1]: got id=%q threshold=%v", sd.Metrics[1].ID, sd.Metrics[1].Threshold)
	}
	if sd.CostModel == nil || sd.CostModel.TokenCost != 0.002 || sd.CostModel.TimeCost != 0.1 {
		t.Errorf("CostModel: got %+v", sd.CostModel)
	}
}

func TestMergeScorecardDefs_TuneExistingMetrics(t *testing.T) {
	base := &ScorecardDef{
		Name: "base",
		Metrics: []ScorecardMetric{
			{ID: "m1", Name: "Metric One", Threshold: 0.8, Weight: 0.3},
			{ID: "m2", Name: "Metric Two", Threshold: 50, Weight: 0.7},
		},
	}
	overlay := &ScorecardDef{
		Import: "base",
		Metrics: []ScorecardMetric{
			{ID: "m1", Threshold: 0.95, Weight: 0.5},
			{ID: "m2", Threshold: 30},
		},
	}
	merged, err := MergeScorecardDefs(base, overlay)
	if err != nil {
		t.Fatalf("MergeScorecardDefs: %v", err)
	}
	if merged.Name != "base" {
		t.Errorf("Name: got %q, want base", merged.Name)
	}
	if len(merged.Metrics) != 2 {
		t.Fatalf("Metrics: got %d, want 2", len(merged.Metrics))
	}
	// m1: threshold and weight overridden
	if merged.Metrics[0].Threshold != 0.95 || merged.Metrics[0].Weight != 0.5 {
		t.Errorf("m1: got threshold=%v weight=%v, want 0.95 0.5", merged.Metrics[0].Threshold, merged.Metrics[0].Weight)
	}
	// m2: threshold overridden, weight unchanged (overlay had 0)
	if merged.Metrics[1].Threshold != 30 || merged.Metrics[1].Weight != 0.7 {
		t.Errorf("m2: got threshold=%v weight=%v, want 30 0.7", merged.Metrics[1].Threshold, merged.Metrics[1].Weight)
	}
}

func TestMergeScorecardDefs_AddNewMetrics(t *testing.T) {
	base := &ScorecardDef{
		Name: "base",
		Metrics: []ScorecardMetric{
			{ID: "m1", Name: "Metric One", Threshold: 0.8},
		},
	}
	overlay := &ScorecardDef{
		Import: "base",
		Metrics: []ScorecardMetric{
			{ID: "m1", Threshold: 0.9},
			{ID: "m2", Name: "Custom Metric", Threshold: 0.5, Weight: 0.2},
		},
	}
	merged, err := MergeScorecardDefs(base, overlay)
	if err != nil {
		t.Fatalf("MergeScorecardDefs: %v", err)
	}
	if len(merged.Metrics) != 2 {
		t.Fatalf("Metrics: got %d, want 2", len(merged.Metrics))
	}
	if merged.Metrics[0].ID != "m1" || merged.Metrics[0].Threshold != 0.9 {
		t.Errorf("m1: got id=%q threshold=%v", merged.Metrics[0].ID, merged.Metrics[0].Threshold)
	}
	if merged.Metrics[1].ID != "m2" || merged.Metrics[1].Name != "Custom Metric" || merged.Metrics[1].Threshold != 0.5 {
		t.Errorf("m2: got id=%q name=%q threshold=%v", merged.Metrics[1].ID, merged.Metrics[1].Name, merged.Metrics[1].Threshold)
	}
}

func TestMergeScorecardDefs_CostModelOverride(t *testing.T) {
	base := &ScorecardDef{
		Name: "base",
		CostModel: &CostModelDef{TokenCost: 0.001, TimeCost: 0.05},
		Metrics:   []ScorecardMetric{{ID: "m1", Name: "M1"}},
	}
	overlay := &ScorecardDef{
		Import: "base",
		CostModel: &CostModelDef{TokenCost: 0.003, TimeCost: 0.2},
		Metrics:   []ScorecardMetric{},
	}
	merged, err := MergeScorecardDefs(base, overlay)
	if err != nil {
		t.Fatalf("MergeScorecardDefs: %v", err)
	}
	if merged.CostModel == nil {
		t.Fatal("CostModel: got nil")
	}
	if merged.CostModel.TokenCost != 0.003 || merged.CostModel.TimeCost != 0.2 {
		t.Errorf("CostModel: got token_cost=%v time_cost=%v, want 0.003 0.2", merged.CostModel.TokenCost, merged.CostModel.TimeCost)
	}
}

func TestMergeScorecardDefs_OverlayWithoutImportReturnsAsIs(t *testing.T) {
	base := &ScorecardDef{
		Name:    "base",
		Metrics: []ScorecardMetric{{ID: "m1", Name: "M1"}},
	}
	overlay := &ScorecardDef{
		Name: "standalone",
		Metrics: []ScorecardMetric{
			{ID: "m2", Name: "Standalone Metric", Threshold: 1.0},
		},
	}
	merged, err := MergeScorecardDefs(base, overlay)
	if err != nil {
		t.Fatalf("MergeScorecardDefs: %v", err)
	}
	if merged != overlay {
		t.Error("expected overlay returned as-is when Import is empty")
	}
	if merged.Name != "standalone" || len(merged.Metrics) != 1 || merged.Metrics[0].ID != "m2" {
		t.Errorf("merged: got name=%q metrics=%d id=%q", merged.Name, len(merged.Metrics), merged.Metrics[0].ID)
	}
}

func TestRegisterScorecardVocabulary(t *testing.T) {
	sd := &ScorecardDef{
		Metrics: []ScorecardMetric{
			{ID: "m1", Name: "Metric One", DisplayName: "Primary Outcome"},
			{ID: "m2", Name: "Metric Two"},
		},
	}
	v := NewRichMapVocabulary()
	RegisterScorecardVocabulary(sd, v)
	if v.Name("m1") != "Primary Outcome" {
		t.Errorf("m1: got %q, want Primary Outcome", v.Name("m1"))
	}
	if v.Name("m2") != "Metric Two" {
		t.Errorf("m2: got %q, want Metric Two (fallback to Name)", v.Name("m2"))
	}
	if v.Name("m3") != "m3" {
		t.Errorf("m3: got %q, want m3 (unregistered pass-through)", v.Name("m3"))
	}
}
