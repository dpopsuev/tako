package autodoc

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
)

// RenderNodeTable generates a Markdown table with per-node reference information.
func RenderNodeTable(def *circuit.CircuitDef, opts *MermaidOptions) string {
	nodeDS := classifyNodes(def, opts)
	zoneOf := buildZoneMap(def)

	var b strings.Builder
	b.WriteString("| Node | Description | Zone | Handler | Type | Element | Hooks | D/S |\n")
	b.WriteString("|------|-------------|------|---------|------|---------|-------|-----|\n")

	for i := range def.Nodes {
		nd := &def.Nodes[i]
		desc := nd.Description
		if desc == "" {
			desc = "-"
		}
		zone := zoneOf[nd.Name]
		if zone == "" {
			zone = "-"
		}
		handler := nd.EffectiveHandler()
		if handler == "" {
			handler = "-"
		}
		handlerType := nd.EffectiveHandlerType(def.HandlerType)
		if handlerType == "" {
			handlerType = "-"
		}
		element := nd.Approach
		if element == "" {
			element = "-"
		}
		hooks := "-"
		if len(nd.After) > 0 {
			hooks = strings.Join(nd.After, ", ")
		}

		ds := "-"
		switch nodeDS[nd.Name] {
		case dsDeterministic:
			ds = "D"
		case dsStochastic:
			ds = "S"
		}

		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			nd.Name, desc, zone, handler, handlerType, element, hooks, ds)
	}

	return b.String()
}

// RenderSummary generates a summary section with aggregate statistics.
func RenderSummary(def *circuit.CircuitDef, opts *MermaidOptions) string {
	nodeDS := classifyNodes(def, opts)

	detCount, stochCount, unknownCount := 0, 0, 0
	for i := range def.Nodes {
		switch nodeDS[def.Nodes[i].Name] {
		case dsDeterministic:
			detCount++
		case dsStochastic:
			stochCount++
		default:
			unknownCount++
		}
	}

	shortcuts, loops := 0, 0
	for i := range def.Edges {
		if def.Edges[i].Shortcut {
			shortcuts++
		}
		if def.Edges[i].Loop {
			loops++
		}
	}

	var b strings.Builder
	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "- **Nodes:** %d", len(def.Nodes))
	if detCount > 0 || stochCount > 0 {
		fmt.Fprintf(&b, " (%d deterministic, %d stochastic", detCount, stochCount)
		if unknownCount > 0 {
			fmt.Fprintf(&b, ", %d unclassified", unknownCount)
		}
		b.WriteString(")")
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "- **Edges:** %d", len(def.Edges))
	if shortcuts > 0 || loops > 0 {
		parts := []string{}
		if shortcuts > 0 {
			parts = append(parts, fmt.Sprintf("%d shortcut", shortcuts))
		}
		if loops > 0 {
			parts = append(parts, fmt.Sprintf("%d loop", loops))
		}
		fmt.Fprintf(&b, " (%s)", strings.Join(parts, ", "))
	}
	b.WriteString("\n")
	if len(def.Zones) > 0 {
		fmt.Fprintf(&b, "- **Zones:** %d\n", len(def.Zones))
	}

	return b.String()
}
