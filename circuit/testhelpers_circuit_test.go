package circuit

// Shared test helpers for circuit/ test files.

type testArtifact struct {
	typeName   string
	confidence float64
	raw        any
}

func (a *testArtifact) Type() string        { return a.typeName }
func (a *testArtifact) Confidence() float64 { return a.confidence }
func (a *testArtifact) Raw() any            { return a.raw }
