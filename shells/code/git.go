package code

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/dpopsuev/tako/agent/capability"
)

type gitStatusFunc struct{ root string }
type gitDiffFunc struct{ root string }
type gitCommitFunc struct{ root string }

func (f *gitStatusFunc) Description() string {
	return "Show git status: current branch, staged and unstaged changes"
}

func (f *gitStatusFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

func (f *gitStatusFunc) Execute(ctx context.Context, input json.RawMessage) (capability.Result, error) {
	out, err := gitExec(ctx, f.root, "status", "--porcelain", "-b")
	if err != nil {
		return capability.ErrorResult(fmt.Sprintf("git_status: %s", err)), nil
	}
	return capability.TextResult(out), nil
}

func (f *gitDiffFunc) Description() string {
	return "Show git diff for staged and unstaged changes"
}

func (f *gitDiffFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Optional file path to diff"}}}`)
}

func (f *gitDiffFunc) Execute(ctx context.Context, input json.RawMessage) (capability.Result, error) {
	var in struct {
		Path string `json:"path"`
	}
	json.Unmarshal(input, &in)

	args := []string{"diff"}
	if in.Path != "" {
		args = append(args, "--", in.Path)
	}

	out, err := gitExec(ctx, f.root, args...)
	if err != nil {
		return capability.ErrorResult(fmt.Sprintf("git_diff: %s", err)), nil
	}
	if out == "" {
		return capability.TextResult("no changes"), nil
	}
	return capability.TextResult(out), nil
}

func (f *gitCommitFunc) Description() string {
	return "Stage files and create a git commit"
}

func (f *gitCommitFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"message":{"type":"string","description":"Commit message"},"files":{"type":"array","items":{"type":"string"},"description":"Files to stage (if empty, commits all staged)"}},"required":["message"]}`)
}

func (f *gitCommitFunc) Execute(ctx context.Context, input json.RawMessage) (capability.Result, error) {
	var in struct {
		Message string   `json:"message"`
		Files   []string `json:"files"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return capability.Result{}, fmt.Errorf("git_commit: %w", err)
	}
	if in.Message == "" {
		return capability.ErrorResult("git_commit: message required"), nil
	}

	if len(in.Files) > 0 {
		args := append([]string{"add"}, in.Files...)
		if _, err := gitExec(ctx, f.root, args...); err != nil {
			return capability.ErrorResult(fmt.Sprintf("git_commit: stage failed: %s", err)), nil
		}
	}

	out, err := gitExec(ctx, f.root, "commit", "-m", in.Message)
	if err != nil {
		return capability.ErrorResult(fmt.Sprintf("git_commit: %s\n%s", err, out)), nil
	}

	hash, _ := gitExec(ctx, f.root, "rev-parse", "--short", "HEAD")
	return capability.TextResult(fmt.Sprintf("committed %s: %s", strings.TrimSpace(hash), in.Message)), nil
}

func gitExec(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if root != "" {
		cmd.Dir = root
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return stdout.String(), fmt.Errorf("%s", strings.TrimSpace(errMsg))
	}
	return stdout.String(), nil
}
