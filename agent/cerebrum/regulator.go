package cerebrum

import (
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/shell"
)

type Observer func() map[string]any

type RawContext struct {
	Need         []byte
	Observer     Observer
	Molecule     *reactivity.Molecule
	Capabilities []shell.Capability
	Domain       Domain
	Contracts    []reactivity.ContractInfo
	Directives   []reactivity.Directive
	Config       *reactivity.Config
	Turn         int
}

type Context struct {
	Need         string
	State        map[string]any
	Desired      map[string]any
	Residual     map[string]float64
	Capabilities []shell.Capability
	Phase        reactivity.AtomType
	Domain       Domain
	Contracts    []reactivity.ContractInfo
	Directives   []reactivity.Directive
	Filled       map[string]string
	Distance     float64
	DeltaDistance float64
	Turn         int
}

type Regulator interface {
	Regulate(raw RawContext) Context
}

func defaultRegulate(raw RawContext) Context {
	m := raw.Molecule

	var state map[string]any
	if raw.Observer != nil {
		state = raw.Observer()
	}

	var desired map[string]any
	if m.Catalyst() != nil {
		desired = m.Catalyst().Desired
	}

	summaryMax := reactivity.DefaultConfig.ContractSummaryMax
	if raw.Config != nil {
		summaryMax = raw.Config.ContractSummaryMax
	}

	filled := make(map[string]string, len(raw.Contracts))
	for _, c := range raw.Contracts {
		if m.Mass(c.Phase) > 0 {
			atoms := m.Atoms(c.Phase)
			if len(atoms) > 0 {
				filled[c.Phase.String()] = truncate(string(atoms[0].Content), summaryMax)
			}
		}
	}

	return Context{
		Need:         string(raw.Need),
		State:        state,
		Desired:      desired,
		Residual:     m.Residual(),
		Capabilities: raw.Capabilities,
		Phase:        m.Phase(),
		Domain:       raw.Domain,
		Contracts:    raw.Contracts,
		Directives:   raw.Directives,
		Filled:       filled,
		Distance:     m.Distance(),
		DeltaDistance: m.DeltaDistance(),
		Turn:         raw.Turn,
	}
}
