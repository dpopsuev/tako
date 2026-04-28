package lint

import (
	"testing"
)

const validInstrumentYAML = `apiVersion: tako/v1
kind: Instrument
metadata:
  name: test-tool
  namespace: testkit
spec:
  dispatch: cli
  tune: "which test-tool"
  actions:
    scan:
      command: "test-tool scan"
      input_schema: '{"type": "object"}'
      output_schema: '{"type": "object"}'
`

func lintInstrument(t *testing.T, yaml, ruleID string) []Finding {
	t.Helper()
	ctx, err := NewGenericLintContext([]byte(yaml), "instrument.yaml")
	if err != nil {
		t.Fatalf("NewGenericLintContext: %v", err)
	}
	for _, rule := range instrumentRules() {
		if rule.ID() == ruleID {
			return rule.Check(ctx)
		}
	}
	t.Fatalf("rule %q not found", ruleID)
	return nil
}

func TestInstrumentLint_ValidManifest_NoFindings(t *testing.T) {
	ctx, err := NewGenericLintContext([]byte(validInstrumentYAML), "instrument.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, rule := range instrumentRules() {
		findings := rule.Check(ctx)
		if len(findings) > 0 {
			t.Errorf("rule %s fired on valid manifest: %v", rule.ID(), findings)
		}
	}
}

func TestInstrumentLint_NonInstrument_Skipped(t *testing.T) {
	circuitYAML := `kind: Circuit
circuit: test
nodes:
  - name: a
    instrument: transformer
edges:
  - id: a-done
    from: a
    to: _done
start: a
done: _done
`
	ctx, err := NewGenericLintContext([]byte(circuitYAML), "circuit.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, rule := range instrumentRules() {
		findings := rule.Check(ctx)
		if len(findings) > 0 {
			t.Errorf("rule %s should skip non-instrument YAML, got: %v", rule.ID(), findings)
		}
	}
}

func TestInstrumentLint_I1_MissingTune(t *testing.T) {
	yaml := `apiVersion: tako/v1
kind: Instrument
metadata:
  name: bad-tool
  namespace: testkit
spec:
  dispatch: cli
  actions:
    run:
      command: "run"
      input_schema: '{}'
      output_schema: '{}'
`
	findings := lintInstrument(t, yaml, "I1/instrument-missing-tune")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityError {
		t.Errorf("severity = %v, want error", findings[0].Severity)
	}
}

func TestInstrumentLint_I2_MissingDispatch(t *testing.T) {
	yaml := `apiVersion: tako/v1
kind: Instrument
metadata:
  name: bad-tool
  namespace: testkit
spec:
  tune: "true"
  actions:
    run:
      command: "run"
`
	findings := lintInstrument(t, yaml, "I2/instrument-invalid-dispatch")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestInstrumentLint_I2_InvalidDispatch(t *testing.T) {
	yaml := `apiVersion: tako/v1
kind: Instrument
metadata:
  name: bad-tool
  namespace: testkit
spec:
  dispatch: websocket
  tune: "true"
  actions:
    run:
      command: "run"
`
	findings := lintInstrument(t, yaml, "I2/instrument-invalid-dispatch")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestInstrumentLint_I3_MissingNamespace(t *testing.T) {
	yaml := `apiVersion: tako/v1
kind: Instrument
metadata:
  name: bad-tool
spec:
  dispatch: cli
  tune: "true"
  actions:
    run:
      command: "run"
`
	findings := lintInstrument(t, yaml, "I3/instrument-missing-namespace")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestInstrumentLint_I4_MalformedInputSchema(t *testing.T) {
	yaml := `apiVersion: tako/v1
kind: Instrument
metadata:
  name: bad-tool
  namespace: testkit
spec:
  dispatch: cli
  tune: "true"
  actions:
    run:
      command: "run"
      input_schema: "not valid json {"
`
	findings := lintInstrument(t, yaml, "I4/instrument-malformed-schema")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestInstrumentLint_I4_MalformedOutputSchema(t *testing.T) {
	yaml := `apiVersion: tako/v1
kind: Instrument
metadata:
  name: bad-tool
  namespace: testkit
spec:
  dispatch: cli
  tune: "true"
  actions:
    run:
      command: "run"
      input_schema: '{}'
      output_schema: "[invalid"
`
	findings := lintInstrument(t, yaml, "I4/instrument-malformed-schema")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestInstrumentLint_I4_ValidSchema_NoFinding(t *testing.T) {
	findings := lintInstrument(t, validInstrumentYAML, "I4/instrument-malformed-schema")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestInstrumentLint_I5_MissingSchema(t *testing.T) {
	yaml := `apiVersion: tako/v1
kind: Instrument
metadata:
  name: bare-tool
  namespace: testkit
spec:
  dispatch: cli
  tune: "true"
  actions:
    run:
      command: "run"
`
	findings := lintInstrument(t, yaml, "I5/instrument-missing-schema")
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (input + output), got %d: %v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Severity != SeverityWarning {
			t.Errorf("severity = %v, want warning", f.Severity)
		}
	}
}
