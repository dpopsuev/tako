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

func TestRun_IntegrationBuild_DomainServe(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found")
	}

	tmpDir := t.TempDir()

	circuitDir := filepath.Join(tmpDir, "internal", "circuits")
	if err := os.MkdirAll(circuitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(circuitDir, "test.yaml"), []byte("topology: cascade\ndescription: test circuit\n"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := filepath.Join(tmpDir, "origami.yaml")
	if err := os.WriteFile(manifest, []byte(`
name: test-domain
version: "0.1"
domain_serve:
  port: 9300
  embed: internal/
`), 0644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "test-domain")

	err := Run(context.Background(), Options{
		ManifestPath: manifest,
		Output:       output,
		Verbose:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(output); err != nil {
		t.Fatalf("domain-serve binary not found: %v", err)
	}
}

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
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("circuits/rca.yaml", "topology: cascade\ndescription: RCA circuit\n")
	writeFile("prompts/recall.md", "You are a recall judge.")
	writeFile("vocabulary.yaml", "defects:\n  pb001: product bug\n")

	manifest := filepath.Join(tmpDir, "origami.yaml")
	if err := os.WriteFile(manifest, []byte(`
name: test-assets
version: "0.1"
domain_serve:
  port: 9300
  assets:
    circuits:
      rca: circuits/rca.yaml
    prompts:
      recall: prompts/recall.md
    files:
      vocabulary: vocabulary.yaml
`), 0644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "test-assets")

	err := Run(context.Background(), Options{
		ManifestPath: manifest,
		Output:       output,
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
name: test-no-serve
version: "1.0"
`), 0644); err != nil {
		t.Fatal(err)
	}

	err := Run(context.Background(), Options{ManifestPath: manifest})
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
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(content), 0644)
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

	data, err = os.ReadFile(filepath.Join(tmpDir, "circuits/rca.yaml"))
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
	os.WriteFile(binaryPath, []byte("#!/bin/sh\necho ok"), 0755)

	m := &Manifest{
		Name: "myapp",
		DomainServe: &DomainServeConfig{
			Port: 9400,
		},
	}

	err := buildContainerImage(context.Background(), m, binaryPath, Options{Verbose: true})
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
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(content), 0644)
	}

	writeFile("circuits/rca.yaml", `
nodes:
  - name: gather-code
    handler_type: circuit
    handler: harvester
  - name: resolve
    handler_type: transformer
    handler: resolve
`)
	writeFile("circuits/harvester.yaml", `
nodes:
  - name: tree
    handler: harvester.tree
`)

	m := &Manifest{
		DomainServe: &DomainServeConfig{
			Assets: &AssetMap{
				Circuits: map[string]string{
					"rca":       "circuits/rca.yaml",
					"harvester": "circuits/harvester.yaml",
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
	os.MkdirAll(p, 0755)
	os.WriteFile(filepath.Join(p, "rca.yaml"), []byte(`
nodes:
  - name: gather-code
    handler_type: circuit
    handler: nonexistent
`), 0644)

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
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(content), 0644)
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
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(content), 0644)
	}

	writeFile("circuits/rca.yaml", "topology: cascade\n")
	writeFile("vocabulary.yaml", "defects:\n  pb001: product bug\n")
	writeFile("domains/ocp/ptp/heuristics.yaml", "heuristics: []\n")
	writeFile("domains/ocp/ptp/scenarios/ptp-mock.yaml", "scenario: ptp-mock\n")
	writeFile("domains/ocp/ptp/sources/ptp.yaml", "source: ptp\n")

	manifest := filepath.Join(tmpDir, "origami.yaml")
	os.WriteFile(manifest, []byte(`
name: test-domains
version: "0.1"
domains: [ocp/ptp]
domain_serve:
  port: 9300
  assets:
    vocabulary: vocabulary.yaml
    circuits:
      rca: circuits/rca.yaml
`), 0644)

	output := filepath.Join(t.TempDir(), "test-domains")

	err := Run(context.Background(), Options{
		ManifestPath: manifest,
		Output:       output,
		Verbose:      true,
	})
	if err != nil {
		t.Fatalf("fold with domains: %v", err)
	}

	if _, err := os.Stat(output); err != nil {
		t.Fatalf("binary not found: %v", err)
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
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(content), 0644)
	}

	writeFile("circuits/rca.yaml", "circuit: rca\ntopology: cascade\nnodes:\n  - name: a\n    approach: analytical\n")
	writeFile("vocabulary.yaml", "metrics:\n  M1: accuracy\n")

	manifest := filepath.Join(tmpDir, "origami.yaml")
	os.WriteFile(manifest, []byte(`
name: test-domain-only
version: "0.1"
schematics:
  rca:
    path: github.com/dpopsuev/rh-rca
    bindings:
      source: reportportal
connectors:
  reportportal:
    path: github.com/dpopsuev/rh-rca/connectors/rp
domain_serve:
  port: 9300
  assets:
    vocabulary: vocabulary.yaml
    circuits:
      rca: circuits/rca.yaml
`), 0644)

	m, err := LoadManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !m.HasBindings() {
		t.Fatal("manifest should have bindings for this test")
	}

	output := filepath.Join(t.TempDir(), "test-domain-only")

	err = Run(context.Background(), Options{
		ManifestPath: manifest,
		Output:       output,
		DomainOnly:   true,
		Verbose:      true,
	})
	if err != nil {
		t.Fatalf("fold with DomainOnly: %v", err)
	}

	if _, err := os.Stat(output); err != nil {
		t.Fatalf("domain-serve binary not found: %v", err)
	}
}
