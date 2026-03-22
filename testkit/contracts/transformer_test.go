package contracts_test

import (
	"testing"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/testkit/contracts"
	"github.com/dpopsuev/origami/testkit/stubs"
)

func TestStubTransformer_PassesContract(t *testing.T) {
	contracts.RunTransformerContract(t, func() engine.Transformer {
		return stubs.NewStubTransformer("test-stub", nil)
	})
}
