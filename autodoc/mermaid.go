package autodoc

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// MermaidOptions configures Mermaid rendering.
type MermaidOptions struct {
	Registry engine.TransformerRegistry // optional; enables D/S boundary visualization
}

// RenderMermaid generates a Mermaid flowchart from a CircuitDef with zone
// subgraphs, labeled edges (conditions, shortcuts, loops), and optional D/S
// boundary visualization.
func RenderMermaid(def *circuit.CircuitDef, opts *MermaidOptions) string {
	var b strings.Builder
	b.WriteString("graph LR\n")

	nodeDS := classifyNodes(def, opts)

	if len(def.Zones) > 0 {
		renderZonesEnhanced(&b, def, nodeDS)
	} else {
		renderFlatNodes(&b, def, nodeDS)
	}

	renderEdgesEnhanced(&b, def)
	return b.String()
}

// RenderDSBoundary generates a Mermaid flowchart with D/S boundary emphasis:
// deterministic nodes as rectangles, stochastic as stadium shapes, boundary
// edges labeled.
func RenderDSBoundary(def *circuit.CircuitDef, opts *MermaidOptions) string {
	var b strings.Builder
	b.WriteString("graph LR\n")

	nodeDS := classifyNodes(def, opts)

	if len(def.Zones) > 0 {
		renderZonesDS(&b, def, nodeDS)
	} else {
		renderFlatNodesDS(&b, def, nodeDS)
	}

	renderEdgesDS(&b, def, nodeDS)
	return b.String()
}

// RenderContextFlow generates a Mermaid data-flow diagram showing context key
// propagation through zones based on context_filter definitions.
func RenderContextFlow(def *circuit.CircuitDef) string {
	var b strings.Builder
	b.WriteString("graph TD\n")

	if len(def.Zones) == 0 {
		b.WriteString("    noZones[\"No zones defined\"]\n")
		return b.String()
	}

	zoneNames := sortedZoneNames(def)
	for _, zn := range zoneNames {
		z := def.Zones[zn]
		id := sanitize(zn)
		label := zn
		if z.ContextFilter != nil {
			if len(z.ContextFilter.Pass) > 0 {
				label += fmt.Sprintf("\\npass: %s", strings.Join(z.ContextFilter.Pass, ", "))
			}
			if len(z.ContextFilter.Block) > 0 {
				label += fmt.Sprintf("\\nblock: %s", strings.Join(z.ContextFilter.Block, ", "))
			}
		}
		fmt.Fprintf(&b, "    %s[\"%s\"]\n", id, label)
	}

	zoneOf := buildZoneMap(def)
	emitted := make(map[string]bool)
	for _, e := range def.Edges {
		fromZ := zoneOf[e.From]
		toZ := zoneOf[e.To]
		if fromZ != "" && toZ != "" && fromZ != toZ {
			key := fromZ + "->" + toZ
			if !emitted[key] {
				emitted[key] = true
				fmt.Fprintf(&b, "    %s --> %s\n", sanitize(fromZ), sanitize(toZ))
			}
		}
	}

	return b.String()
}

type dsClass int

const (
	dsUnknown dsClass = iota
	dsDeterministic
	dsStochastic
)

var knownStochastic = map[string]bool{
	"core.llm": true,
	"llm":      true,
}

func classifyNodes(def *circuit.CircuitDef, opts *MermaidOptions) map[string]dsClass {
	m := make(map[string]dsClass, len(def.Nodes))
	for _, nd := range def.Nodes {
		ht := nd.EffectiveHandlerType(def.HandlerType)
		name := nd.EffectiveHandler()
		if ht != circuit.HandlerTypeTransformer || name == "" {
			m[nd.Name] = dsUnknown
			continue
		}
		if opts != nil && opts.Registry != nil {
			if t, err := opts.Registry.Get(name); err == nil {
				m[nd.Name] = dsStochastic
				if engine.IsDeterministic(t) {
					m[nd.Name] = dsDeterministic
				}
				continue
			}
		}
		if knownStochastic[name] {
			m[nd.Name] = dsStochastic
		} else {
			m[nd.Name] = dsDeterministic
		}
	}
	return m
}

func renderZonesEnhanced(b *strings.Builder, def *circuit.CircuitDef, nodeDS map[string]dsClass) {
	zoneNames := sortedZoneNames(def)
	zonedNodes := make(map[string]bool)

	for _, zn := range zoneNames {
		z := def.Zones[zn]
		fmt.Fprintf(b, "    subgraph %s [\"%s\"]\n", sanitize(zn), capitalize(zn))
		for _, n := range z.Nodes {
			fmt.Fprintf(b, "        %s\n", nodeShape(n, nodeDS[n], false))
			zonedNodes[n] = true
		}
		b.WriteString("    end\n")
	}

	for _, nd := range def.Nodes {
		if !zonedNodes[nd.Name] {
			fmt.Fprintf(b, "    %s\n", nodeShape(nd.Name, nodeDS[nd.Name], false))
		}
	}
}

