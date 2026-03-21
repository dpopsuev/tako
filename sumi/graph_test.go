package sumi

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/view"
)

var updateGolden = flag.Bool("update", false, "update .golden snapshot files")

func testGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file %s not found; run with -update to create it: %v", path, err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch for %s:\n--- got ---\n%s\n--- want ---\n%s", path, got, string(want))
	}
}

func loadTestCircuit(t *testing.T, path string) *framework.CircuitDef {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	def, err := framework.LoadCircuit(data)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	return def
}

func TestRenderGraph_DialecticCircuit(t *testing.T) {
	def := loadTestCircuit(t, "../testdata/defect-dialectic.yaml")
	engine := &view.GridLayout{}
	layout, err := engine.Layout(def)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}

	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})

	if output == "" {
		t.Fatal("RenderGraph returned empty")
	}

	for _, nd := range def.Nodes {
		if !strings.Contains(output, nd.Name) {
			t.Errorf("output missing node %q", nd.Name)
		}
	}

	for _, zoneName := range []string{"thesis", "discovery", "synthesis"} {
		if !strings.Contains(output, zoneName) {
			t.Errorf("output missing zone %q", zoneName)
		}
	}
}

func TestRenderGraph_RCACircuit(t *testing.T) {
	def := loadTestCircuit(t, "../testdata/rca-investigation.yaml")
	engine := &view.GridLayout{}
	layout, err := engine.Layout(def)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}

	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})

	if output == "" {
		t.Fatal("RenderGraph returned empty")
	}

	for _, nd := range def.Nodes {
		if !strings.Contains(output, nd.Name) {
			t.Errorf("output missing node %q", nd.Name)
		}
	}
}

func TestRenderGraph_EmptyCircuit(t *testing.T) {
	def := &framework.CircuitDef{Circuit: "empty"}
	layout := view.CircuitLayout{}
	snap := view.CircuitSnapshot{CircuitName: "empty"}

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	if output != "(empty circuit)" {
		t.Errorf("expected empty message, got %q", output)
	}
}

func TestRenderGraph_NodeStates(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "test",
		Nodes: []framework.NodeDef{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		},
		Edges: []framework.EdgeDef{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
		},
		Start: "a",
		Done:  "c",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()

	store.OnEvent(framework.WalkEvent{Type: framework.EventNodeEnter, Node: "a", Walker: "w1"})
	store.OnEvent(framework.WalkEvent{Type: framework.EventNodeExit, Node: "a"})
	store.OnEvent(framework.WalkEvent{Type: framework.EventNodeEnter, Node: "b", Walker: "w1"})

	snap := store.Snapshot()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	testGolden(t, "node-states", output)
}

func TestRenderGraph_DSBadges(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "badges",
		HandlerType: "transformer",
		Nodes: []framework.NodeDef{
			{Name: "det", Handler: "core.jq"},
			{Name: "stoch", Handler: "core.llm"},
			{Name: "dial", Handler: "core.dialectic"},
		},
		Edges: []framework.EdgeDef{
			{From: "det", To: "stoch"},
			{From: "stoch", To: "dial"},
		},
		Start: "det",
		Done:  "dial",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	testGolden(t, "ds-badges", output)
}

func TestRenderGraph_Breakpoints(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "bp",
		Nodes: []framework.NodeDef{
			{Name: "a"},
			{Name: "b"},
		},
		Edges: []framework.EdgeDef{
			{From: "a", To: "b"},
		},
		Start: "a",
		Done:  "b",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()
	store.SetBreakpoints([]string{"b"})
	snap := store.Snapshot()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	testGolden(t, "breakpoints", output)
}

func TestDSBadge(t *testing.T) {
	tests := []struct {
		transformer string
		want        string
	}{
		{"", ""},
		{"core.jq", "⚙"},
		{"core.file", "⚙"},
		{"core.template", "⚙"},
		{"core.llm", "✦"},
		{"custom.analyzer", "✦"},
		{"core.dialectic", "Δ"},
	}
	for _, tt := range tests {
		got := DSBadge(tt.transformer)
		if got != tt.want {
			t.Errorf("DSBadge(%q) = %q, want %q", tt.transformer, got, tt.want)
		}
	}
}

