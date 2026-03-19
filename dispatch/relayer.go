package dispatch

import (
	"context"

	framework "github.com/dpopsuev/origami"
)

// MuxRelayer wraps a MuxDispatcher as a framework.PromptRelayer.
// This bridges the mediator delegate's prompt relay interface with
// the MuxDispatcher's concrete Dispatch method.
type MuxRelayer struct {
	Disp *MuxDispatcher
}

var _ framework.PromptRelayer = (*MuxRelayer)(nil)

func (r *MuxRelayer) Dispatch(ctx context.Context, rc framework.PromptRelayContext) ([]byte, error) {
	return r.Disp.Dispatch(ctx, DispatchContext{
		CaseID:        rc.CaseID,
		Step:          rc.Step,
		PromptContent: rc.PromptContent,
		ArtifactPath:  rc.ArtifactPath,
	})
}
