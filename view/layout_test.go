package view

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

// --- Level 1 test helpers ---

func assertCol(t *testing.T, layout CircuitLayout, node string, col int) { //nolint:unparam // test flexibility
	t.Helper()
	gc, ok := layout.Grid[node]
	if !ok {
		t.Fatalf("node %q not in grid", node)
	}
	if gc.Col != col {
		t.Errorf("node %q col = %d, want %d", node, gc.Col, col)
	}
}

func assertBefore(t *testing.T, layout CircuitLayout, a, b string) {
	t.Helper()
	gcA, okA := layout.Grid[a]
	gcB, okB := layout.Grid[b]
	if !okA {
		t.Fatalf("node %q not in grid", a)
	}
	if !okB {
		t.Fatalf("node %q not in grid", b)
	}
	if gcA.Col >= gcB.Col {
		t.Errorf("expected col(%s)=%d < col(%s)=%d", a, gcA.Col, b, gcB.Col)
	}
}

func assertSameCol(t *testing.T, layout CircuitLayout, a, b string) {
	t.Helper()
	gcA, okA := layout.Grid[a]
	gcB, okB := layout.Grid[b]
	if !okA {
		t.Fatalf("node %q not in grid", a)
	}
	if !okB {
		t.Fatalf("node %q not in grid", b)
	}
	if gcA.Col != gcB.Col {
		t.Errorf("expected col(%s)=%d == col(%s)=%d", a, gcA.Col, b, gcB.Col)
	}
}

func assertDiffRow(t *testing.T, layout CircuitLayout, a, b string) {
	t.Helper()
	gcA, okA := layout.Grid[a]
	gcB, okB := layout.Grid[b]
	if !okA {
		t.Fatalf("node %q not in grid", a)
	}
	if !okB {
		t.Fatalf("node %q not in grid", b)
	}
	if gcA.Row == gcB.Row {
		t.Errorf("expected row(%s) != row(%s), both = %d", a, b, gcA.Row)
	}
}

func assertColSpan(t *testing.T, layout CircuitLayout, expected int) {
	t.Helper()
	if len(layout.Grid) == 0 {
		t.Fatal("empty grid")
	}
	minC, maxC := 1<<30, 0
	for _, gc := range layout.Grid {
		if gc.Col < minC {
			minC = gc.Col
		}
		if gc.Col > maxC {
			maxC = gc.Col
		}
	}
	span := maxC - minC + 1
	if span != expected {
		t.Errorf("col span = %d, want %d", span, expected)
	}
}

func assertRowSpan(t *testing.T, layout CircuitLayout, expected int) {
	t.Helper()
	if len(layout.Grid) == 0 {
		t.Fatal("empty grid")
	}
	minR, maxR := 1<<30, 0
	for _, gc := range layout.Grid {
		if gc.Row < minR {
			minR = gc.Row
		}
		if gc.Row > maxR {
			maxR = gc.Row
		}
	}
	span := maxR - minR + 1
	if span != expected {
		t.Errorf("row span = %d, want %d", span, expected)
	}
}

func assertMinRowSpan(t *testing.T, layout CircuitLayout, minSpan int) {
	t.Helper()
	if len(layout.Grid) == 0 {
		t.Fatal("empty grid")
	}
	minR, maxR := 1<<30, 0
	for _, gc := range layout.Grid {
		if gc.Row < minR {
			minR = gc.Row
		}
		if gc.Row > maxR {
			maxR = gc.Row
		}
	}
	span := maxR - minR + 1
	if span < minSpan {
		t.Errorf("row span = %d, want >= %d", span, minSpan)
	}
}

// --- Topology fixture builders ---

func linearDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "linear",
		Start:   circuit.NodeName("A"),
		Done:    circuit.NodeName("_done"),
		Nodes: []circuit.NodeDef{
			{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "A", To: "B"},
			{ID: "e2", From: "B", To: "C"},
			{ID: "e3", From: "C", To: "D"},
			{ID: "e4", From: "D", To: circuit.NodeName("_done")},
		},
	}
}

func shortcutDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "shortcut",
		Start:   circuit.NodeName("A"),
		Done:    circuit.NodeName("_done"),
		Nodes: []circuit.NodeDef{
			{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "A", To: "B"},
			{ID: "e2", From: "B", To: "C"},
			{ID: "e3", From: "C", To: "D"},
			{ID: "e4", From: "D", To: circuit.NodeName("_done")},
			{ID: "e5", From: "A", To: "D", Shortcut: true},
		},
	}
}

func loopDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "loop",
		Start:   circuit.NodeName("A"),
		Done:    circuit.NodeName("_done"),
		Nodes: []circuit.NodeDef{
			{Name: "A"}, {Name: "B"}, {Name: "C"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "A", To: "B"},
			{ID: "e2", From: "B", To: "C"},
			{ID: "e3", From: "C", To: circuit.NodeName("_done")},
			{ID: "e4", From: "C", To: "A", Loop: true},
		},
	}
}

func diamondDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "diamond",
		Start:   circuit.NodeName("start"),
		Done:    circuit.NodeName("_done"),
		Nodes: []circuit.NodeDef{
			{Name: circuit.NodeName("start")}, {Name: "a"}, {Name: "b"}, {Name: "join"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "start", To: "a"},
			{ID: "e2", From: "start", To: "b"},
			{ID: "e3", From: "a", To: "join"},
			{ID: "e4", From: "b", To: "join"},
			{ID: "e5", From: "join", To: circuit.NodeName("_done")},
		},
	}
}

func staircaseDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "staircase",
		Start:   circuit.NodeName("start"),
		Done:    circuit.NodeName("_done"),
		Nodes: []circuit.NodeDef{
			{Name: circuit.NodeName("start")},
			{Name: "a"}, {Name: "b"},
			{Name: "c"}, {Name: "d"},
			{Name: "end"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "start", To: "a"},
			{ID: "e2", From: "start", To: "b"},
			{ID: "e3", From: "a", To: "c"},
			{ID: "e4", From: "a", To: "d"},
			{ID: "e5", From: "b", To: "end"},
			{ID: "e6", From: "c", To: "end"},
			{ID: "e7", From: "d", To: "end"},
			{ID: "e8", From: "end", To: circuit.NodeName("_done")},
		},
	}
}

func dialecticDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "dialectic",
		Start:   "indict",
		Done:    circuit.NodeName("_done"),
		Nodes: []circuit.NodeDef{
			{Name: "indict"}, {Name: "discover"}, {Name: "defend"},
			{Name: "hearing"}, {Name: "cmrr"}, {Name: "verdict"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "indict", To: "discover"},
			{ID: "e2", From: "indict", To: "defend"},
			{ID: "e3", From: "discover", To: "defend"},
			{ID: "e4", From: "defend", To: "verdict"},
			{ID: "e5", From: "defend", To: "hearing"},
			{ID: "e6", From: "hearing", To: "verdict"},
			{ID: "e7", From: "hearing", To: "cmrr"},
			{ID: "e8", From: "cmrr", To: "hearing", Loop: true},
			{ID: "e9", From: "verdict", To: circuit.NodeName("_done")},
		},
	}
}

func megaFanoutDef() *circuit.CircuitDef {
	nodes := []circuit.NodeDef{{Name: "hub"}}
	edges := []circuit.EdgeDef{}
	for i := 0; i < 8; i++ {
		name := "t" + string(rune('1'+i))
		nodes = append(nodes, circuit.NodeDef{Name: circuit.NodeName(name)})
		edges = append(edges, circuit.EdgeDef{
			ID: "fan-" + name, From: circuit.NodeName("hub"), To: circuit.NodeName(name),
		})
	}
	nodes = append(nodes, circuit.NodeDef{Name: circuit.NodeName("merge")})
	for i := 0; i < 8; i++ {
		name := "t" + string(rune('1'+i))
		edges = append(edges, circuit.EdgeDef{
			ID: "join-" + name, From: circuit.NodeName(name), To: circuit.NodeName("merge"),
		})
	}
	edges = append(edges, circuit.EdgeDef{ID: "fin", From: circuit.NodeName("merge"), To: circuit.NodeName("_done")})
	return &circuit.CircuitDef{
		Circuit: "mega-fanout",
		Start:   circuit.NodeName("hub"),
		Done:    circuit.NodeName("_done"),
		Nodes:   nodes,
		Edges:   edges,
	}
}

