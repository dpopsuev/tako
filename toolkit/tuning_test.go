package toolkit

import (
	"errors"
	"testing"
)

func TestLoadQuickWins_ValidYAML(t *testing.T) {
	t.Parallel()
	data := []byte(`
quick_wins:
  - id: qw1
    name: First QW
    description: do something
    metric_target: M1
  - id: qw2
    name: Second QW
    description: do another
    metric_target: M2
    prereqs: [qw1]
`)
	qws := LoadQuickWins(data)
	if len(qws) != 2 {
		t.Fatalf("len = %d, want 2", len(qws))
	}
	if qws[0].ID != "qw1" || qws[0].Name != "First QW" {
		t.Errorf("qw[0] = %+v", qws[0])
	}
	if len(qws[1].Prereqs) != 1 || qws[1].Prereqs[0] != "qw1" {
		t.Errorf("qw[1].Prereqs = %v", qws[1].Prereqs)
	}
}

func TestLoadQuickWins_InvalidYAML(t *testing.T) {
	t.Parallel()
	qws := LoadQuickWins([]byte("{invalid"))
	if qws != nil {
		t.Errorf("invalid YAML should return nil, got %v", qws)
	}
}

func TestLoadQuickWins_Empty(t *testing.T) {
	t.Parallel()
	qws := LoadQuickWins([]byte("quick_wins: []"))
	if len(qws) != 0 {
		t.Errorf("empty list should return empty slice, got %v", qws)
	}
}

func TestNewTuningRunner_Defaults(t *testing.T) {
	t.Parallel()
	runner := NewTuningRunner(nil, 0.90)
	if runner.TargetVal != 0.90 {
		t.Errorf("TargetVal = %v, want 0.90", runner.TargetVal)
	}
	if runner.MaxNoImprove != 3 {
		t.Errorf("MaxNoImprove = %d, want 3", runner.MaxNoImprove)
	}
}

func TestTuningRunner_AllExhausted(t *testing.T) {
	t.Parallel()
	qws := []QuickWin{
		{ID: "qw1", Apply: func() error { return nil }},
		{ID: "qw2", Apply: func() error { return nil }},
	}
	runner := NewTuningRunner(qws, 0.95)
	report := runner.Run(0.50)

	if report.BaselineVal != 0.50 {
		t.Errorf("BaselineVal = %v, want 0.50", report.BaselineVal)
	}
	if report.StopReason != "all quick wins exhausted" {
		t.Errorf("StopReason = %q", report.StopReason)
	}
	if len(report.Results) != 2 {
		t.Errorf("Results len = %d, want 2", len(report.Results))
	}
}

func TestTuningRunner_TargetReached(t *testing.T) {
	t.Parallel()
	qws := []QuickWin{
		{ID: "qw1", Apply: func() error { return nil }},
		{ID: "qw2", Apply: func() error { return nil }},
	}
	runner := NewTuningRunner(qws, 0.40)
	report := runner.Run(0.50)

	if report.StopReason == "" {
		t.Error("expected target-reached stop reason")
	}
	if len(report.Results) != 0 {
		t.Errorf("should stop before running any QW, got %d results", len(report.Results))
	}
}

func TestTuningRunner_ApplyError(t *testing.T) {
	t.Parallel()
	qws := []QuickWin{
		{ID: "fail", Apply: func() error { return errors.New("broken") }},
	}
	runner := NewTuningRunner(qws, 0.95)
	report := runner.Run(0.50)

	if len(report.Results) != 1 {
		t.Fatalf("Results len = %d, want 1", len(report.Results))
	}
	if report.Results[0].Error != "broken" {
		t.Errorf("Error = %q, want broken", report.Results[0].Error)
	}
}

func TestTuningRunner_NilApply(t *testing.T) {
	t.Parallel()
	qws := []QuickWin{
		{ID: "noop"},
	}
	runner := NewTuningRunner(qws, 0.95)
	report := runner.Run(0.50)

	if len(report.Results) != 1 {
		t.Fatalf("Results len = %d, want 1", len(report.Results))
	}
	if report.Results[0].Error != "not yet implemented" {
		t.Errorf("Error = %q, want 'not yet implemented'", report.Results[0].Error)
	}
}

func TestTuningRunner_NoImproveStreak(t *testing.T) {
	t.Parallel()
	callCount := 0
	qws := make([]QuickWin, 5)
	for i := range qws {
		qws[i] = QuickWin{
			ID: "qw",
			Apply: func() error {
				callCount++
				return errors.New("always fails")
			},
		}
	}
	runner := NewTuningRunner(qws, 0.95)
	runner.MaxNoImprove = 2
	report := runner.Run(0.50)

	if callCount != 2 {
		t.Errorf("should stop after %d failures, called %d", runner.MaxNoImprove, callCount)
	}
	if report.StopReason == "" || report.StopReason == "all quick wins exhausted" {
		t.Errorf("expected no-improvement stop reason, got %q", report.StopReason)
	}
}

func TestTuningReport_Fields(t *testing.T) {
	t.Parallel()
	r := TuningReport{
		BaselineVal:     0.50,
		FinalVal:        0.75,
		CumulativeDelta: 0.25,
		QWsApplied:      3,
		QWsReverted:     1,
		StopReason:      "target reached",
	}
	if r.CumulativeDelta != 0.25 {
		t.Errorf("CumulativeDelta = %v", r.CumulativeDelta)
	}
}
