package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "origami")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(getModuleRoot(t), "cmd", "origami")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build origami binary: %v\n%s", err, out)
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
handler_type: transformer
vars:
  greeting: hello
nodes:
  - name: start
    approach: rapid
    handler: echo
  - name: finish
    approach: analytical
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

func TestCLI_Validate(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	circuitPath := filepath.Join(dir, "circuit.yaml")
	if err := os.WriteFile(circuitPath, []byte(integrationCircuit), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "validate", circuitPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("origami validate failed: %v\n%s", err, out)
	}
	if len(out) == 0 {
		t.Error("expected output from validate")
	}
}

func TestCLI_Validate_Invalid(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	circuitPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(circuitPath, []byte("circuit: bad\nnodes: []\n"), 0644); err != nil {
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
		t.Fatalf("origami version failed: %v\n%s", err, out)
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
	if err := os.WriteFile(dataPath, []byte(`{"result":"hello"}`), 0644); err != nil {
		t.Fatal(err)
	}

	circuitYAML := `
circuit: cli-run-integration
handler_type: transformer
vars:
  mode: fast
nodes:
  - name: load
    approach: rapid
    handler: file
    prompt: data.json
  - name: classify
    approach: analytical
    handler: file
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
	if err := os.WriteFile(circuitPath, []byte(circuitYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "run", circuitPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("origami run failed: %v\n%s", err, out)
	}
}

func TestOuroboros_Analyze_Stdin(t *testing.T) {
	goldenResponse := `{"model_name": "claude-sonnet-4-20250514", "provider": "Anthropic", "version": "20250514", "wrapper": "Cursor"}

` + "```go\n" + `func sumAbsolute(numbers []int, label string, verbose bool) (int, string, error) {
	total := 0
	for _, num := range numbers {
		if num > 0 { total += num } else if num < 0 { total -= num }
	}
	if total == 0 { return 0, "", fmt.Errorf("empty result for %s", label) }
	return total, "", nil
}
` + "```\n"

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte(goldenResponse))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	oldStdout := os.Stdout
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = outW
	defer func() { os.Stdout = oldStdout }()

	analyzeErr := ouroborosAnalyze([]string{"--response-file", "-"})

	outW.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, outR)
	os.Stdout = oldStdout

	if analyzeErr != nil {
		t.Fatalf("ouroborosAnalyze with stdin failed: %v", analyzeErr)
	}

	var result analyzeResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("parse output JSON: %v\nraw: %s", err, buf.String())
	}

	if result.Identity.ModelName != "claude-sonnet-4-20250514" {
		t.Errorf("model_name: got %q, want claude-sonnet-4-20250514", result.Identity.ModelName)
	}
	if result.Identity.Provider != "Anthropic" {
		t.Errorf("provider: got %q, want Anthropic", result.Identity.Provider)
	}
	if result.Key != "claude-sonnet-4-20250514" {
		t.Errorf("key: got %q, want claude-sonnet-4-20250514", result.Key)
	}
	if result.Wrapper {
		t.Error("wrapper should be false for foundation model")
	}
	if result.Code == "" {
		t.Error("code should not be empty")
	}
}

func TestCLI_Skill_Scaffold(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()

	circuitYAML := `
circuit: test-scaffold
handler_type: transformer
nodes:
  - name: scan
    approach: rapid
    handler: llm
    prompt: "Scan for vulnerabilities"
  - name: classify
    approach: analytical
    handler: http
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
	if err := os.WriteFile(circuitPath, []byte(circuitYAML), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "skill-out")
	cmd := exec.Command(bin, "skill", "scaffold", "--tool", "mytest", "--out", outDir, circuitPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("origami skill scaffold failed: %v\n%s", err, out)
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
handler_type: transformer
nodes:
  - name: start
    approach: rapid
    handler: echo
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
	os.WriteFile(circuitPath, []byte(circuitYAML), 0644)

	cmd := exec.Command(bin, "skill", "scaffold", circuitPath)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("origami skill scaffold failed: %v\n%s", err, out)
	}

	skillPath := filepath.Join(dir, ".cursor", "skills", "myapp-calibrate", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected SKILL.md at %s: %v", skillPath, err)
	}
}

func TestOuroboros_Save_Stdin(t *testing.T) {
	report := `{
  "run_id": "test-stdin-save",
  "start_time": "2026-02-21T12:00:00Z",
  "end_time": "2026-02-21T12:01:00Z",
  "config": {"max_iterations": 15, "probe_id": "refactor-v1", "terminate_on_repeat": true},
  "results": [],
  "unique_models": [],
  "termination_reason": "stdin test"
}`

	runsDir := t.TempDir()

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte(report))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	saveErr := ouroborosSave([]string{"--report-file", "-", "--runs-dir", runsDir})
	if saveErr != nil {
		t.Fatalf("ouroborosSave with stdin failed: %v", saveErr)
	}

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatalf("read runs dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Name() == "test-stdin-save.json" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected test-stdin-save.json in %s, got %v", runsDir, entries)
	}
}
