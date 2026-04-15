package core

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

func TestMatchRule_AllOf(t *testing.T) {
	r := MatchRule{AllOf: []string{"foo", "bar"}, Result: "hit"}
	if !r.Matches("foo and bar") {
		t.Error("expected match when both present")
	}
	if r.Matches("only foo here") {
		t.Error("should not match when 'bar' missing")
	}
}

func TestMatchRule_AnyOf(t *testing.T) {
	r := MatchRule{AnyOf: []string{"alpha", "beta"}, Result: "hit"}
	if !r.Matches("has alpha in it") {
		t.Error("expected match for alpha")
	}
	if !r.Matches("has beta in it") {
		t.Error("expected match for beta")
	}
	if r.Matches("has gamma only") {
		t.Error("should not match when none present")
	}
}

func TestMatchRule_NoneOf(t *testing.T) {
	r := MatchRule{AllOf: []string{"good"}, NoneOf: []string{"bad"}, Result: "hit"}
	if !r.Matches("good stuff") {
		t.Error("expected match without excluded term")
	}
	if r.Matches("good and bad together") {
		t.Error("should not match when excluded term present")
	}
}

func TestMatchRule_Regex(t *testing.T) {
	r := MatchRule{Regex: []string{`\d{3}-\d{4}`}, Result: "phone"}
	if !r.Matches("call 555-1234 now") {
		t.Error("expected regex match")
	}
	if r.Matches("no numbers here") {
		t.Error("should not match without pattern")
	}
}

func TestMatchRule_Combined(t *testing.T) {
	r := MatchRule{
		AllOf:  []string{"error"},
		AnyOf:  []string{"timeout", "refused"},
		NoneOf: []string{"debug"},
		Result: "infra",
	}
	if !r.Matches("error: connection refused") {
		t.Error("expected match")
	}
	if r.Matches("error: connection refused [debug]") {
		t.Error("debug should exclude")
	}
	if r.Matches("warning: timeout occurred") {
		t.Error("missing 'error' should exclude")
	}
}

func TestMatchRule_Empty(t *testing.T) {
	r := MatchRule{Result: "always"}
	if !r.Matches("anything") {
		t.Error("empty rule should always match")
	}
}

func TestMatchRuleSet_FirstMatchWins(t *testing.T) {
	rs := MatchRuleSet{
		Rules: []MatchRule{
			{AllOf: []string{"specific", "pattern"}, Result: "first"},
			{AnyOf: []string{"pattern"}, Result: "second"},
		},
		Default: "none",
	}

	result, matched := rs.Evaluate("specific pattern in text")
	if !matched || result != "first" {
		t.Errorf("got %v (matched=%v), want first", result, matched)
	}

	result, matched = rs.Evaluate("just pattern here")
	if !matched || result != "second" {
		t.Errorf("got %v (matched=%v), want second", result, matched)
	}

	result, matched = rs.Evaluate("nothing relevant")
	if matched {
		t.Error("should not match")
	}
	if result != "none" {
		t.Errorf("default = %v, want none", result)
	}
}

func TestMatchRuleSet_EvaluateString(t *testing.T) {
	rs := MatchRuleSet{
		Rules:   []MatchRule{{AllOf: []string{"x"}, Result: "found"}},
		Default: "default",
	}
	if got := rs.EvaluateString("has x"); got != "found" {
		t.Errorf("got %q, want found", got)
	}
	if got := rs.EvaluateString("no match"); got != "default" {
		t.Errorf("got %q, want default", got)
	}
}

func TestMatchRuleSet_StructuredResult(t *testing.T) {
	rs := MatchRuleSet{
		Rules: []MatchRule{
			{
				AllOf: []string{"timeout"},
				Result: map[string]any{
					"category":   "infra",
					"hypothesis": "ti001",
					"skip":       true,
				},
			},
		},
		Default: map[string]any{"category": "unknown"},
	}

	result, matched := rs.Evaluate("timeout on node")
	if !matched {
		t.Fatal("expected match")
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map", result)
	}
	if m["category"] != "infra" {
		t.Errorf("category = %v", m["category"])
	}
}

