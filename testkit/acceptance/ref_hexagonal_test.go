package acceptance

import (
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
)

func TestRefHexagonal_LoadsAndRuns(t *testing.T) {
	tc := &engine.TraceCollector{}
	err := runFixture(t, "circuits/ref-hexagonal.yaml", nil, engine.WithRunObserver(tc))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	enters := tc.EventsOfType(circuit.EventNodeEnter)
	if len(enters) != 3 {
		t.Fatalf("node_enter events = %d, want 3", len(enters))
	}

	want := []string{"ingest", "process", "output"}
	for i, e := range enters {
		if e.Node != want[i] {
			t.Errorf("node[%d] = %q, want %q", i, e.Node, want[i])
		}
	}
}

func TestRefHexagonal_ProducesThreeNodeExits(t *testing.T) {
	tc := &engine.TraceCollector{}
	err := runFixture(t, "circuits/ref-hexagonal.yaml", nil, engine.WithRunObserver(tc))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	exits := tc.EventsOfType(circuit.EventNodeExit)
	if len(exits) != 3 {
		t.Fatalf("node_exit events = %d, want 3", len(exits))
	}
}
