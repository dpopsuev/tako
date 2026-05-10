package rehearsal

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

type SubprocessActor struct {
	Binary    string
	Blueprint string
	Workspace string
	Env       []string
}

var _ Actor = (*SubprocessActor)(nil)

func (a *SubprocessActor) Run(ctx context.Context, task string) error {
	args := []string{"agent"}
	if a.Blueprint != "" {
		args = append(args, "--blueprint", a.Blueprint)
	}
	args = append(args, task)

	cmd := exec.CommandContext(ctx, a.Binary, args...)
	if a.Workspace != "" {
		cmd.Dir = a.Workspace
	}
	cmd.Env = append(cmd.Environ(), a.Env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	slog.Info("subprocess.run",
		slog.String("binary", a.Binary),
		slog.String("blueprint", a.Blueprint),
		slog.String("workspace", a.Workspace),
		slog.Duration("elapsed", elapsed),
		slog.Int("stdout_len", stdout.Len()),
		slog.Int("stderr_len", stderr.Len()))

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			slog.Warn("subprocess.exit",
				slog.Int("code", exitErr.ExitCode()),
				slog.String("stderr", truncateString(stderr.String(), 500)))
			return fmt.Errorf("tako agent exited %d: %s", exitErr.ExitCode(), truncateString(stderr.String(), 200))
		}
		return fmt.Errorf("subprocess: %w", err)
	}

	return nil
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
