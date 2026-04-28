package def

import (
	"errors"
	"os"
	"testing"
)

func TestLoadInstrumentManifest_ValidExec(t *testing.T) {
	m, err := LoadInstrumentManifest("testdata/component/valid-instrument.yaml")
	if err != nil {
		t.Fatalf("LoadInstrumentManifest: %v", err)
	}
	if m.Kind != KindInstrument {
		t.Errorf("Kind = %q, want %q", m.Kind, KindInstrument)
	}
	if m.Name != "oculus" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Namespace != "instruments" {
		t.Errorf("Namespace = %q", m.Namespace)
	}
	if m.Dispatch != DispatchCLI {
		t.Errorf("Dispatch = %q, want exec", m.Dispatch)
	}
	if m.Binary != "oculus" {
		t.Errorf("Binary = %q", m.Binary)
	}
	if m.Tune != "--version" {
		t.Errorf("Tune = %q", m.Tune)
	}
	if len(m.Actions) != 2 {
		t.Fatalf("Actions count = %d, want 2", len(m.Actions))
	}
	scan, ok := m.Actions["scan"]
	if !ok {
		t.Fatal("missing scan action")
	}
	if scan.Command != "scan --format=json" {
		t.Errorf("scan.Command = %q", scan.Command)
	}
	if scan.InputSchema == "" {
		t.Error("scan.InputSchema is empty")
	}
	if scan.OutputSchema == "" {
		t.Error("scan.OutputSchema is empty")
	}
	if !m.HasAction("layers") {
		t.Error("missing layers action")
	}
}

func TestLoadInstrumentManifest_ValidMCP(t *testing.T) {
	m, err := LoadInstrumentManifest("testdata/component/instrument-mcp.yaml")
	if err != nil {
		t.Fatalf("LoadInstrumentManifest: %v", err)
	}
	if m.Dispatch != DispatchMCP {
		t.Errorf("Dispatch = %q, want mcp", m.Dispatch)
	}
	if m.Endpoint != "http://localhost:8080/mcp" {
		t.Errorf("Endpoint = %q", m.Endpoint)
	}
	if len(m.Actions) != 2 {
		t.Errorf("Actions count = %d, want 2", len(m.Actions))
	}
}

func TestLoadInstrumentManifest_MissingTune(t *testing.T) {
	_, err := LoadInstrumentManifest("testdata/component/instrument-missing-tune.yaml")
	if err == nil {
		t.Fatal("expected error for missing tune")
	}
	if !errors.Is(err, ErrInstrumentManifest) {
		t.Errorf("expected ErrInstrumentManifest, got %v", err)
	}
}

func TestLoadInstrumentManifest_InvalidDispatch(t *testing.T) {
	_, err := LoadInstrumentManifest("testdata/component/instrument-invalid-dispatch.yaml")
	if err == nil {
		t.Fatal("expected error for invalid dispatch mode")
	}
	if !errors.Is(err, ErrInstrumentManifest) {
		t.Errorf("expected ErrInstrumentManifest, got %v", err)
	}
}

