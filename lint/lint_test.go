package lint

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

type testTransformer struct {
	name string
	det  bool
}

func (t *testTransformer) Name() string        { return t.name }
func (t *testTransformer) Deterministic() bool { return t.det }
func (t *testTransformer) Transform(_ context.Context, _ *engine.TransformerContext) (any, error) {
	return nil, nil
}

func minimalYAML() []byte {
	return []byte(`
kind: circuit
circuit: test
description: a test circuit
handler_type: transformer
nodes:
  - name: recall
    approach: rapid
    handler: core.jq
    meta:
      expr: "input"
  - name: triage
    approach: methodical
    handler: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: recall to triage
    from: recall
    to: triage
  - id: e2
    name: triage to done
    from: triage
    to: _done
start: recall
done: _done
`)
}

func TestRun_CleanCircuit_ZeroFindings(t *testing.T) {
	findings, err := Run(minimalYAML(), "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 0 {
		for _, f := range findings {
			t.Logf("  %s", f)
		}
		t.Fatalf("expected 0 findings on clean circuit, got %d", len(findings))
	}
}

func TestRun_InvalidApproach(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: node
nodes:
  - name: recall
    handler: recall
    approach: rapd
edges:
  - id: e1
    name: e1
    from: recall
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S2/invalid-approach" {
			found = true
			if !strings.Contains(f.Message, "rapd") {
				t.Errorf("expected message to contain 'rapd', got %q", f.Message)
			}
			if f.Suggestion == "" {
				t.Error("expected a suggestion for 'rapd'")
			}
		}
	}
	if !found {
		t.Error("expected S2/invalid-approach finding")
	}
}

func TestRun_InvalidMergeStrategy(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: a
    approach: rapid
  - name: b
    approach: methodical
edges:
  - id: e1
    name: e1
    from: a
    to: b
    merge: squash
  - id: e2
    name: e2
    from: b
    to: _done
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S3/invalid-merge-strategy" {
			found = true
			if !strings.Contains(f.Message, "squash") {
				t.Errorf("expected message to contain 'squash', got %q", f.Message)
			}
		}
	}
	if !found {
		t.Error("expected S3/invalid-merge-strategy finding")
	}
}

func TestRun_MissingEdgeName(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: a
    approach: rapid
edges:
  - id: e1
    from: a
    to: _done
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S4/missing-edge-name" {
			found = true
		}
	}
	if !found {
		t.Error("expected S4/missing-edge-name finding at strict profile")
	}
}

func TestRun_InvalidCacheTTL(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: a
    approach: rapid
    cache:
      ttl: "not-a-duration"
edges:
  - id: e1
    name: e1
    from: a
    to: _done
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S7/invalid-cache-ttl" {
			found = true
		}
	}
	if !found {
		t.Error("expected S7/invalid-cache-ttl finding")
	}
}

func TestRun_MissingDescription(t *testing.T) {
	yml := []byte(`
circuit: test
nodes:
  - name: a
    approach: rapid
edges:
  - id: e1
    name: e1
    from: a
    to: _done
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S8/missing-circuit-description" {
			found = true
		}
	}
	if !found {
		t.Error("expected S8/missing-circuit-description finding")
	}
}

func TestRun_OrphanNode(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: start_node
    approach: rapid
  - name: orphan
    approach: analytical
edges:
  - id: e1
    name: e1
    from: start_node
    to: _done
start: start_node
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "G1/orphan-node" {
			found = true
			if !strings.Contains(f.Message, "orphan") {
				t.Errorf("expected message to mention 'orphan', got %q", f.Message)
			}
		}
	}
	if !found {
		t.Error("expected G1/orphan-node finding")
	}
}

func TestRun_UnreachableDone(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: a
    approach: rapid
  - name: b
    approach: methodical
edges:
  - id: e1
    name: e1
    from: a
    to: b
  - id: e2
    name: e2
    from: b
    to: a
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "G2/unreachable-done" {
			found = true
		}
	}
	if !found {
		t.Error("expected G2/unreachable-done finding")
	}
}

func TestRun_PreferWhenOverCondition(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: a
    approach: rapid
edges:
  - id: e1
    name: e1
    from: a
    to: _done
    condition: "output.confidence >= 0.9 && state.ready"
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "B1/prefer-when-over-condition" {
			found = true
		}
	}
	if !found {
		t.Error("expected B1/prefer-when-over-condition finding")
	}
}

func TestRun_ProfileMin_OnlyErrors(t *testing.T) {
	yml := []byte(`
circuit: test
nodes:
  - name: a
    approach: fyre
edges:
  - id: e1
    from: a
    to: _done
    condition: "always"
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml", WithProfile(ProfileMin))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.Severity != SeverityError {
			t.Errorf("profile=min should only return errors, got %s: %s", f.Severity, f.RuleID)
		}
	}
	if !HasErrors(findings) {
		t.Error("expected at least one error finding (invalid approach)")
	}
}

