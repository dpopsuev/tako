package fab

import (
	"github.com/dpopsuev/tako/artifact"
)

// Contract defines the transition condition between two Stations.
type Contract struct {
	From      string
	To        string
	Evaluator ContractEvaluator
}

// ContractEvaluator decides whether an Envelope satisfies a Contract.
type ContractEvaluator interface {
	Evaluate(contract Contract, envelope artifact.Envelope) (bool, error)
}
