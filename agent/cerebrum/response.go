package cerebrum

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type ParsedAtom struct {
	Type     string   `json:"type"`
	Taxonomy string   `json:"taxonomy"`
	Content  string   `json:"content"`
	Targets  []string `json:"targets,omitempty"`
}

type ToolCall struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type Response struct {
	Atoms    []ParsedAtom `json:"atoms"`
	ToolCall *ToolCall    `json:"tool_call,omitempty"`
	Done     bool         `json:"done"`
}

func ParseResponse(raw string, currentPhase reactivity.AtomType, turn int) ([]reactivity.Atom, *ToolCall, error) {
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fallbackParse(raw, currentPhase, turn), nil, nil
	}

	if len(resp.Atoms) == 0 && resp.ToolCall == nil {
		return fallbackParse(raw, currentPhase, turn), nil, nil
	}

	atoms := make([]reactivity.Atom, 0, len(resp.Atoms))
	for i, pa := range resp.Atoms {
		atomType := parseAtomType(pa.Type, currentPhase)
		taxonomy := pa.Taxonomy
		if taxonomy == "" {
			taxonomy = fmt.Sprintf("%s.response.turn-%d", atomType, turn)
		}
		atoms = append(atoms, reactivity.Atom{
			ID:        fmt.Sprintf("atom-%s-%d-%d", atomType, turn, i),
			Type:      atomType,
			Source:    reactivity.Fresh,
			Taxonomy:  taxonomy,
			Content:   []byte(pa.Content),
			Targets:   pa.Targets,
			CreatedAt: time.Now(),
		})
	}

	return atoms, resp.ToolCall, nil
}

func fallbackParse(raw string, phase reactivity.AtomType, turn int) []reactivity.Atom {
	return []reactivity.Atom{{
		ID:        fmt.Sprintf("atom-%s-%d", phase, turn),
		Type:      phase,
		Source:    reactivity.Fresh,
		Taxonomy:  fmt.Sprintf("%s.response.turn-%d", phase, turn),
		Content:   []byte(raw),
		CreatedAt: time.Now(),
	}}
}

func parseAtomType(s string, fallback reactivity.AtomType) reactivity.AtomType {
	switch s {
	case "intent":
		return reactivity.IntentAtom
	case "assessment":
		return reactivity.AssessmentAtom
	case "plan":
		return reactivity.ExpansionAtom
	case "execution":
		return reactivity.ExecutionAtom
	case "retrospection":
		return reactivity.RetrospectionAtom
	default:
		return fallback
	}
}
