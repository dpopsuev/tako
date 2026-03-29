package contracts_test

import (
	"testing"

	"github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/testkit/contracts"
)

// stubObserver is a minimal SessionObserver for contract testing.
type stubObserver struct{}

func (s *stubObserver) OnStepDispatched(_, _ string)         {}
func (s *stubObserver) OnStepCompleted(_, _ string, _ int64) {}
func (s *stubObserver) OnCircuitDone()                       {}
func (s *stubObserver) OnSessionEnd()                        {}

func TestSessionObserverContract_StubObserver(t *testing.T) {
	contracts.RunSessionObserverContract(t, func() mcp.SessionObserver {
		return &stubObserver{}
	})
}
