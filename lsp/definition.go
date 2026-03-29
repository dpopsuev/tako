package lsp

import (
	"context"
	"encoding/json"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func (s *Server) handleDefinition(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DefinitionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	doc := s.getDocument(params.TextDocument.URI)
	if doc == nil || doc.Def == nil || doc.LintCtx == nil {
		return reply(ctx, nil, nil)
	}

	loc := computeDefinition(doc, params.Position)
	if loc == nil {
		return reply(ctx, nil, nil)
	}
	return reply(ctx, loc, nil)
}

func computeDefinition(doc *document, pos protocol.Position) *protocol.Location {
	lines := strings.Split(doc.Content, "\n")
	if int(pos.Line) >= len(lines) {
		return nil
	}

	line := lines[pos.Line]
	trimmed := strings.TrimSpace(line)

	var targetNode string

	// Navigate from edge from/to or start to node definition
	for _, prefix := range []string{"from:", "to:", "start:"} {
		if strings.HasPrefix(trimmed, prefix) {
			targetNode = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			break
		}
	}

	// Navigate from zone nodes: [a, b] list items
	if targetNode == "" {
		targetNode = zoneNodeAtCursor(lines, int(pos.Line), pos.Character)
	}

	if targetNode == "" || doc.LintCtx == nil {
		return nil
	}

	nodeLine := doc.LintCtx.NodeLine(targetNode)
	if nodeLine <= 0 {
		return nil
	}

	return &protocol.Location{
		URI: doc.URI,
		Range: protocol.Range{
			Start: protocol.Position{Line: safeUint32(nodeLine - 1), Character: 0},
			End:   protocol.Position{Line: safeUint32(nodeLine - 1), Character: 0},
		},
	}
}

// zoneNodeAtCursor extracts a node name from a zone `nodes: [a, b]` line
// at the given cursor position. Returns "" if the line isn't a zone nodes list.
func zoneNodeAtCursor(lines []string, lineIdx int, char uint32) string {
	line := lines[lineIdx]
	indent := len(line) - len(strings.TrimLeft(line, " "))
	if indent == 0 {
		return ""
	}
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "nodes:") {
		return ""
	}

	bracketStart := strings.Index(line, "[")
	bracketEnd := strings.Index(line, "]")
	if bracketStart < 0 || bracketEnd < 0 || bracketEnd <= bracketStart {
		return ""
	}

	inner := line[bracketStart+1 : bracketEnd]
	parts := strings.Split(inner, ",")
	offset := bracketStart + 1
	for _, part := range parts {
		name := strings.TrimSpace(part)
		nameStart := offset + strings.Index(part, name)
		nameEnd := nameStart + len(name)
		if int(char) >= nameStart && int(char) <= nameEnd && name != "" {
			return name
		}
		offset += len(part) + 1 // +1 for comma
	}
	return ""
}
