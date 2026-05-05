package code

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/tako/agent/shell"
)

type editInput struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type editFunc struct {
	root string
}

func (f *editFunc) Description() string {
	return "Replace an exact string in a file. old_string must match exactly once."
}

func (f *editFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File path to edit"},"old_string":{"type":"string","description":"Exact string to find (must match once)"},"new_string":{"type":"string","description":"Replacement string"}},"required":["path","old_string","new_string"]}`)
}

func (f *editFunc) Execute(ctx context.Context, input json.RawMessage) (shell.Result, error) {
	var in editInput
	if err := json.Unmarshal(input, &in); err != nil {
		return shell.Result{}, fmt.Errorf("edit: %w", err)
	}
	if in.Path == "" || in.OldString == "" {
		return shell.ErrorResult("edit: path and old_string required"), nil
	}

	path := in.Path
	if f.root != "" && !filepath.IsAbs(path) {
		path = filepath.Join(f.root, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return shell.ErrorResult(fmt.Sprintf("edit: %s", err)), nil
	}

	content := string(data)
	count := strings.Count(content, in.OldString)
	switch count {
	case 0:
		return shell.ErrorResult("edit: old_string not found in file"), nil
	case 1:
		// proceed
	default:
		return shell.ErrorResult(fmt.Sprintf("edit: old_string found %d times, must match exactly once", count)), nil
	}

	newContent := strings.Replace(content, in.OldString, in.NewString, 1)
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return shell.Result{}, fmt.Errorf("edit: %w", err)
	}

	return shell.TextResult(fmt.Sprintf("edited %s: replaced %d bytes with %d bytes", in.Path, len(in.OldString), len(in.NewString))), nil
}
