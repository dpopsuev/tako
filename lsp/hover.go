package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/element"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type approachInfo struct {
	Description string
	Color       string
}

var approachDocs = map[string]approachInfo{
	"rapid":      {Description: "Bold, fast, confident. First to declare a verdict.", Color: "#DC143C (Crimson)"},
	"analytical": {Description: "Methodical, thorough, evidence-first. Examines every log.", Color: "#007BA7 (Cerulean)"},
	"methodical": {Description: "Pragmatic, categorical, infrastructure-focused.", Color: "#0047AB (Cobalt)"},
	"holistic":   {Description: "Creative, lateral thinker, cross-domain correlator.", Color: "#FFBF00 (Amber)"},
	"rigorous":   {Description: "Skeptical, evidence-demanding. The final quality gate.", Color: "#0F52BA (Sapphire)"},
	"aggressive": {Description: "Dispatcher, orchestrator. Manages the circuit queue.", Color: "#DC143C (Crimson)"},
}

var personaDocs = map[string]string{
	"herald":   "Fire persona. Bold, fast classifier. \"I saw the error. I already know what happened.\"",
	"seeker":   "Water persona. Deep evidence gatherer. \"Let's not jump to conclusions.\"",
	"sentinel": "Earth persona. Infrastructure specialist. \"I've filed this under infrastructure.\"",
	"weaver":   "Air persona. Cross-repo correlator. \"What if the bug isn't in the code?\"",
	"arbiter":  "Diamond persona. Adversarial reviewer. \"The evidence is inconclusive.\"",
	"catalyst": "Lightning persona. Circuit orchestrator. \"New failure incoming! All units respond!\"",
	"oracle":   "Void persona. Pattern recognizer across time. Sees trends invisible to others.",
	"phantom":  "Antithesis persona. The adversarial counterpart used in the Dialectic system.",
}


var exprContextDocs = map[string]string{
	"output": "The artifact produced by the source node. Fields depend on the node family.",
	"state":  "Walker state: `state.loops.<node>` (loop count), `state.visited` (set of visited nodes).",
	"config": "Circuit vars from the `vars:` section. Access as `config.<var_name>`.",
}

func (s *Server) handleHover(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.HoverParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	doc := s.getDocument(params.TextDocument.URI)
	if doc == nil {
		return reply(ctx, nil, nil)
	}

	hover := computeHover(doc, params.Position, s.vocab)
	if hover == nil {
		return reply(ctx, nil, nil)
	}
	return reply(ctx, hover, nil)
}

