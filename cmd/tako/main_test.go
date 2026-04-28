package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tako")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(getModuleRoot(t), "cmd", "tako")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build tako binary: %v\n%s", err, out)
	}
	return bin
}

func getModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir != "/" {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find go.mod")
	return ""
}

const integrationCircuit = `
circuit: cli-integration
vars:
  greeting: hello
nodes:
  - name: start
    approach: rapid
    instrument: transformer
    action: echo
  - name: finish
    approach: analytical
    instrument: transformer
    action: echo
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

func TestCLI_Validate(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	circuitPath := filepath.Join(dir, "circuit.yaml")
	if err := os.WriteFile(circuitPath, []byte(integrationCircuit), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "validate", circuitPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tako validate failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Error("expected output from validate")
	}
}

func TestCLI_Validate_Invalid(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	circuitPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(circuitPath, []byte("circuit: bad\nnodes: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "validate", circuitPath)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected validation to fail for invalid circuit")
	}
}

func TestCLI_Version(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tako version failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Error("expected version output")
	}
}

func TestCLI_UnknownCommand(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "nonexistent")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestCLI_Run(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()

	dataPath := filepath.Join(dir, "data.json")
	if err := os.WriteFile(dataPath, []byte(`{"result":"hello"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	circuitYAML := `
circuit: cli-run-integration
vars:
  mode: fast
nodes:
  - name: load
    approach: rapid
    instrument: transformer
    action: file
    prompt: data.json
  - name: classify
    approach: analytical
    instrument: transformer
    action: file
    input: "${load.output}"
    prompt: data.json
edges:
  - id: E1
    name: load-to-classify
    from: load
    to: classify
    when: "true"
  - id: E2
    name: done
    from: classify
    to: _done
    when: "true"
start: load
done: _done
`
	circuitPath := filepath.Join(dir, "circuit.yaml")
	if err := os.WriteFile(circuitPath, []byte(circuitYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "run", circuitPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tako run failed: %v\n%s", err, out)
	}
}

func TestCLI_Skill_Scaffold(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()

	circuitYAML := `
circuit: test-scaffold
nodes:
  - name: scan
    approach: rapid
    instrument: transformer
    action: llm
    prompt: "Scan for vulnerabilities"
  - name: classify
    approach: analytical
    instrument: transformer
    action: http
edges:
  - id: E1
    name: scan-to-classify
    from: scan
    to: classify
    when: "true"
  - id: E2
    name: done
    from: classify
    to: _done
    when: "true"
start: scan
done: _done
`
	circuitPath := filepath.Join(dir, "circuit.yaml")
	if err := os.WriteFile(circuitPath, []byte(circuitYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "skill-out")
	cmd := exec.Command(bin, "skill", "scaffold", "--tool", "mytest", "--out", outDir, circuitPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tako skill scaffold failed: %v\n%s", err, out)
	}

	skillPath := filepath.Join(outDir, "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read generated SKILL.md: %v", err)
	}

	checks := []string{
		"mytest-calibrate",
		"test-scaffold",
		"scan",
		"classify",
		"scan-to-classify",
		"start_calibration",
		"circuit(action: step)",
		"circuit(action: submit)",
		"circuit(action: report",
	}
	for _, check := range checks {
		if !strings.Contains(string(content), check) {
			t.Errorf("SKILL.md missing %q", check)
		}
	}
}

func TestCLI_Skill_Scaffold_DefaultOut(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()

	circuitYAML := `
circuit: myapp
nodes:
  - name: start
    approach: rapid
    instrument: transformer
    action: echo
edges:
  - id: E1
    name: done
    from: start
    to: _done
    when: "true"
start: start
done: _done
`
	circuitPath := filepath.Join(dir, "circuit.yaml")
	os.WriteFile(circuitPath, []byte(circuitYAML), 0o644)

	cmd := exec.Command(bin, "skill", "scaffold", circuitPath)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tako skill scaffold failed: %v\n%s", err, out)
	}

	skillPath := filepath.Join(dir, ".cursor", "skills", "myapp-calibrate", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected SKILL.md at %s: %v", skillPath, err)
	}
}
