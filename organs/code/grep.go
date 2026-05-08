package code

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dpopsuev/tako/agent/organ"
)

type grepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	Limit   int    `json:"limit"`
}

type grepFunc struct {
	root string
}

func (f *grepFunc) Description() string {
	return "Search file contents using a regex pattern. Uses ripgrep (rg) when available, falls back to built-in grep. Returns file:line: content."
}

func (f *grepFunc) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string","description":"Regex pattern to search for"},"path":{"type":"string","description":"File or directory to search"},"limit":{"type":"integer","description":"Max results (default 100)"}},"required":["pattern","path"]}`)
}

func (f *grepFunc) Execute(ctx context.Context, input json.RawMessage) (organ.Result, error) {
	var in grepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return organ.Result{}, fmt.Errorf("grep: %w", err)
	}
	if in.Pattern == "" || in.Path == "" {
		return organ.ErrorResult("grep: pattern and path required"), nil
	}

	limit := 100
	if in.Limit > 0 {
		limit = in.Limit
	}

	path := in.Path
	if f.root != "" && !filepath.IsAbs(path) {
		path = filepath.Join(f.root, path)
	}

	if rgPath, err := exec.LookPath("rg"); err == nil {
		return f.ripgrep(ctx, rgPath, in.Pattern, path, limit)
	}
	return f.fallbackGrep(ctx, in.Pattern, path, limit)
}

func (f *grepFunc) ripgrep(ctx context.Context, rgPath, pattern, path string, limit int) (organ.Result, error) {
	args := []string{
		"--line-number",
		"--no-heading",
		"--color=never",
		"--max-count", fmt.Sprintf("%d", limit),
		pattern,
		path,
	}

	cmd := exec.CommandContext(ctx, rgPath, args...)
	cmd.Dir = f.root
	out, err := cmd.CombinedOutput()

	output := string(out)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return organ.TextResult("no matches found"), nil
		}
		if output != "" {
			return organ.ErrorResult(output), nil
		}
		return organ.ErrorResult(fmt.Sprintf("rg: %v", err)), nil
	}

	if output == "" {
		return organ.TextResult("no matches found"), nil
	}
	return organ.TextResult(output), nil
}

func (f *grepFunc) fallbackGrep(_ context.Context, pattern, path string, limit int) (organ.Result, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return organ.ErrorResult(fmt.Sprintf("grep: invalid regex %q: %s", pattern, err)), nil
	}

	var sb strings.Builder
	matchCount := 0

	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if matchCount >= limit {
			return filepath.SkipAll
		}
		return grepFile(p, re, limit, &matchCount, &sb)
	})
	if err != nil {
		return organ.ErrorResult(fmt.Sprintf("grep: %s", err)), nil
	}

	if matchCount == 0 {
		return organ.TextResult("no matches found"), nil
	}
	if matchCount >= limit {
		fmt.Fprintf(&sb, "... (truncated at %d matches)\n", limit)
	}

	return organ.TextResult(sb.String()), nil
}

func grepFile(path string, re *regexp.Regexp, limit int, count *int, sb *strings.Builder) error {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			fmt.Fprintf(sb, "%s:%d: %s\n", path, lineNum, line)
			*count++
			if *count >= limit {
				return nil
			}
		}
	}
	return nil
}
