package rehearsal

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type Handle string

type ExecResult struct {
	ExitCode int32
	Stdout   string
	Stderr   string
}

type Sandbox interface {
	Create(ctx context.Context, level string) (Handle, error)
	Exec(ctx context.Context, handle Handle, cmd []string, timeout int64) (ExecResult, error)
	Destroy(ctx context.Context, handle Handle) error
	Name() string
}

type NoopSandbox struct {
	WorkDir string
}

func (n *NoopSandbox) Create(_ context.Context, _ string) (Handle, error) {
	return Handle(n.WorkDir), nil
}

func (n *NoopSandbox) Exec(ctx context.Context, _ Handle, cmd []string, _ int64) (ExecResult, error) {
	if len(cmd) == 0 {
		return ExecResult{}, fmt.Errorf("empty command")
	}
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	c.Dir = n.WorkDir
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	exitCode := int32(0)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = int32(exitErr.ExitCode())
		} else {
			return ExecResult{}, err
		}
	}
	return ExecResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

func (n *NoopSandbox) Destroy(_ context.Context, _ Handle) error {
	return nil
}

func (n *NoopSandbox) Name() string { return "noop" }
