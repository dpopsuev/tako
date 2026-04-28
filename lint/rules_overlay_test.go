package lint

import (
	"strings"
	"testing"

	"github.com/dpopsuev/tako/circuit"
)

func TestImportOverlay_EmptyImport(t *testing.T) {
	yml := []byte(`
circuit: overlay-test
description: overlay with empty import
import:
nodes:
  - name: recall
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: recall to done
    from: recall
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var found bool
	for _, f := range findings {
		if f.RuleID == "S18/import-overlay" && strings.Contains(f.Message, "value is empty") {
			found = true
			if f.Severity != SeverityError {
				t.Errorf("expected Error severity for empty import, got %s", f.Severity)
			}
			break
		}
	}
	if !found {
		t.Error("expected S18/import-overlay finding for empty import")
	}
}

func TestImportOverlay_RedefinesStartDone(t *testing.T) {
	yml := []byte(`
circuit: overlay-test
description: overlay redefining start
import: base-circuit
start: custom_start
done: _done
nodes:
  - name: recall
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: recall to done
    from: recall
    to: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var startFound bool
	for _, f := range findings {
		if f.RuleID == "S18/import-overlay" && strings.Contains(f.Message, "redefines start") {
			startFound = true
			break
		}
	}
	if !startFound {
		t.Error("expected S18/import-overlay warning for redefining start")
	}

	// overlay with done redefinition
	yml2 := []byte(`
circuit: overlay-test
description: overlay redefining done
import: base-circuit
start: recall
done: custom_done
nodes:
  - name: recall
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: recall to done
    from: recall
    to: custom_done
`)
	findings2, err := Run(yml2, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var doneFound bool
	for _, f := range findings2 {
		if f.RuleID == "S18/import-overlay" && strings.Contains(f.Message, "redefines done") {
			doneFound = true
			break
		}
	}
	if !doneFound {
		t.Error("expected S18/import-overlay warning for redefining done")
	}
}

func TestImportOverlay_ValidImportNoStartDone(t *testing.T) {
	yml := []byte(`
circuit: overlay-test
description: overlay with valid import, no start/done override
import: base-circuit
ports:
  - name: extra
    direction: out
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "S18/import-overlay" {
			t.Errorf("unexpected S18 finding for valid overlay: %s", f.Message)
		}
	}
}

func TestPortValidation_InvalidDirection(t *testing.T) {
	yml := []byte(`
circuit: port-test
description: test
instrument: transformer
nodes:
  - name: recall
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
  - name: done
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: recall to done
    from: recall
    to: done
start: recall
done: done
ports:
  - name: bad
    direction: invalid
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var found bool
	for _, f := range findings {
		if f.RuleID == "S19/port-validation" && strings.Contains(f.Message, "invalid") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected S19/port-validation finding for invalid direction")
	}
}

func TestPortValidation_DuplicateNames(t *testing.T) {
	yml := []byte(`
circuit: port-test
description: test
instrument: transformer
nodes:
  - name: recall
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
  - name: done
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: recall to done
    from: recall
    to: done
start: recall
done: done
ports:
  - name: dup
    direction: in
  - name: dup
    direction: out
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var found bool
	for _, f := range findings {
		if f.RuleID == "S19/port-validation" && strings.Contains(f.Message, "duplicated") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected S19/port-validation finding for duplicate port names")
	}
}

func TestPortValidation_ValidPorts(t *testing.T) {
	yml := []byte(`
circuit: port-test
description: test
instrument: transformer
nodes:
  - name: recall
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
  - name: done
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: recall to done
    from: recall
    to: done
start: recall
done: done
ports:
  - name: in1
    direction: in
  - name: out1
    direction: out
  - name: loop1
    direction: loop
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "S19/port-validation" {
			t.Errorf("unexpected S19 finding for valid ports: %s", f.Message)
		}
	}
}

func TestCalibrationContract_MissingFieldOrScorer(t *testing.T) {
	yml := []byte(`
circuit: cal-test
description: test
instrument: transformer
nodes:
  - name: recall
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
  - name: done
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: recall to done
    from: recall
    to: done
start: recall
done: done
calibration:
  inputs:
    - field: input
      scorer_name: input_scorer
    - field: ""
      scorer_name: missing_field
  outputs:
    - field: result
      scorer_name: ""
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var inputFound, outputFound bool
	for _, f := range findings {
		if f.RuleID == "S20/calibration-contract" {
			if strings.Contains(f.Message, "input") && strings.Contains(f.Message, "field") {
				inputFound = true
			}
			if strings.Contains(f.Message, "output") && strings.Contains(f.Message, "scorer_name") {
				outputFound = true
			}
		}
	}
	if !inputFound {
		t.Error("expected S20 finding for calibration input missing field")
	}
	if !outputFound {
		t.Error("expected S20 finding for calibration output missing scorer_name")
	}
}

func TestCalibrationContract_ValidCalibration(t *testing.T) {
	yml := []byte(`
circuit: cal-test
description: test
instrument: transformer
nodes:
  - name: recall
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
  - name: done
    approach: rapid
    action: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: recall to done
    from: recall
    to: done
start: recall
done: done
calibration:
  inputs:
    - field: input
      scorer_name: input_scorer
  outputs:
    - field: result
      scorer_name: result_scorer
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "S20/calibration-contract" {
			t.Errorf("unexpected S20 finding for valid calibration: %s", f.Message)
		}
	}
}

func TestCalibrationContract_NoCalibration(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit:     "test",
		Description: "test",
		Nodes:       []circuit.NodeDef{{Name: "a", Approach: "rapid", Action: "core.jq"}},
		Edges:       []circuit.EdgeDef{{ID: "e1", From: "a", To: "_done"}},
		Start:       "a",
		Done:        "_done",
	}
	ctx := NewLintContextFromDef(def, "inline")
	r := &CalibrationContract{}
	findings := r.Check(ctx)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when no calibration, got %d", len(findings))
	}
}
