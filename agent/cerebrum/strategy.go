package cerebrum

import (
	"fmt"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type Classifier interface {
	Classify(m *reactivity.Molecule) Domain
}

type ClassifierFunc func(m *reactivity.Molecule) Domain

func (f ClassifierFunc) Classify(m *reactivity.Molecule) Domain { return f(m) }

type PromptBuilder interface {
	Build(m *reactivity.Molecule, need []byte, domain Domain) string
}

type PromptBuilderFunc func(m *reactivity.Molecule, need []byte, domain Domain) string

func (f PromptBuilderFunc) Build(m *reactivity.Molecule, need []byte, domain Domain) string {
	return f(m, need, domain)
}

type ResponseParser interface {
	Parse(raw string, phase reactivity.AtomType, turn int) ([]reactivity.Atom, *ToolCall, error)
}

type ResponseParserFunc func(raw string, phase reactivity.AtomType, turn int) ([]reactivity.Atom, *ToolCall, error)

func (f ResponseParserFunc) Parse(raw string, phase reactivity.AtomType, turn int) ([]reactivity.Atom, *ToolCall, error) {
	return f(raw, phase, turn)
}

var (
	DefaultClassifier    Classifier     = ClassifierFunc(Classify)
	DefaultPromptBuilder PromptBuilder  = PromptBuilderFunc(naivePrompt)
	DefaultParser        ResponseParser = ResponseParserFunc(ParseResponse)

	BasicPromptBuilder PromptBuilder  = PromptBuilderFunc(naivePrompt)
	PlainTextParser    ResponseParser = ResponseParserFunc(rawParse)
	StaticClassifier   Classifier     = ClassifierFunc(func(_ *reactivity.Molecule) Domain { return Complicated })
)

func naivePrompt(m *reactivity.Molecule, need []byte, _ Domain) string {
	return fmt.Sprintf("phase:%s mass:%d need:%s", m.Phase(), m.Mass(m.Phase()), string(need))
}

func rawParse(raw string, phase reactivity.AtomType, turn int) ([]reactivity.Atom, *ToolCall, error) {
	return fallbackParse(raw, phase, turn), nil, nil
}
