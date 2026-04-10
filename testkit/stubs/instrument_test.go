package stubs_test

import (
	"errors"
	"testing"

	"github.com/dpopsuev/battery/tool"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/testkit/contracts"
	"github.com/dpopsuev/origami/testkit/stubs"
)

func TestStubInstrumentTool_Contract(t *testing.T) {
	contracts.RunInstrumentToolContract(t, func() tool.Tool {
		return stubs.NewStubInstrumentTool("test-scan", "test scanner")
	})
}

func TestStubInstrumentNode_Contract(t *testing.T) {
	contracts.RunInstrumentNodeContract(t, func() circuit.Node {
		tl := stubs.NewStubInstrumentTool("test-scan", "test scanner")
		return stubs.NewStubInstrumentNode(tl)
	})
}

func TestStubInstrumentTool_ErrorInjection(t *testing.T) {
	tl := stubs.NewStubInstrumentTool("scan", "scanner")
	injected := errors.New("connection refused")
	tl.SetError(injected)

	_, err := tl.Execute(t.Context(), nil)
	if !errors.Is(err, injected) {
		t.Errorf("expected injected error, got %v", err)
	}
}

func TestStubInstrumentTool_CallTracking(t *testing.T) {
	tl := stubs.NewStubInstrumentTool("scan", "scanner")
	tl.Execute(t.Context(), []byte(`{"repo":"."}`))
	tl.Execute(t.Context(), []byte(`{"repo":"/tmp"}`))

	if tl.CallCount() != 2 {
		t.Errorf("CallCount = %d, want 2", tl.CallCount())
	}
	calls := tl.Calls()
	if string(calls[0]) != `{"repo":"."}` {
		t.Errorf("calls[0] = %s", calls[0])
	}
}
