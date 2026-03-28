package engine

import (
	"context"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

func findingTestNode(name string, confidence float64, raw any) func(circuit.NodeDef) circuit.Node {
	return func(_ circuit.NodeDef) circuit.Node {
		return &stubNode{name: name, artifact: &stubArtifact{typ: "test", confidence: confidence, raw: raw}}
	}
}

func findingEnforcerNode(name string, collector circuit.FindingCollector, f *circuit.Finding) func(circuit.NodeDef) circuit.Node {
	return func(_ circuit.NodeDef) circuit.Node {
		return &findingEnforcerNodeImpl{name: name, collector: collector, finding: f}
	}
}

type findingEnforcerNodeImpl struct {
	name      string
	collector circuit.FindingCollector
	finding   *circuit.Finding
}

func (n *findingEnforcerNodeImpl) Name() string                { return n.name }
func (n *findingEnforcerNodeImpl) ElementAffinity() circuit.Element { return "" }
func (n *findingEnforcerNodeImpl) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	if n.finding != nil {
		if c, ok := nc.WalkerState.Context[circuit.FindingCollectorKey].(circuit.FindingCollector); ok {
			_ = c.Report(ctx, n.finding)
		}
	}
	return &stubArtifact{typ: "enforcer", confidence: 1.0, raw: "checked"}, nil
}

func TestArtifactStore(t *testing.T) {
	s := &ArtifactStore{}

	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}

	a := &stubArtifact{typ: "test", confidence: 0.9, raw: "data"}
	s.Set("nodeA", a)

	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1", s.Len())
	}
	if got := s.Get("nodeA"); got != a {
		t.Error("Get(nodeA) returned wrong artifact")
	}
	if got := s.Get("missing"); got != nil {
		t.Error("Get(missing) should return nil")
	}

	all := s.All()
	if len(all) != 1 {
		t.Errorf("All() len = %d, want 1", len(all))
	}
}

func TestArtifactCaptureObserver(t *testing.T) {
	store := &ArtifactStore{}
	obs := &artifactCaptureObserver{store: store}

	a := &stubArtifact{typ: "test", confidence: 0.8, raw: "data"}
	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "nodeA", Artifact: a})
	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "nodeB"})

	if store.Len() != 1 {
		t.Errorf("store.Len() = %d, want 1 (only EventNodeExit captured)", store.Len())
	}
	if store.Get("nodeA") != a {
		t.Error("nodeA artifact not captured")
	}
}

func TestArtifactCaptureObserver_FilteredNodes(t *testing.T) {
	store := &ArtifactStore{}
	obs := &artifactCaptureObserver{
		store:         store,
		observedNodes: map[string]bool{"nodeA": true},
	}

	a1 := &stubArtifact{typ: "test", confidence: 0.8, raw: "a"}
	a2 := &stubArtifact{typ: "test", confidence: 0.9, raw: "b"}
	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "nodeA", Artifact: a1})
	obs.OnEvent(&circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "nodeB", Artifact: a2})

	if store.Len() != 1 {
		t.Errorf("store.Len() = %d, want 1 (only observed nodeA)", store.Len())
	}
}

func twoNodeCircuit() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit:     "test-work",
		HandlerType: "node",
		Nodes: []circuit.NodeDef{
			{Name: "step1"},
			{Name: "step2"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "step1", To: "step2"},
		},
		Start: "step1",
		Done:  "step2",
	}
}

func twoNodeEnforcerCircuit() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit:     "test-enforcer",
		HandlerType: "node",
		Nodes: []circuit.NodeDef{
			{Name: "check"},
			{Name: "report"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e-check", From: "check", To: "report"},
		},
		Start: "check",
		Done:  "report",
	}
}

