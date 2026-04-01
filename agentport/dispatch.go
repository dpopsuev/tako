// dispatch.go — dispatch interfaces and types.
// These are Origami-owned types (absorbed from Jericho v0.1.0).
// Defined here (agentport) to avoid import cycles between engine/ and dispatch/.
package agentport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

// DiscardLogger returns a logger that discards all output.
func DiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

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

// UnwrapFinalizer walks the dispatcher chain and returns the first Finalizer.
func UnwrapFinalizer(d Dispatcher) Finalizer {
	for d != nil {
		if f, ok := d.(Finalizer); ok {
			return f
		}
		if u, ok := d.(Unwrapper); ok {
			d = u.Inner()
			continue
		}
		return nil
	}
	return nil
}

// StdinTemplate defines instructions for interactive dispatch.
type StdinTemplate struct {
	Instructions []string
}

// DefaultStdinTemplate returns generic instructions.
func DefaultStdinTemplate() StdinTemplate {
	return StdinTemplate{
		Instructions: []string{
			"1. Open the prompt file and process it",
			"2. Save the JSON response to the artifact path above",
			"3. Press Enter to continue",
		},
	}
}

// StdinDispatcher delivers prompts via stdout/stdin interaction.
type StdinDispatcher struct {
	reader   *bufio.Reader
	template StdinTemplate
}

// NewStdinDispatcher creates a dispatcher that reads from os.Stdin.
func NewStdinDispatcher() *StdinDispatcher {
	return &StdinDispatcher{reader: bufio.NewReader(os.Stdin), template: DefaultStdinTemplate()}
}

// NewStdinDispatcherWithTemplate creates a dispatcher with custom instructions.
func NewStdinDispatcherWithTemplate(t StdinTemplate) *StdinDispatcher {
	return &StdinDispatcher{reader: bufio.NewReader(os.Stdin), template: t}
}

// Dispatch prints a banner and blocks on stdin.
func (d *StdinDispatcher) Dispatch(_ context.Context, ctx Context) ([]byte, error) {
	fmt.Println()
	fmt.Printf("  Case: %-6s  Step: %s\n", ctx.CaseID, ctx.Step)
	fmt.Printf("  Prompt:   %s\n", ctx.PromptPath)
	fmt.Printf("  Artifact: %s\n", ctx.ArtifactPath)
	for _, line := range d.template.Instructions {
		fmt.Printf("  %s\n", line)
	}
	fmt.Print("  > ")
	_, _ = d.reader.ReadString('\n')

	data, err := os.ReadFile(ctx.ArtifactPath)
	if err != nil {
		return nil, fmt.Errorf("artifact not found at %s: %w", ctx.ArtifactPath, err)
	}
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", ctx.ArtifactPath, err)
	}
	return raw, nil
}
