package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/element"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/uri"
)

// InlayHint represents an LSP 3.17 inlay hint (not in go.lsp.dev/protocol v0.12).
type InlayHint struct {
	Position    Position     `json:"position"`
	Label       string       `json:"label"`
	Kind        int          `json:"kind"` // 1=Type, 2=Parameter
	PaddingLeft bool         `json:"paddingLeft,omitempty"`
	Tooltip     *HintTooltip `json:"tooltip,omitempty"`
}

// Position is a minimal position for inlay hint JSON serialization.
type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

// HintTooltip wraps markdown content for hover-over detail.
type HintTooltip struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func markdownTooltip(md string) *HintTooltip {
	return &HintTooltip{Kind: "markdown", Value: md}
}

// inlayHintParams mirrors the LSP InlayHintParams (not in protocol v0.12).
type inlayHintParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

func (s *Server) handleInlayHint(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params inlayHintParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	doc := s.getDocument(uri.URI(params.TextDocument.URI))
	if doc == nil {
		return reply(ctx, []InlayHint{}, nil)
	}

	hints := computeInlayHints(doc)

	s.mu.Lock()
	bridge := s.kamiBridge
	s.mu.Unlock()
	if bridge != nil && bridge.Connected() {
		hints = append(hints, bridge.LiveInlayHints(doc)...)
	}

	return reply(ctx, hints, nil)
}

func computeInlayHints(doc *document) []InlayHint {
	if doc.Def == nil {
		return nil
	}

	lines := strings.Split(doc.Content, "\n")
	var hints []InlayHint

	hints = append(hints, approachTraitHints(doc, lines)...)
	hints = append(hints, personaHints(doc, lines)...)
	hints = append(hints, edgeConnectionHints(doc, lines)...)
	hints = append(hints, neighborHints(doc, lines)...)

	return hints
}

func approachTraitHints(doc *document, lines []string) []InlayHint {
	var hints []InlayHint
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "approach:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, "approach:"))
		a := element.Approach(val)
		info, ok := approachDocs[val]
		if !ok {
			continue
		}

		emoji := element.ApproachEmoji(a)
		traits := element.ApproachTraits(a)
		label := fmt.Sprintf("%s spd:%s ev:%d lp:%d", emoji, traits.Speed, traits.EvidenceDepth, traits.MaxLoops)
		hints = append(hints, InlayHint{
			Position:    Position{Line: uint32(i), Character: uint32(len(line))},
			Label:       label,
			Kind:        1,
			PaddingLeft: true,
			Tooltip:     markdownTooltip(fmt.Sprintf("### %s %s\n\n%s\n\n```\n%s\n```", emoji, val, info.Description, element.ApproachTraitsSummary(a))),
		})
	}
	return hints
}

func personaHints(doc *document, lines []string) []InlayHint {
	var hints []InlayHint
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "persona:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, "persona:"))
		desc, ok := personaDocs[val]
		if !ok {
			continue
		}

		short := desc
		if idx := strings.Index(desc, "."); idx > 0 && idx < 60 {
			short = desc[:idx+1]
		}
		hints = append(hints, InlayHint{
			Position:    Position{Line: uint32(i), Character: uint32(len(line))},
			Label:       short,
			Kind:        1,
			PaddingLeft: true,
			Tooltip:     markdownTooltip(fmt.Sprintf("### %s\n\n%s", val, desc)),
		})
	}
	return hints
}

func edgeConnectionHints(doc *document, lines []string) []InlayHint {
	if doc.Def == nil {
		return nil
	}

	inferred := inferEdgeCopy(doc.Def)

	var hints []InlayHint
	for i, edge := range doc.Def.Edges {
		line := findEdgeIDLine(lines, edge.ID)
		if line < 0 {
			continue
		}

		label := edge.From + " \u2192 " + edge.To
		var tags []string
		if edge.Shortcut {
			tags = append(tags, "shortcut")
		} else if inferred[i].Shortcut {
			tags = append(tags, "shortcut (inferred)")
		}
		if edge.Loop {
			tags = append(tags, "loop")
		} else if inferred[i].Loop {
			tags = append(tags, "loop (inferred)")
		}
		if doc.Def.Done != "" && edge.To == doc.Def.Done {
			tags = append(tags, "terminal")
		}
		if len(tags) > 0 {
			label += " \u00b7 " + strings.Join(tags, ", ")
		}

		var tooltip *HintTooltip
		cond := edge.When
		if cond == "" {
			cond = edge.Condition
		}
		if cond != "" {
			tooltip = markdownTooltip(fmt.Sprintf("**%s** %s \u2192 %s\n\n`when:` `%s`", edge.ID, edge.From, edge.To, cond))
		}

		hints = append(hints, InlayHint{
			Position:    Position{Line: uint32(line), Character: uint32(len(lines[line]))},
			Label:       label,
			Kind:        1,
			PaddingLeft: true,
			Tooltip:     tooltip,
		})
	}
	return hints
}

