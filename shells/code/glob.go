package code

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/tako/agent/shell"
)

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

type globFunc struct {
	root string
}

func (f *globFunc) Description() string {
	return "Find files matching a glob pattern (e.g. **/*.go)"
}

func (f *globFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Glob pattern (e.g. *.go, **/*.go)"},"path":{"type":"string","description":"Base directory (default: working dir)"}},"required":["pattern"]}`)
}

func (f *globFunc) Execute(ctx context.Context, input json.RawMessage) (shell.Result, error) {
	var in globInput
	if err := json.Unmarshal(input, &in); err != nil {
		return shell.Result{}, fmt.Errorf("glob: %w", err)
	}
	if in.Pattern == "" {
		return shell.ErrorResult("glob: pattern required"), nil
	}

	base := in.Path
	if base == "" {
		base = f.root
	}

	pattern := in.Pattern
	if base != "" {
		pattern = filepath.Join(base, in.Pattern)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return shell.ErrorResult(fmt.Sprintf("glob: invalid pattern: %s", err)), nil
	}

	if len(matches) == 0 {
		return shell.TextResult("no files found"), nil
	}

	return shell.TextResult(strings.Join(matches, "\n")), nil
}
