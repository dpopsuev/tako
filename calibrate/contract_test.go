package calibrate

import (
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
)

type fakeArtifact struct{ data any }

func (f fakeArtifact) Type() string        { return "test" }
func (f fakeArtifact) Confidence() float64 { return 1.0 }
func (f fakeArtifact) Raw() any            { return f.data }

func TestExtractFields_BasicOutputMapping(t *testing.T) {
	contract := &CalibrationContract{
		Outputs: []ContractField{
			{Field: "investigate.defect_type", ScorerName: "actual_defect_type", Type: "string"},
			{Field: "investigate.convergence_score", ScorerName: "actual_convergence", Type: "float"},
			{Field: "triage.symptom_category", ScorerName: "actual_category", Type: "string"},
		},
	}
	result := engine.BatchWalkResult{
		Path: []string{"recall", "triage", "investigate"},
		StepArtifacts: map[string]circuit.Artifact{
			"investigate": fakeArtifact{data: map[string]any{
				"defect_type":       "pb001",
				"convergence_score": 0.85,
			}},
			"triage": fakeArtifact{data: map[string]any{
				"symptom_category": "product_bug",
			}},
		},
	}

	fields := ExtractFields(contract, result)

	if fields["actual_defect_type"] != "pb001" {
		t.Errorf("defect_type: got %v, want pb001", fields["actual_defect_type"])
	}
	if fields["actual_convergence"] != 0.85 {
		t.Errorf("convergence: got %v, want 0.85", fields["actual_convergence"])
	}
	if fields["actual_category"] != "product_bug" {
		t.Errorf("category: got %v, want product_bug", fields["actual_category"])
	}
	if fields["_path"] == nil {
		t.Error("_path should always be set")
	}
}

func TestExtractFields_MissingNode(t *testing.T) {
	contract := &CalibrationContract{
		Outputs: []ContractField{
			{Field: "resolve.selected_repos", ScorerName: "actual_repos", Type: "array"},
		},
	}
	result := engine.BatchWalkResult{
		StepArtifacts: map[string]circuit.Artifact{},
	}

	fields := ExtractFields(contract, result)

	if _, ok := fields["actual_repos"]; ok {
		t.Error("missing node should not produce a field")
	}
}

func TestExtractFields_NestedPath(t *testing.T) {
	contract := &CalibrationContract{
		Outputs: []ContractField{
			{Field: "investigate.gap_brief.verdict", ScorerName: "verdict_confidence", Type: "string"},
		},
	}
	result := engine.BatchWalkResult{
		StepArtifacts: map[string]circuit.Artifact{
			"investigate": fakeArtifact{data: map[string]any{
				"gap_brief": map[string]any{
					"verdict": "confident",
				},
			}},
		},
	}

	fields := ExtractFields(contract, result)

	if fields["verdict_confidence"] != "confident" {
		t.Errorf("nested path: got %v, want confident", fields["verdict_confidence"])
	}
}

func TestExtractFields_StatePath(t *testing.T) {
	contract := &CalibrationContract{
		Outputs: []ContractField{
			{Field: "state.loops.investigate", ScorerName: "actual_loops", Type: "float"},
			{Field: "state.status", ScorerName: "walk_status", Type: "string"},
		},
	}
	result := engine.BatchWalkResult{
		State: &circuit.WalkerState{
			LoopCounts:  map[string]int{"investigate": 2},
			CurrentNode: "done",
			Status:      "complete",
		},
	}

	fields := ExtractFields(contract, result)

	if fields["actual_loops"] != 2 {
		t.Errorf("loops: got %v, want 2", fields["actual_loops"])
	}
	if fields["walk_status"] != "complete" {
		t.Errorf("status: got %v, want complete", fields["walk_status"])
	}
}

func TestExtractFields_NilContract(t *testing.T) {
	result := engine.BatchWalkResult{}
	fields := ExtractFields(nil, result)
	if fields != nil {
		t.Error("nil contract should return nil")
	}
}

