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

var (
	DefaultClassifier    Classifier    = ClassifierFunc(Classify)
	DefaultPromptBuilder PromptBuilder = PromptBuilderFunc(buildPrompt)

	BasicPromptBuilder PromptBuilder = PromptBuilderFunc(naivePrompt)
	StaticClassifier   Classifier    = ClassifierFunc(func(_ *reactivity.Molecule) Domain { return Complicated })
)

func naivePrompt(m *reactivity.Molecule, need []byte, _ Domain) string {
	return fmt.Sprintf("phase:%s mass:%d need:%s", m.Phase(), m.Mass(m.Phase()), string(need))
}
