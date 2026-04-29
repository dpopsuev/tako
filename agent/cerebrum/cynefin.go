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
	total := m.TotalMass()
	unseals := m.UnsealCount()
	recollected := m.SourceMass(reactivity.Recollected)

	if unseals >= 2 {
		return Chaotic
	}
	if unseals >= 1 && total > 10 {
		return Complex
	}
	if total > 0 && float64(recollected)/float64(total) > 0.3 && total < 10 {
		return Clear
	}
	return Complicated
}