func TestRun_InvalidWalkerPersona(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: a
    approach: rapid
edges:
  - id: e1
    name: e1
    from: a
    to: _done
walkers:
  - name: agent1
    approach: rapid
    persona: "NonExistentPersona"
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S11/invalid-walker-persona" {
			found = true
		}
	}
	if !found {
		t.Error("expected S11/invalid-walker-persona finding")
	}
}

func TestRun_FanInWithoutMerge(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: a
    approach: rapid
  - name: b
    approach: methodical
  - name: c
    approach: analytical
edges:
  - id: e1
    name: e1
    from: a
    to: c
  - id: e2
    name: e2
    from: b
    to: c
  - id: e3
    name: e3
    from: a
    to: b
  - id: e4
    name: e4
    from: c
    to: _done
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "G7/fan-in-without-merge" {
			found = true
		}
	}
	if !found {
		t.Error("expected G7/fan-in-without-merge finding")
	}
}

func TestLintContext_LineNumbers(t *testing.T) {
	ctx, err := NewLintContext(minimalYAML(), "test.yaml")
	if err != nil {
		t.Fatalf("NewLintContext: %v", err)
	}
	if line := ctx.NodeLine("recall"); line == 0 {
		t.Error("expected non-zero line for node 'recall'")
	}
	if line := ctx.EdgeLine("e1"); line == 0 {
		t.Error("expected non-zero line for edge 'e1'")
	}
	if line := ctx.TopLevelLine("circuit"); line == 0 {
		t.Error("expected non-zero line for top-level 'circuit'")
	}
}

func TestNewLintContextFromDef(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Nodes:   []circuit.NodeDef{{Name: "a", Approach: "rapid"}},
		Edges:   []circuit.EdgeDef{{ID: "e1", Name: "e1", From: "a", To: "_done"}},
		Start:   "a",
		Done:    "_done",
	}
	ctx := NewLintContextFromDef(def, "inline")
	runner := DefaultRunner()
	findings := runner.Run(ctx, WithProfile(ProfileStrict))
	// Should not crash; line numbers will be 0
	for _, f := range findings {
		if f.RuleID == "S8/missing-circuit-description" && f.Line != 0 {
			t.Error("expected line=0 for def-only context")
		}
	}
}

func TestFinding_String(t *testing.T) {
	f := Finding{
		RuleID:   "S2/invalid-approach",
		Severity: SeverityError,
		Message:  `unknown approach "fyre"`,
		File:     "circuit.yaml",
		Line:     12,
	}
	s := f.String()
	if !strings.Contains(s, "circuit.yaml:12") {
		t.Errorf("expected file:line, got %q", s)
	}
	if !strings.Contains(s, "error") {
		t.Errorf("expected severity, got %q", s)
	}
}

func TestAllRules_Count(t *testing.T) {
	rules := AllRules()
	// 24 structural + 9 semantic + 12 best-practice + 1 prompt + 1 crossref + 5 scenario = 51
	if len(rules) != 52 {
		t.Errorf("expected 52 rules, got %d", len(rules))
	}

	ids := make(map[string]bool)
	for _, r := range rules {
		if ids[r.ID()] {
			t.Errorf("duplicate rule ID: %s", r.ID())
		}
		ids[r.ID()] = true
	}
}

func TestHasErrors(t *testing.T) {
	if HasErrors(nil) {
		t.Error("nil should not have errors")
	}
	if HasErrors([]Finding{{Severity: SeverityWarning}}) {
		t.Error("warnings should not count as errors")
	}
	if !HasErrors([]Finding{{Severity: SeverityError}}) {
		t.Error("errors should be detected")
	}
}

