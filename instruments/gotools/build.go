// Package gotools provides build and test instruments backed by the Go toolchain.
// Each wraps exec.Command and returns typed simulate/sdlc results — same
// contract as the stub transformers.
package gotools

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

// BuildTransformer runs `go build ./...` on the target repository.
type BuildTransformer struct {
	repoPath string

	mu             sync.Mutex
	lastStationLog trace.StationLogger
}

// NewBuildTransformer creates a build transformer for the given repository.
func NewBuildTransformer(repoPath string) *BuildTransformer {
	return &BuildTransformer{repoPath: repoPath}
}

// Name implements engine.Transformer.
func (b *BuildTransformer) Name() string { return "go-build" }

// LastStationLog implements engine.StationLoggable.
func (b *BuildTransformer) LastStationLog() trace.StationLogger {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastStationLog
}

// outputSnippetMax is the maximum length of the output snippet stored in station logs.
const outputSnippetMax = 500

// Transform implements engine.Transformer.
func (b *BuildTransformer) Transform(ctx context.Context, _ *engine.TransformerContext) (any, error) {
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = b.repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)
	output := strings.TrimSpace(stderr.String() + stdout.String())

	snippet := output
	if len(snippet) > outputSnippetMax {
		snippet = snippet[:outputSnippetMax]
	}

	b.mu.Lock()
	b.lastStationLog = &sdlctype.BuildStationLog{
		Pass:          err == nil,
		OutputSnippet: snippet,
		DurationMs:    elapsed.Milliseconds(),
	}
	b.mu.Unlock()

	return &sdlctype.BuildResult{
		Pass:   err == nil,
		Output: output,
	}, nil // return nil error — the circuit uses output.pass for edge evaluation
}
