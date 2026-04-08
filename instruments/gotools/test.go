package gotools

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

// TestTransformer runs `go test ./... -short -count=1` on the target repository.
type TestTransformer struct {
	repoPath string
	args     []string
}

// TestOption configures the test transformer.
type TestOption func(*TestTransformer)

// WithTestArgs appends additional go test flags.
func WithTestArgs(args ...string) TestOption {
	return func(t *TestTransformer) { t.args = append(t.args, args...) }
}

// WithPackages replaces the default "./..." with specific package patterns.
func WithPackages(pkgs ...string) TestOption {
	return func(t *TestTransformer) { t.args = pkgs }
}

// NewTestTransformer creates a test transformer for the given repository.
func NewTestTransformer(repoPath string, opts ...TestOption) *TestTransformer {
	t := &TestTransformer{
		repoPath: repoPath,
		args:     []string{"./..."},
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// Name implements engine.Transformer.
func (t *TestTransformer) Name() string { return "go-test" }

// Transform implements engine.Transformer.
func (t *TestTransformer) Transform(ctx context.Context, _ *engine.TransformerContext) (any, error) {
	args := append([]string{"test", "-short", "-count=1"}, t.args...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = t.repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stdout.String() + stderr.String())

	total, failed := parseTestCounts(output)

	return &sdlctype.TestResult{
		Pass:   err == nil,
		Total:  total,
		Failed: failed,
		Output: output,
	}, nil // return nil error — the circuit uses output.pass for edge evaluation
}

// parseTestCounts extracts total and failed counts from go test output.
func parseTestCounts(output string) (total, failed int) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "ok"):
			total++
		case strings.HasPrefix(line, "FAIL"):
			total++
			failed++
		}
	}
	return total, failed
}