func TestApplyFixes_InvalidApproach(t *testing.T) {
	yml := []byte(`circuit: test
description: test
nodes:
  - name: a
    approach: rapd
edges:
  - id: e1
    name: e1
    from: a
    to: _done
start: a
done: _done
`)
	fixed, fixes, err := ApplyFixes(yml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("ApplyFixes: %v", err)
	}
	if len(fixes) == 0 {
		t.Fatal("expected at least one fix")
	}
	if !strings.Contains(string(fixed), "approach: rapid") {
		t.Errorf("expected fix to replace 'rapd' with 'rapid', got:\n%s", string(fixed))
	}
}

func TestApplyFixes_ConditionToWhen(t *testing.T) {
	yml := []byte(`circuit: test
description: test
nodes:
  - name: a
    approach: rapid
edges:
  - id: e1
    name: e1
    from: a
    to: _done
    condition: "output.confidence >= 0.9 && state.ready"
start: a
done: _done
`)
	fixed, fixes, err := ApplyFixes(yml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("ApplyFixes: %v", err)
	}
	if len(fixes) == 0 {
		t.Fatal("expected at least one fix")
	}
	if !strings.Contains(string(fixed), "when:") {
		t.Errorf("expected 'condition:' to be renamed to 'when:', got:\n%s", string(fixed))
	}
	if strings.Contains(string(fixed), "condition:") {
		t.Errorf("expected 'condition:' to be removed, got:\n%s", string(fixed))
	}
}

func TestApplyFixes_NoFixNeeded(t *testing.T) {
	fixed, fixes, err := ApplyFixes(minimalYAML(), "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("ApplyFixes: %v", err)
	}
	if len(fixes) != 0 {
		for _, f := range fixes {
			t.Logf("  fix: %s at line %d: %s", f.Finding.RuleID, f.StartLine, f.Finding.Message)
		}
		t.Errorf("expected 0 fixes on clean YAML, got %d", len(fixes))
	}
	if fixed != nil {
		t.Error("expected nil bytes when no fixes applied")
	}
}

func TestRun_StochasticTransformer_Fallback(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: recall
    approach: rapid
    handler: core.llm
    prompt: "recall items"
  - name: triage
    approach: methodical
    handler: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: e1
    from: recall
    to: triage
  - id: e2
    name: e2
    from: triage
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "B7/stochastic-transformer" {
			found = true
			if !strings.Contains(f.Message, "recall") {
				t.Errorf("expected message to mention 'recall', got %q", f.Message)
			}
			if !strings.Contains(f.Message, "core.llm") {
				t.Errorf("expected message to mention 'core.llm', got %q", f.Message)
			}
		}
	}
	if !found {
		t.Error("expected B7/stochastic-transformer finding for core.llm node")
	}
}

