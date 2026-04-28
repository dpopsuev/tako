package autodoc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dpopsuev/tako/circuit"
)

func testManifest() *Manifest {
	return &Manifest{
		Name:        "testproject",
		Description: "A test project for autodoc",
		Version:     "1.0",
	}
}

func simpleCircuit() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit:     "simple",
		Description: "A simple test circuit",
		Nodes: []circuit.NodeDef{
			{Name: "start-node", Approach: "rapid"},
			{Name: "end-node", Approach: "methodical"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "proceed", From: "start-node", To: "end-node"},
			{ID: "E2", Name: "done", From: "end-node", To: "_done"},
		},
		Start: "start-node",
		Done:  "_done",
	}
}

func zonedCircuit() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit:     "zoned",
		Description: "A circuit with zones",
		Zones: map[string]circuit.ZoneDef{
			"discovery": {Nodes: []circuit.NodeName{"scan"}, Approach: "methodical"},
			"analysis":  {Nodes: []circuit.NodeName{"classify", "assess"}, Approach: "rapid"},
			"output":    {Nodes: []circuit.NodeName{"report"}, Approach: "holistic"},
		},
		Nodes: []circuit.NodeDef{
			{Name: "scan", Approach: "methodical", Instrument: "node", Action: "scan"},
			{Name: "classify", Approach: "rapid", Instrument: "node", Action: "classify"},
			{Name: "assess", Approach: "rigorous", Instrument: "node", Action: "assess"},
			{Name: "report", Approach: "holistic", Instrument: "node", Action: "report"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "V1", Name: "findings-ready", From: "scan", To: "classify"},
			{ID: "V2", Name: "no-findings", From: "scan", To: "report", Shortcut: true},
			{ID: "V3", Name: "classified", From: "classify", To: "assess"},
			{ID: "V4", Name: "assessment-complete", From: "assess", To: "report"},
			{ID: "V5", Name: "rescan", From: "assess", To: "scan", Loop: true},
			{ID: "V6", Name: "done", From: "report", To: "_done"},
		},
		Start: "scan",
		Done:  "_done",
	}
}

func dsCircuit() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit:     "mixed-ds",
		Description: "Circuit with mixed deterministic/stochastic nodes",
		Nodes: []circuit.NodeDef{
			{Name: "filter", Instrument: "transformer", Action: "core.jq"},
			{Name: "analyze", Instrument: "transformer", Action: "core.llm"},
			{Name: "format", Instrument: "transformer", Action: "core.jq"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", Name: "to-analyze", From: "filter", To: "analyze"},
			{ID: "E2", Name: "to-format", From: "analyze", To: "format"},
			{ID: "E3", Name: "done", From: "format", To: "_done"},
		},
		Start: "filter",
		Done:  "_done",
	}
}

func contextFilterCircuit() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit:     "ctx-flow",
		Description: "Circuit with context filters",
		Zones: map[string]circuit.ZoneDef{
			"intake": {
				Nodes: []circuit.NodeName{"ingest"},
				ContextFilter: &circuit.ContextFilterDef{
					Pass: []string{"case_id", "launch_id"},
				},
			},
			"analysis": {
				Nodes: []circuit.NodeName{"analyze"},
				ContextFilter: &circuit.ContextFilterDef{
					Block: []string{"raw_logs"},
				},
			},
		},
		Nodes: []circuit.NodeDef{
			{Name: "ingest"},
			{Name: "analyze"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "E1", From: "ingest", To: "analyze"},
		},
		Start: "ingest",
		Done:  "_done",
	}
}

// --- Manifest tests ---

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tako.yaml")
	os.WriteFile(path, []byte("name: myproject\ndescription: My project\nversion: \"1.0\"\n"), 0o644)

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "myproject" {
		t.Errorf("name = %q, want myproject", m.Name)
	}
	if m.Description != "My project" {
		t.Errorf("description = %q, want 'My project'", m.Description)
	}
}

func TestLoadManifest_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tako.yaml")
	os.WriteFile(path, []byte("description: no name\n"), 0o644)

	_, err := LoadManifest(path)
	if err == nil {
		t.Error("expected error for manifest without name")
	}
}

