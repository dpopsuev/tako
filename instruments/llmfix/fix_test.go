package llmfix

import (
	"context"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/simulate/sdlc"
)

// Contract: extractFindings handles typed ScanResult.
func TestExtractFindings_TypedInput(t *testing.T) {
	tc := &engine.TransformerContext{
		Input: &sdlc.ScanResult{
			Findings: []sdlc.Finding{
				{File: "foo.go", Rule: "unused-import", Severity: "error"},
			},
		},
	}
	findings := extractFindings(tc)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Rule != "unused-import" {
		t.Errorf("rule = %q, want unused-import", findings[0].Rule)
	}
}

// Contract: extractFindings handles nil input.
func TestExtractFindings_NilInput(t *testing.T) {
	tc := &engine.TransformerContext{}
	findings := extractFindings(tc)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for nil input, got %d", len(findings))
	}
}

// Contract: pickHighestSeverity prefers errors.
func TestPickHighestSeverity(t *testing.T) {
	findings := []sdlc.Finding{
		{Rule: "warning-rule", Severity: "warning"},
		{Rule: "error-rule", Severity: "error"},
		{Rule: "info-rule", Severity: "info"},
	}
	picked := pickHighestSeverity(findings)
	if picked.Rule != "error-rule" {
		t.Errorf("picked = %q, want error-rule", picked.Rule)
	}
}

// Contract: buildFixPrompt produces non-empty prompt.
func TestBuildFixPrompt(t *testing.T) {
	finding := sdlc.Finding{
		File:     "engine/graph.go",
		Line:     42,
		Rule:     "unused-import",
		Message:  "fmt is imported but not used",
		Severity: "error",
	}
	prompt := buildFixPrompt(finding, t.TempDir())
	if prompt == "" {
		t.Fatal("empty prompt")
	}
	if !contains(prompt, "unused-import") {
		t.Error("prompt missing rule name")
	}
	if !contains(prompt, "engine/graph.go") {
		t.Error("prompt missing file path")
	}
}

// Contract: parseChanges extracts JSON from LLM response.
func TestParseChanges_MarkdownFenced(t *testing.T) {
	response := "Here's the fix:\n\n```json\n" +
		`[{"file": "foo.go", "content": "package main\n"}]` +
		"\n```\n\nDone."
	changes := parseChanges(response)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].File != "foo.go" {
		t.Errorf("file = %q, want foo.go", changes[0].File)
	}
}

// Contract: parseChanges handles bare JSON.
func TestParseChanges_BareJSON(t *testing.T) {
	response := `[{"file": "bar.go", "content": "package bar\n"}]`
	changes := parseChanges(response)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
}

// Contract: parseChanges returns nil for garbage.
func TestParseChanges_Garbage(t *testing.T) {
	changes := parseChanges("I don't know how to fix this")
	if changes != nil {
		t.Errorf("expected nil for garbage, got %v", changes)
	}
}

// Contract: FixTransformer with no findings returns clean FixResult.
func TestFixTransformer_NoFindings(t *testing.T) {
	// Stub provider — won't be called since there are no findings.
	tx := NewFixTransformer(nil, "test", t.TempDir(), WithDryRun())
	result, err := tx.Transform(context.Background(), &engine.TransformerContext{
		Input: &sdlc.ScanResult{Clean: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	fr, ok := result.(*sdlc.FixResult)
	if !ok {
		t.Fatalf("expected *sdlc.FixResult, got %T", result)
	}
	if fr.Applied == "" {
		t.Error("Applied should describe what happened")
	}
}

// Contract: same return type as stub — Liskov.
func TestFixContract_MatchesStub(t *testing.T) {
	stubTx := sdlc.StubTransformers(true)["fix"]
	result, err := stubTx.Transform(context.Background(), &engine.TransformerContext{})
	if err != nil {
		t.Fatal(err)
	}
	_, ok := result.(*sdlc.FixResult)
	if !ok {
		t.Fatalf("stub fix returned %T, want *sdlc.FixResult", result)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