func TestElementColor_AllElementsCovered(t *testing.T) {
	elements := []string{"fire", "water", "earth", "lightning", "air", "void", "diamond"}
	for _, el := range elements {
		if _, ok := ElementColor[el]; !ok {
			t.Errorf("ElementColor missing %q", el)
		}
	}
}

func TestRenderGraph_ShortcutsVisibleBelow(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "shortcuts",
		Nodes: []framework.NodeDef{
			{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"},
		},
		Edges: []framework.EdgeDef{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
			{From: "c", To: "d"},
			{From: "a", To: "d", Shortcut: true},
		},
		Start: "a",
		Done:  "d",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	testGolden(t, "shortcuts-visible-below", output)
}

func TestRenderGraph_ChannelsSeparated(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "multi-shortcut",
		Nodes: []framework.NodeDef{
			{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}, {Name: "e"},
		},
		Edges: []framework.EdgeDef{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
			{From: "c", To: "d"},
			{From: "d", To: "e"},
			{From: "a", To: "e", Shortcut: true},
			{From: "b", To: "e", Shortcut: true},
		},
		Start: "a",
		Done:  "e",
	}

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	routing := ComputeEdgeRouting(def, layout, def.Done)

	if routing.Channels < 2 {
		t.Errorf("expected at least 2 channels for overlapping shortcuts, got %d", routing.Channels)
	}

	ch0, ch1 := -1, -1
	for _, re := range routing.Below {
		if re.From == "b" && re.To == "e" {
			ch0 = re.Channel
		}
		if re.From == "a" && re.To == "e" {
			ch1 = re.Channel
		}
	}
	if ch0 == ch1 {
		t.Errorf("overlapping shortcuts a→e and b→e should be on different channels, both on %d", ch0)
	}
}

func TestRenderGraph_LoopRoutesToTarget(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "loop-target",
		Nodes: []framework.NodeDef{
			{Name: "a"}, {Name: "b"}, {Name: "c"},
		},
		Edges: []framework.EdgeDef{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
			{From: "c", To: "a", Loop: true},
		},
		Start: "a",
		Done:  "c",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	testGolden(t, "loop-routes-to-target", output)
}

func TestRenderGraph_VirtualDoneFiltered(t *testing.T) {
	def := loadTestCircuit(t, "../testdata/rca-investigation.yaml")
	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})

	// _done is a virtual terminal node; it should NOT be rendered as a box
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "_done") {
			t.Error("virtual _done node should not appear in rendered output")
		}
	}

	// Real nodes must still be present
	for _, name := range []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"} {
		if !strings.Contains(output, name) {
			t.Errorf("real node %q missing from output", name)
		}
	}
}

func TestRenderGraph_RealDoneNotFiltered(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "real-done",
		Nodes: []framework.NodeDef{
			{Name: "a"}, {Name: "b"},
		},
		Edges: []framework.EdgeDef{
			{From: "a", To: "b"},
		},
		Start: "a",
		Done:  "b",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	output := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})

	if !strings.Contains(output, "b") {
		t.Error("real done node 'b' should still be rendered")
	}
}

func TestComputeEdgeRouting_Deduplication(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "dedup",
		Nodes: []framework.NodeDef{
			{Name: "a"}, {Name: "b"},
		},
		Edges: []framework.EdgeDef{
			{From: "a", To: "b"},
			{From: "a", To: "b"},
			{From: "a", To: "b", Shortcut: true},
		},
		Start: "a",
		Done:  "b",
	}

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	routing := ComputeEdgeRouting(def, layout, def.Done)

	total := len(routing.Inline) + len(routing.Below) + len(routing.Loops)
	if total != 1 {
		t.Errorf("expected 1 deduplicated edge, got %d", total)
	}
	if len(routing.Inline) != 1 {
		t.Errorf("expected 1 inline edge, got %d inline, %d below, %d loops",
			len(routing.Inline), len(routing.Below), len(routing.Loops))
	}
	if routing.Inline[0].Shortcut {
		t.Error("merged edge should be non-shortcut (normal edge wins)")
	}
}

