package fold

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRuntime_DomainServeBinary_Starts builds a domain-serve binary and
// verifies it starts and responds to --version. This is the runtime
// validation that codegen-only tests miss — the binary must be executable.
func TestRuntime_DomainServeBinary_Starts(t *testing.T) {
	if testing.Short() {
		t.Skip("runtime test skipped in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found")
	}

	tmpDir := t.TempDir()

	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("circuits/test.yaml", "kind: Schematic\ncircuit: test\nnodes:\n  - name: a\n    handler: transformer:passthrough\nedges: []\nstart: a\ndone: a\n")
	writeFile("prompts/recall.md", "You are a test prompt.")

	manifest := filepath.Join(tmpDir, "origami.yaml")
	if err := os.WriteFile(manifest, []byte(`
apiVersion: origami/v1
kind: Board
metadata:
  name: runtime-test
spec:
  domain_serve:
    port: 19876
    assets:
      circuits:
        test: circuits/test.yaml
      prompts:
        recall: prompts/recall.md
`), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "bin")
	outputPath := filepath.Join(outputDir, "runtime-test-domain-serve")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := Run(ctx, &Options{
		ManifestPath: manifest,
		Output:       outputPath,
		DomainOnly:   true,
		Local:        true,
	})
	if err != nil {
		t.Fatalf("fold Run: %v", err)
	}

	// Verify binary exists and is executable.
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("binary not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("binary is empty")
	}

	// Run with --healthz (will fail because no server is running, but proves
	// the binary starts and exits — not a crash, not a missing-import panic).
	cmd := exec.CommandContext(ctx, outputPath, "--healthz")
	out, err := cmd.CombinedOutput()
	// --healthz exits 1 because no server is listening — that's expected.
	// What we're checking: the binary RUNS (no panic, no missing symbol).
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("unexpected error type: %v", err)
		}
		if exitErr.ExitCode() != 1 {
			t.Fatalf("expected exit code 1 (no server), got %d: %s", exitErr.ExitCode(), out)
		}
		// Exit 1 = binary ran, healthz failed (expected). Good.
		return
	}
	// Exit 0 = somehow a server was running on that port. Also fine.
	_ = out
}

// TestRuntime_GeneratedCode_ContainsResourceRegistry verifies the fold-generated
// code for wired binaries includes ResourceRegistry wiring.
func TestRuntime_GeneratedCode_ContainsResourceRegistry(t *testing.T) {
	root := origamiRootE2E(t)
	m := &Manifest{
		Name:    "resource-test",
		Version: "1.0",
		DomainServe: &DomainServeConfig{
			Port:   9300,
			Assets: &AssetMap{Circuits: map[string]string{"test": "circuits/test.yaml"}},
		},
		Schematics: map[string]SchematicRef{
			"rca": {
				Path:     "github.com/dpopsuev/origami-rca",
				Bindings: map[string]string{"source": "reportportal"},
			},
		},
		Connectors: map[string]ConnectorRef{
			"reportportal": {Path: "github.com/dpopsuev/origami-rca/connectors/rp"},
		},
	}

	g, err := Resolve(m, root, &DefaultModuleResolver{})
	if err != nil {
		t.Fatal(err)
	}

	// Use factory mode.
	g.Root.SessionFactory = "Factory()"

	src, err := GenerateWiredBinary(m, g)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)

	for _, want := range []string{
		"fwresource.DefaultRegistry()",
		"bridgedCfg.ResourceRegistry",
		"bridgedCfg.StateDir",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code missing %q", want)
		}
	}
}
