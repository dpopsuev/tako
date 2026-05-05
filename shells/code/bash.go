package code

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/dpopsuev/tako/agent/shell"
)

type bashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type bashFunc struct {
	root string
}

func (f *bashFunc) Description() string {
	return "Execute a shell command and return stdout/stderr"
}

func (f *bashFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Shell command to execute"},"timeout":{"type":"integer","description":"Timeout in seconds (default 120)"}},"required":["command"]}`)
}

func (f *bashFunc) Execute(ctx context.Context, input json.RawMessage) (shell.Result, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return shell.Result{}, fmt.Errorf("bash: %w", err)
	}
	if in.Command == "" {
		return shell.ErrorResult("bash: empty command"), nil
	}

	timeout := 120
	if in.Timeout > 0 {
		timeout = in.Timeout
	}

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", in.Command)
	if f.root != "" {
		cmd.Dir = f.root
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\nSTDERR:\n" + stderr.String()
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result += fmt.Sprintf("\nexit code: %d", exitErr.ExitCode())
		} else {
			return shell.Result{}, fmt.Errorf("bash: %w", err)
		}
	}

	return shell.TextResult(result), nil
}
