package view

import (
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/agentport"
)

const (
	layerSpacingX = 200.0
	nodeSpacingY  = 100.0
	zoneMargin    = 30.0
)

// LogicalLayout computes float64 x/y coordinates using a layered
// (Sugiyama-style) approach: assign ranks via topological sort, then
// spread nodes vertically within each rank. Zone containment is
// respected — nodes in the same zone are placed in adjacent rows
// within each column.
//
// The output is suitable as a seed for client-side layout engines
// (dagre, ELK) or for direct rendering in a canvas.
type LogicalLayout struct{}

func (LogicalLayout) Layout(def *circuit.CircuitDef) (CircuitLayout, error) {
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

	cols := make(map[int][]string)
	for node, r := range rank {
		cols[r] = append(cols[r], node)
	}
	for col := range cols {
		sortByZone(cols[col], nodeZone)
	}

	logical := make(map[string]LogicalPosition, len(rank))
	for col, nodes := range cols {
		x := float64(col) * layerSpacingX
		totalHeight := float64(len(nodes)-1) * nodeSpacingY
		startY := -totalHeight / 2.0

		for row, node := range nodes {
			logical[node] = LogicalPosition{
				X:    x,
				Y:    startY + float64(row)*nodeSpacingY,
				Zone: nodeZone[node],
			}
		}
	}

	edges := make([]EdgeLayout, 0, len(def.Edges))
	for i := range def.Edges {
		edges = append(edges, EdgeLayout{From: string(def.Edges[i].From), To: string(def.Edges[i].To)})
	}

	zones := make([]ZoneLayout, 0, len(def.Zones))
	for name, zd := range def.Zones {
		zElem, _ := agentport.ResolveApproach(strings.ToLower(zd.Approach))
		zones = append(zones, ZoneLayout{Name: name, Element: string(zElem)})
	}

	return CircuitLayout{Logical: logical, Edges: edges, Zones: zones}, nil
}