func TestRun_StochasticTransformer_WithRegistry(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: a
    approach: rapid
    handler: custom.stochastic
  - name: b
    approach: methodical
    handler: custom.deterministic
edges:
  - id: e1
    name: e1
    from: a
    to: b
  - id: e2
    name: e2
    from: b
    to: _done
start: a
done: _done
`)
	stoch := &testTransformer{name: "custom.stochastic", det: false}
	deter := &testTransformer{name: "custom.deterministic", det: true}
	reg := &engine.GraphRegistries{
		Transformers: engine.TransformerRegistry{
			"custom.stochastic":    stoch,
			"custom.deterministic": deter,
		},
	}
	findings, err := Run(yml, "test.yaml", WithProfile(ProfileStrict), WithRegistries(reg))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	stochCount := 0
	for _, f := range findings {
		if f.RuleID == "B7/stochastic-transformer" {
			stochCount++
			if !strings.Contains(f.Message, "custom.stochastic") {
				t.Errorf("expected stochastic finding for custom.stochastic, got %q", f.Message)
			}
		}
	}
	if stochCount != 1 {
		t.Errorf("expected exactly 1 B7 finding, got %d", stochCount)
	}
}

func TestRun_StochasticTransformer_AllDeterministic(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: a
    approach: rapid
    handler: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: e1
    from: a
    to: _done
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "B7/stochastic-transformer" {
			t.Errorf("unexpected B7 finding for deterministic circuit: %s", f.Message)
		}
	}
}

func TestRun_StochasticSummary(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: recall
    approach: rapid
    handler: core.llm
    prompt: "recall items"
  - name: triage
    approach: methodical
    handler: core.jq
    meta:
      expr: "input"
  - name: assess
    approach: analytical
    handler: llm
    prompt: "assess"
edges:
  - id: e1
    name: e1
    from: recall
    to: triage
  - id: e2
    name: e2
    from: triage
    to: assess
  - id: e3
    name: e3
    from: assess
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "B7s/stochastic-summary" {
			found = true
			if !strings.Contains(f.Message, "2 stochastic") {
				t.Errorf("expected summary to mention '2 stochastic', got %q", f.Message)
			}
			if !strings.Contains(f.Message, "out of 3") {
				t.Errorf("expected summary to mention 'out of 3', got %q", f.Message)
			}
			if !strings.Contains(f.Message, "recall") || !strings.Contains(f.Message, "assess") {
				t.Errorf("expected summary to list node names, got %q", f.Message)
			}
		}
	}
	if !found {
		t.Error("expected B7s/stochastic-summary finding")
	}
}

func TestRun_StochasticSummary_NoneWhenAllDeterministic(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: a
    approach: rapid
    handler: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: e1
    from: a
    to: _done
start: a
done: _done
`)
	findings, err := Run(yml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "B7s/stochastic-summary" {
			t.Errorf("unexpected B7s summary for deterministic circuit: %s", f.Message)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"fire", "fire", 0},
		{"fyre", "fire", 1},
		{"", "abc", 3},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
	}
	for _, tt := range tests {
		if got := levenshtein(tt.a, tt.b); got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestRun_TemplateParamValidity(t *testing.T) {
	validTemplate := `# Case {{.CaseID}}
Step: {{.StepName}}`
	invalidTemplate := `# Case {{.CaseID}}
Error: {{.Failure.ErrorMesage}}`
	referenceDoc := `# Gap Analysis
No template directives here.`

	promptFS := fstest.MapFS{
		"prompts/recall/judge-similarity.md":  &fstest.MapFile{Data: []byte(validTemplate)},
		"prompts/triage/classify-symptoms.md": &fstest.MapFile{Data: []byte(invalidTemplate)},
		"prompts/review/gap-analysis.md":      &fstest.MapFile{Data: []byte(referenceDoc)},
	}

	validator := PromptValidator(func(content string) []PromptFieldError {
		if strings.Contains(content, "ErrorMesage") {
			return []PromptFieldError{{Field: "Failure.ErrorMesage", Message: `type FailureParams has no field "ErrorMesage"`}}
		}
		return nil
	})

	circuitYAML := []byte(`
circuit: test
description: test circuit
nodes:
  - name: init
    approach: analytical
  - name: done
    approach: analytical
edges:
  - id: e1
    name: start
    from: init
    to: done
    when: "true"
`)

	findings, err := Run(circuitYAML, "test.yaml",
		WithProfile(ProfileStrict),
		WithPromptFS(promptFS),
		WithPromptValidator(validator),
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var p1Findings []Finding
	for _, f := range findings {
		if f.RuleID == "P1/template-param-validity" {
			p1Findings = append(p1Findings, f)
		}
	}

	if len(p1Findings) != 1 {
		t.Fatalf("expected 1 P1 finding, got %d: %+v", len(p1Findings), p1Findings)
	}
	if !strings.Contains(p1Findings[0].Message, "ErrorMesage") {
		t.Errorf("expected finding to mention ErrorMesage, got: %s", p1Findings[0].Message)
	}
}

func TestRun_TemplateParamValidity_NoOpWithoutOptions(t *testing.T) {
	circuitYAML := []byte(`
circuit: test
description: test
nodes:
  - name: init
    approach: analytical
  - name: done
    approach: analytical
edges:
  - id: e1
    name: start
    from: init
    to: done
    when: "true"
`)

	findings, err := Run(circuitYAML, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "P1/template-param-validity" {
			t.Errorf("unexpected P1 finding without options: %+v", f)
		}
	}
}

// --- B9: missing-kind ---

func TestB9_MissingKind_NoKind(t *testing.T) {
	yaml := []byte(`
name: something
description: no kind field
`)
	findings, err := Run(yaml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var b9 []Finding
	for _, f := range findings {
		if f.RuleID == "B9/missing-kind" {
			b9 = append(b9, f)
		}
	}
	if len(b9) != 1 {
		t.Fatalf("expected 1 B9 finding, got %d: %+v", len(b9), b9)
	}
	if b9[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %v", b9[0].Severity)
	}
}

func TestB9_MissingKind_WithKind(t *testing.T) {
	yaml := []byte(`
kind: scenario
version: v1
metadata:
  name: test
  description: test scenario
`)
	findings, err := Run(yaml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "B9/missing-kind" {
			t.Errorf("unexpected B9 finding for file with kind: %+v", f)
		}
	}
}

func TestB9_MissingKind_UnknownKind(t *testing.T) {
	yaml := []byte(`
kind: foobar
version: v1
`)
	findings, err := Run(yaml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var b9 []Finding
	for _, f := range findings {
		if f.RuleID == "B9/missing-kind" {
			b9 = append(b9, f)
		}
	}
	if len(b9) != 1 {
		t.Fatalf("expected 1 B9 info finding for unknown kind, got %d: %+v", len(b9), b9)
	}
	if b9[0].Severity != SeverityInfo {
		t.Errorf("expected info severity for unknown kind, got %v", b9[0].Severity)
	}
}

func TestB9_CircuitWithKind(t *testing.T) {
	yaml := []byte(`
kind: circuit
version: v1
metadata:
  name: test
  description: test circuit
circuit: test
description: test
nodes:
  - name: init
    approach: analytical
  - name: done
    approach: analytical
edges:
  - id: e1
    name: start
    from: init
    to: done
    when: "true"
`)
	findings, err := Run(yaml, "test.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "B9/missing-kind" {
			t.Errorf("unexpected B9 finding for circuit with kind: %+v", f)
		}
	}
}

// --- B10: deprecated-fk-arrow ---

func TestB10_DeprecatedArrow(t *testing.T) {
	yaml := []byte(`
kind: store-schema
version: 1
tables:
  - name: child
    columns:
      - parent_id: integer not_null -> parent
`)
	findings, err := Run(yaml, "schema.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var b10 []Finding
	for _, f := range findings {
		if f.RuleID == "B10/deprecated-fk-arrow" {
			b10 = append(b10, f)
		}
	}
	if len(b10) != 1 {
		t.Fatalf("expected 1 B10 finding, got %d: %+v", len(b10), b10)
	}
}

// --- S21: edge-node-reference ---

func TestS21_EdgeNodeReference_InvalidFrom(t *testing.T) {
	// Invalid edge from-node is now caught at parse time by normalize()
	// graph validation. The circuit is rejected before lint rules can
	// analyze it, falling back to generic context (Def=nil).
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: recall
    approach: rapid
  - name: triage
    approach: methodical
edges:
  - id: e1
    name: e1
    from: recal
    to: triage
  - id: e2
    name: e2
    from: triage
    to: _done
start: recall
done: _done
`)
	// Verify parse-time rejection catches the invalid reference.
	_, err := circuit.LoadCircuit(yml)
	if err == nil {
		t.Fatal("expected parse error for undefined from-node 'recal'")
	}
	if !strings.Contains(err.Error(), "recal") {
		t.Errorf("expected error to mention 'recal', got %q", err.Error())
	}

	// Run still succeeds (falls back to generic context), but no S21 findings.
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "S21/edge-node-reference" {
			t.Log("S21 finding still produced via generic context — acceptable")
		}
	}
}

func TestS21_EdgeNodeReference_InvalidTo(t *testing.T) {
	// Invalid edge to-node is now caught at parse time by normalize()
	// graph validation. The circuit is rejected before lint rules run.
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: recall
    approach: rapid
  - name: triage
    approach: methodical
edges:
  - id: e1
    name: e1
    from: recall
    to: triag
  - id: e2
    name: e2
    from: triage
    to: _done
start: recall
done: _done
`)
	// Verify parse-time rejection catches the invalid reference.
	_, err := circuit.LoadCircuit(yml)
	if err == nil {
		t.Fatal("expected parse error for undefined to-node 'triag'")
	}
	if !strings.Contains(err.Error(), "triag") {
		t.Errorf("expected error to mention 'triag', got %q", err.Error())
	}

	// Run still succeeds (falls back to generic context), but no S21 findings.
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "S21/edge-node-reference" {
			t.Log("S21 finding still produced via generic context — acceptable")
		}
	}
}

func TestS21_EdgeNodeReference_DoneSentinelValid(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
nodes:
  - name: recall
    approach: rapid
edges:
  - id: e1
    name: e1
    from: recall
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "S21/edge-node-reference" {
			t.Errorf("unexpected S21 finding for valid circuit: %s", f.Message)
		}
	}
}

// --- S22: hook-reference ---

func TestS22_HookReference_WhitespaceHook(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: recall
    approach: rapid
    handler: core.jq
    meta:
      expr: "input"
    before: ["inject failure"]
edges:
  - id: e1
    name: e1
    from: recall
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S22/hook-reference" && strings.Contains(f.Message, "whitespace") {
			found = true
		}
	}
	if !found {
		t.Error("expected S22/hook-reference finding for hook with whitespace")
	}
}

