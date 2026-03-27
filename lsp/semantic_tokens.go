package lsp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// Origami element token types registered with the LSP client.
// The index in this slice is the token type ID used in the encoded data.
var semanticTokenTypes = []string{
	"origami-fire",
	"origami-water",
	"origami-earth",
	"origami-air",
	"origami-diamond",
	"origami-lightning",
}

var approachTokenIndex = map[string]uint32{
	"rapid":      0, // fire
	"analytical": 1, // water
	"methodical": 2, // earth
	"holistic":   3, // air
	"rigorous":   4, // diamond
	"aggressive": 5, // lightning
}

// SemanticTokensLegend returns the legend for initialize response.
func SemanticTokensLegend() map[string]any {
	return map[string]any{
		"tokenTypes":     semanticTokenTypes,
		"tokenModifiers": []string{},
	}
}

// SemanticTokensProvider returns the provider capability for initialize response.
func SemanticTokensProvider() map[string]any {
	return map[string]any{
		"legend": SemanticTokensLegend(),
		"full":   true,
	}
}

func (s *Server) handleSemanticTokensFull(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CompletionParams // reuse for URI extraction
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	doc := s.getDocument(params.TextDocument.URI)
	if doc == nil {
		return reply(ctx, map[string]any{"data": []uint32{}}, nil)
	}

	data := computeSemanticTokens(doc)
	return reply(ctx, map[string]any{"data": data}, nil)
}

type tokenHit struct {
	line      uint32
	startChar uint32
	length    uint32
	tokenType uint32
}

func computeSemanticTokens(doc *document) []uint32 {
	lines := strings.Split(doc.Content, "\n")
	var hits []tokenHit

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "approach:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "approach:"))
			if idx, ok := approachTokenIndex[val]; ok {
				col := strings.Index(line, val)
				if col >= 0 {
					hits = append(hits, tokenHit{
						line:      uint32(i),
						startChar: uint32(col),
						length:    uint32(len(val)),
						tokenType: idx,
					})
				}
			}
		}
	}

	if doc.Def != nil {
		hits = appendZoneTokens(hits, doc.Def, lines)
	}

	return encodeTokens(hits)
}

func appendZoneTokens(hits []tokenHit, def *circuit.CircuitDef, lines []string) []tokenHit {
	for zoneName, zone := range def.Zones {
		if zone.Approach == "" {
			continue
		}
		idx, ok := approachTokenIndex[strings.ToLower(zone.Approach)]
		if !ok {
			continue
		}
		hits = appendZoneLineHits(hits, lines, zoneName, idx)
	}
	return hits
}

func appendZoneLineHits(hits []tokenHit, lines []string, zoneName string, tokenType uint32) []tokenHit {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != zoneName+":" && !strings.HasPrefix(trimmed, zoneName+":") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		ctx := guessContext(lines, i)
		if ctx != "zones" || indent <= 0 {
			continue
		}
		col := strings.Index(line, zoneName)
		if col >= 0 {
			hits = append(hits, tokenHit{
				line:      uint32(i),
				startChar: uint32(col),
				length:    uint32(len(zoneName)),
				tokenType: tokenType,
			})
		}
	}
	return hits
}

// encodeTokens converts absolute positions to LSP-relative encoding:
// [deltaLine, deltaStartChar, length, tokenType, tokenModifiers]
func encodeTokens(hits []tokenHit) []uint32 {
	if len(hits) == 0 {
		return []uint32{}
	}

	data := make([]uint32, 0, len(hits)*5)
	var prevLine, prevChar uint32

	for _, h := range hits {
		deltaLine := h.line - prevLine
		deltaChar := h.startChar
		if deltaLine == 0 {
			deltaChar = h.startChar - prevChar
		}

		data = append(data, deltaLine, deltaChar, h.length, h.tokenType, 0)

		prevLine = h.line
		prevChar = h.startChar
	}

	return data
}
