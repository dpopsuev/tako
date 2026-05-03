package cerebrum

import (
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestParseResponse_MarkdownFences_JSON(t *testing.T) {
	raw := "```json\n{\"atoms\": [{\"type\": \"execution\", \"taxonomy\": \"execution.action\", \"content\": \"do it\"}], \"instrument_call\": {\"name\": \"look_fridge\", \"input\": {}}}\n```"

	atoms, instrumentCall, err := ParseResponse(raw, reactivity.ExecutionAtom, 0)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}

	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	if atoms[0].Type != reactivity.ExecutionAtom {
		t.Errorf("expected ExecutionAtom, got %v", atoms[0].Type)
	}
	if instrumentCall == nil {
		t.Fatal("expected instrument_call, got nil")
	}
	if instrumentCall.Name != "look_fridge" {
		t.Errorf("expected instrument name 'look_fridge', got %q", instrumentCall.Name)
	}
}

func TestParseResponse_MarkdownFences_NoLang(t *testing.T) {
	raw := "```\n{\"atoms\": [{\"type\": \"intent\", \"content\": \"hello\"}]}\n```"

	atoms, _, err := ParseResponse(raw, reactivity.IntentAtom, 0)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	if atoms[0].Type != reactivity.IntentAtom {
		t.Errorf("expected IntentAtom, got %v", atoms[0].Type)
	}
	if string(atoms[0].Content) != "hello" {
		t.Errorf("expected content 'hello', got %q (fallback parse produced raw text instead of parsed JSON)", atoms[0].Content)
	}
}

func TestParseResponse_MarkdownFences_WithSurroundingText(t *testing.T) {
	raw := "Here is my response:\n```json\n{\"atoms\": [{\"type\": \"assessment\", \"content\": \"analyzed\"}]}\n```\nDone."

	atoms, _, err := ParseResponse(raw, reactivity.AssessmentAtom, 0)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	if atoms[0].Type != reactivity.AssessmentAtom {
		t.Errorf("expected AssessmentAtom, got %v", atoms[0].Type)
	}
	if string(atoms[0].Content) != "analyzed" {
		t.Errorf("expected content 'analyzed', got %q (fallback parse produced raw text instead of parsed JSON)", atoms[0].Content)
	}
}

func TestParseResponse_InstrumentCallOnly(t *testing.T) {
	raw := `{"atoms": [], "instrument_call": {"name": "turn_on_stove", "input": ""}}`

	atoms, instrumentCall, _ := ParseResponse(raw, reactivity.ExecutionAtom, 0)
	if instrumentCall == nil {
		t.Fatal("expected instrument_call, got nil")
	}
	if instrumentCall.Name != "turn_on_stove" {
		t.Errorf("expected 'turn_on_stove', got %q", instrumentCall.Name)
	}
	_ = atoms
}

func TestParseResponse_PlainJSON_NoFences(t *testing.T) {
	raw := `{"atoms": [{"type": "knowledge", "taxonomy": "knowledge.fact", "content": "2+2=4"}]}`

	atoms, _, err := ParseResponse(raw, reactivity.KnowledgeAtom, 0)
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
