package engine

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

type affinityWalker struct {
	identity circuit.AgentIdentity
	state    *circuit.WalkerState
}

func (w *affinityWalker) Identity() circuit.AgentIdentity       { return w.identity }
func (w *affinityWalker) SetIdentity(id *circuit.AgentIdentity) { w.identity = *id }
func (w *affinityWalker) State() *circuit.WalkerState           { return w.state }
func (w *affinityWalker) Handle(_ context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	return node.Process(context.Background(), nc)
}

func TestSingleScheduler(t *testing.T) {
	w := &affinityWalker{identity: circuit.AgentIdentity{PersonaName: "solo"}, state: circuit.NewWalkerState("s1")}
	sched := &SingleScheduler{Walker: w}

	node := &stubNode{name: "classify", element: circuit.ElementFire}
	got := sched.Select(SchedulerContext{Node: node, Walkers: []circuit.Walker{w}})
	if got.Identity().PersonaName != "solo" {
		t.Errorf("expected solo, got %s", got.Identity().PersonaName)
	}
}

func TestAffinityScheduler_PicksHighestAffinity(t *testing.T) {
	herald := &affinityWalker{
		identity: circuit.AgentIdentity{
			PersonaName:  "Herald",
			Element:      circuit.ElementFire,
			StepAffinity: map[string]float64{"classify": 0.9, "investigate": 0.2},
		},
		state: circuit.NewWalkerState("h1"),
	}
	seeker := &affinityWalker{
		identity: circuit.AgentIdentity{
			PersonaName:  "Seeker",
			Element:      circuit.ElementWater,
			StepAffinity: map[string]float64{"classify": 0.2, "investigate": 0.9},
		},
		state: circuit.NewWalkerState("s1"),
	}

	sched := &AffinityScheduler{}
	walkers := []circuit.Walker{herald, seeker}

	classifyNode := &stubNode{name: "classify", element: circuit.ElementFire}
	got := sched.Select(SchedulerContext{Node: classifyNode, Walkers: walkers})
	if got.Identity().PersonaName != "Herald" {
		t.Errorf("classify: expected Herald, got %s", got.Identity().PersonaName)
	}

	investigateNode := &stubNode{name: "investigate", element: circuit.ElementWater}
	got = sched.Select(SchedulerContext{Node: investigateNode, Walkers: walkers})
	if got.Identity().PersonaName != "Seeker" {
		t.Errorf("investigate: expected Seeker, got %s", got.Identity().PersonaName)
	}
}

func TestAffinityScheduler_TieBreakByElement(t *testing.T) {
	fire := &affinityWalker{
		identity: circuit.AgentIdentity{
			PersonaName:  "Fire",
			Element:      circuit.ElementFire,
			StepAffinity: map[string]float64{"node": 0.5},
		},
		state: circuit.NewWalkerState("f1"),
	}
	water := &affinityWalker{
		identity: circuit.AgentIdentity{
			PersonaName:  "Water",
			Element:      circuit.ElementWater,
			StepAffinity: map[string]float64{"node": 0.5},
		},
		state: circuit.NewWalkerState("w1"),
	}

	sched := &AffinityScheduler{}
	fireNode := &stubNode{name: "node", element: circuit.ElementFire}

	got := sched.Select(SchedulerContext{Node: fireNode, Walkers: []circuit.Walker{water, fire}})
	if got.Identity().PersonaName != "Fire" {
		t.Errorf("expected Fire (element tiebreak), got %s", got.Identity().PersonaName)
	}
}

func TestAffinityScheduler_FallbackToFirst(t *testing.T) {
	w1 := &affinityWalker{
		identity: circuit.AgentIdentity{PersonaName: "First"},
		state:    circuit.NewWalkerState("1"),
	}
	w2 := &affinityWalker{
		identity: circuit.AgentIdentity{PersonaName: "Second"},
		state:    circuit.NewWalkerState("2"),
	}

	sched := &AffinityScheduler{}
	node := &stubNode{name: "unknown"}

	got := sched.Select(SchedulerContext{Node: node, Walkers: []circuit.Walker{w1, w2}})
	if got.Identity().PersonaName != "First" {
		t.Errorf("expected First (fallback), got %s", got.Identity().PersonaName)
	}
}

func TestAffinityScheduler_SingleWalker(t *testing.T) {
	w := &affinityWalker{
		identity: circuit.AgentIdentity{PersonaName: "Only"},
		state:    circuit.NewWalkerState("o1"),
	}

	sched := &AffinityScheduler{}
	got := sched.Select(SchedulerContext{Node: &stubNode{name: "x"}, Walkers: []circuit.Walker{w}})
	if got.Identity().PersonaName != "Only" {
		t.Errorf("expected Only, got %s", got.Identity().PersonaName)
	}
}

func TestAffinityScheduler_EmptyWalkers(t *testing.T) {
	sched := &AffinityScheduler{}
	got := sched.Select(SchedulerContext{Node: &stubNode{name: "x"}, Walkers: nil})
	if got != nil {
		t.Errorf("expected nil for empty walkers, got %v", got)
	}
}

func TestAffinityScheduler_Mismatch_PerfectMatch(t *testing.T) {
	w := &affinityWalker{
		identity: circuit.AgentIdentity{
			PersonaName:  "Perfect",
			Element:      circuit.ElementFire,
			StepAffinity: map[string]float64{"node": 1.0},
		},
		state: circuit.NewWalkerState("p1"),
	}
	sched := &AffinityScheduler{}
	node := &stubNode{name: "node", element: circuit.ElementFire}
	sched.Select(SchedulerContext{Node: node, Walkers: []circuit.Walker{w}})

	if sched.LastMismatch() != 0.0 {
		t.Errorf("perfect match should have mismatch 0.0, got %f", sched.LastMismatch())
	}
}

func TestAffinityScheduler_Mismatch_WorstCase(t *testing.T) {
	w := &affinityWalker{
		identity: circuit.AgentIdentity{PersonaName: "Worst"},
		state:    circuit.NewWalkerState("w1"),
	}
	sched := &AffinityScheduler{}
	node := &stubNode{name: "node", element: circuit.ElementFire}
	sched.Select(SchedulerContext{Node: node, Walkers: []circuit.Walker{w}})

	if sched.LastMismatch() < 0.5 {
		t.Errorf("no affinity + wrong element should have high mismatch, got %f", sched.LastMismatch())
	}
}

func TestZoneForNode(t *testing.T) {
	zones := []Zone{
		{Name: "front", NodeNames: []string{"A", "B"}},
		{Name: "back", NodeNames: []string{"C"}},
	}

	z := zoneForNode("B", zones)
	if z == nil || z.Name != "front" {
		t.Errorf("expected zone 'front' for node B, got %v", z)
	}

	z = zoneForNode("C", zones)
	if z == nil || z.Name != "back" {
		t.Errorf("expected zone 'back' for node C, got %v", z)
	}

	z = zoneForNode("Z", zones)
	if z != nil {
		t.Errorf("expected nil for unknown node Z, got %v", z)
	}
}