func TestS22_HookReference_EmptySegment(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: recall
    approach: rapid
    handler: core.jq
    meta:
      expr: "input"
    before: ["inject."]
edges:
  - id: e1
    name: e1
    from: recall
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S22/hook-reference" && strings.Contains(f.Message, "empty segment") {
			found = true
		}
	}
	if !found {
		t.Error("expected S22/hook-reference finding for hook with empty segment")
	}
}

func TestS22_HookReference_SimilarNames(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: recall
    approach: rapid
    handler: core.jq
    meta:
      expr: "input"
    before: ["inject.failure"]
  - name: triage
    approach: methodical
    handler: core.jq
    meta:
      expr: "input"
    before: ["inject.failurre"]
edges:
  - id: e1
    name: e1
    from: recall
    to: triage
  - id: e2
    name: e2
    from: triage
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S22/hook-reference" && strings.Contains(f.Message, "similar") {
			found = true
		}
	}
	if !found {
		t.Error("expected S22/hook-reference finding for similar hook names")
	}
}

func TestS22_HookReference_ValidHooks(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: recall
    approach: rapid
    handler: core.jq
    meta:
      expr: "input"
    before: ["inject.failure"]
    after: ["extract.result"]
edges:
  - id: e1
    name: e1
    from: recall
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "S22/hook-reference" {
			t.Errorf("unexpected S22 finding for valid hooks: %s", f.Message)
		}
	}
}

