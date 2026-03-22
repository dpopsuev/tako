package sumi

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InspectorPanel implements Panel for the node inspector.
type InspectorPanel struct {
	def     *circuit.CircuitDef
	snap    *view.CircuitSnapshot
	noColor bool

	selectedNode string
	scrollY      int
	contentLines int
}

// NewInspectorPanel creates an inspector panel.
func NewInspectorPanel(def *circuit.CircuitDef, snap *view.CircuitSnapshot, noColor bool) *InspectorPanel {
	return &InspectorPanel{
		def:     def,
		snap:    snap,
		noColor: noColor,
	}
}

func (p *InspectorPanel) ID() string                { return "inspector" }
func (p *InspectorPanel) Title() string             { return "Inspector" }
func (p *InspectorPanel) Focusable() bool           { return true }
func (p *InspectorPanel) PreferredSize() (int, int) { return inspectorMinW, 10 }

// SetNode changes which node the inspector is showing.
func (p *InspectorPanel) SetNode(name string) {
	if name != p.selectedNode {
		p.selectedNode = name
		p.scrollY = 0
	}
}

// SelectedNode returns the node currently being inspected.
func (p *InspectorPanel) SelectedNode() string {
	return p.selectedNode
}

func (p *InspectorPanel) Update(msg tea.Msg) tea.Cmd {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch km.String() {
	case "up":
		if p.scrollY > 0 {
			p.scrollY--
		}
	case "down":
		p.scrollY++
	case "home":
		p.scrollY = 0
	}
	return nil
}

func (p *InspectorPanel) View(area Rect) string {
	inner := area.Inner()
	if inner.W <= 0 || inner.H <= 0 {
		return ""
	}

	content := p.renderContent(inner.W)
	lines := strings.Split(content, "\n")
	p.contentLines = len(lines)

	maxScroll := len(lines) - inner.H
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.scrollY > maxScroll {
		p.scrollY = maxScroll
	}

	start := p.scrollY
	end := start + inner.H
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[start:end]

	return strings.Join(visible, "\n")
}

func (p *InspectorPanel) renderContent(width int) string {
	if p.selectedNode == "" {
		return "Select a node to inspect"
	}

	nd := p.findNode(p.selectedNode)
	if nd == nil {
		return fmt.Sprintf("Node %q not found", p.selectedNode)
	}

	ns := p.snap.Nodes[p.selectedNode]

	var sb strings.Builder

	writeField := func(label, value string) {
		if p.noColor {
			sb.WriteString(fmt.Sprintf("%-10s %s\n", label+":", value))
		} else {
			sb.WriteString(styleInspLabel.Render(fmt.Sprintf("%-10s", label+":")) + " " + value + "\n")
		}
	}

	writeField("Name", nd.Name)
	writeField("Approach", renderApproach(nd.Approach, p.noColor))
	writeField("State", renderState(ns.State, p.noColor))

	zone := ns.Zone
	if zone == "" {
		zone = "(none)"
	}
	writeField("Zone", zone)

	handler := nd.EffectiveHandler()
	ht := nd.EffectiveHandlerType(p.def.HandlerType)
	if handler != "" && handler != nd.Name {
		writeField("Handler", handler)
	}
	if ht != "" {
		writeField("Type", ht)
	}
	badge := DSBadge(handler)
	if badge != "" {
		writeField("D/S", badge)
	}
	if nd.Description != "" {
		sb.WriteByte('\n')
		writeField("Desc", "")
		for _, line := range wrapText(nd.Description, width-2) {
			sb.WriteString("  " + line + "\n")
		}
	}

	// Walker info
	walkers := p.walkersAtNode(nd.Name)
	if len(walkers) > 0 {
		sb.WriteByte('\n')
		if p.noColor {
			sb.WriteString("Walkers:\n")
		} else {
			sb.WriteString(styleInspSection.Render("Walkers:") + "\n")
		}
		for _, wp := range walkers {
			sb.WriteString(fmt.Sprintf("  ● %s (%s)\n", wp.WalkerID, wp.Element))
		}
	}

	// Edge info
	incoming, outgoing := p.edgesFor(nd.Name)
	if len(incoming) > 0 || len(outgoing) > 0 {
		sb.WriteByte('\n')
		if p.noColor {
			sb.WriteString("Edges:\n")
		} else {
			sb.WriteString(styleInspSection.Render("Edges:") + "\n")
		}
		for _, e := range incoming {
			cond := ""
			if e.When != "" {
				cond = " [" + e.When + "]"
			}
			sb.WriteString(fmt.Sprintf("  ← %s%s\n", e.From, cond))
		}
		for _, e := range outgoing {
			cond := ""
			if e.When != "" {
				cond = " [" + e.When + "]"
			}
			sb.WriteString(fmt.Sprintf("  → %s%s\n", e.To, cond))
		}
	}

	// Zone members
	if ns.Zone != "" {
		if zd, ok := p.def.Zones[ns.Zone]; ok && len(zd.Nodes) > 1 {
			sb.WriteByte('\n')
			if p.noColor {
				sb.WriteString(fmt.Sprintf("Zone %s members:\n", ns.Zone))
			} else {
				sb.WriteString(styleInspSection.Render(fmt.Sprintf("Zone %s members:", ns.Zone)) + "\n")
			}
			for _, peer := range zd.Nodes {
				marker := "  "
				if peer == nd.Name {
					marker = "▸ "
				}
				sb.WriteString(fmt.Sprintf("  %s%s\n", marker, peer))
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (p *InspectorPanel) findNode(name string) *circuit.NodeDef {
	for i := range p.def.Nodes {
		if p.def.Nodes[i].Name == name {
			return &p.def.Nodes[i]
		}
	}
	return nil
}

func (p *InspectorPanel) walkersAtNode(name string) []view.WalkerPosition {
	var out []view.WalkerPosition
	for _, wp := range p.snap.Walkers {
		if wp.Node == name {
			out = append(out, wp)
		}
	}
	return out
}

func (p *InspectorPanel) edgesFor(name string) (incoming, outgoing []circuit.EdgeDef) {
	for _, e := range p.def.Edges {
		if e.To == name {
			incoming = append(incoming, e)
		}
		if e.From == name {
			outgoing = append(outgoing, e)
		}
	}
	return
}

func renderApproach(approach string, noColor bool) string {
	if noColor || approach == "" {
		return approach
	}
	elem := resolveApproachToElement(approach)
	return ElementFg(elem).Render(approach)
}

func renderState(state view.NodeVisualState, noColor bool) string {
	s := string(state)
	if noColor {
		return s
	}
	switch state {
	case view.NodeActive:
		return StyleActive.Render(s)
	case view.NodeCompleted:
		return StyleCompleted.Render(s)
	case view.NodeError:
		return StyleError.Render(s)
	default:
		return lipgloss.NewStyle().Faint(true).Render(s)
	}
}

func wrapText(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	current := words[0]
	for _, w := range words[1:] {
		if len(current)+1+len(w) > width {
			lines = append(lines, current)
			current = w
		} else {
			current += " " + w
		}
	}
	lines = append(lines, current)
	return lines
}

var (
	styleInspLabel   = lipgloss.NewStyle().Faint(true)
	styleInspSection = lipgloss.NewStyle().Bold(true).Underline(true)
)