func TestLoadInstrumentManifest_WrongKind(t *testing.T) {
	_, err := LoadInstrumentManifest("testdata/component/valid-component.yaml")
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

func TestLoadInstrumentManifest_FileNotFound(t *testing.T) {
	_, err := LoadInstrumentManifest("testdata/component/nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseInstrumentManifest_ExecMissingCommand(t *testing.T) {
	data := []byte(`apiVersion: tako/v1
kind: Instrument
metadata:
  name: test
  namespace: instruments
spec:
  dispatch: cli
  binary: bash
  tune: "test --version"
  actions:
    run: {}
`)
	_, err := ParseInstrumentManifest(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for exec action without command")
	}
	if !errors.Is(err, ErrInstrumentManifest) {
		t.Errorf("expected ErrInstrumentManifest, got %v", err)
	}
}

func TestParseInstrumentManifest_MCPMissingEndpoint(t *testing.T) {
	data := []byte(`apiVersion: tako/v1
kind: Instrument
metadata:
  name: test
  namespace: instruments
spec:
  dispatch: mcp
  tune: "curl -sf http://localhost/health"
  actions:
    scan:
      command: "scan"
`)
	_, err := ParseInstrumentManifest(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for mcp dispatch without endpoint")
	}
}

func TestParseInstrumentManifest_DockerMissingImage(t *testing.T) {
	data := []byte(`apiVersion: tako/v1
kind: Instrument
metadata:
  name: test
  namespace: instruments
spec:
  dispatch: container
  tune: "docker inspect test"
  actions:
    scan:
      command: "scan --json"
`)
	_, err := ParseInstrumentManifest(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for docker dispatch without image")
	}
}

func TestParseInstrumentManifest_GoMissingGoFunc(t *testing.T) {
	data := []byte(`apiVersion: tako/v1
kind: Instrument
metadata:
  name: test
  namespace: instruments
spec:
  dispatch: inproc
  tune: "true"
  actions:
    transform: {}
`)
	_, err := ParseInstrumentManifest(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for go action without go_func")
	}
}

func TestParseInstrumentManifest_NoActions(t *testing.T) {
	data := []byte(`apiVersion: tako/v1
kind: Instrument
metadata:
  name: test
  namespace: instruments
spec:
  dispatch: cli
  binary: bash
  tune: "test --version"
`)
	_, err := ParseInstrumentManifest(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for empty actions")
	}
}

func TestParseInstrumentManifest_MissingAPIVersion(t *testing.T) {
	data := []byte(`kind: Instrument
metadata:
  name: test
  namespace: instruments
spec:
  dispatch: cli
  binary: bash
  tune: "test --version"
  actions:
    run:
      command: "test run"
`)
	_, err := ParseInstrumentManifest(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for missing apiVersion")
	}
}

func TestParseInstrumentManifest_MissingName(t *testing.T) {
	data := []byte(`apiVersion: tako/v1
kind: Instrument
metadata:
  namespace: instruments
spec:
  dispatch: cli
  binary: bash
  tune: "test --version"
  actions:
    run:
      command: "test run"
`)
	_, err := ParseInstrumentManifest(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseInstrumentManifest_UnknownKind(t *testing.T) {
	data := []byte(`apiVersion: tako/v1
kind: Bogus
metadata:
  name: test
  namespace: instruments
spec:
  dispatch: cli
  binary: bash
  tune: "test --version"
  actions:
    run:
      command: "test run"
`)
	_, err := ParseInstrumentManifest(data, "test.yaml")
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func TestParseInstrumentManifest_GoDispatch(t *testing.T) {
	data := []byte(`apiVersion: tako/v1
kind: Instrument
metadata:
  name: core-transformers
  namespace: engine
spec:
  dispatch: inproc
  tune: "true"
  actions:
    llm:
      go_func: "transformers.LLMTransformer"
    jq:
      go_func: "transformers.JQTransformer"
`)
	m, err := ParseInstrumentManifest(data, "test.yaml")
	if err != nil {
		t.Fatalf("ParseInstrumentManifest: %v", err)
	}
	if m.Dispatch != DispatchInproc {
		t.Errorf("Dispatch = %q, want go", m.Dispatch)
	}
	if len(m.Actions) != 2 {
		t.Errorf("Actions count = %d, want 2", len(m.Actions))
	}
	llm := m.Actions["llm"]
	if llm.GoFunc != "transformers.LLMTransformer" {
		t.Errorf("llm.GoFunc = %q", llm.GoFunc)
	}
}

func TestInstrumentManifest_HasAction(t *testing.T) {
	data := []byte(`apiVersion: tako/v1
kind: Instrument
metadata:
  name: test
  namespace: test
spec:
  dispatch: cli
  binary: bash
  tune: "true"
  actions:
    scan:
      command: "scan"
    build:
      command: "build"
`)
	m, err := ParseInstrumentManifest(data, "test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !m.HasAction("scan") {
		t.Error("expected HasAction(scan) = true")
	}
	if m.HasAction("nonexistent") {
		t.Error("expected HasAction(nonexistent) = false")
	}
}

func TestInstrumentManifest_Action(t *testing.T) {
	data := []byte(`apiVersion: tako/v1
kind: Instrument
metadata:
  name: test
  namespace: test
spec:
  dispatch: cli
  binary: bash
  tune: "true"
  actions:
    scan:
      command: "scan --json"
`)
	m, err := ParseInstrumentManifest(data, "test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	a, err := m.Action("scan")
	if err != nil {
		t.Fatal(err)
	}
	if a.Command != "scan --json" {
		t.Errorf("Command = %q", a.Command)
	}
	_, err = m.Action("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent action")
	}
}

func TestValidDispatchModes_Complete(t *testing.T) {
	modes := map[string]bool{
		string(DispatchCLI):       false,
		string(DispatchMCP):       false,
		string(DispatchContainer): false,
		string(DispatchInproc):    false,
	}
	for _, v := range ValidDispatchModes {
		if _, ok := modes[v]; !ok {
			t.Errorf("ValidDispatchModes has unexpected value %q", v)
		}
		modes[v] = true
	}
	for k, found := range modes {
		if !found {
			t.Errorf("ValidDispatchModes missing %q", k)
		}
	}
}

func TestLoadInstrumentManifest_AllFixturesParse(t *testing.T) {
	valid := []string{
		"testdata/component/valid-instrument.yaml",
		"testdata/component/instrument-mcp.yaml",
	}
	for _, path := range valid {
		t.Run(path, func(t *testing.T) {
			if _, err := os.Stat(path); err != nil {
				t.Skipf("fixture not found: %s", path)
			}
			if _, err := LoadInstrumentManifest(path); err != nil {
				t.Errorf("LoadInstrumentManifest(%s): %v", path, err)
			}
		})
	}
}