func megaFaninDef() *circuit.CircuitDef {
	nodes := []circuit.NodeDef{{Name: circuit.NodeName("start")}}
	edges := []circuit.EdgeDef{}
	for i := 0; i < 8; i++ {
		name := "s" + string(rune('1'+i))
		nodes = append(nodes, circuit.NodeDef{Name: circuit.NodeName(name)})
		edges = append(edges, circuit.EdgeDef{
			ID: "fan-" + name, From: "start", To: circuit.NodeName(name),
		})
	}
	nodes = append(nodes, circuit.NodeDef{Name: circuit.NodeName("merge")})
	for i := 0; i < 8; i++ {
		name := "s" + string(rune('1'+i))
		edges = append(edges, circuit.EdgeDef{
			ID: "join-" + name, From: circuit.NodeName(name), To: circuit.NodeName("merge"),
		})
	}
	edges = append(edges, circuit.EdgeDef{ID: "fin", From: circuit.NodeName("merge"), To: circuit.NodeName("_done")})
	return &circuit.CircuitDef{
		Circuit: "mega-fanin",
		Start:   circuit.NodeName("start"),
		Done:    circuit.NodeName("_done"),
		Nodes:   nodes,
		Edges:   edges,
	}
}

func deepCascadeDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "deep-cascade",
		Start:   "root",
		Done:    circuit.NodeName("_done"),
		Nodes: []circuit.NodeDef{
			{Name: "root"},
			{Name: "a"}, {Name: "b"},
			{Name: "c"}, {Name: "d"}, {Name: "e"}, {Name: "f"},
			{Name: "g"}, {Name: "h"},
			{Name: circuit.NodeName("merge")},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "root", To: "a"},
			{ID: "e2", From: "root", To: "b"},
			{ID: "e3", From: "a", To: "c"},
			{ID: "e4", From: "a", To: "d"},
			{ID: "e5", From: "b", To: "e"},
			{ID: "e6", From: "b", To: "f"},
			{ID: "e7", From: "c", To: "g"},
			{ID: "e8", From: "c", To: "h"},
			{ID: "e9", From: "d", To: circuit.NodeName("merge")},
			{ID: "e10", From: "e", To: circuit.NodeName("merge")},
			{ID: "e11", From: "f", To: circuit.NodeName("merge")},
			{ID: "e12", From: "g", To: circuit.NodeName("merge")},
			{ID: "e13", From: "h", To: circuit.NodeName("merge")},
			{ID: "e14", From: circuit.NodeName("merge"), To: circuit.NodeName("_done")},
		},
	}
}

func wideGridDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "wide-grid",
		Start:   circuit.NodeName("start"),
		Done:    circuit.NodeName("_done"),
		Nodes: []circuit.NodeDef{
			{Name: circuit.NodeName("start")},
			{Name: "a1"}, {Name: "a2"}, {Name: "a3"},
			{Name: "b1"}, {Name: "b2"}, {Name: "b3"},
			{Name: "c1"}, {Name: "c2"}, {Name: "c3"},
			{Name: circuit.NodeName("merge")},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "start", To: "a1"},
			{ID: "e2", From: "start", To: "a2"},
			{ID: "e3", From: "start", To: "a3"},
			{ID: "e4", From: "a1", To: "b1"},
			{ID: "e5", From: "a2", To: "b2"},
			{ID: "e6", From: "a3", To: "b3"},
			{ID: "e7", From: "b1", To: "c1"},
			{ID: "e8", From: "b2", To: "c2"},
			{ID: "e9", From: "b3", To: "c3"},
			{ID: "e10", From: "c1", To: circuit.NodeName("merge")},
			{ID: "e11", From: "c2", To: circuit.NodeName("merge")},
			{ID: "e12", From: "c3", To: circuit.NodeName("merge")},
			{ID: "e13", From: circuit.NodeName("merge"), To: circuit.NodeName("_done")},
		},
	}
}

