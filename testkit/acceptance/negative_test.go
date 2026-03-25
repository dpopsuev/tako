package acceptance

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// Negative tests: PASS when the system correctly REJECTS malformed input.
// Defense-in-depth gate 1: structural validation.

func TestNegative_MalformedYAML_ReturnsError(t *testing.T) {
	_, err := circuit.LoadCircuit([]byte("not: valid: yaml: ["))
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestNegative_EmptyInput_ReturnsError(t *testing.T) {
	def, err := circuit.LoadCircuit([]byte(""))
	if err != nil {
		return // rejected at parse time — good
	}
	// LoadCircuit accepts empty input today (gap — should reject).
	// Verify Build gate catches it instead.
	_, buildErr := engine.BuildGraph(def, standardRegistries())
	if buildErr == nil {
		t.Fatal("expected error for empty circuit at parse or build time, got nil at both gates")
	}
}

func TestNegative_NoNodes_ReturnsError(t *testing.T) {
	yaml := `circuit: broken
start: a
done: done
handler_type: transformer
`
	def, err := circuit.LoadCircuit([]byte(yaml))
	if err != nil {
		return // rejected at parse time — good
	}
	_, buildErr := engine.BuildGraph(def, standardRegistries())
	if buildErr == nil {
		t.Fatal("expected error for circuit with no nodes, got nil")
	}
}

func TestNegative_EdgeReferencesNonexistentNode_ReturnsError(t *testing.T) {
	yaml := `circuit: broken
start: a
done: done
handler_type: transformer
nodes:
  - name: a
    handler: echo
edges:
  - from: a
    to: nonexistent
`
	def, err := circuit.LoadCircuit([]byte(yaml))
	if err != nil {
		return // rejected at parse time — good
	}
	_, buildErr := engine.BuildGraph(def, standardRegistries())
	if buildErr == nil {
		t.Fatal("expected error for edge referencing nonexistent node, got nil")
	}
}
