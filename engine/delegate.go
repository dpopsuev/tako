package engine

// Category: Core Primitives — delegate node types.

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"gopkg.in/yaml.v3"
)

const artifactTypeDelegate = "delegate"

// DelegateNode is a Node that generates a sub-circuit instead of producing
// an artifact directly. When the walk loop encounters a DelegateNode, it
// calls GenerateCircuit to obtain a circuit.CircuitDef, builds and walks the
// sub-graph, and wraps the results in a DelegateArtifact.
type DelegateNode interface {
	circuit.Node
	GenerateCircuit(ctx context.Context, nc circuit.NodeContext) (*circuit.CircuitDef, error)
}

// DelegateArtifact wraps the result of a sub-walk produced by a DelegateNode.
type DelegateArtifact struct {
	// GeneratedCircuit is the circuit.CircuitDef returned by GenerateCircuit.
	GeneratedCircuit *circuit.CircuitDef `json:"generated_circuit"`

	// InnerArtifacts maps inner node names to their produced artifacts.
	InnerArtifacts map[string]circuit.Artifact `json:"-"`

	// NodeCount is the number of nodes in the generated circuit.
	NodeCount int `json:"node_count"`

	// Elapsed is the wall-clock duration of the inner walk.
	Elapsed time.Duration `json:"elapsed"`

	// InnerError is non-nil if the sub-walk failed.
	InnerError error `json:"inner_error,omitempty"`
}

func (a *DelegateArtifact) Type() string       { return artifactTypeDelegate }
func (a *DelegateArtifact) Confidence() float64 { return a.confidence() }
func (a *DelegateArtifact) Raw() any            { return a.InnerArtifacts }

// confidence returns the average confidence across inner artifacts,
// or 0 if there are none.
func (a *DelegateArtifact) confidence() float64 {
	if len(a.InnerArtifacts) == 0 {
		return 0
	}
	var sum float64
	var count int
	for _, art := range a.InnerArtifacts {
		if art != nil {
			sum += art.Confidence()
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// dslDelegateNode is a DelegateNode produced by BuildGraph when a circuit.NodeDef
// has delegate: true and generator: set.
type dslDelegateNode struct {
	name       string
	element    circuit.Element
	gen        Transformer
	config     map[string]any
	nodeConfig *circuit.NodeConfig
	meta       map[string]any
}

func (n *dslDelegateNode) Name() string             { return n.name }
func (n *dslDelegateNode) ElementAffinity() circuit.Element { return n.element }

func (n *dslDelegateNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	da, err := n.GenerateCircuit(ctx, nc)
	if err != nil {
		return nil, err
	}
	return &DelegateArtifact{GeneratedCircuit: da, NodeCount: len(da.Nodes)}, nil
}

func (n *dslDelegateNode) GenerateCircuit(ctx context.Context, nc circuit.NodeContext) (*circuit.CircuitDef, error) {
	var input any
	if nc.PriorArtifact != nil {
		input = nc.PriorArtifact.Raw()
	}

	tc := &TransformerContext{
		Input:       input,
		Config:      n.config,
		NodeName:    n.name,
		NodeConfig:  n.nodeConfig,
		Meta:        n.meta,
		WalkerState: nc.WalkerState,
	}

	result, err := n.gen.Transform(ctx, tc)
	if err != nil {
		return nil, fmt.Errorf("generator %s: %w", n.gen.Name(), err)
	}

	switch v := result.(type) {
	case *circuit.CircuitDef:
		return v, nil
	case circuit.CircuitDef:
		return &v, nil
	case map[string]any:
		data, err := yaml.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("generator %s: marshal circuit map: %w", n.gen.Name(), err)
		}
		return LoadCircuit(data)
	case string:
		return LoadCircuit([]byte(v))
	case []byte:
		return LoadCircuit(v)
	default:
		return nil, fmt.Errorf("generator %s: unexpected result type %T (want *circuit.CircuitDef, map, string, or []byte)", n.gen.Name(), result)
	}
}

// circuitRefNode is a DelegateNode that references a pre-loaded circuit.CircuitDef.
type circuitRefNode struct {
	name       string
	element    circuit.Element
	circuitDef *circuit.CircuitDef
	meta       map[string]any
}

func (n *circuitRefNode) Name() string             { return n.name }
func (n *circuitRefNode) ElementAffinity() circuit.Element { return n.element }

func (n *circuitRefNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	return &DelegateArtifact{GeneratedCircuit: n.circuitDef, NodeCount: len(n.circuitDef.Nodes)}, nil
}

func (n *circuitRefNode) GenerateCircuit(_ context.Context, _ circuit.NodeContext) (*circuit.CircuitDef, error) {
	return n.circuitDef, nil
}
