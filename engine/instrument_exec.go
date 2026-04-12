package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// ExecDispatcher implements InstrumentDispatcher for dispatch: cli.
// It shells out to the binary + action command, sends JSON on stdin, reads JSON from stdout.
type ExecDispatcher struct {
	Binary  string // executable name from InstrumentManifest.Binary
	Command string // action-specific args from ActionDef.Command
	WorkDir string // optional working directory
}

// compile-time check.
var _ InstrumentDispatcher = (*ExecDispatcher)(nil)

func (d *ExecDispatcher) Dispatch(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	if d.Binary == "" && d.Command == "" {
		return nil, fmt.Errorf("%w: empty command", ErrInstrumentDispatch)
	}

	fullCmd := d.Binary + " " + d.Command

	//nolint:gosec // command comes from validated instrument manifest, not user input
	cmd := exec.CommandContext(ctx, "bash", "-c", fullCmd)
	if d.WorkDir != "" {
		cmd.Dir = d.WorkDir
	}

	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %w", ErrInstrumentDispatch, ctx.Err())
		}
		stderrMsg := stderr.String()
		if stderrMsg != "" {
			return nil, fmt.Errorf("%w: command failed: %w\nstderr: %s", ErrInstrumentDispatch, err, stderrMsg)
		}
		return nil, fmt.Errorf("%w: command failed: %w", ErrInstrumentDispatch, err)
	}

	return json.RawMessage(stdout.Bytes()), nil
}
