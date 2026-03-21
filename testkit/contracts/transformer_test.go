package contracts_test

import (
	"testing"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/testkit/contracts"
	"github.com/dpopsuev/origami/testkit/stubs"
)

func TestStubTransformer_PassesContract(t *testing.T) {
	contracts.RunTransformerContract(t, func() framework.Transformer {
		return stubs.NewStubTransformer("test-stub", nil)
	})
}