// --- Level 2: test helpers ---

func assertInline(t *testing.T, er EdgeRouting, from, to string) {
	t.Helper()
	for _, re := range er.Inline {
		if re.From == from && re.To == to {
			return
		}
	}
	t.Errorf("expected inline edge %s->%s, not found", from, to)
}

func assertBelow(t *testing.T, er EdgeRouting, from, to string) {
	t.Helper()
	for _, re := range er.Below {
		if re.From == from && re.To == to {
			return
		}
	}
	t.Errorf("expected below edge %s->%s, not found", from, to)
}

func assertLoop(t *testing.T, er EdgeRouting, from, to string) {
	t.Helper()
	for _, re := range er.Loops {
		if re.From == from && re.To == to {
			return
		}
	}
	t.Errorf("expected loop edge %s->%s, not found", from, to)
}

func assertChannelCount(t *testing.T, er EdgeRouting, count int) {
	t.Helper()
	if er.Channels != count {
		t.Errorf("channels = %d, want %d", er.Channels, count)
	}
}

func assertCrossRow(t *testing.T, er EdgeRouting, layout view.CircuitLayout, from, to string) {
	t.Helper()
	for _, re := range er.Inline {
		if re.From == from && re.To == to {
			fromGC := layout.Grid[from]
			toGC := layout.Grid[to]
			if fromGC.Row == toGC.Row {
				t.Errorf("edge %s->%s classified as inline but same row (%d); expected cross-row", from, to, fromGC.Row)
			}
			return
		}
	}
	t.Errorf("expected cross-row inline edge %s->%s, not found in inline list", from, to)
}

func assertNoEdge(t *testing.T, er EdgeRouting, from, to string) {
	t.Helper()
	for _, re := range er.Inline {
		if re.From == from && re.To == to {
			t.Errorf("unexpected inline edge %s->%s", from, to)
			return
		}
	}
	for _, re := range er.Below {
		if re.From == from && re.To == to {
			t.Errorf("unexpected below edge %s->%s", from, to)
			return
		}
	}
	for _, re := range er.Loops {
		if re.From == from && re.To == to {
			t.Errorf("unexpected loop edge %s->%s", from, to)
			return
		}
	}
}

// --- Level 2: topology fixture builders (sumi package) ---

func sumiLinearDef() *framework.CircuitDef {
	return &framework.CircuitDef{
		Circuit: "linear",
		Start:   "A",
		Done:    "_done",
		Nodes:   []framework.NodeDef{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}},
		Edges: []framework.EdgeDef{
			{ID: "e1", From: "A", To: "B"},
			{ID: "e2", From: "B", To: "C"},
			{ID: "e3", From: "C", To: "D"},
			{ID: "e4", From: "D", To: "_done"},
		},
	}
}

func sumiShortcutDef() *framework.CircuitDef {
	return &framework.CircuitDef{
		Circuit: "shortcut",
		Start:   "A",
		Done:    "_done",
		Nodes:   []framework.NodeDef{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}},
		Edges: []framework.EdgeDef{
			{ID: "e1", From: "A", To: "B"},
			{ID: "e2", From: "B", To: "C"},
			{ID: "e3", From: "C", To: "D"},
			{ID: "e4", From: "D", To: "_done"},
			{ID: "e5", From: "A", To: "D", Shortcut: true},
		},
	}
}

func sumiLoopDef() *framework.CircuitDef {
	return &framework.CircuitDef{
		Circuit: "loop",
		Start:   "A",
		Done:    "_done",
		Nodes:   []framework.NodeDef{{Name: "A"}, {Name: "B"}, {Name: "C"}},
		Edges: []framework.EdgeDef{
			{ID: "e1", From: "A", To: "B"},
			{ID: "e2", From: "B", To: "C"},
			{ID: "e3", From: "C", To: "_done"},
			{ID: "e4", From: "C", To: "A", Loop: true},
		},
	}
}

