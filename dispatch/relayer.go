package dispatch

import (
	"context"

	"github.com/dpopsuev/origami/circuit"
)

// MuxRelayer wraps a MuxDispatcher as a circuit.PromptRelayer.
// This bridges the mediator delegate's prompt relay interface with
// the MuxDispatcher's concrete Dispatch method.
type MuxRelayer struct {
	Disp *MuxDispatcher
}

var _ circuit.PromptRelayer = (*MuxRelayer)(nil)

func (r *MuxRelayer) Dispatch(ctx context.Context, rc circuit.PromptRelayContext) ([]byte, error) {
	return r.Disp.Dispatch(ctx, Context{
		CaseID:        rc.CaseID,
		Step:          rc.Step,
		PromptContent: rc.PromptContent,
		ArtifactPath:  rc.ArtifactPath,
	})
}
