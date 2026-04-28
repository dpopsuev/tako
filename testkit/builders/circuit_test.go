package builders_test

import (
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/testkit/builders"
)

func TestCircuitDefBuilder_Basic(t *testing.T) {
	def := builders.NewCircuitDef("test").
		AddNode("A", "handler-a").
		AddNode("B", "handler-b").
		AddEdge("A", "B", "true").
		Start("A").Done("DONE").
		Build()

	if def.Circuit != "test" {
		t.Errorf("Circuit = %q, want %q", def.Circuit, "test")
	}
	if len(def.Nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(def.Nodes))
	}
	if def.Nodes[0].Name != "A" || def.Nodes[0].Action != "handler-a" {
		t.Errorf("Nodes[0] = {%q, %q}, want {A, handler-a}", def.Nodes[0].Name, def.Nodes[0].Action)
	}
	if def.Nodes[1].Name != "B" || def.Nodes[1].Action != "handler-b" {
		t.Errorf("Nodes[1] = {%q, %q}, want {B, handler-b}", def.Nodes[1].Name, def.Nodes[1].Action)
	}
	if len(def.Edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(def.Edges))
	}
	if def.Edges[0].From != "A" || def.Edges[0].To != "B" || def.Edges[0].When != "true" {
		t.Errorf("edge = {from:%q, to:%q, when:%q}, want {A, B, true}", def.Edges[0].From, def.Edges[0].To, def.Edges[0].When)
	}
	if def.Start != "A" {
		t.Errorf("Start = %q, want %q", def.Start, "A")
	}
	if def.Done != "DONE" {
		t.Errorf("Done = %q, want %q", def.Done, "DONE")
	}
}

func TestCircuitDefBuilder_Validate(t *testing.T) {
	def := builders.NewCircuitDef("test").
		AddNode("A", "handler-a").
		AddNode("B", "handler-b").
		AddEdge("A", "B", "true").
		Start("A").Done("DONE").
		Build()

	if err := def.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestCircuitDefBuilder_WithVar(t *testing.T) {
	def := builders.NewCircuitDef("test").
		WithVar("key", "value").
		WithVar("count", 42).
		AddNode("A", "a").
		AddEdge("A", "DONE", "true").
		Start("A").Done("DONE").
		Build()

	if def.Vars["key"] != "value" {
		t.Errorf("Vars[key] = %v, want %q", def.Vars["key"], "value")
	}
	if def.Vars["count"] != 42 {
		t.Errorf("Vars[count] = %v, want %d", def.Vars["count"], 42)
	}
}

func TestCircuitDefBuilder_AddNodeDef(t *testing.T) {
	nd := circuit.NodeDef{
		Name:   "custom",
		Action: "h",
		Prompt: "Do the thing",
	}
	def := builders.NewCircuitDef("test").
		AddNodeDef(&nd).
		AddEdge("custom", "DONE", "true").
		Start("custom").Done("DONE").
		Build()

	if len(def.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(def.Nodes))
	}
	if def.Nodes[0].Prompt != "Do the thing" {
		t.Errorf("Prompt = %q, want %q", def.Nodes[0].Prompt, "Do the thing")
	}
}

func TestCircuitDefBuilder_AddEdgeDef(t *testing.T) {
	ed := circuit.EdgeDef{
		ID:   "custom-edge",
		From: "A",
		To:   "B",
		Loop: true,
	}
	def := builders.NewCircuitDef("test").
		AddNode("A", "a").
		AddNode("B", "b").
		AddEdgeDef(&ed).
		Start("A").Done("DONE").
		Build()

	if len(def.Edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(def.Edges))
	}
	if !def.Edges[0].Loop {
		t.Error("expected Loop = true")
	}
	if def.Edges[0].ID != "custom-edge" {
		t.Errorf("ID = %q, want %q", def.Edges[0].ID, "custom-edge")
	}
}

func TestCircuitDefBuilder_BuildReturnsCopy(t *testing.T) {
	b := builders.NewCircuitDef("test").
		AddNode("A", "a").
		AddEdge("A", "DONE", "true").
		Start("A").Done("DONE")

	def1 := b.Build()
	def2 := b.Build()

	// Mutating one should not affect the other.
	def1.Circuit = "modified"
	if def2.Circuit == "modified" {
		t.Error("Build should return independent copies")
	}
}