func TestFoldContracts_SingleCircuit(t *testing.T) {
	single := &CalibrationContract{
		Outputs: []ContractField{{Field: "a.b", ScorerName: "x", Type: "string"}},
	}
	folded := FoldContracts(map[string]*CalibrationContract{"alpha": single})
	if folded != single {
		t.Error("single circuit should return the contract as-is")
	}
}

func TestFoldContracts_MultiCircuit(t *testing.T) {
	alphaContract := &CalibrationContract{
		Inputs:  []ContractField{{Field: "inp.a", ScorerName: "gt_a", Type: "string"}},
		Outputs: []ContractField{{Field: "inv.dt", ScorerName: "defect_type", Type: "string"}},
	}
	harv := &CalibrationContract{
		Outputs: []ContractField{{Field: "gather.files", ScorerName: "files_found", Type: "array"}},
	}
	folded := FoldContracts(map[string]*CalibrationContract{
		"alpha": alphaContract,
		"beta":  harv,
	})

	if len(folded.Inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(folded.Inputs))
	}
	if len(folded.Outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(folded.Outputs))
	}

	outNames := map[string]bool{}
	for _, o := range folded.Outputs {
		outNames[o.ScorerName] = true
	}
	if !outNames["alpha.defect_type"] {
		t.Error("missing alpha.defect_type")
	}
	if !outNames["beta.files_found"] {
		t.Error("missing beta.files_found")
	}
}

func TestFoldContracts_Empty(t *testing.T) {
	folded := FoldContracts(map[string]*CalibrationContract{})
	if folded != nil {
		t.Error("empty map should return nil")
	}
}

func TestContractFromDef_Nil(t *testing.T) {
	if ContractFromDef(nil) != nil {
		t.Error("nil def should return nil")
	}
}

func TestExtractFields_ArrayProjection(t *testing.T) {
	contract := &CalibrationContract{
		Outputs: []ContractField{
			{Field: "read.files[].path", ScorerName: "actual_files", Type: "array"},
			{Field: "tree.trees[].repo", ScorerName: "actual_repos", Type: "array"},
		},
	}
	result := engine.BatchWalkResult{
		StepArtifacts: map[string]circuit.Artifact{
			"read": fakeArtifact{data: map[string]any{
				"files": []any{
					map[string]any{"repo": "linuxptp-daemon", "path": "daemon.go"},
					map[string]any{"repo": "linuxptp-daemon", "path": "process.go"},
					map[string]any{"repo": "cloud-event-proxy", "path": "main.go"},
				},
			}},
			"tree": fakeArtifact{data: map[string]any{
				"trees": []any{
					map[string]any{"repo": "linuxptp-daemon", "branch": "main"},
					map[string]any{"repo": "cloud-event-proxy", "branch": "main"},
				},
			}},
		},
	}

	fields := ExtractFields(contract, result)

	files, ok := fields["actual_files"].([]any)
	if !ok {
		t.Fatalf("actual_files type = %T, want []any", fields["actual_files"])
	}
	if len(files) != 3 {
		t.Fatalf("actual_files len = %d, want 3", len(files))
	}
	if files[0] != "daemon.go" || files[1] != "process.go" || files[2] != "main.go" {
		t.Errorf("actual_files = %v", files)
	}

	repos, ok := fields["actual_repos"].([]any)
	if !ok {
		t.Fatalf("actual_repos type = %T, want []any", fields["actual_repos"])
	}
	if len(repos) != 2 {
		t.Fatalf("actual_repos len = %d, want 2", len(repos))
	}
}

func TestContractFromDef_Converts(t *testing.T) {
	def := &circuit.CalibrationContractDef{
		Outputs: []circuit.CalibrationFieldDef{
			{Field: "investigate.defect_type", ScorerName: "actual_defect_type", Type: "string"},
		},
	}
	c := ContractFromDef(def)
	if len(c.Outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(c.Outputs))
	}
	if c.Outputs[0].ScorerName != "actual_defect_type" {
		t.Errorf("scorer name: got %s, want actual_defect_type", c.Outputs[0].ScorerName)
	}
}