func TestMatchEvaluator_LoadAndEvaluate(t *testing.T) {
	yamlData := []byte(`
classification:
  rules:
    - all_of: ["timeout"]
      result: infra
    - any_of: ["ptp", "clock"]
      result: product
  default: unknown

component:
  rules:
    - all_of: ["phc2sys"]
      result: linuxptp-daemon
    - any_of: ["cloud event", "sidecar"]
      result: cloud-event-proxy
  default: unknown
`)

	eval, err := NewMatchEvaluator(yamlData)
	if err != nil {
		t.Fatal(err)
	}

	names := eval.Names()
	if len(names) != 2 {
		t.Errorf("expected 2 rule sets, got %d", len(names))
	}

	result, matched, err := eval.Evaluate("classification", "timeout on node")
	if err != nil {
		t.Fatal(err)
	}
	if !matched || result != "infra" {
		t.Errorf("classification = %v (matched=%v), want infra", result, matched)
	}

	result, matched, err = eval.Evaluate("component", "phc2sys offset issue")
	if err != nil {
		t.Fatal(err)
	}
	if !matched || result != "linuxptp-daemon" {
		t.Errorf("component = %v (matched=%v), want linuxptp-daemon", result, matched)
	}

	_, _, err = eval.Evaluate("nonexistent", "text")
	if err == nil {
		t.Error("expected error for unknown rule set")
	}
}

func TestMatchEvaluator_Get(t *testing.T) {
	eval, err := NewMatchEvaluator([]byte(`
test:
  rules:
    - all_of: ["x"]
      result: found
  default: fallback
`))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := eval.Get("test")
	if err != nil {
		t.Fatal(err)
	}
	if rs == nil {
		t.Error("expected non-nil rule set")
	}

	_, err = eval.Get("missing")
	if err == nil {
		t.Error("expected error for missing rule set")
	}
}

func TestMatchEvaluator_InvalidYAML(t *testing.T) {
	_, err := NewMatchEvaluator([]byte(`{invalid yaml`))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestMatchTransformer_Name(t *testing.T) {
	m := NewMatch()
	if m.Name() != "match" {
		t.Errorf("Name() = %q, want match", m.Name())
	}
}

func TestMatchTransformer_Transform(t *testing.T) {
	yamlData := []byte(`
colors:
  rules:
    - any_of: ["red", "crimson"]
      result: warm
    - any_of: ["blue", "cyan"]
      result: cool
  default: neutral
`)
	eval, err := NewMatchEvaluator(yamlData)
	if err != nil {
		t.Fatal(err)
	}

	m := NewMatch()
	tc := &engine.TransformerContext{
		Input: "the sky is blue",
		NodeConfig: &circuit.NodeConfig{
			Evaluator: eval,
			RuleSet:   "colors",
		},
	}

	result, err := m.Transform(context.Background(), tc)
	if err != nil {
		t.Fatal(err)
	}

	rm, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T", result)
	}
	if rm["result"] != "cool" {
		t.Errorf("result = %v, want cool", rm["result"])
	}
	if rm["matched"] != true {
		t.Error("expected matched=true")
	}
}

func TestMatchTransformer_MapInput(t *testing.T) {
	eval, _ := NewMatchEvaluator([]byte(`
test:
  rules:
    - all_of: ["error"]
      result: found
  default: missing
`))

	m := NewMatch()
	tc := &engine.TransformerContext{
		Input: map[string]any{"text": "error occurred"},
		NodeConfig: &circuit.NodeConfig{
			Evaluator: eval,
			RuleSet:   "test",
		},
	}

	result, err := m.Transform(context.Background(), tc)
	if err != nil {
		t.Fatal(err)
	}
	rm := result.(map[string]any)
	if rm["result"] != "found" {
		t.Errorf("result = %v, want found", rm["result"])
	}
}

func TestMatchTransformer_MissingEvaluator(t *testing.T) {
	m := NewMatch()
	tc := &engine.TransformerContext{NodeConfig: &circuit.NodeConfig{}}
	_, err := m.Transform(context.Background(), tc)
	if err == nil {
		t.Error("expected error for missing evaluator")
	}
}

func TestMatchTransformer_MissingRuleSet(t *testing.T) {
	eval, _ := NewMatchEvaluator([]byte(`
test:
  rules:
    - all_of: ["x"]
      result: y
`))
	m := NewMatch()
	tc := &engine.TransformerContext{
		NodeConfig: &circuit.NodeConfig{Evaluator: eval},
	}
	_, err := m.Transform(context.Background(), tc)
	if err == nil {
		t.Error("expected error for missing rule_set")
	}
}

func TestMatchEvaluator_SkipsNonRuleSetKeys(t *testing.T) {
	eval, err := NewMatchEvaluator([]byte(`
plain_list:
  - "a"
  - "b"
rules_section:
  rules:
    - all_of: ["match"]
      result: ok
  default: miss
convergence:
  jira_keywords: ["x"]
`))
	if err != nil {
		t.Fatal(err)
	}
	names := eval.Names()
	if len(names) != 1 || names[0] != "rules_section" {
		t.Errorf("expected only rules_section, got %v", names)
	}
}

func TestMatchRule_InvalidRegex(t *testing.T) {
	r := MatchRule{Regex: []string{"[invalid"}, Result: "x"}
	if r.Matches("anything") {
		t.Error("invalid regex should not match")
	}
}