func TestDiscoverCircuits(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "internal", "circuits"), 0o755)
	os.WriteFile(filepath.Join(dir, "internal", "circuits", "a.yaml"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "internal", "circuits", "b.yml"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "internal", "circuits", "readme.txt"), []byte(""), 0o644)

	found, err := DiscoverCircuits(dir)
	if err != nil {
		t.Fatalf("DiscoverCircuits: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("found %d circuits, want 2", len(found))
	}
}

func TestDiscoverCircuits_NoDir(t *testing.T) {
	found, err := DiscoverCircuits("/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("found %d circuits, want 0", len(found))
	}
}

// --- Mermaid tests ---

func TestRenderMermaid_Simple(t *testing.T) {
	out := RenderMermaid(simpleCircuit(), nil)
	if !strings.Contains(out, "graph LR") {
		t.Error("missing graph LR header")
	}
	if !strings.Contains(out, "start_node") {
		t.Error("missing sanitized node name start_node")
	}
	if !strings.Contains(out, "proceed") {
		t.Error("missing edge label 'proceed'")
	}
}

func TestRenderMermaid_Zones(t *testing.T) {
	out := RenderMermaid(zonedCircuit(), nil)
	if !strings.Contains(out, "subgraph") {
		t.Error("missing subgraph for zones")
	}
	if !strings.Contains(out, "Discovery") || !strings.Contains(out, "Analysis") || !strings.Contains(out, "Output") {
		t.Errorf("missing zone labels: %s", out)
	}
}

func TestRenderMermaid_Shortcuts(t *testing.T) {
	out := RenderMermaid(zonedCircuit(), nil)
	if !strings.Contains(out, "-.->") {
		t.Error("expected dotted arrow for shortcut edge")
	}
}

func TestRenderMermaid_Loops(t *testing.T) {
	out := RenderMermaid(zonedCircuit(), nil)
	if !strings.Contains(out, "==>") {
		t.Error("expected thick arrow for loop edge")
	}
}

func TestRenderMermaid_EdgeLabels(t *testing.T) {
	out := RenderMermaid(zonedCircuit(), nil)
	if !strings.Contains(out, "findings-ready") {
		t.Error("missing edge name in label")
	}
}

// --- D/S Boundary tests ---

func TestRenderDSBoundary(t *testing.T) {
	out := RenderDSBoundary(dsCircuit(), nil)
	if !strings.Contains(out, "([") {
		t.Error("expected stadium shape for stochastic node")
	}
	if !strings.Contains(out, "[\"filter\"]") {
		t.Error("expected rectangle shape for deterministic node")
	}
	if !strings.Contains(out, "[D→S]") {
		t.Error("expected D/S boundary label on crossing edge")
	}
}

func TestRenderDSBoundary_NoBoundary(t *testing.T) {
	def := &circuit.CircuitDef{
		Nodes: []circuit.NodeDef{
			{Name: "a", Instrument: "transformer", Action: "core.jq"},
			{Name: "b", Instrument: "transformer", Action: "core.jq"},
		},
		Edges: []circuit.EdgeDef{{ID: "E1", From: "a", To: "b"}},
	}
	out := RenderDSBoundary(def, nil)
	if strings.Contains(out, "[D→S]") {
		t.Error("unexpected D/S boundary label in all-deterministic circuit")
	}
}

func TestRenderDSBoundary_WithZones(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "zoned-ds",
		Zones: map[string]circuit.ZoneDef{
			"prep":    {Nodes: []circuit.NodeName{"filter"}},
			"analyze": {Nodes: []circuit.NodeName{"llm-node"}},
		},
		Nodes: []circuit.NodeDef{
			{Name: "filter", Instrument: "transformer", Action: "core.jq"},
			{Name: "llm-node", Instrument: "transformer", Action: "core.llm"},
		},
		Edges: []circuit.EdgeDef{{ID: "E1", From: "filter", To: "llm-node"}},
	}
	out := RenderDSBoundary(def, nil)
	if !strings.Contains(out, "deterministic") {
		t.Error("expected zone DS annotation for prep zone")
	}
	if !strings.Contains(out, "stochastic") {
		t.Error("expected zone DS annotation for analyze zone")
	}
}

