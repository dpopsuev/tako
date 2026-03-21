package contracts

import (
	"context"
	"testing"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/calibrate"
)

// RunCaseCollectorContract runs the CaseCollector compliance suite.
// The factory must return a ready-to-use collector.
func RunCaseCollectorContract(t *testing.T, factory func() calibrate.CaseCollector) {
	t.Helper()

	t.Run("Collect_ReturnsValuesMap", func(t *testing.T) {
		collector := factory()
		results := []framework.BatchWalkResult{
			{
				CaseID:        "C01",
				Path:          []string{"start", "done"},
				StepArtifacts: map[string]framework.Artifact{},
			},
		}
		values, _, err := collector.Collect(context.Background(), results)
		if err != nil {
			t.Fatalf("Collect returned error: %v", err)
		}
		if values == nil {
			t.Error("Collect must return a non-nil values map")
		}
	})

	t.Run("Collect_EmptyResults_NoPanic", func(t *testing.T) {
		collector := factory()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Collect panicked on empty results: %v", r)
				}
			}()
			_, _, _ = collector.Collect(context.Background(), nil)
		}()
	})

	t.Run("Collect_EmptySlice_NoPanic", func(t *testing.T) {
		collector := factory()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Collect panicked on empty slice: %v", r)
				}
			}()
			_, _, _ = collector.Collect(context.Background(), []framework.BatchWalkResult{})
		}()
	})
}
