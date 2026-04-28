package contracts_test

import (
	"testing"

	"github.com/dpopsuev/tako/engine"
	"github.com/dpopsuev/tako/testkit/contracts"
	"github.com/dpopsuev/tako/testkit/stubs"
)

func TestStubTransformer_PassesContract(t *testing.T) {
	contracts.RunInstrumentContract(t, func() engine.Instrument {
		return stubs.NewStubTransformer("test-stub", nil)
	})
}