func sumiDiamondDef() *framework.CircuitDef {
	return &framework.CircuitDef{
		Circuit: "diamond",
		Start:   "start",
		Done:    "_done",
		Nodes:   []framework.NodeDef{{Name: "start"}, {Name: "a"}, {Name: "b"}, {Name: "join"}},
		Edges: []framework.EdgeDef{
			{ID: "e1", From: "start", To: "a"},
			{ID: "e2", From: "start", To: "b"},
			{ID: "e3", From: "a", To: "join"},
			{ID: "e4", From: "b", To: "join"},
			{ID: "e5", From: "join", To: "_done"},
		},
	}
}

func sumiStaircaseDef() *framework.CircuitDef {
	return &framework.CircuitDef{
		Circuit: "staircase",
		Start:   "start",
		Done:    "_done",
		Nodes: []framework.NodeDef{
			{Name: "start"}, {Name: "a"}, {Name: "b"},
			{Name: "c"}, {Name: "d"}, {Name: "end"},
		},
		Edges: []framework.EdgeDef{
			{ID: "e1", From: "start", To: "a"},
			{ID: "e2", From: "start", To: "b"},
			{ID: "e3", From: "a", To: "c"},
			{ID: "e4", From: "a", To: "d"},
			{ID: "e5", From: "b", To: "end"},
			{ID: "e6", From: "c", To: "end"},
			{ID: "e7", From: "d", To: "end"},
			{ID: "e8", From: "end", To: "_done"},
		},
	}
}

func sumiDialecticDef() *framework.CircuitDef {
	return &framework.CircuitDef{
		Circuit: "dialectic",
		Start:   "indict",
		Done:    "_done",
		Nodes: []framework.NodeDef{
			{Name: "indict"}, {Name: "discover"}, {Name: "defend"},
			{Name: "hearing"}, {Name: "cmrr"}, {Name: "verdict"},
		},
		Edges: []framework.EdgeDef{
			{ID: "e1", From: "indict", To: "discover"},
			{ID: "e2", From: "indict", To: "defend"},
			{ID: "e3", From: "discover", To: "defend"},
			{ID: "e4", From: "defend", To: "verdict"},
			{ID: "e5", From: "defend", To: "hearing"},
			{ID: "e6", From: "hearing", To: "verdict"},
			{ID: "e7", From: "hearing", To: "cmrr"},
			{ID: "e8", From: "cmrr", To: "hearing", Loop: true},
			{ID: "e9", From: "verdict", To: "_done"},
		},
	}
}

func sumiMegaFanoutDef() *framework.CircuitDef {
	nodes := []framework.NodeDef{{Name: "hub"}}
	edges := []framework.EdgeDef{}
	for i := 0; i < 8; i++ {
		name := "t" + string(rune('1'+i))
		nodes = append(nodes, framework.NodeDef{Name: name})
		edges = append(edges, framework.EdgeDef{ID: "fan-" + name, From: "hub", To: name})
	}
	nodes = append(nodes, framework.NodeDef{Name: "merge"})
	for i := 0; i < 8; i++ {
		name := "t" + string(rune('1'+i))
		edges = append(edges, framework.EdgeDef{ID: "join-" + name, From: name, To: "merge"})
	}
	edges = append(edges, framework.EdgeDef{ID: "fin", From: "merge", To: "_done"})
	return &framework.CircuitDef{Circuit: "mega-fanout", Start: "hub", Done: "_done", Nodes: nodes, Edges: edges}
}