func renderFlatNodes(b *strings.Builder, def *circuit.CircuitDef, nodeDS map[string]dsClass) {
	for _, nd := range def.Nodes {
		fmt.Fprintf(b, "    %s\n", nodeShape(nd.Name, nodeDS[nd.Name], false))
	}
}

func renderEdgesEnhanced(b *strings.Builder, def *circuit.CircuitDef) {
	for _, e := range def.Edges {
		from := sanitize(e.From)
		to := sanitize(e.To)
		label := edgeLabel(e)

		if e.Shortcut {
			fmt.Fprintf(b, "    %s -.->|\"%s\"| %s\n", from, label, to)
		} else if e.Loop {
			fmt.Fprintf(b, "    %s ==>|\"%s\"| %s\n", from, label, to)
		} else {
			fmt.Fprintf(b, "    %s -->|\"%s\"| %s\n", from, label, to)
		}
	}
}

func renderZonesDS(b *strings.Builder, def *circuit.CircuitDef, nodeDS map[string]dsClass) {
	zoneNames := sortedZoneNames(def)
	zonedNodes := make(map[string]bool)

	for _, zn := range zoneNames {
		z := def.Zones[zn]
		dCount, sCount := zoneDSCount(z.Nodes, nodeDS)
		majority := "mixed"
		if sCount == 0 && dCount > 0 {
			majority = "deterministic"
		} else if dCount == 0 && sCount > 0 {
			majority = "stochastic"
		}
		fmt.Fprintf(b, "    subgraph %s [\"%s (%s)\"]\n", sanitize(zn), capitalize(zn), majority)
		for _, n := range z.Nodes {
			fmt.Fprintf(b, "        %s\n", nodeShape(n, nodeDS[n], true))
			zonedNodes[n] = true
		}
		b.WriteString("    end\n")
	}

	for _, nd := range def.Nodes {
		if !zonedNodes[nd.Name] {
			fmt.Fprintf(b, "    %s\n", nodeShape(nd.Name, nodeDS[nd.Name], true))
		}
	}
}

func renderFlatNodesDS(b *strings.Builder, def *circuit.CircuitDef, nodeDS map[string]dsClass) {
	for _, nd := range def.Nodes {
		fmt.Fprintf(b, "    %s\n", nodeShape(nd.Name, nodeDS[nd.Name], true))
	}
}

func renderEdgesDS(b *strings.Builder, def *circuit.CircuitDef, nodeDS map[string]dsClass) {
	for _, e := range def.Edges {
		from := sanitize(e.From)
		to := sanitize(e.To)
		label := edgeLabel(e)

		isBoundary := nodeDS[e.From] == dsDeterministic && nodeDS[e.To] == dsStochastic
		if isBoundary {
			label += " [D→S]"
		}

		if e.Shortcut {
			fmt.Fprintf(b, "    %s -.->|\"%s\"| %s\n", from, label, to)
		} else if e.Loop {
			fmt.Fprintf(b, "    %s ==>|\"%s\"| %s\n", from, label, to)
		} else {
			fmt.Fprintf(b, "    %s -->|\"%s\"| %s\n", from, label, to)
		}
	}
}

func nodeShape(name string, ds dsClass, useDSShapes bool) string {
	id := sanitize(name)
	if !useDSShapes {
		return id
	}
	switch ds {
	case dsStochastic:
		return fmt.Sprintf("%s([\"%s\"])", id, name) // stadium shape
	case dsDeterministic:
		return fmt.Sprintf("%s[\"%s\"]", id, name) // rectangle
	default:
		return id
	}
}

func edgeLabel(e circuit.EdgeDef) string {
	parts := []string{}
	if e.Name != "" {
		parts = append(parts, e.Name)
	} else {
		parts = append(parts, e.ID)
	}
	if e.Shortcut {
		parts = append(parts, "shortcut")
	}
	if e.Loop {
		parts = append(parts, "loop")
	}
	return strings.Join(parts, " ")
}

func zoneDSCount(nodes []string, nodeDS map[string]dsClass) (det, stoch int) {
	for _, n := range nodes {
		switch nodeDS[n] {
		case dsDeterministic:
			det++
		case dsStochastic:
			stoch++
		}
	}
	return
}

func sortedZoneNames(def *circuit.CircuitDef) []string {
	names := make([]string, 0, len(def.Zones))
	for n := range def.Zones {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func buildZoneMap(def *circuit.CircuitDef) map[string]string {
	m := make(map[string]string)
	for zn, z := range def.Zones {
		for _, n := range z.Nodes {
			m[n] = zn
		}
	}
	return m
}

func sanitize(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
