package circuit

// Category: DSL & Build — Mermaid rendering of circuit definitions.

import (
	"fmt"
	"sort"
	"strings"
)

// Render generates a Mermaid flowchart string from a circuit definition (P6).
// When Zones are defined, nodes are grouped into Mermaid subgraphs.
// When Zones are empty, a flat graph is rendered.
func Render(def *CircuitDef) string {
	var b strings.Builder
	b.WriteString("graph LR\n")

	if len(def.Zones) > 0 {
		renderWithZones(&b, def)
	}

	renderEdges(&b, def)
	return b.String()
}

func renderWithZones(b *strings.Builder, def *CircuitDef) {
	zoneNames := make([]string, 0, len(def.Zones))
	for name := range def.Zones {
		zoneNames = append(zoneNames, name)
	}
	sort.Strings(zoneNames)

	zonedNodes := make(map[string]bool)
	for _, name := range zoneNames {
		z := def.Zones[name]
		fmt.Fprintf(b, "    subgraph %s [%s]\n", sanitizeID(name), capitalizeFirst(name))
		for _, n := range z.Nodes {
			fmt.Fprintf(b, "        %s\n", sanitizeID(n))
			zonedNodes[n] = true
		}
		b.WriteString("    end\n")
	}

	for i := range def.Nodes {
		if !zonedNodes[def.Nodes[i].Name] {
			fmt.Fprintf(b, "    %s\n", sanitizeID(def.Nodes[i].Name))
		}
	}
}

func renderEdges(b *strings.Builder, def *CircuitDef) {
	for i := range def.Edges {
		e := &def.Edges[i]
		from := sanitizeID(e.From)
		to := sanitizeID(e.To)
		label := e.Name
		if label == "" {
			label = e.ID
		}
		fmt.Fprintf(b, "    %s -->|\"%s: %s\"| %s\n", from, e.ID, label, to)
	}
}

func sanitizeID(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
