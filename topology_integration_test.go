package framework_test

import (
	"context"
	"strings"
	"testing"

	framework "github.com/dpopsuev/origami"
	_ "github.com/dpopsuev/origami/topology"
)

func topoTestRegistries() framework.GraphRegistries {
	return framework.GraphRegistries{
		Nodes: framework.NodeRegistry{
			"_default": func(def framework.NodeDef) framework.Node { return &topoTestNode{name: def.Name} },
		},
	}
}

type topoTestNode struct{ name string }

func (n *topoTestNode) Name() string                                                            { return n.name }
func (n *topoTestNode) ElementAffinity() framework.Element                                      { return "" }
func (n *topoTestNode) Process(_ context.Context, _ framework.NodeContext) (framework.Artifact, error) {
	return nil, nil
}

func TestBuildGraph_TopologyCascadeValid(t *testing.T) {
	yaml := `
circuit: test
handler_type: node
topology: cascade
nodes:
  - name: A
    approach: analytical
    handler: _default
    edges: [B]
  - name: B
    approach: analytical
    handler: _default
    edges: [C]
  - name: C
    approach: analytical
    handler: _default
    edges: [DONE]
start: A
done: DONE
`
	def, err := framework.LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if def.Topology != "cascade" {
		t.Fatalf("Topology = %q, want cascade", def.Topology)
	}

	_, err = def.BuildGraph(topoTestRegistries())
	if err != nil {
		t.Fatalf("BuildGraph should pass for valid cascade: %v", err)
	}
}

func TestBuildGraph_TopologyCascadeViolation(t *testing.T) {
	yaml := `
circuit: test
handler_type: node
topology: cascade
nodes:
  - name: A
    approach: analytical
    handler: _default
    edges: [B, C]
  - name: B
    approach: analytical
    handler: _default
    edges: [C]
  - name: C
    approach: analytical
    handler: _default
    edges: [DONE]
start: A
done: DONE
`
	def, err := framework.LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	_, err = def.BuildGraph(topoTestRegistries())
	if err == nil {
		t.Fatal("BuildGraph should fail: entry node A has 2 outputs, cascade requires 1")
	}
	if !strings.Contains(err.Error(), `node "A"`) {
		t.Errorf("error should mention node A: %v", err)
	}
	if !strings.Contains(err.Error(), "outputs") {
		t.Errorf("error should mention outputs: %v", err)
	}
}

func TestBuildGraph_NoTopologySkipsValidation(t *testing.T) {
	yaml := `
circuit: test
handler_type: node
nodes:
  - name: A
    approach: analytical
    handler: _default
    edges: [B, C]
  - name: B
    approach: analytical
    handler: _default
    edges: [DONE]
  - name: C
    approach: analytical
    handler: _default
    edges: [DONE]
start: A
done: DONE
`
	def, err := framework.LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}
	if def.Topology != "" {
		t.Fatalf("Topology = %q, want empty", def.Topology)
	}

	_, err = def.BuildGraph(topoTestRegistries())
	if err != nil {
		t.Fatalf("BuildGraph should pass without topology: %v", err)
	}
}

func TestBuildGraph_UnknownTopology(t *testing.T) {
	yaml := `
circuit: test
handler_type: node
topology: invalid-topology
nodes:
  - name: A
    approach: analytical
    handler: _default
    edges: [DONE]
start: A
done: DONE
`
	def, err := framework.LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	_, err = def.BuildGraph(topoTestRegistries())
	if err == nil {
		t.Fatal("BuildGraph should fail for unknown topology")
	}
	if !strings.Contains(err.Error(), "unknown topology") {
		t.Errorf("error should mention unknown topology: %v", err)
	}
}

func TestBuildGraph_CascadeWithShortcuts(t *testing.T) {
	yaml := `
circuit: test
handler_type: node
topology: cascade
nodes:
  - name: recall
    approach: analytical
    handler: _default
    edges:
      - name: normal
        to: triage
      - name: shortcut
        to: review
        shortcut: true
  - name: triage
    approach: analytical
    handler: _default
    edges: [review]
  - name: review
    approach: analytical
    handler: _default
    edges: [DONE]
start: recall
done: DONE
`
	def, err := framework.LoadCircuit([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	_, err = def.BuildGraph(topoTestRegistries())
	if err != nil {
		t.Fatalf("BuildGraph should pass: shortcuts are excluded from cardinality: %v", err)
	}
}
