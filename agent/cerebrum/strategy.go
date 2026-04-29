package cerebrum

import (
	"context"
	"fmt"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/instrument"
	"github.com/dpopsuev/tako/memory"
)

type Classifier interface {
	Classify(m *reactivity.Molecule) Domain
}

type ClassifierFunc func(m *reactivity.Molecule) Domain

func (f ClassifierFunc) Classify(m *reactivity.Molecule) Domain { return f(m) }

type PromptBuilder interface {
	Build(m *reactivity.Molecule, need []byte, domain Domain, shell instrument.Shell, recollected []reactivity.Atom) string
}

type PromptBuilderFunc func(m *reactivity.Molecule, need []byte, domain Domain, shell instrument.Shell, recollected []reactivity.Atom) string

func (f PromptBuilderFunc) Build(m *reactivity.Molecule, need []byte, domain Domain, shell instrument.Shell, recollected []reactivity.Atom) string {
	return f(m, need, domain, shell, recollected)
}

type ResponseParser interface {
	Parse(raw string, phase reactivity.AtomType, turn int) ([]reactivity.Atom, *ToolCall, error)
}

type ResponseParserFunc func(raw string, phase reactivity.AtomType, turn int) ([]reactivity.Atom, *ToolCall, error)

func (f ResponseParserFunc) Parse(raw string, phase reactivity.AtomType, turn int) ([]reactivity.Atom, *ToolCall, error) {
	return f(raw, phase, turn)
}

type Recollector interface {
	Recollect(mesh memory.Mesh, need []byte) []reactivity.Atom
}

type RecollectorFunc func(mesh memory.Mesh, need []byte) []reactivity.Atom

func (f RecollectorFunc) Recollect(mesh memory.Mesh, need []byte) []reactivity.Atom {
	return f(mesh, need)
}

type Dispatcher interface {
	Dispatch(ctx context.Context, shell instrument.Shell, tc *ToolCall) (reactivity.Atom, error)
}

type DispatcherFunc func(ctx context.Context, shell instrument.Shell, tc *ToolCall) (reactivity.Atom, error)

func (f DispatcherFunc) Dispatch(ctx context.Context, shell instrument.Shell, tc *ToolCall) (reactivity.Atom, error) {
	return f(ctx, shell, tc)
}

var (
	DefaultClassifier    Classifier    = ClassifierFunc(Classify)
	DefaultPromptBuilder PromptBuilder = PromptBuilderFunc(buildPrompt)
	DefaultParser        ResponseParser = ResponseParserFunc(ParseResponse)
	DefaultRecollector   Recollector   = RecollectorFunc(recollect)
	DefaultDispatcher    Dispatcher    = DispatcherFunc(dispatch)

	NaivePromptBuilder PromptBuilder = PromptBuilderFunc(naivePrompt)
	NullRecollector    Recollector   = RecollectorFunc(func(_ memory.Mesh, _ []byte) []reactivity.Atom { return nil })
	RawParser          ResponseParser = ResponseParserFunc(rawParse)
	FixedClassifier    Classifier    = ClassifierFunc(func(_ *reactivity.Molecule) Domain { return Complicated })
)

func naivePrompt(m *reactivity.Molecule, need []byte, _ Domain, _ instrument.Shell, _ []reactivity.Atom) string {
	return fmt.Sprintf("phase:%s mass:%d need:%s", m.Phase(), m.Mass(m.Phase()), string(need))
}

func rawParse(raw string, phase reactivity.AtomType, turn int) ([]reactivity.Atom, *ToolCall, error) {
	return fallbackParse(raw, phase, turn), nil, nil
}
