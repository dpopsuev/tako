package fab

import "github.com/dpopsuev/tako/artifact"

// AlwaysPass is a ContractEvaluator that always returns true.
type AlwaysPass struct{}

var _ ContractEvaluator = AlwaysPass{}

func (AlwaysPass) Evaluate(_ Contract, _ artifact.Envelope) (bool, error) {
	return true, nil
}

// StubAssembly creates a minimal 2-station linear assembly for testing.
func StubAssembly() Assembly {
	return Assembly{
		Name: "stub",
		Stations: map[string]Station{
			"intake":   {Name: "intake", Instruments: []string{"echo"}, Intake: true},
			"terminus": {Name: "terminus", Terminus: true},
		},
		Contracts: []Contract{
			{From: "intake", To: "terminus", Evaluator: AlwaysPass{}},
		},
	}
}