func TestGridLayout_LinearCircuit(t *testing.T) {
	def := testCircuitDef()
	var gl GridLayout
	layout, err := gl.Layout(def)
	if err != nil {
		t.Fatal(err)
	}

	if len(layout.Grid) != 4 {
		t.Fatalf("grid has %d nodes, want 4", len(layout.Grid))
	}

	recall := layout.Grid["recall"]
	if recall.Col != 0 {
		t.Errorf("recall col = %d, want 0 (start node)", recall.Col)
	}

	triage := layout.Grid["triage"]
	investigate := layout.Grid["investigate"]
	report := layout.Grid["report"]

	if triage.Col <= recall.Col {
		t.Error("triage should be after recall")
	}
	if investigate.Col <= triage.Col {
		t.Error("investigate should be after triage")
	}
	if report.Col <= investigate.Col {
		t.Error("report should be after investigate")
	}
}

func TestGridLayout_ZoneGrouping(t *testing.T) {
	def := testCircuitDef()
	var gl GridLayout
	layout, err := gl.Layout(def)
	if err != nil {
		t.Fatal(err)
	}

	recall := layout.Grid["recall"]
	if recall.Zone != "analysis" {
		t.Errorf("recall zone = %q, want %q", recall.Zone, "analysis")
	}
	report := layout.Grid["report"]
	if report.Zone != "output" {
		t.Errorf("report zone = %q, want %q", report.Zone, "output")
	}
}

func TestGridLayout_EmptyCircuit(t *testing.T) {
	def := &circuit.CircuitDef{}
	var gl GridLayout
	layout, err := gl.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Grid) != 0 {
		t.Errorf("empty circuit should produce empty grid, got %d", len(layout.Grid))
	}
}

func TestGridLayout_ParallelNodes(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "parallel",
		Start:   circuit.NodeName("start"),
		Nodes: []circuit.NodeDef{
			{Name: circuit.NodeName("start")},
			{Name: "a"},
			{Name: "b"},
			{Name: "join"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "start", To: "a"},
			{ID: "e2", From: "start", To: "b"},
			{ID: "e3", From: "a", To: "join"},
			{ID: "e4", From: "b", To: "join"},
		},
	}

	var gl GridLayout
	layout, err := gl.Layout(def)
	if err != nil {
		t.Fatal(err)
	}

	startCell := layout.Grid["start"]
	aCell := layout.Grid["a"]
	bCell := layout.Grid["b"]
	joinCell := layout.Grid["join"]

	if startCell.Col != 0 {
		t.Errorf("start col = %d, want 0", startCell.Col)
	}
	if aCell.Col != 1 || bCell.Col != 1 {
		t.Errorf("a col = %d, b col = %d, both should be 1", aCell.Col, bCell.Col)
	}
	if aCell.Row == bCell.Row {
		t.Error("parallel nodes a and b should be in different rows")
	}
	if joinCell.Col != 2 {
		t.Errorf("join col = %d, want 2", joinCell.Col)
	}
}

func TestGridLayout_LoopEdgesIgnored(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "loop",
		Start:   "a",
		Nodes: []circuit.NodeDef{
			{Name: "a"},
			{Name: "b"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "a", To: "b"},
			{ID: "e2", From: "b", To: "a", Loop: true},
		},
	}

	var gl GridLayout
	layout, err := gl.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	if layout.Grid["a"].Col != 0 {
		t.Errorf("a col = %d, want 0", layout.Grid["a"].Col)
	}
	if layout.Grid["b"].Col != 1 {
		t.Errorf("b col = %d, want 1", layout.Grid["b"].Col)
	}
}

func TestGridLayout_Edges(t *testing.T) {
	def := testCircuitDef()
	var gl GridLayout
	layout, err := gl.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Edges) != 3 {
		t.Errorf("edges = %d, want 3", len(layout.Edges))
	}
}

