package view

import (
	"sort"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/bugle/element"
)

// GridLayout computes cell-based positions using Kahn's algorithm
// (topological sort). Nodes are assigned to columns by topological rank
// and to rows within each column. Nodes in the same zone are grouped
// into adjacent rows.
type GridLayout struct{}

func (GridLayout) Layout(def *circuit.CircuitDef) (CircuitLayout, error) {
	if len(def.Nodes) == 0 {
		return CircuitLayout{}, nil
	}

	adj, inDeg := buildAdjacency(def)
	order, err := topoSort(def, adj, inDeg)
	if err != nil {
		return CircuitLayout{}, err
	}

	nodeZone := buildNodeZoneMap(def)
	rank := assignRanks(order, adj)

	grid := assignGridCells(rank, nodeZone, def.Start)

	edges := make([]EdgeLayout, 0, len(def.Edges))
	for _, e := range def.Edges {
		edges = append(edges, EdgeLayout{From: e.From, To: e.To})
	}

	zones := make([]ZoneLayout, 0, len(def.Zones))
	for name, zd := range def.Zones {
		zElem, _ := element.ResolveApproach(strings.ToLower(zd.Approach))
		zones = append(zones, ZoneLayout{Name: name, Element: string(zElem)})
	}

	return CircuitLayout{Grid: grid, Edges: edges, Zones: zones}, nil
}

func buildAdjacency(def *circuit.CircuitDef) (map[string][]string, map[string]int) {
	adj := make(map[string][]string, len(def.Nodes))
	inDeg := make(map[string]int, len(def.Nodes))
	for _, n := range def.Nodes {
		adj[n.Name] = nil
		inDeg[n.Name] = 0
	}
	for _, e := range def.Edges {
		if e.Loop {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
		inDeg[e.To]++
	}
	return adj, inDeg
}

func topoSort(def *circuit.CircuitDef, adj map[string][]string, inDeg map[string]int) ([]string, error) {
	queue := make([]string, 0)

	if def.Start != "" {
		if inDeg[def.Start] == 0 {
			queue = append(queue, def.Start)
		}
	}
	for _, n := range def.Nodes {
		if inDeg[n.Name] == 0 && n.Name != def.Start {
			queue = append(queue, n.Name)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		for _, next := range adj[node] {
			inDeg[next]--
			if inDeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	// If cycles remain (e.g., hearing <-> cmrr), append unsorted nodes
	// in their definition order. This isn't pure topological but is
	// sufficient for layout positioning.
	if len(order) != len(def.Nodes) {
		sorted := make(map[string]bool, len(order))
		for _, n := range order {
			sorted[n] = true
		}
		for _, n := range def.Nodes {
			if !sorted[n.Name] {
				order = append(order, n.Name)
			}
		}
	}
	return order, nil
}

func buildNodeZoneMap(def *circuit.CircuitDef) map[string]string {
	nz := make(map[string]string)
	for zoneName, zd := range def.Zones {
		for _, nodeName := range zd.Nodes {
			nz[nodeName] = zoneName
		}
	}
	return nz
}

func assignRanks(order []string, adj map[string][]string) map[string]int {
	rank := make(map[string]int, len(order))
	for _, node := range order {
		if _, exists := rank[node]; !exists {
			rank[node] = 0
		}
		for _, next := range adj[node] {
			if rank[node]+1 > rank[next] {
				rank[next] = rank[node] + 1
			}
		}
	}
	return rank
}

// assignGridCells places nodes into grid cells. Nodes with the same rank
// share a column. Within a column, nodes are sorted by zone to keep
// same-zone nodes adjacent. The start node is guaranteed to be in col 0.
func assignGridCells(rank map[string]int, nodeZone map[string]string, start string) map[string]GridCell {
	cols := make(map[int][]string)
	for node, r := range rank {
		cols[r] = append(cols[r], node)
	}

	for col := range cols {
		sortByZone(cols[col], nodeZone)
	}

	// Deterministic: sort each column's nodes for stable row assignment.
	// sortByZone groups by zone; within each zone group and the no-zone
	// tail, nodes appear in map iteration order which is random. Re-sort
	// alphabetically within each zone group to make layout reproducible.
	for col := range cols {
		stabilizeSameZoneOrder(cols[col], nodeZone)
	}

	grid := make(map[string]GridCell, len(rank))
	for col, nodes := range cols {
		for row, node := range nodes {
			grid[node] = GridCell{
				Row:  row,
				Col:  col,
				Zone: nodeZone[node],
			}
		}
	}
	return grid
}

// stabilizeSameZoneOrder sorts nodes alphabetically within each zone group
// (and the no-zone tail) so that row assignment is deterministic regardless
// of map iteration order.
func stabilizeSameZoneOrder(nodes []string, nodeZone map[string]string) {
	if len(nodes) <= 1 {
		return
	}
	i := 0
	for i < len(nodes) {
		z := nodeZone[nodes[i]]
		j := i + 1
		for j < len(nodes) && nodeZone[nodes[j]] == z {
			j++
		}
		if j-i > 1 {
			sort.Strings(nodes[i:j])
		}
		i = j
	}
}

// sortByZone groups nodes by zone so nodes in the same zone are adjacent.
// Stable within each zone group (preserves topological order).
func sortByZone(nodes []string, nodeZone map[string]string) {
	if len(nodes) <= 1 {
		return
	}

	type entry struct {
		name string
		zone string
	}
	entries := make([]entry, len(nodes))
	for i, n := range nodes {
		entries[i] = entry{name: n, zone: nodeZone[n]}
	}

	// Group by zone: collect zones in order of first appearance, then
	// emit all nodes for each zone in their original order.
	seen := make(map[string]bool)
	var zoneOrder []string
	for _, e := range entries {
		if e.zone != "" && !seen[e.zone] {
			seen[e.zone] = true
			zoneOrder = append(zoneOrder, e.zone)
		}
	}

	idx := 0
	// Nodes with a zone, grouped by zone order.
	for _, z := range zoneOrder {
		for _, e := range entries {
			if e.zone == z {
				nodes[idx] = e.name
				idx++
			}
		}
	}
	// Nodes without a zone go last.
	for _, e := range entries {
		if e.zone == "" {
			nodes[idx] = e.name
			idx++
		}
	}
}
