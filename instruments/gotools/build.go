// Package gotools provides build and test instruments backed by the Go toolchain.
// Each wraps exec.Command and returns typed simulate/sdlc results — same
// contract as the stub transformers.
package gotools

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/simulate/sdlc"
)

// BuildTransformer runs `go build ./...` on the target repository.
type BuildTransformer struct {
	repoPath string
}

// NewBuildTransformer creates a build transformer for the given repository.
func NewBuildTransformer(repoPath string) *BuildTransformer {
	return &BuildTransformer{repoPath: repoPath}
}

// Name implements engine.Transformer.
func (b *BuildTransformer) Name() string { return "go-build" }

// Transform implements engine.Transformer.
func (b *BuildTransformer) Transform(ctx context.Context, _ *engine.TransformerContext) (any, error) {
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = b.repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stderr.String() + stdout.String())

	return &sdlc.BuildResult{
		Pass:   err == nil,
		Output: output,
	}, nil // return nil error — the circuit uses output.pass for edge evaluation
}
