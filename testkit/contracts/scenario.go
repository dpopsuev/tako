package contracts

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/calibrate"
)

// RunScenarioLoaderContract runs the ScenarioLoader compliance suite.
// The factory must return a ready-to-use loader that produces at least
// one case. Each factory call should return a fresh instance.
func RunScenarioLoaderContract(t *testing.T, factory func() calibrate.ScenarioLoader) {
	t.Helper()

	t.Run("Load_ReturnsNonEmptyCases", func(t *testing.T) {
		loader := factory()
		cases, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		if len(cases) == 0 {
			t.Error("Load must return at least one case")
		}
	})

	t.Run("Load_AllCasesHaveUniqueIDs", func(t *testing.T) {
		loader := factory()
		cases, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		seen := make(map[string]bool, len(cases))
		for _, c := range cases {
			if seen[c.ID] {
				t.Errorf("duplicate case ID %q", c.ID)
			}
			seen[c.ID] = true
		}
	})

	t.Run("Load_SecondCallReturnsFreshData", func(t *testing.T) {
		loader := factory()
		cases1, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("first Load: %v", err)
		}
		cases2, err := loader.Load(context.Background())
		if err != nil {
			t.Fatalf("second Load: %v", err)
		}
		if len(cases1) != len(cases2) {
			t.Errorf("second Load returned %d cases, first returned %d — expected same count",
				len(cases2), len(cases1))
		}
		// Verify case IDs are consistent across calls.
		for i := range cases1 {
			if i < len(cases2) && cases1[i].ID != cases2[i].ID {
				t.Errorf("case %d ID changed between calls: %q vs %q",
					i, cases1[i].ID, cases2[i].ID)
			}
		}
	})
}
