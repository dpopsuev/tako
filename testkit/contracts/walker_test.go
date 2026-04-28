package contracts_test

import (
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/testkit/contracts"
)

func TestProcessWalker_PassesContract(t *testing.T) {
	contracts.RunWalkerContract(t, func() circuit.Walker {
		return circuit.NewProcessWalker("test-case")
	})
}
