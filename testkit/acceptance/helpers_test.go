package acceptance

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
)

// repoRoot returns the absolute path to the tako repo root.
func repoRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

// testdataPath returns the absolute path to a testdata fixture.
func testdataPath(t *testing.T, rel string) string {
	t.Helper()
	p := filepath.Join(repoRoot(), "testdata", rel)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture not found: %s", p)
	}
	return p
}

// contextEchoTransformer returns the walker context as the artifact output.
type contextEchoTransformer struct{}

func (t *contextEchoTransformer) Name() string { return "context-echo" }
func (t *contextEchoTransformer) Transform(_ context.Context, tc *engine.InstrumentContext) (any, error) {
	out := make(map[string]any)
	if tc.WalkerState != nil {
		for k, v := range tc.WalkerState.Context {
			out[k] = v
		}
	}
	return out, nil
}

// echoTransformer returns input + node name as artifact.
type echoTransformer struct{}

func (t *echoTransformer) Name() string { return "echo" }
func (t *echoTransformer) Transform(_ context.Context, tc *engine.InstrumentContext) (any, error) {
	return map[string]any{"echoed": tc.Input, "node": tc.NodeName}, nil
}

// standardTransformers returns registries with passthrough, echo, and context-echo.
func standardTransformers() engine.InstrumentRegistry {
	return engine.InstrumentRegistry{
		"echo":         &echoTransformer{},
		"context-echo": &contextEchoTransformer{},
	}
}

// standardRegistries returns GraphRegistries with standard test transformers.
func standardRegistries() *engine.GraphRegistries {
	return &engine.GraphRegistries{
		Instruments: standardTransformers(),
	}
}

// loadFixture reads and parses a circuit YAML fixture.
func loadFixture(t *testing.T, rel string) *circuit.CircuitDef {
	t.Helper()
	data, err := os.ReadFile(testdataPath(t, rel))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	def, err := circuit.LoadCircuit(data)
	if err != nil {
		t.Fatalf("parse circuit: %v", err)
	}
	return def
}

// runFixture runs a circuit file from testdata/ with standard registries.
func runFixture(t *testing.T, rel string, input any, opts ...engine.RunOption) error { //nolint:unparam // test flexibility
	t.Helper()
	absPath := testdataPath(t, rel)
	allOpts := append([]engine.RunOption{
		engine.WithInstruments(standardTransformers()),
	}, opts...)
	return engine.Run(context.Background(), absPath, input, allOpts...)
}
