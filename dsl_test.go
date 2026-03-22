package framework

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLoadCircuit_ValidYAML(t *testing.T) {
	data := []byte(`
circuit: test-pipe
description: "A test circuit"
handler_type: node
nodes:
  - name: a
    element: fire
    handler: start
  - name: b
    element: water
    handler: finish
edges:
  - id: E1
    name: a-to-b
    from: a
    to: b
    condition: "always"
  - id: E2
    name: b-done
    from: b
    to: _done
    condition: "terminal"
start: a
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if def.Circuit != "test-pipe" {
		t.Errorf("Circuit = %q, want %q", def.Circuit, "test-pipe")
	}
	if def.Description != "A test circuit" {
		t.Errorf("Description = %q, want %q", def.Description, "A test circuit")
	}
	if len(def.Nodes) != 2 {
		t.Errorf("len(Nodes) = %d, want 2", len(def.Nodes))
	}
	if len(def.Edges) != 2 {
		t.Errorf("len(Edges) = %d, want 2", len(def.Edges))
	}
	if def.Start != "a" {
		t.Errorf("Start = %q, want %q", def.Start, "a")
	}
	if def.Done != "_done" {
		t.Errorf("Done = %q, want %q", def.Done, "_done")
	}
}

func TestLoadCircuit_WithZones(t *testing.T) {
	data := []byte(`
circuit: zoned
handler_type: node
nodes:
  - name: x
    handler: x
  - name: y
    handler: y
zones:
  front:
    nodes: [x]
    approach: rapid
    stickiness: 2
  back:
    nodes: [y]
    approach: analytical
edges:
  - id: E1
    name: x-to-y
    from: x
    to: y
  - id: E2
    name: y-done
    from: y
    to: _done
start: x
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if len(def.Zones) != 2 {
		t.Fatalf("len(Zones) = %d, want 2", len(def.Zones))
	}
	front := def.Zones["front"]
	if front.Stickiness != 2 {
		t.Errorf("front.Stickiness = %d, want 2", front.Stickiness)
	}
	if front.Approach != "rapid" {
		t.Errorf("front.Approach = %q, want %q", front.Approach, "rapid")
	}
}

func TestLoadCircuit_Ports(t *testing.T) {
	// Test that ports are parsed from YAML
	data := []byte(`
circuit: ports-test
handler_type: node
nodes:
  - name: a
    handler: a
  - name: b
    handler: b
edges:
  - id: E1
    from: a
    to: b
  - id: E2
    from: b
    to: _done
ports:
  - name: keywords
    direction: in
    type: "[]string"
    description: "Search keywords from upstream"
  - name: post-triage
    direction: out
    type: "map[string]any"
wiring:
  - from: "rca.out:post-triage"
    to: "gnd.in:keywords"
    adapter: "keyword-extractor"
start: a
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if len(def.Ports) != 2 {
		t.Fatalf("len(Ports) = %d, want 2", len(def.Ports))
	}
	if def.Ports[0].Name != "keywords" || def.Ports[0].Direction != "in" || def.Ports[0].Type != "[]string" {
		t.Errorf("Ports[0] = %+v, want name=keywords direction=in type=[]string", def.Ports[0])
	}
	if def.Ports[1].Name != "post-triage" || def.Ports[1].Direction != "out" {
		t.Errorf("Ports[1] = %+v, want name=post-triage direction=out", def.Ports[1])
	}
	if len(def.Wiring) != 1 {
		t.Fatalf("len(Wiring) = %d, want 1", len(def.Wiring))
	}
	if def.Wiring[0].From != "rca.out:post-triage" || def.Wiring[0].To != "gnd.in:keywords" || def.Wiring[0].Adapter != "keyword-extractor" {
		t.Errorf("Wiring[0] = %+v, want from=rca.out:post-triage to=gnd.in:keywords adapter=keyword-extractor", def.Wiring[0])
	}

	// Test that overlay merges ports and wiring correctly
	base := []byte(`
circuit: base
nodes:
  - name: a
    handler: a
edges:
  - id: E1
    from: a
    to: _done
ports:
  - name: base-port
    direction: in
    type: string
start: a
done: _done
`)
	overlay := []byte(`
import: base-ports
circuit: overlay
ports:
  - name: overlay-port
    direction: out
    type: "[]string"
wiring:
  - from: "base.out:result"
    to: "other.in:input"
nodes:
  - name: b
    handler: b
edges:
  - id: E2
    from: a
    to: b
  - id: E3
    from: b
    to: _done
start: a
done: _done
`)
	resolver := func(name string) ([]byte, error) {
		if name == "base-ports" {
			return base, nil
		}
		return nil, os.ErrNotExist
	}
	merged, err := LoadCircuitWithOverlay(overlay, resolver)
	if err != nil {
		t.Fatalf("LoadCircuitWithOverlay: %v", err)
	}
	// Ports: base + overlay (no collision)
	if len(merged.Ports) != 2 {
		t.Fatalf("merged len(Ports) = %d, want 2 (base + overlay)", len(merged.Ports))
	}
	portNames := make(map[string]bool)
	for _, p := range merged.Ports {
		portNames[p.Name] = true
	}
	if !portNames["base-port"] || !portNames["overlay-port"] {
		t.Errorf("merged Ports missing expected names: %v", portNames)
	}
	// Wiring: overlay appended
	if len(merged.Wiring) != 1 {
		t.Fatalf("merged len(Wiring) = %d, want 1", len(merged.Wiring))
	}
	if merged.Wiring[0].From != "base.out:result" || merged.Wiring[0].To != "other.in:input" {
		t.Errorf("merged Wiring[0] = %+v", merged.Wiring[0])
	}

	// Test port name collision = error
	badOverlay := []byte(`
import: base-ports
circuit: bad
ports:
  - name: base-port
    direction: out
nodes:
  - name: c
    handler: c
edges:
  - id: E4
    from: a
    to: c
  - id: E5
    from: c
    to: _done
start: a
done: _done
`)
	_, err = LoadCircuitWithOverlay(badOverlay, resolver)
	if err == nil {
		t.Fatal("expected error for port name collision, got nil")
	}
	if !contains(err.Error(), "base-port") || !contains(err.Error(), "append-only") {
		t.Errorf("error should mention port collision: %v", err)
	}
}

func TestLoadCircuit_InvalidYAML(t *testing.T) {
	data := []byte(`{invalid yaml: [`)
	_, err := LoadCircuit(data)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidate_Valid(t *testing.T) {
	def := &CircuitDef{
		Circuit: "test",
		Nodes:    []NodeDef{{Name: "a"}, {Name: "b"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "b"}, {ID: "E2", From: "b", To: "_done"}},
		Start:    "a",
		Done:     "_done",
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_EmptyCircuitName(t *testing.T) {
	def := &CircuitDef{Nodes: []NodeDef{{Name: "a"}}, Edges: []EdgeDef{{ID: "E1", From: "a", To: "_done"}}, Start: "a", Done: "_done"}
	if err := def.Validate(); err == nil {
		t.Fatal("expected error for empty circuit name")
	}
}

func TestValidate_MissingStartNode(t *testing.T) {
	def := &CircuitDef{
		Circuit: "test",
		Nodes:    []NodeDef{{Name: "a"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "_done"}},
		Start:    "nonexistent",
		Done:     "_done",
	}
	if err := def.Validate(); err == nil {
		t.Fatal("expected error for missing start node")
	}
}

func TestValidate_BrokenEdgeSource(t *testing.T) {
	def := &CircuitDef{
		Circuit: "test",
		Nodes:    []NodeDef{{Name: "a"}},
		Edges:    []EdgeDef{{ID: "E1", From: "ghost", To: "a"}},
		Start:    "a",
		Done:     "_done",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for broken edge source")
	}
	if !contains(err.Error(), "ghost") {
		t.Errorf("error should name the invalid reference: %v", err)
	}
}

func TestValidate_BrokenEdgeTarget(t *testing.T) {
	def := &CircuitDef{
		Circuit: "test",
		Nodes:    []NodeDef{{Name: "a"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "ghost"}},
		Start:    "a",
		Done:     "_done",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for broken edge target")
	}
	if !contains(err.Error(), "ghost") {
		t.Errorf("error should name the invalid reference: %v", err)
	}
}

func TestValidate_BrokenZoneNode(t *testing.T) {
	def := &CircuitDef{
		Circuit: "test",
		Nodes:    []NodeDef{{Name: "a"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "_done"}},
		Zones:    map[string]ZoneDef{"z": {Nodes: []string{"ghost"}}},
		Start:    "a",
		Done:     "_done",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for broken zone node reference")
	}
	if !contains(err.Error(), "ghost") {
		t.Errorf("error should name the invalid reference: %v", err)
	}
}

func TestValidate_DuplicateNodeName(t *testing.T) {
	def := &CircuitDef{
		Circuit: "test",
		Nodes:    []NodeDef{{Name: "a"}, {Name: "a"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "_done"}},
		Start:    "a",
		Done:     "_done",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate node name")
	}
}

func TestValidate_DuplicateEdgeID(t *testing.T) {
	def := &CircuitDef{
		Circuit: "test",
		Nodes:    []NodeDef{{Name: "a"}, {Name: "b"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "b"}, {ID: "E1", From: "b", To: "_done"}},
		Start:    "a",
		Done:     "_done",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate edge id")
	}
}

func TestRoundTripFidelity(t *testing.T) {
	original := &CircuitDef{
		Circuit:    "roundtrip",
		Description: "test round trip",
		HandlerType: "node",
		Nodes:       []NodeDef{{Name: "a", Approach: "rapid", Handler: "start"}, {Name: "b", Handler: "end"}},
		Edges:       []EdgeDef{{ID: "E1", Name: "a-b", From: "a", To: "b"}, {ID: "E2", Name: "b-done", From: "b", To: "_done"}},
		Start:       "a",
		Done:        "_done",
	}

	data, err := original.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}

	restored, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit round-trip: %v", err)
	}

	if restored.Circuit != original.Circuit {
		t.Errorf("Circuit = %q, want %q", restored.Circuit, original.Circuit)
	}
	if len(restored.Nodes) != len(original.Nodes) {
		t.Errorf("len(Nodes) = %d, want %d", len(restored.Nodes), len(original.Nodes))
	}
	if len(restored.Edges) != len(original.Edges) {
		t.Errorf("len(Edges) = %d, want %d", len(restored.Edges), len(original.Edges))
	}
	if restored.Start != original.Start {
		t.Errorf("Start = %q, want %q", restored.Start, original.Start)
	}
	if restored.Done != original.Done {
		t.Errorf("Done = %q, want %q", restored.Done, original.Done)
	}
	for i, n := range restored.Nodes {
		if n.Name != original.Nodes[i].Name {
			t.Errorf("Node[%d].Name = %q, want %q", i, n.Name, original.Nodes[i].Name)
		}
	}
}

func TestLoadCircuit_RealF0F6(t *testing.T) {
	data, err := os.ReadFile("testdata/rca-investigation.yaml")
	if err != nil {
		t.Fatalf("read rca-investigation.yaml: %v", err)
	}
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if def.Circuit != "rca-investigation" {
		t.Errorf("Circuit = %q, want %q", def.Circuit, "rca-investigation")
	}
	if len(def.Nodes) != 7 {
		t.Errorf("len(Nodes) = %d, want 7", len(def.Nodes))
	}
	if len(def.Zones) != 3 {
		t.Errorf("len(Zones) = %d, want 3", len(def.Zones))
	}
}

func TestLoadCircuit_RealDefectDialectic(t *testing.T) {
	data, err := os.ReadFile("testdata/defect-dialectic.yaml")
	if err != nil {
		t.Fatalf("read defect-dialectic.yaml: %v", err)
	}
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if def.Circuit != "defect-dialectic" {
		t.Errorf("Circuit = %q, want %q", def.Circuit, "defect-dialectic")
	}
	if len(def.Nodes) != 6 {
		t.Errorf("len(Nodes) = %d, want 6", len(def.Nodes))
	}
}

func TestLoadCircuit_NodeDescription(t *testing.T) {
	data := []byte(`
circuit: desc-test
nodes:
  - name: recall
    description: "Pattern-match against known failures database"
    element: fire
  - name: triage
    element: earth
edges:
  - id: E1
    name: proceed
    from: recall
    to: triage
start: recall
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if def.Nodes[0].Description != "Pattern-match against known failures database" {
		t.Errorf("Nodes[0].Description = %q, want pattern-match description", def.Nodes[0].Description)
	}
	if def.Nodes[1].Description != "" {
		t.Errorf("Nodes[1].Description = %q, want empty (optional field)", def.Nodes[1].Description)
	}
}

func TestLoadCircuit_NodeDescription_RoundTrip(t *testing.T) {
	data := []byte(`
circuit: roundtrip
nodes:
  - name: a
    description: "First node"
  - name: b
    description: "Second node"
edges:
  - id: E1
    from: a
    to: b
start: a
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	for i, want := range []string{"First node", "Second node"} {
		if def.Nodes[i].Description != want {
			t.Errorf("Nodes[%d].Description = %q, want %q", i, def.Nodes[i].Description, want)
		}
	}
}

func TestLoadCircuit_CompactEdges(t *testing.T) {
	data := []byte(`
circuit: compact
nodes:
  - name: a
    element: fire
    edges:
      - name: go-to-b
        to: b
        when: "output.ready == true"
      - name: skip-to-c
        to: c
        shortcut: true
        when: "output.skip == true"
  - name: b
    element: water
    edges:
      - name: to-c
        to: c
        when: "true"
  - name: c
    element: earth
    edges:
      - name: done
        to: _done
        when: "true"
start: a
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(def.Edges) != 4 {
		t.Fatalf("len(Edges) = %d, want 4", len(def.Edges))
	}

	e0 := def.Edges[0]
	if e0.From != "a" || e0.To != "b" || e0.Name != "go-to-b" {
		t.Errorf("edge[0] = %+v, want from=a to=b name=go-to-b", e0)
	}
	if e0.ID != "a-go-to-b" {
		t.Errorf("edge[0].ID = %q, want %q", e0.ID, "a-go-to-b")
	}

	e1 := def.Edges[1]
	if !e1.Shortcut {
		t.Error("edge[1] should be shortcut")
	}

	e3 := def.Edges[3]
	if e3.From != "c" || e3.To != "_done" {
		t.Errorf("edge[3] = %+v, want from=c to=_done", e3)
	}
}

func TestLoadCircuit_FlowStyleEdges(t *testing.T) {
	data := []byte(`
circuit: linear
nodes:
  - name: setup
    edges: [run]
  - name: run
    edges: [report]
  - name: report
    edges: [_done]
start: setup
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(def.Edges) != 3 {
		t.Fatalf("len(Edges) = %d, want 3", len(def.Edges))
	}
	if def.Edges[0].From != "setup" || def.Edges[0].To != "run" {
		t.Errorf("edge[0] = %+v, want setup -> run", def.Edges[0])
	}
	if def.Edges[0].ID != "setup-run" {
		t.Errorf("edge[0].ID = %q, want %q", def.Edges[0].ID, "setup-run")
	}
}

func TestLoadCircuit_CompactVerboseEquivalence(t *testing.T) {
	compact := []byte(`
circuit: equiv
nodes:
  - name: a
    handler: start
    edges:
      - name: proceed
        to: b
        when: "true"
  - name: b
    handler: finish
    edges:
      - name: done
        to: _done
        when: "true"
start: a
done: _done
`)
	verbose := []byte(`
circuit: equiv
nodes:
  - name: a
    handler: start
  - name: b
    handler: finish
edges:
  - id: a-proceed
    name: proceed
    from: a
    to: b
    when: "true"
  - id: b-done
    name: done
    from: b
    to: _done
    when: "true"
start: a
done: _done
`)
	cDef, err := LoadCircuit(compact)
	if err != nil {
		t.Fatalf("LoadCircuit compact: %v", err)
	}
	vDef, err := LoadCircuit(verbose)
	if err != nil {
		t.Fatalf("LoadCircuit verbose: %v", err)
	}

	if len(cDef.Nodes) != len(vDef.Nodes) {
		t.Fatalf("node count: compact=%d verbose=%d", len(cDef.Nodes), len(vDef.Nodes))
	}
	if len(cDef.Edges) != len(vDef.Edges) {
		t.Fatalf("edge count: compact=%d verbose=%d", len(cDef.Edges), len(vDef.Edges))
	}
	for i, ce := range cDef.Edges {
		ve := vDef.Edges[i]
		if ce.ID != ve.ID || ce.From != ve.From || ce.To != ve.To || ce.When != ve.When || ce.Name != ve.Name {
			t.Errorf("edge[%d] mismatch:\n  compact: %+v\n  verbose: %+v", i, ce, ve)
		}
	}
}

func TestLoadCircuit_HandlerFallsBackToName(t *testing.T) {
	data := []byte(`
circuit: fam
handler_type: node
nodes:
  - name: recall
    edges: [triage]
  - name: triage
    handler: triage-custom
    edges: [_done]
start: recall
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if h := def.Nodes[0].EffectiveHandler(); h != "recall" {
		t.Errorf("Nodes[0].EffectiveHandler() = %q, want %q (implicit from name)", h, "recall")
	}
	if h := def.Nodes[1].EffectiveHandler(); h != "triage-custom" {
		t.Errorf("Nodes[1].EffectiveHandler() = %q, want %q (explicit)", h, "triage-custom")
	}
}

func TestLoadCircuit_AutoGenerateEdgeID(t *testing.T) {
	data := []byte(`
circuit: ids
nodes:
  - name: a
    edges:
      - name: first path
        to: b
        when: "output.x == 1"
      - name: second path
        to: b
        when: "output.x == 2"
      - to: c
start: a
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	wantIDs := []string{"a-first-path", "a-second-path", "a-c"}
	for i, want := range wantIDs {
		if def.Edges[i].ID != want {
			t.Errorf("edge[%d].ID = %q, want %q", i, def.Edges[i].ID, want)
		}
	}
}

func TestLoadCircuit_MixedEdges(t *testing.T) {
	data := []byte(`
circuit: mixed
nodes:
  - name: a
    edges:
      - name: inline
        to: b
        when: "true"
  - name: b
edges:
  - id: EXT1
    name: external
    from: b
    to: _done
    when: "true"
start: a
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if len(def.Edges) != 2 {
		t.Fatalf("len(Edges) = %d, want 2", len(def.Edges))
	}
	if def.Edges[0].ID != "EXT1" {
		t.Errorf("edge[0].ID = %q, want EXT1 (top-level first)", def.Edges[0].ID)
	}
	if def.Edges[1].ID != "a-inline" {
		t.Errorf("edge[1].ID = %q, want a-inline (inline second)", def.Edges[1].ID)
	}
}

func TestLoadCircuit_CompactEdge_MissingTo(t *testing.T) {
	data := []byte(`
circuit: bad
nodes:
  - name: a
    edges:
      - name: oops
        when: "true"
start: a
done: _done
`)
	_, err := LoadCircuit(data)
	if err == nil {
		t.Fatal("expected error for inline edge missing 'to'")
	}
	if !contains(err.Error(), "missing") {
		t.Errorf("error should mention 'missing': %v", err)
	}
}

func TestLoadCircuit_CompactEdge_LoopFlag(t *testing.T) {
	data := []byte(`
circuit: loopy
nodes:
  - name: a
    edges:
      - name: forward
        to: b
        when: "true"
  - name: b
    edges:
      - name: back
        to: a
        loop: true
        when: "state.loops.b < 3"
      - name: done
        to: _done
        when: "true"
start: a
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	backEdge := def.Edges[1]
	if !backEdge.Loop {
		t.Error("back edge should have loop=true")
	}
	if backEdge.Name != "back" || backEdge.From != "b" || backEdge.To != "a" {
		t.Errorf("back edge = %+v", backEdge)
	}
}

func TestInferTopology_LinearChain(t *testing.T) {
	def := &CircuitDef{
		Circuit: "linear",
		Nodes:   []NodeDef{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b"},
			{ID: "E2", From: "b", To: "c"},
			{ID: "E3", From: "c", To: "_done"},
		},
		Start: "a",
		Done:  "_done",
	}
	InferTopology(def)
	for _, e := range def.Edges {
		if e.Shortcut {
			t.Errorf("edge %s should not be shortcut in linear chain", e.ID)
		}
		if e.Loop {
			t.Errorf("edge %s should not be loop in linear chain", e.ID)
		}
	}
}

func TestInferTopology_ForwardSkip(t *testing.T) {
	def := &CircuitDef{
		Circuit: "skip",
		Nodes:   []NodeDef{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b"},
			{ID: "E2", From: "b", To: "c"},
			{ID: "E3", From: "a", To: "c"},
			{ID: "E4", From: "c", To: "_done"},
		},
		Start: "a",
		Done:  "_done",
	}
	InferTopology(def)
	if !def.Edges[2].Shortcut {
		t.Error("edge E3 (a->c) should be inferred as shortcut")
	}
	if def.Edges[0].Shortcut {
		t.Error("edge E1 (a->b) should not be shortcut")
	}
}

func TestInferTopology_BackwardEdge(t *testing.T) {
	def := &CircuitDef{
		Circuit: "loop",
		Nodes:   []NodeDef{{Name: "a"}, {Name: "b"}},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b"},
			{ID: "E2", From: "b", To: "a"},
			{ID: "E3", From: "b", To: "_done"},
		},
		Start: "a",
		Done:  "_done",
	}
	InferTopology(def)
	if !def.Edges[1].Loop {
		t.Error("edge E2 (b->a) should be inferred as loop")
	}
	if def.Edges[0].Loop {
		t.Error("edge E1 (a->b) should not be loop")
	}
}

func TestInferTopology_DiamondGraph(t *testing.T) {
	def := &CircuitDef{
		Circuit: "diamond",
		Nodes:   []NodeDef{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b"},
			{ID: "E2", From: "a", To: "c"},
			{ID: "E3", From: "b", To: "d"},
			{ID: "E4", From: "c", To: "d"},
			{ID: "E5", From: "d", To: "_done"},
		},
		Start: "a",
		Done:  "_done",
	}
	InferTopology(def)
	for _, e := range def.Edges {
		if e.Loop {
			t.Errorf("edge %s should not be loop in diamond", e.ID)
		}
	}
	if def.Edges[2].Shortcut || def.Edges[3].Shortcut {
		t.Error("edges to d should not be shortcuts (both are direct)")
	}
}

func TestInferTopology_TerminalEdge(t *testing.T) {
	def := &CircuitDef{
		Circuit: "terminal",
		Nodes:   []NodeDef{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		Edges: []EdgeDef{
			{ID: "E1", From: "a", To: "b"},
			{ID: "E2", From: "b", To: "c"},
			{ID: "E3", From: "a", To: "_done"},
			{ID: "E4", From: "c", To: "_done"},
		},
		Start: "a",
		Done:  "_done",
	}
	InferTopology(def)
	if def.Edges[2].Shortcut {
		t.Error("edge E3 (a->_done) should NOT be shortcut (terminal edges excluded)")
	}
}

func TestInferTopology_RCACircuit(t *testing.T) {
	data, err := os.ReadFile("testdata/rca-investigation.yaml")
	if err != nil {
		t.Fatalf("read rca-investigation.yaml: %v", err)
	}
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	shortcutsBefore := map[string]bool{}
	loopsBefore := map[string]bool{}
	for _, e := range def.Edges {
		if e.Shortcut {
			shortcutsBefore[e.ID] = true
		}
		if e.Loop {
			loopsBefore[e.ID] = true
		}
	}

	InferTopology(def)

	for _, e := range def.Edges {
		if e.Shortcut && !shortcutsBefore[e.ID] {
			t.Logf("INFO: edge %s (%s->%s) inferred as shortcut (was not declared)", e.ID, e.From, e.To)
		}
		if e.Loop && !loopsBefore[e.ID] {
			t.Logf("INFO: edge %s (%s->%s) inferred as loop (was not declared)", e.ID, e.From, e.To)
		}
	}

	for id := range shortcutsBefore {
		for _, e := range def.Edges {
			if e.ID == id && !e.Shortcut {
				t.Errorf("edge %s was declared shortcut but inference cleared it", id)
			}
		}
	}
	for id := range loopsBefore {
		for _, e := range def.Edges {
			if e.ID == id && !e.Loop {
				t.Errorf("edge %s was declared loop but inference cleared it", id)
			}
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// stubTransformer is a no-op Transformer for testing handler resolution.
type stubTransformer struct{ id string }

func (s *stubTransformer) Name() string { return s.id }
func (s *stubTransformer) Transform(_ context.Context, _ *TransformerContext) (any, error) {
	return map[string]any{"stub": s.id}, nil
}

// TestResolveNode_NoHandlerType_Fails verifies that a node with handler:
// but no handler_type (and no circuit default) produces a clear error.
func TestResolveNode_NoHandlerType_Fails(t *testing.T) {
	yaml := `
circuit: bug-demo
nodes:
  - name: recall
    handler: recall
    prompt: test.md
edges:
  - id: E1
    from: recall
    to: _done
    when: "true"
start: recall
done: _done
`
	def, err := LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"recall": &stubTransformer{id: "recall"},
		},
	}

	_, err = BuildGraph(def, reg)
	if err == nil {
		t.Fatal("expected BuildGraph to fail without handler_type, but it succeeded")
	}
	if !contains(err.Error(), "no handler_type") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestResolveNode_Handler_ResolvesTransformer verifies that the new
// handler + handler_type path correctly resolves a transformer.
func TestResolveNode_Handler_ResolvesTransformer(t *testing.T) {
	yaml := `
circuit: handler-demo
handler_type: transformer
nodes:
  - name: recall
    handler: recall
    prompt: test.md
edges:
  - id: E1
    from: recall
    to: _done
    when: "true"
start: recall
done: _done
`
	def, err := LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"recall": &stubTransformer{id: "recall"},
		},
	}

	g, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph with handler: recall failed: %v", err)
	}
	node, ok := g.NodeByName("recall")
	if !ok {
		t.Fatal("expected node 'recall' to exist in graph")
	}
	if _, ok := node.(*transformerNode); !ok {
		t.Errorf("expected *transformerNode, got %T", node)
	}
}

// TestResolveNode_Handler_NodeType verifies handler_type: node.
func TestResolveNode_Handler_NodeType(t *testing.T) {
	yaml := `
circuit: node-demo
handler_type: node
nodes:
  - name: setup
    handler: setup
edges:
  - id: E1
    from: setup
    to: _done
    when: "true"
start: setup
done: _done
`
	def, err := LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	called := false
	reg := GraphRegistries{
		Nodes: NodeRegistry{
			"setup": func(nd NodeDef) Node {
				called = true
				return &stubNode{name: nd.Name}
			},
		},
	}

	_, err = BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph with handler_type: node failed: %v", err)
	}
	if !called {
		t.Error("expected NodeRegistry factory to be called")
	}
}

// TestResolveNode_Handler_DelegateType verifies handler_type: delegate.
func TestResolveNode_Handler_DelegateType(t *testing.T) {
	yaml := `
circuit: delegate-demo
nodes:
  - name: plan
    handler_type: delegate
    handler: planner
edges:
  - id: E1
    from: plan
    to: _done
    when: "true"
start: plan
done: _done
`
	def, err := LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"planner": &stubTransformer{id: "planner"},
		},
	}

	g, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph with handler_type: delegate failed: %v", err)
	}
	node, ok := g.NodeByName("plan")
	if !ok {
		t.Fatal("expected node 'plan' to exist in graph")
	}
	if _, ok := node.(*dslDelegateNode); !ok {
		t.Errorf("expected *dslDelegateNode, got %T", node)
	}
}

// TestResolveNode_Handler_MissingHandlerType verifies the error when handler
// is set but handler_type is missing on both node and circuit.
func TestResolveNode_Handler_MissingHandlerType(t *testing.T) {
	yaml := `
circuit: no-type
nodes:
  - name: recall
    handler: recall
edges:
  - id: E1
    from: recall
    to: _done
    when: "true"
start: recall
done: _done
`
	def, err := LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"recall": &stubTransformer{id: "recall"},
		},
	}

	_, err = BuildGraph(def, reg)
	if err == nil {
		t.Fatal("expected error when handler is set but handler_type is missing")
	}
	if !contains(err.Error(), "no handler_type") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestResolveNode_Handler_NodeOverridesCircuitDefault verifies node-level
// handler_type takes precedence over circuit-level default.
func TestResolveNode_Handler_NodeOverridesCircuitDefault(t *testing.T) {
	yaml := `
circuit: override-demo
handler_type: transformer
nodes:
  - name: plan
    handler_type: delegate
    handler: planner
  - name: recall
    handler: recall
    prompt: test.md
edges:
  - id: E1
    from: plan
    to: recall
    when: "true"
  - id: E2
    from: recall
    to: _done
    when: "true"
start: plan
done: _done
`
	def, err := LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"planner": &stubTransformer{id: "planner"},
			"recall":  &stubTransformer{id: "recall"},
		},
	}

	g, err := BuildGraph(def, reg)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	planNode, ok := g.NodeByName("plan")
	if !ok {
		t.Fatal("expected node 'plan' to exist in graph")
	}
	if _, ok := planNode.(*dslDelegateNode); !ok {
		t.Errorf("expected plan to be *dslDelegateNode, got %T", planNode)
	}

	recallNode, ok := g.NodeByName("recall")
	if !ok {
		t.Fatal("expected node 'recall' to exist in graph")
	}
	if _, ok := recallNode.(*transformerNode); !ok {
		t.Errorf("expected recall to be *transformerNode, got %T", recallNode)
	}
}

// TestEffectiveHandlerType verifies the EffectiveHandlerType helper.
func TestEffectiveHandlerType(t *testing.T) {
	tests := []struct {
		name           string
		nd             NodeDef
		circuitDefault string
		want           string
	}{
		{
			name:           "node-level handler_type wins",
			nd:             NodeDef{HandlerType: "extractor", Handler: "x"},
			circuitDefault: "transformer",
			want:           "extractor",
		},
		{
			name:           "circuit default when node has handler but no type",
			nd:             NodeDef{Handler: "x"},
			circuitDefault: "transformer",
			want:           "transformer",
		},
		{
			name:           "empty returns empty",
			nd:             NodeDef{},
			circuitDefault: "",
			want:           "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.nd.EffectiveHandlerType(tt.circuitDefault)
			if got != tt.want {
				t.Errorf("EffectiveHandlerType() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestEffectiveTimeout verifies node-level override > circuit-level default > none.
func TestEffectiveTimeout(t *testing.T) {
	tests := []struct {
		name           string
		nd             NodeDef
		circuitDefault string
		want           time.Duration
		wantErr        bool
	}{
		{name: "no timeout set", nd: NodeDef{Name: "a"}, circuitDefault: "", want: 0},
		{name: "circuit default only", nd: NodeDef{Name: "a"}, circuitDefault: "30s", want: 30 * time.Second},
		{name: "node override", nd: NodeDef{Name: "a", Timeout: "2m"}, circuitDefault: "30s", want: 2 * time.Minute},
		{name: "node override no circuit", nd: NodeDef{Name: "a", Timeout: "5s"}, circuitDefault: "", want: 5 * time.Second},
		{name: "invalid node timeout", nd: NodeDef{Name: "bad", Timeout: "xyz"}, circuitDefault: "", wantErr: true},
		{name: "invalid circuit default", nd: NodeDef{Name: "bad"}, circuitDefault: "nope", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.nd.EffectiveTimeout(tt.circuitDefault)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("EffectiveTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadCircuit_Timeout(t *testing.T) {
	data := []byte(`
circuit: timeout-test
timeout: "30s"
nodes:
  - name: fast
    handler: fast
  - name: slow
    handler: slow
    timeout: "2m"
edges:
  - id: E1
    from: fast
    to: slow
  - id: E2
    from: slow
    to: _done
start: fast
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if def.Timeout != "30s" {
		t.Errorf("circuit timeout = %q, want %q", def.Timeout, "30s")
	}
	if def.Nodes[0].Timeout != "" {
		t.Errorf("fast node timeout = %q, want empty", def.Nodes[0].Timeout)
	}
	if def.Nodes[1].Timeout != "2m" {
		t.Errorf("slow node timeout = %q, want %q", def.Nodes[1].Timeout, "2m")
	}
}

// TestEffectiveHandler verifies the EffectiveHandler helper.
func TestEffectiveHandler(t *testing.T) {
	tests := []struct {
		name string
		nd   NodeDef
		want string
	}{
		{name: "handler wins", nd: NodeDef{Handler: "x"}, want: "x"},
		{name: "falls back to name", nd: NodeDef{Name: "n"}, want: "n"},
		{name: "empty name and handler", nd: NodeDef{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.nd.EffectiveHandler()
			if got != tt.want {
				t.Errorf("EffectiveHandler() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Overlay tests ---

func TestLoadCircuitWithOverlay_NoImport(t *testing.T) {
	data := []byte(`
circuit: plain
nodes:
  - name: a
    handler: a
edges:
  - id: E1
    from: a
    to: DONE
start: a
done: DONE
`)
	def, err := LoadCircuitWithOverlay(data, nil)
	if err != nil {
		t.Fatalf("LoadCircuitWithOverlay: %v", err)
	}
	if def.Circuit != "plain" {
		t.Errorf("Circuit = %q, want %q", def.Circuit, "plain")
	}
	if def.Import != "" {
		t.Errorf("Import should be empty, got %q", def.Import)
	}
}

func TestLoadCircuitWithOverlay_MergesBaseAndOverlay(t *testing.T) {
	base := []byte(`
circuit: base-circuit
description: base description
topology: cascade
handler_type: transformer
vars:
  threshold: 0.80
  max_loops: 3
nodes:
  - name: step-a
    handler: alpha
    edges:
      - to: step-b
        when: "true"
  - name: step-b
    handler: beta
    edges:
      - id: step-b-done
        to: DONE
        when: "true"
start: step-a
done: DONE
`)

	overlay := []byte(`
circuit: my-circuit
import: test-base
description: consumer description
vars:
  threshold: 0.95
  extra_var: hello
nodes:
  - name: step-c
    handler: gamma
    edges:
      - to: step-a
        when: "true"
edges:
  - id: step-b-done
    from: step-b
    to: step-c
    when: "output.needs_extra == true"
start: step-a
done: DONE
`)

	resolver := func(name string) ([]byte, error) {
		if name == "test-base" {
			return base, nil
		}
		return nil, os.ErrNotExist
	}

	def, err := LoadCircuitWithOverlay(overlay, resolver)
	if err != nil {
		t.Fatalf("LoadCircuitWithOverlay: %v", err)
	}

	if def.Circuit != "my-circuit" {
		t.Errorf("Circuit = %q, want %q", def.Circuit, "my-circuit")
	}
	if def.Description != "consumer description" {
		t.Errorf("Description = %q, want %q", def.Description, "consumer description")
	}
	if def.Import != "" {
		t.Errorf("Import should be cleared after merge, got %q", def.Import)
	}
	if def.Topology != "cascade" {
		t.Errorf("Topology = %q, want %q (inherited from base)", def.Topology, "cascade")
	}

	// Vars: overlay wins, base preserved
	if v, ok := def.Vars["threshold"].(float64); !ok || v != 0.95 {
		t.Errorf("Vars[threshold] = %v, want 0.95", def.Vars["threshold"])
	}
	if v, ok := def.Vars["max_loops"].(int); !ok || v != 3 {
		t.Errorf("Vars[max_loops] = %v, want 3", def.Vars["max_loops"])
	}
	if v, ok := def.Vars["extra_var"].(string); !ok || v != "hello" {
		t.Errorf("Vars[extra_var] = %v, want %q", def.Vars["extra_var"], "hello")
	}

	// Nodes: base + overlay appended
	if len(def.Nodes) != 3 {
		t.Fatalf("len(Nodes) = %d, want 3", len(def.Nodes))
	}
	if def.Nodes[0].Name != "step-a" {
		t.Errorf("Nodes[0].Name = %q, want %q", def.Nodes[0].Name, "step-a")
	}
	if def.Nodes[2].Name != "step-c" {
		t.Errorf("Nodes[2].Name = %q, want %q", def.Nodes[2].Name, "step-c")
	}

	// Edges: step-b-done replaced by overlay version
	found := false
	for _, e := range def.Edges {
		if e.ID == "step-b-done" {
			found = true
			if e.To != "step-c" {
				t.Errorf("edge step-b-done.To = %q, want %q (replaced by overlay)", e.To, "step-c")
			}
			if e.When != `output.needs_extra == true` {
				t.Errorf("edge step-b-done.When = %q, want overlay value", e.When)
			}
		}
	}
	if !found {
		t.Errorf("edge step-b-done not found in merged edges")
	}
}

func TestLoadCircuitWithOverlay_CannotOverrideBaseNode(t *testing.T) {
	base := []byte(`
circuit: base
nodes:
  - name: alpha
    handler: a
    edges:
      - to: DONE
start: alpha
done: DONE
`)
	overlay := []byte(`
import: base
circuit: overlay
nodes:
  - name: alpha
    handler: b
start: alpha
done: DONE
`)

	resolver := func(name string) ([]byte, error) {
		return base, nil
	}

	_, err := LoadCircuitWithOverlay(overlay, resolver)
	if err == nil {
		t.Fatal("expected error for overriding base node, got nil")
	}
}

func TestLoadCircuitWithOverlay_NilResolverErrors(t *testing.T) {
	overlay := []byte(`
import: something
circuit: overlay
nodes:
  - name: a
    handler: a
edges:
  - id: E1
    from: a
    to: DONE
start: a
done: DONE
`)

	_, err := LoadCircuitWithOverlay(overlay, nil)
	if err == nil {
		t.Fatal("expected error when resolver is nil, got nil")
	}
}

func TestLoadCircuitWithOverlay_RCASchematic(t *testing.T) {
	base, err := os.ReadFile("schematics/rca/circuit.yaml")
	if err != nil {
		t.Skipf("RCA schematic circuit not found: %v", err)
	}

	overlay := []byte(`
import: rca
circuit: rca

zones:
  analysis:
    nodes: [gather-code, resolve, investigate]

nodes:
  - name: gather-code
    description: "Gather code context from source repositories via GND circuit"
    handler_type: circuit
    handler: gnd
    approach: methodical
    before: [inject.code-keywords]
    after: [bridge.code-context]
    edges:
      - name: gather-code-resolve
        to: resolve
        when: "true"

edges:
  - id: triage-investigate
    name: triage-investigate
    from: triage
    to: gather-code
    when: "output.skip_investigation != true"

start: recall
done: DONE
`)

	resolver := func(name string) ([]byte, error) {
		if name == "rca" {
			return base, nil
		}
		return nil, os.ErrNotExist
	}

	def, err := LoadCircuitWithOverlay(overlay, resolver)
	if err != nil {
		t.Fatalf("LoadCircuitWithOverlay: %v", err)
	}

	if def.Circuit != "rca" {
		t.Errorf("Circuit = %q, want %q", def.Circuit, "rca")
	}

	nodeNames := make(map[string]bool)
	for _, n := range def.Nodes {
		nodeNames[n.Name] = true
	}

	for _, want := range []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report", "gather-code"} {
		if !nodeNames[want] {
			t.Errorf("missing expected node %q", want)
		}
	}

	// Analysis zone should include gather-code from overlay
	if z, ok := def.Zones["analysis"]; ok {
		zoneNodes := make(map[string]bool)
		for _, n := range z.Nodes {
			zoneNodes[n] = true
		}
		if !zoneNodes["gather-code"] {
			t.Errorf("analysis zone missing gather-code node")
		}
	} else {
		t.Error("analysis zone not found")
	}

	// triage-investigate edge should point to gather-code (overlay override)
	for _, e := range def.Edges {
		if e.ID == "triage-investigate" {
			if e.To != "gather-code" {
				t.Errorf("triage-investigate.To = %q, want %q", e.To, "gather-code")
			}
		}
	}
}

// --- Vocabulary auto-registration (ORG-TSK-134) ---

func TestLoadCircuit_CodeDisplayName_Parsed(t *testing.T) {
	data := []byte(`
circuit: vocab-test
nodes:
  - name: recall
    code: F0
    display_name: Recall
  - name: triage
    code: F1
    display_name: Triage
  - name: plain
    handler: plain
edges:
  - id: recall-triage
    name: proceed
    from: recall
    to: triage
    display_name: Proceed to triage
  - id: triage-done
    from: triage
    to: _done
start: recall
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if def.Nodes[0].Code != "F0" || def.Nodes[0].DisplayName != "Recall" {
		t.Errorf("Nodes[0] Code=%q DisplayName=%q, want F0/Recall", def.Nodes[0].Code, def.Nodes[0].DisplayName)
	}
	if def.Nodes[1].Code != "F1" || def.Nodes[1].DisplayName != "Triage" {
		t.Errorf("Nodes[1] Code=%q DisplayName=%q, want F1/Triage", def.Nodes[1].Code, def.Nodes[1].DisplayName)
	}
	if def.Nodes[2].Code != "" || def.Nodes[2].DisplayName != "" {
		t.Errorf("Nodes[2] should have empty Code/DisplayName, got %q/%q", def.Nodes[2].Code, def.Nodes[2].DisplayName)
	}
	if def.Edges[0].DisplayName != "Proceed to triage" {
		t.Errorf("Edges[0].DisplayName = %q, want %q", def.Edges[0].DisplayName, "Proceed to triage")
	}
}

func TestLoadCircuit_RegisterVocabulary(t *testing.T) {
	def := &CircuitDef{
		Circuit: "vocab",
		Nodes: []NodeDef{
			{Name: "recall", Code: "F0", DisplayName: "Recall"},
			{Name: "triage", Code: "F1", DisplayName: "Triage"},
			{Name: "plain"},
		},
		Edges: []EdgeDef{
			{ID: "recall-triage", From: "recall", To: "triage", DisplayName: "Proceed to triage"},
			{ID: "triage-done", From: "triage", To: "_done"},
		},
		Start: "recall",
		Done:  "_done",
	}
	v := NewRichMapVocabulary()
	def.RegisterVocabulary(v)

	// F0 → Recall
	if name := v.Name("F0"); name != "Recall" {
		t.Errorf("v.Name(F0) = %q, want Recall", name)
	}
	// F0_RECALL alias
	if name := v.Name("F0_RECALL"); name != "Recall" {
		t.Errorf("v.Name(F0_RECALL) = %q, want Recall", name)
	}
	// recall node name → Recall
	if name := v.Name("recall"); name != "Recall" {
		t.Errorf("v.Name(recall) = %q, want Recall", name)
	}
	// Edge ID → DisplayName
	if name := v.Name("recall-triage"); name != "Proceed to triage" {
		t.Errorf("v.Name(recall-triage) = %q, want Proceed to triage", name)
	}
	// triage-done has no DisplayName, should pass through
	if name := v.Name("triage-done"); name != "triage-done" {
		t.Errorf("v.Name(triage-done) = %q, want pass-through", name)
	}
}

// --- Node output schema (ORG-TSK-135) ---

func TestLoadCircuit_OutputFields_Parsed(t *testing.T) {
	data := []byte(`
circuit: output-test
nodes:
  - name: extract
    handler: extract
    output:
      - name: result
        type: string
        required: true
      - name: score
        type: float
      - name: tags
        type: array
  - name: plain
    handler: plain
edges:
  - id: E1
    from: extract
    to: plain
  - id: E2
    from: plain
    to: _done
start: extract
done: _done
`)
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	out := def.Nodes[0].OutputFields()
	if len(out) != 3 {
		t.Fatalf("len(OutputFields) = %d, want 3", len(out))
	}
	if out[0].Name != "result" || out[0].Type != "string" || !out[0].Required {
		t.Errorf("Output[0] = %+v, want result/string/required", out[0])
	}
	if out[1].Name != "score" || out[1].Type != "float" || out[1].Required {
		t.Errorf("Output[1] = %+v, want score/float/optional", out[1])
	}
	if def.Nodes[1].OutputFields() != nil {
		t.Errorf("plain node should have nil OutputFields, got %v", def.Nodes[1].OutputFields())
	}
}

func TestNodeDef_ValidateOutput(t *testing.T) {
	tests := []struct {
		name    string
		nd      NodeDef
		output  map[string]any
		wantErr bool
	}{
		{
			name:    "no schema",
			nd:      NodeDef{Name: "a"},
			output:  map[string]any{},
			wantErr: false,
		},
		{
			name: "valid required",
			nd: NodeDef{
				Name: "a",
				Output: []OutputField{
					{Name: "x", Type: "string", Required: true},
				},
			},
			output:  map[string]any{"x": "hello"},
			wantErr: false,
		},
		{
			name: "missing required",
			nd: NodeDef{
				Name: "a",
				Output: []OutputField{
					{Name: "x", Type: "string", Required: true},
				},
			},
			output:  map[string]any{},
			wantErr: true,
		},
		{
			name: "wrong type",
			nd: NodeDef{
				Name: "a",
				Output: []OutputField{
					{Name: "x", Type: "string", Required: true},
				},
			},
			output:  map[string]any{"x": 42},
			wantErr: true,
		},
		{
			name: "int accepts float64",
			nd: NodeDef{
				Name: "a",
				Output: []OutputField{
					{Name: "n", Type: "int", Required: true},
				},
			},
			output:  map[string]any{"n": float64(42)},
			wantErr: false,
		},
		{
			name: "valid array",
			nd: NodeDef{
				Name: "a",
				Output: []OutputField{
					{Name: "arr", Type: "array", Required: true},
				},
			},
			output:  map[string]any{"arr": []any{"a", "b"}},
			wantErr: false,
		},
		{
			name: "valid object",
			nd: NodeDef{
				Name: "a",
				Output: []OutputField{
					{Name: "obj", Type: "object", Required: true},
				},
			},
			output:  map[string]any{"obj": map[string]any{"k": "v"}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.nd.ValidateOutput(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
