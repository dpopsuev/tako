package circuit

import "context"

// PromptRelayer dispatches a prompt and blocks until an artifact is returned.
// Used by MCPCircuitTransformer for sub-circuit delegation via the mediator.
type PromptRelayer interface {
	Dispatch(ctx context.Context, dc PromptRelayContext) ([]byte, error)
}

// PromptRelayContext carries the prompt data for relay dispatch.
type PromptRelayContext struct {
	CaseID        string
	Step          string
	PromptContent string
	ArtifactPath  string
}

// ContextKeyPromptRelayer is the walker context key for the integrated circuit's
// dispatcher.
const ContextKeyPromptRelayer = "_prompt_relayer"