func computeHover(doc *document, pos protocol.Position, vocab circuit.RichVocabulary) *protocol.Hover {
	lines := strings.Split(doc.Content, "\n")
	if int(pos.Line) >= len(lines) {
		return nil
	}

	line := lines[pos.Line]
	trimmed := strings.TrimSpace(line)

	// Approach hover
	if strings.HasPrefix(trimmed, "approach:") {
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, "approach:"))
		a := element.Approach(val)
		if info, ok := approachDocs[val]; ok {
			emoji := element.ApproachEmoji(a)
			traits := element.ApproachTraitsSummary(a)
			md := fmt.Sprintf("### %s %s\n\n%s\n\n```\n%s\n```\n\n**Color:** %s",
				emoji, val, info.Description, traits, info.Color)
			return &protocol.Hover{
				Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: md},
			}
		}
	}

	// Persona hover
	if strings.HasPrefix(trimmed, "persona:") {
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, "persona:"))
		if desc, ok := personaDocs[val]; ok {
			md := fmt.Sprintf("### Persona: %s\n\n%s", val, desc)
			return &protocol.Hover{
				Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: md},
			}
		}
	}

	// Expression context hover
	if strings.HasPrefix(trimmed, "when:") {
		md := "### Edge Expression Context\n\n"
		for k, v := range exprContextDocs {
			md += fmt.Sprintf("- **%s** — %s\n", k, v)
		}
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: md},
		}
	}

	// Node name hover (in from:/to:/start: and - name: declarations)
	nodeName := ""
	for _, prefix := range []string{"from:", "to:", "start:"} {
		if strings.HasPrefix(trimmed, prefix) {
			nodeName = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			break
		}
	}
	if nodeName == "" && strings.HasPrefix(trimmed, "name:") || strings.HasPrefix(trimmed, "- name:") {
		raw := strings.TrimPrefix(trimmed, "- ")
		nodeName = strings.TrimSpace(strings.TrimPrefix(raw, "name:"))
	}
	if nodeName != "" && doc.Def != nil {
		for _, n := range doc.Def.Nodes {
			if n.Name == nodeName {
				md := fmt.Sprintf("### Node: %s\n\n", n.Name)
				if n.Description != "" {
					md += n.Description + "\n\n"
				}
				handler := n.EffectiveHandler()
				if handler != "" && handler != n.Name {
					md += fmt.Sprintf("**Handler:** %s\n\n", handler)
				}
				if n.Approach != "" {
					emoji := element.ApproachEmoji(element.Approach(n.Approach))
					md += fmt.Sprintf("**Approach:** %s %s\n\n", emoji, n.Approach)
				}
				if vocab != nil {
					if d := vocab.Description(n.Name); d != "" {
						md += fmt.Sprintf("---\n\n%s\n\n", d)
					}
				}
				md += connectedEdges(doc.Def, n.Name)
				return &protocol.Hover{
					Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: md},
				}
			}
		}
	}

	// Top-level key hover
	topLevelDocs := map[string]string{
		"circuit":    "Circuit name identifier. Used in logs, reports, and MCP routing.",
		"description": "Human-readable circuit description.",
		"imports":     "List of external circuit files to import and merge.",
		"vars":        "Circuit variables. Accessible in edge expressions as `config.<name>`.",
		"zones":       "Logical node groupings. Map zone name to `{ nodes: [...], approach: ..., stickiness: N }`.",
		"nodes":       "Circuit nodes. Each node has a name, family, and optional approach/extractor/transformer.",
		"edges":       "Conditional transitions between nodes. Each edge has `from`, `to`, and `when` expression.",
		"walkers":     "Walker definitions. Each walker has a name, approach, persona, and optional step affinity.",
		"start":       "The starting node for circuit execution.",
		"done":        "The terminal sentinel node. Reaching this node completes the walk.",
	}
	for key, desc := range topLevelDocs {
		if strings.HasPrefix(trimmed, key+":") {
			md := fmt.Sprintf("### %s\n\n%s", key, desc)
			return &protocol.Hover{
				Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: md},
			}
		}
	}

	return nil
}

func connectedEdges(def *circuit.CircuitDef, nodeName string) string {
	var inbound, outbound []string
	for _, e := range def.Edges {
		label := formatEdgeLabel(e)
		if e.To == nodeName {
			inbound = append(inbound, fmt.Sprintf("- %s **%s** `%s`", e.From+" →", e.ID, label))
		}
		if e.From == nodeName {
			outbound = append(outbound, fmt.Sprintf("- → %s **%s** `%s`", e.To, e.ID, label))
		}
	}
	if len(inbound) == 0 && len(outbound) == 0 {
		return ""
	}

	md := "**Connected edges:**\n\n"
	for _, s := range inbound {
		md += s + "\n"
	}
	for _, s := range outbound {
		md += s + "\n"
	}
	return md
}

func formatEdgeLabel(e circuit.EdgeDef) string {
	var tags []string
	if e.Shortcut {
		tags = append(tags, "shortcut")
	}
	if e.Loop {
		tags = append(tags, "loop")
	}

	cond := e.When
	if cond == "" {
		cond = e.Condition
	}

	if len(tags) > 0 && cond != "" {
		return strings.Join(tags, ", ") + " · " + cond
	}
	if len(tags) > 0 {
		return strings.Join(tags, ", ")
	}
	if cond != "" {
		return cond
	}
	return "unconditional"
}
