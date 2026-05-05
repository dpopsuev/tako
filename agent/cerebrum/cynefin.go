package cerebrum

import "github.com/dpopsuev/tako/agent/reactivity"

type Domain int

const (
	Clear Domain = iota
	Complicated
	Complex
	Chaotic
)

func (d Domain) String() string {
	return [...]string{"clear", "complicated", "complex", "chaotic"}[d]
}

func Classify(m *reactivity.Molecule) Domain {
	return ClassifyWithConfig(m, &reactivity.DefaultConfig)
}

func ClassifyWithConfig(m *reactivity.Molecule, cfg *reactivity.Config) Domain {
	total := m.TotalMass()
	unseals := m.UnsealCount()
	recollected := m.SourceMass(reactivity.Recollected)

	if unseals >= cfg.ChaoticUnsealMin {
		return Chaotic
	}
	if unseals >= cfg.ComplexUnsealMin && total > cfg.ComplexMassMin {
		return Complex
	}
	if total > 0 && float64(recollected)/float64(total) > cfg.ClearRecollectionMin && total < cfg.ClearMassMax {
		return Clear
	}
	return Complicated
}
