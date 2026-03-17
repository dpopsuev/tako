package calibrate

import (
	"context"
	"testing"
)

func TestLoadGenericScenario(t *testing.T) {
	data := []byte(`
name: test-scenario
cases:
  - id: C1
    input:
      question: "What is 2+2?"
    expected:
      answer: 4
  - id: C2
    input:
      question: "What is 3+3?"
    expected:
      answer: 6
`)
	s, err := LoadGenericScenario(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "test-scenario" {
		t.Errorf("name = %q, want test-scenario", s.Name)
	}
	if len(s.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(s.Cases))
	}
	if s.Cases[0].ID != "C1" {
		t.Errorf("cases[0].id = %q, want C1", s.Cases[0].ID)
	}
	if s.Cases[0].Expected["answer"] != 4 {
		t.Errorf("cases[0].expected.answer = %v, want 4", s.Cases[0].Expected["answer"])
	}
}

func TestLoadGenericScenario_Errors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty name", []byte("cases: [{id: C1}]")},
		{"no cases", []byte("name: test")},
		{"invalid yaml", []byte(":::")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadGenericScenario(tt.data)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestGenericScenarioLoader_Load(t *testing.T) {
	s := &GenericScenario{
		Name: "test",
		Cases: []GenericCase{
			{ID: "C1", Input: map[string]any{"q": "hello"}},
			{ID: "C2", Input: map[string]any{"q": "world"}},
		},
	}
	loader := &GenericScenarioLoader{Scenario: s}
	cases, err := loader.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(cases))
	}
	if cases[0].ID != "C1" {
		t.Errorf("cases[0].ID = %q, want C1", cases[0].ID)
	}
	if cases[0].Context["q"] != "hello" {
		t.Errorf("cases[0].Context[q] = %v, want hello", cases[0].Context["q"])
	}
}
