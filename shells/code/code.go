package code

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/tako/agent/shell"
)

// Capabilities returns code operation capabilities rooted at the given path.
func Capabilities(rootPath string) []shell.Capability {
	rf := &readFileFunc{root: rootPath}
	wf := &writeFileFunc{root: rootPath}
	gb := &goBuildFunc{root: rootPath}
	gt := &goTestFunc{root: rootPath}
	gv := &goVetFunc{root: rootPath}
	bf := &bashFunc{root: rootPath}
	ef := &editFunc{root: rootPath}
	gl := &globFunc{root: rootPath}
	gr := &grepFunc{root: rootPath}
	gs := &gitStatusFunc{root: rootPath}
	gd := &gitDiffFunc{root: rootPath}
	gc := &gitCommitFunc{root: rootPath}
	return []shell.Capability{
		{Name: "read_file", Description: rf.Description(), Schema: rf.InputSchema(), Mode: shell.ReadAction, Risk: 0, Source: shell.Environment, Reads: []string{"filesystem"}, Execute: rf.Execute},
		{Name: "write_file", Description: wf.Description(), Schema: wf.InputSchema(), Mode: shell.WriteAction, Risk: 0.7, Source: shell.Environment, Writes: []string{"filesystem"}, Execute: wf.Execute},
		{Name: "edit", Description: ef.Description(), Schema: ef.InputSchema(), Mode: shell.WriteAction, Risk: 0.5, Source: shell.Environment, Reads: []string{"filesystem"}, Writes: []string{"filesystem"}, Execute: ef.Execute},
		{Name: "bash", Description: bf.Description(), Schema: bf.InputSchema(), Mode: shell.WriteAction, Risk: 0.8, Source: shell.Environment, Execute: bf.Execute},
		{Name: "glob", Description: gl.Description(), Schema: gl.InputSchema(), Mode: shell.ReadAction, Risk: 0, Source: shell.Environment, Reads: []string{"filesystem"}, Execute: gl.Execute},
		{Name: "grep", Description: gr.Description(), Schema: gr.InputSchema(), Mode: shell.ReadAction, Risk: 0, Source: shell.Environment, Reads: []string{"filesystem"}, Execute: gr.Execute},
		{Name: "git_status", Description: gs.Description(), Schema: gs.InputSchema(), Mode: shell.ReadAction, Risk: 0, Source: shell.Environment, Reads: []string{"git"}, Execute: gs.Execute},
		{Name: "git_diff", Description: gd.Description(), Schema: gd.InputSchema(), Mode: shell.ReadAction, Risk: 0, Source: shell.Environment, Reads: []string{"git"}, Execute: gd.Execute},
		{Name: "git_commit", Description: gc.Description(), Schema: gc.InputSchema(), Mode: shell.WriteAction, Risk: 0.7, Source: shell.Environment, Reads: []string{"git"}, Writes: []string{"git"}, Execute: gc.Execute},
		{Name: "go_build", Description: gb.Description(), Schema: gb.InputSchema(), Mode: shell.ReadAction, Risk: 0, Source: shell.Environment, Reads: []string{"filesystem"}, Execute: gb.Execute},
		{Name: "go_test", Description: gt.Description(), Schema: gt.InputSchema(), Mode: shell.WriteAction, Risk: 0.3, Source: shell.Environment, Reads: []string{"filesystem"}, Writes: []string{"filesystem"}, Execute: gt.Execute},
		{Name: "go_vet", Description: gv.Description(), Schema: gv.InputSchema(), Mode: shell.ReadAction, Risk: 0, Source: shell.Environment, Reads: []string{"filesystem"}, Execute: gv.Execute},
	}
}

// --- read_file ---

type readFileFunc struct{ root string }

func (f *readFileFunc) Name() string        { return "read_file" }
func (f *readFileFunc) Description() string { return "Read a file's contents. Input: relative path from project root." }
func (f *readFileFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"relative file path"}},"required":["path"]}`)
}

func (f *readFileFunc) Execute(_ context.Context, input json.RawMessage) (shell.Result, error) {
	p := extractPath(input)
	if p == "" {
		return shell.ErrorResult("path is required"), nil
	}
	abs := filepath.Join(f.root, filepath.Clean(p))
	if !strings.HasPrefix(abs, f.root) {
		return shell.ErrorResult("path escapes project root"), nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return shell.ErrorResult(err.Error()), nil
	}
	return shell.TextResult(string(data)), nil
}

// --- write_file ---

type writeFileFunc struct{ root string }

func (f *writeFileFunc) Name() string        { return "write_file" }
func (f *writeFileFunc) Description() string { return "Write content to a file. Creates parent directories if needed." }
func (f *writeFileFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"relative file path"},"content":{"type":"string","description":"file content to write"}},"required":["path","content"]}`)
}