func TestRunWithEnforcer_WorkAndEnforcerBothRun(t *testing.T) {
	workDef := twoNodeCircuit()
	enforcerDef := twoNodeEnforcerCircuit()

	workReg := &GraphRegistries{
		Nodes: NodeRegistry{
			"step1": findingTestNode("step1", 0.9, map[string]any{"result": "ok"}),
			"step2": findingTestNode("step2", 0.95, map[string]any{"result": "done"}),
		},
	}

	router := NewFindingRouter(nil, FindingHandlers{})
	enforcerReg := &GraphRegistries{
		Nodes: NodeRegistry{
			"check": findingEnforcerNode("check", router, &circuit.Finding{
				Severity: circuit.FindingWarning,
				Domain:   "test.quality",
				Source:   "quality-checker",
				NodeName: "step1",
				Message:  "low coverage",
			}),
			"report": findingTestNode("report", 1.0, "report-done"),
		},
	}

	findings, err := RunWithEnforcer(
		context.Background(),
		workDef,
		workReg,
		&ParallelEnforcerConfig{
			EnforcerDef: enforcerDef,
			Registries:  enforcerReg,
			Router:      router,
		},
	)

	if err != nil {
		t.Fatalf("RunWithEnforcer error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Severity != circuit.FindingWarning {
		t.Errorf("finding severity = %q, want %q", findings[0].Severity, circuit.FindingWarning)
	}
}

func TestRunWithEnforcer_EnforcerCancelledOnWorkComplete(t *testing.T) {
	workDef := twoNodeCircuit()

	enforcerDef := &circuit.CircuitDef{
		Circuit:     "test-enforcer",
		HandlerType: "node",
		Nodes: []circuit.NodeDef{
			{Name: "slow-check"},
			{Name: "done"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e-slow", From: "slow-check", To: "done"},
		},
		Start: "slow-check",
		Done:  "done",
	}

	workReg := &GraphRegistries{
		Nodes: NodeRegistry{
			"step1": findingTestNode("step1", 0.9, "done"),
			"step2": findingTestNode("step2", 0.95, "done"),
		},
	}

	enforcerReg := &GraphRegistries{
		Nodes: NodeRegistry{
			"slow-check": func(_ circuit.NodeDef) circuit.Node {
				return &slowNode{name: "slow-check", duration: 2 * time.Second}
			},
			"done": findingTestNode("done", 1.0, "done"),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err := RunWithEnforcer(ctx, workDef, workReg, &ParallelEnforcerConfig{
		EnforcerDef: enforcerDef,
		Registries:  enforcerReg,
	})

	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RunWithEnforcer error: %v", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("RunWithEnforcer took %v; enforcer should have been cancelled quickly", elapsed)
	}
}

func TestRunWithEnforcer_DefaultRouter(t *testing.T) {
	workDef := twoNodeCircuit()
	enforcerDef := twoNodeEnforcerCircuit()

	workReg := &GraphRegistries{
		Nodes: NodeRegistry{
			"step1": findingTestNode("step1", 0.9, "ok"),
			"step2": findingTestNode("step2", 0.95, "ok"),
		},
	}

	enforcerReg := &GraphRegistries{
		Nodes: NodeRegistry{
			"check":  findingEnforcerNode("check", nil, nil),
			"report": findingTestNode("report", 1.0, "done"),
		},
	}

	findings, err := RunWithEnforcer(context.Background(), workDef, workReg, &ParallelEnforcerConfig{
		EnforcerDef: enforcerDef,
		Registries:  enforcerReg,
	})

	if err != nil {
		t.Fatalf("RunWithEnforcer error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %d, want 0 (no enforcer findings)", len(findings))
	}
}

func TestRunWithEnforcer_ErrorFinding(t *testing.T) {
	workDef := twoNodeCircuit()

	enforcerDef := &circuit.CircuitDef{
		Circuit:     "test-enforcer",
		HandlerType: "node",
		Nodes: []circuit.NodeDef{
			{Name: "audit"},
			{Name: "done"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e-audit", From: "audit", To: "done"},
		},
		Start: "audit",
		Done:  "done",
	}

	var brokerCalled bool
	router := NewFindingRouter(nil, FindingHandlers{
		Broker: func(_ circuit.Finding) { brokerCalled = true },
	})

	workReg := &GraphRegistries{
		Nodes: NodeRegistry{
			"step1": findingTestNode("step1", 0.9, "ok"),
			"step2": findingTestNode("step2", 0.95, "ok"),
		},
	}

	enforcerReg := &GraphRegistries{
		Nodes: NodeRegistry{
			"audit": findingEnforcerNode("audit", router, &circuit.Finding{
				Severity: circuit.FindingError,
				Domain:   "security.auth",
				Source:   "security-auditor",
				NodeName: "step1",
				Message:  "vulnerability detected",
			}),
			"done": findingTestNode("done", 1.0, "done"),
		},
	}

	findings, err := RunWithEnforcer(context.Background(), workDef, workReg, &ParallelEnforcerConfig{
		EnforcerDef: enforcerDef,
		Registries:  enforcerReg,
		Router:      router,
	})

	if err != nil {
		t.Fatalf("RunWithEnforcer error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Severity != circuit.FindingError {
		t.Errorf("severity = %q, want %q", findings[0].Severity, circuit.FindingError)
	}
	if !brokerCalled {
		t.Error("Broker handler not called for FindingError")
	}
}
