package cerebrum

import (
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestParseResponse_MarkdownFences_JSON(t *testing.T) {
	raw := "```json\n{\"atoms\": [{\"type\": \"execution\", \"taxonomy\": \"execution.action\", \"content\": \"do it\"}]}\n```"

	atoms, err := ParseResponse(raw, reactivity.ExecutionAtom, 0)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}

	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	if atoms[0].Type != reactivity.ExecutionAtom {
		t.Errorf("expected ExecutionAtom, got %v", atoms[0].Type)
	}
}

func TestParseResponse_MarkdownFences_NoLang(t *testing.T) {
	raw := "```\n{\"atoms\": [{\"type\": \"intent\", \"content\": \"hello\"}]}\n```"

	atoms, err := ParseResponse(raw, reactivity.IntentAtom, 0)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	if string(atoms[0].Content) != "hello" {
		t.Errorf("expected content 'hello', got %q", atoms[0].Content)
	}
}

func TestParseResponse_MarkdownFences_WithSurroundingText(t *testing.T) {
	raw := "Here is my response:\n```json\n{\"atoms\": [{\"type\": \"assessment\", \"content\": \"analyzed\"}]}\n```\nDone."

	atoms, err := ParseResponse(raw, reactivity.AssessmentAtom, 0)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	if string(atoms[0].Content) != "analyzed" {
		t.Errorf("expected content 'analyzed', got %q", atoms[0].Content)
	}
}

func TestParseResponse_PlainJSON_NoFences(t *testing.T) {
	raw := `{"atoms": [{"type": "knowledge", "taxonomy": "knowledge.fact", "content": "2+2=4"}]}`

	atoms, err := ParseResponse(raw, reactivity.KnowledgeAtom, 0)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	if string(atoms[0].Content) != "2+2=4" {
		t.Errorf("expected content '2+2=4', got %q", atoms[0].Content)
	}
}

func TestParseResponse_PlainText_Fallback(t *testing.T) {
	raw := "just some text without any JSON"

	atoms, err := ParseResponse(raw, reactivity.IntentAtom, 0)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 fallback atom, got %d", len(atoms))
	}
	if atoms[0].Type != reactivity.IntentAtom {
		t.Errorf("fallback should use current phase type")
	}
}
