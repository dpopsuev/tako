package lint

import (
	"testing"
)

func TestExtractPath_Simple(t *testing.T) {
	doc := map[string]any{
		"calibration": map[string]any{
			"outputs": []any{
				map[string]any{"field": "output.defect_type", "scorer_name": "actual_defect_type"},
				map[string]any{"field": "output.category", "scorer_name": "actual_category"},
			},
		},
	}
	got := ExtractPath(doc, "calibration.outputs[].scorer_name")
	if len(got) != 2 {
		t.Fatalf("expected 2 values, got %d: %v", len(got), got)
	}
	if got[0] != "actual_defect_type" || got[1] != "actual_category" {
		t.Errorf("unexpected values: %v", got)
	}
}

func TestExtractPath_Wildcard(t *testing.T) {
	doc := map[string]any{
		"metrics": []any{
			map[string]any{
				"id": "M1",
				"params": map[string]any{
					"actual":   "actual_defect_type",
					"expected": "rca_defect_type",
				},
			},
		},
	}
	got := ExtractPath(doc, "metrics[].params.*")
	if len(got) != 2 {
		t.Fatalf("expected 2 values, got %d: %v", len(got), got)
	}
	// Wildcard order is non-deterministic from map, so check set membership.
	vals := map[string]bool{}
	for _, v := range got {
		vals[v.(string)] = true
	}
	if !vals["actual_defect_type"] || !vals["rca_defect_type"] {
		t.Errorf("missing expected values: %v", vals)
	}
}

func TestExtractPath_NestedArray(t *testing.T) {
	doc := map[string]any{
		"rcas": []any{
			map[string]any{"relevant_repos": []any{"repo-a", "repo-b"}},
			map[string]any{"relevant_repos": []any{"repo-c"}},
		},
	}
	got := ExtractPath(doc, "rcas[].relevant_repos[]")
	if len(got) != 3 {
		t.Fatalf("expected 3 values, got %d: %v", len(got), got)
	}
}

func TestExtractPath_Empty(t *testing.T) {
	doc := map[string]any{"other": "value"}
	got := ExtractPath(doc, "calibration.outputs[].scorer_name")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestCrossRefEngine_RefsSubsetOfExports(t *testing.T) {
	engine := &CrossRefEngine{
		Rules: []CrossRefRule{
			{
				RuleID:      "S30/calibration-fields",
				Desc:        "test",
				RuleSev:     SeverityError,
				ExportKind:  "circuit",
				ExportPath:  "calibration.outputs[].scorer_name",
				RefKind:     "scorecard",
				RefPaths:    "metrics[].params.actual,metrics[].params.expected",
				CheckType:   "refs_subset_of_exports",
				ExportLabel: "calibration contract",
				RefLabel:    "scorecard param",
			},
		},
	}

	ctx := &LintContext{
		File: "test",
		ProjectFiles: map[string][]ProjectFile{
			"circuit": {{
				File: "circuit.yaml",
				Kind: "circuit",
				Data: map[string]any{
					"calibration": map[string]any{
						"outputs": []any{
							map[string]any{"scorer_name": "actual_defect_type"},
							map[string]any{"scorer_name": "actual_category"},
						},
					},
				},
			}},
			"scorecard": {{
				File: "scorecard.yaml",
				Kind: "scorecard",
				Data: map[string]any{
					"metrics": []any{
						map[string]any{
							"id": "M1",
							"params": map[string]any{
								"actual":   "actual_defect_type",
								"expected": "rca_defect_type",
							},
						},
					},
				},
			}},
		},
	}

	// "rca_defect_type" is NOT in the calibration contract exports.
	findings := engine.Check(ctx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].RuleID != "S30/calibration-fields" {
		t.Errorf("rule = %s, want S30/calibration-fields", findings[0].RuleID)
	}
	if findings[0].Severity != SeverityError {
		t.Errorf("severity = %v, want error", findings[0].Severity)
	}
	t.Logf("finding: %s", findings[0].Message)
}

func TestCrossRefEngine_AllValid(t *testing.T) {
	engine := &CrossRefEngine{
		Rules: []CrossRefRule{
			{
				RuleID:      "S30/calibration-fields",
				Desc:        "test",
				RuleSev:     SeverityError,
				ExportKind:  "circuit",
				ExportPath:  "calibration.outputs[].scorer_name",
				RefKind:     "scorecard",
				RefPaths:    "metrics[].params.actual",
				CheckType:   "refs_subset_of_exports",
				ExportLabel: "calibration contract",
				RefLabel:    "scorecard param",
			},
		},
	}

	ctx := &LintContext{
		File: "test",
		ProjectFiles: map[string][]ProjectFile{
			"circuit": {{
				File: "circuit.yaml",
				Kind: "circuit",
				Data: map[string]any{
					"calibration": map[string]any{
						"outputs": []any{
							map[string]any{"scorer_name": "actual_defect_type"},
						},
					},
				},
			}},
			"scorecard": {{
				File: "scorecard.yaml",
				Kind: "scorecard",
				Data: map[string]any{
					"metrics": []any{
						map[string]any{
							"params": map[string]any{"actual": "actual_defect_type"},
						},
					},
				},
			}},
		},
	}

	findings := engine.Check(ctx)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestCrossRefEngine_NoProjectFiles(t *testing.T) {
	engine := &CrossRefEngine{Rules: DefaultCrossRefRules()}
	ctx := &LintContext{File: "test"}

	findings := engine.Check(ctx)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings without project files, got %d", len(findings))
	}
}

func TestLoadProjectFiles(t *testing.T) {
	files := map[string][]byte{
		"circuit.yaml": []byte(`kind: Circuit
circuit: alpha
calibration:
  outputs:
    - scorer_name: actual_defect_type`),
		"scorecard.yaml": []byte(`kind: Scorecard
scorecard: alpha
metrics:
  - id: M1
    params:
      actual: actual_defect_type`),
		"bad.yaml": []byte(`:::`),
	}

	index := LoadProjectFiles(files)
	if len(index["Circuit"]) != 1 {
		t.Errorf("expected 1 circuit, got %d", len(index["Circuit"]))
	}
	if len(index["Scorecard"]) != 1 {
		t.Errorf("expected 1 scorecard, got %d", len(index["Scorecard"]))
	}
}
