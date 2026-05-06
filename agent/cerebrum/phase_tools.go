package cerebrum

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

type phaseToolInput struct {
	Taxonomy   string   `json:"taxonomy"`
	Content    string   `json:"content"`
	Dimensions []string `json:"dimensions"`
}

var phaseToolNames = map[string]reactivity.AtomType{
	"intent":         reactivity.IntentAtom,
	"assessment":     reactivity.AssessmentAtom,
	"knowledge":      reactivity.KnowledgeAtom,
	"expansion":      reactivity.ExpansionAtom,
	"reduction":      reactivity.ReductionAtom,
	"selection":      reactivity.SelectionAtom,
	"execution":      reactivity.ExecutionAtom,
	"acclimation":    reactivity.AcclimationAtom,
	"refinement":     reactivity.RefinementAtom,
	"retrospection":  reactivity.RetrospectionAtom,
}

var phaseToolSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"taxonomy":   {"type": "string", "description": "atom taxonomy (e.g. assessment.constraint)"},
		"content":    {"type": "string", "description": "your reasoning for this phase"},
		"dimensions": {"type": "array", "items": {"type": "string"}, "description": "which Desired dimensions this addresses"}
	},
	"required": ["taxonomy", "content", "dimensions"]
}`)

func phaseToolFor(phase reactivity.AtomType) tangle.Tool {
	name := phase.String()
	return tangle.Tool{
		Name:        name,
		Description: instructionsForPhase(phase),
		InputSchema: phaseToolSchema,
	}
}

func isPhaseToolCall(name string) bool {
	_, ok := phaseToolNames[name]
	return ok
}

func phaseToolCallToAtom(tc tangle.ToolCall, currentPhase reactivity.AtomType, turn int) (reactivity.Atom, error) {
	var input phaseToolInput
	if err := json.Unmarshal(tc.Input, &input); err != nil {
		return reactivity.Atom{}, fmt.Errorf("phase tool %s: %w", tc.Name, err)
	}

	atomType, ok := phaseToolNames[tc.Name]
	if !ok {
		atomType = currentPhase
	}

	taxonomy := input.Taxonomy
	if taxonomy == "" {
		taxonomy = fmt.Sprintf("%s.response.turn-%d", atomType, turn)
	}

	return reactivity.Atom{
		ID:         fmt.Sprintf("atom-%s-%d", atomType, turn),
		Type:       atomType,
		Source:     reactivity.Fresh,
		Taxonomy:   taxonomy,
		Content:    []byte(input.Content),
		Dimensions: input.Dimensions,
		CreatedAt:  time.Now(),
	}, nil
}