func inferEdgeCopy(def *circuit.CircuitDef) []circuit.EdgeDef {
	cp := *def
	cp.Edges = make([]circuit.EdgeDef, len(def.Edges))
	copy(cp.Edges, def.Edges)
	circuit.InferTopology(&cp)
	return cp.Edges
}

type edgeNeighbor struct {
	name string
	loop bool
}

func neighborHints(doc *document, lines []string) []InlayHint {
	if doc.Def == nil {
		return nil
	}

	inbound := map[string][]edgeNeighbor{}
	outbound := map[string][]edgeNeighbor{}
	for _, e := range doc.Def.Edges {
		outbound[e.From] = append(outbound[e.From], edgeNeighbor{e.To, e.Loop})
		inbound[e.To] = append(inbound[e.To], edgeNeighbor{e.From, false})
	}

	var hints []InlayHint
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- name:") && !strings.HasPrefix(trimmed, "name:") {
			continue
		}
		raw := trimmed
		if strings.HasPrefix(raw, "- ") {
			raw = raw[2:]
		}
		nodeName := strings.TrimSpace(strings.TrimPrefix(raw, "name:"))
		if nodeName == "" {
			continue
		}

		found := false
		for _, n := range doc.Def.Nodes {
			if n.Name == nodeName {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		ins := inbound[nodeName]
		outs := outbound[nodeName]
		if len(ins) == 0 && len(outs) == 0 {
			continue
		}

		label := compactNeighbors(ins, outs, doc.Def.Start == nodeName)
		tooltip := neighborTooltip(nodeName, doc)

		hints = append(hints, InlayHint{
			Position:    Position{Line: uint32(i), Character: uint32(len(line))},
			Label:       label,
			Kind:        1,
			PaddingLeft: true,
			Tooltip:     markdownTooltip(tooltip),
		})
	}
	return hints
}

func compactNeighbors(inbound, outbound []edgeNeighbor, isStart bool) string {
	dedup := func(neighbors []edgeNeighbor) []string {
		seen := map[string]bool{}
		var result []string
		for _, n := range neighbors {
			suffix := ""
			if n.loop {
				suffix = "\u21bb"
			}
			key := n.name + suffix
			if !seen[key] {
				seen[key] = true
				result = append(result, n.name+suffix)
			}
		}
		return result
	}

	var parts []string
	if isStart {
		parts = append(parts, "\u2190 start")
	}
	if ins := dedup(inbound); len(ins) > 0 {
		parts = append(parts, "\u2190 "+strings.Join(ins, ", "))
	}

	outs := dedup(outbound)
	if len(outs) > 0 {
		parts = append(parts, "\u2192 "+strings.Join(outs, ", "))
	}

	return strings.Join(parts, " \u00b7 ")
}

func neighborTooltip(nodeName string, doc *document) string {
	md := fmt.Sprintf("### %s — connected edges\n\n", nodeName)
	for _, e := range doc.Def.Edges {
		if e.From != nodeName && e.To != nodeName {
			continue
		}
		cond := e.When
		if cond == "" {
			cond = e.Condition
		}
		if cond == "" {
			cond = "unconditional"
		}

		var tags []string
		if e.Shortcut {
			tags = append(tags, "shortcut")
		}
		if e.Loop {
			tags = append(tags, "loop")
		}

		prefix := fmt.Sprintf("- **%s** %s \u2192 %s", e.ID, e.From, e.To)
		if len(tags) > 0 {
			prefix += " _(" + strings.Join(tags, ", ") + ")_"
		}
		md += prefix + " `" + cond + "`\n"
	}
	return md
}

// compactTraits extracts the failure mode from traits as a concise risk label.
func compactTraits(traits string) string {
	parts := strings.Split(traits, "|")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		kv := strings.SplitN(p, ":", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == "failure" {
			return "risk: " + strings.TrimSpace(kv[1])
		}
	}
	return ""
}

func findEdgeIDLine(lines []string, edgeID string) int {
	target := "id: " + edgeID
	for i, line := range lines {
		if strings.TrimSpace(line) == "- "+target || strings.TrimSpace(line) == target {
			return i
		}
	}
	return -1
}
