package acceptance

// Feature: Multi-Walker Collectives
//   As a circuit designer
//   I want to assign walkers to nodes based on affinity and elements
//   So that specialist agents handle steps best suited to their expertise

import (
	"testing"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

func TestCollective_TwoWalkersScheduleByAffinity(t *testing.T) {
	// Scenario: Two walkers with different step_affinity schedule by affinity
	//   Given a circuit with nodes requiring different elements
	//   And walkers with step_affinity preferences
	//   When I run with WithCollective and AffinitySelector
	//   Then walker_switch events fire as affinity changes

	coordinator := circuit.NewProcessWalker("coordinator")
	coordinator.SetIdentity(&agentport.AgentIdentity{
		Name:         "Coordinator",
		Element:      agentport.ElementEarth,
		StepAffinity: map[string]float64{"plan": 0.95, "synthesize": 0.95},
	})

	specialistA := circuit.NewProcessWalker("specialist-a")
	specialistA.SetIdentity(&agentport.AgentIdentity{
		Name:         "Specialist A",
		Element:      agentport.ElementWater,
		StepAffinity: map[string]float64{"research_a": 0.95},
	})

	tc := &engine.TraceCollector{}
	err := runFixture(t, "scenarios/team-delegation.yaml", nil,
		engine.WithRunObserver(tc),
		engine.WithCollective(
			[]circuit.Walker{coordinator, specialistA},
			&engine.AffinitySelector{},
			engine.WithCollectiveObserver(tc),
			engine.WithMaxSteps(20),
		),
	)
	if err != nil {
		t.Fatalf("runFixture: %v", err)
	}

	switches := tc.EventsOfType(circuit.EventWalkerSwitch)
	if len(switches) < 1 {
		t.Errorf("walker_switch events = %d, want at least 1 (delegation pattern)", len(switches))
	}

	walkersSeen := make(map[string]bool)
	for _, e := range switches {
		walkersSeen[e.Walker] = true
	}
	if len(walkersSeen) < 2 {
		t.Errorf("unique walkers in switches = %d, want at least 2", len(walkersSeen))
	}
}

func TestCollective_WalkCompletesWithMultipleWalkers(t *testing.T) {
	// Scenario: Walk completes successfully with multiple walkers
	//   Given a circuit with parallel fan-out/fan-in
	//   And a collective of specialized walkers
	//   When I run the circuit
	//   Then walk_complete event fires
	//   And all nodes are visited

	coordinator := circuit.NewProcessWalker("coordinator")
	coordinator.SetIdentity(&agentport.AgentIdentity{
		Name:         "Coordinator",
		Element:      agentport.ElementEarth,
		StepAffinity: map[string]float64{"plan": 0.95, "synthesize": 0.95},
	})

	specialistA := circuit.NewProcessWalker("specialist-a")
	specialistA.SetIdentity(&agentport.AgentIdentity{
		Name:         "Specialist A",
		Element:      agentport.ElementWater,
		StepAffinity: map[string]float64{"research_a": 0.95},
	})

	specialistB := circuit.NewProcessWalker("specialist-b")
	specialistB.SetIdentity(&agentport.AgentIdentity{
		Name:         "Specialist B",
		Element:      agentport.ElementFire,
		StepAffinity: map[string]float64{"research_b": 0.95},
	})

	tc := &engine.TraceCollector{}
	err := runFixture(t, "scenarios/team-delegation.yaml", nil,
		engine.WithRunObserver(tc),
		engine.WithCollective(
			[]circuit.Walker{coordinator, specialistA, specialistB},
			&engine.AffinitySelector{},
			engine.WithCollectiveObserver(tc),
			engine.WithMaxSteps(20),
		),
	)
	if err != nil {
		t.Fatalf("runFixture: %v", err)
	}

	completes := tc.EventsOfType(circuit.EventWalkComplete)
	if len(completes) != 1 {
		t.Errorf("walk_complete events = %d, want 1", len(completes))
	}

	enters := tc.EventsOfType(circuit.EventNodeEnter)
	visitedNodes := make(map[string]bool)
	for _, e := range enters {
		visitedNodes[e.Node] = true
	}

	requiredNodes := []string{"plan", "synthesize"}
	for _, nodeName := range requiredNodes {
		if !visitedNodes[nodeName] {
			t.Errorf("node %q was not visited", nodeName)
		}
	}

	if !visitedNodes["research_a"] && !visitedNodes["research_b"] {
		t.Error("neither research_a nor research_b was visited")
	}
}
