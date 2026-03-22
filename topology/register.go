package topology

import (
	"fmt"

	"github.com/dpopsuev/origami/circuit"
)

func init() {
	reg := DefaultRegistry()
	circuit.RegisterTopologyValidator(func(topoName string, shape circuit.GraphShape) error {
		topoDef, ok := reg.Lookup(topoName)
		if !ok {
			return fmt.Errorf("unknown topology %q (known: %v)", topoName, reg.List())
		}
		adapted := &shapeAdapter{shape: shape}
		result := Validate(adapted, topoDef)
		if !result.OK() {
			return fmt.Errorf("%s", result.Error())
		}
		return nil
	})
}

// shapeAdapter converts circuit.GraphShape to topology.GraphShape.
type shapeAdapter struct {
	shape circuit.GraphShape
}

func (a *shapeAdapter) StartNode() string { return a.shape.StartNode }
func (a *shapeAdapter) DoneNode() string  { return a.shape.DoneNode }
func (a *shapeAdapter) NodeCount() int    { return len(a.shape.Nodes) }

func (a *shapeAdapter) NodeInfos() []NodeInfo {
	infos := make([]NodeInfo, len(a.shape.Nodes))
	for i, n := range a.shape.Nodes {
		infos[i] = NodeInfo{Name: n.Name, Inputs: n.Inputs, Outputs: n.Outputs}
	}
	return infos
}
