package circuit

// Category: DSL & Build — topology inference for circuit graphs.

import "strings"

// InferTopology computes shortcut and loop flags from graph topology.
// Pass 1: DFS cycle detection marks back edges as loops.
// Pass 2: for each non-loop forward edge, checks whether an indirect path
// (length >= 2) exists — if so, the edge is a shortcut.
// Edges to the done pseudo-node are excluded from shortcut inference.
func InferTopology(def *CircuitDef) {
	if def.Start == "" || len(def.Nodes) == 0 {
		return
	}

	edgesByNode := make(map[NodeName][]int)
	for i := range def.Edges {
		edgesByNode[def.Edges[i].From] = append(edgesByNode[def.Edges[i].From], i)
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[NodeName]int)

	var dfs func(node NodeName)
	dfs = func(node NodeName) {
		color[node] = gray
		for _, idx := range edgesByNode[node] {
			e := &def.Edges[idx]
			if e.Loop {
				continue
			}
			switch color[e.To] {
			case white:
				dfs(e.To)
			case gray:
				e.Loop = true
			}
		}
		color[node] = black
	}
	dfs(def.Start)

	dagAdj := make(map[NodeName][]NodeName)
	for i := range def.Edges {
		if !def.Edges[i].Loop {
			dagAdj[def.Edges[i].From] = append(dagAdj[def.Edges[i].From], def.Edges[i].To)
		}
	}

	for i := range def.Edges {
		e := &def.Edges[i]
		if e.Loop || e.Shortcut || e.To == def.Done {
			continue
		}
		if hasIndirectPath(e.From, e.To, dagAdj) {
			e.Shortcut = true
		}
	}
}

// hasIndirectPath returns true if `to` is reachable from `from` via a path
// of length >= 2 (through at least one intermediate node).
func hasIndirectPath(from, to NodeName, adj map[NodeName][]NodeName) bool {
	visited := map[NodeName]bool{from: true}
	var queue []NodeName
	for _, next := range adj[from] {
		if next != to && !visited[next] {
			visited[next] = true
			queue = append(queue, next)
		}
	}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if node == to {
			return true
		}
		for _, next := range adj[node] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

// TopologyValidator validates a built graph's structure against a named topology.
// When nil, topology validation is skipped. The topology/ package provides the
// default implementation via RegisterTopologyValidator.
type TopologyValidator func(topoName string, shape GraphShape) error

// GraphShape describes the structural properties of a graph for topology validation.
type GraphShape struct {
	StartNode string
	DoneNode  string
	Nodes     []GraphNodeInfo
}

// GraphNodeInfo describes a single node's edge cardinality.
type GraphNodeInfo struct {
	Name    string
	Inputs  int
	Outputs int
}

// DefaultTopologyValidator is the active topology validation function.
// It is nil until a topology package registers itself via
// RegisterTopologyValidator. When nil, BuildGraph skips topology checks.
var DefaultTopologyValidator TopologyValidator

// RegisterTopologyValidator sets the default topology validator.
// Called by topology/ init() to wire in the built-in topology registry.
func RegisterTopologyValidator(v TopologyValidator) {
	DefaultTopologyValidator = v
}

// toUpperReplace is a helper to uppercase and replace characters in a string.
func toUpperReplace(s, old, replacement string) string {
	return strings.ToUpper(strings.ReplaceAll(s, old, replacement))
}
