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

type InstrumentCall struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type Response struct {
	Atoms    []ParsedAtom `json:"atoms"`
	InstrumentCall *InstrumentCall    `json:"instrument_call,omitempty"`
	Done     bool         `json:"done"`
}

func ParseResponse(raw string, currentPhase reactivity.AtomType, turn int) ([]reactivity.Atom, *InstrumentCall, error) {
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return fallbackParse(raw, currentPhase, turn), nil, nil
	}

	if len(resp.Atoms) == 0 && resp.InstrumentCall == nil {
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

	return atoms, resp.InstrumentCall, nil
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
	case "knowledge":
		return reactivity.KnowledgeAtom
	case "expansion":
		return reactivity.ExpansionAtom
	case "reduction":
		return reactivity.ReductionAtom
	case "selection":
		return reactivity.SelectionAtom
	case "execution":
		return reactivity.ExecutionAtom
	case "acclimation":
		return reactivity.AcclimationAtom
	case "refinement":
		return reactivity.RefinementAtom
	case "retrospection":
		return reactivity.RetrospectionAtom
	default:
		return fallback
	}
}
