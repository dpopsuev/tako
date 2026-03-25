package contracts

import (
	"testing"

	"github.com/dpopsuev/origami/testkit/stubs"
	"github.com/dpopsuev/origami/toolkit"
)

func TestStubTransport_PassesContract(t *testing.T) {
	RunTransportContract(t, func() toolkit.Transport {
		return stubs.NewStubTransport()
	})
}

func TestStubTrigger_PassesContract(t *testing.T) {
	handle := stubs.NewStubSessionHandle("contract-test")
	RunTriggerContract(t, func() toolkit.Trigger {
		tr := stubs.NewStubTrigger()
		tr.WithHandle(handle)
		return tr
	}, handle)
}
