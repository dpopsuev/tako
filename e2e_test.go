package framework

import (
	"context"
	"os"
	"testing"
)

// --- Test helpers ---

func loadScenario(t *testing.T, path string) *CircuitDef {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return def
}

func stubNodeReg(families ...string) NodeRegistry {
	reg := NodeRegistry{}
	for _, f := range families {
		f := f
		reg[f] = func(d NodeDef) Node { return &stubBuildNode{name: d.Name} }
	}
	return reg
}

func forwardEdgeFactory(edgeIDs ...string) EdgeFactory {
	ef := EdgeFactory{}
	for _, id := range edgeIDs {
		id := id
		ef[id] = func(d EdgeDef) Edge {
			return &stubEdge{id: d.ID, from: d.From, to: d.To, parallel: d.Parallel}
		}
	}
	return ef
}

// --- Scenario tests ---

func TestE2E_KitchenSink(t *testing.T) {
	path := "testdata/scenarios/kitchen-sink.yaml"
	hookCalled := false
	hooks := HookRegistry{}
	hooks.Register(NewHookFunc("track-node", func(_ context.Context, _ string, _ Artifact) error {
		hookCalled = true
		return nil
	}))

	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
		WithHooks(hooks),
	)
	if err != nil {
		t.Fatalf("Walk kitchen-sink: %v", err)
	}
	if !hookCalled {
		t.Error("hook track-node should have been called")
	}
}

func TestE2E_TeamDelegation(t *testing.T) {
	def := loadScenario(t, "testdata/scenarios/team-delegation.yaml")

	walkers, wErr := BuildWalkersFromDef(def.Walkers)
	if wErr != nil {
		t.Fatalf("BuildWalkersFromDef: %v", wErr)
	}
	if len(walkers) != 3 {
		t.Fatalf("expected 3 walkers from def, got %d", len(walkers))
	}

	team := &Team{
		Walkers:   walkers,
		Scheduler: &AffinityScheduler{},
		MaxSteps:  30,
	}

	trace := &TraceCollector{}
	err := Run(context.Background(), "testdata/scenarios/team-delegation.yaml", nil,
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
		WithTeam(team),
		WithRunObserver(trace),
	)
	if err != nil {
		t.Fatalf("Walk team-delegation: %v", err)
	}

	complete := trace.EventsOfType(EventWalkComplete)
	if len(complete) == 0 {
		t.Error("walk should complete")
	}
}

func TestE2E_MemoryPersistence(t *testing.T) {
	path := "testdata/scenarios/memory-across-walks.yaml"
	store := NewInMemoryStore()

	w1 := &stubWalker{
		identity: AgentIdentity{PersonaName: "agent-a"},
		state:    NewWalkerState("agent-a"),
	}
	store.Set("agent-a", "count", 42)

	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
		WithWalker(w1),
		WithMemory(store),
	)
	if err != nil {
		t.Fatalf("Walk 1: %v", err)
	}

	w2 := &stubWalker{
		identity: AgentIdentity{PersonaName: "agent-a"},
		state:    NewWalkerState("agent-a"),
	}

	err = Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
		WithWalker(w2),
		WithMemory(store),
	)
	if err != nil {
		t.Fatalf("Walk 2: %v", err)
	}

	v, ok := store.Get("agent-a", "count")
	if !ok || v != 42 {
		t.Errorf("memory not persisted across walks: got %v/%v", v, ok)
	}

	_, ok = store.Get("agent-b", "count")
	if ok {
		t.Error("agent-b should not see agent-a's memory")
	}
}

func TestE2E_SchemaValidation(t *testing.T) {
	path := "testdata/scenarios/schema-validated.yaml"

	err := Run(context.Background(), path, nil,
		WithTransformers(TransformerRegistry{"echo": &echoTransformer{}}),
	)
	if err != nil {
		t.Fatalf("Walk schema-validated: %v", err)
	}
}

func TestE2E_RealYAML_RCAInvestigation(t *testing.T) {
	def := loadScenario(t, "testdata/rca-investigation.yaml")

	families := make([]string, 0, len(def.Nodes))
	seen := map[string]bool{}
	for _, nd := range def.Nodes {
		h := nd.EffectiveHandler()
		if h != "" && !seen[h] {
			families = append(families, h)
			seen[h] = true
		}
	}

	edgeIDs := make([]string, len(def.Edges))
	for i, ed := range def.Edges {
		edgeIDs[i] = ed.ID
	}

	graph, err := BuildGraph(def, GraphRegistries{Nodes: stubNodeReg(families...), Edges: forwardEdgeFactory(edgeIDs...)})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := &stubWalker{
		identity: AgentIdentity{PersonaName: "rca-tester"},
		state:    NewWalkerState("rca-test"),
	}
	err = graph.Walk(context.Background(), walker, def.Start)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if walker.State().Status != "done" {
		t.Errorf("status = %q, want done", walker.State().Status)
	}
	if len(walker.visited) == 0 {
		t.Error("no nodes visited")
	}
}