// --- Table tests ---

func TestRenderNodeTable(t *testing.T) {
	out := RenderNodeTable(zonedCircuit(), nil)
	if !strings.Contains(out, "| scan |") {
		t.Error("missing scan node row")
	}
	if !strings.Contains(out, "discovery") {
		t.Error("missing zone column value")
	}
	if !strings.Contains(out, "| Node |") {
		t.Error("missing table header")
	}
	if !strings.Contains(out, "| Description |") {
		t.Error("missing Description column header")
	}
}

func TestRenderNodeTable_WithDescription(t *testing.T) {
	def := &circuit.CircuitDef{
		Nodes: []circuit.NodeDef{
			{Name: "a", Description: "First node"},
			{Name: "b"},
		},
		Edges: []circuit.EdgeDef{{ID: "E1", From: "a", To: "b"}},
	}
	out := RenderNodeTable(def, nil)
	if !strings.Contains(out, "First node") {
		t.Error("missing description in table")
	}
	if !strings.Contains(out, "| b | - |") {
		t.Error("missing dash for empty description")
	}
}

func TestRenderNodeTable_DSColumn(t *testing.T) {
	out := RenderNodeTable(dsCircuit(), nil)
	if !strings.Contains(out, "| D |") {
		t.Error("missing D tag for deterministic node")
	}
	if !strings.Contains(out, "| S |") {
		t.Error("missing S tag for stochastic node")
	}
}

func TestRenderSummary(t *testing.T) {
	out := RenderSummary(zonedCircuit(), nil)
	if !strings.Contains(out, "**Nodes:** 4") {
		t.Error("missing node count")
	}
	if !strings.Contains(out, "**Edges:** 6") {
		t.Error("missing edge count")
	}
	if !strings.Contains(out, "1 shortcut") {
		t.Error("missing shortcut count")
	}
	if !strings.Contains(out, "1 loop") {
		t.Error("missing loop count")
	}
	if !strings.Contains(out, "**Zones:** 3") {
		t.Error("missing zone count")
	}
}

func TestRenderSummary_DS(t *testing.T) {
	out := RenderSummary(dsCircuit(), nil)
	if !strings.Contains(out, "2 deterministic") {
		t.Error("missing deterministic count")
	}
	if !strings.Contains(out, "1 stochastic") {
		t.Error("missing stochastic count")
	}
}

// --- Context Flow tests ---

func TestRenderContextFlow(t *testing.T) {
	out := RenderContextFlow(contextFilterCircuit())
	if !strings.Contains(out, "graph TD") {
		t.Error("missing graph TD header")
	}
	if !strings.Contains(out, "pass: case_id, launch_id") {
		t.Error("missing pass filter in label")
	}
	if !strings.Contains(out, "block: raw_logs") {
		t.Error("missing block filter in label")
	}
	if !strings.Contains(out, "intake -->") {
		t.Error("missing cross-zone edge")
	}
}

func TestRenderContextFlow_NoZones(t *testing.T) {
	out := RenderContextFlow(simpleCircuit())
	if !strings.Contains(out, "No zones defined") {
		t.Error("expected no-zones message")
	}
}

// --- Scaffold tests ---

func TestScaffold(t *testing.T) {
	dir := t.TempDir()

	cfg := ScaffoldConfig{
		ProjectRoot: dir,
		Manifest:    testManifest(),
		Circuits:    []*circuit.CircuitDef{simpleCircuit(), zonedCircuit()},
	}

	if err := Scaffold(&cfg); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("missing README.md: %v", err)
	}
	if !strings.Contains(string(readme), "Testproject") {
		t.Error("README missing project name")
	}
	if !strings.Contains(string(readme), "2 circuit(s)") {
		t.Error("README missing circuit count")
	}
	if !strings.Contains(string(readme), markerBegin) {
		t.Error("README missing autodoc begin marker")
	}

	circuitPage, err := os.ReadFile(filepath.Join(dir, "docs", "circuits", "simple.md"))
	if err != nil {
		t.Fatalf("missing circuit page: %v", err)
	}
	if !strings.Contains(string(circuitPage), "graph LR") {
		t.Error("circuit page missing Mermaid diagram")
	}
	if !strings.Contains(string(circuitPage), "| Node |") {
		t.Error("circuit page missing node table")
	}

	indexPage, err := os.ReadFile(filepath.Join(dir, "docs", "circuits", "index.md"))
	if err != nil {
		t.Fatalf("missing circuit index: %v", err)
	}
	if !strings.Contains(string(indexPage), "simple") {
		t.Error("circuit index missing simple circuit")
	}

	for _, stub := range []string{
		"docs/concepts/architecture.md",
		"docs/getting-started/installation.md",
		"docs/contributing/development.md",
		"docs/reference/cli.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, stub)); err != nil {
			t.Errorf("missing stub: %s", stub)
		}
	}
}

