package contracts_test

import (
	"testing"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/testkit/contracts"
)

func TestProcessWalker_PassesContract(t *testing.T) {
	contracts.RunWalkerContract(t, func() framework.Walker {
		return framework.NewProcessWalker("test-case")
	})
}
