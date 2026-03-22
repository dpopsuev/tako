package contracts_test

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/testkit/contracts"
)

func TestProcessWalker_PassesContract(t *testing.T) {
	contracts.RunWalkerContract(t, func() circuit.Walker {
		return circuit.NewProcessWalker("test-case")
	})
}
