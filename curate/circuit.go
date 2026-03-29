package curate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

func resolveApproachElement(approach string) agentport.Element {
	e, _ := agentport.ResolveApproach(strings.ToLower(approach))
	return e
}

// LoadCurationCircuit reads and parses the curation circuit YAML from a file path.
func LoadCurationCircuit(yamlPath string) (*circuit.CircuitDef, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("curate: read circuit %q: %w", yamlPath, err)
	}
	return ParseCurationCircuit(data)
}

// ParseCurationCircuit parses curation circuit YAML bytes.
func ParseCurationCircuit(data []byte) (*circuit.CircuitDef, error) {
	return circuit.LoadCircuit(data)
}

// curationNode implements circuit.Node for curation circuit stages.
type curationNode struct {
	name    string
	element agentport.Element
	family  string
}

func (n *curationNode) Name() string                       { return n.name }
func (n *curationNode) ElementAffinity() agentport.Element { return n.element }
func (n *curationNode) Process(_ context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	return nil, nil
}

//nolint:gocritic // hugeParam: signature required by engine.NodeRegistry
func newCurationNode(def circuit.NodeDef) circuit.Node {
	return &curationNode{
		name:    string(def.Name),
		element: resolveApproachElement(def.Approach),
		family:  def.EffectiveHandler(),
	}
}

// DefaultNodeRegistry returns a NodeRegistry with all curation node families registered.
func DefaultNodeRegistry() engine.NodeRegistry {
	return engine.NodeRegistry{
		"fetch":    newCurationNode,
		"extract":  newCurationNode,
		"validate": newCurationNode,
		"enrich":   newCurationNode,
		"promote":  newCurationNode,
	}
}

// curationEdge wraps an EdgeDef with custom evaluation logic.
type curationEdge struct {
	def      circuit.EdgeDef
	evalFunc func(circuit.Artifact, *circuit.WalkerState) *circuit.Transition
}

func (e *curationEdge) ID() string       { return e.def.ID }
func (e *curationEdge) From() string     { return string(e.def.From) }
func (e *curationEdge) To() string       { return string(e.def.To) }
func (e *curationEdge) IsShortcut() bool { return e.def.Shortcut }
func (e *curationEdge) IsLoop() bool     { return e.def.Loop }
func (e *curationEdge) Evaluate(a circuit.Artifact, s *circuit.WalkerState) *circuit.Transition {
	if e.evalFunc != nil {
		return e.evalFunc(a, s)
	}
	return &circuit.Transition{NextNode: string(e.def.To), Explanation: e.def.Condition}
}

// CurationArtifact is a generic artifact carrying a Record and evaluation metadata.
type CurationArtifact struct {
	ArtifactType string       `json:"type"`
	Rec          *Record      `json:"record,omitempty"`
	RawEvid      *RawEvidence `json:"raw_evidence,omitempty"`
	Conf         float64      `json:"confidence"`
	Complete     bool         `json:"complete"`
	MoreSources  bool         `json:"more_sources"`
}

func (a *CurationArtifact) Type() string        { return a.ArtifactType }
func (a *CurationArtifact) Confidence() float64 { return a.Conf }
func (a *CurationArtifact) Raw() any            { return a }

// MaxFetchLoops controls how many times CE3 will loop back to fetch
// before giving up and promoting incomplete records.
const MaxFetchLoops = 3

// DefaultEdgeFactory returns an EdgeFactory with evaluation logic for the
// curation circuit edges CE1-CE6.
func DefaultEdgeFactory() engine.EdgeFactory {
	return engine.EdgeFactory{
		"CE1": func(def circuit.EdgeDef) circuit.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(_ circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
					return &circuit.Transition{NextNode: string(def.To), Explanation: "proceed to extraction"}
				},
			}
		},
		"CE2": func(def circuit.EdgeDef) circuit.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(_ circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
					return &circuit.Transition{NextNode: string(def.To), Explanation: "proceed to validation"}
				},
			}
		},
		"CE3": func(def circuit.EdgeDef) circuit.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(a circuit.Artifact, s *circuit.WalkerState) *circuit.Transition {
					ca, ok := a.(*CurationArtifact)
					if !ok {
						return nil
					}
					if !ca.Complete && ca.MoreSources {
						loopCount := s.IncrementLoop("CE3")
						if loopCount > MaxFetchLoops {
							return nil
						}
						return &circuit.Transition{
							NextNode:    string(def.To),
							Explanation: "missing required fields, more sources available",
						}
					}
					return nil
				},
			}
		},
		"CE4": func(def circuit.EdgeDef) circuit.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
					ca, ok := a.(*CurationArtifact)
					if !ok {
						return nil
					}
					if ca.Complete || (!ca.MoreSources && ca.Rec != nil) {
						return &circuit.Transition{NextNode: string(def.To), Explanation: "completeness above threshold"}
					}
					return nil
				},
			}
		},
		"CE5": func(def circuit.EdgeDef) circuit.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(_ circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
					return &circuit.Transition{NextNode: string(def.To), Explanation: "proceed to promotion"}
				},
			}
		},
		"CE6": func(def circuit.EdgeDef) circuit.Edge {
			return &curationEdge{
				def: def,
				evalFunc: func(_ circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
					return &circuit.Transition{NextNode: string(def.To), Explanation: "always (terminal)"}
				},
			}
		},
	}
}

// BuildCurationGraph parses circuit YAML bytes and builds a engine.Graph
// with the default curation registries.
func BuildCurationGraph(yamlData []byte) (engine.Graph, error) {
	def, err := ParseCurationCircuit(yamlData)
	if err != nil {
		return nil, err
	}
	return engine.BuildGraph(def, &engine.GraphRegistries{Nodes: DefaultNodeRegistry(), Edges: DefaultEdgeFactory()})
}
