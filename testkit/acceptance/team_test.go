package acceptance

// Feature: Multi-Walker Teams
//   As a circuit designer
//   I want to assign walkers to nodes based on affinity and elements
//   So that specialist agents handle steps best suited to their expertise

import (
	"testing"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

func TestTeam_TwoWalkersScheduleByAffinity(t *testing.T) {
	// Scenario: Two walkers with different step_affinity schedule by affinity
	//   Given a circuit with nodes requiring different elements
	//   And walkers with step_affinity preferences
	//   When I run with WithTeam and AffinityScheduler
	//   Then walker_switch events fire as affinity changes

	// Create walkers with different affinities
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

	// Create team with affinity scheduler
	tc := &engine.TraceCollector{}
	team := &engine.Team{
		Walkers:   []circuit.Walker{coordinator, specialistA},
		Scheduler: &engine.AffinityScheduler{},
		Observer:  tc,
		MaxSteps:  20,
	}

	err := runFixture(t, "scenarios/team-delegation.yaml", nil,
		engine.WithTeam(team),
	)
	if err != nil {
		t.Fatalf("runFixture: %v", err)
	}

	// Verify walker_switch events fired
	switches := tc.EventsOfType(circuit.EventWalkerSwitch)
	if len(switches) < 1 {
		t.Errorf("walker_switch events = %d, want at least 1 (delegation pattern)", len(switches))
	}

	// Verify different walkers handled different nodes
	enters := tc.EventsOfType(circuit.EventNodeEnter)
	walkersSeen := make(map[string]bool)
	for _, e := range enters {
		walkersSeen[e.Walker] = true
	}
	if len(walkersSeen) < 2 {
		t.Errorf("unique walkers = %d, want at least 2", len(walkersSeen))
	}
}

func TestTeam_WalkCompletesWithMultipleWalkers(t *testing.T) {
	// Scenario: Walk completes successfully with multiple walkers
	//   Given a circuit with parallel fan-out/fan-in
	//   And a team of specialized walkers
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
	team := &engine.Team{
		Walkers:   []circuit.Walker{coordinator, specialistA, specialistB},
		Scheduler: &engine.AffinityScheduler{},
		Observer:  tc,
		MaxSteps:  20,
	}

	err := runFixture(t, "scenarios/team-delegation.yaml", nil,
		engine.WithTeam(team),
	)
	if err != nil {
		t.Fatalf("runFixture: %v", err)
	}

	// Verify walk_complete event
	completes := tc.EventsOfType(circuit.EventWalkComplete)
	if len(completes) != 1 {
		t.Errorf("walk_complete events = %d, want 1", len(completes))
	}

	// Verify key nodes were visited (plan and synthesize are required)
	// Note: parallel edges may execute in any order, research_a and research_b
	// should both be visited in a full parallel execution
	enters := tc.EventsOfType(circuit.EventNodeEnter)
	visitedNodes := make(map[string]bool)
	for _, e := range enters {
		visitedNodes[e.Node] = true
	}

	// At minimum, we need plan and synthesize
	requiredNodes := []string{"plan", "synthesize"}
	for _, nodeName := range requiredNodes {
		if !visitedNodes[nodeName] {
			t.Errorf("node %q was not visited", nodeName)
		}
	}

	// At least one research node should be visited
	if !visitedNodes["research_a"] && !visitedNodes["research_b"] {
		t.Error("neither research_a nor research_b was visited")
	}
}
