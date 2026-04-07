// types.go — dispatch interfaces and types.
// These are Origami-owned types (absorbed from Jericho v0.1.0).
package dispatch

import (
	"context"
	"time"
)

// Dispatcher abstracts how a prompt is delivered to an external agent
// and how the resulting artifact is collected back.
type Dispatcher interface {
	Dispatch(ctx context.Context, dc Context) ([]byte, error)
}

// Context carries all the metadata a dispatcher needs to deliver
// a prompt and collect an artifact.
type Context struct {
	DispatchID    int64         `json:"dispatch_id"`
	CaseID        string        `json:"case_id"`
	Step          string        `json:"step"`
	PromptPath    string        `json:"prompt_path"`
	PromptContent string        `json:"prompt_content"`
	ArtifactPath  string        `json:"artifact_path"`
	Provider      string        `json:"provider"`
	Timeout       time.Duration `json:"timeout"`
}

// PullHints allows workers to declare preferences when pulling steps.
type PullHints struct {
	PreferredCaseID   string
	PreferredZone     string
	Stickiness        int
	ConsecutiveMisses int
}

// ExternalDispatcher is the agent-facing side of a mux dispatcher.
type ExternalDispatcher interface {
	GetNextStep(ctx context.Context) (Context, error)
	GetNextStepWithHints(ctx context.Context, hints PullHints) (Context, error)
	SubmitArtifact(ctx context.Context, dispatchID int64, data []byte) error
}

// Finalizer is for dispatchers that need post-dispatch cleanup.
type Finalizer interface {
	MarkDone(artifactPath string)
}

// Unwrapper exposes the inner dispatcher in decorator chains.
type Unwrapper interface {
	Inner() Dispatcher
}
