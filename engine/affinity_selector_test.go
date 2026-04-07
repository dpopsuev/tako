package engine

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
)

type affinityStubNode struct {
	name    string
	element roster.Element
}

func (n *affinityStubNode) Name() string                    { return n.name }
func (n *affinityStubNode) ElementAffinity() roster.Element { return n.element }
func (n *affinityStubNode) Process(_ context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	return nil, nil
}

func makeAffinityWalker(name string, element roster.Element, stepAffinity map[string]float64) circuit.Walker {
	w := circuit.NewProcessWalker(name)
	id := roster.AgentIdentity{
		Name:         name,
		Element:      element,
		StepAffinity: stepAffinity,
	}
	w.SetIdentity(&id)
	return w
}

func TestAffinitySelector_SingleWalker(t *testing.T) {
	sel := &AffinitySelector{}
	w := makeAffinityWalker("solo", roster.ElementEarth, nil)
	node := &affinityStubNode{name: "triage", element: roster.ElementEarth}

	result := sel.SelectWalker(node, []circuit.Walker{w}, nil)
	if result == nil {
		t.Fatal("expected non-nil walker")
	}
}

func TestAffinitySelector_EmptyWalkers(t *testing.T) {
	sel := &AffinitySelector{}
	node := &affinityStubNode{name: "triage"}

	result := sel.SelectWalker(node, nil, nil)
	if result != nil {
		t.Error("expected nil for empty walkers")
	}
}

func TestAffinitySelector_PicksByStepAffinity(t *testing.T) {
	sel := &AffinitySelector{}
	low := makeAffinityWalker("low", "", map[string]float64{"investigate": 0.2})
	high := makeAffinityWalker("high", "", map[string]float64{"investigate": 0.9})
	node := &affinityStubNode{name: "investigate"}

	result := sel.SelectWalker(node, []circuit.Walker{low, high}, nil)
	if result.Identity().Name != "high" {
		t.Errorf("expected high affinity walker, got %s", result.Identity().Name)
	}
}

func TestAffinitySelector_ElementBreaksTie(t *testing.T) {
	sel := &AffinitySelector{}
	noMatch := makeAffinityWalker("no-match", roster.ElementWater, map[string]float64{"triage": 0.5})
	match := makeAffinityWalker("match", roster.ElementFire, map[string]float64{"triage": 0.5})
	node := &affinityStubNode{name: "triage", element: roster.ElementFire}

	result := sel.SelectWalker(node, []circuit.Walker{noMatch, match}, nil)
	if result.Identity().Name != "match" {
		t.Errorf("expected element match to break tie, got %s", result.Identity().Name)
	}
}

func TestAffinitySelector_LastMismatch(t *testing.T) {
	sel := &AffinitySelector{}
	w := makeAffinityWalker("perfect", roster.ElementEarth, map[string]float64{"triage": 1.0})
	node := &affinityStubNode{name: "triage", element: roster.ElementEarth}

	sel.SelectWalker(node, []circuit.Walker{w}, nil)
	if sel.LastMismatch() != 0.0 {
		t.Errorf("mismatch = %f, want 0.0 for perfect match", sel.LastMismatch())
	}
}