func sumiMegaFaninDef() *framework.CircuitDef {
	nodes := []framework.NodeDef{{Name: "start"}}
	edges := []framework.EdgeDef{}
	for i := 0; i < 8; i++ {
		name := "s" + string(rune('1'+i))
		nodes = append(nodes, framework.NodeDef{Name: name})
		edges = append(edges, framework.EdgeDef{ID: "fan-" + name, From: "start", To: name})
	}
	nodes = append(nodes, framework.NodeDef{Name: "merge"})
	for i := 0; i < 8; i++ {
		name := "s" + string(rune('1'+i))
		edges = append(edges, framework.EdgeDef{ID: "join-" + name, From: name, To: "merge"})
	}
	edges = append(edges, framework.EdgeDef{ID: "fin", From: "merge", To: "_done"})
	return &framework.CircuitDef{Circuit: "mega-fanin", Start: "start", Done: "_done", Nodes: nodes, Edges: edges}
}

func sumiDeepCascadeDef() *framework.CircuitDef {
	return &framework.CircuitDef{
		Circuit: "deep-cascade",
		Start:   "root",
		Done:    "_done",
		Nodes: []framework.NodeDef{
			{Name: "root"}, {Name: "a"}, {Name: "b"},
			{Name: "c"}, {Name: "d"}, {Name: "e"}, {Name: "f"},
			{Name: "g"}, {Name: "h"}, {Name: "merge"},
		},
		Edges: []framework.EdgeDef{
			{ID: "e1", From: "root", To: "a"},
			{ID: "e2", From: "root", To: "b"},
			{ID: "e3", From: "a", To: "c"},
			{ID: "e4", From: "a", To: "d"},
			{ID: "e5", From: "b", To: "e"},
			{ID: "e6", From: "b", To: "f"},
			{ID: "e7", From: "c", To: "g"},
			{ID: "e8", From: "c", To: "h"},
			{ID: "e9", From: "d", To: "merge"},
			{ID: "e10", From: "e", To: "merge"},
			{ID: "e11", From: "f", To: "merge"},
			{ID: "e12", From: "g", To: "merge"},
			{ID: "e13", From: "h", To: "merge"},
			{ID: "e14", From: "merge", To: "_done"},
		},
	}
}

func sumiWideGridDef() *framework.CircuitDef {
	return &framework.CircuitDef{
		Circuit: "wide-grid",
		Start:   "start",
		Done:    "_done",
		Nodes: []framework.NodeDef{
			{Name: "start"},
			{Name: "a1"}, {Name: "a2"}, {Name: "a3"},
			{Name: "b1"}, {Name: "b2"}, {Name: "b3"},
			{Name: "c1"}, {Name: "c2"}, {Name: "c3"},
			{Name: "merge"},
		},
		Edges: []framework.EdgeDef{
			{ID: "e1", From: "start", To: "a1"},
			{ID: "e2", From: "start", To: "a2"},
			{ID: "e3", From: "start", To: "a3"},
			{ID: "e4", From: "a1", To: "b1"},
			{ID: "e5", From: "a2", To: "b2"},
			{ID: "e6", From: "a3", To: "b3"},
			{ID: "e7", From: "b1", To: "c1"},
			{ID: "e8", From: "b2", To: "c2"},
			{ID: "e9", From: "b3", To: "c3"},
			{ID: "e10", From: "c1", To: "merge"},
			{ID: "e11", From: "c2", To: "merge"},
			{ID: "e12", From: "c3", To: "merge"},
			{ID: "e13", From: "merge", To: "_done"},
		},
	}
}

// --- Level 2: Structural edge routing tests ---

func TestEdgeRouting_Linear(t *testing.T) {
	def := sumiLinearDef()
	layout, _ := view.GridLayout{}.Layout(def)
	er := ComputeEdgeRouting(def, layout, def.Done)

	assertInline(t, er, "A", "B")
	assertInline(t, er, "B", "C")
	assertInline(t, er, "C", "D")
	assertChannelCount(t, er, 0)
	if len(er.Below) != 0 {
		t.Errorf("linear should have 0 below edges, got %d", len(er.Below))
	}
	if len(er.Loops) != 0 {
		t.Errorf("linear should have 0 loops, got %d", len(er.Loops))
	}
}

