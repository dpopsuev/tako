package engine

import (
	"testing"

	"github.com/dpopsuev/tako/circuit"
)

type stubMergeArtifact struct{ val string }

func (a *stubMergeArtifact) Type() string        { return "merge-test" }
func (a *stubMergeArtifact) Confidence() float64 { return 1.0 }
func (a *stubMergeArtifact) Raw() any            { return a.val }

func TestApplyMergeStrategy_Append(t *testing.T) {
	results := []branchResult{
		{nodeName: "a", artifact: &stubMergeArtifact{val: "a"}},
		{nodeName: "b", artifact: &stubMergeArtifact{val: "b"}},
	}
	merged := applyMergeStrategy(circuit.MergeAppend, results)
	la, ok := merged.(*ListArtifact)
	if !ok {
		t.Fatalf("expected *ListArtifact, got %T", merged)
	}
	if len(la.Items) != 2 {
		t.Errorf("want 2 items, got %d", len(la.Items))
	}
}

func TestApplyMergeStrategy_Latest(t *testing.T) {
	results := []branchResult{
		{nodeName: "a", artifact: &stubMergeArtifact{val: "first"}},
		{nodeName: "b", artifact: &stubMergeArtifact{val: "second"}},
	}
	merged := applyMergeStrategy(circuit.MergeLatest, results)
	if merged.(*stubMergeArtifact).val != "second" {
		t.Errorf("want second, got %v", merged.Raw())
	}
}

func TestApplyMergeStrategy_Custom(t *testing.T) {
	results := []branchResult{
		{nodeName: "a", artifact: &stubMergeArtifact{val: "first"}},
		{nodeName: "b", artifact: &stubMergeArtifact{val: "second"}},
	}
	merged := applyMergeStrategy(circuit.MergeCustom, results)
	if merged.(*stubMergeArtifact).val != "first" {
		t.Errorf("custom should return first, got %v", merged.Raw())
	}
}

func TestApplyMergeStrategy_Default(t *testing.T) {
	results := []branchResult{
		{nodeName: "a", artifact: &stubMergeArtifact{val: "first"}},
	}
	merged := applyMergeStrategy("", results)
	if merged.(*stubMergeArtifact).val != "first" {
		t.Errorf("default should return first, got %v", merged.Raw())
	}
}

func TestApplyMergeStrategy_Empty(t *testing.T) {
	merged := applyMergeStrategy(circuit.MergeAppend, nil)
	if merged != nil {
		t.Errorf("empty results should return nil, got %v", merged)
	}
}

func TestListArtifact(t *testing.T) {
	items := []circuit.Artifact{
		&stubMergeArtifact{val: "a"},
		&stubMergeArtifact{val: "b"},
	}
	la := &ListArtifact{Items: items}
	if la.Type() != "list" {
		t.Errorf("Type() = %q, want list", la.Type())
	}
	if la.Confidence() != 0 {
		t.Errorf("Confidence() = %f, want 0", la.Confidence())
	}
	raw := la.Raw().([]circuit.Artifact)
	if len(raw) != 2 {
		t.Errorf("len(Raw()) = %d, want 2", len(raw))
	}
}
