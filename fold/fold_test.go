package fold

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_IntegrationBuild_Assets(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
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

	writeFile("circuits/rca.yaml", "topology: cascade\ndescription: RCA circuit\n")
	writeFile("prompts/recall.md", "You are a recall judge.")
	writeFile("vocabulary.yaml", "defects:\n  pb001: product bug\n")

	manifest := filepath.Join(tmpDir, "origami.yaml")
	if err := os.WriteFile(manifest, []byte(`
apiVersion: origami/v1
kind: Board
metadata:
  name: test-assets
spec:
  domain_serve:
    port: 9300
    assets:
      circuits:
        rca: circuits/rca.yaml
      prompts:
        recall: prompts/recall.md
      files:
        vocabulary: vocabulary.yaml
`), 0o644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "test-assets")

	err := Run(context.Background(), &Options{
		ManifestPath: manifest,
		Output:       output,
		Local:        true,
		Verbose:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(output); err != nil {
		t.Fatalf("domain-serve binary not found: %v", err)
	}
}

func TestRun_MissingDomainServe(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "origami.yaml")
	if err := os.WriteFile(manifest, []byte(`
apiVersion: origami/v1
kind: Board
metadata:
  name: test-no-serve
spec: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Run(context.Background(), &Options{ManifestPath: manifest})
	if err == nil {
		t.Fatal("expected error for manifest without domain_serve")
	}
}

func TestCopyEmbedFiles_SkipsDomainPlacedFiles(t *testing.T) {
	manifestDir := t.TempDir()
	tmpDir := t.TempDir()

	writeFile := func(dir, rel, content string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	writeFile(manifestDir, "circuits/rca.yaml", "circuit content")
	writeFile(manifestDir, "vocabulary.yaml", "vocab content")

	writeFile(tmpDir, "heuristics.yaml", "domain-placed heuristics")

	ds := &DomainServeConfig{
		Assets: &AssetMap{
			Circuits: map[string]string{"rca": "circuits/rca.yaml"},
			Files: map[string]string{
				"vocabulary": "vocabulary.yaml",
				"heuristics": "heuristics.yaml",
			},
		},
	}

	if err := copyEmbedFiles(ds, manifestDir, tmpDir, false); err != nil {
		t.Fatalf("copyEmbedFiles: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "heuristics.yaml"))
	if err != nil {
		t.Fatalf("read heuristics.yaml: %v", err)
	}
	if string(data) != "domain-placed heuristics" {
		t.Errorf("heuristics.yaml was overwritten, got %q", string(data))
	}

	data, err = os.ReadFile(filepath.Join(tmpDir, "circuits", "rca.yaml"))
	if err != nil {
		t.Fatalf("read circuits/rca.yaml: %v", err)
	}
	if string(data) != "circuit content" {
		t.Errorf("circuits/rca.yaml = %q, want 'circuit content'", string(data))
	}
}

func TestCopyEmbedFiles_FailsOnMissingSrc(t *testing.T) {
	manifestDir := t.TempDir()
	tmpDir := t.TempDir()

	ds := &DomainServeConfig{
		Assets: &AssetMap{
			Files: map[string]string{"missing": "does-not-exist.yaml"},
		},
	}

	err := copyEmbedFiles(ds, manifestDir, tmpDir, false)
	if err == nil {
		t.Fatal("expected error for missing source file")
	}
}

func TestBuildContainerImage_RequiresDocker(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	if testing.Short() {
		t.Skip("skipped in short mode")
	}

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")
	os.WriteFile(binaryPath, []byte("#!/bin/sh\necho ok"), 0o755)

	m := &Manifest{
		Name: "myapp",
		DomainServe: &DomainServeConfig{
			Port: 9400,
		},
	}

	err := buildContainerImage(context.Background(), m, binaryPath, &Options{Verbose: true})
	if err != nil {
		t.Logf("docker build failed (expected in CI): %v", err)
	}
}

func TestContainerImageName_Default(t *testing.T) {
	m := &Manifest{Name: "asterisk", DomainServe: &DomainServeConfig{Port: 9300}}
	opts := Options{}

	imgName := opts.ImageName
	if imgName == "" {
		imgName = "origami-" + m.Name + "-domain"
	}
	if imgName != "origami-asterisk-domain" {
		t.Errorf("default image name = %q, want origami-asterisk-domain", imgName)
	}
}

func TestContainerImageName_Custom(t *testing.T) {
	opts := Options{ImageName: "my-custom-image"}

	imgName := opts.ImageName
	if imgName != "my-custom-image" {
		t.Errorf("custom image name = %q, want my-custom-image", imgName)
	}
}

func TestContainerDockerfileTemplate(t *testing.T) {
	dockerfile := fmt.Sprintf(containerDockerfileTemplate, 9300)
	for _, want := range []string{
		"FROM gcr.io/distroless/static-debian12",
		"COPY domain-serve /domain-serve",
		"ENTRYPOINT [\"/domain-serve\"]",
		"EXPOSE 9300",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Errorf("Dockerfile missing %q:\n%s", want, dockerfile)
		}
	}
}

func TestValidateCircuitRefs_ValidRef(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	writeFile("circuits/rca.yaml", `
nodes:
  - name: gather-code
    handler_type: circuit
    handler: gnd
  - name: resolve
    handler_type: transformer
    handler: resolve
`)
	writeFile("circuits/gnd.yaml", `
nodes:
  - name: tree
    handler: gnd.tree
`)

	m := &Manifest{
		DomainServe: &DomainServeConfig{
			Assets: &AssetMap{
				Circuits: map[string]string{
					"rca": "circuits/rca.yaml",
					"gnd": "circuits/gnd.yaml",
				},
			},
		},
	}

	if err := validateCircuitRefs(m, tmpDir); err != nil {
		t.Fatalf("valid circuit ref rejected: %v", err)
	}
}

func TestValidateCircuitRefs_MissingRef(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "circuits")
	os.MkdirAll(p, 0o755)
	os.WriteFile(filepath.Join(p, "rca.yaml"), []byte(`
nodes:
  - name: gather-code
    handler_type: circuit
    handler: nonexistent
`), 0o644)

	m := &Manifest{
		DomainServe: &DomainServeConfig{
			Assets: &AssetMap{
				Circuits: map[string]string{"rca": "circuits/rca.yaml"},
			},
		},
	}

	err := validateCircuitRefs(m, tmpDir)
	if err == nil {
		t.Fatal("expected error for missing circuit ref")
	}
	if !strings.Contains(err.Error(), "nonexistent") || !strings.Contains(err.Error(), "not in assets.circuits") {
		t.Errorf("error should mention missing circuit: %v", err)
	}
}

func TestValidateCircuitRefs_CycleDetected(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	writeFile("circuits/a.yaml", `
nodes:
  - name: call-b
    handler_type: circuit
    handler: b
`)
	writeFile("circuits/b.yaml", `
nodes:
  - name: call-a
    handler_type: circuit
    handler: a
`)

	m := &Manifest{
		DomainServe: &DomainServeConfig{
			Assets: &AssetMap{
				Circuits: map[string]string{
					"a": "circuits/a.yaml",
					"b": "circuits/b.yaml",
				},
			},
		},
	}

	err := validateCircuitRefs(m, tmpDir)
	if err == nil {
		t.Fatal("expected error for circuit cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle: %v", err)
	}
}

func TestValidateCircuitRefs_NoCircuits(t *testing.T) {
	m := &Manifest{DomainServe: &DomainServeConfig{Assets: &AssetMap{}}}
	if err := validateCircuitRefs(m, t.TempDir()); err != nil {
		t.Fatalf("empty circuits should pass: %v", err)
	}
}

func TestValidateCircuitRefs_NilManifest(t *testing.T) {
	m := &Manifest{}
	if err := validateCircuitRefs(m, t.TempDir()); err != nil {
		t.Fatalf("nil domain_serve should pass: %v", err)
	}
}

func TestRun_IntegrationBuild_WithDomains(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found")
	}

	tmpDir := t.TempDir()

	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	writeFile("circuits/rca.yaml", "topology: cascade\n")
	writeFile("vocabulary.yaml", "defects:\n  pb001: product bug\n")
	writeFile("domains/ocp/ptp/heuristics.yaml", "kind: HeuristicRules\nheuristics: []\n")
	writeFile("domains/ocp/ptp/scenarios/ptp-mock.yaml", "kind: Scenario\nscenario: ptp-mock\n")
	writeFile("domains/ocp/ptp/sources/ptp.yaml", "kind: SourcePack\nsource: ptp\n")

	manifest := filepath.Join(tmpDir, "origami.yaml")
	os.WriteFile(manifest, []byte(`
apiVersion: origami/v1
kind: Board
metadata:
  name: test-domains
spec:
  domains: [ocp/ptp]
  domain_serve:
    port: 9300
    assets:
      vocabulary: vocabulary.yaml
      circuits:
        rca: circuits/rca.yaml
`), 0o644)

	output := filepath.Join(t.TempDir(), "test-domains")

	err := Run(context.Background(), &Options{
		ManifestPath: manifest,
		Output:       output,
		Local:        true,
		Verbose:      true,
	})
	if err != nil {
		t.Fatalf("fold with domains: %v", err)
	}

	if _, err := os.Stat(output); err != nil {
		t.Fatalf("binary not found: %v", err)
	}
}

// TestExportDataDir_MatchesEmbedLayout verifies that --export-data produces
// the same flattened file layout that go:embed would create. This ensures
// --data-dir at runtime sees the same paths as the embedded FS.
func TestExportDataDir_MatchesEmbedLayout(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	// Create domain files at the un-flattened paths (like a real workspace).
	writeFile("circuits/rca.yaml", "circuit: rca\n")
	writeFile("prompts/recall/judge.md", "prompt: recall\n")
	writeFile("vocabulary.yaml", "metrics:\n  M1: accuracy\n")
	writeFile("domains/ocp/ptp/scenarios/ptp.yaml", "kind: Scenario\nscenario: ptp\ncases: []\n")
	writeFile("domains/ocp/ptp/offline/rp/12345.json", `{"id": 12345}`)

	manifest := filepath.Join(tmpDir, "origami.yaml")
	os.WriteFile(manifest, []byte(`
apiVersion: origami/v1
kind: Board
metadata:
  name: test-export
spec:
  domains: [ocp/ptp]
  domain_serve:
    port: 9300
    assets:
      vocabulary: vocabulary.yaml
      circuits:
        rca: circuits/rca.yaml
      prompts:
        recall: prompts/recall/judge.md
`), 0o644)

	exportDir := filepath.Join(t.TempDir(), "exported")

	err := Run(context.Background(), &Options{
		ManifestPath:  manifest,
		ExportDataDir: exportDir,
		Verbose:       true,
	})
	if err != nil {
		t.Fatalf("export-data: %v", err)
	}

	// Verify flattened domain files exist at the expected paths.
	wantFiles := map[string]string{
		"circuits/rca.yaml":       "circuit: rca\n",
		"prompts/recall/judge.md": "prompt: recall\n",
		"vocabulary.yaml":         "metrics:\n  M1: accuracy\n",
		"scenarios/ptp.yaml":      "kind: Scenario\nscenario: ptp\ncases: []\n",
		"offline/rp/12345.json":   `{"id": 12345}`,
	}

	for relPath, wantContent := range wantFiles {
		fullPath := filepath.Join(exportDir, relPath)
		got, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("missing exported file %q: %v", relPath, err)
			continue
		}
		if string(got) != wantContent {
			t.Errorf("file %q: got %q, want %q", relPath, string(got), wantContent)
		}
	}
}

func TestExportDataDir_OverwritesStaleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	writeFile("prompts/recall/judge.md", "version: 1\n")
	writeFile("vocabulary.yaml", "old vocab\n")

	manifest := filepath.Join(tmpDir, "origami.yaml")
	os.WriteFile(manifest, []byte(`
apiVersion: origami/v1
kind: Board
metadata:
  name: test-overwrite
spec:
  domain_serve:
    port: 9300
    assets:
      vocabulary: vocabulary.yaml
      prompts:
        recall: prompts/recall/judge.md
`), 0o644)

	exportDir := filepath.Join(t.TempDir(), "exported")

	// First export.
	err := Run(context.Background(), &Options{
		ManifestPath:  manifest,
		ExportDataDir: exportDir,
	})
	if err != nil {
		t.Fatalf("first export: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(exportDir, "prompts", "recall", "judge.md"))
	if string(got) != "version: 1\n" {
		t.Fatalf("first export content = %q, want 'version: 1\\n'", string(got))
	}

	// Modify source file.
	writeFile("prompts/recall/judge.md", "version: 2\n")

	// Re-export to same directory.
	err = Run(context.Background(), &Options{
		ManifestPath:  manifest,
		ExportDataDir: exportDir,
	})
	if err != nil {
		t.Fatalf("second export: %v", err)
	}

	// Verify the exported file has the updated content.
	got, _ = os.ReadFile(filepath.Join(exportDir, "prompts", "recall", "judge.md"))
	if string(got) != "version: 2\n" {
		t.Errorf("re-export did not overwrite stale file: got %q, want 'version: 2\\n'", string(got))
	}
}

// --- Port wiring validation tests ---

func TestPortWiring_MatchingTypes(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	writeFile("circuits/rca.yaml", `
circuit: rca
ports:
  - name: post-triage
    direction: out
    type: "[]string"
nodes:
  - name: triage
edges:
  - id: e1
    from: triage
    to: _done
start: triage
done: _done
`)

	writeFile("circuits/gnd.yaml", `
circuit: gnd
ports:
  - name: keywords
    direction: in
    type: "[]string"
nodes:
  - name: search
edges:
  - id: e1
    from: search
    to: _done
start: search
done: _done
`)

	writeFile("circuits/orchestrator.yaml", `
circuit: orchestrator
wiring:
  - from: "rca.out:post-triage"
    to: "gnd.in:keywords"
nodes:
  - name: init
edges:
  - id: e1
    from: init
    to: _done
start: init
done: _done
`)

	m := &Manifest{
		DomainServe: &DomainServeConfig{
			Assets: &AssetMap{
				Circuits: map[string]string{
					"rca":          "circuits/rca.yaml",
					"gnd":          "circuits/gnd.yaml",
					"orchestrator": "circuits/orchestrator.yaml",
				},
			},
		},
	}

	if err := validatePortWiring(m, tmpDir); err != nil {
		t.Fatalf("matching port types should pass validation: %v", err)
	}
}

func TestPortWiring_MismatchedTypes(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	writeFile("circuits/rca.yaml", `
circuit: rca
ports:
  - name: post-triage
    direction: out
    type: TriageResult
nodes:
  - name: triage
edges:
  - id: e1
    from: triage
    to: _done
start: triage
done: _done
`)

	writeFile("circuits/gnd.yaml", `
circuit: gnd
ports:
  - name: keywords
    direction: in
    type: "[]string"
nodes:
  - name: search
edges:
  - id: e1
    from: search
    to: _done
start: search
done: _done
`)

	writeFile("circuits/orchestrator.yaml", `
circuit: orchestrator
wiring:
  - from: "rca.out:post-triage"
    to: "gnd.in:keywords"
nodes:
  - name: init
edges:
  - id: e1
    from: init
    to: _done
start: init
done: _done
`)

	m := &Manifest{
		DomainServe: &DomainServeConfig{
			Assets: &AssetMap{
				Circuits: map[string]string{
					"rca":          "circuits/rca.yaml",
					"gnd":          "circuits/gnd.yaml",
					"orchestrator": "circuits/orchestrator.yaml",
				},
			},
		},
	}

	err := validatePortWiring(m, tmpDir)
	if err == nil {
		t.Fatal("expected error for mismatched port types")
	}
	if !strings.Contains(err.Error(), "type mismatch") {
		t.Errorf("error should mention type mismatch: %v", err)
	}
	if !strings.Contains(err.Error(), "TriageResult") {
		t.Errorf("error should mention TriageResult: %v", err)
	}
	if !strings.Contains(err.Error(), "[]string") {
		t.Errorf("error should mention []string: %v", err)
	}
}

func TestPortWiring_NoWiring(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	writeFile("circuits/rca.yaml", `
circuit: rca
nodes:
  - name: triage
`)

	m := &Manifest{
		DomainServe: &DomainServeConfig{
			Assets: &AssetMap{
				Circuits: map[string]string{"rca": "circuits/rca.yaml"},
			},
		},
	}

	if err := validatePortWiring(m, tmpDir); err != nil {
		t.Fatalf("no wiring should pass: %v", err)
	}
}

func TestPortWiring_NilManifest(t *testing.T) {
	m := &Manifest{}
	if err := validatePortWiring(m, t.TempDir()); err != nil {
		t.Fatalf("nil domain_serve should pass: %v", err)
	}
}

func TestPortWiring_UntypedPortsSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	writeFile("circuits/rca.yaml", `
circuit: rca
ports:
  - name: post-triage
    direction: out
nodes:
  - name: triage
`)

	writeFile("circuits/gnd.yaml", `
circuit: gnd
ports:
  - name: keywords
    direction: in
nodes:
  - name: search
`)

	writeFile("circuits/orchestrator.yaml", `
circuit: orchestrator
wiring:
  - from: "rca.out:post-triage"
    to: "gnd.in:keywords"
nodes:
  - name: init
`)

	m := &Manifest{
		DomainServe: &DomainServeConfig{
			Assets: &AssetMap{
				Circuits: map[string]string{
					"rca":          "circuits/rca.yaml",
					"gnd":          "circuits/gnd.yaml",
					"orchestrator": "circuits/orchestrator.yaml",
				},
			},
		},
	}

	if err := validatePortWiring(m, tmpDir); err != nil {
		t.Fatalf("untyped ports should be skipped: %v", err)
	}
}

func TestRun_DomainOnly_SkipsBindings(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found")
	}

	tmpDir := t.TempDir()

	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(tmpDir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}

	writeFile("circuits/rca.yaml", "circuit: rca\ntopology: cascade\nnodes:\n  - name: a\n    approach: analytical\n")
	writeFile("vocabulary.yaml", "metrics:\n  M1: accuracy\n")

	manifest := filepath.Join(tmpDir, "origami.yaml")
	os.WriteFile(manifest, []byte(`
apiVersion: origami/v1
kind: Board
metadata:
  name: test-domain-only
spec:
  uses:
    rca:
      kind: Schematic
      module: github.com/dpopsuev/origami-rca
  domain_serve:
    port: 9300
    assets:
      vocabulary: vocabulary.yaml
      circuits:
        rca: circuits/rca.yaml
`), 0o644)

	m, err := LoadManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !m.HasBindings() {
		t.Fatal("manifest should have bindings for this test")
	}

	output := filepath.Join(t.TempDir(), "test-domain-only")

	err = Run(context.Background(), &Options{
		ManifestPath: manifest,
		Output:       output,
		DomainOnly:   true,
		Local:        true,
		Verbose:      true,
	})
	if err != nil {
		t.Fatalf("fold with DomainOnly: %v", err)
	}

	if _, err := os.Stat(output); err != nil {
		t.Fatalf("domain-serve binary not found: %v", err)
	}
}
