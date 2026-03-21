package stubs

import "fmt"

// StubArtifact is a generic test artifact used by all stubs.
type StubArtifact struct {
	Typ  string
	Node string
	Conf float64
	Data any
}

func NewStubArtifact(typ, node string) *StubArtifact {
	return &StubArtifact{Typ: typ, Node: node, Conf: 1.0}
}

func (a *StubArtifact) Type() string       { return a.Typ }
func (a *StubArtifact) Confidence() float64 {
	if a.Conf == 0 {
		return 1.0
	}
	return a.Conf
}
func (a *StubArtifact) Raw() any {
	if a.Data != nil {
		return a.Data
	}
	return fmt.Sprintf("%s:%s", a.Typ, a.Node)
}