func TestEdgeRouting_Shortcut(t *testing.T) {
	def := sumiShortcutDef()
	layout, _ := view.GridLayout{}.Layout(def)
	er := ComputeEdgeRouting(def, layout, def.Done)

	assertInline(t, er, "A", "B")
	assertInline(t, er, "B", "C")
	assertInline(t, er, "C", "D")
	assertBelow(t, er, "A", "D")
	assertChannelCount(t, er, 1)
}

func TestEdgeRouting_Loop(t *testing.T) {
	def := sumiLoopDef()
	layout, _ := view.GridLayout{}.Layout(def)
	er := ComputeEdgeRouting(def, layout, def.Done)

	assertInline(t, er, "A", "B")
	assertInline(t, er, "B", "C")
	assertLoop(t, er, "C", "A")
	assertChannelCount(t, er, 0)
}

func TestEdgeRouting_Diamond(t *testing.T) {
	def := sumiDiamondDef()
	layout, _ := view.GridLayout{}.Layout(def)
	er := ComputeEdgeRouting(def, layout, def.Done)

	assertInline(t, er, "start", "a")
	assertInline(t, er, "start", "b")
	assertInline(t, er, "a", "join")
	assertInline(t, er, "b", "join")

	assertCrossRow(t, er, layout, "start", "b")
	assertCrossRow(t, er, layout, "b", "join")
	assertChannelCount(t, er, 0)
}

func TestEdgeRouting_Staircase(t *testing.T) {
	def := sumiStaircaseDef()
	layout, _ := view.GridLayout{}.Layout(def)
	er := ComputeEdgeRouting(def, layout, def.Done)

	assertInline(t, er, "start", "a")
	assertInline(t, er, "start", "b")
	assertInline(t, er, "a", "c")
	assertInline(t, er, "a", "d")
	assertChannelCount(t, er, 0)
}

func TestEdgeRouting_Dialectic(t *testing.T) {
	def := sumiDialecticDef()
	layout, _ := view.GridLayout{}.Layout(def)
	er := ComputeEdgeRouting(def, layout, def.Done)

	assertInline(t, er, "indict", "discover")
	assertLoop(t, er, "cmrr", "hearing")
	if len(er.Loops) != 1 {
		t.Errorf("dialectic should have 1 loop, got %d", len(er.Loops))
	}
}

func TestEdgeRouting_MegaFanout(t *testing.T) {
	def := sumiMegaFanoutDef()
	layout, _ := view.GridLayout{}.Layout(def)
	er := ComputeEdgeRouting(def, layout, def.Done)

	for i := 0; i < 8; i++ {
		name := "t" + string(rune('1'+i))
		assertInline(t, er, "hub", name)
		assertInline(t, er, name, "merge")
	}
	assertChannelCount(t, er, 0)

	crossRowCount := 0
	for _, re := range er.Inline {
		fromGC := layout.Grid[re.From]
		toGC := layout.Grid[re.To]
		if fromGC.Row != toGC.Row {
			crossRowCount++
		}
	}
	if crossRowCount < 14 {
		t.Errorf("expected at least 14 cross-row edges (7 fan-out + 7 fan-in), got %d", crossRowCount)
	}
}

func TestEdgeRouting_MegaFanin(t *testing.T) {
	def := sumiMegaFaninDef()
	layout, _ := view.GridLayout{}.Layout(def)
	er := ComputeEdgeRouting(def, layout, def.Done)

	for i := 0; i < 8; i++ {
		name := "s" + string(rune('1'+i))
		assertInline(t, er, "start", name)
		assertInline(t, er, name, "merge")
	}
	assertChannelCount(t, er, 0)
}

func TestEdgeRouting_DeepCascade(t *testing.T) {
	def := sumiDeepCascadeDef()
	layout, _ := view.GridLayout{}.Layout(def)
	er := ComputeEdgeRouting(def, layout, def.Done)

	assertInline(t, er, "root", "a")
	assertInline(t, er, "root", "b")
	assertInline(t, er, "a", "c")
	assertInline(t, er, "c", "g")

	totalEdges := len(er.Inline) + len(er.Below) + len(er.Loops)
	if totalEdges < 13 {
		t.Errorf("deep-cascade should have >= 13 routed edges, got %d", totalEdges)
	}
	if len(er.Loops) != 0 {
		t.Errorf("deep-cascade should have 0 loops, got %d", len(er.Loops))
	}
}