func TestE2E_RealYAML_DefectDialectic(t *testing.T) {
	def := loadScenario(t, "testdata/defect-dialectic.yaml")

	families := make([]string, 0, len(def.Nodes))
	seen := map[string]bool{}
	for _, nd := range def.Nodes {
		h := nd.EffectiveHandler()
		if h != "" && !seen[h] {
			families = append(families, h)
			seen[h] = true
		}
	}

	edgeIDs := make([]string, len(def.Edges))
	for i, ed := range def.Edges {
		edgeIDs[i] = ed.ID
	}

	graph, err := BuildGraph(def, GraphRegistries{Nodes: stubNodeReg(families...), Edges: forwardEdgeFactory(edgeIDs...)})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := &stubWalker{
		identity: AgentIdentity{PersonaName: "dialectic-tester"},
		state:    NewWalkerState("dialectic-test"),
	}
	err = graph.Walk(context.Background(), walker, def.Start)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if walker.State().Status != "done" {
		t.Errorf("status = %q, want done", walker.State().Status)
	}
}

func TestE2E_RealYAML_HierarchicalDelegation(t *testing.T) {
	def := loadScenario(t, "testdata/patterns/hierarchical-delegation.yaml")

	families := make([]string, 0, len(def.Nodes))
	seen := map[string]bool{}
	for _, nd := range def.Nodes {
		h := nd.EffectiveHandler()
		if h != "" && !seen[h] {
			families = append(families, h)
			seen[h] = true
		}
	}

	edgeIDs := make([]string, len(def.Edges))
	for i, ed := range def.Edges {
		edgeIDs[i] = ed.ID
	}

	graph, err := BuildGraph(def, GraphRegistries{Nodes: stubNodeReg(families...), Edges: forwardEdgeFactory(edgeIDs...)})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walkers, wErr := BuildWalkersFromDef(def.Walkers)
	if wErr != nil {
		t.Fatalf("BuildWalkersFromDef: %v", wErr)
	}
	if len(walkers) < 2 {
		t.Fatalf("expected multiple walkers from def, got %d", len(walkers))
	}

	team := &Team{
		Walkers:   walkers,
		Scheduler: &AffinityScheduler{},
		MaxSteps:  30,
	}

	err = graph.WalkTeam(context.Background(), team, def.Start)
	if err != nil {
		t.Fatalf("WalkTeam: %v", err)
	}
}

func TestE2E_RealYAML_IntentClassifier(t *testing.T) {
	def := loadScenario(t, "testdata/patterns/intent-classifier.yaml")

	if def.Circuit != "intent-classifier" {
		t.Errorf("circuit name = %q, want intent-classifier", def.Circuit)
	}

	families := make([]string, 0, len(def.Nodes))
	seen := map[string]bool{}
	for _, nd := range def.Nodes {
		h := nd.EffectiveHandler()
		if h != "" && !seen[h] {
			families = append(families, h)
			seen[h] = true
		}
	}

	edgeIDs := make([]string, len(def.Edges))
	for i, ed := range def.Edges {
		edgeIDs[i] = ed.ID
	}

	graph, err := BuildGraph(def, GraphRegistries{Nodes: stubNodeReg(families...), Edges: forwardEdgeFactory(edgeIDs...)})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walker := NewProcessWalker("classifier")
	if err := graph.Walk(context.Background(), walker, def.Start); err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if walker.State().Status != "done" {
		t.Errorf("status = %q, want done", walker.State().Status)
	}

	hasShortcut := false
	hasCacheDef := false
	hasMerge := false
	for _, ed := range def.Edges {
		if ed.Shortcut {
			hasShortcut = true
		}
		if ed.Merge != "" {
			hasMerge = true
		}
	}
	for _, nd := range def.Nodes {
		if nd.Cache != nil {
			hasCacheDef = true
		}
	}

	if !hasShortcut {
		t.Error("expected at least one shortcut edge in intent-classifier")
	}
	if !hasCacheDef {
		t.Error("expected at least one node with cache: definition")
	}
	if !hasMerge {
		t.Error("expected at least one edge with merge strategy")
	}
}
