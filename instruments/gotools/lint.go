package gotools

import (
	"context"
	"sync"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

// LintTransformer runs `golangci-lint run ./...` on the target repository.
type LintTransformer struct {
	repoPath string

	mu             sync.Mutex
	lastStationLog trace.StationLogger
}

// NewLintTransformer creates a lint transformer for the given repository.
func NewLintTransformer(repoPath string) *LintTransformer {
	return &LintTransformer{repoPath: repoPath}
}

// Name implements engine.Transformer.
func (l *LintTransformer) Name() string { return "lint" }

// LastStationLog implements engine.StationLoggable.
func (l *LintTransformer) LastStationLog() trace.StationLogger {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastStationLog
}

// Transform implements engine.Transformer.
func (l *LintTransformer) Transform(ctx context.Context, _ *engine.TransformerContext) (any, error) {
	r := runCommand(ctx, l.repoPath, "golangci-lint", "run", "./...")
	l.mu.Lock()
	l.lastStationLog = buildStationLog(r)
	l.mu.Unlock()

	return &sdlctype.LintResult{
		Pass:   r.pass,
		Output: r.output,
	}, nil
}

var _ engine.Transformer = (*LintTransformer)(nil)