func TestEdgeRouting_WideGrid(t *testing.T) {
	def := sumiWideGridDef()
	layout, _ := view.GridLayout{}.Layout(def)
	er := ComputeEdgeRouting(def, layout, def.Done)

	assertInline(t, er, "start", "a1")
	assertInline(t, er, "start", "a2")
	assertInline(t, er, "start", "a3")
	assertInline(t, er, "a1", "b1")
	assertInline(t, er, "b1", "c1")
	assertInline(t, er, "c1", "merge")
	assertChannelCount(t, er, 0)
	if len(er.Below) != 0 {
		t.Errorf("wide-grid should have 0 below edges, got %d", len(er.Below))
	}
}

// --- Level 3: Abstract rendering tests ---

func TestRenderAbstract_Linear(t *testing.T) {
	def := sumiLinearDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	want := "*-*-*-*\n"
	if got != want {
		t.Errorf("RenderAbstract linear:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderAbstract_Shortcut(t *testing.T) {
	def := sumiShortcutDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d:\n%s", len(lines), got)
	}
	if !strings.Contains(lines[0], "*-*-*-*") {
		t.Errorf("line 0 should contain *-*-*-*, got: %s", lines[0])
	}
	if !strings.Contains(got, "+") {
		t.Errorf("expected below-path markers (+), got:\n%s", got)
	}
}

func TestRenderAbstract_Loop(t *testing.T) {
	def := sumiLoopDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d:\n%s", len(lines), got)
	}
	if !strings.Contains(lines[0], "*-*-*") {
		t.Errorf("line 0 should contain *-*-*, got: %s", lines[0])
	}
	found := false
	for _, line := range lines[1:] {
		if strings.Contains(line, "<") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected loop marker (<), got:\n%s", got)
	}
}

