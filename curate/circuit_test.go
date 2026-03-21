package curate

import (
	"github.com/dpopsuev/origami"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func curationYAMLPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "testdata", "curation.yaml")
}

func loadTestYAML(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(curationYAMLPath())
	if err != nil {
		t.Fatalf("read curation.yaml: %v", err)
	}
	return data
}

func TestParseCurationCircuit(t *testing.T) {
	data := loadTestYAML(t)
	def, err := ParseCurationCircuit(data)
	if err != nil {
		t.Fatalf("ParseCurationCircuit: %v", err)
	}
	if def.Circuit != "curation" {
		t.Errorf("Circuit = %q, want curation", def.Circuit)
	}
	if len(def.Nodes) != 5 {
		t.Errorf("len(Nodes) = %d, want 5", len(def.Nodes))
	}
	if len(def.Edges) != 6 {
		t.Errorf("len(Edges) = %d, want 6", len(def.Edges))
	}
	if def.Start != "fetch" {
		t.Errorf("Start = %q, want fetch", def.Start)
	}
	if def.Done != "_done" {
		t.Errorf("Done = %q, want _done", def.Done)
	}
}

func TestParseCurationCircuit_Validates(t *testing.T) {
	data := loadTestYAML(t)
	def, err := ParseCurationCircuit(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := def.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestParseCurationCircuit_Zones(t *testing.T) {
	data := loadTestYAML(t)
	def, err := ParseCurationCircuit(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(def.Zones) != 3 {
		t.Errorf("len(Zones) = %d, want 3", len(def.Zones))
	}
	zone, ok := def.Zones["intake"]
	if !ok {
		t.Fatal("missing intake zone")
	}
	if len(zone.Nodes) != 1 || zone.Nodes[0] != "fetch" {
		t.Errorf("intake.Nodes = %v, want [fetch]", zone.Nodes)
	}
}

func TestBuildCurationGraph(t *testing.T) {
	data := loadTestYAML(t)
	g, err := BuildCurationGraph(data)
	if err != nil {
		t.Fatalf("BuildCurationGraph: %v", err)
	}
	if g.Name() != "curation" {
		t.Errorf("Name() = %q, want curation", g.Name())
	}
	if len(g.Nodes()) != 5 {
		t.Errorf("len(Nodes()) = %d, want 5", len(g.Nodes()))
	}
	if len(g.Edges()) != 6 {
		t.Errorf("len(Edges()) = %d, want 6", len(g.Edges()))
	}

	fetch, ok := g.NodeByName("fetch")
	if !ok {
		t.Fatal("fetch node not found")
	}
	if fetch.Name() != "fetch" {
		t.Errorf("fetch.Name() = %q", fetch.Name())
	}

	fetchEdges := g.EdgesFrom("fetch")
	if len(fetchEdges) != 1 {
		t.Fatalf("EdgesFrom(fetch) = %d, want 1", len(fetchEdges))
	}
	if fetchEdges[0].ID() != "CE1" {
		t.Errorf("fetchEdges[0].ID() = %q, want CE1", fetchEdges[0].ID())
	}
}

func TestBuildCurationGraph_EdgeEvaluation(t *testing.T) {
	data := loadTestYAML(t)
	g, err := BuildCurationGraph(data)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	t.Run("CE1 always fires", func(t *testing.T) {
		edge := g.EdgesFrom("fetch")[0]
		state := framework.NewWalkerState("test")
		art := &CurationArtifact{}
		tr := edge.Evaluate(art, state)
		if tr == nil {
			t.Fatal("CE1 should always fire (proceed to extraction)")
		}
		if tr.NextNode != "extract" {
			t.Errorf("NextNode = %q, want extract", tr.NextNode)
		}
	})

	t.Run("CE2 always fires", func(t *testing.T) {
		edges := g.EdgesFrom("extract")
		if len(edges) != 1 {
			t.Fatalf("EdgesFrom(extract) = %d, want 1", len(edges))
		}
		state := framework.NewWalkerState("test")
		art := &CurationArtifact{}
		tr := edges[0].Evaluate(art, state)
		if tr == nil {
			t.Fatal("CE2 should always fire (proceed to validation)")
		}
		if tr.NextNode != "validate" {
			t.Errorf("NextNode = %q, want validate", tr.NextNode)
		}
	})

	t.Run("CE3+CE4 routing", func(t *testing.T) {
		validateEdges := g.EdgesFrom("validate")
		if len(validateEdges) != 2 {
			t.Fatalf("EdgesFrom(validate) = %d, want 2 (CE3, CE4)", len(validateEdges))
		}
		state := framework.NewWalkerState("test")

		incompleteArt := &CurationArtifact{
			Rec:         &Record{ID: "R01", Fields: map[string]Field{"x": {Name: "x", Value: 1}}},
			Complete:    false,
			MoreSources: true,
		}
		tr := validateEdges[0].Evaluate(incompleteArt, state) // CE3
		if tr == nil {
			t.Fatal("CE3 should fire for incomplete + more sources")
		}
		if tr.NextNode != "fetch" {
			t.Errorf("CE3 NextNode = %q, want fetch", tr.NextNode)
		}

		completeArt := &CurationArtifact{
			Rec:      &Record{ID: "R01", Fields: map[string]Field{"x": {Name: "x", Value: 1}}},
			Complete: true,
		}
		tr = validateEdges[1].Evaluate(completeArt, state) // CE4
		if tr == nil {
			t.Fatal("CE4 should fire for complete record")
		}
		if tr.NextNode != "enrich" {
			t.Errorf("CE4 NextNode = %q, want enrich", tr.NextNode)
		}
	})

	t.Run("CE3 respects loop limit", func(t *testing.T) {
		state := framework.NewWalkerState("test")
		state.LoopCounts["CE3"] = MaxFetchLoops

		validateEdges := g.EdgesFrom("validate")
		art := &CurationArtifact{Complete: false, MoreSources: true}
		tr := validateEdges[0].Evaluate(art, state)
		if tr != nil {
			t.Error("CE3 should not fire when loop limit exceeded")
		}
	})

	t.Run("CE5 always fires", func(t *testing.T) {
		edges := g.EdgesFrom("enrich")
		if len(edges) != 1 {
			t.Fatalf("EdgesFrom(enrich) = %d, want 1", len(edges))
		}
		state := framework.NewWalkerState("test")
		art := &CurationArtifact{}
		tr := edges[0].Evaluate(art, state)
		if tr == nil {
			t.Fatal("CE5 should always fire (proceed to promotion)")
		}
	})

	t.Run("CE6 always fires", func(t *testing.T) {
		edges := g.EdgesFrom("promote")
		if len(edges) != 1 {
			t.Fatalf("EdgesFrom(promote) = %d, want 1", len(edges))
		}
		state := framework.NewWalkerState("test")
		art := &CurationArtifact{}
		tr := edges[0].Evaluate(art, state)
		if tr == nil {
			t.Fatal("CE6 should always fire")
		}
		if tr.NextNode != "_done" {
			t.Errorf("CE6 NextNode = %q, want _done", tr.NextNode)
		}
	})
}

func TestDefaultNodeRegistry(t *testing.T) {
	reg := DefaultNodeRegistry()
	families := []string{"fetch", "extract", "validate", "enrich", "promote"}
	for _, f := range families {
		factory, ok := reg[f]
		if !ok {
			t.Errorf("missing node factory for %q", f)
			continue
		}
		node := factory(framework.NodeDef{Name: f, Approach: "analytical", Handler: f, HandlerType: "node"})
		if node.Name() != f {
			t.Errorf("node.Name() = %q, want %q", node.Name(), f)
		}
	}
}

func TestDefaultEdgeFactory(t *testing.T) {
	factory := DefaultEdgeFactory()
	edgeIDs := []string{"CE1", "CE2", "CE3", "CE4", "CE5", "CE6"}
	for _, id := range edgeIDs {
		fn, ok := factory[id]
		if !ok {
			t.Errorf("missing edge factory for %q", id)
			continue
		}
		edge := fn(framework.EdgeDef{ID: id, From: "test", To: "test2"})
		if edge.ID() != id {
			t.Errorf("edge.ID() = %q, want %q", edge.ID(), id)
		}
	}
}

func TestCurationArtifact_Interface(t *testing.T) {
	var a framework.Artifact = &CurationArtifact{
		ArtifactType: "test",
		Conf:         0.75,
	}
	if a.Type() != "test" {
		t.Errorf("Type() = %q, want test", a.Type())
	}
	if a.Confidence() != 0.75 {
		t.Errorf("Confidence() = %f, want 0.75", a.Confidence())
	}
}

func TestLoadCurationCircuit_FromFile(t *testing.T) {
	yamlPath := curationYAMLPath()
	def, err := LoadCurationCircuit(yamlPath)
	if err != nil {
		t.Fatalf("LoadCurationCircuit: %v", err)
	}
	if def.Circuit != "curation" {
		t.Errorf("Circuit = %q, want curation", def.Circuit)
	}
}
