package llmfix

import (
	"context"
	"strings"
	"testing"

	"github.com/dpopsuev/tako/engine"
	"github.com/dpopsuev/tako/simulate/sdlc/sdlctype"
)

// Contract: extractFindings handles typed ScanResult.
func TestExtractFindings_TypedInput(t *testing.T) {
	tc := &engine.InstrumentContext{
		Input: &sdlctype.ScanResult{
			Findings: []sdlctype.Finding{
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
	tc := &engine.InstrumentContext{}
	findings := extractFindings(tc)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for nil input, got %d", len(findings))
	}
}

// Contract: pickFixableFinding skips non-file findings.
func TestPickFixableFinding(t *testing.T) {
	findings := []sdlctype.Finding{
		{File: "engine", Rule: "hot-spot", Severity: "warning"},               // package name, not file
		{File: "dispatch", Rule: "hot-spot", Severity: "warning"},             // package name
		{File: "engine/graph.go", Rule: "layer-violation", Severity: "error"}, // real file
	}
	picked, ok := pickFixableFinding(findings)
	if !ok {
		t.Fatal("expected a fixable finding")
	}
	if picked.File != "engine/graph.go" {
		t.Errorf("picked = %q, want engine/graph.go", picked.File)
	}
}

func TestPickFixableFinding_NoneFixable(t *testing.T) {
	findings := []sdlctype.Finding{
		{File: "engine", Rule: "hot-spot"},
		{File: "circuit", Rule: "hot-spot"},
	}
	_, ok := pickFixableFinding(findings)
	if ok {
		t.Error("expected no fixable finding for package-level observations")
	}
}

// Contract: buildFixPrompt produces non-empty prompt.
func TestBuildFixPrompt(t *testing.T) {
	finding := sdlctype.Finding{
		File:     "engine/graph.go",
		Line:     42,
		Rule:     "unused-import",
		Message:  "fmt is imported but not used",
		Severity: "error",
	}
	f := NewFixTransformer(nil, "test", t.TempDir(), WithDryRun())
	prompt := f.buildFixPrompt(finding)
	if prompt == "" {
		t.Fatal("empty prompt")
	}
	if !contains(prompt, "unused-import") {
		t.Error("prompt missing rule name")
	}
	if !contains(prompt, "engine/graph.go") {
		t.Error("prompt missing file path")
	}
	// Guards must be present to prevent junk file creation.
	for _, guard := range []string{
		"ONLY modify the file",
		"Do NOT create new files",
		"Do NOT add new packages",
		"COMPLETE file content",
	} {
		if !contains(prompt, guard) {
			t.Errorf("prompt missing guard: %q", guard)
		}
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
	result, err := tx.Transform(context.Background(), &engine.InstrumentContext{
		Input: &sdlctype.ScanResult{Clean: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	fr, ok := result.(*sdlctype.FixResult)
	if !ok {
		t.Fatalf("expected *sdlctype.FixResult, got %T", result)
	}
	if fr.Applied == "" {
		t.Error("Applied should describe what happened")
	}
}

// Contract: same return type as stub — Liskov.
func TestFixContract_MatchesStub(t *testing.T) {
	stubTx := engine.InstrumentFunc("fix", func(_ context.Context, _ *engine.InstrumentContext) (any, error) {
		return &sdlctype.FixResult{Applied: "stub"}, nil
	})
	result, err := stubTx.Transform(context.Background(), &engine.InstrumentContext{})
	if err != nil {
		t.Fatal(err)
	}
	_, ok := result.(*sdlctype.FixResult)
	if !ok {
		t.Fatalf("stub fix returned %T, want *sdlctype.FixResult", result)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
