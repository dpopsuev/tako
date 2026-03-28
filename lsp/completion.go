package lsp

import (
	"context"
	"encoding/json"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

const (
	ctxNodes   = "nodes"
	ctxEdges   = "edges"
	ctxWalkers = "walkers"
	ctxZones   = "zones"
)

var topLevelKeys = []string{
	"circuit", "description", "imports", "vars", "zones",
	"nodes", "edges", "walkers", "start", "done",
}

var nodeFieldKeys = []string{
	"name", "approach", "family", "extractor", "transformer",
	"provider", "prompt", "input", "after", "schema", "cache",
}

var edgeFieldKeys = []string{
	"id", "name", "from", "to", "shortcut", "loop",
	"parallel", "condition", "when", "merge",
}

var walkerFieldKeys = []string{
	"name", "approach", "persona", "preamble", "step_affinity",
}

var approachValues = []string{
	"rapid", "analytical", "methodical", "holistic", "rigorous", "aggressive",
}

var personaValues = []string{
	"herald", "seeker", "sentinel", "weaver",
	"arbiter", "catalyst", "oracle", "phantom",
}

func (s *Server) handleCompletion(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CompletionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	doc := s.getDocument(params.TextDocument.URI)
	if doc == nil {
		return reply(ctx, &protocol.CompletionList{}, nil)
	}

	items := computeCompletions(doc, params.Position)
	return reply(ctx, &protocol.CompletionList{Items: items}, nil)
}

func computeCompletions(doc *document, pos protocol.Position) []protocol.CompletionItem {
	lines := strings.Split(doc.Content, "\n")
	if int(pos.Line) >= len(lines) {
		return nil
	}

	line := lines[pos.Line]
	trimmed := strings.TrimSpace(line)
	indent := len(line) - len(strings.TrimLeft(line, " "))

	// Top-level key completion (indent 0)
	if indent == 0 && (trimmed == "" || !strings.Contains(trimmed, ":")) {
		return keyCompletions(topLevelKeys, protocol.CompletionItemKindField)
	}

	// After "approach:" suggest approach values
	if strings.HasPrefix(trimmed, "approach:") || strings.HasPrefix(trimmed, "approach: ") {
		return valueCompletions(approachValues, protocol.CompletionItemKindEnum)
	}

	// After "persona:" suggest persona values
	if strings.HasPrefix(trimmed, "persona:") || strings.HasPrefix(trimmed, "persona: ") {
		return valueCompletions(personaValues, protocol.CompletionItemKindEnum)
	}

	// Node references in "from:", "to:", "start:"
	if (strings.HasPrefix(trimmed, "from:") || strings.HasPrefix(trimmed, "to:") ||
		strings.HasPrefix(trimmed, "start:")) && doc.Def != nil {
		return nodeNameCompletions(doc)
	}

	// Zone nodes: field at non-zero indent (inside a zone definition)
	if indent > 0 && isZoneNodesValue(lines, int(pos.Line)) {
		return nodeNameCompletions(doc)
	}

	// Node field completion (indent ~4-6, inside nodes list)
	ctx := guessContext(lines, int(pos.Line))
	switch ctx {
	case ctxNodes:
		return keyCompletions(nodeFieldKeys, protocol.CompletionItemKindField)
	case ctxEdges:
		return keyCompletions(edgeFieldKeys, protocol.CompletionItemKindField)
	case ctxWalkers:
		if isStepAffinityChild(lines, int(pos.Line)) {
			return nodeNameCompletions(doc)
		}
		return keyCompletions(walkerFieldKeys, protocol.CompletionItemKindField)
	}

	return nil
}

func guessContext(lines []string, curLine int) string {
	for i := curLine; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		switch {
		case line == "nodes:" || strings.HasPrefix(line, "nodes:"):
			return ctxNodes
		case line == "edges:" || strings.HasPrefix(line, "edges:"):
			return ctxEdges
		case line == "walkers:" || strings.HasPrefix(line, "walkers:"):
			return ctxWalkers
		case line == "zones:" || strings.HasPrefix(line, "zones:"):
			return ctxZones
		}
	}
	return ""
}

func keyCompletions(keys []string, kind protocol.CompletionItemKind) []protocol.CompletionItem {
	items := make([]protocol.CompletionItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, protocol.CompletionItem{
			Label:      k,
			Kind:       kind,
			InsertText: k + ": ",
		})
	}
	return items
}

func valueCompletions(values []string, kind protocol.CompletionItemKind) []protocol.CompletionItem {
	items := make([]protocol.CompletionItem, 0, len(values))
	for _, v := range values {
		items = append(items, protocol.CompletionItem{
			Label: v,
			Kind:  kind,
		})
	}
	return items
}

func nodeNameCompletions(doc *document) []protocol.CompletionItem {
	if doc.Def == nil {
		return nil
	}
	names := make([]string, 0, len(doc.Def.Nodes))
	for i := range doc.Def.Nodes {
		names = append(names, doc.Def.Nodes[i].Name)
	}
	return valueCompletions(names, protocol.CompletionItemKindReference)
}

// isZoneNodesValue checks if the line is inside a zone's `nodes:` field value.
// Zone nodes use inline syntax like `nodes: [recall, triage]`.
func isZoneNodesValue(lines []string, curLine int) bool {
	trimmed := strings.TrimSpace(lines[curLine])
	return strings.HasPrefix(trimmed, "nodes:") || strings.HasPrefix(trimmed, "nodes: ")
}

// isStepAffinityChild checks if the cursor is on a key line inside a
// step_affinity map (node names as keys with float values).
func isStepAffinityChild(lines []string, curLine int) bool {
	indent := len(lines[curLine]) - len(strings.TrimLeft(lines[curLine], " "))
	for i := curLine - 1; i >= 0; i-- {
		line := lines[i]
		lineIndent := len(line) - len(strings.TrimLeft(line, " "))
		if lineIndent < indent {
			return strings.TrimSpace(line) == "step_affinity:" ||
				strings.HasPrefix(strings.TrimSpace(line), "step_affinity:")
		}
	}
	return false
}