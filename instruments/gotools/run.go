package gotools

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/dpopsuev/tako/simulate/sdlc/sdlctype"
)

// cmdResult holds the result of running a shell command.
type cmdResult struct {
	pass    bool
	output  string
	elapsed time.Duration
}

// runCommand executes a command and captures output.
func runCommand(ctx context.Context, dir, name string, args ...string) cmdResult {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)
	output := strings.TrimSpace(stderr.String() + stdout.String())

	return cmdResult{
		pass:    err == nil,
		output:  output,
		elapsed: elapsed,
	}
}

// buildStationLog creates a BuildStationLog from a command result.
func buildStationLog(r cmdResult) *sdlctype.BuildStationLog {
	snippet := r.output
	if len(snippet) > outputSnippetMax {
		snippet = snippet[:outputSnippetMax]
	}
	return &sdlctype.BuildStationLog{
		Pass:          r.pass,
		OutputSnippet: snippet,
		DurationMs:    r.elapsed.Milliseconds(),
	}
}
