package toolkit

import "context"

// Transport serves a circuit via a specific protocol (MCP, HTTP, CLI).
// Fold codegen calls Transport.Serve() in the generated main.go.
type Transport interface {
	Serve(ctx context.Context, handler TransportHandler) error
	Shutdown(ctx context.Context) error
}

// Trigger starts a circuit session from an external event.
type Trigger interface {
	Start(ctx context.Context, params TriggerParams) (SessionHandle, error)
}

// TriggerParams are the inputs to Trigger.Start.
type TriggerParams struct {
	Parallel int
	Force    bool
	Extra    map[string]any
}

// SessionHandle is a running circuit session returned by Trigger.Start.
type SessionHandle interface {
	ID() string
	Done() <-chan struct{}
	Result() any
	Err() error
	Cancel()
}

// TransportHandler bridges a Transport to the circuit engine.
// The framework provides a default implementation; transports adapt
// their protocol to these methods.
type TransportHandler interface {
	StartSession(ctx context.Context, params TriggerParams) (SessionHandle, error)
	SubmitArtifact(ctx context.Context, sessionID string, dispatchID int64, step string, data []byte) error
	GetNextStep(ctx context.Context, sessionID string, timeoutMs int) (StepContext, error)
	GetReport(ctx context.Context, sessionID string) (any, error)
}

// StepContext carries the data for a single dispatched step.
type StepContext struct {
	Done          bool
	Available     bool
	CaseID        string
	Step          string
	PromptPath    string
	PromptContent string
	DispatchID    int64
}
