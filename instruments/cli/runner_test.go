package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/engine"
)

const testCircuitYAML = `
circuit: cli-test
handler_type: transformer
nodes:
  - name: start
    approach: fire
    handler: echo
  - name: finish
    approach: water
    handler: echo
edges:
  - id: E1
    name: go
    from: start
    to: finish
    when: "true"
  - id: E2
    name: done
    from: finish
    to: _done
    when: "true"
start: start
done: _done
`

func writeTempCircuit(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "circuit.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// echoTransformer echoes input for testing.
type echoTransformer struct{}

func (e *echoTransformer) Name() string { return "echo" }
func (e *echoTransformer) Transform(_ context.Context, tc *engine.TransformerContext) (any, error) {
	return map[string]any{"echoed": tc.Input, "node": tc.NodeName}, nil
}

func TestRunWithInput(t *testing.T) {
	path := writeTempCircuit(t, testCircuitYAML)

	var buf bytes.Buffer
	r := NewCLIRunner(path,
		WithOutput(&buf),
		WithRunOptions(engine.WithTransformers(engine.TransformerRegistry{
			"echo": &echoTransformer{},
		})),
	)

	err := r.RunWithInput(context.Background(), map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("RunWithInput: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("expected output, got empty buffer")
	}

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	// The circuit has two nodes; we expect artifacts for both.
	if len(out) == 0 {
		t.Error("expected at least one artifact in output")
	}
}

func TestRunWithInput_NilInput(t *testing.T) {
	path := writeTempCircuit(t, testCircuitYAML)

	var buf bytes.Buffer
	r := NewCLIRunner(path,
		WithOutput(&buf),
		WithRunOptions(engine.WithTransformers(engine.TransformerRegistry{
			"echo": &echoTransformer{},
		})),
	)

	err := r.RunWithInput(context.Background(), nil)
	if err != nil {
		t.Fatalf("RunWithInput(nil): %v", err)
	}
}

func TestRunFromStdin(t *testing.T) {
	path := writeTempCircuit(t, testCircuitYAML)

	// Replace os.Stdin for the test.
	input := `{"msg": "from stdin"}`
	tmpFile, err := os.CreateTemp(t.TempDir(), "stdin-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.WriteString(input); err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	oldStdin := os.Stdin
	os.Stdin = tmpFile
	t.Cleanup(func() { os.Stdin = oldStdin })

	var buf bytes.Buffer
	r := NewCLIRunner(path,
		WithOutput(&buf),
		WithRunOptions(engine.WithTransformers(engine.TransformerRegistry{
			"echo": &echoTransformer{},
		})),
	)

	if err := r.RunFromStdin(context.Background()); err != nil {
		t.Fatalf("RunFromStdin: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("expected output, got empty buffer")
	}
}

func TestRunWithInput_InvalidCircuitPath(t *testing.T) {
	r := NewCLIRunner("/nonexistent/path/circuit.yaml")

	err := r.RunWithInput(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for invalid circuit path")
	}
	if !strings.Contains(err.Error(), "cli: run circuit") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunFromStdin_InvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "stdin-bad-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.WriteString("not json"); err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	oldStdin := os.Stdin
	os.Stdin = tmpFile
	t.Cleanup(func() { os.Stdin = oldStdin })

	r := NewCLIRunner("/any/path.yaml")

	err = r.RunFromStdin(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "cli: unmarshal stdin") {
		t.Errorf("unexpected error: %v", err)
	}
}