func TestGridLayout_Zones(t *testing.T) {
	def := testCircuitDef()
	var gl GridLayout
	layout, err := gl.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Zones) != 2 {
		t.Errorf("zones = %d, want 2", len(layout.Zones))
	}
	zoneMap := make(map[string]string)
	for _, z := range layout.Zones {
		zoneMap[z.Name] = z.Element
	}
	if zoneMap["analysis"] != "fire" {
		t.Errorf("analysis element = %q, want fire", zoneMap["analysis"])
	}
	if zoneMap["output"] != "water" {
		t.Errorf("output element = %q, want water", zoneMap["output"])
	}
}

// --- Level 1: Structural layout tests ---

func TestGridLayout_Linear(t *testing.T) {
	def := linearDef()
	layout, err := GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	assertCol(t, layout, "A", 0)
	assertBefore(t, layout, "A", "B")
	assertBefore(t, layout, "B", "C")
	assertBefore(t, layout, "C", "D")
	assertRowSpan(t, layout, 1)
	assertColSpan(t, layout, 5) // A, B, C, D, _done
}

func TestGridLayout_Shortcut(t *testing.T) {
	def := shortcutDef()
	layout, err := GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	assertCol(t, layout, "A", 0)
	assertBefore(t, layout, "A", "B")
	assertBefore(t, layout, "B", "C")
	assertBefore(t, layout, "C", "D")
	assertRowSpan(t, layout, 1)
}

func TestGridLayout_Loop(t *testing.T) {
	def := loopDef()
	layout, err := GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	assertCol(t, layout, "A", 0)
	assertBefore(t, layout, "A", "B")
	assertBefore(t, layout, "B", "C")
	assertRowSpan(t, layout, 1)
}

func TestGridLayout_Diamond(t *testing.T) {
	def := diamondDef()
	layout, err := GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	assertCol(t, layout, "start", 0)
	assertSameCol(t, layout, "a", "b")
	assertDiffRow(t, layout, "a", "b")
	assertBefore(t, layout, "start", "a")
	assertBefore(t, layout, "a", "join")
	assertRowSpan(t, layout, 2)
}

func TestGridLayout_Staircase(t *testing.T) {
	def := staircaseDef()
	layout, err := GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	assertCol(t, layout, "start", 0)
	assertSameCol(t, layout, "a", "b")
	assertDiffRow(t, layout, "a", "b")
	assertSameCol(t, layout, "c", "d")
	assertDiffRow(t, layout, "c", "d")
	assertBefore(t, layout, "start", "a")
	assertBefore(t, layout, "a", "c")
	assertBefore(t, layout, "c", "end")
	assertMinRowSpan(t, layout, 2)
}

func TestGridLayout_Dialectic(t *testing.T) {
	def := dialecticDef()
	layout, err := GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	assertCol(t, layout, "indict", 0)
	assertBefore(t, layout, "indict", "discover")
	assertBefore(t, layout, "discover", "defend")
	assertBefore(t, layout, "defend", "hearing")
	assertBefore(t, layout, "hearing", "verdict")
	assertMinRowSpan(t, layout, 1)
}

func TestGridLayout_MegaFanout(t *testing.T) {
	def := megaFanoutDef()
	layout, err := GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	assertCol(t, layout, "hub", 0)
	for i := 0; i < 8; i++ {
		name := "t" + string(rune('1'+i))
		assertBefore(t, layout, "hub", name)
		assertBefore(t, layout, name, "merge")
	}
	assertRowSpan(t, layout, 8)
}

func TestGridLayout_MegaFanin(t *testing.T) {
	def := megaFaninDef()
	layout, err := GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	assertCol(t, layout, "start", 0)
	for i := 0; i < 8; i++ {
		name := "s" + string(rune('1'+i))
		assertBefore(t, layout, "start", name)
		assertBefore(t, layout, name, "merge")
	}
	assertRowSpan(t, layout, 8)
}

func TestGridLayout_DeepCascade(t *testing.T) {
	def := deepCascadeDef()
	layout, err := GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	assertCol(t, layout, "root", 0)
	assertSameCol(t, layout, "a", "b")
	assertDiffRow(t, layout, "a", "b")
	assertBefore(t, layout, "root", "a")
	assertBefore(t, layout, "a", "c")
	assertBefore(t, layout, "c", "g")
	assertBefore(t, layout, "g", "merge")
	assertMinRowSpan(t, layout, 4)
}