func (f *writeFileFunc) Execute(_ context.Context, input json.RawMessage) (shell.Result, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return shell.ErrorResult(fmt.Sprintf("invalid input: %v", err)), nil
	}
	if args.Path == "" {
		return shell.ErrorResult("path is required"), nil
	}
	abs := filepath.Join(f.root, filepath.Clean(args.Path))
	if !strings.HasPrefix(abs, f.root) {
		return shell.ErrorResult("path escapes project root"), nil
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return shell.ErrorResult(err.Error()), nil
	}
	if err := os.WriteFile(abs, []byte(args.Content), 0o644); err != nil {
		return shell.ErrorResult(err.Error()), nil
	}
	return shell.TextResult(fmt.Sprintf("wrote %d bytes to %s", len(args.Content), args.Path)), nil
}

// --- go_build ---

type goBuildFunc struct{ root string }

func (f *goBuildFunc) Name() string        { return "go_build" }
func (f *goBuildFunc) Description() string { return "Run go build on a package. Input: package pattern (e.g. './...' or './agent/...')." }
func (f *goBuildFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"package":{"type":"string","description":"Go package pattern","default":"./..."}},"required":[]}`)
}

func (f *goBuildFunc) Execute(ctx context.Context, input json.RawMessage) (shell.Result, error) {
	pkg := extractField(input, "package")
	if pkg == "" {
		pkg = "./..."
	}
	return runGoCmd(ctx, f.root, "build", pkg)
}

// --- go_test ---

type goTestFunc struct{ root string }

func (f *goTestFunc) Name() string        { return "go_test" }
func (f *goTestFunc) Description() string { return "Run go test on a package. Input: package pattern and optional flags." }
func (f *goTestFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"package":{"type":"string","description":"Go package pattern","default":"./..."},"flags":{"type":"string","description":"additional flags (e.g. '-race -count=1')"}},"required":[]}`)
}

func (f *goTestFunc) Execute(ctx context.Context, input json.RawMessage) (shell.Result, error) {
	pkg := extractField(input, "package")
	if pkg == "" {
		pkg = "./..."
	}
	flags := extractField(input, "flags")
	args := []string{"test", pkg}
	if flags != "" {
		args = append(args, strings.Fields(flags)...)
	}
	return runGoCmdArgs(ctx, f.root, args...)
}

// --- go_vet ---

type goVetFunc struct{ root string }

func (f *goVetFunc) Name() string        { return "go_vet" }
func (f *goVetFunc) Description() string { return "Run go vet on a package." }
func (f *goVetFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"package":{"type":"string","description":"Go package pattern","default":"./..."}},"required":[]}`)
}

func (f *goVetFunc) Execute(ctx context.Context, input json.RawMessage) (shell.Result, error) {
	pkg := extractField(input, "package")
	if pkg == "" {
		pkg = "./..."
	}
	return runGoCmd(ctx, f.root, "vet", pkg)
}

// --- helpers ---

func extractPath(input json.RawMessage) string {
	return extractField(input, "path")
}

func extractField(input json.RawMessage, field string) string {
	var obj map[string]any
	if json.Unmarshal(input, &obj) == nil {
		if v, ok := obj[field].(string); ok {
			return v
		}
	}
	var s string
	if json.Unmarshal(input, &s) == nil {
		return s
	}
	return ""
}

func runGoCmd(ctx context.Context, dir, subcmd, pkg string) (shell.Result, error) {
	return runGoCmdArgs(ctx, dir, subcmd, pkg)
}

func runGoCmdArgs(ctx context.Context, dir string, args ...string) (shell.Result, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return shell.ErrorResult(fmt.Sprintf("%s\n%s", err.Error(), string(out))), nil
	}
	output := string(out)
	if output == "" {
		output = fmt.Sprintf("go %s: ok", args[0])
	}
	return shell.TextResult(output), nil
}