func TestScaffold_Idempotent(t *testing.T) {
	dir := t.TempDir()

	cfg := ScaffoldConfig{
		ProjectRoot: dir,
		Manifest:    testManifest(),
		Circuits:    []*circuit.CircuitDef{simpleCircuit()},
	}

	if err := Scaffold(&cfg); err != nil {
		t.Fatalf("first Scaffold: %v", err)
	}

	readmePath := filepath.Join(dir, "README.md")
	original, _ := os.ReadFile(readmePath)

	handwritten := strings.Replace(string(original),
		"## Prerequisites", "## My Custom Section\n\nCustom content.\n\n## Prerequisites", 1)
	os.WriteFile(readmePath, []byte(handwritten), 0o644)

	cfg.Circuits = append(cfg.Circuits, zonedCircuit())
	if err := Scaffold(&cfg); err != nil {
		t.Fatalf("second Scaffold: %v", err)
	}

	updated, _ := os.ReadFile(readmePath)
	if !strings.Contains(string(updated), "My Custom Section") {
		t.Error("idempotent update lost hand-written content")
	}
	if !strings.Contains(string(updated), "2 circuit(s)") {
		t.Error("idempotent update didn't refresh auto-generated section")
	}
}

func TestScaffold_WithScorecards(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "internal", "scorecards"), 0o755)
	scPath := filepath.Join(dir, "internal", "scorecards", "test.yaml")
	os.WriteFile(scPath, []byte("scorecard: test\n"), 0o644)

	cfg := ScaffoldConfig{
		ProjectRoot: dir,
		Manifest:    testManifest(),
		Circuits:    []*circuit.CircuitDef{simpleCircuit()},
		Scorecards:  []string{scPath},
	}

	if err := Scaffold(&cfg); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	sc, err := os.ReadFile(filepath.Join(dir, "docs", "reference", "scorecards.md"))
	if err != nil {
		t.Fatalf("missing scorecards.md: %v", err)
	}
	if !strings.Contains(string(sc), "test") {
		t.Error("scorecard reference missing test entry")
	}
}

// --- Enrichment tests ---

type mockEnricher struct {
	called int
	lastIn string
}

func (m *mockEnricher) Enrich(_ context.Context, prompt string) (string, error) {
	m.called++
	m.lastIn = prompt
	return "# Enriched\n\n" + markerBegin + "\noriginal content\n" + markerEnd + "\n", nil
}

func TestEnrichDocs(t *testing.T) {
	dir := t.TempDir()
	circuitsDir := filepath.Join(dir, "circuits")
	os.MkdirAll(circuitsDir, 0o755)

	content := "# Circuit\n\n" + markerBegin + "\nauto content\n" + markerEnd + "\n"
	os.WriteFile(filepath.Join(circuitsDir, "test.md"), []byte(content), 0o644)

	enricher := &mockEnricher{}
	err := EnrichDocs(context.Background(), EnrichConfig{
		OutputDir: dir,
		Enricher:  enricher,
		Manifest:  testManifest(),
	})
	if err != nil {
		t.Fatalf("EnrichDocs: %v", err)
	}
	if enricher.called != 1 {
		t.Errorf("enricher called %d times, want 1", enricher.called)
	}

	enriched, _ := os.ReadFile(filepath.Join(circuitsDir, "test.md"))
	if !strings.Contains(string(enriched), "Enriched") {
		t.Error("enriched content not written")
	}
}