// --- S23: invalid-handler-type ---

func TestS23_InvalidHandlerType_CircuitLevel(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformr
nodes:
  - name: recall
    approach: rapid
    handler: core.jq
    meta:
      expr: "input"
edges:
  - id: e1
    name: e1
    from: recall
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S23/invalid-handler-type" {
			found = true
			if !strings.Contains(f.Message, "transformr") {
				t.Errorf("expected message to mention 'transformr', got %q", f.Message)
			}
			if f.Suggestion == "" {
				t.Error("expected suggestion for 'transformr'")
			}
		}
	}
	if !found {
		t.Error("expected S23/invalid-handler-type finding")
	}
}

func TestS23_InvalidHandlerType_NodeLevel(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: recall
    approach: rapid
    handler_type: circit
    handler: sub-circuit
edges:
  - id: e1
    name: e1
    from: recall
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "S23/invalid-handler-type" {
			found = true
			if !strings.Contains(f.Message, "circit") {
				t.Errorf("expected message to mention 'circit', got %q", f.Message)
			}
		}
	}
	if !found {
		t.Error("expected S23/invalid-handler-type finding for node-level handler_type")
	}
}

func TestS23_InvalidHandlerType_ValidTypes(t *testing.T) {
	yml := []byte(`
circuit: test
description: test
handler_type: transformer
nodes:
  - name: recall
    approach: rapid
    handler: core.jq
    meta:
      expr: "input"
  - name: sub
    approach: analytical
    handler_type: circuit
    handler: sub-circuit
edges:
  - id: e1
    name: e1
    from: recall
    to: sub
  - id: e2
    name: e2
    from: sub
    to: _done
start: recall
done: _done
`)
	findings, err := Run(yml, "test.yaml")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "S23/invalid-handler-type" {
			t.Errorf("unexpected S23 finding for valid handler types: %s", f.Message)
		}
	}
}

func TestB10_NoArrowWithReferences(t *testing.T) {
	yaml := []byte(`
kind: store-schema
version: 1
tables:
  - name: child
    columns:
      - parent_id: integer not_null references parent
`)
	findings, err := Run(yaml, "schema.yaml", WithProfile(ProfileStrict))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if f.RuleID == "B10/deprecated-fk-arrow" {
			t.Errorf("unexpected B10 finding for references syntax: %+v", f)
		}
	}
}