func TestGridLayout_WideGrid(t *testing.T) {
	def := wideGridDef()
	layout, err := GridLayout{}.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	assertCol(t, layout, "start", 0)
	assertSameCol(t, layout, "a1", "a2")
	assertSameCol(t, layout, "a2", "a3")
	assertSameCol(t, layout, "b1", "b2")
	assertSameCol(t, layout, "b2", "b3")
	assertSameCol(t, layout, "c1", "c2")
	assertSameCol(t, layout, "c2", "c3")
	assertBefore(t, layout, "a1", "b1")
	assertBefore(t, layout, "b1", "c1")
	assertBefore(t, layout, "c1", "merge")
	assertRowSpan(t, layout, 3)
	assertColSpan(t, layout, 6) // start, a*, b*, c*, merge, _done
}

func TestLogicalLayout_LinearCircuit(t *testing.T) {
	def := testCircuitDef()
	var ll LogicalLayout
	layout, err := ll.Layout(def)
	if err != nil {
		t.Fatal(err)
	}

	if len(layout.Logical) != 4 {
		t.Fatalf("logical has %d nodes, want 4", len(layout.Logical))
	}

	recall := layout.Logical["recall"]
	triage := layout.Logical["triage"]
	investigate := layout.Logical["investigate"]
	report := layout.Logical["report"]

	if recall.X >= triage.X {
		t.Error("recall.X should be < triage.X")
	}
	if triage.X >= investigate.X {
		t.Error("triage.X should be < investigate.X")
	}
	if investigate.X >= report.X {
		t.Error("investigate.X should be < report.X")
	}
}

func TestLogicalLayout_EmptyCircuit(t *testing.T) {
	def := &circuit.CircuitDef{}
	var ll LogicalLayout
	layout, err := ll.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Logical) != 0 {
		t.Errorf("empty circuit should produce empty logical, got %d", len(layout.Logical))
	}
}

func TestLogicalLayout_ParallelNodes(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "parallel",
		Start:   circuit.NodeName("start"),
		Nodes: []circuit.NodeDef{
			{Name: circuit.NodeName("start")},
			{Name: "a"},
			{Name: "b"},
			{Name: "join"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "start", To: "a"},
			{ID: "e2", From: "start", To: "b"},
			{ID: "e3", From: "a", To: "join"},
			{ID: "e4", From: "b", To: "join"},
		},
	}

	var ll LogicalLayout
	layout, err := ll.Layout(def)
	if err != nil {
		t.Fatal(err)
	}

	aPos := layout.Logical["a"]
	bPos := layout.Logical["b"]

	if aPos.X != bPos.X {
		t.Errorf("a.X = %f, b.X = %f, should be equal (same rank)", aPos.X, bPos.X)
	}
	if aPos.Y == bPos.Y {
		t.Error("parallel nodes should have different Y positions")
	}
}

func TestLogicalLayout_ZoneAssignment(t *testing.T) {
	def := testCircuitDef()
	var ll LogicalLayout
	layout, err := ll.Layout(def)
	if err != nil {
		t.Fatal(err)
	}

	if layout.Logical["recall"].Zone != "analysis" {
		t.Errorf("recall zone = %q, want analysis", layout.Logical["recall"].Zone)
	}
	if layout.Logical["report"].Zone != "output" {
		t.Errorf("report zone = %q, want output", layout.Logical["report"].Zone)
	}
}

func TestLogicalLayout_Edges(t *testing.T) {
	def := testCircuitDef()
	var ll LogicalLayout
	layout, err := ll.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Edges) != 3 {
		t.Errorf("edges = %d, want 3", len(layout.Edges))
	}
}

func TestLogicalLayout_Zones(t *testing.T) {
	def := testCircuitDef()
	var ll LogicalLayout
	layout, err := ll.Layout(def)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Zones) != 2 {
		t.Errorf("zones = %d, want 2", len(layout.Zones))
	}
}
