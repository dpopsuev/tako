package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

var (
	ErrCLITimeout  = errors.New("instrument/cli: command timed out")
	ErrCLINoOutput = errors.New("instrument/cli: command produced no output")
)

type Command struct {
	command     string
	args        []string
	timeout     time.Duration
	description string
}


type CommandInput struct {
	Input string `json:"input"`
}

func NewCommand(command string, description string, args ...string) (*Command, error) {
	resolved, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("instrument/cli: command %q not found: %w", command, err)
	}
	return &Command{
		command:     resolved,
		args:        args,
		timeout:     5 * time.Minute,
		description: description,
	}, nil
}

func (f *Command) Name() string        { return f.command }
func (f *Command) Description() string { return f.description }
func (f *Command) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}}}`)
}

func (f *Command) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var cli CommandInput
	if err := json.Unmarshal(input, &cli); err != nil {
		cli.Input = string(input)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, f.command, f.args...) //nolint:gosec
	cmd.Stdin = bytes.NewReader([]byte(cli.Input))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Run(); err != nil {
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			return ErrorResult(fmt.Sprintf("timeout after %v: %s", f.timeout, stderr.String())), ErrCLITimeout
		}
		return ErrorResult(fmt.Sprintf("failed: %v: %s", err, stderr.String())), err
	}

	output := stdout.String()
	if output == "" {
		return ErrorResult(stderr.String()), ErrCLINoOutput
	}

	result := TextResult(output)
	result.Content = append(result.Content, Content{
		Type: "text",
		Text: fmt.Sprintf("elapsed=%s", time.Since(start)),
	})

	return result, nil
}
