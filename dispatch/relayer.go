package dispatch

import (
	"context"

	"github.com/dpopsuev/origami/engine"
)

// MuxRelayer wraps a MuxDispatcher as a engine.PromptRelayer.
// This bridges the mediator delegate's prompt relay interface with
// the MuxDispatcher's concrete Dispatch method.
type MuxRelayer struct {
	Disp *MuxDispatcher
}

var _ engine.PromptRelayer = (*MuxRelayer)(nil)

func (r *MuxRelayer) Dispatch(ctx context.Context, rc engine.PromptRelayContext) ([]byte, error) {
	return r.Disp.Dispatch(ctx, DispatchContext{
		CaseID:        rc.CaseID,
		Step:          rc.Step,
		PromptContent: rc.PromptContent,
		ArtifactPath:  rc.ArtifactPath,
	})
}
