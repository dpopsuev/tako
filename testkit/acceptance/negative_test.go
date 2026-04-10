package acceptance

import (
	"os"
	"path/filepath"
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
nodes:
  - name: a
    instrument: transformer
    action: echo
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

// ── Component manifest gate (kind: + typed sections) ──

func writeTestComponent(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "component.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNegative_ComponentMissingKind_ReturnsError(t *testing.T) {
	path := writeTestComponent(t, `component: test
namespace: test
version: "1.0"
`)
	_, err := circuit.LoadComponentManifest(path)
	if err == nil {
		t.Fatal("expected error for component.yaml without kind:, got nil")
	}
}

func TestNegative_ComponentWrongKind_ReturnsError(t *testing.T) {
	path := writeTestComponent(t, `kind: Schematic
component: test
namespace: test
version: "1.0"
`)
	_, err := circuit.LoadComponentManifest(path)
	if err == nil {
		t.Fatal("expected error for kind: Schematic in component.yaml, got nil")
	}
}

func TestNegative_TransportInSourcesSection_ReturnsError(t *testing.T) {
	path := writeTestComponent(t, `kind: Component
component: test
namespace: test
version: "1.0"
needs:
  sources:
    - name: mcp
      type: Transport
`)
	_, err := circuit.LoadComponentManifest(path)
	if err == nil {
		t.Fatal("expected error for Transport in sources: section, got nil")
	}
}

func TestNegative_SourceReaderInTransportsSection_ReturnsError(t *testing.T) {
	path := writeTestComponent(t, `kind: Component
component: test
namespace: test
version: "1.0"
needs:
  transports:
    - name: data
      type: SourceReader
`)
	_, err := circuit.LoadComponentManifest(path)
	if err == nil {
		t.Fatal("expected error for SourceReader in transports: section, got nil")
	}
}