func TestRenderAbstract_Diamond(t *testing.T) {
	def := sumiDiamondDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	if !strings.Contains(got, "*") {
		t.Fatalf("output should contain nodes:\n%s", got)
	}
	nodeCount := strings.Count(got, "*")
	if nodeCount != 4 {
		t.Errorf("expected 4 nodes (*), got %d:\n%s", nodeCount, got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("diamond should produce at least 3 lines (2 node rows + gap), got %d:\n%s", len(lines), got)
	}
}

func TestRenderAbstract_MegaFanout(t *testing.T) {
	def := sumiMegaFanoutDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	nodeCount := strings.Count(got, "*")
	if nodeCount != 10 {
		t.Errorf("expected 10 nodes, got %d:\n%s", nodeCount, got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 15 {
		t.Errorf("mega-fanout should produce at least 15 lines (8 node rows + 7 gaps), got %d:\n%s", len(lines), got)
	}
}

func TestRenderAbstract_WideGrid(t *testing.T) {
	def := sumiWideGridDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	nodeCount := strings.Count(got, "*")
	if nodeCount != 11 {
		t.Errorf("expected 11 nodes, got %d:\n%s", nodeCount, got)
	}
}

func TestRenderAbstract_Staircase(t *testing.T) {
	def := sumiStaircaseDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	nodeCount := strings.Count(got, "*")
	if nodeCount != 6 {
		t.Errorf("expected 6 nodes, got %d:\n%s", nodeCount, got)
	}
}

func TestRenderAbstract_Dialectic(t *testing.T) {
	def := sumiDialecticDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	nodeCount := strings.Count(got, "*")
	if nodeCount != 6 {
		t.Errorf("expected 6 nodes, got %d:\n%s", nodeCount, got)
	}
	if !strings.Contains(got, "<") {
		t.Errorf("dialectic should have a loop marker (<):\n%s", got)
	}
}

func TestRenderAbstract_MegaFanin(t *testing.T) {
	def := sumiMegaFaninDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	nodeCount := strings.Count(got, "*")
	if nodeCount != 10 {
		t.Errorf("expected 10 nodes, got %d:\n%s", nodeCount, got)
	}
}

func TestRenderAbstract_DeepCascade(t *testing.T) {
	def := sumiDeepCascadeDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	nodeCount := strings.Count(got, "*")
	if nodeCount != 10 {
		t.Errorf("expected 10 nodes, got %d:\n%s", nodeCount, got)
	}
}

func TestRenderAbstract_CompositeStub(t *testing.T) {
	def := &framework.CircuitDef{
		Circuit: "composite-stub",
		Start:   "A",
		Done:    "_done",
		Nodes: []framework.NodeDef{
			{Name: "A"},
			{Name: "B", Meta: map[string]any{"composite": true}},
			{Name: "C"},
		},
		Edges: []framework.EdgeDef{
			{ID: "e1", From: "A", To: "B"},
			{ID: "e2", From: "B", To: "C"},
			{ID: "e3", From: "C", To: "_done"},
		},
	}
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	if !strings.Contains(got, "@") {
		t.Errorf("composite node B should render as @, got:\n%s", got)
	}
	regularCount := strings.Count(got, "*")
	if regularCount != 2 {
		t.Errorf("expected 2 regular nodes (*), got %d:\n%s", regularCount, got)
	}
	compositeCount := strings.Count(got, "@")
	if compositeCount != 1 {
		t.Errorf("expected 1 composite node (@), got %d:\n%s", compositeCount, got)
	}
}

func TestRenderAbstract_Empty(t *testing.T) {
	def := &framework.CircuitDef{Circuit: "empty"}
	layout := view.CircuitLayout{}
	got := RenderAbstract(def, layout)
	if got != "(empty)" {
		t.Errorf("expected (empty), got: %q", got)
	}
}

// --- Level 4: Golden snapshot tests ---

func TestGolden_RCA(t *testing.T) {
	def := loadTestCircuit(t, "../testdata/rca-investigation.yaml")
	layout, err := view.GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()
	got := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	testGolden(t, "rca", got)
}

func TestGolden_Dialectic(t *testing.T) {
	def := loadTestCircuit(t, "../testdata/defect-dialectic.yaml")
	layout, err := view.GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()
	got := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	testGolden(t, "dialectic", got)
}

func TestGolden_TeamDelegation(t *testing.T) {
	def := loadTestCircuit(t, "../testdata/scenarios/team-delegation.yaml")
	layout, err := view.GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()
	got := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	testGolden(t, "team-delegation", got)
}

func TestGolden_MegaFanout(t *testing.T) {
	def := sumiMegaFanoutDef()
	layout, _ := view.GridLayout{}.Layout(def)
	store := view.NewCircuitStore(def)
	defer store.Close()
	snap := store.Snapshot()
	got := RenderGraph(def, layout, snap, RenderOpts{NoColor: true})
	testGolden(t, "mega-fanout", got)
}

func TestGolden_AbstractLinear(t *testing.T) {
	def := sumiLinearDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	testGolden(t, "abstract-linear", got)
}

func TestGolden_AbstractDiamond(t *testing.T) {
	def := sumiDiamondDef()
	layout, _ := view.GridLayout{}.Layout(def)
	got := RenderAbstract(def, layout)
	testGolden(t, "abstract-diamond", got)
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		current, total, width int
		wantFilled            int
	}{
		{0, 10, 10, 0},
		{5, 10, 10, 5},
		{10, 10, 10, 10},
		{0, 0, 10, 0},
	}
	for _, tt := range tests {
		bar := progressBar(tt.current, tt.total, tt.width)
		if tt.total == 0 {
			if !strings.Contains(bar, "░") {
				t.Errorf("expected empty bar for total=0")
			}
			continue
		}
		filled := strings.Count(bar, "█")
		if filled != tt.wantFilled {
			t.Errorf("progressBar(%d,%d,%d) filled=%d, want %d", tt.current, tt.total, tt.width, filled, tt.wantFilled)
		}
	}
}
