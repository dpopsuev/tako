package dispatch

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
)

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
