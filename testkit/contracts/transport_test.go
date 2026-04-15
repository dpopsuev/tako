package contracts

import (
	"testing"

	"github.com/dpopsuev/origami/testkit"
	"github.com/dpopsuev/origami/testkit/stubs"
)

func TestStubTransport_PassesContract(t *testing.T) {
	RunTransportContract(t, func() testkit.Transport {
		return stubs.NewStubTransport()
	})
}

func TestStubTrigger_PassesContract(t *testing.T) {
	handle := stubs.NewStubSessionHandle("contract-test")
	RunTriggerContract(t, func() testkit.Trigger {
		tr := stubs.NewStubTrigger()
		tr.WithHandle(handle)
		return tr
	}, handle)
}
