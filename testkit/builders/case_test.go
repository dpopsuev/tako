package builders_test

import (
	"testing"

	"github.com/dpopsuev/tako/testkit/builders"
)

func TestBatchCaseBuilder_Basic(t *testing.T) {
	bc := builders.NewBatchCase("C01").
		WithInput("key", "val").
		Build()

	if bc.ID != "C01" {
		t.Errorf("ID = %q, want %q", bc.ID, "C01")
	}
	if bc.Context["key"] != "val" {
		t.Errorf("Context[key] = %v, want %q", bc.Context["key"], "val")
	}
}

func TestBatchCaseBuilder_WithExpected(t *testing.T) {
	bc := builders.NewBatchCase("C01").
		WithInput("source", "test.log").
		WithExpected("metric", 1.0).
		WithExpected("label", "pass").
		Build()

	expected, ok := bc.Context["expected"].(map[string]any)
	if !ok {
		t.Fatal("expected key should contain map[string]any")
	}
	if expected["metric"] != 1.0 {
		t.Errorf("expected[metric] = %v, want 1.0", expected["metric"])
	}
	if expected["label"] != "pass" {
		t.Errorf("expected[label] = %v, want %q", expected["label"], "pass")
	}
}

func TestBatchCaseBuilder_MultipleInputs(t *testing.T) {
	bc := builders.NewBatchCase("C02").
		WithInput("a", 1).
		WithInput("b", 2).
		WithInput("c", 3).
		Build()

	if len(bc.Context) != 3 {
		t.Errorf("got %d context keys, want 3", len(bc.Context))
	}
}

func TestBatchCaseBuilder_EmptyCase(t *testing.T) {
	bc := builders.NewBatchCase("C03").Build()

	if bc.ID != "C03" {
		t.Errorf("ID = %q, want %q", bc.ID, "C03")
	}
	if bc.Context == nil {
		t.Error("Context should be initialized, got nil")
	}
}
