package view

import (
	"github.com/dpopsuev/origami/circuit"
)

// GridCell is a cell-based position in a row/column grid.
type GridCell struct {
	Row  int    `json:"row"`
	Col  int    `json:"col"`
	Zone string `json:"zone,omitempty"`
}

// LogicalPosition is a coordinate-based position for GUI layouts.
type LogicalPosition struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zone string  `json:"zone,omitempty"`
}

// EdgeLayout holds the source and target node names for an edge,
// plus optional waypoints for rendering.
type EdgeLayout struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ZoneLayout holds the zone name and the bounding area for zone rendering.
type ZoneLayout struct {
	Name    string `json:"name"`
	Element string `json:"element,omitempty"`
}

// CircuitLayout holds the computed positions for all nodes, edges, and
// zones in a circuit. It is the output of a LayoutEngine and the input
// to a CircuitRenderer.
type CircuitLayout struct {
	Grid    map[string]GridCell        `json:"grid,omitempty"`
	Logical map[string]LogicalPosition `json:"logical,omitempty"`
	Edges   []EdgeLayout               `json:"edges"`
	Zones   []ZoneLayout               `json:"zones,omitempty"`
}

// LayoutEngine computes positions for circuit nodes and edges.
// Implementations provide different strategies: grid-based for TUI,
// coordinate-based for GUI.
type LayoutEngine interface {
	Layout(def *circuit.CircuitDef) (CircuitLayout, error)
}
