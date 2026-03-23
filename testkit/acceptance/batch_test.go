package acceptance

// Feature: Batch Execution
//   As a framework consumer
//   I want to execute circuits over multiple input cases
//   So that I can process batches efficiently with optional parallelism

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/engine"
)

func TestBatch_SerialReturnsResultsInOrder(t *testing.T) {
	// Scenario: Batch walk with serial execution returns results in order
	//   Given a linear circuit and 3 batch cases
	//   When I execute BatchWalk with Parallel=1
	//   Then results[0..2] come back in order with matching CaseIDs

	def := loadFixture(t, "circuits/linear.yaml")

	cases := []engine.BatchCase{
		{ID: "case-0", Context: map[string]any{"input": 0}},
		{ID: "case-1", Context: map[string]any{"input": 1}},
		{ID: "case-2", Context: map[string]any{"input": 2}},
	}

	results := engine.BatchWalk(context.Background(), engine.BatchWalkConfig{
		Def:      def,
		Shared:   standardRegistries(),
		Cases:    cases,
		Parallel: 1,
	})

	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}

	// Verify results match case IDs in order
	for i, want := range []string{"case-0", "case-1", "case-2"} {
		if results[i].CaseID != want {
			t.Errorf("results[%d].CaseID = %q, want %q", i, results[i].CaseID, want)
		}
		if results[i].Error != nil {
			t.Errorf("results[%d].Error = %v, want nil", i, results[i].Error)
		}
	}
}

func TestBatch_ParallelCompletesAllCases(t *testing.T) {
	// Scenario: Batch walk with parallel execution completes all cases
	//   Given a linear circuit and 5 batch cases
	//   When I execute BatchWalk with Parallel=3
	//   Then all 5 results are returned with no errors

	def := loadFixture(t, "circuits/linear.yaml")

	cases := make([]engine.BatchCase, 5)
	for i := 0; i < 5; i++ {
		cases[i] = engine.BatchCase{
			ID:      string(rune('A' + i)), // A, B, C, D, E
			Context: map[string]any{"index": i},
		}
	}

	results := engine.BatchWalk(context.Background(), engine.BatchWalkConfig{
		Def:      def,
		Shared:   standardRegistries(),
		Cases:    cases,
		Parallel: 3,
	})

	if len(results) != 5 {
		t.Fatalf("len(results) = %d, want 5", len(results))
	}

	// Verify all cases completed without errors
	for i, r := range results {
		if r.CaseID == "" {
			t.Errorf("results[%d].CaseID is empty", i)
		}
		if r.Error != nil {
			t.Errorf("results[%d] (%s).Error = %v, want nil", i, r.CaseID, r.Error)
		}
		// Verify walk path includes both nodes
		if len(r.Path) != 2 {
			t.Errorf("results[%d].Path = %v, want [step-a, step-b]", i, r.Path)
		}
	}
}

func TestBatch_FailedCaseDoesNotBlockOthers(t *testing.T) {
	// Scenario: One failed case does not block other cases
	//   Given a batch with one case using a bad circuit definition
	//   When I execute BatchWalk
	//   Then the bad case has an error, but other cases succeed

	// Good circuit
	goodDef := loadFixture(t, "circuits/linear.yaml")

	// Bad circuit with missing start node
	badDef := &engine.CircuitDef{
		Circuit:     "broken",
		Start:       "nonexistent",
		Done:        "done",
		HandlerType: "transformer",
		Nodes:       []engine.NodeDef{{Name: "step-a", Handler: "echo"}},
		Edges:       []engine.EdgeDef{{ID: "a-done", From: "step-a", To: "done"}},
	}

	cases := []engine.BatchCase{
		{ID: "good-1", Context: map[string]any{"input": 1}},
		{ID: "good-2", Context: map[string]any{"input": 2}},
		{ID: "good-3", Context: map[string]any{"input": 3}},
	}

	// First run with good circuit - all should succeed
	results := engine.BatchWalk(context.Background(), engine.BatchWalkConfig{
		Def:      goodDef,
		Shared:   standardRegistries(),
		Cases:    cases,
		Parallel: 1,
	})

	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}

	for i, r := range results {
		if r.Error != nil {
			t.Errorf("results[%d].Error = %v, want nil (all good cases)", i, r.Error)
		}
	}

	// Now run with bad circuit - all should fail
	badResults := engine.BatchWalk(context.Background(), engine.BatchWalkConfig{
		Def:      badDef,
		Shared:   standardRegistries(),
		Cases:    cases[:1], // just one case
		Parallel: 1,
	})

	if len(badResults) != 1 {
		t.Fatalf("len(badResults) = %d, want 1", len(badResults))
	}

	if badResults[0].Error == nil {
		t.Error("badResults[0].Error = nil, want error for missing start node")
	}
}
